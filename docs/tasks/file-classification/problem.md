# Problem statement

## Context

A main LLM uses classification results while working with files. Code quality checks are one example: classification identifies potential rule violations for the main LLM to verify.

## Observed problem

We need a classification tool that improves the main LLM's efficiency and reduces its cost of processing files. The project currently contains only a server scaffold, so this capability is not available through it.

## Affected audience

Users who rely on a main LLM to perform tasks involving files.

## Evidence

- The user explicitly identified improving the main LLM's efficiency and reducing its file-processing costs as the purpose of the tool.
- The supplied `CodeQuality.yaml` workflow demonstrates delegation: a script reads content, sends classification questions to Jev, and returns candidates and errors rather than file contents.

## Impact

Without a classification tool in this project, users must arrange classification outside this server. Processing content through the main LLM consumes its tokens; the supplied workflow demonstrates how delegated classification can avoid returning that content to the main LLM.

No numerical cost or efficiency baseline has been provided.

## Current state

The server starts over stdio and registers no tools. Its startup path is implemented in `cmd/classifier-mcp/main.go`, `internal/appinit/app.go`, and `internal/server/server.go`.

## Desired state

The main LLM works more efficiently and incurs lower costs when processing files because classification work can be delegated.

## Problem boundary

The problem concerns classification that supports the main LLM's work with files. Code quality checking is an example, not a restriction to source code. Actions based on classification results belong to the main LLM's broader task.

## Domain glossary

- Main LLM: the model responsible for the primary agent's task and for using classification results.
- Classification: evaluating content against task-specific criteria.
- Classifier: the model that performs classification.

## Open questions

None for the problem definition.
