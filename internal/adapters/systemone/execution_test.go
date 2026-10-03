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
		done := make(chan error, 3)
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
		diagnostic := <-done
		require.ErrorIs(t, diagnostic, context.Canceled)
		close(release)
		for range 2 {
			require.NoError(t, <-done)
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
				require.NoError(t, diagnostic)
				require.Contains(t, result.Answers, "q")
				require.Equal(t, test.delay, time.Since(start))
			})
		})
	}
}

// TestRetryWaitCancellation retains the final overload cause and stops before another HTTP attempt.
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
		done := make(chan error, 1)
		go func() { _, diagnostic := client.Evaluate(ctx, newModelRequest()); done <- diagnostic }()
		synctest.Wait()
		cancel()
		diagnostic := <-done
		require.Contains(t, diagnostic.Error(), "last overload\n")
		require.ErrorIs(t, diagnostic, context.Canceled)
	})
}

// TestFailuresAreNotRetried covers ambiguous transport outcomes, unrelated statuses, and incompatible answers.
func (s *executionSuite) TestFailuresAreNotRetried() {
	for _, test := range []struct {
		// name identifies the externally observable failure.
		name string
		// status supplies the endpoint HTTP status when a response exists.
		status int
		// body supplies the external HTTP response used to verify the concrete cause.
		body string
		// cause is a concrete ambiguous HTTP transport failure.
		cause error
	}{
		{name: "network", status: 0, body: "", cause: errors.New("ambiguous connection failure")},
		{name: "timeout", status: 0, body: "", cause: context.DeadlineExceeded},
		{name: "unrelated status", status: 503, body: "unavailable", cause: nil},
		{name: "incompatible", status: 200, body: `{"model":"actual","answers":{}}`, cause: nil},
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
			diagnostic := failure
			if test.cause != nil {
				s.ErrorIs(diagnostic, test.cause)
			} else {
				s.NotEmpty(diagnostic.Error())
			}
		})
	}
}
