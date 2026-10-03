// Package main provides the classifier MCP executable.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/n-r-w/classifier-mcp/internal/appinit"
)

// main reports startup or transport failure on stderr and exits with a nonzero status.
func main() {
	if err := appinit.Run(); err != nil {
		slog.ErrorContext(context.Background(), "classifier-mcp stopped", "error", err)
		os.Exit(1)
	}
}
