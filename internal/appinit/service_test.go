package appinit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/suite"

	"github.com/n-r-w/classifier-mcp/internal/config"
)

// classificationSuite exercises the public MCP contract with real internal wiring and a controlled HTTP endpoint.
type classificationSuite struct {
	// Suite supplies assertion and lifecycle state for the isolated entry-point cases.
	suite.Suite
}

// TestClassification runs entry-point scenarios with isolated MCP and HTTP sessions.
func TestClassification(t *testing.T) { t.Parallel(); suite.Run(t, new(classificationSuite)) }

// validAnswer combines a tied Choice, zero optional values, and a fractional ordered-scale Score.
const validAnswer = `{
  "model": "actual-model",
  "answers": {
    "team": {
      "type": "choice",
      "choice": "a",
      "probabilities": {
        "a": 0.5,
        "b": 0.5
      },
      "confidence": 0
    },
    "condition": {
      "type": "noul",
      "noul": 0
    },
    "urgency": {
      "type": "score",
      "score": 1.6,
      "probabilities": {
        "0": 0.1,
        "1": 0.2,
        "2": 0.7
      },
      "confidence": 0
    }
  },
  "usage": {
    "input_tokens": 0,
    "output_tokens": 0,
    "cost": 0
  }
}`

// validArguments exercises object and array guidance plus a category described by null.
const validArguments = `{
  "objects": [
    {
      "id": "one",
      "source": {
        "type": "text",
        "text": "private source"
      }
    }
  ],
  "task": "Classify",
  "questions": {
    "team": {
      "type": "choice",
      "instructions": {
        "ask": "Team?"
      },
      "criteria": {
        "a": null,
        "b": [
          "Other"
        ]
      }
    },
    "condition": {
      "type": "truth",
      "instructions": "Condition?"
    },
    "urgency": {
      "type": "score",
      "instructions": [
        "Urgency?"
      ],
      "criteria": [
        "low",
        {
          "level": "medium"
        },
        [
          "high"
        ]
      ]
    }
  }
}`

// connect assembles the application with explicit HTTP settings and connects an in-memory MCP client.
func (s *classificationSuite) connect(handler http.HandlerFunc, timeout time.Duration) *mcp.ClientSession {
	return s.connectWithConfig(handler, config.Config{
		Endpoint: "", Model: "configured-alias", APIKey: "secret-key", HTTPTimeout: timeout,
		MaxParallelism: 4, MaxAttempts: 3, RetryDelay: time.Second,
	})
}

// connectWithConfig supplies explicit operational limits to the real application assembly.
func (s *classificationSuite) connectWithConfig(handler http.HandlerFunc, cfg config.Config) *mcp.ClientSession {
	upstream := httptest.NewServer(handler)
	s.T().Cleanup(upstream.Close)
	cfg.Endpoint = upstream.URL
	application := New(cfg)
	a, b := mcp.NewInMemoryTransports()
	serverSession, err := application.Connect(s.T().Context(), a, nil)
	s.Require().NoError(err)
	s.T().Cleanup(func() { s.NoError(serverSession.Close()) })
	identity := &mcp.Implementation{
		Name:        "test-client",
		Version:     "dev",
		Title:       "",
		Description: "",
		WebsiteURL:  "",
		Icons:       nil,
	}
	client := mcp.NewClient(identity, nil)
	session, err := client.Connect(s.T().Context(), b, nil)
	s.Require().NoError(err)
	s.T().Cleanup(func() { s.NoError(session.Close()) })
	return session
}

// call submits raw arguments through tools/call, preserving the SDK validation and envelope paths.
func (s *classificationSuite) call(session *mcp.ClientSession, arguments string) *mcp.CallToolResult {
	params := &mcp.CallToolParams{
		Meta:           nil,
		Name:           "classify",
		Arguments:      json.RawMessage(arguments),
		InputResponses: nil,
		RequestState:   "",
	}
	result, err := session.CallTool(s.T().Context(), params)
	s.Require().NoError(err)
	return result
}

