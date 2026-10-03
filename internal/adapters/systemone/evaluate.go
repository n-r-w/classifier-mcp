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

	"github.com/n-r-w/classifier-mcp/internal/domain"
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
) (classify.Response, mo.Option[domain.Diagnostic]) {
	if err := ctx.Err(); err != nil {
		return cancelFailure(mo.None[domain.Diagnostic](), err)
	}
	body, err := json.Marshal(mapRequest(request, c.model))
	if err != nil {
		return fail("request_failed", fmt.Sprintf("encode System One request: %v", err), nil, nil, 0)
	}
	last := mo.None[domain.Diagnostic]()
	for attempt := 1; ; attempt++ {
		if err = c.acquire(ctx); err != nil {
			return cancelFailure(last, err)
		}
		result, diagnostic := c.attempt(ctx, request, body, attempt)
		<-c.capacity
		detail, failed := diagnostic.Get()
		if !failed {
			return result, diagnostic
		}
		if detail.Code == canceledCode && detail.Attempts.OrEmpty() == attempt-1 {
			return cancelFailure(last, ctx.Err())
		}
		if detail.Code != "upstream_error" ||
			(detail.HTTPStatus.OrEmpty() != http.StatusTooManyRequests && detail.HTTPStatus.OrEmpty() != 529) ||
			attempt == c.maxAttempts {
			return result, diagnostic
		}
		last = diagnostic
		delay := detail.RetryAfterSeconds.OrElse(c.retryDelay.Seconds())
		if err = waitRetry(ctx, delay); err != nil {
			return cancelFailure(last, err)
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
) (classify.Response, mo.Option[domain.Diagnostic]) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return fail("request_failed", fmt.Sprintf("create System One request: %v", err), nil, nil, attempt-1)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	if err = ctx.Err(); err != nil {
		return fail(canceledCode, err.Error(), nil, nil, attempt-1)
	}
	response, err := c.http.Do(req)
	if err != nil {
		return fail(
			resolveRequestFailureCode(ctx),
			fmt.Sprintf("System One HTTP request failed: %v", err),
			nil,
			nil,
			attempt,
		)
	}
	defer func() { _ = response.Body.Close() }()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return fail(
			resolveRequestFailureCode(ctx),
			fmt.Sprintf("read System One response: %v", err),
			response,
			data,
			attempt,
		)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fail("upstream_error", fmt.Sprintf("System One returned HTTP %d: %s", response.StatusCode, data),
			response, data, attempt)
	}
	parsed, err := decodeResponse(data, request)
	if err != nil {
		failed, diagnostic := fail("invalid_response", fmt.Sprintf("invalid System One response: %v", err),
			response, data, attempt)
		failed.Usage = parsed.Usage
		return failed, diagnostic
	}
	return parsed, mo.None[domain.Diagnostic]()
}

// resolveRequestFailureCode distinguishes caller cancellation from ambiguous transport failures and client timeouts.
func resolveRequestFailureCode(ctx context.Context) string {
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return canceledCode
	}
	return "request_failed"
}

// cancelFailure retains the last overload diagnostics when cancellation interrupts capacity or retry waits.
func cancelFailure(last mo.Option[domain.Diagnostic], cause error) (classify.Response, mo.Option[domain.Diagnostic]) {
	diagnostic, present := last.Get()
	if !present {
		_, initial := fail(canceledCode, cause.Error(), nil, nil, 0)
		return classify.Response{}, initial
	}
	diagnostic.Code = canceledCode
	diagnostic.Message += ": " + cause.Error()
	return classify.Response{}, mo.Some(diagnostic)
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

// fail retains the endpoint body and concrete cause with the actual HTTP attempt count.
func fail(
	code, message string,
	response *http.Response,
	body []byte,
	attempts int,
) (classify.Response, mo.Option[domain.Diagnostic]) {
	diagnostic := domain.Diagnostic{
		Code:              code,
		Operation:         classifyOperation,
		Message:           message,
		HTTPStatus:        mo.None[int](),
		UpstreamBody:      mo.EmptyableToOption(string(body)),
		UpstreamRequestID: mo.None[string](),
		Attempts:          mo.EmptyableToOption(attempts),
		RetryAfterSeconds: mo.None[float64](),
	}
	if response != nil {
		diagnostic.HTTPStatus = mo.Some(response.StatusCode)
		diagnostic.UpstreamRequestID = mo.EmptyableToOption(
			response.Header.Get("X-Request-ID"),
		)
		if diagnostic.UpstreamRequestID.IsNone() {
			diagnostic.UpstreamRequestID = mo.EmptyableToOption(
				response.Header.Get("X-OpenRouter-Request-ID"),
			)
		}
		if diagnostic.UpstreamRequestID.IsNone() {
			var metadata responseMetadataDTO
			if err := json.Unmarshal(body, &metadata); err == nil {
				diagnostic.UpstreamRequestID = mo.EmptyableToOption(metadata.ID.OrEmpty())
			}
		}
		diagnostic.RetryAfterSeconds = parseRetryAfter(response.Header.Get("Retry-After"))
	}
	return classify.Response{}, mo.Some(diagnostic)
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
