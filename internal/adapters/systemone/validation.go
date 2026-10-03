package systemone

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/samber/mo"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/n-r-w/classifier-mcp/internal/domain"
	"github.com/n-r-w/classifier-mcp/internal/usecases/classify"
)

// decodeResponse requires an actual model identity and an atomic answer set for all questions.
func decodeResponse(data []byte, request classify.Request) (classify.Response, error) {
	var dto responseDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		return classify.Response{}, fmt.Errorf("decode body: %w", err)
	}
	usage, err := convertUsage(dto.Usage.OrEmpty())
	if err != nil {
		return classify.Response{}, err
	}
	result := classify.Response{Model: dto.Model, Answers: nil, Usage: mo.TupleToOption(usage, usage.HasValues())}
	if strings.TrimSpace(dto.Model) == "" {
		return result, errors.New("model is missing or blank")
	}
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

// convertUsage rejects invalid billing values and preserves each provider field's presence.
func convertUsage(u usageDTO) (domain.Usage, error) {
	if value, present := u.InputTokens.Get(); present && value < 0 {
		return domain.Usage{}, errors.New("usage.input_tokens must be non-negative")
	}
	if value, present := u.OutputTokens.Get(); present && value < 0 {
		return domain.Usage{}, errors.New("usage.output_tokens must be non-negative")
	}
	if value, present := u.Cost.Get(); present && (!isFinite(value) || value < 0) {
		return domain.Usage{}, errors.New("usage.cost must be finite and non-negative")
	}
	return domain.Usage{InputTokens: u.InputTokens, OutputTokens: u.OutputTokens, Cost: u.Cost}, nil
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
	selected := probabilities[selection]
	// A one-millionth tolerance accepts rounded ties without selecting a replacement category.
	const roundingTolerance = 0.000001
	for _, p := range probabilities {
		if p > selected+roundingTolerance {
			return domain.Assessment{}, errors.New("choice is not a highest-probability category")
		}
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

// convertScore validates the weighted assessment and derives interpretation data from the caller's scale.
func convertScore(a answerDTO, q domain.ScoreQuestion, full bool) (domain.Assessment, error) {
	value, present := a.Score.Get()
	if a.Type != q.Kind() || !present {
		return domain.Assessment{}, errors.New("score answer has missing or incompatible fields")
	}
	if !isFinite(value) || value < 0 || value > float64(len(q.Criteria)-1) {
		return domain.Assessment{}, errors.New("score is outside the supplied scale")
	}
	keys := make([]string, len(q.Criteria))
	legend := make(map[string]any, len(q.Criteria))
	for i, description := range q.Criteria {
		key := strconv.Itoa(i)
		keys[i] = key
		legend[key] = description
	}
	if full || a.Probabilities.IsSome() {
		probabilities := a.Probabilities.OrEmpty()
		if err := validateDistribution(probabilities, keys); err != nil {
			return domain.Assessment{}, err
		}
		if err := validateWeightedScore(value, probabilities, keys); err != nil {
			return domain.Assessment{}, err
		}
	}
	if raw, reported := a.Legend.Get(); reported {
		var actual map[string]any
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if err := decoder.Decode(&actual); err != nil {
			return domain.Assessment{}, fmt.Errorf("decode returned legend: %w", err)
		}
		if err := validateLegend(actual, legend); err != nil {
			return domain.Assessment{}, err
		}
	}
	return mo.NewEither3Arg3[domain.Choice, domain.Noul, domain.Score](
		domain.Score{Value: value, Probabilities: a.Probabilities, Legend: legend, Confidence: a.Confidence},
	), nil
}

// validateDistribution requires exactly the caller's keys and a normalized finite distribution.
func validateDistribution(values map[string]float64, keys []string) error {
	if len(values) != len(keys) {
		return errors.New("probabilities must contain exactly the supplied criteria keys")
	}
	sum := 0.0
	for _, key := range keys {
		value, exists := values[key]
		if !exists || !isProbability(value) {
			return fmt.Errorf("probabilities[%q] must be numeric and in [0, 1]", key)
		}
		sum += value
	}
	// Allow provider rounding while preserving the original probabilities rather than renormalizing.
	const roundingTolerance = 0.0001
	if math.Abs(sum-1) > roundingTolerance {
		return errors.New("probabilities must sum to 1 within provider rounding tolerance")
	}
	return nil
}

// validateWeightedScore allows numeric rounding proportional to the caller's scale span.
func validateWeightedScore(score float64, probabilities map[string]float64, keys []string) error {
	weighted := 0.0
	for i, key := range keys {
		weighted += float64(i) * probabilities[key]
	}
	const roundingTolerance = 0.0001
	if math.Abs(weighted-score) > roundingTolerance*float64(max(1, len(keys)-1)) {
		return errors.New("score is incompatible with its probability distribution")
	}
	return nil
}

// validateAnswerFields rejects extra variant fields and null distribution or legend values.
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
		allowed["legend"] = true
	}
	for field := range fields {
		if !allowed[field] {
			return fmt.Errorf("incompatible answer field %q", field)
		}
	}
	if rawLegend, exists := fields["legend"]; exists && bytes.Equal(bytes.TrimSpace(rawLegend), []byte("null")) {
		return errors.New("legend must be an object when reported")
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

// validateLegend compares structured JSON values and exact numbers, without rounding identifiers.
func validateLegend(actualLegend, expectedLegend map[string]any) error {
	compiler := jsonschema.NewCompiler()
	const location = "urn:classifier-mcp:score-legend"
	if err := compiler.AddResource(location, map[string]any{"const": expectedLegend}); err != nil {
		return fmt.Errorf("register legend comparison: %w", err)
	}
	schema, err := compiler.Compile(location)
	if err != nil {
		return fmt.Errorf("compile legend comparison: %w", err)
	}
	// JSON numeric spelling can differ (1 and 1.0), but the values must match exactly.
	if validationErr := schema.Validate(actualLegend); validationErr != nil {
		return fmt.Errorf("legend does not match the supplied criteria: %w", validationErr)
	}
	return nil
}
