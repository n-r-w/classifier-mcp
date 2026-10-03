package systemone

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/samber/mo"

	"github.com/n-r-w/classifier-mcp/internal/usecases/classify"
)

// mapRequest transmits each question unchanged with one object's task and content.
func mapRequest(request classify.Request, model string) requestDTO {
	questions := make(map[string]questionDTO, len(request.Questions))
	for id, question := range request.Questions {
		dto := questionDTO{}
		if q, isChoice := question.Arg1(); isChoice {
			dto = questionDTO{Type: q.Kind(), Instructions: q.Instructions, Criteria: mo.Some[any](q.Criteria)}
		} else if q, isNoul := question.Arg2(); isNoul {
			dto = questionDTO{Type: q.Kind(), Instructions: q.Instructions, Criteria: mo.None[any]()}
			if criteria, present := q.Criteria.Get(); present {
				dto.Criteria = mo.Some[any](criteria)
			}
		} else if q, isScore := question.Arg3(); isScore {
			dto = questionDTO{Type: q.Kind(), Instructions: q.Instructions, Criteria: mo.Some[any](q.Criteria)}
		}
		questions[id] = dto
	}
	source := mo.None[sourceDTO]()
	if location, present := request.Source.Get(); present {
		lines := mo.None[lineRangeDTO]()
		if selected, hasLines := location.Lines.Get(); hasLines {
			lines = mo.Some(
				lineRangeDTO{Start: json.Number(selected.Start.String()), End: json.Number(selected.End.String())},
			)
		}
		source = mo.Some(sourceDTO{Path: location.Path, Lines: lines})
	}
	return requestDTO{Model: model, Questions: questions, State: stateDTO{
		Task: request.Task, Content: request.Content, Source: source,
	}}
}

// Evaluate validates external answers and retries only explicit overload responses.
func (c *Client) Evaluate(
	ctx context.Context,
	request classify.Request,
) (classify.Response, error) {
	if err := ctx.Err(); err != nil {
		return classify.Response{}, err
	}
	body, err := json.Marshal(mapRequest(request, c.model))
	if err != nil {
		return classify.Response{}, fmt.Errorf("encode System One request: %w", err)
	}
	last := mo.None[attemptFailure]()
	for attempt := 1; ; attempt++ {
		if err = c.acquire(ctx); err != nil {
			return classify.Response{}, cancellationError(last, err)
		}
		result, diagnostic := c.attempt(ctx, request, body, attempt)
		<-c.capacity
		detail, failed := diagnostic.Get()
		if !failed {
			return result, nil
		}
		if detail.kind == canceledCode && detail.attempts == attempt-1 {
			return classify.Response{}, cancellationError(last, ctx.Err())
		}
		if detail.kind != "upstream_error" ||
			(detail.httpStatus.OrEmpty() != http.StatusTooManyRequests && detail.httpStatus.OrEmpty() != 529) ||
			attempt == c.maxAttempts {
			return result, detail.cause
		}
		last = diagnostic
		delay := detail.retryAfterSeconds.OrElse(c.retryDelay.Seconds())
		if err = waitRetry(ctx, delay); err != nil {
			return classify.Response{}, cancellationError(last, err)
		}
	}
}

// acquire waits for the shared HTTP capacity and checks cancellation after a racing acquisition.
func (c *Client) acquire(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.capacity <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-c.capacity
			return err
		}
		return nil
	}
}

// attempt sends one HTTP request while the caller owns a capacity slot.
func (c *Client) attempt(ctx context.Context, request classify.Request, body []byte,
	attempt int,
) (classify.Response, mo.Option[attemptFailure]) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return fail("request_failed", fmt.Errorf("create System One request: %w", err), nil, attempt-1)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	if err = ctx.Err(); err != nil {
		return fail(canceledCode, err, nil, attempt-1)
	}
	response, err := c.http.Do(req)
	if err != nil {
		return fail(
			resolveRequestFailureCode(ctx),
			fmt.Errorf("system one HTTP request failed: %w", err),
			nil,
			attempt,
		)
	}
	defer func() { _ = response.Body.Close() }()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return fail(
			resolveRequestFailureCode(ctx),
			fmt.Errorf("read System One response: %w", err),
			response,
			attempt,
		)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fail("upstream_error", errors.New(providerCause(response.StatusCode, data)),
			response, attempt)
	}
	parsed, err := decodeResponse(data, request)
	if err != nil {
		return fail("invalid_response", fmt.Errorf("invalid System One response: %w", err), response, attempt)
	}
	return parsed, mo.None[attemptFailure]()
}

// resolveRequestFailureCode distinguishes caller cancellation from ambiguous transport failures and client timeouts.
func resolveRequestFailureCode(ctx context.Context) string {
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return canceledCode
	}
	return "request_failed"
}

// cancellationError keeps the last overload cause when cancellation interrupts a retry or capacity wait.
func cancellationError(last mo.Option[attemptFailure], cause error) error {
	previous, present := last.Get()
	if !present {
		return cause
	}
	return fmt.Errorf("%w: %w", previous.cause, cause)
}

// waitRetry observes cancellation for zero delays and waits the full supplied finite delay.
func waitRetry(ctx context.Context, seconds float64) error {
	// Splitting at the duration representation boundary avoids overflow for very large endpoint delays.
	const maxChunkSeconds = float64(math.MaxInt64 / int64(time.Second))
	for seconds > 0 {
		chunk := min(seconds, maxChunkSeconds)
		timer := time.NewTimer(time.Duration(chunk * float64(time.Second)))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		seconds -= chunk
	}
	return ctx.Err()
}

// fail retains attempt state privately until the client decides whether to retry or return its cause.
func fail(
	kind string,
	cause error,
	response *http.Response,
	attempts int,
) (classify.Response, mo.Option[attemptFailure]) {
	failure := attemptFailure{
		kind:              kind,
		cause:             cause,
		httpStatus:        mo.None[int](),
		attempts:          attempts,
		retryAfterSeconds: mo.None[float64](),
	}
	if response != nil {
		failure.httpStatus = mo.Some(response.StatusCode)
		failure.retryAfterSeconds = parseRetryAfter(response.Header.Get("Retry-After"))
	}
	return classify.Response{}, mo.Some(failure)
}

// parseRetryAfter converts a finite delay or HTTP date to seconds, retaining a supplied zero.
func parseRetryAfter(value string) mo.Option[float64] {
	if value == "" {
		return mo.None[float64]()
	}
	seconds, err := strconv.ParseFloat(value, 64)
	if err == nil && isFinite(seconds) && seconds >= 0 {
		return mo.Some(seconds)
	}
	date, err := http.ParseTime(value)
	if err != nil {
		return mo.None[float64]()
	}
	seconds = max(0, time.Until(date).Seconds())
	return mo.Some(seconds)
}

// providerCause extracts the endpoint's actual message without returning its metadata object.
func providerCause(status int, body []byte) string {
	statusCause := fmt.Sprintf("System One returned HTTP %d", status)
	if len(body) == 0 {
		return statusCause
	}
	if !json.Valid(body) {
		return string(body)
	}
	var reported providerErrorDTO
	if err := json.Unmarshal(body, &reported); err != nil {
		return statusCause
	}
	var text string
	if err := json.Unmarshal(reported.Error, &text); err == nil && text != "" {
		return text
	}
	var cause providerCauseDTO
	if err := json.Unmarshal(reported.Error, &cause); err == nil {
		if message, present := cause.Message.Get(); present && message != "" {
			return message
		}
	}
	if err := json.Unmarshal(reported.Message, &text); err == nil && text != "" {
		return text
	}
	return statusCause
}
