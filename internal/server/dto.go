package server

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/samber/mo"
)

// classifyInput is the schema-validated inline tool request.
type classifyInput struct {
	// Nonempty caller-ordered list with unique case-sensitive identities.
	Objects []objectInput `json:"objects"`
	// Common context whose string must contain non-whitespace text.
	Task string `json:"task"`
	// Nonempty map of independent caller question IDs to their guidance.
	Questions map[string]questionInput `json:"questions"`
	// None selects compact; Some must be compact or full.
	ResultMode mo.Option[string] `json:"result_mode,omitzero"`
}

// objectInput associates one text source with a caller-defined identity.
type objectInput struct {
	// Nonempty identity used to associate the returned outcome with this source.
	ID string `json:"id"`
	// Inline source alternative checked by the published input schema.
	Source textSource `json:"source"`
}

// textSource carries the inline source discriminator and permits an empty text.
type textSource struct {
	// Discriminator fixed to text for the inline source alternative.
	Type string `json:"type"`
	// Caller content retained exactly; an empty string is permitted.
	Text string `json:"text"`
}

// questionInput retains structured guidance until its assessment variant is selected.
type questionInput struct {
	// Assessment discriminator selecting the required rubric shape.
	Type string `json:"type"`
	// Caller string, object, or array; decoded numbers retain their original precision.
	Instructions any `json:"instructions"`
	// Raw rubric JSON; None is allowed only for Noul.
	Criteria mo.Option[json.RawMessage] `json:"criteria,omitzero"`
}

// validate rejects blank string instructions; the schema owns shape and field validation.
func (q questionInput) validate() error {
	if text, ok := q.Instructions.(string); ok && strings.TrimSpace(text) == "" {
		return errors.New("instructions must not be blank")
	}
	return nil
}

// classifyOutput is the ordered list serialized into both MCP result representations.
type classifyOutput struct {
	// Private success/error alternatives in the same order as the input objects.
	Results []any `json:"results"`
}

// successOutput contains a complete answer set and provider-reported model identity.
type successOutput struct {
	// Exact input identity for this completed object.
	ID string `json:"id"`
	// Fixed to ok for the success alternative.
	Status string `json:"status"`
	// Actual response model identifier, without alias substitution.
	Model string `json:"model"`
	// Complete question-ID map containing the selected assessment projections.
	Answers map[string]any `json:"answers"`
	// Omitted when no provider billing values are available; present values may be zero.
	Usage mo.Option[usageOutput] `json:"-"`
}

// MarshalJSON omits absent fields and retains reported zero values as non-null JSON.
func (o successOutput) MarshalJSON() ([]byte, error) {
	fields := map[string]any{fieldID: o.ID, fieldStatus: o.Status, "model": o.Model, "answers": o.Answers}
	addOptional(fields, fieldUsage, o.Usage)
	return json.Marshal(fields)
}

// errorOutput contains one atomic failure and any usage available for that request.
type errorOutput struct {
	// Exact input identity for this failed object.
	ID string `json:"id"`
	// Fixed to error, with no success model or answer fields.
	Status string `json:"status"`
	// Concrete unfiltered cause and available HTTP diagnostics.
	Error diagnosticOutput `json:"error"`
	// Optional provider billing values retained even when this object failed.
	Usage mo.Option[usageOutput] `json:"-"`
}

// MarshalJSON omits absent fields and retains reported zero values as non-null JSON.
func (o errorOutput) MarshalJSON() ([]byte, error) {
	fields := map[string]any{fieldID: o.ID, fieldStatus: o.Status, "error": o.Error}
	addOptional(fields, fieldUsage, o.Usage)
	return json.Marshal(fields)
}

// choiceOutput adds the complete category distribution only for full projection.
type choiceOutput struct {
	// Discriminator fixed to choice.
	Type string `json:"type"`
	// Provider category selection from the caller set.
	Choice string `json:"choice"`
	// Probability of the selected category, copied from its distribution entry.
	Probability float64 `json:"probability"`
	// Optional provider uncertainty in [0, 1]; reported zero is retained.
	Confidence mo.Option[float64] `json:"-"`
	// Present only in full mode, with every caller category key.
	Probabilities mo.Option[map[string]float64] `json:"-"`
}

