package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"time"
)

// TestStructuredNumbersAcrossStdio checks forwarding, HTTP legend validation, and both MCP output representations.
func (s *startupSuite) TestStructuredNumbersAcrossStdio() {
	const arguments = `{
  "objects":[{"id":"one","source":{"type":"text","text":"fixture source"}}],
  "task":"Classify the numeric identifiers without changing them.",
  "questions":{
   "team":{"type":"choice","instructions":{"ticket_id":9007199254740993},
    "criteria":{"a":{"entity_id":9007199254740995},"b":null}},
   "condition":{"type":"noul","instructions":[{"ticket_id":9007199254740993}],
    "criteria":{"true":{"entity_id":9007199254740995},"false":"Other"}},
   "urgency":{"type":"score","instructions":{"ticket_id":9007199254740993},
    "criteria":[{"level_id":9007199254740997},["Higher",{"level_id":9007199254740999}]]}
  },
  "result_mode":"full"
 }`
	var input map[string]any
	inputDecoder := json.NewDecoder(bytes.NewReader([]byte(arguments)))
	inputDecoder.UseNumber()
	s.Require().NoError(inputDecoder.Decode(&input))
	criteria := input["questions"].(map[string]any)["urgency"].(map[string]any)["criteria"].([]any)
	expectedLegend := map[string]any{"0": criteria[0], "1": criteria[1]}
	modes := []string{
		"returned legend", "derived legend", "equivalent numeric legend", "incompatible legend",
	}
	for _, mode := range modes {
		s.Run(mode, func() {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request map[string]any
				decoder := json.NewDecoder(r.Body)
				decoder.UseNumber()
				if !s.NoError(decoder.Decode(&request)) {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				s.Equal(input["questions"], request["questions"])
				score := map[string]any{
					"type":          "score",
					"score":         0.25,
					"probabilities": map[string]float64{"0": 0.75, "1": 0.25},
					"confidence":    0,
				}
				if mode == "returned legend" {
					score["legend"] = expectedLegend
				}
				if mode == "equivalent numeric legend" {
					score["legend"] = map[string]any{
						"0": map[string]any{"level_id": json.Number("9007199254740997.0")}, "1": criteria[1],
					}
				}
				if mode == "incompatible legend" {
					score["legend"] = map[string]any{
						"0": map[string]any{"level_id": json.Number("9007199254740996")},
						"1": criteria[1],
					}
				}
				answer := map[string]any{"model": "actual-model", "answers": map[string]any{
					"team": map[string]any{
						"type":          "choice",
						"choice":        "a",
						"probabilities": map[string]float64{"a": 0.5, "b": 0.5},
						"confidence":    0,
					},
					"condition": map[string]any{"type": "noul", "noul": 0},
					"urgency":   score,
				}}
				s.NoError(json.NewEncoder(w).Encode(answer))
			}))
			defer upstream.Close()
			ctx, cancel := context.WithTimeout(s.T().Context(), 30*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, s.binary)
			command.Env = startupEnv(upstream.URL, "configured-model")
			stdin, err := command.StdinPipe()
			s.Require().NoError(err)
			stdout, err := command.StdoutPipe()
			s.Require().NoError(err)
			s.Require().NoError(command.Start())
			defer func() { s.NoError(stdin.Close()); s.NoError(command.Wait()) }()
			encoder := json.NewEncoder(stdin)
			decoder := json.NewDecoder(stdout)
			read := func(id int) rpcResponse {
				for {
					var response rpcResponse
					s.Require().NoError(decoder.Decode(&response))
					if response.ID.OrEmpty() == id {
						return response
					}
				}
			}
			s.Require().
				NoError(encoder.Encode(json.RawMessage(`{
 "jsonrpc":"2.0","id":1,"method":"initialize",
 "params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"number-test","version":"dev"}}
}`)))
			initialized := read(1)
			s.Require().True(initialized.Error.IsNone())
			s.Require().
				NoError(encoder.Encode(json.RawMessage(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)))
			call := fmt.Sprintf(
				`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"classify","arguments":%s}}`,
				arguments,
			)
			s.Require().NoError(encoder.Encode(json.RawMessage(call)))
			response := read(2)
			s.Require().True(response.Error.IsNone())
			var result toolWireResult
			s.Require().NoError(json.Unmarshal(response.Result.OrEmpty(), &result))
			var structured map[string]any
			structuredDecoder := json.NewDecoder(bytes.NewReader(result.StructuredContent))
			structuredDecoder.UseNumber()
			s.Require().NoError(structuredDecoder.Decode(&structured))
			s.Require().Len(result.Content, 1)
			s.Equal("text", result.Content[0].Type)
			s.Equal(string(result.StructuredContent), result.Content[0].Text)
			object := structured["results"].([]any)[0].(map[string]any)
			if mode == "incompatible legend" {
				s.Require().True(result.IsError)
				s.Equal("error", object["status"])
				s.Equal("invalid_response", object["error"].(map[string]any)["code"])
				s.NotContains(object, "answers")
				return
			}
			s.Require().False(result.IsError)
			s.Equal("ok", object["status"])
			score := object["answers"].(map[string]any)["urgency"].(map[string]any)
			s.Equal(expectedLegend, score["legend"])
			s.Equal(json.Number("0"), score["confidence"])
			s.NotContains(result.Content[0].Text, "fixture source")
		})
	}
}
