# Code review: `phase/1-skeleton-providers` vs `main`

Date: 2026-10-04

Scope: diff of `main..HEAD` on branch `phase/1-skeleton-providers`.

Method: diff and file reading only. `go vet` and `go test` were not run because `rtk` failed to spawn them. Findings 1 and 2 were re-checked against the code and hold. The rest rest on the reviewer's reading and have not been re-verified.

Status: **all findings fixed** — see "Resolution" at the end. The second review pass (after the first round of fixes) is folded into that section.

## High severity

### 1. One-shot output corrupts on stream retry
`cmd/moca/oneshot.go:56`

On `EventReset` the handler only prints a notice to stderr. Text already written to stdout stays there, and the retried response is appended after it. This ignores the `llm.EventReset` contract, which says to discard everything streamed so far.

Failure scenario: the stream emits "The answer is" and then hits `ErrStall` or `ECONNRESET`. The retry wrapper emits `EventReset` and replays the whole response. `moca -p ... > out.txt` ends up with `The answer isThe answer is 42`.

Proposed fix: buffer stdout in one-shot mode and flush it on success, or drop the buffer on `EventReset`.

### 2. Truncated tool calls are reported as normal tool use
`internal/provider/openai_completions.go:199`

`if len(calls) > 0 { out.Stop = llm.StopToolUse }` overrides a `finish_reason` of `"length"`. A call cut off by `max_tokens` has invalid JSON arguments, but it is emitted as a normal tool call.

Failure scenario: the model hits `max_tokens` mid-arguments and the assembled args are `{"path":"/a`. The stop reason is forced to `StopToolUse`, and the agent loop tries to run a malformed call instead of handling truncation.

Proposed fix: only upgrade `StopEnd` to `StopToolUse`, and leave `StopLength` alone.

### 3. Streams that end early count as successful turns
`internal/provider/anthropic.go:160` (and the two OpenAI adapters)

None of the three SSE adapters checks for its terminal event (`message_stop`, `response.completed`, or `[DONE]`/`finish_reason`). `readSSE` returns nil at EOF, so a clean close mid-response is returned as a successful, truncated turn.

Failure scenario: a proxy or opencode gateway closes the connection cleanly partway through a response. `Stream` returns a partial `Message` with `Stop=StopEnd`. The agent treats the cut-off answer as complete, and the retry layer never sees an error to retry.

Proposed fix: return `io.ErrUnexpectedEOF` when no terminal event was seen, and make sure `retryable()` covers it.

### 4. Redacted or unsigned thinking blocks break every later turn
`internal/provider/anthropic.go:124`

`redacted_thinking` blocks are stored as ordinary thinking blocks with an empty signature, and the redacted `data` payload is never captured.

Failure scenario: a model returns a redacted block, or the history holds thinking from another provider with no signature. On the next turn `body()` sends `{"type":"thinking","thinking":"","signature":""}`, which Anthropic rejects with a 400. The error then repeats on every later turn of that session.

Proposed fix: keep redacted blocks and their `data`, send them back as `redacted_thinking`, and drop thinking blocks that have no valid signature.

## Medium severity

### 5. Anthropic request bodies can be invalid
`internal/provider/anthropic.go:61`

Messages whose blocks are all dropped or empty are sent with `content: []`. The completions adapter skips these but the Anthropic builder does not. `cache_control` is also placed on the last block whatever its type.

Failure scenario: a history message holds only an unsupported or empty block, or only a thinking block that is last in the final message. Anthropic returns 400 (`messages: content cannot be empty`, or `cache_control` not allowed on thinking blocks). The 400 is non-retryable and the whole turn fails.

Proposed fix: skip empty messages, and place `cache_control` on the last cacheable block.

### 6. Retry classification is too narrow and unbounded
`internal/provider/errors.go:48`

`retryable()` does not treat a bare `io.EOF`, connection refused, or HTTP/2 stream errors as transient. 408 is not retried. `Retry-After` is never capped.

Failure scenario: a pooled keep-alive connection is closed by the server, so POST returns `Post ...: EOF`, and the user gets a hard failure on a transient error. Separately, a 429 with `Retry-After: 3600` makes the retry loop in `retry.go` sleep for an hour, because `he.RetryAfter > wait` is accepted unbounded.

Proposed fix: add those cases to `retryable()` and cap the wait (for example at 60s).

