// Package domain contains classification assessment models.
package domain

import "github.com/samber/mo"

// Question represents exactly one of the three question definitions.
type Question = mo.Either3[ChoiceQuestion, NoulQuestion, ScoreQuestion]

// Assessment represents exactly one of the three validated assessments.
type Assessment = mo.Either3[Choice, Noul, Score]

// ChoiceQuestion defines mutually exclusive categories and their caller-supplied guidance.
type ChoiceQuestion struct {
	// Independent caller guidance; structured numeric values retain their JSON precision.
	Instructions any
	// Complete set of category names and descriptions; null lets the name provide the guidance.
	Criteria map[string]any
}

// Kind supplies the System One discriminator for category selection.
func (q ChoiceQuestion) Kind() string { return ChoiceKind }

// NoulQuestion defines a condition with optional guidance for its positive and negative cases.
type NoulQuestion struct {
	// Condition guidance expressed as a string, object, or array.
	Instructions any
	// None uses instructions alone; Some contains exactly the positive and negative descriptions.
	Criteria mo.Option[map[string]any]
}

// Kind supplies the System One discriminator for condition probability.
func (q NoulQuestion) Kind() string { return NoulKind }

// ScoreQuestion defines an ordered scale whose indices start at zero.
type ScoreQuestion struct {
	// Independent guidance for evaluating the ordered scale.
	Instructions any
	// Caller descriptions in level order; index zero is the first level.
	Criteria []any
}

// Kind supplies the System One discriminator for ordered-scale assessment.
func (q ScoreQuestion) Kind() string { return ScoreKind }

// Choice retains the provider selection and distribution, including the provider's tie decision.
type Choice struct {
	// Provider-selected category from the caller set, including the provider tie decision.
	Selection string
	// Complete mutually exclusive category distribution with values in [0, 1].
	Probabilities map[string]float64
	// Confidence measures uncertainty across the distribution, not the selected probability.
	Confidence mo.Option[float64]
}

// Kind supplies the response discriminator for category selection.
func (a Choice) Kind() string { return ChoiceKind }

// Noul is the probability that a condition holds, rather than a thresholded Boolean.
type Noul struct {
	// Likelihood that the condition holds in [0, 1]; zero is a reported result.
	Probability float64
}

// Kind supplies the response discriminator for condition probability.
func (a Noul) Kind() string { return NoulKind }

// Score retains the probability-weighted value on a caller's scale.
type Score struct {
	// Probability-weighted scale index, potentially between adjacent levels.
	Value float64
	// Probabilities may be absent when the caller requests only the compact result.
	Probabilities mo.Option[map[string]float64]
	// Decimal level indices map to the original caller descriptions, preserving structured values.
	Legend map[string]any
	// Optional provider uncertainty in [0, 1]; None remains unreported.
	Confidence mo.Option[float64]
}

// Kind supplies the response discriminator for ordered-scale assessment.
func (a Score) Kind() string { return ScoreKind }

// Usage preserves unreported token counts and cost separately from reported zero.
type Usage struct {
	// Provider-reported input token count; None distinguishes missing from zero.
	InputTokens mo.Option[int64]
	// Provider-reported generated token count; None distinguishes missing from zero.
	OutputTokens mo.Option[int64]
	// Cost stays in the endpoint's billing unit; OpenRouter reports credits.
	Cost mo.Option[float64]
}

// HasValues reports whether the provider supplied at least one usage value.
func (u Usage) HasValues() bool {
	return u.InputTokens.IsSome() || u.OutputTokens.IsSome() || u.Cost.IsSome()
}
