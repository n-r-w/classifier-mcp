// Package classify orchestrates text and local-file classification.
package classify

import (
	"context"

	"github.com/samber/mo"

	"github.com/n-r-w/classifier-mcp/internal/domain"
	"github.com/n-r-w/classifier-mcp/internal/server"
)

// Service evaluates each object independently in input order.
type Service struct {
	// Consumer-owned evaluator for one content and its complete common question set.
	model IModel
	// sources acquires file content through the consumer-owned source boundary.
	sources ISourceReader
}

var _ server.IClassifier = (*Service)(nil)

// New constructs the classification use case.
func New(model IModel, sources ISourceReader) *Service {
	return &Service{model: model, sources: sources}
}

// Classify preserves successful objects when another request fails.
func (s *Service) Classify(ctx context.Context, command server.Command) []server.Outcome {
	outcomes := make([]server.Outcome, 0, len(command.Objects))
	for _, object := range command.Objects {
		content, location, sourceDiagnostic := s.acquire(ctx, object.Source)
		if detail, present := sourceDiagnostic.Get(); present {
			outcomes = append(outcomes, mo.Right[server.Success, server.Failure](server.Failure{
				ID: object.ID, Diagnostic: detail, Usage: mo.None[domain.Usage](),
			}))
			continue
		}
		response, diagnostic := s.model.Evaluate(
			ctx,
			Request{
				Content:   content,
				Source:    location,
				Task:      command.Task,
				Questions: command.Questions,
				Full:      command.Full,
			},
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

// acquire selects caller text or obtains local text with resolved source metadata.
func (s *Service) acquire(
	ctx context.Context,
	source domain.Source,
) (content string, location mo.Option[domain.FileSource], diagnostic mo.Option[domain.Diagnostic]) {
	if text, present := source.Left(); present {
		return text.Text, mo.None[domain.FileSource](), mo.None[domain.Diagnostic]()
	}
	file, _ := source.Right()
	acquired, diagnostic := s.sources.Read(ctx, file)
	return acquired.Content, mo.Some(acquired.Source), diagnostic
}
