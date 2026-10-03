package main

import (
	"encoding/json"

	"github.com/samber/mo"
)

// rpcResponse retains the original response JSON so the test observes server wire precision.
type rpcResponse struct {
	// ID associates a response with its positive request ID; notifications have no ID.
	ID mo.Option[int] `json:"id"`
	// Result is absent when the protocol reports an error instead.
	Result mo.Option[json.RawMessage] `json:"result"`
	// Error keeps protocol failures distinct from classification object errors.
	Error mo.Option[json.RawMessage] `json:"error"`
}

// toolWireResult exposes both MCP result representations without normalizing the structured object.
type toolWireResult struct {
	// IsError defaults to false when omitted by MCP.
	IsError bool `json:"isError"`
	// StructuredContent retains exact number tokens from the server's output object.
	StructuredContent json.RawMessage `json:"structuredContent"`
	// Content contains the compatibility text representation of the same result.
	Content []wireTextContent `json:"content"`
}

// wireTextContent is the textual content alternative expected for classification results.
type wireTextContent struct {
	// Type identifies the protocol content alternative; classify returns text.
	Type string `json:"type"`
	// Text carries serialized results rather than the submitted source text.
	Text string `json:"text"`
}
