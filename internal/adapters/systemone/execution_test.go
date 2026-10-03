package systemone

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/samber/mo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/n-r-w/classifier-mcp/internal/domain"
	"github.com/n-r-w/classifier-mcp/internal/usecases/classify"
)

// executionSuite checks deterministic capacity, retry timing, and cancellation at the HTTP boundary.
type executionSuite struct {
	// Suite supplies assertions and lifecycle state for isolated HTTP execution cases.
	suite.Suite
}

// TestExecution runs independent client execution cases.
func TestExecution(t *testing.T) { t.Parallel(); suite.Run(t, new(executionSuite)) }

// newModelRequest supplies a complete Noul definition for each synthetic HTTP response.
func newModelRequest() classify.Request {
	return classify.Request{
		Content: "private source",
		Source:  mo.None[domain.FileSource](),
		Task:    "task",
		Full:    false,
		Questions: map[string]domain.Question{
			"q": mo.NewEither3Arg2[domain.ChoiceQuestion, domain.NoulQuestion, domain.ScoreQuestion](
				domain.NoulQuestion{Instructions: "condition", Criteria: mo.None[map[string]any]()}),
		},
	}
}

// modelAnswer is a compatible response used to detect successful retry completion.
const modelAnswer = `{"model":"actual","answers":{"q":{"type":"noul","noul":0}}}`

// newHTTPResponse creates an external response with exact body and retry metadata.
func newHTTPResponse(request *http.Request, status int, body, retry string) *http.Response {
	header := make(http.Header)
	if retry != "" {
		header.Set("Retry-After", retry)
	}
	return &http.Response{
		Status:           "",
		StatusCode:       status,
		Proto:            "HTTP/1.1",
		ProtoMajor:       1,
		ProtoMinor:       1,
		Header:           header,
		Body:             io.NopCloser(strings.NewReader(body)),
		ContentLength:    int64(len(body)),
		TransferEncoding: nil,
		Close:            false,
		Uncompressed:     false,
		Trailer:          nil,
		Request:          request,
		TLS:              nil,
	}
}

// TestSharedCapacityAndQueuedCancellation stops a queued call without starting its HTTP request.
func (s *executionSuite) TestSharedCapacityAndQueuedCancellation() {
	synctest.Test(s.T(), func(t *testing.T) {
		transport := NewMockRoundTripper(gomock.NewController(t))
		release := make(chan struct{})
		var started atomic.Int32
		transport.EXPECT().RoundTrip(gomock.Any()).DoAndReturn(func(request *http.Request) (*http.Response, error) {
			started.Add(1)
			select {
			case <-release:
				return newHTTPResponse(request, http.StatusOK, modelAnswer, ""), nil
			case <-request.Context().Done():
				return nil, request.Context().Err()
			}
		}).AnyTimes()
		client := New(&http.Client{Transport: transport, CheckRedirect: nil, Jar: nil, Timeout: time.Minute},
			"http://fixture/systemone", "configured", "", 2, 3, time.Second)
		done := make(chan mo.Option[domain.Diagnostic], 3)
		for range 2 {
			go func() { _, diagnostic := client.Evaluate(t.Context(), newModelRequest()); done <- diagnostic }()
		}
		synctest.Wait()
		assert.Equal(t, int32(2), started.Load())
		ctx, cancel := context.WithCancel(t.Context())
		go func() { _, diagnostic := client.Evaluate(ctx, newModelRequest()); done <- diagnostic }()
		synctest.Wait()
		assert.Equal(t, int32(2), started.Load())
		cancel()
		synctest.Wait()
		diagnostic := (<-done).OrEmpty()
		require.Equal(t, "canceled", diagnostic.Code)
		assert.True(t, diagnostic.Attempts.IsNone())
		close(release)
		for range 2 {
			require.True(t, (<-done).IsNone())
		}
	})
}

