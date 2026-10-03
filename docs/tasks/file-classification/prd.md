# Idea: File classification

## Definitions

- Main LLM: the model responsible for the primary agent's task and for using classification results.
- Classification: evaluating content against task-specific criteria.
- Classifier: the model that performs classification.

## Context and problem

The agreed problem is recorded in [problem.md](problem.md).

## Goal

Improve the main LLM's efficiency and reduce its cost of processing files by delegating classification.

## Scenarios

- The main agent submits text with a task and classification criteria.
- The main agent references a local text file or part of a file instead of transmitting its contents. The tool obtains the content for classification.
- The main agent requests several independent assessments of the same object.
- Some objects cannot be processed. The main agent receives the successful results and the concrete causes of failures for the other objects.

## Scope and non-scope

### In scope

- Directly supplied text and references to local text files or their parts.
- Any text file, regardless of filename extension.
- Category selection, independent condition checks, and assessments on caller-defined ordered scales.
- Lists of objects with identifiable outcomes and partial success.
- Concise concrete error causes for independent objects.
- Local, personal use.

### Out of scope

- Retrieving content from remote URLs.
- Extracting text from binary documents or images.
- Editing files or performing actions based on classification results.
- Multi-user hosted deployment.

## Requirements

### Functional requirements

- [x] FRQ-01: Classify a caller-supplied list of objects according to the caller's task and criteria.
  - Origin: source, the original usage scenario and the user's approval of the requirements.
  - Goal: Delegate classification work.
  - Goal achievement: Partial. Provides the core operation used by the main agent.
- [x] FRQ-02: Allow the caller to supply text directly or reference a local file or part of a file. For references, the tool obtains the content.
  - Origin: source, the user's explicit clarification about input choice and the decision to use local sources.
  - Goal: Avoid requiring the main LLM to read and transmit file contents.
  - Goal achievement: Partial. Enables delegated reading while preserving direct text input.
- [x] FRQ-03: Accept text files without restrictions based on filename extension.
  - Origin: source, the user's clarification that any text file is in scope and extension restrictions are inappropriate.
  - Goal: Apply classification to text files without arbitrary exclusions.
  - Goal achievement: Partial. Includes source code and other text files.
- [x] FRQ-04: Support multiple independent assessments of each object: category selection, condition checks, and assessments on caller-defined ordered scales. Return compact or full assessment results, as selected by the caller.
  - Origin: source, the supplied CodeQuality.yaml scenario, the user's selection of all three assessment types, and approval of the requirements.
  - Goal: Delegate several classification decisions together.
  - Goal achievement: Partial. Covers the agreed assessment types without requiring the main LLM to make those judgments itself.
- [x] FRQ-05: Return ordered identifiable outcomes for every processable batch. Each object failure remains an independent result, including when every object fails.
  - Origin: source, the user's decision that independent item failures remain normal batch outcomes.
  - Goal: Avoid repeating successful classification work.
  - Goal achievement: Partial. Preserves usable results when individual objects fail.
- [x] FRQ-06: For each failed object, return its identity and a concise string containing the actual source, transport, provider, decoding, or cancellation cause.
  - Origin: source, the user's aligned decision to return concise causes without masking or verbose service metadata.
  - Goal: Enable the caller to understand and resolve failures.
  - Goal achievement: Partial. Avoids additional investigation caused by concealed error details.

### Non-functional requirements

- [x] NRQ-01: Return minimal object associations and assessments: success is `id` plus `answers`; failure is `id` plus an `error` string. Assessment fields contain the provider values, optional confidence, selected Choice probability, and requested full distributions. The server preserves those values without statistical cross-checks.
  - Origin: source, the user's approved minimal-response and provider-value decisions.
  - Goal: Reduce the amount of response content processed by the main LLM.
  - Goal achievement: Partial. The caller retains task, criteria, and source descriptions; the server returns the assessments and actual failure causes.

## Open questions

None for the agreed requirements. Technical choices remain for the technical solution.

## Technical supplement

User-specified integration constraints:

- Provide the tool through MCP.
- Use the System One API contract. The integration is not restricted to Jev.
- Require an explicitly configured model identifier and API endpoint URL through environment variables. Neither setting has a built-in default.
- Supply credentials through environment configuration, either inherited by the server process or supplied in the MCP client's server configuration.
- Support OpenRouter and other endpoints that implement the same System One request and response contract.

An absent model identifier or endpoint URL prevents server startup and produces an error identifying the missing setting. The server does not select a model or endpoint implicitly.

Compatibility is based on the System One API contract, not merely on a model being described as a classifier or supporting Chat Completions.

## References

- [Problem statement](problem.md).
- User decisions and clarifications recorded during requirements discussion.
- Supplied CodeQuality.yaml workflow, particularly load_rules, find_candidates, and verify_candidates, as an example of delegated classification.
- [System One](https://docs.typesafe.ai/concepts/system-one).
- [OpenRouter System One compatibility](https://openrouter.ai/docs/guides/community/typesafe-sdk).
- [Liquid D1](https://openrouter.ai/liquid/d1), [Kev 4B](https://openrouter.ai/jaredpalmer/kev-4b), and [Mercury Decide](https://openrouter.ai/inception/mercury-decide:free): published examples of other models using the System One contract.
