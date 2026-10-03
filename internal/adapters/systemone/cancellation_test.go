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
)

// TestResponseBodyCancellation returns the concrete read cancellation cause without another HTTP attempt.
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
		done := make(chan error, 1)
		go func() { _, failure := client.Evaluate(ctx, newModelRequest()); done <- failure }()
		<-written
		synctest.Wait()
		cancel()
		diagnostic := <-done
		require.ErrorIs(t, diagnostic, context.Canceled)
	})
}

// TestCancellationBeforeRetryHTTPKeepsLastOverload retains the last cause when cancellation races with capacity.
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
	diagnostic := failure
	s.Contains(diagnostic.Error(), "last overload\n")
	s.ErrorIs(diagnostic, context.Canceled)
}
