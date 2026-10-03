package server

import (
	"github.com/samber/mo"

	"github.com/n-r-w/classifier-mcp/internal/domain"
)

// projectUsage maps a reported usage object without turning missing values into zeros.
func projectUsage(usage mo.Option[domain.Usage]) mo.Option[usageOutput] {
	value, present := usage.Get()
	if !present {
		return mo.None[usageOutput]()
	}
	return mo.Some(usageOutput{InputTokens: value.InputTokens, OutputTokens: value.OutputTokens, Cost: value.Cost})
}

// projectSuccess selects the caller projection while retaining provider numbers unchanged.
func projectSuccess(value Success, full bool) successOutput {
	answers := make(map[string]any, len(value.Answers))
	for id, answer := range value.Answers {
		if assessment, isChoice := answer.Arg1(); isChoice {
			output := choiceOutput{
				Type:          assessment.Kind(),
				Choice:        assessment.Selection,
				Probability:   assessment.Probabilities[assessment.Selection],
				Confidence:    assessment.Confidence,
				Probabilities: mo.None[map[string]float64](),
			}
			if full {
				output.Probabilities = mo.Some(assessment.Probabilities)
			}
			answers[id] = output
		} else if assessment, isNoul := answer.Arg2(); isNoul {
			answers[id] = noulOutput{Type: assessment.Kind(), Noul: assessment.Probability}
		} else if assessment, isScore := answer.Arg3(); isScore {
			output := scoreOutput{
				Type:          assessment.Kind(),
				Score:         assessment.Value,
				Confidence:    assessment.Confidence,
				Probabilities: mo.None[map[string]float64](),
				Legend:        mo.None[map[string]any](),
			}
			if full {
				output.Probabilities = assessment.Probabilities
				output.Legend = mo.Some(assessment.Legend)
			}
			answers[id] = output
		}
	}
	return successOutput{
		ID:      value.ID,
		Status:  "ok",
		Model:   value.Model,
		Answers: answers,
		Usage:   projectUsage(value.Usage),
	}
}
