# Technical solution: File classification

## Problem statement

The agreed problem is recorded in [problem.md](problem.md). The approved requirements are recorded in [prd.md](prd.md).

The tool delegates reading and classification to the MCP server so that the main LLM can use classification outcomes without processing the source contents itself. Direct text input remains available.

## Proposed solution

### MCP interface

Expose exactly one tool, `classify`, through the existing stdio transport and the official Go MCP SDK. The [MCP contract supplement](#mcp-contract-supplement) defines its complete caller-visible interface and examples.

A call supplies a list of objects, a task, and a common set of classification questions. Each object has a distinct identity within the call. The response preserves the association between each object and its outcome, including repeated references to different parts of the same file.

Use typed input and output models at the MCP boundary. Keep the SDK's structured result and its text representation for compatibility. The SDK can serialize the same result in both representations; this does not establish that a client sends both copies to its LLM. Verify result rendering in the actual MCP client during integration rather than removing structured output based on an assumed token cost.

A single list-based tool avoids requiring a separate main-LLM tool call for every object or assessment type.

Covers FRQ-01 and FRQ-05.

### Content sources

Represent a source as either directly supplied text or a local file reference. A file reference can identify the whole file or a contiguous line range.

Line numbers start at 1, and both range boundaries are inclusive. A reference to lines 10 through 20 identifies exactly those lines. An omitted range identifies the whole file.

Use absolute paths directly. Resolve relative paths against the MCP server process's working directory. Do not interpret file references as remote URLs.

Read and interpret the source as text without checking its filename extension. Source acquisition errors belong to the corresponding object. Do not silently shorten a requested range or replace unreadable content with a different classification input.

The caller supplies fragment boundaries. The server does not resolve function names, parse language syntax, or reproduce the capabilities of a symbol search tool. Line ranges apply to source code and other text files alike.

Covers FRQ-02 and FRQ-03.

### System One integration

Use the System One API request and response contract, not a Jev-specific integration. Implement the HTTP boundary with `net/http` and private protocol DTOs rather than adding another SDK.

The full API endpoint URL, model identifier, and credentials come from process environment configuration. The model identifier and endpoint URL must be explicitly supplied; neither has a built-in default. An absent setting prevents startup and produces an error identifying the missing setting. Environment values may be inherited by the process or supplied by the MCP client's server configuration.

Send the configured model identifier to the endpoint without choosing a replacement model or applying Jev-specific alias rules. Endpoint implementations are responsible for interpreting their accepted model identifiers.

OpenRouter is a supported System One endpoint. Other endpoints must implement the same request and response contract. Models such as Liquid D1, Kev 4B, and Mercury Decide publicly document this compatibility. The integration does not assume that every model described as a classifier, or listed with a decisions modality, implements this API.

Use the three System One assessment types:

- Choice for selecting a category from caller-defined alternatives.
- Noul for the probability that a caller-defined condition holds.
- Score for assessment on a caller-defined ordered scale.

Question instructions and criteria come from the caller. The model input contains the acquired content, its source location when applicable, and the caller's task. The adapter maps internal classification models to the System One representation.

Do not treat common API compatibility as proof of identical model accuracy, calibration, or limits. Do not hard-code Jev's context limits or tokenizer into the generic client.

The alternative OpenRouter SDK introduces a dependency while leaving file acquisition, object orchestration, and result projection to this server. The direct HTTP client is sufficient for this bounded contract.

Covers FRQ-01 and FRQ-04 and the integration constraints in the PRD.

### Execution model

Send one System One request per object, with all requested questions about that object in the same request. Questions are independent and do not consume each other's answers.

Use bounded parallelism across model requests. The bound applies to the process, including concurrent MCP calls, rather than multiplying the number of active requests for each call. Respect cancellation while waiting for capacity and during HTTP operations.

Combining every object's contents into one model state is not selected. Every question would see the combined material, requiring additional references to distinguish objects and consuming a shared context window. Per-object requests match the supplied CodeQuality.yaml workflow and isolate unrelated contents and failures.

Do not persistently cache file contents or classification results. Each call uses the contents obtained for that call.

Covers FRQ-04 and FRQ-05.

### Results

Return compact assessments by default. Allow the caller to request full probability distributions.

A compact Choice retains the selected category, its probability, and model-provided confidence when present. A compact Score retains the scale assessment and model-provided confidence when present. A Noul already consists of one probability and does not need a separate compact representation.

Full results retain the probability distributions and the scale information needed to interpret them. Preserve the meanings of probability and confidence rather than using those terms interchangeably. An absent optional value is not a zero value.

Return the actual model reported by the provider and provider-reported usage information for interpretation and cost assessment. Missing usage or cost data is not treated as zero. Reported usage does not establish the cost of failed attempts for which the provider supplied no usage data.

The server does not choose decision thresholds, turn Noul probabilities into categorical booleans, invent categories, or generate explanations. These decisions remain with the caller.

Return classification data, object associations, and failure diagnostics, not source contents. Do not attach the classified code or file contents as MCP resources or include them as explanatory text.

Covers FRQ-04, FRQ-05, and NRQ-01.

### Errors and model limits

Preserve successful outcomes when another object fails. An object-level error identifies the affected object, the failed operation, the concrete cause, and available technical diagnostics. For upstream failures, preserve the HTTP status and diagnostic information returned by the endpoint rather than replacing them with a generic message.

Malformed overall requests and invalid shared classification definitions fail before model requests are made. Failures attributable to an individual source remain individual outcomes.

Do not automatically truncate content, summarize it, or split it into independently classified chunks. Combining chunk assessments can change the question being answered and would require an explicit aggregation policy. A provider rejection caused by its context limit is returned as an error; the caller can select a smaller fragment.

Use bounded retries for explicit HTTP 429 and 529 responses. Respect a supplied retry delay. Preserve the final cause and retry information when attempts are exhausted. Do not automatically repeat a request after an ambiguous network timeout, because the previous request may already have executed.

This selective retry policy handles explicit overload responses inside the MCP server without requiring an additional main-LLM interaction, while avoiding automatic repetition of requests with an unknown execution outcome.

Covers FRQ-05 and FRQ-06.

### Application boundaries

Use the existing project layers:

- `internal/server` owns MCP input and output DTOs, transport validation, and its use-case contract.
- `internal/usecases` orchestrates content acquisition, classification, and per-object outcomes through consumer-owned interfaces.
- `internal/adapters` implements local content acquisition and the System One HTTP client. Protocol DTOs remain private to the adapter.
- `internal/config` loads and validates environment configuration using the project's configuration conventions.
- `internal/appinit` assembles concrete dependencies and connects them to the existing entry point.

Keep JSON-tagged DTOs out of the use-case and domain layers. Interfaces and their method contract types belong to their consumers. Interface names follow the project's uppercase I prefix rule.

Represent source alternatives, assessment alternatives, and success or failure outcomes with typed models rather than independent flags that can contradict each other.

Validate external data at its owning boundary: configuration during startup, MCP input at the server boundary, file contents and ranges at the content boundary, and model responses at the HTTP boundary. An incomplete or incompatible model response produces a diagnostic error rather than a fabricated assessment.

Configuration is immutable after startup. Each MCP call owns its input and outcome state. Concurrent requests share only the concurrency control and a reusable HTTP client; they do not share mutable classification results.

Operational logging uses the global structured logger with context. Logs go to stderr so that stdout remains the MCP transport.

Covers FRQ-01 through FRQ-06 and NRQ-01.

## Overengineering and overspecification considerations

- Keep the existing stdio transport and layer structure. Do not introduce an HTTP-hosted MCP service for a local personal tool.
- Use one System One client. Do not add a provider registry or adapters for unrelated classifier APIs without a concrete requirement.
- Use caller-supplied line ranges instead of language parsers or an embedded LSP subsystem.
- Do not add persistent caches, automatic source discovery, generated explanations, model fallback, or automatic chunk aggregation.
- Define the public fields, alternatives, defaults, and error semantics in the MCP contract supplement. Go signatures and numeric concurrency, timeout, and retry settings remain implementation decisions. The required explicit model and endpoint configuration is not deferred.

## Open questions

No unresolved questions about the approved technical approach.

Live System One behavior and MCP client rendering have not been tested. These are integration verification items, not completed checks. Actual calls are required to establish a selected model's suitability for the caller's tasks.

## MCP contract supplement

This supplement specifies the proposed external contract for review. It does not describe an implemented tool. The examples contain illustrative classifier values, not results of live model calls.

### Tool inventory

- `classify`: evaluates a list of inline texts, local files, or local file fragments against a common task and a common set of questions. Returns one outcome per object without returning source contents.
- There are no separate tools for reading files, selecting categories, checking conditions, or scoring content. Reading a referenced source is part of `classify`.
- The server publishes `classify` through `tools/list` with an `inputSchema` and an `outputSchema`. The schemas describe the alternatives and fields below. They do not expose model, endpoint, credentials, concurrency, timeout, or retry controls as call arguments.

### Input parameters

The `arguments` object has four fields:

- `objects`: required non-empty array of classification objects.
- `task`: required non-empty string describing the common task and context. A string containing only whitespace is invalid. There is no default task.
- `questions`: required non-empty object mapping caller-defined question IDs to question definitions. IDs are non-empty, case-sensitive strings. Every object is evaluated against every question.
- `result_mode`: optional string with exactly two values, `compact` and `full`. Omission means `compact`. The mode applies to every object and question in the call and changes only the returned projection, not the model questions.

Each classification object contains:

- `id`: required non-empty, case-sensitive string, unique within `objects`. The caller supplies this identity; the server does not derive it from the path or text.
- `source`: required object with exactly one of the two source forms below.

A text source contains:

- `type`: required string with the value `text`.
- `text`: required string containing the text to classify. An empty string is allowed and is not replaced with other content.

A file source contains:

- `type`: required string with the value `file`.
- `path`: required non-empty string identifying a local path. Absolute paths are used directly. Relative paths are resolved against the server process's working directory.
- `lines`: optional object containing required integer fields `start` and `end`. Both are 1-based and inclusive. Omission means the whole file. There is no default boundary and no open-ended range.

For a requested range, `1 <= start <= end` must hold, and both boundaries must exist in the file. The input schema checks the boundary field types; the content boundary checks the numerical bounds per object. A bounds failure produces an error for that object rather than silently shortening the range. Two fragments from the same path have different object IDs, so their outcomes remain distinguishable.

The source forms are exclusive: `text` does not accept `path` or `lines`; `file` does not accept `text`. Defined input objects reject unknown fields. Caller-defined maps and structured question guidance retain their arbitrary keys. Optional fields are omitted when unused; JSON `null` is not an omission except for Choice criterion values, which explicitly allow `null`.

### Question definitions

Every question has required `type` and `instructions` fields:

- `type`: exactly one of `choice`, `noul`, and `score`.
- `instructions`: a string, JSON object, or JSON array describing the independent question and its context. A string containing only whitespace is invalid. Structured guidance is passed through as System One guidance, not interpreted as another source reference.

Question IDs associate answers with definitions. IDs are not classification criteria and do not replace instructions. Questions cannot refer to another question's answer.

A `choice` question also has required `criteria`:

- `criteria` is a non-empty object mapping category names to their descriptions.
- Category names are non-empty, case-sensitive strings. These names are the complete set of alternatives.
- Each description is a string, JSON object, JSON array, or `null`. A `null` description means that the category name supplies the guidance without a separate description.

A `noul` question has optional `criteria`:

- Omission means the condition is defined by `instructions` alone.
- When supplied, `criteria` is an object containing exactly `true` and `false`. Both values are strings, JSON objects, or JSON arrays describing when the condition holds and when it does not hold.
- The answer is the probability that the condition holds. The server does not apply a Boolean threshold.

A `score` question also has required `criteria`:

- `criteria` is a non-empty ordered array of level descriptions. Each description is a string, JSON object, or JSON array.
- Array order defines the scale. The first level has index 0, and the last level has index `len(criteria) - 1`.
- The result is a probability-weighted value across these indices. It can lie between levels and is not a selected category or a percentage.

The server does not impose Jev-specific limits on category counts, scale lengths, or context size. A configured endpoint's rejection of a question size or content size is an upstream error for the affected object.

### Mapping to System One

For each object, the server sends one HTTP request containing:

- `model`: the explicitly configured model identifier, unchanged.
- `questions`: the common question map with IDs, types, instructions, and criteria as defined above.
- `state`: an object containing `task` and the acquired `content`. For a file reference, it also contains `source` with the resolved `path` and the requested `lines` when present. Inline text has no file location.

For example, a referenced fragment becomes the following state. The content is transmitted to the classifier, not returned to the main LLM:

```json
{
  "task": "Assess whether support text reports a defect and how urgent it is.",
  "content": "Checkout returns an error after I click Pay.",
  "source": {
    "path": "/workspace/tickets.txt",
    "lines": {"start": 10, "end": 10}
  }
}
```

The adapter requires a response model identifier and one answer of the matching type for every requested question ID. Missing or unexpected question IDs and incompatible answer fields produce an object-level `invalid_response` error. A returned distribution must have exactly the question's category or level keys and numeric probabilities in `[0, 1]`. The server does not fabricate answers to repair an incomplete response.

### Output parameters

A completed list evaluation returns a structured object with one required field:

- `results`: an array with the same length and order as input `objects`. Each entry has the corresponding input `id` and exactly one of the success and error forms below. Parallel execution does not change this order.

A successful entry contains:

- `id`: required string copied from the input object.
- `status`: required string with the value `ok`.
- `model`: required string reported by the upstream response. It is not substituted with the configured alias.
- `answers`: required object keyed by exactly the input question IDs. Each answer has the matching assessment type and the selected result shape below.
- `usage`: optional object containing only available provider-reported `input_tokens`, `output_tokens`, and `cost` fields.

`input_tokens` and `output_tokens` are non-negative integers for that object's model request. `cost` is a non-negative number in the endpoint's documented billing unit; OpenRouter reports cost in credits. The server does not convert currencies or estimate costs. A missing or null upstream usage field is omitted. An empty usage object is omitted. A reported value of zero is retained, including zero token counts or zero cost. Usage is not a call-wide total and does not account for unreported failed attempts.

Successful entries do not contain `error`, source text, or a copy of the input task, questions, or file contents. The caller associates the `id` with its submitted source and retains the original scale criteria for interpreting compact Scores.

#### Choice results

In `compact` mode, each Choice answer contains:

- `type`: required string with the value `choice`.
- `choice`: required string naming the provider-selected category from the question's criteria.
- `probability`: required number in `[0, 1]`, taken from the provider's probability distribution at the selected category.
- `confidence`: optional provider-reported number in `[0, 1]`.

In `full` mode, the answer contains the same fields plus required `probabilities`, an object mapping every category name from the question to its probability in `[0, 1]`. The selected `probability` equals `probabilities[choice]`. The distribution describes mutually exclusive alternatives and sums to 1, subject to provider numeric rounding. The server preserves the provider's values rather than renormalizing them.

A Choice response without the distribution needed to obtain the selected probability is an `invalid_response` error even in compact mode. A response naming an unknown category is also invalid. The server preserves the provider's selection among tied highest probabilities rather than imposing its own tie-break rule.

#### Noul results

In both modes, each Noul answer contains exactly:

- `type`: required string with the value `noul`.
- `noul`: required number in `[0, 1]`, meaning the probability that the condition holds.

For `noul: 0.8`, the condition has reported probability 0.8 of holding and 0.2 of not holding. The server does not return a Boolean or generate a separate confidence statistic.

#### Score results

In `compact` mode, each Score answer contains:

- `type`: required string with the value `score`.
- `score`: required number in `[0, len(criteria) - 1]`, preserving the provider's probability-weighted assessment on the caller's scale.
- `confidence`: optional provider-reported number in `[0, 1]`.

In `full` mode, the answer contains the same fields plus:

- `probabilities`: required object mapping every level index, expressed as a decimal string, to its probability in `[0, 1]`. For three levels, the keys are exactly `0`, `1`, and `2`.
- `legend`: required object mapping those index strings to the corresponding descriptions from the input criteria. The descriptions preserve their string, object, or array form.

A Score distribution sums to 1, subject to provider numeric rounding. For probabilities `0: 0.1`, `1: 0.2`, and `2: 0.7`, the weighted score is `0 * 0.1 + 1 * 0.2 + 2 * 0.7 = 1.6`. The server does not round 1.6 to level 2. A missing distribution in full mode or an incompatible returned legend produces `invalid_response`. The full legend can be constructed from the supplied criteria; doing so does not invent classification data.

For Choice and Score, absent or null upstream `confidence` is omitted. A reported zero is retained. Confidence summarizes uncertainty across the distribution; it is not the selected category's probability. The server does not compute a replacement confidence or assume that every compatible model uses TypeSafe's confidence formula.

### Object errors

An unsuccessful entry contains:

- `id`: required string copied from the input object.
- `status`: required string with the value `error`.
- `error`: required diagnostic object. The entry has no `answers` or success `model` field.

The diagnostic object contains:

- `code`: required string from the closed set below.
- `operation`: required string, either `read_source` or `classify`.
- `message`: required string identifying the concrete cause. For example, a range error states the requested boundaries and the file's available line count.
- `http_status`: optional integer containing the upstream HTTP status when a response was received.
- `upstream_body`: optional string retaining the endpoint's error body or the incompatible response portion needed to diagnose a decoding or validation error.
- `upstream_request_id`: optional string containing an available upstream request identifier.
- `attempts`: optional positive integer counting HTTP attempts, including the first attempt. Present for errors after an HTTP attempt, including errors after exhausted retries; absent for source failures or cancellation before an attempt.
- `retry_after_seconds`: optional non-negative number containing the final supplied retry delay converted to seconds. A supplied zero remains zero.

The error codes and operations are:

- `invalid_source`, `read_source`: the supplied file path or line boundaries fail semantic validation.
- `source_read_failed`, `read_source`: the server cannot open, read, or interpret the referenced source as text. Preserve the operating-system or decoding cause.
- `upstream_error`, `classify`: the endpoint returns an unsuccessful HTTP status. Preserve status, concrete endpoint cause, and available diagnostics.
- `request_failed`, `classify`: the HTTP request fails without an endpoint response, including connection failures and ambiguous network timeouts.
- `invalid_response`, `classify`: the response cannot be decoded into the required System One answers or lacks data required by the selected result mode.
- `canceled`, either operation: the call's cancellation is observed during that operation. Cancellation before model capacity is acquired uses `classify`.

Diagnostics retain technical causes rather than replacing them with a generic error. Credentials and echoed source contents are excluded from diagnostics and logs. When an upstream body contains either, remove those portions while retaining its status, error code, message, and other diagnostic fields. Error responses do not attach source resources.

Each object is atomic: an incomplete answer set produces an error for that object, not a partly populated success entry. Failures of other objects do not remove successful entries. When an upstream response supplies usage even though the object fails, the error entry may also contain `usage` with the same fields and omission rules as a successful entry.

### Request errors and MCP envelope

The server distinguishes three boundaries:

- Protocol failure: an unknown tool or a malformed MCP `tools/call` message produces a JSON-RPC error rather than a classification result. For example, an unknown tool name is not an object error.
- Invalid overall arguments: missing required fields, incompatible JSON types, conflicting source forms, unknown defined fields, duplicate object IDs, or invalid common task or question definitions reject the whole call before file reads or model requests. The result has `isError: true` and a text diagnostic naming the offending argument and cause. It has no `results` or `structuredContent`. SDK input-schema failures follow this text-error path.
- Valid overall arguments: file path and line-boundary semantics, content acquisition, and model execution are evaluated per object. The server returns `structuredContent` containing `results`. The MCP `content` array contains one text block serializing that same object for compatibility.

For a completed list response, `isError` is false when at least one object succeeds. It is true when all objects fail. Partial success is therefore not a whole-call error: the caller inspects each entry's `status`. An omitted MCP `isError` means false under the MCP protocol.

The `outputSchema` describes the structured `results` object and its success/error and assessment alternatives. An all-failed list still conforms to this schema. Request errors have no structured output to validate. The SDK serializes typed output into both `structuredContent` and a text block when the handler has not supplied separate content.

MCP cancellation requests stop further work and cancel pending HTTP operations. A canceled client call is not guaranteed to receive a response. When the transport permits a final list response, preserve completed successes and return `canceled` entries for unfinished objects. An absent response after cancellation must not be interpreted as evidence that an upstream request did not run.

### Call and response examples

#### Mixed input and compact output

This call uses inline text, a whole file, and two different fragments of the same file. Omitting `result_mode` would have the same effect as `compact`.

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "classify",
    "arguments": {
      "objects": [
        {"id": "inline", "source": {"type": "text", "text": "Checkout fails after I click Pay."}},
        {"id": "whole", "source": {"type": "file", "path": "tickets.txt"}},
        {"id": "fragment-a", "source": {"type": "file", "path": "tickets.txt", "lines": {"start": 10, "end": 20}}},
        {"id": "fragment-b", "source": {"type": "file", "path": "tickets.txt", "lines": {"start": 30, "end": 40}}}
      ],
      "task": "Assess support text for defect reports and urgency.",
      "questions": {
        "team": {
          "type": "choice",
          "instructions": "Which team should handle this content?",
          "criteria": {"payments": "Checkout or billing", "other": "Other issues"}
        },
        "is_defect": {
          "type": "noul",
          "instructions": "Does this content report broken product behavior?",
          "criteria": {"true": "Broken or unexpected behavior", "false": "A question or feature request"}
        },
        "urgency": {
          "type": "score",
          "instructions": "How urgent is the reported issue?",
          "criteria": ["Can wait", "Fix soon", "Blocking work"]
        }
      },
      "result_mode": "compact"
    }
  }
}
```

For illustration, assume the file has 35 lines. The first three objects succeed. `fragment-b` fails because its requested end line is 40. The following is the complete `structuredContent`, not the outer JSON-RPC response:

```json
{
  "results": [
    {
      "id": "inline", "status": "ok", "model": "typesafe/jev-1.13-20260917",
      "answers": {
        "team": {"type": "choice", "choice": "payments", "probability": 0.9, "confidence": 0.8},
        "is_defect": {"type": "noul", "noul": 0.95},
        "urgency": {"type": "score", "score": 1.6, "confidence": 0.4}
      },
      "usage": {"input_tokens": 420, "output_tokens": 55, "cost": 0.00002}
    },
    {
      "id": "whole", "status": "ok", "model": "typesafe/jev-1.13-20260917",
      "answers": {
        "team": {"type": "choice", "choice": "other", "probability": 0.7},
        "is_defect": {"type": "noul", "noul": 0.4},
        "urgency": {"type": "score", "score": 0.8}
      },
      "usage": {"input_tokens": 800, "output_tokens": 55}
    },
    {
      "id": "fragment-a", "status": "ok", "model": "typesafe/jev-1.13-20260917",
      "answers": {
        "team": {"type": "choice", "choice": "payments", "probability": 0.5, "confidence": 0},
        "is_defect": {"type": "noul", "noul": 0},
        "urgency": {"type": "score", "score": 1, "confidence": 0}
      },
      "usage": {"input_tokens": 310, "output_tokens": 55, "cost": 0}
    },
    {
      "id": "fragment-b", "status": "error",
      "error": {"code": "invalid_source", "operation": "read_source", "message": "Requested lines 30 through 40 in tickets.txt; the file has 35 lines."}
    }
  ]
}
```

The examples distinguish omitted confidence and cost from reported zero values. Because three objects succeed, the outer MCP result has `isError: false`.

#### Full output

For the same call with `result_mode: "full"`, the following is the complete `structuredContent`. Choice adds every category probability. Score adds every level probability and the legend. Noul, object associations, usage, and the source error do not change.

```json
{
  "results": [
    {
      "id": "inline", "status": "ok", "model": "typesafe/jev-1.13-20260917",
      "answers": {
        "team": {"type": "choice", "choice": "payments", "probability": 0.9, "confidence": 0.8, "probabilities": {"payments": 0.9, "other": 0.1}},
        "is_defect": {"type": "noul", "noul": 0.95},
        "urgency": {"type": "score", "score": 1.6, "confidence": 0.4, "probabilities": {"0": 0.1, "1": 0.2, "2": 0.7}, "legend": {"0": "Can wait", "1": "Fix soon", "2": "Blocking work"}}
      },
      "usage": {"input_tokens": 420, "output_tokens": 55, "cost": 0.00002}
    },
    {
      "id": "whole", "status": "ok", "model": "typesafe/jev-1.13-20260917",
      "answers": {
        "team": {"type": "choice", "choice": "other", "probability": 0.7, "probabilities": {"payments": 0.3, "other": 0.7}},
        "is_defect": {"type": "noul", "noul": 0.4},
        "urgency": {"type": "score", "score": 0.8, "probabilities": {"0": 0.4, "1": 0.4, "2": 0.2}, "legend": {"0": "Can wait", "1": "Fix soon", "2": "Blocking work"}}
      },
      "usage": {"input_tokens": 800, "output_tokens": 55}
    },
    {
      "id": "fragment-a", "status": "ok", "model": "typesafe/jev-1.13-20260917",
      "answers": {
        "team": {"type": "choice", "choice": "payments", "probability": 0.5, "confidence": 0, "probabilities": {"payments": 0.5, "other": 0.5}},
        "is_defect": {"type": "noul", "noul": 0},
        "urgency": {"type": "score", "score": 1, "confidence": 0, "probabilities": {"0": 0.5, "1": 0, "2": 0.5}, "legend": {"0": "Can wait", "1": "Fix soon", "2": "Blocking work"}}
      },
      "usage": {"input_tokens": 310, "output_tokens": 55, "cost": 0}
    },
    {
      "id": "fragment-b", "status": "error",
      "error": {"code": "invalid_source", "operation": "read_source", "message": "Requested lines 30 through 40 in tickets.txt; the file has 35 lines."}
    }
  ]
}
```

These compact and full examples project the same hypothetical provider answers. Two separate live calls are not guaranteed to return identical assessments or usage.

#### Complete MCP envelope with a valid zero result

A smaller call illustrates the complete envelope without repeating the mixed-list payload:

```json
{
  "jsonrpc": "2.0", "id": 2, "method": "tools/call",
  "params": {
    "name": "classify",
    "arguments": {
      "objects": [{"id": "note", "source": {"type": "text", "text": "Please add a dark theme."}}],
      "task": "Identify defect reports.",
      "questions": {"is_defect": {"type": "noul", "instructions": "Does this report broken behavior?"}}
    }
  }
}
```

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "isError": false,
    "structuredContent": {
      "results": [{"id": "note", "status": "ok", "model": "typesafe/jev-1.13-20260917", "answers": {"is_defect": {"type": "noul", "noul": 0}}, "usage": {"input_tokens": 100, "output_tokens": 8, "cost": 0}}]
    },
    "content": [{"type": "text", "text": "{\"results\":[{\"id\":\"note\",\"status\":\"ok\",\"model\":\"typesafe/jev-1.13-20260917\",\"answers\":{\"is_defect\":{\"type\":\"noul\",\"noul\":0}},\"usage\":{\"input_tokens\":100,\"output_tokens\":8,\"cost\":0}}]}"}]
  }
}
```

