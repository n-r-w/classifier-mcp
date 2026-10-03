package appinit

import (
	"net/http"
	"time"
)

// TestProviderCausesAreConcise preserves the actual message and omits the service metadata envelope.
func (s *classificationSuite) TestProviderCausesAreConcise() {
	cases := []struct {
		// name distinguishes the provider's error representation.
		name string
		// body is the external error payload, including unused metadata.
		body string
		// cause is the complete actual text message expected for this object.
		cause string
	}{
		{
			name:  "ordinary text",
			body:  "private source failed; fixture credential secret-key.\n",
			cause: "private source failed; fixture credential secret-key.\n",
		},
		{
			name: "error object",
			body: `{
  "error": {
    "message": "bad secret-key and private source",
    "code": 502,
    "metadata": {
      "provider": "fixture"
    }
  },
  "request": {
    "content": "private source"
  }
}`,
			cause: "bad secret-key and private source",
		},
		{name: "error string", body: `{
  "error": "object rejected",
  "provider": "fixture"
}`, cause: "object rejected"},
		{name: "message", body: `{
  "message": "request rejected",
  "provider": "fixture"
}`, cause: "request rejected"},
		{
			name:  "nested cause with unused message object",
			body:  `{"error":{"message":"quota exhausted","code":400},"message":{"unused":"metadata"}}`,
			cause: "quota exhausted",
		},
		{
			name:  "string cause with unused message object",
			body:  `{"error":"quota exhausted","message":{"unused":"metadata"}}`,
			cause: "quota exhausted",
		},
	}
	for _, test := range cases {
		s.Run(test.name, func() {
			session := s.connect(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, err := w.Write([]byte(test.body))
				s.NoError(err)
			}, time.Minute)
			result := s.call(session, validArguments)
			s.Require().False(result.IsError)
			s.Equal(map[string]any{"id": "one", "error": test.cause}, s.structured(result)["results"].([]any)[0])
		})
	}
}