### 7. `--effort off` does not turn thinking off
`internal/provider/catalog.go:92`

`glmThinking` maps `EffortOff` to `""`, and the adapters only send `reasoning_effort` when the value is non-empty. The server keeps its default thinking setting.

Failure scenario: `moca -p x --effort off` on `opencode-go/glm-5.3-flash` clamps to Off, `body()` skips `reasoning_effort`, and the server keeps reasoning. The user pays for thinking tokens they tried to disable.

Note: the `glmThinking` definition was not opened during re-checking.

Proposed fix: give `off` an explicit wire value, or use the provider's actual disable parameter.

### 8. Budget-mode thinking ignores effort
`internal/provider/catalog.go:91`

In budget thinking mode every effort level maps to `""`, and the thinking budget depends only on `MaxTokens`. `--effort low/medium/high/max` make no difference. `BudgetTokens` can also be at or above `max_tokens` for small `MaxOutput`.

Failure scenario: on `claude-haiku-4-5`, effort low and effort max both send `budget_tokens = MaxTokens-4096`. A custom or overridden model with `MaxOutput <= 1024` gets `budget_tokens=1024 >= max_tokens`, which the Anthropic API rejects with a 400.

Proposed fix: map effort levels to budget fractions, and clamp the budget below `max_tokens`.

### 9. A built-in provider model missing from the catalog passes validation but fails later
`internal/provider/registry.go:40`

A model declared under a built-in provider that is not in the catalog gets `Protocol ""`. It passes both `Validate` and `NewRegistry`, then fails in `Resolve`.

Failure scenario: the config has `providers.opencode-go.models.new-model: {contextWindow: 200000}` and `model: opencode-go/new-model`. `NewRegistry` succeeds, then `Resolve` fails with `provider opencode-go has no base URL for protocol ""`. The registry error text tells the user to declare the model under `providers.<name>.models`, which they did.

Proposed fix: reject a missing protocol in `Validate` or `NewRegistry` with a clear message.

## Low severity

### 10. Subcommands fail on a fresh install; SIGTERM is not handled
`cmd/moca/main.go:49`

The "no model configured" check runs before subcommand and TUI dispatch. The signal context also handles only `os.Interrupt`.

Failure scenario: a new user with no config runs `moca login anthropic` and gets `no model configured`, when the expected message is that the subcommand lands in a later phase. A SIGTERM from a supervisor or timeout kills the process without cancelling in-flight requests, and exit code 130 is never produced.

Proposed fix: move the model check after subcommand dispatch, and use `signal.NotifyContext` with `SIGTERM` as well.

## Suggested order

Findings 1-4 first. 1 and 2 are small, self-contained changes.

## Resolution

Fixed with a failing test first for each item; `go vet`, `gofmt` and `go test -race ./...` are clean (78 tests).

| # | Finding | Resolution |
|---|---|---|
| 1 | `-p` duplicates text on retry | stdout buffered, discarded on `EventReset` (`cmd/moca/oneshot.go`) |
| 2 | Truncated tool calls kept | kept with partial arguments + the provider's `length` stop; the phase-2 loop attaches the §6 "cut off — split the work" error result (final design, 2026-10-05 — matches the phase-2 loop contract) |
| 3 | Early EOF counted as success | fixed in the first round (`io.ErrUnexpectedEOF`) |
| 4 | Redacted/unsigned/foreign thinking | `replay.go`: verbatim only for the producing model with a signature, else `[prior reasoning]` text or dropped |
| 5 | Anthropic body hygiene | fixed in the first round |
| 6 | Retry classification | refused/EPIPE/aborted/HTTP-2 transient errors retryable; fractional and negative `Retry-After`; non-SSE 200 fails fast |
| 7 | `--effort off` ignored | `glmThinking[off] = "none"` — **unverified against the live endpoint** |
| 8 | Budget ignores effort | `BudgetTokens(maxTokens, effort)`, thinking omitted when `max_tokens < 2048` |
| 9 | Built-in provider model without protocol | check runs after the per-model `protocol` override |
| 10 | Subcommands / SIGTERM | fixed in the first round |

Second-pass additions: `-p` exits 1 with a diagnostic on `length` / `refusal` stops; a `tool_use` block that never completed is kept with a nil input (its error result comes from the phase-2 loop); replayed tool input is sanitized in all three adapters; the mid-stream retry is announced and backs off (it still does not count against the five attempts and is still limited to one).
