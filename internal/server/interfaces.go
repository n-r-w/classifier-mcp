package server

import (
	"context"

	"github.com/samber/mo"

	"github.com/n-r-w/classifier-mcp/internal/domain"
)

// IClassifier evaluates a validated list of text and file sources.
type IClassifier interface {
	// Classify accepts validated shared definitions and returns one atomic entry per object, in order.
	Classify(context.Context, Command) []Outcome
}

// Command carries validated common definitions and source objects.
type Command struct {
	// Validated caller list whose order defines the outcome order.
	Objects []Object
	// Common nonblank classification context passed unchanged to every request.
	Task string
	// Independent definitions shared by all objects, keyed by caller identity.
	Questions map[string]domain.Question
	// Full requires distribution data needed by the caller's full projection.
	Full bool
}

// Object associates a content source with caller identity.
type Object struct {
	// Nonempty case-sensitive identity copied to its outcome.
	ID string
	// Exclusive inline text or local file reference.
	Source domain.Source
}

// Outcome is exactly one success or failure entry in the completed list.
type Outcome = mo.Either[Success, Failure]

// Success contains the complete requested assessments for one object.
type Success struct {
	// ID is the caller's input identity.
	ID string
	// Answers contains the validated assessment set without unused provider metadata.
	Answers map[string]domain.Assessment
}

// Failure contains one concrete text cause while the batch continues independently.
type Failure struct {
	// ID is the caller's input identity.
	ID string
	// Error is the source, transport, provider, answer, or cancellation cause.
	Error string
}
