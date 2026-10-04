package appinit

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// TestTruthProtocolProjection checks truth guidance, content, and numeric bounds through MCP and HTTP.
func (s *classificationSuite) TestTruthProtocolProjection() {
	for _, mode := range []string{"compact", "full"} {
		for _, probability := range []float64{0, 0.8, 1, -0.1, 1.1} {
			s.Run(mode+"/"+strconv.FormatFloat(probability, 'f', -1, 64), func() {
				instructions := map[string]any{
					"condition": "Does the text report a defect?",
					"reference": json.Number("9007199254740993"),
				}
				question := map[string]any{"type": "truth", "instructions": instructions}
				if mode == "full" {
					question["criteria"] = map[string]any{
						"true":  map[string]any{"examples": []any{"A defect"}},
						"false": []any{"A request"},
					}
				}
				const content = "Le paiement échoue. 支払いに失敗しました。"
				const task = "Identify defect reports."
				session := s.connect(func(w http.ResponseWriter, r *http.Request) {
					var request map[string]any
					decoder := json.NewDecoder(r.Body)
					decoder.UseNumber()
					if !s.NoError(decoder.Decode(&request)) {
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					s.Equal(map[string]any{"task": task, "content": content}, request["state"])
					expected := map[string]any{"type": "noul", "instructions": instructions}
					if criteria, present := question["criteria"]; present {
						expected["criteria"] = criteria
					}
					s.Equal(map[string]any{"q": expected}, request["questions"])
					body, err := json.Marshal(
						map[string]any{
							"answers": map[string]any{"q": map[string]any{"type": "noul", "noul": probability}},
						},
					)
					s.NoError(err)
					_, err = w.Write(body)
					s.NoError(err)
				}, time.Minute)
				arguments, err := json.Marshal(map[string]any{
					"objects": []any{
						map[string]any{"id": "note", "source": map[string]any{"type": "text", "text": content}},
					},
					"task":        task,
					"questions":   map[string]any{"q": question},
					"result_mode": mode,
				})
				s.Require().NoError(err)
				result := s.call(session, string(arguments))
				s.Require().False(result.IsError)
				object := s.structured(result)["results"].([]any)[0].(map[string]any)
				if probability < 0 || probability > 1 {
					s.Contains(object, "error")
					s.NotContains(object, "answers")
				} else {
					s.Equal(
						map[string]any{
							"id":      "note",
							"answers": map[string]any{"q": map[string]any{"truth": probability}},
						},
						object,
					)
				}
			})
		}
	}
}
