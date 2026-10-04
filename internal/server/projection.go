package server

import (
	"github.com/samber/mo"
)

// projectSuccess selects the caller projection while retaining provider numbers unchanged.
func projectSuccess(value Success, full bool) successOutput {
	answers := make(map[string]any, len(value.Answers))
	for id, answer := range value.Answers {
		if assessment, isChoice := answer.Arg1(); isChoice {
			output := choiceOutput{
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
			answers[id] = truthOutput{Truth: assessment.Probability}
		} else if assessment, isScore := answer.Arg3(); isScore {
			output := scoreOutput{
				Score:         assessment.Value,
				Confidence:    assessment.Confidence,
				Probabilities: mo.None[map[string]float64](),
			}
			if full {
				output.Probabilities = assessment.Probabilities
			}
			answers[id] = output
		}
	}
	return successOutput{
		ID:      value.ID,
		Answers: answers,
	}
}
