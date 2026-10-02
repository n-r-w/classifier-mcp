package main

import (
	"log/slog"
	"os"

	"github.com/n-r-w/classifier-mcp/internal/appinit"
)

func main() {
	if err := appinit.Run(); err != nil {
		slog.Error("classifier-mcp stopped", "error", err)
		os.Exit(1)
	}
}
