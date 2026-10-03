package appinit

import (
	"fmt"
	"net/http"
	"time"
)

// TestDiagnosticBodiesArePreserved keeps text, JSON, and malformed JSON exactly as returned by the endpoint.
func (s *classificationSuite) TestDiagnosticBodiesArePreserved() {
	cases := []struct {
		// name labels the response format under test.
		name string
		// body is the exact fixture response text expected in diagnostics.
		body string
		// requestID is the provider trace identity supplied by a header or body metadata.
		requestID string
	}{
		{
			name:      "ordinary text",
			body:      "Endpoint failure: private source; fixture credential secret-key.\n",
			requestID: "header-private source-secret-key",
		},
		{name: "full JSON", body: `{
   "id":"provider-private source-secret-key",
   "error":{"code":"upstream","message":"bad \u0073ecret-key and \u0070rivate source"},
   "request":{"state":{"content":"private source","task":"Classify"}},
   "headers":{"Authorization":"Bearer secret-key"},
   "credentials":{"api_key":"secret-key"},
   "source":"private source"
  }`, requestID: "provider-private source-secret-key"},
		{
			name:      "malformed JSON",
			body:      `{"error":"\u0073ecret-key \u0070rivate source"`,
			requestID: "header-private source-secret-key",
		},
	}
	for _, test := range cases {
		s.Run(test.name, func() {
			session := s.connect(func(w http.ResponseWriter, _ *http.Request) {
				if test.name != "full JSON" {
					w.Header().Set("X-Request-ID", test.requestID)
				}
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(http.StatusBadGateway)
				_, err := w.Write([]byte(test.body))
				s.NoError(err)
			}, time.Minute)
			result := s.call(session, validArguments)
			s.Require().True(result.IsError)
			object := s.structured(result)["results"].([]any)[0].(map[string]any)
			s.NotContains(object, "answers")
			s.NotContains(object, "model")
			diagnostic := object["error"].(map[string]any)
			s.Equal("upstream_error", diagnostic["code"])
			s.Equal("classify", diagnostic["operation"])
			s.Equal(test.body, diagnostic["upstream_body"])
			s.Equal(fmt.Sprintf("System One returned HTTP 502: %s", test.body), diagnostic["message"])
			s.Equal(test.requestID, diagnostic["upstream_request_id"])
			s.InDelta(http.StatusBadGateway, diagnostic["http_status"], 0)
			s.InDelta(1, diagnostic["attempts"], 0)
			s.InDelta(0, diagnostic["retry_after_seconds"], 0)
		})
	}
}