// TestRetryDelays observes the supplied delay, HTTP dates, fallback delay, and explicit zero on a fake clock.
func (s *executionSuite) TestRetryDelays() {
	for _, test := range []struct {
		// name distinguishes the endpoint delay representation.
		name string
		// retry selects the Retry-After response header relative to the fake clock.
		retry func(time.Time) string
		// delay is the expected duration before the second HTTP attempt.
		delay time.Duration
	}{
		{name: "seconds", retry: func(time.Time) string { return "2.5" }, delay: 2500 * time.Millisecond},
		{name: "date", retry: func(now time.Time) string {
			return now.Add(time.Minute).UTC().Format(http.TimeFormat)
		}, delay: time.Minute},
		{name: "zero", retry: func(time.Time) string { return "0" }, delay: 0},
		{name: "past date", retry: func(now time.Time) string {
			return now.Add(-time.Minute).UTC().Format(http.TimeFormat)
		}, delay: 0},
		{name: "missing", retry: func(time.Time) string { return "" }, delay: time.Second},
		{name: "invalid", retry: func(time.Time) string { return "not a delay" }, delay: time.Second},
	} {
		s.Run(test.name, func() {
			synctest.Test(s.T(), func(t *testing.T) {
				transport := NewMockRoundTripper(gomock.NewController(t))
				start := time.Now()
				attempts := 0
				transport.EXPECT().
					RoundTrip(gomock.Any()).
					DoAndReturn(func(request *http.Request) (*http.Response, error) {
						attempts++
						if attempts == 1 {
							return newHTTPResponse(request, http.StatusTooManyRequests, "busy", test.retry(start)), nil
						}
						require.GreaterOrEqual(t, time.Since(start), test.delay)
						return newHTTPResponse(request, http.StatusOK, modelAnswer, ""), nil
					}).
					Times(2)
				client := New(&http.Client{Transport: transport, CheckRedirect: nil, Jar: nil, Timeout: time.Minute},
					"http://fixture/systemone", "configured", "", 1, 2, time.Second)
				result, diagnostic := client.Evaluate(t.Context(), newModelRequest())
				require.True(t, diagnostic.IsNone())
				require.Equal(t, "actual", result.Model)
				require.Equal(t, test.delay, time.Since(start))
			})
		})
	}
}

// TestRetryWaitCancellation preserves final overload details and does not send another HTTP attempt.
func (s *executionSuite) TestRetryWaitCancellation() {
	synctest.Test(s.T(), func(t *testing.T) {
		transport := NewMockRoundTripper(gomock.NewController(t))
		transport.EXPECT().RoundTrip(gomock.Any()).DoAndReturn(func(request *http.Request) (*http.Response, error) {
			response := newHTTPResponse(request, 529, "last overload\n", "1e30")
			response.Header.Set("X-Request-ID", "final-id")
			return response, nil
		}).Times(1)
		client := New(&http.Client{Transport: transport, CheckRedirect: nil, Jar: nil, Timeout: time.Minute},
			"http://fixture/systemone", "configured", "", 1, 3, time.Second)
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan domain.Diagnostic, 1)
		go func() { _, diagnostic := client.Evaluate(ctx, newModelRequest()); done <- diagnostic.OrEmpty() }()
		synctest.Wait()
		cancel()
		diagnostic := <-done
		require.Equal(t, "canceled", diagnostic.Code)
		require.Equal(t, 1, diagnostic.Attempts.OrEmpty())
		require.Equal(t, 529, diagnostic.HTTPStatus.OrEmpty())
		require.Equal(t, "last overload\n", diagnostic.UpstreamBody.OrEmpty())
		require.Equal(t, "final-id", diagnostic.UpstreamRequestID.OrEmpty())
		require.InDelta(t, 1e30, diagnostic.RetryAfterSeconds.OrEmpty(), 0)
		require.Contains(t, diagnostic.Message, "last overload\n")
		require.Contains(t, diagnostic.Message, context.Canceled.Error())
	})
}

// TestFailuresAreNotRetried covers ambiguous transport outcomes, unrelated statuses, and incompatible answers.
func (s *executionSuite) TestFailuresAreNotRetried() {
	for _, test := range []struct {
		// name identifies the externally observable failure.
		name string
		// status supplies the endpoint HTTP status when a response exists.
		status int
		// body is retained unchanged in response diagnostics.
		body string
		// cause is a concrete ambiguous HTTP transport failure.
		cause error
		// code is the expected object failure category.
		code string
	}{
		{name: "network", status: 0, body: "", cause: errors.New("ambiguous connection failure"), code: "request_failed"},
		{name: "timeout", status: 0, body: "", cause: context.DeadlineExceeded, code: "request_failed"},
		{name: "unrelated status", status: 503, body: "unavailable", cause: nil, code: "upstream_error"},
		{name: "incompatible", status: 200, body: `{"model":"actual","answers":{}}`, cause: nil, code: "invalid_response"},
	} {
		s.Run(test.name, func() {
			transport := NewMockRoundTripper(gomock.NewController(s.T()))
			transport.EXPECT().RoundTrip(gomock.Any()).DoAndReturn(func(request *http.Request) (*http.Response, error) {
				if test.cause != nil {
					return nil, test.cause
				}
				return newHTTPResponse(request, test.status, test.body, "0"), nil
			}).Times(1)
			client := New(&http.Client{Transport: transport, CheckRedirect: nil, Jar: nil, Timeout: time.Minute},
				"http://fixture/systemone", "configured", "", 1, 3, time.Second)
			_, failure := client.Evaluate(s.T().Context(), newModelRequest())
			diagnostic := failure.OrEmpty()
			s.Equal(test.code, diagnostic.Code)
			s.Equal(1, diagnostic.Attempts.OrEmpty())
			if test.cause != nil {
				s.Contains(diagnostic.Message, test.cause.Error())
			} else {
				s.Equal(test.body, diagnostic.UpstreamBody.OrEmpty())
			}
		})
	}
}
