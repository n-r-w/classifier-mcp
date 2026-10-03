package domain

import "github.com/samber/mo"

// Diagnostic identifies the cause of one failed object with the endpoint body and concrete technical details.
type Diagnostic struct {
	// Classification failure category used by the caller to distinguish causes.
	Code string
	// Operation that failed: read_source for acquisition or classify for model execution.
	Operation string
	// Concrete cause retained without masking or filtering.
	Message string
	// None when no endpoint response was received; otherwise the received HTTP status.
	HTTPStatus mo.Option[int]
	// None when no body is available; otherwise the complete response text unchanged.
	UpstreamBody mo.Option[string]
	// Optional provider identity from response headers or body metadata.
	UpstreamRequestID mo.Option[string]
	// Attempts is absent before HTTP work; a reported value includes the first attempt.
	Attempts mo.Option[int]
	// RetryAfterSeconds is the final supplied delay in seconds; a reported zero remains present.
	RetryAfterSeconds mo.Option[float64]
}
