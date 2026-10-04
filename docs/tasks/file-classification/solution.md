# Technical solution: File classification

## Problem statement

The agreed problem is recorded in [problem.md](problem.md). The approved requirements are recorded in [prd.md](prd.md).

The tool delegates reading and classification to the MCP server so that the main LLM can use successful classification outcomes without processing the source contents itself. Direct text input remains available.

## Proposed solution

### MCP interface

Expose exactly one tool, `classify`, through the existing stdio transport and the official Go MCP SDK. The [MCP contract supplement](#mcp-contract-supplement) defines its complete caller-visible interface and examples.

A call supplies a list of objects, a task, and a common set of classification questions. Each object has a distinct identity within the call. The response preserves the association between each object and its outcome, including repeated references to different parts of the same file.

Use private typed input and output models at the MCP boundary. Register the raw SDK tool handler so that request guidance and result data are not normalized through floating-point values. Validate separate decoded copies against the published schemas; never forward those validation copies. Decode structured guidance with number-preserving JSON handling.

Serialize the typed result once and populate the MCP structured result and its text representation from that same JSON. Keeping both representations does not establish that a client sends both copies to its LLM. Verify result rendering in the actual MCP client during integration rather than removing structured output based on an assumed token cost.

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
- Noul (`noul`) for the HTTP probability that a caller-defined condition holds; MCP uses `truth`.
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

A compact Choice retains the selected category, its probability, and model-provided confidence when present. A compact Score retains the scale assessment and model-provided confidence when present. Truth consists of one probability in both modes.

Full results contain the requested probability distributions. The caller retains scale descriptions for interpretation. Preserve the meanings of probability and confidence rather than using those terms interchangeably. An absent optional value is not a zero value.

Return each successful object as `id` and `answers`, and each failed object as `id` and an `error` string. Preserve the provider's assessment values, selected Choice probability, optional confidence, and requested full distributions. The System One model remains mandatory configuration and request data. Provider response model, usage, and legend metadata have no caller projection.

The adapter checks format, types, required fields, finite numeric ranges, question association, and category/level keys. The provider owns statistical calculations. The server returns a reported Score of `1.97` with probabilities `0: 0`, `1: 0.02`, `2: 0.98` and confidence `0.96` as supplied. The server does not compare weighted means, probability totals, or the selected category against an argmax.

Successful results exclude source contents and criteria replay. Object error strings contain concrete causes. For an HTTP error, extract the endpoint's actual message without returning its metadata object. The server adds no source resources, masking, recursive filtering, truncation, replacement values, or telemetry.

Covers FRQ-04, FRQ-05, and NRQ-01.

### Errors and model limits

Preserve successful outcomes when another object fails. Each object-level error contains its input identity and concrete text cause. These outcomes remain normal batch results, even when every object fails.

Malformed overall requests and invalid shared classification definitions fail before model requests are made. Failures attributable to an individual source remain individual outcomes.

Do not automatically truncate content, summarize it, or split it into independently classified chunks. Combining chunk assessments can change the question being answered and would require an explicit aggregation policy. A provider rejection caused by its context limit is returned as an error; the caller can select a smaller fragment.

Use bounded retries for explicit HTTP 429 and 529 responses. Respect a supplied retry delay. Return the final concrete cause when attempts are exhausted. Retain only private state required for retry and cancellation decisions. Do not automatically repeat a request after an ambiguous network timeout, because the previous request may already have executed.

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

This supplement specifies the caller contract. The examples contain illustrative classifier values, not results of live model calls.

### Tool inventory

- `classify`: evaluates a list of inline texts, local files, or local file fragments against a common task and a common set of questions. Returns one outcome per object. Successful outcomes exclude source contents; failures contain concise concrete causes.
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
- `path`: required non-empty string identifying a local UTF-8 path. Use absolute paths. Relative paths are resolved against the server process's working directory.
- `lines`: optional object containing required integer fields `start` and `end`. Both are 1-based and inclusive. Omission means the whole file. There is no default boundary and no open-ended range.

For a requested range, `1 <= start <= end` must hold, and both boundaries must exist in the file. The input schema checks the boundary field types; the content boundary checks the numerical bounds per object. A bounds failure produces an error for that object rather than silently shortening the range. Two fragments from the same path have different object IDs, so their outcomes remain distinguishable.

The source forms are exclusive: `text` does not accept `path` or `lines`; `file` does not accept `text`. Defined input objects reject unknown fields. Caller-defined maps and structured question guidance retain their arbitrary keys. Optional fields are omitted when unused; JSON `null` is not an omission except for Choice criterion values, which explicitly allow `null`.

### Question definitions

Write `task`, `instructions`, and `criteria` in ASD-STE100, unless the classification needs another language. Object content retains its original language.

Every question has required `type` and `instructions` fields:

- `type`: exactly one of `choice`, `truth`, and `score`.
- `instructions`: a string, JSON object, or JSON array describing the independent question and its context. A string containing only whitespace is invalid. Objects and arrays contain structured guidance, passed to System One without changing numeric values, including identifiers larger than 2^53. It is not interpreted as another source reference.

Question IDs associate answers with definitions. IDs are not classification criteria and do not replace instructions. Questions cannot refer to another question's answer.

A `choice` question also has required `criteria`:

- `criteria` is a non-empty object mapping category names to their descriptions.
- Category names are non-empty, case-sensitive strings. These names are the complete set of alternatives.
- Each description is a string, JSON object, JSON array, or `null`. A `null` description means that the category name supplies the guidance without a separate description.

A `truth` question has optional `criteria`:

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
- `questions`: the common question map; the adapter maps MCP `truth` to HTTP `noul` and preserves IDs and guidance.
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

The adapter requires one answer of the matching HTTP assessment type for every requested question ID. Each returned distribution contains exactly the expected category or level keys with finite probabilities in `[0, 1]`. The configured model and question type remain mandatory System One request fields. Unused provider model, usage, and legend metadata do not affect answer acceptance.

### Output parameters

The structured object contains `results`, an array with the input length and order. Each entry is exactly one of:

- Success: `id` copied from the input and `answers` keyed by every input question ID.
- Failure: `id` copied from the input and a concise concrete `error` string.

Each object is atomic. If any required answer is absent or incompatible, its entry contains the cause and no partial answer set.

#### Choice results

Compact Choice contains `choice`, selected `probability` in `[0, 1]`, and optional provider `confidence` in `[0, 1]`. Full mode adds `probabilities` with every caller category key. The selected probability is the provider distribution entry for the selected category. Unknown categories and missing required distributions produce item errors. The server preserves the selected category and distribution values without total or argmax checks.

#### Truth results

Truth contains only `truth`, the reported probability in `[0, 1]`, in both modes. For `truth: 0.8`, the caller receives the reported likelihood 0.8.

#### Score results

Compact Score contains the reported `score` within `[0, len(criteria) - 1]` and optional provider `confidence` in `[0, 1]`. Full mode adds `probabilities`, keyed by exactly the decimal level indices. The caller retains the input scale descriptions.

For example, compact output can be `{"score": 1.97, "confidence": 0.96}`. Full output adds `"probabilities": {"0": 0, "1": 0.02, "2": 0.98}`. The server preserves all these values; it does not replace the score with a recomputed weighted mean.

Absent or null provider confidence is omitted. A reported zero remains present. Low confidence means uncertain; it is not the probability of correctness.

### Object errors

An item error is a string containing the actual source, network, provider, decoding, or cancellation cause. File range causes include requested boundaries and the available line count when known. HTTP failures use the provider message; transport errors preserve the concrete network cause. The caller receives no diagnostic envelope or verbose provider JSON dump.

The retry client keeps its HTTP status, attempt and retry-delay state privately. Source and model failures remain independent entries for every processable batch. No item error escalates the whole tool.

### Request errors and MCP envelope

The server distinguishes three boundaries:

- Protocol failure: an unknown tool or a malformed MCP `tools/call` message produces a JSON-RPC error rather than a classification result. For example, an unknown tool name is not an object error.
- Invalid overall arguments: missing required fields, incompatible JSON types, conflicting source forms, unknown defined fields, duplicate object IDs, or invalid common task or question definitions reject the whole call before file reads or model requests. The result has `isError: true` and a text diagnostic naming the offending argument and cause. It has no `results` or `structuredContent`. Server input-schema failures follow this text-error path.
- Valid overall arguments: file path and line-boundary semantics, content acquisition, and model execution are evaluated per object. The server returns `structuredContent` containing `results`. The MCP `content` array contains one text block serializing that same object for compatibility.

Every processable batch response has `isError: false`, including an all-failed batch. The caller reads each entry's `answers` or `error`. Whole-call failures remain for unprocessable overall input or an internal failure to produce the batch response. An omitted MCP `isError` means false under the protocol.

The `outputSchema` describes the structured `results` object and its success/error and assessment alternatives. An all-failed list still conforms to this schema. Request errors have no structured output to validate. The server validates a separate result copy and uses one serialization of the typed output for both `structuredContent` and the text block. The validation copy is not used as output, so validation data is never forwarded or substituted into results.

MCP cancellation requests stop further work and cancel pending HTTP operations. A canceled client call is not guaranteed to receive a response. When the transport permits a final list response, preserve completed successes and return concrete cancellation causes for unfinished objects. An absent response after cancellation must not be interpreted as evidence that an upstream request did not run.

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
        {"id": "whole", "source": {"type": "file", "path": "/workspace/tickets.txt"}},
        {"id": "fragment-a", "source": {"type": "file", "path": "/workspace/tickets.txt", "lines": {"start": 10, "end": 20}}},
        {"id": "fragment-b", "source": {"type": "file", "path": "/workspace/tickets.txt", "lines": {"start": 30, "end": 40}}}
      ],
      "task": "Assess support text for defect reports and urgency.",
      "questions": {
        "team": {
          "type": "choice",
          "instructions": "Which team should handle this content?",
          "criteria": {"payments": "Checkout or billing", "other": "Other issues"}
        },
        "is_defect": {
          "type": "truth",
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

For illustration, assume `/workspace/tickets.txt` has 35 lines. The first three objects succeed. `fragment-b` fails because its requested end line is 40. The following is the complete `structuredContent`, not the outer JSON-RPC response:

```json
{
  "results": [
    {
      "id": "inline",
      "answers": {
        "team": {
          "choice": "payments",
          "probability": 0.9,
          "confidence": 0.8
        },
        "is_defect": {
          "truth": 0.95
        },
        "urgency": {
          "score": 1.6,
          "confidence": 0.4
        }
      }
    },
    {
      "id": "whole",
      "answers": {
        "team": {
          "choice": "other",
          "probability": 0.7
        },
        "is_defect": {
          "truth": 0.4
        },
        "urgency": {
          "score": 0.8
        }
      }
    },
    {
      "id": "fragment-a",
      "answers": {
        "team": {
          "choice": "payments",
          "probability": 0.5,
          "confidence": 0
        },
        "is_defect": {
          "truth": 0
        },
        "urgency": {
          "score": 1,
          "confidence": 0
        }
      }
    },
    {
      "id": "fragment-b",
      "error": "requested lines 30 through 40 in /workspace/tickets.txt; the file has 35 lines"
    }
  ]
}
```

The examples distinguish omitted confidence from reported zero. The outer MCP result has `isError: false` for every processable batch.

#### Full output

For the same call with `result_mode: "full"`, the following is the complete `structuredContent`. Choice adds every category probability. Score adds every level probability. Truth, object associations, and the source error have the same shapes.

```json
{
  "results": [
    {
      "id": "inline",
      "answers": {
        "team": {
          "choice": "payments",
          "probability": 0.9,
          "confidence": 0.8,
          "probabilities": {
            "payments": 0.9,
            "other": 0.1
          }
        },
        "is_defect": {
          "truth": 0.95
        },
        "urgency": {
          "score": 1.6,
          "confidence": 0.4,
          "probabilities": {
            "0": 0.1,
            "1": 0.2,
            "2": 0.7
          }
        }
      }
    },
    {
      "id": "whole",
      "answers": {
        "team": {
          "choice": "other",
          "probability": 0.7,
          "probabilities": {
            "payments": 0.3,
            "other": 0.7
          }
        },
        "is_defect": {
          "truth": 0.4
        },
        "urgency": {
          "score": 0.8,
          "probabilities": {
            "0": 0.4,
            "1": 0.4,
            "2": 0.2
          }
        }
      }
    },
    {
      "id": "fragment-a",
      "answers": {
        "team": {
          "choice": "payments",
          "probability": 0.5,
          "confidence": 0,
          "probabilities": {
            "payments": 0.5,
            "other": 0.5
          }
        },
        "is_defect": {
          "truth": 0
        },
        "urgency": {
          "score": 1,
          "confidence": 0,
          "probabilities": {
            "0": 0.5,
            "1": 0,
            "2": 0.5
          }
        }
      }
    },
    {
      "id": "fragment-b",
      "error": "requested lines 30 through 40 in /workspace/tickets.txt; the file has 35 lines"
    }
  ]
}
```

These compact and full examples project the same hypothetical provider answers. Separate live calls can produce different assessments.

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
      "questions": {"is_defect": {"type": "truth", "instructions": "Does this report broken behavior?"}}
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
      "results": [
        {
          "id": "note",
          "answers": {
            "is_defect": {
              "truth": 0
            }
          }
        }
      ]
    },
    "content": [
      {
        "type": "text",
        "text": "{\"results\":[{\"id\":\"note\",\"answers\":{\"is_defect\":{\"truth\":0}}}]}"
      }
    ]
  }
}
```

#### Upstream failure and invalid overall input

The following is an individual `results` entry after bounded retries are exhausted. A list containing only error entries has `isError: false` and returns its structured list and matching text representation.

```json
{
  "id": "whole",
  "error": "Rate limit exceeded."
}
```

In contrast, a call whose objects reuse the ID `duplicate` fails as a whole before any source is read or any HTTP request is sent. Its MCP response has no structured classification output:

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "isError": true,
    "content": [{"type": "text", "text": "invalid arguments: objects[1].id duplicates an earlier id"}]
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
- [Jev limitations](https://docs.typesafe.ai/model-jaggedness/jev-1.13), including irrelevant context and structural invariants.
- [MCP structured results and output schemas](https://modelcontextprotocol.io/specification/2025-11-25/server/tools).
- `github.com/modelcontextprotocol/go-sdk` v1.8.0, `mcp/server.go`, `Server.AddTool`, and `mcp/protocol.go`, `CallToolResult`: raw tool registration and MCP envelope transport.
