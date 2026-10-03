package appinit

import (
	"encoding/json"
	"net/http"
	"time"
)

// TestScoreDistributionRequirement checks that incomplete full output becomes an atomic object error.
func (s *classificationSuite) TestScoreDistributionRequirement() {
	var body map[string]any
	s.Require().NoError(json.Unmarshal([]byte(validAnswer), &body))
	delete(body["answers"].(map[string]any)["urgency"].(map[string]any), "probabilities")
	data, err := json.Marshal(body)
	s.Require().NoError(err)
	session := s.connect(
		func(w http.ResponseWriter, _ *http.Request) { _, writeErr := w.Write(data); s.NoError(writeErr) },
		time.Minute,
	)
	compact := s.call(session, validArguments)
	s.Require().False(compact.IsError)
	s.structured(compact)
	var input map[string]any
	s.Require().NoError(json.Unmarshal([]byte(validArguments), &input))
	input["result_mode"] = "full"
	arguments, err := json.Marshal(input)
	s.Require().NoError(err)
	full := s.call(session, string(arguments))
	s.Require().False(full.IsError)
	cause := s.structured(full)["results"].([]any)[0].(map[string]any)["error"].(string)
	s.Contains(cause, "probabilities")
}
