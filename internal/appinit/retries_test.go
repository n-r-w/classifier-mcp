package appinit

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// TestOverloadRetries checks actual attempts and the final concise cause through MCP.
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
			s.Require().False(result.IsError)
			cause := s.structured(result)["results"].([]any)[0].(map[string]any)["error"]
			s.Equal(int32(3), attempts.Load())
			s.Equal("overloaded attempt 3\n", cause)
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
	s.Contains(s.structured(result)["results"].([]any)[0].(map[string]any), "answers")
}
