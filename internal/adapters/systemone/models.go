package systemone

import "github.com/samber/mo"

// attemptFailure is private execution state for selective retries and cancellation races.
type attemptFailure struct {
	// kind distinguishes explicit overload from cancellation or unrelated request failures.
	kind string
	// cause preserves the actual error returned when execution terminates.
	cause error
	// httpStatus is present only after an endpoint response and identifies 429/529 overloads.
	httpStatus mo.Option[int]
	// attempts counts requests already attempted; zero identifies cancellation before a request.
	attempts int
	// retryAfterSeconds is the endpoint delay in seconds; None selects the configured delay.
	retryAfterSeconds mo.Option[float64]
}