// structured checks matching MCP representations and excludes the submitted source from success entries.
func (s *classificationSuite) structured(result *mcp.CallToolResult) map[string]any {
	s.Require().NotNil(result.StructuredContent)
	data, err := json.Marshal(result.StructuredContent)
	s.Require().NoError(err)
	var object map[string]any
	s.Require().NoError(json.Unmarshal(data, &object))
	s.Require().Len(result.Content, 1)
	text, ok := result.Content[0].(*mcp.TextContent)
	s.Require().True(ok)
	s.JSONEq(string(data), text.Text)
	for _, entry := range object["results"].([]any) {
		outcome := entry.(map[string]any)
		if _, successful := outcome["answers"]; successful {
			success, marshalErr := json.Marshal(outcome)
			s.Require().NoError(marshalErr)
			s.NotContains(string(success), "private source")
		}
	}
	return object
}

// TestAssessmentsAndSchemas checks every assessment projection, structured guidance, and reported zero values.
func (s *classificationSuite) TestAssessmentsAndSchemas() {
	session := s.connect(func(w http.ResponseWriter, r *http.Request) {
		s.Equal(http.MethodPost, r.Method)
		s.Equal("Bearer secret-key", r.Header.Get("Authorization"))
		s.Equal("application/json", r.Header.Get("Content-Type"))
		var body map[string]any
		if !s.NoError(json.NewDecoder(r.Body).Decode(&body)) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		s.Equal("configured-alias", body["model"])
		s.Equal(map[string]any{"task": "Classify", "content": "private source"}, body["state"])
		var input map[string]any
		s.NoError(json.Unmarshal([]byte(validArguments), &input))
		input["questions"].(map[string]any)["condition"].(map[string]any)["type"] = "noul"
		s.Equal(input["questions"], body["questions"])
		_, err := w.Write([]byte(validAnswer))
		s.NoError(err)
	}, time.Minute)
	tools, err := session.ListTools(s.T().Context(), nil)
	s.Require().NoError(err)
	s.Require().Len(tools.Tools, 1)
	s.Equal("classify", tools.Tools[0].Name)
	schemaBytes, err := json.Marshal(tools.Tools[0].OutputSchema)
	s.Require().NoError(err)
	var schema jsonschema.Schema
	s.Require().NoError(json.Unmarshal(schemaBytes, &schema))
	resolved, err := schema.Resolve(nil)
	s.Require().NoError(err)
	for _, mode := range []string{"default", "compact", "full"} {
		s.Run(mode, func() {
			var input map[string]any
			s.Require().NoError(json.Unmarshal([]byte(validArguments), &input))
			if mode != "default" {
				input["result_mode"] = mode
			}
			data, err := json.Marshal(input)
			s.Require().NoError(err)
			result := s.call(session, string(data))
			s.Require().False(result.IsError)
			output := s.structured(result)
			s.Require().NoError(resolved.Validate(output))
			results := output["results"].([]any)
			s.Require().Len(results, 1)
			object := results[0].(map[string]any)
			s.Equal("one", object["id"])
			answers := object["answers"].(map[string]any)
			team := answers["team"].(map[string]any)
			score := answers["urgency"].(map[string]any)
			s.InDelta(float64(0), team["confidence"], 0)
			s.InDelta(0.5, team["probability"], 0)
			s.Equal(map[string]any{"truth": float64(0)}, answers["condition"])
			s.InDelta(1.6, score["score"], 0)
			s.InDelta(float64(0), score["confidence"], 0)
			if mode != "full" {
				s.Len(team, 3)
				s.Len(score, 2)
			} else {
				s.Len(team, 4)
				s.Len(score, 3)

			}
		})
	}
}

