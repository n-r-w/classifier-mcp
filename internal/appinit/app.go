package appinit

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/n-r-w/classifier-mcp/internal/server"
)

// Run configures process logging and runs the server until shutdown.
func Run() error {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := server.Run(ctx)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
