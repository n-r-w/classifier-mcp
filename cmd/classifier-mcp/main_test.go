package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/suite"
)

// startupSuite checks the built executable over stdio with isolated environment settings.
type startupSuite struct {
	// Suite supplies assertion and lifecycle state for the isolated entry-point cases.
	suite.Suite
	// Temporary platform executable built once for the real startup checks.
	binary string
}

// TestStartup exercises process exit and classification through the command entry point.
func TestStartup(t *testing.T) { t.Parallel(); suite.Run(t, new(startupSuite)) }

// SetupSuite builds one temporary executable with the platform-appropriate suffix.
func (s *startupSuite) SetupSuite() {
	s.binary = filepath.Join(s.T().TempDir(), "classifier-mcp")
	if os.PathSeparator == '\\' {
		s.binary += ".exe"
	}
	build := exec.CommandContext(s.T().Context(), "go", "build", "-o", s.binary, ".")
	output, err := build.CombinedOutput()
	s.Require().NoError(err, string(output))
}

// startupEnv isolates child-process connection settings from the parent process environment.
func startupEnv(endpoint, model string) []string {
	values := os.Environ()
	result := make([]string, 0, len(values)+4)
	for _, value := range values {
		key, _, _ := strings.Cut(value, "=")
		if strings.EqualFold(key, "SYSTEM_ONE_ENDPOINT") || strings.EqualFold(key, "SYSTEM_ONE_MODEL") ||
			strings.EqualFold(key, "SYSTEM_ONE_API_KEY") || strings.EqualFold(key, "SYSTEM_ONE_HTTP_TIMEOUT") ||
			strings.EqualFold(key, "SYSTEM_ONE_MAX_PARALLELISM") || strings.EqualFold(key, "SYSTEM_ONE_MAX_ATTEMPTS") ||
			strings.EqualFold(key, "SYSTEM_ONE_RETRY_DELAY") {
			continue
		}
		result = append(result, value)
	}
	return append(
		result,
		"SYSTEM_ONE_ENDPOINT="+endpoint,
		"SYSTEM_ONE_MODEL="+model,
		"SYSTEM_ONE_API_KEY=",
		"SYSTEM_ONE_HTTP_TIMEOUT=60s",
		"SYSTEM_ONE_MAX_PARALLELISM=4",
		"SYSTEM_ONE_MAX_ATTEMPTS=3",
		"SYSTEM_ONE_RETRY_DELAY=1s",
	)
}

// TestMissingConfigurationExits requires a nonzero process exit that names the missing setting.
func (s *startupSuite) TestMissingConfigurationExits() {
	for _, test := range []struct {
		// endpoint is the explicit process setting; empty triggers its required-setting case.
		endpoint string
		// model is the explicit process identifier; empty triggers its required-setting case.
		model string
		// missing names the setting that the process exit diagnostic must identify.
		missing string
	}{
		{endpoint: "", model: "", missing: "SYSTEM_ONE_ENDPOINT"},
		{endpoint: "http://localhost:1234/systemone", model: "", missing: "SYSTEM_ONE_MODEL"},
	} {
		command := exec.CommandContext(s.T().Context(), s.binary)
		command.Env = startupEnv(test.endpoint, test.model)
		output, err := command.CombinedOutput()
		s.Require().Error(err)
		var exit *exec.ExitError
		s.Require().ErrorAs(err, &exit)
		s.Equal(1, exit.ExitCode())
		s.Contains(string(output), test.missing)
	}
}

// TestStdioConnection verifies initialization, ping, and classification through real startup assembly.
func (s *startupSuite) TestStdioConnection() {
	directory := s.T().TempDir()
	s.Require().
		NoError(os.WriteFile(filepath.Join(directory, "ticket.txt"), []byte("first\nprivate source\nlast\n"), 0o600))
	var attempts atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if attempts.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, err := w.Write([]byte("overloaded"))
			s.NoError(err)
			return
		}
		s.Equal("1", request.URL.Query().Get("version"))
		var body map[string]any
		s.NoError(json.NewDecoder(request.Body).Decode(&body))
		state := body["state"].(map[string]any)
		if state["content"] != "hello" {
			s.Equal("private source\n", state["content"])
			s.Equal(
				map[string]any{
					"path":  filepath.Join(directory, "ticket.txt"),
					"lines": map[string]any{"start": float64(2), "end": float64(2)},
				},
				state["source"],
			)
		}
		_, err := w.Write([]byte(`{
  "model": "actual",
  "answers": {
    "q": {
      "type": "noul",
      "noul": 0
    }
  }
}`))
		s.NoError(err)
	}))
	defer upstream.Close()
	ctx, cancel := context.WithTimeout(s.T().Context(), 30*time.Second)
	defer cancel()
	identity := &mcp.Implementation{
		Name:        "test-client",
		Version:     "dev",
		Title:       "",
		Description: "",
		WebsiteURL:  "",
		Icons:       nil,
	}
	client := mcp.NewClient(identity, nil)
	command := exec.CommandContext(ctx, s.binary)
	command.Env = startupEnv(upstream.URL+"?version=1", "configured")
	command.Dir = directory
	transport := &mcp.CommandTransport{Command: command, TerminateDuration: 0}
	session, err := client.Connect(ctx, transport, nil)
	s.Require().NoError(err)
	defer func() { s.NoError(session.Close()) }()
	s.Require().NoError(session.Ping(ctx, nil))
	params := &mcp.CallToolParams{
		Meta:           nil,
		Name:           "classify",
		InputResponses: nil,
		RequestState:   "",
		Arguments: json.RawMessage(
			`{
  "objects": [
    {
      "id": "x",
      "source": {
        "type": "text",
        "text": "hello"
      }
    }
  ],
  "task": "task",
  "questions": {
    "q": {
      "type": "noul",
      "instructions": "condition"
    }
  }
}`,
		),
	}
	result, err := session.CallTool(ctx, params)
	s.Require().NoError(err)
	s.False(result.IsError)
	s.NotNil(result.StructuredContent)
	params.Arguments = json.RawMessage(
		`{"objects":[{"id":"file","source":{
 "type":"file","path":"ticket.txt","lines":{"start":2,"end":2}
 }}],"task":"task","questions":{"q":{"type":"noul","instructions":"condition"}}}`,
	)
	fileResult, fileErr := session.CallTool(ctx, params)
	s.Require().NoError(fileErr)
	s.False(fileResult.IsError)
	s.NotNil(fileResult.StructuredContent)
	data, marshalErr := json.Marshal(fileResult.StructuredContent)
	s.Require().NoError(marshalErr)
	s.NotContains(string(data), "private source")
	s.Equal(int32(3), attempts.Load())
}
