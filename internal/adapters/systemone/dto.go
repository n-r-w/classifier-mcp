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
	// Acquired text paired with the common classification context.
	State stateDTO `json:"state"`
}

// stateDTO combines the caller's task with one acquired text.
type stateDTO struct {
	// Common caller context shared by the object list.
	Task string `json:"task"`
	// Exact acquired text, including an allowed empty string.
	Content string `json:"content"`
	// Source is present for local files and retains the resolved path and requested range.
	Source mo.Option[sourceDTO] `json:"source,omitzero"`
}

// sourceDTO identifies the acquired local content in model state.
type sourceDTO struct {
	// Path is the resolved absolute file location.
	Path string `json:"path"`
	// Lines retains caller boundaries when a fragment was selected.
	Lines mo.Option[lineRangeDTO] `json:"lines,omitzero"`
}

// lineRangeDTO carries the inclusive 1-based fragment boundaries.
type lineRangeDTO struct {
	// Start is the first requested line.
	Start json.Number `json:"start"`
	// End is the last requested line.
	End json.Number `json:"end"`
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

// responseDTO extracts only the answer data needed by classification.
type responseDTO struct {
	// Answers retains each assessment until validation against the corresponding question.
	Answers map[string]json.RawMessage `json:"answers"`
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
}

// providerErrorDTO extracts the System One error message without projecting service metadata.
type providerErrorDTO struct {
	// Error can be a message string or the documented object containing message.
	Error json.RawMessage `json:"error"`
	// Message remains raw until the direct-message alternative is needed.
	Message json.RawMessage `json:"message"`
}

// providerCauseDTO contains the documented message inside an error object.
type providerCauseDTO struct {
	// Message is the endpoint's actual failure explanation.
	Message mo.Option[string] `json:"message"`
}
