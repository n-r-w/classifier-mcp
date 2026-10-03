package appinit

import (
	"encoding/json"
	"net/http"
	"time"
)

// TestProviderScoreValues preserves the endpoint's exact assessment in both requested projections.
func (s *classificationSuite) TestProviderScoreValues() {
	const response = `{
  "model": "actual",
  "answers": {
    "team": {
      "type": "choice",
      "choice": "a",
      "probabilities": {
        "a": 0.5,
        "b": 0.5
      },
      "confidence": 0
    },
    "condition": {
      "type": "noul",
      "noul": 0
    },
    "urgency": {
      "type": "score",
      "score": 1.97,
      "probabilities": {
        "0": 0,
        "1": 0.02,
        "2": 0.98
      },
      "confidence": 0.96
    }
  }
}`
	session := s.connect(
		func(w http.ResponseWriter, _ *http.Request) { _, err := w.Write([]byte(response)); s.NoError(err) },
		time.Minute,
	)
	for _, mode := range []string{"compact", "full"} {
		s.Run(mode, func() {
			var input map[string]any
			s.Require().NoError(json.Unmarshal([]byte(validArguments), &input))
			input["result_mode"] = mode
			arguments, err := json.Marshal(input)
			s.Require().NoError(err)
			result := s.call(session, string(arguments))
			s.Require().False(result.IsError)
			entries := s.structured(result)["results"].([]any)
			s.Require().Len(entries, 1)
			object := entries[0].(map[string]any)
			s.Require().Len(object, 2)
			s.Equal("one", object["id"])
			answers := object["answers"].(map[string]any)
			expectedScore := map[string]any{"score": 1.97, "confidence": 0.96}
			expectedChoice := map[string]any{"choice": "a", "probability": 0.5, "confidence": float64(0)}
			if mode == "full" {
				expectedScore["probabilities"] = map[string]any{"0": float64(0), "1": 0.02, "2": 0.98}
				expectedChoice["probabilities"] = map[string]any{"a": 0.5, "b": 0.5}
			}
			s.Equal(expectedScore, answers["urgency"])
			s.Equal(expectedChoice, answers["team"])
			s.Equal(map[string]any{"noul": float64(0)}, answers["condition"])
		})
	}
}

// TestIndependentFailuresAreNormalBatchResults returns only concise causes even when every object fails.
func (s *classificationSuite) TestIndependentFailuresAreNormalBatchResults() {
	session := s.connect(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if !s.NoError(json.NewDecoder(r.Body).Decode(&request)) {
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		if request["state"].(map[string]any)["content"] == "first" {
			_, err := w.Write(
				[]byte(
					`{
  "error": {
    "message": "first object rejected",
    "code": 400,
    "metadata": {
      "provider": "fixture",
      "details": "verbose provider dump"
    }
  }
}`,
				),
			)
			s.NoError(err)
		} else {
			_, err := w.Write([]byte("second object rejected"))
			s.NoError(err)
		}
	}, time.Minute)
	arguments := `{
  "objects": [
    {
      "id": "first",
      "source": {
        "type": "text",
        "text": "first"
      }
    },
    {
      "id": "second",
      "source": {
        "type": "text",
        "text": "second"
      }
    }
  ],
  "task": "task",
  "questions": {
    "q": {
      "type": "noul",
      "instructions": "condition"
    }
  }
}`
	result := s.call(session, arguments)
	s.Require().False(result.IsError)
	s.Equal(map[string]any{"results": []any{
		map[string]any{"id": "first", "error": "first object rejected"},
		map[string]any{"id": "second", "error": "second object rejected"},
	}}, s.structured(result))
}

// TestUnusedProviderMetadataDoesNotRejectAnswers accepts useful values despite unused metadata shapes.
func (s *classificationSuite) TestUnusedProviderMetadataDoesNotRejectAnswers() {
	response := `{
  "model": {
    "unused": true
  },
  "usage": {
    "cost": -1,
    "input_tokens": "unused"
  },
  "answers": {
    "team": {
      "type": "choice",
      "choice": "a",
      "probabilities": {
        "a": 0.1,
        "b": 0.6
      }
    },
    "condition": {
      "type": "noul",
      "noul": 0
    },
    "urgency": {
      "type": "score",
      "score": 1.97,
      "probabilities": {
        "0": 0,
        "1": 0.02,
        "2": 0.98
      },
      "legend": "unused"
    }
  }
}`
	session := s.connect(
		func(w http.ResponseWriter, _ *http.Request) { _, err := w.Write([]byte(response)); s.NoError(err) },
		time.Minute,
	)
	result := s.call(session, validArguments)
	s.Require().False(result.IsError)
	object := s.structured(result)["results"].([]any)[0].(map[string]any)
	s.Require().Contains(object, "answers")
	answers := object["answers"].(map[string]any)
	s.Equal(map[string]any{"choice": "a", "probability": 0.1}, answers["team"])
	s.Equal(map[string]any{"score": 1.97}, answers["urgency"])
}