// TestArgumentsFailBeforeHTTP rejects malformed shared definitions without contacting the endpoint.
func (s *classificationSuite) TestArgumentsFailBeforeHTTP() {
	var calls atomic.Int64
	session := s.connect(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, err := w.Write([]byte(validAnswer))
		s.NoError(err)
	}, time.Minute)
	invalid := []string{
		`{}`, `{
  "objects": [],
  "task": "x",
  "questions": {}
}`,
		`{
  "objects": [
    {
      "id": "a",
      "source": {
        "type": "text",
        "text": null
      }
    }
  ],
  "task": "x",
  "questions": {
    "q": {
      "type": "truth",
      "instructions": "x"
    }
  }
}`,
	}
	changes := []func(map[string]any){
		func(v map[string]any) { v["task"] = " \t" },
		func(v map[string]any) {
			v["questions"].(map[string]any)["condition"].(map[string]any)["type"] = "noul"
		},
		func(v map[string]any) { v["result_mode"] = nil },
		func(v map[string]any) { v["result_mode"] = "unknown" },
		func(v map[string]any) { v["extra"] = true },
		func(v map[string]any) { v["objects"] = []any{v["objects"].([]any)[0], v["objects"].([]any)[0]} },
		func(v map[string]any) {
			v["objects"].([]any)[0].(map[string]any)["source"].(map[string]any)["path"] = "file"
		},
		func(v map[string]any) {
			v["questions"].(map[string]any)["condition"].(map[string]any)["criteria"] = map[string]any{"true": "yes"}
		},
		func(v map[string]any) {
			v["questions"].(map[string]any)["urgency"].(map[string]any)["criteria"] = []any{false}
		},
		func(v map[string]any) {
			v["questions"].(map[string]any)["team"].(map[string]any)["criteria"] = map[string]any{"": "x"}
		},
		func(v map[string]any) { v["questions"].(map[string]any)["team"].(map[string]any)["instructions"] = " " },
	}
	for _, change := range changes {
		var input map[string]any
		s.Require().NoError(json.Unmarshal([]byte(validArguments), &input))
		change(input)
		data, err := json.Marshal(input)
		s.Require().NoError(err)
		invalid = append(invalid, string(data))
	}
	for _, input := range invalid {
		result := s.call(session, input)
		s.Require().True(result.IsError)
		s.Nil(result.StructuredContent)
		s.Require().Len(result.Content, 1)
		s.NotEmpty(result.Content[0].(*mcp.TextContent).Text)
	}
	s.Equal(int64(0), calls.Load())
}

