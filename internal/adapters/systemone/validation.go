package systemone

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/samber/mo"

	"github.com/n-r-w/classifier-mcp/internal/domain"
	"github.com/n-r-w/classifier-mcp/internal/usecases/classify"
)

// decodeResponse requires a complete compatible answer set for the requested question IDs.
func decodeResponse(data []byte, request classify.Request) (classify.Response, error) {
	var dto responseDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		return classify.Response{}, fmt.Errorf("decode body: %w", err)
	}
	result := classify.Response{Answers: nil}
	if len(dto.Answers) != len(request.Questions) {
		return result, errors.New("answers must contain exactly all requested question IDs")
	}
	answers := make(map[string]domain.Assessment, len(dto.Answers))
	for id, question := range request.Questions {
		raw, exists := dto.Answers[id]
		if !exists {
			return result, fmt.Errorf("answer for question %q is missing", id)
		}
		answer, decodeErr := decodeAnswer(raw, question, request.Full)
		if decodeErr != nil {
			return result, fmt.Errorf("answer %q: %w", id, decodeErr)
		}
		answers[id] = answer
	}
	// Assign only after every answer passes, so one bad assessment cannot expose a partial success.
	result.Answers = answers
	return result, nil
}

// isFinite excludes values that cannot be represented in a JSON result.
func isFinite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

// isProbability checks the inclusive probability interval used by all assessment types.
func isProbability(value float64) bool { return isFinite(value) && value >= 0 && value <= 1 }

// decodeAnswer checks the external shape before converting to the requested assessment variant.
func decodeAnswer(raw []byte, question domain.Question, full bool) (domain.Assessment, error) {
	var answer answerDTO
	if err := json.Unmarshal(raw, &answer); err != nil {
		return domain.Assessment{}, fmt.Errorf("decode assessment: %w", err)
	}
	if err := validateAnswerFields(raw, answer.Type); err != nil {
		return domain.Assessment{}, err
	}
	if value, present := answer.Confidence.Get(); present && !isProbability(value) {
		return domain.Assessment{}, errors.New("confidence must be in [0, 1]")
	}
	if q, present := question.Arg1(); present {
		return convertChoice(answer, q)
	}
	if _, present := question.Arg2(); present {
		return convertNoul(answer)
	}
	if q, present := question.Arg3(); present {
		return convertScore(answer, q, full)
	}
	return domain.Assessment{}, errors.New("unknown question type")
}

// convertChoice requires the category distribution even when only compact output was requested.
func convertChoice(a answerDTO, q domain.ChoiceQuestion) (domain.Assessment, error) {
	selection, present := a.Choice.Get()
	if a.Type != q.Kind() || !present {
		return domain.Assessment{}, errors.New("choice answer has missing or incompatible fields")
	}
	if _, exists := q.Criteria[selection]; !exists {
		return domain.Assessment{}, errors.New("choice is not a supplied category")
	}
	keys := make([]string, 0, len(q.Criteria))
	for key := range q.Criteria {
		keys = append(keys, key)
	}
	probabilities := a.Probabilities.OrEmpty()
	if err := validateDistribution(probabilities, keys); err != nil {
		return domain.Assessment{}, err
	}
	return mo.NewEither3Arg1[domain.Choice, domain.Noul, domain.Score](
		domain.Choice{Selection: selection, Probabilities: probabilities, Confidence: a.Confidence},
	), nil
}

// convertNoul requires one probability and keeps zero as a reported answer.
func convertNoul(a answerDTO) (domain.Assessment, error) {
	value, present := a.Noul.Get()
	if a.Type != domain.NoulKind || !present {
		return domain.Assessment{}, errors.New("noul answer has missing or incompatible fields")
	}
	if !isProbability(value) {
		return domain.Assessment{}, errors.New("noul must be in [0, 1]")
	}
	return mo.NewEither3Arg2[domain.Choice, domain.Noul, domain.Score](domain.Noul{Probability: value}), nil
}

// convertScore preserves the provider assessment after scale bounds and distribution-key checks.
func convertScore(a answerDTO, q domain.ScoreQuestion, full bool) (domain.Assessment, error) {
	value, present := a.Score.Get()
	if a.Type != q.Kind() || !present {
		return domain.Assessment{}, errors.New("score answer has missing or incompatible fields")
	}
	if !isFinite(value) || value < 0 || value > float64(len(q.Criteria)-1) {
		return domain.Assessment{}, errors.New("score is outside the supplied scale")
	}
	keys := make([]string, len(q.Criteria))
	for i := range q.Criteria {
		keys[i] = strconv.Itoa(i)
	}
	if full || a.Probabilities.IsSome() {
		probabilities := a.Probabilities.OrEmpty()
		if err := validateDistribution(probabilities, keys); err != nil {
			return domain.Assessment{}, err
		}
	}

	return mo.NewEither3Arg3[domain.Choice, domain.Noul, domain.Score](
		domain.Score{Value: value, Probabilities: a.Probabilities, Confidence: a.Confidence},
	), nil
}

// validateDistribution requires every expected key and finite probabilities in [0, 1].
func validateDistribution(values map[string]float64, keys []string) error {
	if len(values) != len(keys) {
		return errors.New("probabilities must contain exactly the supplied criteria keys")
	}
	for _, key := range keys {
		value, exists := values[key]
		if !exists || !isProbability(value) {
			return fmt.Errorf("probabilities[%q] must be numeric and in [0, 1]", key)
		}
	}

	return nil
}

// validateAnswerFields checks used assessment fields while leaving unused provider metadata uninterpreted.
func validateAnswerFields(raw []byte, kind string) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return fmt.Errorf("decode answer fields: %w", err)
	}
	allowed := map[string]bool{"type": true}
	switch kind {
	case domain.ChoiceKind:
		allowed["choice"] = true
		allowed["probabilities"] = true
		allowed["confidence"] = true
	case domain.NoulKind:
		allowed["noul"] = true
	case domain.ScoreKind:
		allowed["score"] = true
		allowed["probabilities"] = true
		allowed["confidence"] = true
	}
	used := map[string]bool{
		domain.ChoiceKind: true, domain.NoulKind: true, domain.ScoreKind: true,
		"probabilities": true, "confidence": true,
	}
	for field := range fields {
		if used[field] && !allowed[field] {
			return fmt.Errorf("incompatible answer field %q", field)
		}
	}
	if rawProbabilities, exists := fields["probabilities"]; exists {
		var probabilities map[string]json.RawMessage
		if err := json.Unmarshal(rawProbabilities, &probabilities); err != nil {
			return fmt.Errorf("decode probabilities: %w", err)
		}
		if probabilities == nil {
			return errors.New("probabilities must be an object")
		}
		for key, value := range probabilities {
			if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return fmt.Errorf("probabilities[%q] must be numeric", key)
			}
		}
	}
	return nil
}
