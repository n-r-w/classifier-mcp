package systemone

import (
	"context"
	"net/http"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/n-r-w/classifier-mcp/internal/domain"
)

// TestExhaustedAttemptsKeepFinalDelay retains the final response and stops before waiting its supplied delay.
func (s *executionSuite) TestExhaustedAttemptsKeepFinalDelay() {
	synctest.Test(s.T(), func(t *testing.T) {
		transport := NewMockRoundTripper(gomock.NewController(t))
		attempts := 0
		transport.EXPECT().RoundTrip(gomock.Any()).DoAndReturn(func(request *http.Request) (*http.Response, error) {
			attempts++
			if attempts == 1 {
				return newHTTPResponse(request, http.StatusTooManyRequests, "first overload", "0"), nil
			}
			return newHTTPResponse(request, 529, "final overload", "7"), nil
		}).Times(2)
		client := New(&http.Client{Transport: transport, CheckRedirect: nil, Jar: nil, Timeout: time.Minute},
			"http://fixture/systemone", "configured", "", 1, 2, time.Second)
		start := time.Now()
		_, failure := client.Evaluate(t.Context(), newModelRequest())
		diagnostic := failure.OrEmpty()
		require.Equal(t, "upstream_error", diagnostic.Code)
		require.Equal(t, 2, diagnostic.Attempts.OrEmpty())
		require.Equal(t, 529, diagnostic.HTTPStatus.OrEmpty())
		require.Equal(t, "final overload", diagnostic.UpstreamBody.OrEmpty())
		require.InDelta(t, 7, diagnostic.RetryAfterSeconds.OrEmpty(), 0)
		require.Zero(t, time.Since(start))
	})
}

// TestRetryWaitReleasesCapacity lets another call complete while an overloaded call waits and is canceled.
func (s *executionSuite) TestRetryWaitReleasesCapacity() {
	synctest.Test(s.T(), func(t *testing.T) {
		transport := NewMockRoundTripper(gomock.NewController(t))
		attempts := 0
		transport.EXPECT().RoundTrip(gomock.Any()).DoAndReturn(func(request *http.Request) (*http.Response, error) {
			attempts++
			if attempts == 1 {
				return newHTTPResponse(request, http.StatusTooManyRequests, "waiting overload", "60"), nil
			}
			return newHTTPResponse(request, http.StatusOK, modelAnswer, ""), nil
		}).Times(2)
		client := New(&http.Client{Transport: transport, CheckRedirect: nil, Jar: nil, Timeout: time.Minute},
			"http://fixture/systemone", "configured", "", 1, 2, time.Second)
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan domain.Diagnostic, 1)
		go func() { _, failure := client.Evaluate(ctx, newModelRequest()); done <- failure.OrEmpty() }()
		synctest.Wait()
		start := time.Now()
		_, second := client.Evaluate(t.Context(), newModelRequest())
		require.True(t, second.IsNone())
		require.Zero(t, time.Since(start))
		cancel()
		first := <-done
		require.Equal(t, "canceled", first.Code)
		require.Equal(t, 1, first.Attempts.OrEmpty())
		require.Equal(t, "waiting overload", first.UpstreamBody.OrEmpty())
	})
}
