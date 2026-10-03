package appinit

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/n-r-w/classifier-mcp/internal/adapters/localfile"
	"github.com/n-r-w/classifier-mcp/internal/adapters/systemone"
	"github.com/n-r-w/classifier-mcp/internal/config"
	"github.com/n-r-w/classifier-mcp/internal/server"
	"github.com/n-r-w/classifier-mcp/internal/usecases/classify"
)

// New assembles the MCP application from validated immutable settings.
func New(cfg config.Config) *mcp.Server {
	client := &http.Client{
		Transport: nil, Jar: nil, Timeout: cfg.HTTPTimeout,
		// Returning redirects as endpoint errors prevents sending credentials or content to another URL.
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
	model := systemone.New(client, cfg.Endpoint, cfg.Model, cfg.APIKey)
	usecase := classify.New(model, localfile.New())
	return server.New(usecase)
}
