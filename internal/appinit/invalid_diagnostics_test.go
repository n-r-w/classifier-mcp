package appinit

import (
	"net/http"
	"sync/atomic"
	"time"
)

// TestNonFiniteRetryDelayKeepsObjectEnvelope rejects unrepresentable metadata without losing the object error.
func (s *classificationSuite) TestNonFiniteRetryDelayKeepsObjectEnvelope() {
	session := s.connect(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "+Inf")
		w.WriteHeader(http.StatusTooManyRequests)
		_, err := w.Write([]byte(`{"error":{"message":"overloaded"}}`))
		s.NoError(err)
	}, time.Minute)
	result := s.call(session, validArguments)
	s.Require().False(result.IsError)
	s.Equal("overloaded", s.structured(result)["results"].([]any)[0].(map[string]any)["error"])
}

// TestZeroRetryDelayUsesConfiguredAttempts checks actual retries with a supplied immediate delay.
func (s *classificationSuite) TestZeroRetryDelayUsesConfiguredAttempts() {
	var calls atomic.Int64
	session := s.connect(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		_, err := w.Write([]byte(`{"error":{"message":"rate limit"}}`))
		s.NoError(err)
	}, time.Minute)
	result := s.call(session, validArguments)
	s.Require().False(result.IsError)
	s.Equal("rate limit", s.structured(result)["results"].([]any)[0].(map[string]any)["error"])
	s.Equal(int64(3), calls.Load())
}
