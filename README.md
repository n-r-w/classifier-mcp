# Classifier MCP

A stdio MCP server that classifies inline text, local text files, and caller-selected file fragments through a System One HTTP endpoint. Successful results contain assessments without source contents. Each failed object has a concise text cause.

## Startup

Set these environment variables in the MCP client's server configuration or in the process environment:

- `SYSTEM_ONE_ENDPOINT`: required full HTTP(S) endpoint URL. No default. OpenRouter uses `https://openrouter.ai/api/v1/systemone`.
- `SYSTEM_ONE_MODEL`: required model identifier, passed unchanged to the endpoint. No default.
- `SYSTEM_ONE_HTTP_TIMEOUT`: positive duration for one complete HTTP request; default `60s`. Examples: `30s`, `2m`, `250ms`.
- `SYSTEM_ONE_API_KEY`: optional bearer credential. OpenRouter requires it; unauthenticated compatible endpoints can omit it.
- `SYSTEM_ONE_MAX_PARALLELISM`: positive integer limiting active model HTTP requests across all concurrent MCP calls; default `4`.
- `SYSTEM_ONE_MAX_ATTEMPTS`: positive integer counting the initial HTTP attempt and overload retries per object; default `3`. Set `1` to disable retries.
- `SYSTEM_ONE_RETRY_DELAY`: non-negative fallback duration after overload when no valid `Retry-After` is supplied; default `1s`. Set `0s` for an immediate fallback retry.

See [.env.example](.env.example). Endpoint URLs can include a query. Keep credentials in `SYSTEM_ONE_API_KEY`; endpoint user information and fragments are rejected. Missing endpoint or model settings stop startup with exit code 1.

Run `task build`, then configure the MCP client to launch `bin/classifier-mcp` (`bin/classifier-mcp.exe` on Windows). The executable inherits its environment; it does not read `.env`.

The server uses stdout for MCP and stderr for logs. Interrupt or terminate the process to stop it. Invalid operational settings stop startup with a diagnostic naming the setting.

Objects run in parallel, with one shared process-wide HTTP request bound. Defaults allow four active requests and at most three attempts per object to limit endpoint load. Only explicit HTTP `429` and `529` responses trigger retries. A valid `Retry-After` delay in seconds or as an HTTP date replaces the fallback delay, including a supplied zero. The server respects that delay without a cap and releases HTTP capacity while waiting. A past HTTP date means zero delay. Missing or invalid delay headers use `SYSTEM_ONE_RETRY_DELAY`.

`SYSTEM_ONE_HTTP_TIMEOUT` applies separately to each complete HTTP attempt. Capacity and retry waits observe call cancellation. A large supplied retry delay can keep a call waiting until it is canceled. Network failures, ambiguous timeouts, unrelated HTTP statuses, and incompatible model answers are returned without automatic retries.

## Connect to an agent

### Claude Code

Build the executable as described above. Replace `/absolute/path/to/classifier-mcp/bin/classifier-mcp` with its absolute path and `YOUR_OPENROUTER_API_KEY` with your OpenRouter key.

From a Bash or Zsh terminal, register the server for all your Claude Code projects:

```sh
claude mcp add --transport stdio --scope user classifier-mcp \
  --env SYSTEM_ONE_ENDPOINT=https://openrouter.ai/api/v1/systemone \
  --env 'SYSTEM_ONE_MODEL=~typesafe/jev-latest' \
  --env SYSTEM_ONE_API_KEY=YOUR_OPENROUTER_API_KEY \
  -- /absolute/path/to/classifier-mcp/bin/classifier-mcp
```

Alternatively, add this entry to `mcpServers` in your project's `.mcp.json`. This JSON configuration also works on Windows; use an executable path such as `"C:\\tools\\classifier-mcp.exe"` with JSON-escaped backslashes. Replace the executable path and key before starting Claude Code.

```json
{
  "mcpServers": {
    "classifier-mcp": {
      "type": "stdio",
      "command": "/absolute/path/to/classifier-mcp/bin/classifier-mcp",
      "args": [],
      "env": {
        "SYSTEM_ONE_ENDPOINT": "https://openrouter.ai/api/v1/systemone",
        "SYSTEM_ONE_MODEL": "~typesafe/jev-latest",
        "SYSTEM_ONE_API_KEY": "YOUR_OPENROUTER_API_KEY"
      }
    }
  }
}
```

Start a new Claude Code session. For a project `.mcp.json`, approve the server when prompted. Run `/mcp` and check that `classifier-mcp` is connected and exposes `classify`. You can also check the connection from a terminal:

```sh
claude mcp get classifier-mcp
```

For example, ask Claude: "Use classifier-mcp's classify tool to assess `/absolute/path/to/tickets.txt` for defect reports and urgency. Pass the file reference rather than reading its contents first."

Use absolute file paths in agent requests when the server's working directory is uncertain. Optional execution controls are listed in [Startup](#startup).

