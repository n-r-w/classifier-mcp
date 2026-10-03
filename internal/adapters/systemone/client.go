// Package systemone implements the System One HTTP boundary.
package systemone

import (
	"net/http"

	"github.com/n-r-w/classifier-mcp/internal/usecases/classify"
)

// Client sends single-attempt System One requests through a reusable HTTP client.
type Client struct {
	// Reusable transport with the startup timeout and redirect policy; shared across calls.
	http *http.Client
	// Complete configured evaluation URL, including any query parameters.
	endpoint string
	// Explicit caller-configured identifier sent unchanged to the endpoint.
	model string
	// Bearer credential; an empty value omits the Authorization header.
	apiKey string
}

var _ classify.IModel = (*Client)(nil)

// New constructs a client with validated connection settings.
func New(client *http.Client, endpoint, model, apiKey string) *Client {
	return &Client{http: client, endpoint: endpoint, model: model, apiKey: apiKey}
}
