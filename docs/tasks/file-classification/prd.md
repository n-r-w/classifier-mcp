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
- Concrete error causes and available technical diagnostics.
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
- [x] FRQ-05: Return an identifiable outcome for each submitted object. An unsuccessful object does not suppress successful results for other objects.
  - Origin: source, the user's decision to preserve partial success.
  - Goal: Avoid repeating successful classification work.
  - Goal achievement: Partial. Preserves usable results when individual objects fail.
- [x] FRQ-06: For failures, preserve concrete causes and available technical diagnostics unchanged, without masking or filtering.
  - Origin: source, the user's explicit decision to retain unfiltered error diagnostics in this local, personal tool.
  - Goal: Enable the caller to understand and resolve failures.
  - Goal achievement: Partial. Avoids additional investigation caused by concealed error details.

### Non-functional requirements

- [x] NRQ-01: Return classification outcomes, interpretation data, object associations, and failure diagnostics. Successful outputs exclude source contents. Failure diagnostics preserve any source contents or credentials echoed by the endpoint.
  - Origin: formulated from the cost-reduction goal, then refined by the user's explicit exception for unfiltered error diagnostics.
  - Goal: Reduce the amount of source content processed by the main LLM on successful classification.
  - Goal achievement: Partial. Keeps source contents out of successful results while preserving complete failure diagnostics.

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
