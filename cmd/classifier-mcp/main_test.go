package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestStdioConnection(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	binary := filepath.Join(t.TempDir(), "classifier-mcp")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build server: %v\n%s", err, output)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "dev"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{
		Command: exec.CommandContext(ctx, binary),
	}, nil)
	if err != nil {
		t.Fatalf("connect to server: %v", err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			t.Errorf("close session: %v", err)
		}
	}()

	if err := session.Ping(ctx, nil); err != nil {
		t.Fatalf("ping server: %v", err)
	}
}
