package server

import (
	"encoding/json"
	"errors"
	"math/big"
	"strings"

	"github.com/samber/mo"

	"github.com/n-r-w/classifier-mcp/internal/domain"
)

// classifyInput is the schema-validated tool request.
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

// objectInput associates one content source with a caller-defined identity.
type objectInput struct {
	// Nonempty identity used to associate the returned outcome with this source.
	ID string `json:"id"`
	// Exclusive source alternative checked by the published input schema.
	Source sourceInput `json:"source"`
}

// sourceInput retains schema-validated text or file fields until domain mapping.
type sourceInput struct {
	// Type selects text or file.
	Type string `json:"type"`
	// Text is present for inline content, including an empty string.
	Text mo.Option[string] `json:"text,omitzero"`
	// Path is present for a local file reference.
	Path mo.Option[string] `json:"path,omitzero"`
	// Lines is present when the caller selects an inclusive fragment.
	Lines mo.Option[lineRangeInput] `json:"lines,omitzero"`
}

// lineRangeInput carries both required caller boundaries.
type lineRangeInput struct {
	// Start is the inclusive first line; numerical semantics belong to acquisition.
	Start json.Number `json:"start"`
	// End is the inclusive last line; numerical semantics belong to acquisition.
	End json.Number `json:"end"`
}

// mapSource maps the exclusive schema-validated fields to a typed source.
func mapSource(s sourceInput) domain.Source {
	if s.Type == "text" {
		return mo.Left[domain.TextSource, domain.FileSource](domain.TextSource{Text: s.Text.OrEmpty()})
	}
	lines := mo.None[domain.LineRange]()
	if selected, present := s.Lines.Get(); present {
		lines = mo.Some(
			domain.LineRange{Start: parseIntegerBoundary(selected.Start), End: parseIntegerBoundary(selected.End)},
		)
	}
	return mo.Right[domain.TextSource, domain.FileSource](domain.FileSource{Path: s.Path.OrEmpty(), Lines: lines})
}

// parseIntegerBoundary converts a schema-validated integer token, including exponent notation, exactly.
func parseIntegerBoundary(number json.Number) *big.Int {
	value, _ := new(big.Rat).SetString(number.String())
	return value.Num()
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

// successOutput contains the complete requested assessments for one input identity.
type successOutput struct {
	// ID associates the assessments with the caller's object.
	ID string `json:"id"`
	// Answers contains exactly the requested question IDs and their chosen projections.
	Answers map[string]any `json:"answers"`
}

// errorOutput contains the concrete cause for one independently failed object.
type errorOutput struct {
	// ID associates the cause with the caller's object.
	ID string `json:"id"`
	// Error is the actual concise text cause; it contains no service metadata envelope.
	Error string `json:"error"`
}

// choiceOutput retains the provider selection and requested distribution.
type choiceOutput struct {
	// Choice is the provider-selected category from the caller's alternatives.
	Choice string `json:"choice"`
	// Probability is the selected category's reported probability in [0, 1].
	Probability float64 `json:"probability"`
	// Confidence is omitted when unreported; a reported zero remains present.
	Confidence mo.Option[float64] `json:"-"`
	// Probabilities is included only for full mode, with every category key.
	Probabilities mo.Option[map[string]float64] `json:"-"`
}

// MarshalJSON preserves reported zero values and omits unreported optional fields.
func (o choiceOutput) MarshalJSON() ([]byte, error) {
	fields := map[string]any{"choice": o.Choice, "probability": o.Probability}
	addOptional(fields, fieldConfidence, o.Confidence)
	addOptional(fields, fieldProbabilities, o.Probabilities)
	return json.Marshal(fields)
}

// noulOutput has one probability in either result mode.
type noulOutput struct {
	// Noul is the reported likelihood of the condition in [0, 1].
	Noul float64 `json:"noul"`
}

// scoreOutput retains the provider assessment and requested distribution.
type scoreOutput struct {
	// Score is the provider's value within the caller's ordered scale.
	Score float64 `json:"score"`
	// Confidence is optional provider uncertainty in [0, 1].
	Confidence mo.Option[float64] `json:"-"`
	// Probabilities is included only for full mode, keyed by level indices.
	Probabilities mo.Option[map[string]float64] `json:"-"`
}

// MarshalJSON preserves provider values without recomputing the assessment.
func (o scoreOutput) MarshalJSON() ([]byte, error) {
	fields := map[string]any{"score": o.Score}
	addOptional(fields, fieldConfidence, o.Confidence)
	addOptional(fields, fieldProbabilities, o.Probabilities)
	return json.Marshal(fields)
}

// addOptional includes reported values; Option.IsZero also treats Some(0) as zero.
func addOptional[T any](fields map[string]any, name string, value mo.Option[T]) {
	if reported, present := value.Get(); present {
		fields[name] = reported
	}
}
