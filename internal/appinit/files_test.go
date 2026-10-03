package appinit

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// TestLocalSources checks acquisition, exact fragments, metadata, and ordered partial success through MCP.
func (s *classificationSuite) TestLocalSources() {
	dir := s.T().TempDir()
	path := filepath.Join(dir, "tickets.unusual")
	text := "private source first\r\n\r\nlast"
	s.Require().NoError(os.WriteFile(path, []byte(text), 0o600))
	cwd, err := os.Getwd()
	s.Require().NoError(err)
	relative, err := filepath.Rel(cwd, path)
	s.Require().NoError(err)
	states := make(chan map[string]any, 4)
	session := s.connect(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		s.NoError(json.NewDecoder(r.Body).Decode(&body))
		states <- body["state"].(map[string]any)
		_, writeErr := w.Write([]byte(validAnswer))
		s.NoError(writeErr)
	}, time.Minute)
	objects := []any{
		map[string]any{"id": "inline", "source": map[string]any{"type": "text", "text": "private source inline"}},
		map[string]any{"id": "whole", "source": map[string]any{"type": "file", "path": relative}},
		map[string]any{
			"id":     "range",
			"source": map[string]any{"type": "file", "path": path, "lines": map[string]any{"start": 2, "end": 3}},
		},
		map[string]any{
			"id":     "missing",
			"source": map[string]any{"type": "file", "path": filepath.Join(dir, "missing")},
		},
		map[string]any{
			"id":     "bounds",
			"source": map[string]any{"type": "file", "path": path, "lines": map[string]any{"start": 2, "end": 4}},
		},
		map[string]any{"id": "blank", "source": map[string]any{"type": "file", "path": ""}},
		map[string]any{
			"id":     "reversed",
			"source": map[string]any{"type": "file", "path": path, "lines": map[string]any{"start": 3, "end": 2}},
		},
		map[string]any{
			"id":     "zero",
			"source": map[string]any{"type": "file", "path": path, "lines": map[string]any{"start": 0, "end": 1}},
		},
		map[string]any{"id": "after", "source": map[string]any{"type": "text", "text": "private source after"}},
	}
	var input map[string]any
	s.Require().NoError(json.Unmarshal([]byte(validArguments), &input))
	input["objects"] = objects
	data, err := json.Marshal(input)
	s.Require().NoError(err)
	result := s.call(session, string(data))
	s.Require().False(result.IsError)
	output := s.structured(result)
	entries := output["results"].([]any)
	s.Require().Len(entries, len(objects))
	for i, object := range objects {
		s.Equal(object.(map[string]any)["id"], entries[i].(map[string]any)["id"])
	}
	s.Require().Len(states, 4)
	acquired := []map[string]any{<-states, <-states, <-states, <-states}
	s.ElementsMatch([]map[string]any{
		{"task": "Classify", "content": "private source inline"},
		{
			"task": "Classify", "content": text,
			"source": map[string]any{"path": cwd + string(os.PathSeparator) + relative},
		},
		{
			"task": "Classify", "content": "\r\nlast",
			"source": map[string]any{"path": path, "lines": map[string]any{"start": float64(2), "end": float64(3)}},
		},
		{"task": "Classify", "content": "private source after"},
	}, acquired)
	for i := 3; i < 8; i++ {
		entry := entries[i].(map[string]any)
		s.Equal("error", entry["status"])
		diagnostic := entry["error"].(map[string]any)
		s.Equal("read_source", diagnostic["operation"])
		s.NotContains(diagnostic, "attempts")
		if i == 3 {
			s.Equal("source_read_failed", diagnostic["code"])
			s.Contains(diagnostic["message"], "missing")
		} else {
			s.Equal("invalid_source", diagnostic["code"])
		}
	}
	s.Contains(entries[4].(map[string]any)["error"].(map[string]any)["message"], "3 lines")
}

// TestFileShapesRejectWholeCall checks that a malformed source prevents even preceding HTTP work.
func (s *classificationSuite) TestFileShapesRejectWholeCall() {
	var calls atomic.Int64
	session := s.connect(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, err := w.Write([]byte(validAnswer))
		s.NoError(err)
	}, time.Minute)
	for _, source := range []string{
		`{"type":"file","path":"x","text":"conflict"}`,
		`{"type":"file","path":"x","extra":1}`,
		`{"type":"file","path":"x","lines":null}`,
		`{"type":"file","path":"x","lines":{"start":1}}`,
		`{"type":"file","path":"x","lines":{"start":1,"end":2,"extra":0}}`,
		`{"type":"file","path":"x","lines":{"start":1.5,"end":2}}`,
		`{"type":"file","path":null}`,
	} {
		input := strings.Replace(validArguments, `"objects": [`, `"objects": [{"id":"file","source":`+source+`},`, 1)
		result := s.call(session, input)
		s.True(result.IsError)
		s.Nil(result.StructuredContent)
	}
	s.Equal(int64(0), calls.Load())
}

// TestLargeLineBoundsRemainObjectErrors checks integer bounds beyond machine precision.
func (s *classificationSuite) TestLargeLineBoundsRemainObjectErrors() {
	session := s.connect(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte(validAnswer))
		s.NoError(err)
	}, time.Minute)
	input := strings.Replace(validArguments, `"objects": [`, `"objects": [{"id":"huge","source":{
 "type":"file","path":"unused","lines":{"start":1e100,"end":1}
 }},`, 1)
	result := s.call(session, input)
	s.Require().False(result.IsError)
	output := s.structured(result)
	entries := output["results"].([]any)
	s.Require().Len(entries, 2)
	diagnostic := entries[0].(map[string]any)["error"].(map[string]any)
	s.Equal("invalid_source", diagnostic["code"])
	s.Equal("read_source", diagnostic["operation"])
	s.Equal("ok", entries[1].(map[string]any)["status"])
}