// TestAtomicResponseErrorsAndConfidenceOmission checks used response fields and optional confidence presence.
func (s *classificationSuite) TestAtomicResponseErrorsAndConfidenceOmission() {
	cases := []struct {
		// name identifies the response failure exercised by the subtest.
		name string
		// body is the complete external HTTP response text.
		body string
		// status supplies the HTTP response status independently of body validation.
		status int
	}{
		{name: "missing answer", body: `{
  "model": "actual",
  "answers": {}
}`, status: 200},
		{name: "malformed", body: `not JSON private source secret-key`, status: 200},
		{name: "upstream", body: `{
  "error": {
    "message": "failure private source secret-key",
    "code": 429
  }
}`, status: 429},
	}
	changes := []func(map[string]any){
		func(v map[string]any) {
			v["answers"].(map[string]any)["team"].(map[string]any)["probabilities"] = map[string]any{"a": 1, "b": nil}
		},
		func(v map[string]any) { v["answers"].(map[string]any)["condition"].(map[string]any)["score"] = nil },
		func(v map[string]any) {
			v["answers"].(map[string]any)["unexpected"] = map[string]any{"type": "noul", "noul": 0}
		},
		func(v map[string]any) { v["answers"].(map[string]any)["condition"].(map[string]any)["noul"] = nil },
		func(v map[string]any) { v["answers"].(map[string]any)["condition"].(map[string]any)["score"] = 0 },
		func(v map[string]any) { v["answers"].(map[string]any)["team"].(map[string]any)["choice"] = "unknown" },
		func(v map[string]any) {
			v["answers"].(map[string]any)["team"].(map[string]any)["probabilities"] = map[string]any{"a": 0.5}
		},
		func(v map[string]any) { v["answers"].(map[string]any)["team"].(map[string]any)["confidence"] = 2 },
	}
	for _, change := range changes {
		var body map[string]any
		s.Require().NoError(json.Unmarshal([]byte(validAnswer), &body))
		change(body)
		data, err := json.Marshal(body)
		s.Require().NoError(err)
		cases = append(cases, struct {
			// name identifies the response failure exercised by the subtest.
			name string
			// body is the complete external HTTP response text.
			body string
			// status supplies the HTTP response status independently of body validation.
			status int
		}{name: "incompatible", body: string(data), status: 200})
	}
	for _, test := range cases {
		s.Run(test.name, func() {
			session := s.connect(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("X-Request-ID", "request-id")
				w.WriteHeader(test.status)
				_, err := w.Write([]byte(test.body))
				s.NoError(err)
			}, time.Minute)
			result := s.call(session, validArguments)
			s.Require().False(result.IsError)
			object := s.structured(result)["results"].([]any)[0].(map[string]any)
			s.Require().Len(object, 2)
			s.NotContains(object, "answers")
			cause, ok := object["error"].(string)
			s.Require().True(ok)
			s.NotEmpty(cause)
			if test.status == http.StatusTooManyRequests {
				s.Equal("failure private source secret-key", cause)
			}
		})
	}
	var body map[string]any
	s.Require().NoError(json.Unmarshal([]byte(validAnswer), &body))
	answers := body["answers"].(map[string]any)
	delete(answers["team"].(map[string]any), "confidence")
	answers["urgency"].(map[string]any)["confidence"] = nil
	data, err := json.Marshal(body)
	s.Require().NoError(err)
	session := s.connect(
		func(w http.ResponseWriter, _ *http.Request) { _, writeErr := w.Write(data); s.NoError(writeErr) },
		time.Minute,
	)
	result := s.call(session, validArguments)
	s.Require().False(result.IsError)
	projected := s.structured(result)["results"].([]any)[0].(map[string]any)["answers"].(map[string]any)
	s.NotContains(projected["team"], "confidence")
	s.NotContains(projected["urgency"], "confidence")
}

// TestPartialSuccessAndEmptyText preserves input order and successful empty-text classification after an overload.
func (s *classificationSuite) TestPartialSuccessAndEmptyText() {
	session := s.connect(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if !s.NoError(json.NewDecoder(r.Body).Decode(&body)) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if body["state"].(map[string]any)["content"] == "private source" {
			w.WriteHeader(529)
			_, err := w.Write([]byte(`{
  "error": "overloaded"
}`))
			s.NoError(err)
			return
		}
		s.Empty(body["state"].(map[string]any)["content"])
		_, err := w.Write([]byte(validAnswer))
		s.NoError(err)
	}, time.Minute)
	var input map[string]any
	s.Require().NoError(json.Unmarshal([]byte(validArguments), &input))
	input["objects"] = append(
		input["objects"].([]any),
		map[string]any{"id": "empty", "source": map[string]any{"type": "text", "text": ""}},
	)
	data, err := json.Marshal(input)
	s.Require().NoError(err)
	result := s.call(session, string(data))
	s.False(result.IsError)
	objects := s.structured(result)["results"].([]any)
	s.Len(objects, 2)
	s.Equal("overloaded", objects[0].(map[string]any)["error"])
	s.Contains(objects[1].(map[string]any), "answers")
	s.Equal("empty", objects[1].(map[string]any)["id"])
}

// TestConfiguredHTTPTimeout verifies that startup settings reach the real HTTP adapter.
func (s *classificationSuite) TestConfiguredHTTPTimeout() {
	session := s.connect(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte(validAnswer))
		s.NoError(err)
	}, time.Nanosecond)
	result := s.call(session, validArguments)
	s.Require().False(result.IsError)
	cause := s.structured(result)["results"].([]any)[0].(map[string]any)["error"].(string)
	s.Contains(cause, "deadline exceeded")
	s.Contains(cause, `Post "http://`)
}
