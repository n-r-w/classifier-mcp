package systemone

import (
	"encoding/json"

	"github.com/samber/mo"
)

// requestDTO is the System One evaluation request, with the configured model unchanged.
type requestDTO struct {
	// Configured identifier rather than a locally selected model alias.
	Model string `json:"model"`
	// All independent definitions for this one content, keyed by caller question ID.
	Questions map[string]questionDTO `json:"questions"`
	// Acquired inline text paired with the common classification context.
	State stateDTO `json:"state"`
}

// stateDTO combines the caller's task with one acquired inline text.
type stateDTO struct {
	// Common caller context shared by the object list.
	Task string `json:"task"`
	// Exact inline text, including an allowed empty string.
	Content string `json:"content"`
}

// questionDTO preserves caller guidance and omits criteria when Noul has none.
type questionDTO struct {
	// Choice, Noul, or Score discriminator from the validated definition.
	Type string `json:"type"`
	// String, object, or array guidance with number tokens preserved.
	Instructions any `json:"instructions"`
	// None only when Noul omits its optional rubric; required rubrics are nonempty.
	Criteria mo.Option[any] `json:"criteria,omitzero"`
}

// responseDTO retains each answer as JSON until validation against its question.
type responseDTO struct {
	// Required nonblank identity reported by the endpoint.
	Model string `json:"model"`
	// Raw assessments are validated against the matching requested definitions.
	Answers map[string]json.RawMessage `json:"answers"`
	// None for absent or null provider usage; unknown billing values are not estimated.
	Usage mo.Option[usageDTO] `json:"usage"`
}

// answerDTO captures presence separately from zero before selecting an assessment variant.
// The HTTP boundary rejects fields that belong to another variant, including null fields.
type answerDTO struct {
	// Must match the assessment type of the requested question.
	Type string `json:"type"`
	// Required only for Choice; names one of the supplied categories.
	Choice mo.Option[string] `json:"choice"`
	// Required only for Noul; the reported probability lies in [0, 1].
	Noul mo.Option[float64] `json:"noul"`
	// Required only for Score; may be fractional within the supplied scale.
	Score mo.Option[float64] `json:"score"`
	// Choice requires every category; Score requires every level in full mode.
	Probabilities mo.Option[map[string]float64] `json:"probabilities"`
	// Optional provider uncertainty in [0, 1]; absent or null stays None.
	Confidence mo.Option[float64] `json:"confidence"`
	// Raw Score descriptions preserve numeric tokens before comparison with caller criteria.
	Legend mo.Option[json.RawMessage] `json:"legend"`
}

// usageDTO treats absent and null fields as unreported, while retaining reported zeros.
type usageDTO struct {
	// Optional nonnegative count of tokens consumed for this object request.
	InputTokens mo.Option[int64] `json:"input_tokens"`
	// Optional nonnegative count of generated answer tokens.
	OutputTokens mo.Option[int64] `json:"output_tokens"`
	// Optional nonnegative amount in the endpoint billing unit; zero is reported, not absent.
	Cost mo.Option[float64] `json:"cost"`
}

// responseMetadataDTO extracts a provider request identity for failure diagnostics.
type responseMetadataDTO struct {
	// Optional endpoint request identity used when response headers supply none.
	ID mo.Option[string] `json:"id"`
}
