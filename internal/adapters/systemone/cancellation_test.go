package systemone

import (
	"context"
	"io"
	"net/http"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.uber.org/mock/gomock"

	"github.com/n-r-w/classifier-mcp/internal/domain"
)

// TestResponseBodyCancellation retains the received status and partial body without another HTTP attempt.
func (s *executionSuite) TestResponseBodyCancellation() {
	synctest.Test(s.T(), func(t *testing.T) {
		transport := NewMockRoundTripper(gomock.NewController(t))
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		reader, writer := io.Pipe()
		written := make(chan struct{})
		transport.EXPECT().RoundTrip(gomock.Any()).DoAndReturn(func(request *http.Request) (*http.Response, error) {
			response := newHTTPResponse(request, http.StatusOK, "", "")
			response.Body = reader
			go func() {
				_, err := writer.Write([]byte("partial response"))
				assert.NoError(t, err)
				close(written)
				<-request.Context().Done()
				assert.NoError(t, writer.CloseWithError(request.Context().Err()))
			}()
			return response, nil
		}).Times(1)
		client := New(&http.Client{Transport: transport, CheckRedirect: nil, Jar: nil, Timeout: time.Minute},
			"http://fixture/systemone", "configured", "", 1, 3, time.Second)
		done := make(chan domain.Diagnostic, 1)
		go func() { _, failure := client.Evaluate(ctx, newModelRequest()); done <- failure.OrEmpty() }()
		<-written
		synctest.Wait()
		cancel()
		diagnostic := <-done
		require.Equal(t, "canceled", diagnostic.Code)
		require.Equal(t, 1, diagnostic.Attempts.OrEmpty())
		require.Equal(t, http.StatusOK, diagnostic.HTTPStatus.OrEmpty())
		require.Equal(t, "partial response", diagnostic.UpstreamBody.OrEmpty())
		require.Contains(t, diagnostic.Message, context.Canceled.Error())
	})
}

// TestCancellationBeforeRetryHTTPKeepsLastOverload preserves evidence when cancellation races with reacquired capacity.
func (s *executionSuite) TestCancellationBeforeRetryHTTPKeepsLastOverload() {
	ctrl := gomock.NewController(s.T())
	transport := NewMockRoundTripper(ctrl)
	original, cancel := context.WithCancel(s.T().Context())
	defer cancel()
	ctx := NewMockContext(ctrl)
	overloaded := false
	checksAfterOverload := 0
	ctx.EXPECT().Done().DoAndReturn(original.Done).AnyTimes()
	ctx.EXPECT().Deadline().DoAndReturn(original.Deadline).AnyTimes()
	ctx.EXPECT().Value(gomock.Any()).DoAndReturn(original.Value).AnyTimes()
	ctx.EXPECT().Err().DoAndReturn(func() error {
		if overloaded {
			checksAfterOverload++
			// The zero-delay wait and both capacity checks finish before cancellation interrupts request preparation.
			if checksAfterOverload == 4 {
				cancel()
			}
		}
		return original.Err()
	}).AnyTimes()
	transport.EXPECT().RoundTrip(gomock.Any()).DoAndReturn(func(request *http.Request) (*http.Response, error) {
		overloaded = true
		response := newHTTPResponse(request, 529, "last overload\n", "0")
		response.Header.Set("X-Request-ID", "final-id")
		return response, nil
	}).Times(1)
	client := New(&http.Client{Transport: transport, CheckRedirect: nil, Jar: nil, Timeout: 0},
		"http://fixture/systemone", "configured", "", 1, 3, time.Second)
	_, failure := client.Evaluate(ctx, newModelRequest())
	diagnostic := failure.OrEmpty()
	s.Equal("canceled", diagnostic.Code)
	s.Equal(1, diagnostic.Attempts.OrEmpty())
	s.Equal(529, diagnostic.HTTPStatus.OrEmpty())
	s.Equal("last overload\n", diagnostic.UpstreamBody.OrEmpty())
	s.Equal("final-id", diagnostic.UpstreamRequestID.OrEmpty())
	s.True(diagnostic.RetryAfterSeconds.IsSome())
	s.InDelta(0, diagnostic.RetryAfterSeconds.OrEmpty(), 0)
	s.Contains(diagnostic.Message, "last overload\n")
	s.Contains(diagnostic.Message, context.Canceled.Error())
}
