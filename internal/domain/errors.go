package domain

import "github.com/samber/mo"

// Diagnostic identifies the cause of one failed object with the endpoint body and concrete technical details.
type Diagnostic struct {
	// Classification failure category used by the caller to distinguish causes.
	Code string
	// Operation that failed; inline model execution reports classify.
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
	// RetryAfterSeconds retains a reported zero delay even though requests are not retried.
	RetryAfterSeconds mo.Option[float64]
}