See the [Claude Code MCP documentation](https://code.claude.com/docs/en/mcp) for configuration scopes and server management.

### Other MCP agents

Configure a local stdio server with the executable's absolute path as its command, an empty argument list, and the same `SYSTEM_ONE_*` environment settings. The agent starts the process and communicates over stdin/stdout. `SYSTEM_ONE_ENDPOINT` is the classifier's HTTP endpoint; it is not an HTTP MCP server address. Use the agent's MCP connection panel to verify that the `classify` tool is available.

## `classify`

Supply a non-empty `objects` list, a non-blank `task`, and a non-empty `questions` map. Object IDs must be non-empty and unique within the call. Each object has exactly one source form:

- Inline text: `{"type": "text", "text": "content"}`. The string `text` is required; an empty string is allowed.
- Whole local file: `{"type": "file", "path": "tickets.txt"}`.
- Local file fragment: `{"type": "file", "path": "tickets.txt", "lines": {"start": 10, "end": 20}}`.

Absolute paths identify files directly. Relative paths use native filesystem resolution against the server process's working directory, including directory symlinks followed by `..`. Windows drive-relative and root-relative paths use native drive rules. Files are read as UTF-8 text, regardless of extension. File state sent to System One includes the resolved path and any caller-supplied range.

Both range boundaries are required integers, starting at 1 and inclusive. Every requested line must exist, and `start <= end` must hold. Lines end at LF; CRLF bytes and selected line terminators are preserved. A final terminator does not create another line. An empty file has zero lines and can be classified as a whole file. Each call reads its referenced content afresh.

The server reports unreadable files, invalid UTF-8, blank paths, and numerical range failures per object. It does not shorten ranges, cache content, parse code, retrieve URLs, or extract binary documents. Text sources reject `path` and `lines`; file sources reject `text`.

Each question has a non-empty ID, a `type`, and `instructions`. Instructions can be a non-blank string, an object, or an array. Numbers in structured instructions and criteria are passed unchanged, including identifiers larger than 2^53. Question types are:

- `choice`: requires a non-empty category-to-description `criteria` object. Descriptions can be strings, objects, arrays, or null. Compact results contain `choice`, `probability`, and optional provider `confidence`.
- `noul`: returns the probability that a condition holds as `noul`. Optional `criteria` must contain exactly `true` and `false`, with string, object, or array descriptions.
- `score`: requires a non-empty ordered `criteria` array of string, object, or array descriptions. Compact results contain a provider-reported `score` from 0 through the last level index, and optional provider `confidence`.

`result_mode` is `compact` by default, or `full`. Full Choice adds `probabilities`. Full Score adds `probabilities`, keyed by decimal level indices. The caller retains the submitted scale descriptions. Noul has the same shape in both modes. Optional fields must be omitted rather than supplied as null, except null Choice descriptions. Defined input objects reject unknown fields; guidance objects keep arbitrary keys.

Example arguments:

```json
{
  "objects": [
    {"id": "ticket", "source": {"type": "text", "text": "Checkout fails after Pay."}},
    {"id": "whole", "source": {"type": "file", "path": "tickets.txt"}},
    {"id": "fragment", "source": {"type": "file", "path": "tickets.txt", "lines": {"start": 10, "end": 20}}}
  ],
  "task": "Assess support tickets.",
  "questions": {
    "team": {"type": "choice", "instructions": "Which team owns this?", "criteria": {"payments": "Billing", "other": null}},
    "defect": {"type": "noul", "instructions": "Does this report broken behavior?"},
    "urgency": {"type": "score", "instructions": "How urgent is this?", "criteria": ["Can wait", "Fix soon", "Blocking"]}
  },
  "result_mode": "compact"
}
```

Results contain one entry per object in input order. A success contains exactly `id` and `answers`. An item failure contains exactly `id` and an `error` string with its concrete cause. The server returns the provider's values and optional confidence. Reported zero confidence remains present; missing or null confidence is omitted.

The server checks result types, required fields, numeric ranges, question IDs, selected categories, and distribution keys. The provider owns statistical calculations. For example, a reported Score of `1.97`, confidence `0.96`, and probabilities `{"0": 0, "1": 0.02, "2": 0.98}` are returned as supplied. Provider model, usage, and legend metadata are unused.

Invalid shared arguments, conflicting source forms, and unknown defined fields return a text-only tool error before file reads or HTTP work. Per-object source, network, provider, decoding, and cancellation failures remain independent results. For an HTTP error, the item contains the provider's actual message. The server adds no masking, JSON metadata envelope, or source resources.

MCP cancellation stops capacity waits, retry waits, and pending HTTP operations. A canceled client call may receive no response. When a final list response is available, the server preserves completed assessments and concrete cancellation causes for unfinished objects in input order.

Every processable batch returns the same `results` object in MCP `structuredContent` and one JSON text block, with `isError: false`. This includes batches where every object fails. Whole-call validation errors remain tool errors; unknown tools remain protocol errors. The tool publishes input and output schemas through `tools/list`.
