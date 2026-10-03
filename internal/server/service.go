// Package server owns the MCP classification contract.
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Service handles validated MCP classification requests.
type Service struct {
	// classifier evaluates validated objects through the consumer-owned contract.
	classifier IClassifier
	// input validates an isolated request copy before any model work.
	input *jsonschema.Schema
	// output validates an isolated result copy without rewriting guidance numbers.
	output *jsonschema.Schema
}

// New registers the inline classification tool with the published request and result schemas.
func New(classifier IClassifier) *mcp.Server {
	identity := &mcp.Implementation{
		Name: "classifier-mcp", Version: "dev", Title: "Classifier MCP", Description: "", WebsiteURL: "", Icons: nil,
	}
	srv := mcp.NewServer(identity, nil)
	handler := &Service{classifier: classifier, input: compileSchema(inputSchema), output: compileSchema(outputSchema)}
	tool := &mcp.Tool{
		Meta:         nil,
		Annotations:  nil,
		Name:         "classify",
		Title:        "",
		Icons:        nil,
		Description:  "Classify inline texts. result_mode: compact (default) or full. Question types: choice, noul, score.",
		InputSchema:  json.RawMessage(inputSchema),
		OutputSchema: json.RawMessage(outputSchema),
	}
	// The raw registration preserves both argument and output numbers without SDK normalization.
	srv.AddTool(tool, handler.classify)
	return srv
}

// compileSchema prepares an immutable validator from the public schema during startup.
func compileSchema(encoded string) *jsonschema.Schema {
	document, err := jsonschema.UnmarshalJSON(bytes.NewBufferString(encoded))
	if err != nil {
		panic(fmt.Errorf("decode tool schema: %w", err))
	}
	compiler := jsonschema.NewCompiler()
	const location = "urn:classifier-mcp:tool"
	if registerErr := compiler.AddResource(location, document); registerErr != nil {
		panic(fmt.Errorf("register tool schema: %w", registerErr))
	}
	schema, err := compiler.Compile(location)
	if err != nil {
		panic(fmt.Errorf("compile tool schema: %w", err))
	}
	return schema
}

// validateJSON checks a separate number-preserving copy; callers never forward this validation instance.
func validateJSON(encoded []byte, schema *jsonschema.Schema) error {
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	return schema.Validate(instance)
}

// classify decodes private typed DTOs and returns text-only argument errors or ordered structured outcomes.
func (s *Service) classify(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	result := &mcp.CallToolResult{
		Meta: nil, Content: nil, StructuredContent: nil, IsError: false, InputRequests: nil, RequestState: "",
	}
	if err := validateJSON(request.Params.Arguments, s.input); err != nil {
		result.SetError(fmt.Errorf("invalid arguments: %w", err))
		return result, nil
	}
	var input classifyInput
	if err := decodeJSON(request.Params.Arguments, &input); err != nil {
		result.SetError(err)
		return result, nil
	}
	command, err := parseCommand(input)
	if err != nil {
		result.SetError(err)
		return result, nil
	}
	outcomes := s.classifier.Classify(ctx, command)
	output := classifyOutput{Results: make([]any, 0, len(outcomes))}
	// An all-failed list is a tool error; any success keeps the list usable.
	result.IsError = true
	for i := range outcomes {
		if value, isSuccess := outcomes[i].Left(); isSuccess {
			result.IsError = false
			output.Results = append(output.Results, projectSuccess(value, command.Full))
		} else if value, isFailure := outcomes[i].Right(); isFailure {
			diagnostic := value.Diagnostic
			output.Results = append(output.Results, errorOutput{
				ID: value.ID, Status: statusError, Usage: projectUsage(value.Usage),
				Error: diagnosticOutput{
					Code:              diagnostic.Code,
					Operation:         diagnostic.Operation,
					Message:           diagnostic.Message,
					HTTPStatus:        diagnostic.HTTPStatus,
					UpstreamBody:      diagnostic.UpstreamBody,
					UpstreamRequestID: diagnostic.UpstreamRequestID,
					Attempts:          diagnostic.Attempts,
					RetryAfterSeconds: diagnostic.RetryAfterSeconds,
				},
			})
		}
	}
	encoded, err := json.Marshal(output)
	if err != nil {
		return nil, fmt.Errorf("encode tool result: %w", err)
	}
	if validationErr := validateJSON(encoded, s.output); validationErr != nil {
		return nil, fmt.Errorf("validate tool result: %w", validationErr)
	}
	// Both representations use the same serialization rather than the schema-validation copy.
	result.StructuredContent = json.RawMessage(encoded)
	result.Content = []mcp.Content{&mcp.TextContent{Text: string(encoded), Meta: nil, Annotations: nil}}
	return result, nil
}
