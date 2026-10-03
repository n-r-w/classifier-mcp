package systemone

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// Evaluate validates external answers before producing an atomic outcome.
func (c *Client) Evaluate(
	ctx context.Context,
	request classify.Request,
) (classify.Response, mo.Option[domain.Diagnostic]) {
	if err := ctx.Err(); err != nil {
		diagnostic := domain.Diagnostic{
			Code:              canceledCode,
			Operation:         classifyOperation,
			Message:           err.Error(),
			HTTPStatus:        mo.None[int](),
			UpstreamBody:      mo.None[string](),
			UpstreamRequestID: mo.None[string](),
			Attempts:          mo.None[int](),
			RetryAfterSeconds: mo.None[float64](),
		}
		return classify.Response{}, mo.Some(diagnostic)
	}
	body, err := json.Marshal(mapRequest(request, c.model))
	if err != nil {
		return fail("request_failed", fmt.Sprintf("encode System One request: %v", err), nil, nil)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return fail("request_failed", fmt.Sprintf("create System One request: %v", err), nil, nil)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	response, err := c.http.Do(req)
	if err != nil {
		code := "request_failed"
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			code = canceledCode
		}
		return fail(code, fmt.Sprintf("System One HTTP request failed: %v", err), nil, nil)
	}
	defer func() { _ = response.Body.Close() }()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return fail("request_failed", fmt.Sprintf("read System One response: %v", err), response, data)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fail(
			"upstream_error",
			fmt.Sprintf("System One returned HTTP %d: %s", response.StatusCode, data),
			response,
			data,
		)
	}
	parsed, err := decodeResponse(data, request)
	if err != nil {
		failed, diagnostic := fail(
			"invalid_response",
			fmt.Sprintf("invalid System One response: %v", err),
			response,
			data,
		)
		failed.Usage = parsed.Usage
		return failed, diagnostic
	}
	return parsed, mo.None[domain.Diagnostic]()
}

// fail retains the endpoint body and concrete cause with metadata for one HTTP attempt.
func fail(
	code, message string,
	response *http.Response,
	body []byte,
) (classify.Response, mo.Option[domain.Diagnostic]) {
	diagnostic := domain.Diagnostic{
		Code:              code,
		Operation:         classifyOperation,
		Message:           message,
		HTTPStatus:        mo.None[int](),
		UpstreamBody:      mo.EmptyableToOption(string(body)),
		UpstreamRequestID: mo.None[string](),
		Attempts:          mo.Some(1),
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
