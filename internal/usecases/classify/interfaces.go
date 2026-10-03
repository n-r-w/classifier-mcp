package classify

import (
	"context"

	"github.com/samber/mo"

	"github.com/n-r-w/classifier-mcp/internal/domain"
)

//go:generate go tool mockgen -source=interfaces.go -destination=interfaces_mock.go -package=classify

// IModel evaluates all questions for one text.
type IModel interface {
	// Evaluate returns a complete compatible answer set or its concrete failure cause.
	Evaluate(context.Context, Request) (Response, error)
}

// ISourceReader acquires local text and its resolved location.
type ISourceReader interface {
	// Read returns exact content and resolved metadata, or a concrete source error.
	Read(context.Context, domain.FileSource) (AcquiredSource, error)
}

// AcquiredSource pairs content with its resolved file reference.
type AcquiredSource struct {
	// Content is the whole file or exact caller-selected fragment.
	Content string
	// Source retains the resolved path and caller boundaries.
	Source domain.FileSource
}

// Request supplies one acquired content and common definitions.
type Request struct {
	// Exact acquired text for one independent model evaluation.
	Content string
	// Source is present only for acquired file content, with its resolved location.
	Source mo.Option[domain.FileSource]
	// Caller context shared by every object in the list.
	Task string
	// All independent definitions evaluated together for this content.
	Questions map[string]domain.Question
	// Full requires a Score distribution even when the endpoint omits it by default.
	Full bool
}

// Response carries the complete provider assessment set.
type Response struct {
	// Answers contains all compatible assessments on success; unused on failure.
	Answers map[string]domain.Assessment
}
