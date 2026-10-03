// Package classify orchestrates inline object classification.
package classify

import (
	"context"

	"github.com/samber/mo"

	"github.com/n-r-w/classifier-mcp/internal/server"
)

// Service evaluates each object independently in input order.
type Service struct {
	// Consumer-owned evaluator for one content and its complete common question set.
	model IModel
}

var _ server.IClassifier = (*Service)(nil)

// New constructs the classification use case.
func New(model IModel) *Service { return &Service{model: model} }

// Classify preserves successful objects when another request fails.
func (s *Service) Classify(ctx context.Context, command server.Command) []server.Outcome {
	outcomes := make([]server.Outcome, 0, len(command.Objects))
	for _, object := range command.Objects {
		response, diagnostic := s.model.Evaluate(
			ctx,
			Request{Content: object.Text, Task: command.Task, Questions: command.Questions, Full: command.Full},
		)
		if detail, present := diagnostic.Get(); present {
			outcomes = append(
				outcomes,
				mo.Right[server.Success, server.Failure](
					server.Failure{ID: object.ID, Diagnostic: detail, Usage: response.Usage},
				),
			)
			continue
		}
		outcomes = append(
			outcomes,
			mo.Left[server.Success, server.Failure](
				server.Success{ID: object.ID, Model: response.Model, Answers: response.Answers, Usage: response.Usage},
			),
		)
	}
	return outcomes
}