#### Upstream failure and invalid overall input

The following is an individual `results` entry after bounded retries are exhausted. A list containing only error entries has `isError: true`, but still returns its structured list and matching text representation.

```json
{
  "id": "whole",
  "status": "error",
  "error": {
    "code": "upstream_error",
    "operation": "classify",
    "message": "System One returned HTTP 429 after 3 attempts: Rate limit exceeded.",
    "http_status": 429,
    "upstream_body": "{\"error\":{\"code\":429,\"message\":\"Rate limit exceeded\"}}",
    "attempts": 3,
    "retry_after_seconds": 2
  }
}
```

The attempt count is illustrative, not a selected retry limit. In contrast, a call whose objects reuse the ID `duplicate` fails as a whole before any source is read or any HTTP request is sent. Its MCP response has no structured classification output:

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "isError": true,
    "content": [{"type": "text", "text": "Invalid arguments: objects[1].id duplicates objects[0].id: duplicate."}]
  }
}
```

## References

- [Problem statement](problem.md).
- [Approved requirements](prd.md).
- Supplied CodeQuality.yaml workflow, particularly load_rules and find_candidates.
- [System One API reference](https://docs.typesafe.ai/api).
- [OpenRouter System One API](https://openrouter.ai/docs/api/api-reference/systemone/submit-a-system-one-request).
- [OpenRouter System One compatibility](https://openrouter.ai/docs/guides/community/typesafe-sdk).
- [Liquid D1](https://openrouter.ai/liquid/d1).
- [Kev 4B](https://openrouter.ai/jaredpalmer/kev-4b).
- [Mercury Decide](https://openrouter.ai/inception/mercury-decide:free).
- [TypeSafe probability and confidence semantics](https://docs.typesafe.ai/confidence).
- [OpenRouter usage accounting](https://openrouter.ai/docs/guides/guides/usage-accounting), including the provider's cost units.
- [Jev limitations](https://docs.typesafe.ai/model-jaggedness/jev-1.13), including irrelevant context and structural invariants.
- [MCP structured results and output schemas](https://modelcontextprotocol.io/specification/2025-11-25/server/tools).
- `github.com/modelcontextprotocol/go-sdk` v1.8.0, `mcp/server.go`, `toolForErr`: source of typed result serialization behavior.
