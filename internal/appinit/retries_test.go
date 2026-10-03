package appinit

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// TestOverloadRetries checks selective retries through the MCP entry point and retains final attempt diagnostics.
func (s *classificationSuite) TestOverloadRetries() {
	for _, status := range []int{http.StatusTooManyRequests, 529} {
		s.Run(fmt.Sprint(status), func() {
			var attempts atomic.Int32
			session := s.connect(func(w http.ResponseWriter, _ *http.Request) {
				attempt := attempts.Add(1)
				w.Header().Set("Retry-After", "0")
				w.Header().Set("X-Request-ID", fmt.Sprintf("attempt-%d", attempt))
				w.WriteHeader(status)
				_, err := fmt.Fprintf(w, "overloaded attempt %d\n", attempt)
				s.NoError(err)
			}, time.Minute)
			result := s.call(session, validArguments)
			s.True(result.IsError)
			diagnostic := s.structured(result)["results"].([]any)[0].(map[string]any)["error"].(map[string]any)
			s.Equal(int32(3), attempts.Load())
			s.InDelta(3, diagnostic["attempts"], 0)
			s.InDelta(status, diagnostic["http_status"], 0)
			s.Equal("overloaded attempt 3\n", diagnostic["upstream_body"])
			s.Contains(diagnostic["message"], "overloaded attempt 3\n")
			s.Equal("attempt-3", diagnostic["upstream_request_id"])
			s.InDelta(0, diagnostic["retry_after_seconds"], 0)
		})
	}
}

// TestOverloadThenSuccess returns a complete assessment after one explicit overload response.
func (s *classificationSuite) TestOverloadThenSuccess() {
	var attempts atomic.Int32
	session := s.connect(func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, err := w.Write([]byte("busy"))
			s.NoError(err)
			return
		}
		_, err := w.Write([]byte(validAnswer))
		s.NoError(err)
	}, time.Minute)
	result := s.call(session, validArguments)
	s.False(result.IsError)
	s.Equal(int32(2), attempts.Load())
	s.Equal("ok", s.structured(result)["results"].([]any)[0].(map[string]any)["status"])
}
