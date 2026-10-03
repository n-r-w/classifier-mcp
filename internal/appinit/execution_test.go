package appinit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/n-r-w/classifier-mcp/internal/config"
)

// TestConcurrentCallsShareCapacityAndKeepOrder drives real HTTP completions out of input order across two MCP calls.
func (s *classificationSuite) TestConcurrentCallsShareCapacityAndKeepOrder() {
	ctx, cancel := context.WithTimeout(s.T().Context(), 10*time.Second)
	defer cancel()
	started := make(chan string, 6)
	releases := make(map[string]chan struct{}, 6)
	for _, id := range []string{"a0", "a1", "a2", "b0", "b1", "b2"} {
		releases[id] = make(chan struct{})
	}
	defer func() {
		for _, release := range releases {
			select {
			case <-release:
			default:
				close(release)
			}
		}
	}()
	var active, peak atomic.Int32
	session := s.connectWithConfig(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if !s.NoError(json.NewDecoder(r.Body).Decode(&request)) {
			return
		}
		id := request["state"].(map[string]any)["content"].(string)
		count := active.Add(1)
		for previous := peak.Load(); count > previous; previous = peak.Load() {
			if peak.CompareAndSwap(previous, count) {
				break
			}
		}
		started <- id
		select {
		case <-releases[id]:
		case <-r.Context().Done():
		}
		active.Add(-1)
		_, err := fmt.Fprintf(w, `{"model":%q,"answers":{"q":{"type":"noul","noul":0}}}`, id)
		s.NoError(err)
	}, config.Config{
		Endpoint: "", Model: "configured", APIKey: "", HTTPTimeout: time.Minute,
		MaxParallelism: 2, MaxAttempts: 3, RetryDelay: time.Second,
	})
	invoke := func(prefix string, completed chan<- *mcp.CallToolResult) {
		params := &mcp.CallToolParams{
			Meta: nil, Name: "classify", InputResponses: nil, RequestState: "",
			Arguments: json.RawMessage(fmt.Sprintf(`{"objects":[
    {"id":"%[1]s0","source":{"type":"text","text":"%[1]s0"}},
    {"id":"%[1]s1","source":{"type":"text","text":"%[1]s1"}},
    {"id":"%[1]s2","source":{"type":"text","text":"%[1]s2"}}
   ],"task":"task","questions":{"q":{"type":"noul","instructions":"condition"}}}`, prefix)),
		}
		result, err := session.CallTool(ctx, params)
		s.NoError(err)
		completed <- result
	}
	first := make(chan *mcp.CallToolResult, 1)
	second := make(chan *mcp.CallToolResult, 1)
	go invoke("a", first)
	for range 2 {
		select {
		case <-started:
		case <-ctx.Done():
			s.FailNow("initial HTTP requests did not start")
		}
	}
	go invoke("b", second)
	// Keep the first input blocked while later objects from both calls complete.
	close(releases["a1"])
	for range 4 {
		select {
		case id := <-started:
			close(releases[id])
		case <-ctx.Done():
			s.FailNow("remaining HTTP requests did not start")
		}
	}
	close(releases["a0"])
	for _, completed := range []chan *mcp.CallToolResult{first, second} {
		var result *mcp.CallToolResult
		select {
		case result = <-completed:
		case <-ctx.Done():
			s.FailNow("MCP call did not finish")
		}
		s.Require().NotNil(result)
		entries := s.structured(result)["results"].([]any)
		for index, entry := range entries {
			object := entry.(map[string]any)
			prefix := "a"
			if completed == second {
				prefix = "b"
			}
			s.Equal(fmt.Sprintf("%s%d", prefix, index), object["id"])
			s.Equal(object["id"], object["model"])
			s.Equal("ok", object["status"])
		}
	}
	s.Equal(int32(2), peak.Load())
}

// TestMCPCancellationStopsHTTP checks that a cancellation notification reaches the active endpoint operation.
func (s *classificationSuite) TestMCPCancellationStopsHTTP() {
	started := make(chan struct{})
	stopped := make(chan struct{})
	var attempts atomic.Int32
	session := s.connect(func(_ http.ResponseWriter, r *http.Request) {
		_, readErr := io.Copy(io.Discard, r.Body)
		s.NoError(readErr)
		attempts.Add(1)
		close(started)
		<-r.Context().Done()
		close(stopped)
	}, time.Minute)
	ctx, cancel := context.WithCancel(s.T().Context())
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		_, err := session.CallTool(ctx, &mcp.CallToolParams{
			Meta: nil, Name: "classify", Arguments: json.RawMessage(validArguments),
			InputResponses: nil, RequestState: "",
		})
		finished <- err
	}()
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		s.FailNow("HTTP request did not start")
	}
	cancel()
	select {
	case err := <-finished:
		s.Require().Error(err)
	case <-time.After(10 * time.Second):
		s.FailNow("canceled client did not stop")
	}
	select {
	case <-stopped:
	case <-time.After(10 * time.Second):
		s.FailNow("HTTP request did not receive cancellation")
	}
	s.Equal(int32(1), attempts.Load())
}
