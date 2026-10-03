package classify

import (
	"context"

	"github.com/samber/mo"

	"github.com/n-r-w/classifier-mcp/internal/domain"
)

// IModel evaluates all questions for one text.
type IModel interface {
	// Evaluate returns None for the diagnostic only for a complete compatible answer set.
	Evaluate(context.Context, Request) (Response, mo.Option[domain.Diagnostic])
}

// Request supplies one acquired content and common definitions.
type Request struct {
	// Exact acquired text for one independent model evaluation.
	Content string
	// Caller context shared by every object in the list.
	Task string
	// All independent definitions evaluated together for this content.
	Questions map[string]domain.Question
	// Full requires a Score distribution even when the endpoint omits it by default.
	Full bool
}

// Response carries provider data; on failure, only reported usage may be retained.
type Response struct {
	// Endpoint identity used only when the diagnostic is None.
	Model string
	// Complete compatible answer set on success; never consumed on failure.
	Answers map[string]domain.Assessment
	// Optional provider billing values, including any retained on a failed evaluation.
	Usage mo.Option[domain.Usage]
}