// MarshalJSON omits absent fields and retains reported zero values as non-null JSON.
func (o choiceOutput) MarshalJSON() ([]byte, error) {
	fields := map[string]any{fieldType: o.Type, "choice": o.Choice, "probability": o.Probability}
	addOptional(fields, fieldConfidence, o.Confidence)
	addOptional(fields, fieldProbabilities, o.Probabilities)
	return json.Marshal(fields)
}

// noulOutput uses the same probability-only shape in both result modes.
type noulOutput struct {
	// Discriminator fixed to noul in both result modes.
	Type string `json:"type"`
	// Probability that the condition holds in [0, 1], rather than a Boolean.
	Noul float64 `json:"noul"`
}

// scoreOutput adds the level distribution and caller legend only for full projection.
type scoreOutput struct {
	// Discriminator fixed to score.
	Type string `json:"type"`
	// Fractional probability-weighted value from zero through the last scale index.
	Score float64 `json:"score"`
	// Optional provider uncertainty in [0, 1]; None omits the field.
	Confidence mo.Option[float64] `json:"-"`
	// Present only in full mode, keyed by decimal level indices.
	Probabilities mo.Option[map[string]float64] `json:"-"`
	// Present only in full mode; caller descriptions retain structured values and numeric precision.
	Legend mo.Option[map[string]any] `json:"-"`
}

// MarshalJSON omits absent fields and retains reported zero values as non-null JSON.
func (o scoreOutput) MarshalJSON() ([]byte, error) {
	fields := map[string]any{fieldType: o.Type, "score": o.Score}
	addOptional(fields, fieldConfidence, o.Confidence)
	addOptional(fields, fieldProbabilities, o.Probabilities)
	addOptional(fields, "legend", o.Legend)
	return json.Marshal(fields)
}

// usageOutput retains provider field presence for explicit omission-aware JSON serialization.
type usageOutput struct {
	// Optional nonnegative input token count for this object evaluation.
	InputTokens mo.Option[int64] `json:"-"`
	// Optional nonnegative generated-answer token count.
	OutputTokens mo.Option[int64] `json:"-"`
	// Optional nonnegative endpoint billing amount; OpenRouter reports credits.
	Cost mo.Option[float64] `json:"-"`
}

// MarshalJSON omits absent fields and retains reported zero values as non-null JSON.
func (o usageOutput) MarshalJSON() ([]byte, error) {
	fields := make(map[string]any)
	addOptional(fields, "input_tokens", o.InputTokens)
	addOptional(fields, "output_tokens", o.OutputTokens)
	addOptional(fields, "cost", o.Cost)
	return json.Marshal(fields)
}

// diagnosticOutput preserves the concrete cause, endpoint body, and available HTTP metadata.
type diagnosticOutput struct {
	// Closed failure category for interpreting this object error.
	Code string `json:"code"`
	// Failed operation; inline HTTP execution uses classify.
	Operation string `json:"operation"`
	// Concrete cause without masking or replacement.
	Message string `json:"message"`
	// Omitted when no endpoint response was received.
	HTTPStatus mo.Option[int] `json:"-"`
	// Complete unfiltered response text, omitted when no body is available.
	UpstreamBody mo.Option[string] `json:"-"`
	// Available provider request identity, omitted when unavailable.
	UpstreamRequestID mo.Option[string] `json:"-"`
	// Positive HTTP attempt count; omitted before an attempt.
	Attempts mo.Option[int] `json:"-"`
	// Provider retry delay in seconds; a supplied zero remains present.
	RetryAfterSeconds mo.Option[float64] `json:"-"`
}

// MarshalJSON omits absent fields and retains reported zero values as non-null JSON.
func (o diagnosticOutput) MarshalJSON() ([]byte, error) {
	fields := map[string]any{"code": o.Code, "operation": o.Operation, "message": o.Message}
	addOptional(fields, "http_status", o.HTTPStatus)
	addOptional(fields, "upstream_body", o.UpstreamBody)
	addOptional(fields, "upstream_request_id", o.UpstreamRequestID)
	addOptional(fields, "attempts", o.Attempts)
	addOptional(fields, "retry_after_seconds", o.RetryAfterSeconds)
	return json.Marshal(fields)
}

// addOptional includes only reported values; Option.IsZero also treats Some(0) as zero.
// Testing presence here keeps zero confidence, token counts, cost, and retry delays in the MCP contract.
func addOptional[T any](fields map[string]any, name string, value mo.Option[T]) {
	if reported, present := value.Get(); present {
		fields[name] = reported
	}
}
