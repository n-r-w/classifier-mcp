package server

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Run starts the MCP server over stdin and stdout.
func Run(ctx context.Context) error {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "classifier-mcp",
		Version: "dev",
		Title:   "Classifier MCP",
	}, &mcp.ServerOptions{
		Instructions: "Classifier MCP server scaffold.",
	})

	return srv.Run(ctx, &mcp.StdioTransport{})
}
