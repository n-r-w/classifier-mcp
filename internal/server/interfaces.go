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

// Success contains the complete answer set.
type Success struct {
	// Caller identity associating this result with its input object.
	ID string
	// Actual endpoint-reported model identifier rather than the configured alias.
	Model string
	// Complete validated set keyed by exactly the requested question IDs.
	Answers map[string]domain.Assessment
	// None when the endpoint reports no billing values; reported zeros stay present.
	Usage mo.Option[domain.Usage]
}

// Failure contains an object diagnostic and any reported usage.
type Failure struct {
	// Caller identity associating this atomic error with its input object.
	ID string
	// Unfiltered cause and available endpoint metadata for the failed operation.
	Diagnostic domain.Diagnostic
	// Optional billing values from the failed request; missing values are not estimated.
	Usage mo.Option[domain.Usage]
}
