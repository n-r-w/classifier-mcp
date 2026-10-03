package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestNativeSourceTraversal compares symlink traversal through stdio with direct native filesystem access.
func (s *startupSuite) TestNativeSourceTraversal() {
	directory := s.T().TempDir()
	work := filepath.Join(directory, "work")
	target := filepath.Join(directory, "target")
	child := filepath.Join(target, "child")
	s.Require().NoError(os.MkdirAll(work, 0o700))
	s.Require().NoError(os.MkdirAll(child, 0o700))
	const wrong = "WRONG-CWD-CONTENT"
	const intended = "INTENDED-SYMLINK-CONTENT"
	s.Require().NoError(os.WriteFile(filepath.Join(work, "ticket.txt"), []byte(wrong), 0o600))
	s.Require().NoError(os.WriteFile(filepath.Join(target, "ticket.txt"), []byte(intended), 0o600))
	link := filepath.Join(work, "link")
	linkErr := os.Symlink(child, link)
	if linkErr != nil && runtime.GOOS != "linux" &&
		(os.IsPermission(linkErr) || errors.Is(linkErr, syscall.Errno(1314)) ||
			errors.Is(linkErr, syscall.ENOSYS) || errors.Is(linkErr, syscall.ENOTSUP)) {
		s.T().Skipf("native platform denies directory symlink creation: %v", linkErr)
	}
	s.Require().NoError(linkErr)
	separator := string(os.PathSeparator)
	relative := "link" + separator + ".." + separator + "ticket.txt"
	absolute := work + separator + relative
	expected, err := os.ReadFile(absolute)
	s.Require().NoError(err)
	if runtime.GOOS == "linux" {
		s.Equal(intended, string(expected))
	}
	referenced, err := os.Stat(absolute)
	s.Require().NoError(err)
	paths := []string{relative, absolute}
	if runtime.GOOS == "windows" {
		// Drive-relative and root-relative references use the child's native working drive.
		paths = append(
			paths,
			filepath.VolumeName(work)+relative,
			strings.TrimPrefix(absolute, filepath.VolumeName(absolute)),
		)
	}
	states := make(chan map[string]any, len(paths))
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if !s.NoError(json.NewDecoder(r.Body).Decode(&body)) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		states <- body["state"].(map[string]any)
		_, writeErr := w.Write([]byte(`{"model":"actual","answers":{"q":{"type":"noul","noul":0}}}`))
		s.NoError(writeErr)
	}))
	defer upstream.Close()
	ctx, cancel := context.WithTimeout(s.T().Context(), 30*time.Second)
	defer cancel()
	identity := &mcp.Implementation{
		Name:        "path-test",
		Version:     "dev",
		Title:       "",
		Description: "",
		WebsiteURL:  "",
		Icons:       nil,
	}
	client := mcp.NewClient(identity, nil)
	command := exec.CommandContext(ctx, s.binary)
	command.Env = startupEnv(upstream.URL, "configured")
	command.Dir = work
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: command, TerminateDuration: 0}, nil)
	s.Require().NoError(err)
	defer func() { s.NoError(session.Close()) }()
	objects := make([]any, 0, len(paths))
	for _, path := range paths {
		objects = append(objects, map[string]any{"id": path, "source": map[string]any{"type": "file", "path": path}})
	}
	arguments, err := json.Marshal(map[string]any{
		"objects": objects, "task": "identify ticket",
		"questions": map[string]any{"q": map[string]any{"type": "noul", "instructions": "is this a ticket?"}},
	})
	s.Require().NoError(err)
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Meta: nil, Name: "classify", Arguments: json.RawMessage(arguments), InputResponses: nil, RequestState: "",
	})
	s.Require().NoError(err)
	s.Require().False(result.IsError)
	output, err := json.Marshal(result.StructuredContent)
	s.Require().NoError(err)
	s.NotContains(string(output), wrong)
	s.NotContains(string(output), intended)
	var decoded map[string]any
	s.Require().NoError(json.Unmarshal(output, &decoded))
	entries := decoded["results"].([]any)
	s.Require().Len(entries, len(paths))
	for i, path := range paths {
		entry := entries[i].(map[string]any)
		s.Equal(path, entry["id"])
		s.Contains(entry, "answers")
		select {
		case state := <-states:
			s.Equal(string(expected), state["content"], path)
			location := state["source"].(map[string]any)["path"].(string)
			s.True(filepath.IsAbs(location), location)
			if filepath.IsAbs(path) {
				s.Equal(path, location)
			}
			acquired, statErr := os.Stat(location)
			s.Require().NoError(statErr)
			s.True(
				os.SameFile(referenced, acquired),
				"metadata %q must identify native reference %q",
				location,
				absolute,
			)
			metadataContent, readErr := os.ReadFile(location)
			s.Require().NoError(readErr)
			s.Equal(string(expected), string(metadataContent))
		case <-ctx.Done():
			s.T().Fatal("missing System One request state", ctx.Err())
		}
	}
}
