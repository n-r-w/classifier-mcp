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
	s.Require().True(result.IsError)
	diagnostic := s.structured(result)["results"].([]any)[0].(map[string]any)["error"].(map[string]any)
	s.Equal("upstream_error", diagnostic["code"])
	s.NotContains(diagnostic, "retry_after_seconds")
}

// TestZeroRetryDelayRetainsReportedMetadata checks zero delay without retrying the failed HTTP request.
func (s *classificationSuite) TestZeroRetryDelayRetainsReportedMetadata() {
	var calls atomic.Int64
	session := s.connect(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		_, err := w.Write([]byte(`{"error":{"message":"rate limit"}}`))
		s.NoError(err)
	}, time.Minute)
	result := s.call(session, validArguments)
	s.Require().True(result.IsError)
	diagnostic := s.structured(result)["results"].([]any)[0].(map[string]any)["error"].(map[string]any)
	s.InDelta(0, diagnostic["retry_after_seconds"], 0)
	s.InDelta(1, diagnostic["attempts"], 0)
	s.Equal(int64(1), calls.Load())
}
