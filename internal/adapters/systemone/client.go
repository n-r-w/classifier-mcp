// Package systemone implements the System One HTTP boundary.
package systemone

import (
	"net/http"
	"time"

	"github.com/n-r-w/classifier-mcp/internal/usecases/classify"
)

// Client bounds active HTTP requests and retries explicit overload responses across concurrent calls.
type Client struct {
	// Reusable transport with the startup timeout and redirect policy; shared across calls.
	http *http.Client
	// Complete configured evaluation URL, including any query parameters.
	endpoint string
	// Explicit caller-configured identifier sent unchanged to the endpoint.
	model string
	// Bearer credential; an empty value omits the Authorization header.
	apiKey string
	// capacity is shared by every call using this process's assembled client.
	capacity chan struct{}
	// maxAttempts counts the initial request and explicit overload retries.
	maxAttempts int
	// retryDelay is the fallback when an overload response supplies no valid delay.
	retryDelay time.Duration
}

var _ classify.IModel = (*Client)(nil)

// New constructs a client with validated connection settings.
func New(client *http.Client, endpoint, model, apiKey string,
	maxParallelism, maxAttempts int, retryDelay time.Duration,
) *Client {
	return &Client{
		http: client, endpoint: endpoint, model: model, apiKey: apiKey,
		capacity: make(chan struct{}, maxParallelism), maxAttempts: maxAttempts, retryDelay: retryDelay,
	}
}
