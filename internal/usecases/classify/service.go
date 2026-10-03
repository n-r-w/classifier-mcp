// Package classify orchestrates text and local-file classification.
package classify

import (
	"context"
	"sync"

	"github.com/samber/mo"

	"github.com/n-r-w/classifier-mcp/internal/domain"
	"github.com/n-r-w/classifier-mcp/internal/server"
)

// Service evaluates objects independently and returns their outcomes in input order.
type Service struct {
	// Consumer-owned evaluator for one content and its complete common question set.
	model IModel
	// sources acquires file content through the consumer-owned source boundary.
	sources ISourceReader
	// parallelism bounds per-call workers; the model owns shared process-wide HTTP capacity.
	parallelism int
}

var _ server.IClassifier = (*Service)(nil)

// New constructs the classification use case with validated operational settings.
func New(model IModel, sources ISourceReader, parallelism int) *Service {
	return &Service{model: model, sources: sources, parallelism: parallelism}
}

// Classify preserves successful objects when another request fails or the call is canceled.
func (s *Service) Classify(ctx context.Context, command server.Command) []server.Outcome {
	outcomes := make([]server.Outcome, len(command.Objects))
	jobs := make(chan int)
	var workers sync.WaitGroup
	for range min(s.parallelism, len(command.Objects)) {
		workers.Go(func() {
			for index := range jobs {
				outcomes[index] = s.evaluateObject(ctx, command.Objects[index], command)
			}
		})
	}
	for index := range command.Objects {
		jobs <- index
	}
	close(jobs)
	workers.Wait()
	return outcomes
}

// evaluateObject produces one atomic outcome in a worker-owned result slot.
func (s *Service) evaluateObject(ctx context.Context, object server.Object, command server.Command) server.Outcome {
	content, location, sourceErr := s.acquire(ctx, object.Source)
	if sourceErr != nil {
		return mo.Right[server.Success, server.Failure](server.Failure{
			ID: object.ID, Error: sourceErr.Error(),
		})
	}
	response, err := s.model.Evaluate(ctx, Request{
		Content: content, Source: location, Task: command.Task, Questions: command.Questions, Full: command.Full,
	})
	if err != nil {
		return mo.Right[server.Success, server.Failure](server.Failure{
			ID: object.ID, Error: err.Error(),
		})
	}
	return mo.Left[server.Success, server.Failure](server.Success{
		ID: object.ID, Answers: response.Answers,
	})
}

// acquire selects caller text or obtains local text with resolved source metadata.
func (s *Service) acquire(ctx context.Context, source domain.Source,
) (content string, location mo.Option[domain.FileSource], err error) {
	if text, present := source.Left(); present {
		return text.Text, mo.None[domain.FileSource](), nil
	}
	file, _ := source.Right()
	acquired, err := s.sources.Read(ctx, file)
	return acquired.Content, mo.Some(acquired.Source), err
}
