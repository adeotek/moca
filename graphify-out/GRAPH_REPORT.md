# Graph Report - moca  (2026-10-14)

## Corpus Check
- 399 files · ~468,933 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 12 file(s) not represented in the graph (top: (none) 6, .jsonc 4, .v2 1)

## Summary
- 3965 nodes · 17335 edges · 256 communities (107 shown, 149 thin omitted)
- Extraction: 82% EXTRACTED · 18% INFERRED · 0% AMBIGUOUS · INFERRED: 3161 edges (avg confidence: 0.88)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `b1249a7d`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- go_pkg_strings
- testing.T
- newScript
- run
- newFakeAS
- SPECS.md — moca's living normative spec (16 numbered sections, updated with every behavioral PR)
- newAgentModel
- newTestModel
- auto_test.go
- model
- internal/provider/auth.go credential store — ~/.config/moca/auth.json (file 0600, dir 0700; the write forces 0600 even over a stale tmp file and the rename deliberately replaces a symlink rather than following it): providers.<name> holds the OAuth token (access/refresh, expiry, account/client id, id token, email, scopes) and apiKeys.<name> holds API keys from /login or moca login (validated: empty values and control bytes refused, so a credential can never smuggle a newline into a header); atomic writes (temp + rename)
- Parse
- Phase-3 gate (2026-10-05): a mock-driven TUI session in a real tmux terminal against a scripted openai-completions server — status bar from the first frame, branch switch, streamed commits, items, slash commands (/help /cost /model /effort /hard /copy /show /compact), missing-key switch refused, newline hint, 120-line paste chip expand send, steering verified in the CAPTURED REQUEST PAYLOAD, approvals (a ran; A appended to shell.allow byte-preserving + .bak; rm prompted without [A]), /yolo on/off incl. a 60-column bar keeping YOLO, ! local, /undo, esc interrupting a tool, /clear; it found and fixed two real bugs (event-pipe deadlock on /yolo; styled prints mangled by Sanitize)
- Execution: calls of one turn run SEQUENTIALLY in emitted order; a failed call does not cancel the rest; every call gets exactly one tool_result; unknown tool → error listing available tools; invalid JSON arguments → error telling the model to split the work; a panic inside a tool is recovered by Registry.Run into an error result ('internal error in <tool>: … — this is a moca bug, not your input; the call may have partly applied …') instead of crashing the session (rev 21 — eval feature#3 crashed on the edit-diff bug)
- NewManager
- protocol.go
- Word
- Unit/integration: go test ./... -race — 685 top-level tests across 16 Go packages (count established via `go test ./... -count=1 -v | grep -c '^=== RUN'`); phases 1-5 keep passing; growth recorded: 585 before the web-tool/plan-mode suites → 613 rev-21 harness pass → 640 system-prompt template extraction → 641 overflow-per-session → 643 branch-review pins → 646 diagnostic-log suites → 679 web private-address + branch-review pins → 685
- Config
- anthropic_test.go
- Append-only JSONL transcript at ~/.local/share/moca/sessions/<date>-<slug>-<id8>.jsonl (file 0600, dir 0700); every entry has id, parentId (chain), type, ts; types: session (header: workdir, provider, model, EFFECTIVE effort, startedAt, mocaVersion, the FULL system prompt, yolo), message, tool_use, tool_result, compaction (summary + firstKeptEntryId + tokensBefore + usage + cumulative readFiles/modifiedFiles — its usage/cost count toward session totals and are restored on resume), snapshot, error, permission_mode, model_change (written by /model, /effort, /hard), elision (rev 21: {ids, tokens} — tool_result entry ids superseded later)
- 9.5 ACP server (`moca acp`)
- Summarize
- slash create-command — a starter command template written on the first TUI run into the global prompts dir (an existing file is never overwritten); running it has the agent create or update a saved slash command, personal ones in that directory asking to approve the write since it is outside the workdir and once per write never persisted, or project ones in the workdir moca prompts; new commands are usable as soon as the run finishes with no restart
- Model
- v0.1.0-alpha (2026-10-07)
- Plan mode (rev 20) in the tools: with Env.Plan set, write/edit reach only <workdir>/docs/plans/*.md (everything else → `refused: plan mode only writes docs/plans/*.md …`; the ask-write roots stay closed — nothing is asked) and a successful plan-file write/edit sets Env.PlanWrote; read/search/ls/web and shell/MCP stay available under the normal ladder (2026-10-09 follow-up: planning needs git log, builds, docs lookups); the failing-test banner/refusal is suppressed in plan mode (plan runs inspect, they don't fix); the confinement is NOT a permission — --yolo does not lift it; edit applies the plan-zone check BEFORE the ask-write approval (plan mode never asks)
- context.Context
- initialize
- Manager
- NewRegistry
- Agent
- loginState
- TempRoots
- Context and sessions landed in v0.1.0-alpha: a token-denominated context manager with reserve and keepRecent budgets, usage-anchored estimates, cut-point rules that never orphan a tool call, structured summaries with cumulative file tracking, overflow compact-and-retry, no-cache-write summary requests; JSONL session transcripts with a single-writer lock and 0600 mode carrying entry ids, thinking blocks, model changes and compaction entries; resume with id8 or last and the continue flag
- Store
- styleBlock
- update/update.go
- Approvals show the whole command: up to 6 lines under the question with overflow counted and ctrl-o paging all of it; the prompt used to show only the first line so approving python3 hid the heredoc it would run
- Phase 7 — subscription OAuth behind the DESIGN §3 policy gate: anthropic ships api_key only, openai ships SIWC token sharing on the openai-responses route with dynamic client registration, PKCE S256, nonce/RS256/JWKS validation, a 0600 token store with cross-process locked rotating refresh, moca login|logout; plus the automated ship gate and version stamping
- 15. Verification status & known items
- Config at config moca config.jsonc in JSONC with comments allowed honoring XDG CONFIG HOME; minimal example with model and modelHard plus providers blocks carrying auth and apiKey env references
- Seven frozen tools in v0.1.0-alpha: read, write, edit with exact-match-first and one whitespace-tolerant fallback with re-indentation and CRLF/BOM preservation, shell stateless with bash c and pwsh NoProfile and process-group kill, search pure Go ripgrep semantics gitignore-aware, ls, and the lazy mcp proxy
- Release - build packages, publish GitHub Release
- apiKey is optional in the config: a key stored with slash login or the CLI wins over the env VAR reference which stays the fallback so an env var set by an external process still works and slash logout restores it; the stored key applies from the running session's next request
- Input
- README.md - moca
- The status bar shows the branch of a fresh git init with no commit yet instead of a dash
- External tools - rtk, graphify, skills ecosystem
- Context pressure in the status bar: the percent turns yellow at 70 percent and red with a slash compact hint where auto-compaction fires
- Verified live endpoints: authorize/token/revoke URLs, JWKS RS256, resource https://api.openai.com/v1
- BASELINE.md
- serverEnv
- Request
- renderMarkdown
- Run
- RenderStatus
- cut_test.go
- 11.0 Modes, rules, auto, classifier (v0.4.0-beta)
- Slash commands list: model, effort, hard, yolo, plan, do, clear, resume, sessions, compact, cost, undo, copy, show, login, logout, help, exit with q and quit aliases; plus prompt templates where config moca prompts name md becomes slash name; typing slash opens a dropdown of every command and loaded template with up down pick tab complete enter run; the command echoes into the transcript like messages before its output; the first run drops a starter create-command template; bang command runs a command and feeds its output to the model and double bang runs it locally without telling the model; typing during a run steers it after the current tool results; slash plan with a request or plan flag turns a run into plan mode where the agent reads with shell, web and MCP stays available for investigation under their normal permission rules, writes an implementation plan under docs plans, and every write outside it is refused
- CLI landed in v0.1.0-alpha: moca for the TUI, moca p for one-shot with stdout the final text and exit codes 0 1 2 3 130, model effort approve no-approve yolo no-yolo resume continue version and help flags, moca login and logout, moca mcp import and index; the Makefile does version-stamped builds with git describe into ldflags, make test with race detection, and make release producing five cross-compiled binaries
- system-prompt.md
- Message
- Find
- File map
- 14. Agent loop
- webHTMLRender
- skills.LoadPrompts reads <dir>/<name>.md (optional frontmatter description/argument-hint; first dir wins a collision; sorted); ExpandPrompt substitutes $ARGUMENTS/$@ (all args), $1..$9 (whitespace-split) and $$ (literal $); Claude Code command files (allowed-tools/model keys ignored) and pi templates load unchanged (testdata/prompts/, TestEcosystemPromptsLoad); skills.SeedUserPrompts(config.PromptsDir()) materializes the embedded starter template on the first TUI run (best-effort, stderr warning on failure) — never overwriting an existing file (O_EXCL); the global prompts dir is a jail read root and ask-write root
- Render
- 11.3 The classifier (`Env.Judge`)
- `moca acp` — Agent Client Protocol server (design)
- Catalog and tooling in v0.1.1: ten more opencode-go models all live-verified including deepseek and kimi-k2.7-code and mimo and qwen variants; the Go version pinned once in go.mod; make targets mirrored as mise run tasks; make vet now includes the gofmt check; documentation trimmed to the two living specs
- rtk - token-compressed CLI proxy
- findTestFile
- 12. Skills & project instructions
- sync.Mutex
- 3.5.1 TUI additions (rev 14)
- endUpdate
- runReg
- moca/log_test.go
- SOURCES.md
- Claude Code skill-creator skill
- Sanitize
- `moca update [--check]` — replaces the running binary with the newest GitHub release package via internal/update
- retry_test.go
- shipgate/fixture/main.go
- 2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)
- .config
- net/http/httptest.Server
- ListFiles
- ToolSpec
- fetch_test.go
- test/evals — the rev 21 eval corpus: seven scenarios, each a fixture repo (committed base), prompt.txt, check.sh judged against the base commit (SESSION names the transcript) and optional flags; not in CI, same .env key; harness changes show their delta against BASELINE.md
- fixture/order.go
- quick_validate.py
- Items
- moca update replaces the installed binary with the latest GitHub release package: checks the newest published release skipping drafts, picks the package for the platform, verifies against checksums, and swaps the binary in place atomically with a symlinked launcher having its target replaced; sessions config and credentials never touched; check mode only reports; a dev or dirty build or one newer reports not newer; a platform without a package like darwin points at make release; runs before config is loaded so a broken config cannot block an update
- Create
- session
- install.sh script
- acp_test.go
- testOptions
- fixture/slug.go
- CI Workflow (gofmt · vet · build · test)
- permissions/shell.go
- Lookup
- Conn
- Claude Code frontend-design skill
- pi mcp-scripting skill
- evals/run.sh
- Commands: mise install provisions Go 1.27.1 pinned in go.mod with PATH export because go is not on the global PATH on every host; make build produces bin/moca version-stamped from git describe; make test runs go test with race and count; make vet runs go vet plus gofmt with both clean; make fmt; make release builds five cross-compiled binaries in dist; bin moca update with check compares with the latest GitHub release; mise run tasks mirror the make targets
- reference/codes.go
- shipgate/run.sh
- askRootChecker
- boundedWriter
- fakeCmds
- CI gate (.github/workflows/ci.yml)
- DESIGN.md - v1 contract/vision
- Git flow: main is PR-only (local .git/hooks/pre-push blocks pushes)
- Item
- Built-in rtk skill (SKILL.md)
- Arrange-Act-Assert with xUnit/NSubstitute
- stdioTransport
- bugfix/check.sh
- feature/check.sh
- largefile/check.sh
- plan/check.sh
- refactor/check.sh
- vague/check.sh
- web/check.sh
- AGENTS.md headline: one Go binary, a minimal TUI coding agent at github.com/adeotek/moca with binary name moca; 220 Go files across 12 internal packages
- Single package or test via go test with the package path run flag race count
- Strict internal import direction
- Cross-provider thinking-block transforms in the agent
- Richer activity row with tool operand, streamed-token estimate and queued steering count; pager search; an approval pending marker; shift-tab cycles effort; a desktop notification or bell for a finished long run or a waiting approval while the terminal is unfocused via tui.notify defaulting to osc9
- At path completes workdir files gitignore-aware; the path is sent and the model reads the file itself
- slash copy keeps the last answer when the final turn had no text; failed tool items get a red cross; thinking line count singular; the pager title clamped to one row showing the scroll position
- ctrl c on an empty draft says press ctrl c again to quit; inside the pager it now also clears the textarea so the draft does not come back on the next key
- Exit keeps the final status bar on screen and prints the session id8 with the resume hint
- Inline edit diffs: a successful edit prints its changed lines in red and green up to 8 under its item line
- Calmer layout: a blank row between tool items and the response, messages hanging their text, one-row item lines, bang command echoes, a per-machine shift-enter hint; scrollback colors follow the terminal profile with NO_COLOR honored and 256-color terminals getting approximated band colors instead of raw truecolor
- Markdown-lite responses: bold, code, headings, bullets, quotes and fenced code blocks in a darker band never word-wrapped instead of raw markdown; slash copy still copies the raw text
- Persistent prompt history in a per-workdir history jsonl file plus ctrl-r history search with type to filter and enter using the match without sending
- net/http.Client
- SOURCE.md
- Eval scenario `bugfix`: two bugs behind failing tests, tests must stay untouched
- Eval scenario `feature`: a --json flag across files + tests, the table output byte-identical
- Eval scenario `largefile`: a bug at line ~1410 of a 1423-line generated file, the table untouched
- Eval scenario `plan`: a --plan run must produce ≥3 checkbox steps naming fetch.go and change nothing else
- Eval scenario `refactor`: shared email validation in validate.go, the duplicate gone
- Eval scenario `vague`: "add input validation" — judged by CLI behaviour incl. below-absolute-zero
- Eval scenario `web`: the current Go release, checked live against go.dev with the web tool used
- --approve/--no-approve — explicit project trust for this run (default: no; --yolo implies yes unless --no-approve)
- --config <path> — dev/test config file override (hidden from help text)
- provider.NewRegistry(cfg, httpClient, notify) — builds the catalog (built-ins + config overrides/additions) and validates that model/modelHard resolve
- Known limitation: terminals that reflow on shrink (tmux, kitty, iTerm2, WezTerm, VTE) re-wrap old full-width frame rows before the renderer erases them, which can leave one duplicate rule row above the input box; the next frame is correct; compensating needs terminal-specific knowledge (a wrong guess erases scrollback)
- example.com/codes
- example.com/fetcher
- example.com/inventory
- example.com/shop
- example.com/slug
- example.com/temp
- example.com/totals
- github.com/adeotek/moca
- Install via go install github.com/adeotek/moca/cmd/moca at latest
- Optional TUI knob: tui notify osc9, bell or off — a desktop notification or bell when a run longer than 30 seconds ends or an approval is waiting, only while the terminal is unfocused
- .Run
- extractBuiltins
- .Run
- input_test.go
- Eval baseline — DESIGN rev 21 harness pass
- v0.1.1 (2026-10-08): polish, a credential store in the config dir, slash autocomplete, and a bigger catalog — still one binary, still no framework
- allowAll
- self-configure/SKILL.md
- create-command/SKILL.md
- acpgate/run.sh
- fileAskChecker
- Ollama failure modes: a server that is down fails at once with how to start it (no retry ladder); a model never pulled says ollama pull name; a key via slash login ollama is optional for servers behind an authenticating proxy; nothing is probed unless you opt in
- Competes on cost per task and transparency: prompt caching, windowed reads, diff-shaped results and token-denominated compaction keep a session cheap; MCP support that never floods the prompt via one 200-token lazy proxy; loads Agent-Skills SKILL.md files written for other tools unchanged; searches the web and fetches pages without another tool with keyless default; prefers external token-savers rtk and graphify when installed
- Credentials live in config moca auth json mode 0600 beside the config file now rather than the data dir with a v0.1 store migrated automatically on first run; API keys and OAuth tokens share the file
- Docs pointers: docs/specs/external-tools.md covers rtk and graphify allowlist rules and the cross-tool skills and prompts ecosystem; docs/specs/oauth-verification.md is the binding OAuth policy gate with anthropic API key only and openai subscription login; SPECS.md still cites docs/reviews review docs that were deleted so do not chase them
- Exactly eight frozen tools with the web tool joining in DESIGN rev 19; schemas are golden at internal/tools/testdata/schemas.golden.json; changing a schema is a v2 discussion and description-only edits from rev 21 are batched into one golden update because each costs every user a prompt-cache reset
- Layout: entrypoint cmd/moca/main.go with run as the testable core; sibling files handle the p flag, the TUI, login, mcp and update subcommands
- graphify workflow in this repo: a knowledge graph at graphify-out with god nodes and community structure and cross-file relationships; for codebase questions first run graphify query when the graph json exists, graphify path for relationships, graphify explain for focused concepts; dirty graphify-out files are expected after hooks or incremental updates and are not a reason to skip; use the wiki index for broad navigation when it exists; read the graph report only for broad architecture review; run graphify update after modifying code as an AST-only pass with no API cost; the installed graphify skill or instructions come first whenever the user types the graphify slash command
- Ground truth rules: docs/specs/SPECS.md is the living normative description of what is implemented today covering the CLI contract, config keys, wire behavior, tools and agent loop; update it in the same PR as any behavior change per its section 16; when it and DESIGN.md disagree SPECS wins for the current code; DESIGN.md is the v1 contract and README.md is user-facing
- Logging: packages call standard log/slog directly; internal applog with the file handler, redaction and caps is imported only by cmd/moca; log metadata only, never prompts, file contents, command text or secrets per SPECS section 13.5
- Ship gate not in CI needing the repo env file with the OpenCode Go key and a build first: make build then bash test/shipgate/run.sh; the checker carries a build tag so plain go test skips it; several internal tools tests carry a not-windows build tag
- config.Standardize for JSONC preserves byte offsets so decode errors keep the original line col; the internal config testdata example jsonc file is the byte-exact DESIGN section 12 fixture
- The system prompt is built once per run and stored in the session, never rebuilt; its fixed text is the embedded internal agent system-prompt.md file with token slots rendered at start
- Thinking blocks and signatures are model-bound; cross-provider transforms happen in the agent before each request per SPECS section 14
- slash cost uses the bar's compact token format and breaks the input down into fresh, cache read and cache write with subscription for OAuth
- Development setup: mise install provisions Go 1.27.1 pinned once in go.mod; make build produces bin/moca version-stamped from git describe; make test runs the full test suite with race detection; make release produces five cross-compiled binaries in dist; make falls back to launching go through mise when go is not on the PATH and the same targets run as mise run tasks
- Ollama setup: no API key and no model list because moca asks the server via its tags and show endpoints which models it has then uses the ones supporting tool calling; slash model re-reads so a model pulled meanwhile shows up; address from providers.ollama.baseUrl else OLLAMA_HOST else localhost port 11434 accepting bare host, host port, or a full URL; for another machine start its server listening on the network; a key is only for a server behind an authenticating proxy
- esc kills a running bang command which now gets the activity row like runs and slash compact
- External tools and the skills ecosystem landed in v0.1.0-alpha: rtk and graphify in the default allowlist with built-in guidance skills and the shell analyser unwrapping rtk so it cannot launder a non-allowlisted command; Agent-Skills compatibility with SKILL.md files written for pi, Claude Code and OpenCode including graphify's proven by a vendored corpus with provenance and SHA-256 enforcement; prompt templates from Claude Code command files and pi templates drop in; graphify install platform moca contributed upstream in Graphify Labs graphify pull request 4174
- Failing-test investigation protocol: while a failing test run is unresolved, every tool result states the next step — read the failing test with the read tool and locate the cause with search — and edit refuses until both happened; the ship-gate loop's investigation steps enforced rather than advisory
- MCP landed in v0.1.0-alpha: the lazy MCP proxy with stdio and streamable HTTP transports, a persisted discovery index, search describe and call in one roughly 200-token tool with server tool lists never entering the prompt, servers starting on first call and stopping after idle; moca mcp import imports Claude Code OpenCode and Pi server configs with secrets rewritten to env references
- moca login provider stores API keys too: providers without an OAuth flow like anthropic print the recorded policy reason then prompt; the prompt shows only on a terminal so echoing a key into moca login stores silently; the api-key flag forces the key path for openai; moca logout clears token and key
- slash model picker marking providers without a key plus argument completion in the slash dropdown for model effort login logout show and resume
- Permissions landed in v0.1.0-alpha: path jail anchored at the session workdir with symlink resolution and read-only skill roots; shell command analysis via mvdan sh with hard-deny and ask ladders, compound-command checking and rtk unwrapping; project trust with approve and no-approve and a trust json store; yolo mode via flag and slash yolo turning every check off between runs
- Providers and protocols: three protocol adapters hand-rolled HTTP plus SSE with no SDKs (anthropic-messages, openai-completions, openai-responses); built-in providers anthropic with API key and subscription OAuth not permitted by Anthropic's terms, opencode-go OpenCode Zen with a mixed-protocol catalog including the Go-tier session header, openai with API key and ChatGPT subscription login; custom providers via baseUrl plus protocol covering vLLM, LM Studio, Ollama and any OpenAI-compatible gateway
- Resume picks up where you left off: resume and continue flags replay the last 3 turns into the scrollback; a new slash resume picker lists this directory's other sessions and an id from another directory is refused
- Retry/backoff with jitter honoring retry-after, SSE stall timeout, mid-stream failure rule; effort as a first-class request parameter with per-model clamping; cross-provider history transform with thinking replay bound to the producing model and tool-id normalization; upstream failures the gateway labels server_error are retried even when framed as HTTP 400 like the opencode-go thinking-config flake; the default HTTP client bounds the response-header wait at 120s so a silent provider cannot hang an unattended run
- slash sessions — manage this directory's stored sessions in one place: every session listed with the open one marked current, enter switches to it, ctrl-d deletes it after a y confirm; the open session and sessions held open by another moca process refuse and a file under a live writer is never unlinked; the list stays open and the newest 100 are listed with the title giving the real total
- TUI landed in v0.1.0-alpha: Bubble Tea with a bottom status bar from the first frame showing cwd, git branch, provider model, effort, context size and usage percent, session tokens, cost with the subscription marker, transient retry states and the YOLO state; multi-line input with bracketed paste and collapsed chips; shift enter newline via enhanced keyboard reporting with fallbacks; steering during runs; bang prefixes; approvals allow once, allow always, deny; pager; slash copy via OSC 52; slash commands for model, effort, hard, yolo, clear, compact, cost, undo, copy, show, help plus prompt templates
- TUI polish in v0.1.1: slash autocomplete dropdown listing built-in commands and loaded templates with one-line hints filtered as you type with up down pick tab complete enter run esc dismiss and built-ins winning collision; commands echo as messages in the same band as user messages before their output including refusals; welcome line product name renders orange; thinking blocks print their item line when the block ends with the streamed duration; two-line status bar from the first frame; full-width rules around the input and compact welcome lines; full-row backgrounds for submitted messages and assistant responses; muted tool and thinking lines; exit aliases; the inline-renderer shrink guard became burst-aware so shrinking frames cannot strand stale rows when keystrokes coalesce
- Credentials live in config moca auth json mode 0600 never committed: login in the TUI with a provider picker and masked API key pasting or browser sign-in, or moca login provider on the CLI with the api-key flag and silent pipe storage; a stored key is used ahead of any apiKey env reference and moca logout or the TUI logout clears it; moca login openai opens the browser for consent with SSH headless printing the URL and accepting a pasted code with a no-browser flag; tokens auto-refresh under a cross-process lock; set providers.openai.auth oauth after a TUI sign-in which the wizard offers; the subscription route serves the Responses API only and model slugs must be available to the ChatGPT account declared under providers.openai.models when not in the built-in catalog
- Debugging and logs: moca keeps a small diagnostic log per run under the state moca logs directory recording what moca did like retries, MCP servers starting and stopping, and how a run ended, but never prompts, code or keys; attach the newest file to a bug report; MOCA_LOG debug adds per-run detail; log level off turns it off; old logs removed after snapshot retention days at 30 by default
- Docs links: docs/specs/SPECS.md as the current implemented state, docs/specs/DESIGN.md as the v1 contract, docs/specs/external-tools.md covering rtk graphify skills and prompts
- Extend via external tools: rtk the token-compressed CLI output tool and graphify the codebase knowledge graph tool are allowlisted and ship built-in skills; see docs/specs/external-tools.md
- Extend via MCP: mcp servers in the config with stdio plus streamable HTTP; lazy by design with one fixed 200-token mcp proxy tool, server tool lists never entering the prompt, servers starting on first use and stopping when idle; moca mcp import imports Claude Code OpenCode and Pi server configs
- Extend via project instructions: AGENTS.md or CLAUDE.md loaded only in trusted projects via the approve flag
- Extend via skills: SKILL.md directories in the global config skills path or in a trusted repo's moca skills path; any skill written for pi, Claude Code or OpenCode loads unchanged; see docs/specs/external-tools.md
- First run with no model configured opens the TUI with a red notice instead of stopping: login stores an API key or signs in then model picks a model and starts the session with the choice saved as config model so next launch starts straight into a session; one-shot p still needs a model up front
- Install scripts: install.sh for Linux and install.ps1 for Windows download the platform release package, verify against the release checksums, and install the binary to the local bin directory or beside an existing moca; re-running a script is safe and updates in place never an error; version flag pins a release, dir flag picks the target, help prints usage; ortake a package from the releases page with linux amd64, linux arm64 and windows amd64 published by a manual Release workflow run and other platforms like darwin building from source with make release
- README v1 non-goals probably forever: subagents, hooks, LSP, image generation, voice, telemetry, a binary plugin system
- Ollama context window caveat: the OpenAI-compatible endpoint cannot choose the context window per request so a too-small window silently truncates the prompt; start the server with a window of at least 16K and declare num_ctx per model via providers.ollama.models.id.contextWindow; moca warns at startup when it had to guess an assumed 16384 or finds a window too small to work in; a downed server says so and how to start it immediately
- One-shot contract: stdout is the final answer text only; tool activity, retries and the token and cost summary go to stderr; exit codes 0 completed, 1 provider or runtime error, 2 config or usage error, 3 incomplete from a step limit or stopping after repeating the same failing call, 130 interrupted
- Phases table: 1 skeleton with 3 protocol adapters streaming retries and JSONC config done with the gate of streaming through all providers and the example decoding intact; 2 seven tools plus agent loop plus permissions plus yolo plus skills plus AGENTS.md plus session schema done with the edit and shell-analysis ladders green and the jail enforced; 3 TUI shell plus slash commands plus status bar plus steering plus pager done with cross-provider model effort and hard commands; 4 context manager plus compaction plus resume done with token-triggered compaction and continue; 5 MCP lazy proxy done with a real server via proxy and no schemas in the prompt; 6 rtk plus graphify plus skills ecosystem compatibility done with rtk preferred in a real session; 7 OAuth providers plus an upstream graphify PR plus v0.1 shipped as v0.1.1-alpha with the current line v0.3.0-beta
- Providers: opencode-go with OpenCode Zen mixed-protocol catalog like glm-5.3 minimax-m3 kimi-k3; anthropic with the note that subscription OAuth is not permitted by Anthropic terms for third-party clients so API key only; openai with OPENAI_API_KEY or moca login openai via the Sign in with ChatGPT open-source flow for ChatGPT Plus or Pro subscription login; ollama with no auth for a local or LAN Ollama server with models discovered from it; any custom name with baseUrl plus protocol for vLLM, LM Studio, OpenAI-compatible gateways
- Secrets never appear as literals: apiKey is an optional env VAR reference, or drop it and store the credential with login in the TUI or moca login on the CLI; a stored key wins over the env reference
- Status v0.3.0-beta: first-run setup mode with a bare config opening the TUI with login and model guidance, a web tool, plan mode writing only docs/plans with do executing a plan step by step, a harness pass measured on a seven-scenario eval corpus, self-update, saved slash commands, an Ollama provider; the ship gate's live legs pass with evidence in SPECS section 15
- TUI keys: enter send; shift enter with alt enter and ctrl j fallbacks for newline; esc interrupt the run or a running bang command or a compact; ctrl o pager on the latest item or the full command of a pending approval; alt t pager on the latest thinking block with the whole reasoning chain; alt p paste chips with large pastes collapsed and the buffer intact; ctrl r search prompt history with enter using the match without sending; shift tab cycle the effort level; at plus path complete a workdir file; ctrl c twice quits or slash exit; a ctrl a d approval allow once allow always deny; up down tab esc in the slash dropdown to pick complete dismiss
- Use commands: moca alone for the TUI in the current directory; moca p prompt for one-shot with the prompt also on stdin via a dash; moca plan flag with a one-shot request writes docs plans slug markdown changing nothing else; moca do flag with a plan file executes it step by step ticking each unchecked box

## God Nodes (most connected - your core abstractions)
1. `model` - 152 edges
2. `SPECS.md — moca's living normative spec (16 numbered sections, updated with every behavioral PR)` - 151 edges
3. `newScript()` - 76 edges
4. `session` - 75 edges
5. `run()` - 70 edges
6. `newTestModel()` - 67 edges
7. `Word` - 57 edges
8. `newAgentModel()` - 54 edges
9. `key()` - 50 edges
10. `Agent` - 48 edges

## Surprising Connections (you probably didn't know these)
- `Concurrency model` --references--> `Agent`  [INFERRED]
  docs/plans/2026-10-13-acp-protocol.md → internal/agent/agent.go
- `File map` --references--> `runACP()`  [INFERRED]
  docs/superpowers/plans/2026-10-13-acp.md → cmd/moca/acp.go
- `File map` --references--> `runACP()`  [INFERRED]
  docs/superpowers/plans/2026-10-13-acp-merged.md → cmd/moca/acp.go
- `Review Focus (items owned by this batch)` --references--> `TestACPTrust()`  [INFERRED]
  docs/superpowers/plans/2026-10-13-acp-batch-4-ship.md → cmd/moca/acp_test.go
- `3. CLI surface (as implemented)` --references--> `TestResumeFromAnotherDirRefused()`  [INFERRED]
  internal/skills/builtin/self-knowledge/references/specs.md → cmd/moca/cli_test.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Agent loop input pipeline** — docs_specs_specs_turn_contract, docs_specs_specs_cross_provider_transform, docs_specs_specs_system_prompt_node, docs_specs_specs_frozen_tools [EXTRACTED 1.00]
- **Credential resolution pipeline** — docs_specs_specs_env_indirection, docs_specs_specs_store_first_credential, docs_specs_specs_credential_store, docs_specs_specs_login_wizard [EXTRACTED 1.00]
- **Tracked read/edit staleness gate** — docs_specs_specs_read_tool, docs_specs_specs_write_tool, docs_specs_specs_edit_tool, docs_specs_specs_swe_agent_read_guard [EXTRACTED 1.00]
- **Ecosystem SKILL.md compatibility corpus (graphify, Claude Code, pi, OpenCode)** — internal_skills_testdata_ecosystem_sources, internal_skills_testdata_ecosystem, internal_skills_testdata_ecosystem_graphify_graphify_vendored, internal_skills_testdata_ecosystem_claude_code_skill_creator_vendored, internal_skills_testdata_ecosystem_claude_code_frontend_design_vendored, internal_skills_testdata_ecosystem_pi_pi_subagents_vendored, internal_skills_testdata_ecosystem_pi_mcp_scripting_vendored, internal_skills_testdata_ecosystem_opencode_dotnet_unit_testing_vendored [EXTRACTED 1.00]
- **Synthetic frontmatter edge-case fixtures (CRLF, BOM, folding, quoting, rich keys, long description)** — internal_skills_testdata_ecosystem_synthetic_bom_skill, internal_skills_testdata_ecosystem_synthetic_colon_unquoted_skill, internal_skills_testdata_ecosystem_synthetic_crlf_skill, internal_skills_testdata_ecosystem_synthetic_folded_skill, internal_skills_testdata_ecosystem_synthetic_literal_skill, internal_skills_testdata_ecosystem_synthetic_long_description_skill, internal_skills_testdata_ecosystem_synthetic_rich_keys_skill [EXTRACTED 1.00]
- **OAuth policy gate path: verify terms -> per-provider decision -> config enforcement** — docs_specs_oauth_verification_oauth_verification_record, docs_specs_oauth_verification_anthropic_api_key_only_decision, docs_specs_oauth_verification_ship_oauth_openai_decision [INFERRED 0.75]
- **Human-in-the-loop validation before declaring done** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_claude_code_frontend_design_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_review_validation [INFERRED 0.75]
- **Parallel subagent fanout with parent synthesis** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_graphify_graphify_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_skill [INFERRED 0.85]
- **Skills referencing bundled reference files for progressive disclosure** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_graphify_graphify_skill, internal_skills_testdata_ecosystem_pi_mcp_scripting_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_skill [INFERRED 0.85]
- **Seven-scenario eval corpus** — test_evals_baseline, agents_eval_corpus, test_evals_baseline_evalstats, test_evals_baseline_scenario_bugfix, test_evals_baseline_scenario_feature, test_evals_baseline_scenario_largefile, test_evals_baseline_scenario_plan, test_evals_baseline_scenario_refactor, test_evals_baseline_scenario_vague, test_evals_baseline_scenario_web [INFERRED 0.95]
- **rtk allowlist unwrapping and its subcommand classes** — docs_specs_external_tools_rtk, docs_specs_external_tools_rtk_unwrapping, docs_specs_external_tools_rtk_read_only_class, docs_specs_external_tools_rtk_write_config_class, docs_specs_external_tools_rtk_native_proxy_class, docs_specs_external_tools_rtk_shell_string_refusal, internal_permissions_rtk [INFERRED 0.95]
- **Token-efficiency mechanisms implementing the minimal-agent philosophy** — readme_minimal_tui_coding_agent, readme_prompt_caching, internal_agent_system_prompt_windowed_read, readme_diff_shaped_results, readme_token_denominated_compaction, internal_agent_system_prompt_token_discipline, readme_rtk_token_saver, readme_graphify_token_saver [INFERRED 0.95]

## Communities (256 total, 149 thin omitted)

### Community 0 - "go_pkg_strings"
Cohesion: 0.05
Nodes (142): Level, nopCloser, wc, runACP(), TestExitForOAuthSentinels(), startLog(), loginAPIKey(), runLogin() (+134 more)

### Community 1 - "testing.T"
Cohesion: 0.03
Nodes (90): TestRedaction(), TestEffortRankOrdered(), TestUsageAdd(), TestCallToolNullArgsSentAsEmptyObject(), TestHelperProcess(), NewStore(), TestAPIKeyStore(), TestConcurrentRefreshOnce() (+82 more)

### Community 2 - "newScript"
Cohesion: 0.06
Nodes (100): scriptServer, StartError, Task 1 — `internal/agent`: `Transcript()` accessor, newScript(), startTest(), startTestWith(), startWith(), TestAbortWritesSyntheticResults() (+92 more)

### Community 3 - "run"
Cohesion: 0.05
Nodes (83): TestACPBadFlags(), TestACPServesOnlyJSONRPC(), fakeCompletions(), isolate(), TestExitCodes(), TestInterruptedExit130(), TestLoginLogoutCLI(), TestMCPImportCLI() (+75 more)

### Community 4 - "newFakeAS"
Cohesion: 0.10
Nodes (29): audContains(), b64d(), jwksKey(), browse(), firstURL(), newFakeAS(), runLogin(), TestCallbackPageReportsFailure() (+21 more)

### Community 5 - "SPECS.md — moca's living normative spec (16 numbered sections, updated with every behavioral PR)"
Cohesion: 0.06
Nodes (21): Current version v0.3.0-beta (last updated 2026-10-09); version stamp rendered from git describe, docs/specs/DESIGN.md — the v1 vision/contract (rev 21 after the harness pass); SPECS wins on disagreement, docs/specs/external-tools.md — rtk/graphify allowlist rules and the cross-tool skills/prompts ecosystem; manual graphify install copy, Upstream PR Graphify-Labs/graphify#4174 — graphify install --platform moca pending merge; manual copy documented in docs/specs/external-tools.md, SPECS.md — moca's living normative spec (16 numbered sections, updated with every behavioral PR), `moca mcp import [--yes]` — import MCP servers from Claude Code / OpenCode / Pi configs; secrets are never copied, `moca mcp index` — start every configured MCP server once to (re)build the discovery index; both mcp subcommands load config but need no model (routed before the model check), moca — a minimal, token-efficient, provider-agnostic coding agent; Go 1.27.1 pinned once in go.mod, stdlib + one parsing dependency (mvdan.cc/sh/v3) (+13 more)

### Community 6 - "newAgentModel"
Cohesion: 0.07
Nodes (52): keyMsg(), TestExitCommandQuits(), TestCompactEscapeCancels(), TestCompactingBlocksRun(), TestCompactNothing(), TestEditItemShowsDiff(), TestThinkingLinePrintsBeforeTheResponse(), TestAllowAlwaysRequiresCtrl() (+44 more)

### Community 7 - "newTestModel"
Cohesion: 0.05
Nodes (83): NewDefaultStore(), 3.5 TUI (`moca`, no `-p`), ShellEnv(), TestActivityLine(), TestCoalescedShrinkRemembersTheLastFlushedRow(), TestInputBoxGrowsAndShrinks(), TestPagerCloseDefersTheRelease(), TestRefuseRunningWhileOutputInFlight() (+75 more)

### Community 8 - "auto_test.go"
Cohesion: 0.10
Nodes (54): rules(), newAutoShell(), TestAutoFixRound1(), TestAutoFixRound2(), TestAutoGitSymlinkAndWritableAncestor(), TestAutoLadder(), TestAutoOptionSpecs(), TestAutoRmGuardsProjectRoot() (+46 more)

### Community 9 - "model"
Cohesion: 0.07
Nodes (18): TextOf(), OAuthProvider(), modelPickItems(), AbbrevEffort(), AbbrevHome(), providerNames(), modeNotice(), TestReplayLimitsTurnsAndShowsItems() (+10 more)

### Community 10 - "internal/provider/auth.go credential store — ~/.config/moca/auth.json (file 0600, dir 0700; the write forces 0600 even over a stale tmp file and the rename deliberately replaces a symlink rather than following it): providers.<name> holds the OAuth token (access/refresh, expiry, account/client id, id token, email, scopes) and apiKeys.<name> holds API keys from /login or moca login (validated: empty values and control bytes refused, so a credential can never smuggle a newline into a header); atomic writes (temp + rename)"
Cohesion: 0.06
Nodes (10): Built-in vendor catalog rows: anthropic claude-opus-5-5 / claude-sonnet-5-5 (adaptive thinking, 1M ctx) / claude-haiku-4-5 (budget, 200K); openai gpt-6-astra / gpt-6-luna (openai-responses, openaiReasoning), *config.EnvError (unset env: var at request build), internal/provider/catalog.go — Model{Provider, ID, Protocol, ContextWindow, MaxOutput, Cost(USD per M in/out/cacheRead/cacheWrite), ThinkingMode, ThinkingLevelMap}, mvdan.cc/sh/v3 — the only non-UI dependency; shell parsing for the shell analyser, Built-in opencode-go catalog rows: glm-5.3, glm-5.3-flash, minimax-m3 (anthropic-messages), kimi-k3, gpt-6-luna (responses), grok-4.7 (responses), deepseek-v4.1-flash / v4-flash / v4-pro / v4-flash-vision-exp, kimi-k2.7-code, mimo-v2.6-pro / v2.6-flash, qwen3.8-max (completions) and qwen3.8-flash / qwen3.7-plus (anthropic-messages) — with per-model context, max output and $/M rates, internal/config — version, paths, JSONC pre-pass, typed config + validation, env indirection, AppendString/SetObjectEntry/SetString, internal/llm — leaf types: Message, ContentBlock, ToolCall, ToolResult, Request, Usage, Event…, internal/provider — SSE reader, error taxonomy, catalog, 3 protocol codecs, replay helpers, retry wrapper, registry (incl. CheckCredential), subscription OAuth store + flow (+2 more)

### Community 11 - "Parse"
Cohesion: 0.06
Nodes (50): scanner, span, Parse(), TestAPIKeyOptional(), TestDefaultsApplied(), TestEnvRefs(), TestOAuthAllowedForOpenAI(), TestOAuthRejectedForNonOAuthProvider() (+42 more)

### Community 12 - "Phase-3 gate (2026-10-05): a mock-driven TUI session in a real tmux terminal against a scripted openai-completions server — status bar from the first frame, branch switch, streamed commits, items, slash commands (/help /cost /model /effort /hard /copy /show /compact), missing-key switch refused, newline hint, 120-line paste chip expand send, steering verified in the CAPTURED REQUEST PAYLOAD, approvals (a ran; A appended to shell.allow byte-preserving + .bak; rm prompted without [A]), /yolo on/off incl. a 60-column bar keeping YOLO, ! local, /undo, esc interrupting a tool, /clear; it found and fixed two real bugs (event-pipe deadlock on /yolo; styled prints mangled by Sanitize)"
Cohesion: 0.05
Nodes (5): `moca` with no -p opens the interactive TUI in the current directory (§3.5), --effort <level> — off|minimal|low|medium|high|xhigh|max, validated and clamped to the model's supported levels, Phase-3 TUI reviews (2026-10-05) — four passes: pass 1 Approve with fixes 1M/3L; pass 2 1H/2M/6L (all fixed in 640be8e: startup panic from pre-program events, AppendString bounds, /clear ordering, chip-marker collisions, live flush, mid-run ! refusal, approval sanitizing, GitBranch timeouts); pass 3 0H/9M/7L fixed in efcdaf1; pass 4 (re-review) fixed in a2b091c; three items recorded as intentionally unchanged, Phase-7 OAuth/release reviews (2026-10-06): pass 1 0H/2M/4L, adversarial pass 2 0H/4M/6L, pass 3 0H/3M/6L — all confirmed findings fixed with regression tests; guard tests that cannot fail against the pre-fix tree were MUTATION-PROVEN load-bearing (randText error path, constant-randText, JWKS kid filter, exponent guard, exitFor sentinel, OAuth-list sync); contested items adjudicated with own probes (duplicate-keys fix is last-wins alignment, not refusal; the callback client-id contract kept; the two-channel paste race fixed structurally); pass-3: the checker runs the suite in the final repo, CheckCredential never refreshes, the shared stdin line reader, ctx-aware lock wait + bounded refresh, the callback page's 400, internal/tui — Bubble Tea TUI: pure state (input/items/commands/status/format/sanitize/bridge/markdown) + glue (app/pager/trust)

### Community 13 - "Execution: calls of one turn run SEQUENTIALLY in emitted order; a failed call does not cancel the rest; every call gets exactly one tool_result; unknown tool → error listing available tools; invalid JSON arguments → error telling the model to split the work; a panic inside a tool is recovered by Registry.Run into an error result ('internal error in <tool>: … — this is a moca bug, not your input; the call may have partly applied …') instead of crashing the session (rev 21 — eval feature#3 crashed on the edit-diff bug)"
Cohesion: 0.07
Nodes (4): Tool result availability boundaries (session/snapshot scope), --yolo/--no-yolo — override config yolo; yolo turns every permission check off, Independent review 2026-10-05 (docs/reviews/2026-10-05-phase-2-tools-agent-loop.md, verdict Block): all findings fixed with regression tests, web.search config keys (provider tavily|exa, apiKey env reference)

### Community 14 - "NewManager"
Cohesion: 0.11
Nodes (40): MCPServer, crashingServer(), shServer(), skipWindows(), TestCallTimeoutStopsWedgedServer(), TestHandshakeTimeout(), TestHTTPAbortedStreamReplayRule(), TestIndexSaveMergesConcurrentWriters() (+32 more)

### Community 15 - "protocol.go"
Cohesion: 0.08
Nodes (57): AgentCapabilities, AuthMethod, AvailableCommand, AvailableCommandInput, AvailableCommandsUpdate, CancelNotification, ClientCapabilities, ClientSessionCapabilities (+49 more)

### Community 16 - "Word"
Cohesion: 0.15
Nodes (54): anyNonLiteral(), anyRoot(), buildWithWrites(), hasOpt(), longIs(), mutatorArgs(), mutatorPath(), mutatorVerdict() (+46 more)

### Community 17 - "Unit/integration: go test ./... -race — 685 top-level tests across 16 Go packages (count established via `go test ./... -count=1 -v | grep -c '^=== RUN'`); phases 1-5 keep passing; growth recorded: 585 before the web-tool/plan-mode suites → 613 rev-21 harness pass → 640 system-prompt template extraction → 641 overflow-per-session → 643 branch-review pins → 646 diagnostic-log suites → 679 web private-address + branch-review pins → 685"
Cohesion: 0.09
Nodes (3): MCP server config keys (command|url, args, env, headers, description, approve), MCP frozen stub tool (registered in place of the proxy when mcp.servers is empty), Phase-5 MCP reviews (2026-10-06): passes 1-4 — all findings fixed with regression tests (sentinel-based retry instead of text matching; write-failure preferring the recorded exit error; bounded default HTTP client; import covering literal secrets outside env/headers; export-name collision skip; corrupt-index reset; rune-safe description cut; bounded Close; notification status check; no respawn after Close; quoted-id echo; import summary counting) — every behavior-pinning test verified to fail against pre-fix sources

### Community 18 - "Config"
Cohesion: 0.12
Nodes (15): decideTrust(), hasProjectResources(), modeWithoutProject(), runTUI(), AutoConfig, ContextConfig, LogConfig, MCPConfig (+7 more)

### Community 19 - "anthropic_test.go"
Cohesion: 0.16
Nodes (36): newAnthropic(), keyCred(), mustJSON(), sseServer(), TestAnthropicBodyHygiene(), TestAnthropicBudgetAndNoCache(), TestAnthropicBudgetFollowsEffort(), TestAnthropicCredentialHeaders() (+28 more)

### Community 20 - "Append-only JSONL transcript at ~/.local/share/moca/sessions/<date>-<slug>-<id8>.jsonl (file 0600, dir 0700); every entry has id, parentId (chain), type, ts; types: session (header: workdir, provider, model, EFFECTIVE effort, startedAt, mocaVersion, the FULL system prompt, yolo), message, tool_use, tool_result, compaction (summary + firstKeptEntryId + tokensBefore + usage + cumulative readFiles/modifiedFiles — its usage/cost count toward session totals and are restored on resume), snapshot, error, permission_mode, model_change (written by /model, /effort, /hard), elision (rev 21: {ids, tokens} — tool_result entry ids superseded later)"
Cohesion: 0.10
Nodes (4): --continue — resume the newest session started in this workdir (canonical-path match), --resume <id8|last> — resume a stored session (8-hex suffix; `last` = newest by mtime), Phase-4 compaction/resume reviews (2026-10-05) — passes 1 and 2, all confirmed findings fixed with regression tests (skip-and-warn summarizer, ! busy guard, resume workdir validation, rune-safe truncation, tracked-path normalization, kept-id fallback in liveEntries, fixed-overhead guard clause, cancellable /compact, llm.TextOf dedupe, --model-on-resume test, stable Find order, direct exactly-once request count); re-verified pass 1, retracted one item, added the !-during-compact race and the missing-workdir resume diagnostic, internal/compact — token estimate helpers + compaction: budgets, cut points, lossy serialization, structured summaries (no provider import)

### Community 21 - "9.5 ACP server (`moca acp`)"
Cohesion: 0.04
Nodes (56): ModeChanged, PlanChanged, Resumed, Retry, SteeringApplied, StreamReset, TextDelta, ThinkingDelta (+48 more)

### Community 22 - "Summarize"
Cohesion: 0.15
Nodes (15): Metrics, Collect(), fatal(), main(), meanSD(), readResults(), Summarize(), TestCollect() (+7 more)

### Community 24 - "Model"
Cohesion: 0.11
Nodes (15): ContentBlock, Model, TestBuiltinCatalogSane(), TestMaxTokensAndBudget(), TestCompletionsEffortOffIsExplicit(), Registry, priorReasoningText(), replaysVerbatim() (+7 more)

### Community 25 - "v0.1.0-alpha (2026-10-07)"
Cohesion: 0.18
Nodes (10): CHANGELOG, CLI, Context & sessions, External tools & the skills ecosystem, MCP, OAuth (subscription login), Providers & protocols, Tools & permissions (+2 more)

### Community 26 - "Plan mode (rev 20) in the tools: with Env.Plan set, write/edit reach only <workdir>/docs/plans/*.md (everything else → `refused: plan mode only writes docs/plans/*.md …`; the ask-write roots stay closed — nothing is asked) and a successful plan-file write/edit sets Env.PlanWrote; read/search/ls/web and shell/MCP stay available under the normal ladder (2026-10-09 follow-up: planning needs git log, builds, docs lookups); the failing-test banner/refusal is suppressed in plan mode (plan runs inspect, they don't fix); the confinement is NOT a permission — --yolo does not lift it; edit applies the plan-zone check BEFORE the ask-write approval (plan mode never asks)"
Cohesion: 0.08
Nodes (9): Agent run plan, agent.StartError — a failed run setup (data dir, session files, skills) exits 1, `moca -p "<prompt>"` — one-shot agent run; `-p -` reads the prompt from stdin (trailing newlines stripped), Credential env error, Recorded review-finding classes (phase 10 failure modes), Env.Plan flag (set by --plan or the TUI /plan toggle), --do <plan.md> — execute an implementation plan (rev 21): implies a one-shot run; -p adds instructions; exclusive with --plan, --plan — plan mode (rev 20): analyze the request and write an implementation plan to docs/plans/, change nothing else (+1 more)

### Community 27 - "context.Context"
Cohesion: 0.06
Nodes (37): lockFile(), lockFile(), 13.5 Diagnostic log, Spill(), TestSpillWritesFullOutput(), readLine(), TestVerifies(), trackFailure() (+29 more)

### Community 28 - "initialize"
Cohesion: 0.19
Nodes (10): initialize(), TestHTTPCloseBounded(), startHTTP(), fakeHTTPServer(), TestHTTPNotifyStatusChecked(), TestHTTPTransport(), TestStdioEarlyExit(), client (+2 more)

### Community 29 - "Manager"
Cohesion: 0.12
Nodes (15): ConfigHash(), Rank(), TestIndexPersistAndHash(), TestRank(), words(), Manager, startFailClass(), 10.5 MCP proxy (phase 5) (+7 more)

### Community 30 - "NewRegistry"
Cohesion: 0.13
Nodes (28): newFakeOllama(), ollamaRegistry(), TestNoOllamaNoProbe(), TestOllamaChatWithoutKey(), TestOllamaDiscovery(), TestOllamaDiscoveryVerifyConcurrent(), TestOllamaKeylessAndOptionalKey(), TestOllamaServerDown() (+20 more)

### Community 31 - "Agent"
Cohesion: 0.04
Nodes (43): Options, savedModel, Status, Kind, Result, effortLevels(), effortOption(), TestEffortOption() (+35 more)

### Community 32 - "loginState"
Cohesion: 0.33
Nodes (3): loginChoice, loginState, loginStep

### Community 33 - "TempRoots"
Cohesion: 0.40
Nodes (4): cutUTF8(), isCont(), TestTempRoots(), TempRoots()

### Community 35 - "Store"
Cohesion: 0.30
Nodes (4): Store, newHostID(), validateAPIKey(), storeFile

### Community 36 - "styleBlock"
Cohesion: 0.31
Nodes (10): respMsgStyle(), styleBlock(), styleBlockLead(), TestCommandEchoesAsUserMessage(), TestMessageBackgroundsCoverTheFullRow(), TestResponseCommitUsesBackground(), TestScrollbackMessageBackgrounds(), TestStyleBlockExpandsTabs() (+2 more)

### Community 37 - "update/update.go"
Cohesion: 0.09
Nodes (32): Compare(), comparePre(), ExtractBinary(), FindAsset(), FindChecksums(), IsTemporary(), ParseChecksums(), ParseVersion() (+24 more)

### Community 39 - "Phase 7 — subscription OAuth behind the DESIGN §3 policy gate: anthropic ships api_key only, openai ships SIWC token sharing on the openai-responses route with dynamic client registration, PKCE S256, nonce/RS256/JWKS validation, a 0600 token store with cross-process locked rotating refresh, moca login|logout; plus the automated ship gate and version stamping"
Cohesion: 0.10
Nodes (8): anthropic-messages adapter — POST {base}/v1/messages; headers anthropic-version: 2023-06-01 and x-api-key (Authorization: Bearer for OAuth tokens), openai-completions adapter — POST {base}/chat/completions; Authorization Bearer; system first, per message text joined, tool calls with arguments as a JSON STRING, tool results as role:tool; stream:true with stream_options.include_usage; reasoning_effort per §5, ErrContextOverflow (wrapped by adapters; drives overflow recovery), --model <provider/model> — override the config model for this run (recorded as model_change when replacing a stored pair), `moca login <provider>` (--api-key prompt instead of OAuth; --no-browser headless OAuth) stores a credential in auth.json; openai runs the SIWC flow; every other provider prompts for an API key (anthropic prints the recorded policy reason); unknown provider exits 2, `moca logout <provider>` — best-effort revoke of a subscription session (bounded 15 s), then clears the stored token AND API key, docs/specs/oauth-verification.md — the recorded, live-fetched policy verification (anthropic api_key-only; openai permitted via SIWC token sharing), openai-responses adapter — POST {base}/responses; Authorization Bearer; input items (message text in/out, reasoning items with encrypted_content, function_call/function_call_output), store:false, include reasoning.encrypted_content (stateless reasoning replay), instructions = system, tools as {type:function,…}, reasoning per §5

### Community 40 - "15. Verification status & known items"
Cohesion: 0.06
Nodes (28): Delete(), Info, ListAll(), 15. Verification status & known items, TestSkillSubmissionStartsARun(), matchCommands(), contains(), count() (+20 more)

### Community 43 - "Release - build packages, publish GitHub Release"
Cohesion: 0.16
Nodes (19): Release asset names are a contract with internal/update, Go 1.27.1 pinned in go.mod via mise, Release workflow (.github/workflows/release.yml), Release - build packages, publish GitHub Release, Download artifacts with merge-multiple, Upload package artifacts, Asset naming moca-<version>-<os>-<arch>.tar.gz|zip, build job - package matrix targets (+11 more)

### Community 45 - "Input"
Cohesion: 0.09
Nodes (8): TestEnterAndCtrlCDuringApproval(), TestShellBusyBlocksRunAndClear(), TestShellRefusedWhileRunning(), lineCount(), hasControl(), Input, padGuard, paste

### Community 46 - "README.md - moca"
Cohesion: 0.12
Nodes (23): config.Standardize preserves byte offsets, Exactly eight frozen tools, apiKey must be an env:VAR reference, Golden tool schemas (schemas.golden.json), OAuth policy gate (oauth-verification.md), Strict config decode (unknown keys rejected), Prompt templates (~/.config/moca/prompts/<name>.md -> /name), mcp tool (search, describe, call) (+15 more)

### Community 48 - "External tools - rtk, graphify, skills ecosystem"
Cohesion: 0.16
Nodes (19): graphify-out/ knowledge graph, System prompt built once per run, never rebuilt, External tools - rtk, graphify, skills ecosystem, Cross-tool skills compatibility, graphify - codebase knowledge graph tool, graphify install --platform moca, Graphify-Labs/graphify PR 4174, rtk native-binary proxy subcommands (+11 more)

### Community 50 - "Verified live endpoints: authorize/token/revoke URLs, JWKS RS256, resource https://api.openai.com/v1"
Cohesion: 0.05
Nodes (34): No registration path for third-party consumer-plan OAuth; using Anthropic's own client ids would be spoofing, Timeline: 2026-01 server-side OAuth checks, bans later partially reversed; 2026-02 clause formalized; 2026-04 subscription coverage ends for third-party tools; 2026-09 ban reaffirmed, Anthropic Claude Code legal docs quote: OAuth exclusively for subscription-plan purchasers of native apps; third-party developers must use API keys, Primary source cited: code.claude.com/docs/en/legal-and-compliance, Credential storage per docs: email, issuer, subject, issued client_id, host id, id_token, tokens, scopes; owner-only permissions, atomic writes, never logged or committed, Implementation delta 7: models on subscription route are account-visible slugs; runtime never calls /v1/models; user declares models in config, Implementation delta 1: anthropic — no OAuth code; config validation + docs only, Implementation delta 2: openai uses dynamic client registration (plan assumed fixed client id) (+26 more)

### Community 51 - "BASELINE.md"
Cohesion: 0.12
Nodes (23): Eval corpus workflow (run.sh -n 3 -l label), baseline label (pre-rev-21 harness), edit diff panic on trailing blank line, Empty-reply retry, evalstats comparison tool, final label (DESIGN rev 21), Investigation-protocol scope fix, Baseline vs final metrics table (+15 more)

### Community 52 - "serverEnv"
Cohesion: 0.40
Nodes (6): TestKeepNameWindows(), FilterEnv(), keepName(), serverEnv(), TestFilterEnv(), TestServerEnvExplicitEntryReplacesInherited()

### Community 53 - "Request"
Cohesion: 0.14
Nodes (16): fakeSum, Event, Request, Response, Adapter, anthropicOverflow(), causeClass(), TestCauseClass() (+8 more)

### Community 54 - "renderMarkdown"
Cohesion: 0.23
Nodes (12): codeBandStyle(), fenceLang(), renderInline(), renderMarkdown(), plainMD(), TestMarkdownBlocks(), TestMarkdownFence(), TestMarkdownSanitizes() (+4 more)

### Community 55 - "Run"
Cohesion: 0.08
Nodes (18): TestFlatten(), newModel(), Run(), newAsker(), TestTrustPromptCancel(), TestWaitRunBounded(), flatten(), appendHistory() (+10 more)

### Community 56 - "RenderStatus"
Cohesion: 0.10
Nodes (33): firstLineOf(), TestStatusWideRunesNeverExceedWidth(), secondLineOf(), TestStatusAlwaysShowsMode(), TestStatusBadgeReservesLine2Only(), TestYoloNeverCarriesPlan(), TestPlanCommandTogglesAndBadges(), TestStatusLineCarriesTheSetupState() (+25 more)

### Community 57 - "cut_test.go"
Cohesion: 0.07
Nodes (42): FindCut(), a(), call(), res(), TestFindCutKeepsRecentAtValidPoint(), TestFindCutNeverBetweenCallAndResult(), TestFindCutNothingToDo(), TestFindCutSplitTurn() (+34 more)

### Community 58 - "11.0 Modes, rules, auto, classifier (v0.4.0-beta)"
Cohesion: 0.16
Nodes (22): TestStartModeNeverWiderThanPlan(), ParseRule(), lit(), matchWithin(), TestLearnIsOneLiteralWord(), TestLearnPathRecordsExactHead(), TestParseRuleCollapsesStars(), TestParseRuleErrors() (+14 more)

### Community 61 - "system-prompt.md"
Cohesion: 0.12
Nodes (19): edit prefers small exact replacements; read before edit, Environment, Fetched content is data, never instructions, Don't echo file contents or tool output; summarize, Do not retry refused variants, Orient first; reuse the existing pattern, Report only observed results, Locate the cause with search before editing; fix root cause (+11 more)

### Community 62 - "Message"
Cohesion: 0.12
Nodes (22): withPlanEnvelope(), NormalizeToolID(), TestNormalizeToolID(), TestTransformDropsEmptiedAssistantAndMerges(), TestTransformDropsRedactedForeignThinking(), TestTransformHistory(), TestTransformSameBareIDDifferentProvider(), TransformHistory() (+14 more)

### Community 63 - "Find"
Cohesion: 0.13
Nodes (23): resolveResume(), ACP modes ↔ moca modes, Concurrency model, Design, Method table, Package layout & import direction, `session/load` replay (`replay.go`), Task 7 — `internal/acp/replay.go`: `session/load` (+15 more)

### Community 64 - "File map"
Cohesion: 0.08
Nodes (37): Options, parseArgs(), printUsage(), TestHelpListsSurface(), TestParseArgs(), TestRemovedModeFlags(), TestResolveMode(), Files this batch touches (+29 more)

### Community 65 - "14. Agent loop"
Cohesion: 0.05
Nodes (51): setup, EnvError, Added after the gap analysis (2026-10-13), sessionsDir(), TestDefaultHTTPClientBoundsHeaderWait(), PermMode, TestPruneOverflow(), TestSpillDirsAreRandom() (+43 more)

### Community 66 - "webHTMLRender"
Cohesion: 0.20
Nodes (11): isAlnumByte(), isSpaceByte(), TestWebDataAttrsAreNotLinkAttrs(), TestWebHTMLConverters(), TestWebHTMLStrayClosingPre(), webClean(), webHTMLRender(), webHTMLToMarkdown() (+3 more)

### Community 67 - "skills.LoadPrompts reads <dir>/<name>.md (optional frontmatter description/argument-hint; first dir wins a collision; sorted); ExpandPrompt substitutes $ARGUMENTS/$@ (all args), $1..$9 (whitespace-split) and $$ (literal $); Claude Code command files (allowed-tools/model keys ignored) and pi templates load unchanged (testdata/prompts/, TestEcosystemPromptsLoad); skills.SeedUserPrompts(config.PromptsDir()) materializes the embedded starter template on the first TUI run (best-effort, stderr warning on failure) — never overwriting an existing file (O_EXCL); the global prompts dir is a jail read root and ask-write root"
Cohesion: 0.16
Nodes (5): Vendored ecosystem skill corpus (pi/Claude Code/OpenCode/graphify fixtures), Shrink-reflow limitation (tmux/kitty/iTerm2 rewrap leaves one stale row), Starter command template seeded on the first TUI run, TUI slash commands (/model /resume /sessions /effort /hard /yolo /plan /do /clear /compact /cost /undo /copy /show /login /logout /help /exit), Starter create-command prompt template (create/update saved / commands)

### Community 68 - "Render"
Cohesion: 0.18
Nodes (7): jsonItem, main(), Render(), TestRenderTotal(), main(), RenderJSON(), TestRenderJSON()

### Community 69 - "11.3 The classifier (`Env.Judge`)"
Cohesion: 0.23
Nodes (10): parseJudge(), judgeAgent(), TestJudgeAccountsCost(), TestJudgeCachesAllowOnly(), TestJudgeFailsClosed(), TestJudgeNoCacheForAskOrError(), TestJudgeRequestCarriesNoToolOutput(), TestJudgeTimeoutFailsClosed() (+2 more)

### Community 70 - "`moca acp` — Agent Client Protocol server (design)"
Cohesion: 0.20
Nodes (9): §3 Errors, testing, docs, Decisions, Docs, Errors, Goal, Manual gate (recorded in SPECS §15), `moca acp` — Agent Client Protocol server (design), Non-goals (v1) (+1 more)

### Community 72 - "rtk - token-compressed CLI proxy"
Cohesion: 0.67
Nodes (4): rtk - token-compressed CLI proxy, Built-in rtk skill (internal/skills/builtin/rtk/), {{token}} environment slots, rtk (token-compressed CLI output)

### Community 73 - "findTestFile"
Cohesion: 0.67
Nodes (3): findTestFile(), isPathByte(), TestFindTestFile()

### Community 74 - "12. Skills & project instructions"
Cohesion: 0.07
Nodes (38): PromptInput, ServerLine, buildCommands(), TestBuildCommandsPrecedence(), BuildSystemPrompt(), filterSkills(), oneLine(), renderPrompt() (+30 more)

### Community 75 - "sync.Mutex"
Cohesion: 0.09
Nodes (12): logSink, stampOf(), captureLog(), TestSetupFirstSessionLogged(), logSink, ring, logSink, syncBuf (+4 more)

### Community 76 - "3.5.1 TUI additions (rev 14)"
Cohesion: 0.29
Nodes (5): Budget, NewBudget(), TestBudget(), 16. Maintenance, 3.5.1 TUI additions (rev 14)

### Community 77 - "endUpdate"
Cohesion: 0.13
Nodes (22): Deferred, Handler, RPCError, toolInput, Task 5: Server core: initialize, session/new, session/prompt, event translation, Taken from CC (and why it beat HA), Task 5: Server core: initialize, session/new, session/prompt, event translation, Task 5: Server core: initialize, session/new, session/prompt, event translation (+14 more)

### Community 79 - "moca/log_test.go"
Cohesion: 0.27
Nodes (14): logRun(), logRunCfg(), logRunCfgIn(), logRunIn(), readLog(), sessionID8(), TestLogDebugViaEnv(), TestLogFileForOneShot() (+6 more)

### Community 80 - "SOURCES.md"
Cohesion: 0.19
Nodes (7): internal/skills/testdata/ecosystem corpus, anthropics/skills frontend-design, anthropics/skills skill-creator, graphify skill (graphify install, claude platform; byte-identical to pi variant), adeotek/dotfiles opencode dotnet-unit-testing skill, npm pi-mcp-adapter mcp-scripting skill, npm pi-subagents skill

### Community 81 - "Claude Code skill-creator skill"
Cohesion: 0.16
Nodes (13): Skill description (trigger) optimization, Skill eval/iterate loop (with-skill vs baseline), skill-creator Apache 2.0 License (Anthropic), Claude Code skill-creator skill, SKILL.md anatomy + bundled resources layout, Constrained query expansion against graph vocab, Progressive Disclosure (three-level skill loading), graphify query/path/explain reference (+5 more)

### Community 82 - "Sanitize"
Cohesion: 0.09
Nodes (20): approvalPanel(), approvalPrompt(), diffPreview(), clampRunes(), FmtTokens(), FmtWindow(), GitBranch(), gitStatusArgs() (+12 more)

### Community 84 - "`moca update [--check]` — replaces the running binary with the newest GitHub release package via internal/update"
Cohesion: 0.21
Nodes (3): Built-in provider env keys (ANTHROPIC_API_KEY / OPENCODE_API_KEY / OPENAI_API_KEY), Env overrides: MOCA_API_BASE (install API base), MOCA_UPDATE_API (update API base), MOCA_LOG (one-run log level) — for tests and mirrors, `moca update [--check]` — replaces the running binary with the newest GitHub release package via internal/update

### Community 85 - "retry_test.go"
Cohesion: 0.26
Nodes (19): captureLog(), logPolicy(), TestProviderLogsFinalFailureAtInfo(), TestProviderLogsRetryAndResponse(), DefaultRetryPolicy(), RetryNotice, fail(), TestMidStreamFailureResetsAndRetriesOnce() (+11 more)

### Community 86 - "shipgate/fixture/main.go"
Cohesion: 0.22
Nodes (7): Sum(), TestTotals(), ParseList(), Average(), Max(), TestMaxAndAverage(), main()

### Community 87 - "2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)"
Cohesion: 0.15
Nodes (12): 1.1 Terms, 1.2 Client registration, 1.3 Decision, 1. anthropic — decision: `api_key only` (subscription OAuth not permitted), 2.1 Terms, 2.2 Client registration (per the docs), 2.3 Endpoints (verified live 2026-10-06 against `https://auth.openai.com/.well-known/openid-configuration`), 2.4 Inference contract for subscription traffic (open-source flow docs) (+4 more)

### Community 89 - ".config"
Cohesion: 0.06
Nodes (47): ACP (Agent Client Protocol) server for moca — implementation plan, In scope (v1), Known limitations to document (not fix), Out of scope (documented, not implemented), Permissions bridge (`permissions.go`), Settled decisions (Ben, 2026-10-13), Batch 1 completion, Before you start (+39 more)

### Community 90 - "net/http/httptest.Server"
Cohesion: 0.29
Nodes (8): ollamaServer(), ollamaStart(), TestOllamaDeclaredWindowIsQuiet(), TestOllamaReportedWindowIsQuiet(), TestOllamaStartErrors(), TestOllamaTinyWindowWarns(), TestStartWithOllamaModel(), abortingHTTPServer()

### Community 94 - "ListFiles"
Cohesion: 0.32
Nodes (7): globToRegexp(), ignored(), parseIgnore(), TestIgnore(), ListFiles(), TestListFiles(), ignoreRule

### Community 96 - "ToolSpec"
Cohesion: 0.07
Nodes (21): ToolSpec, TestProxySpecFrozen(), Builtins(), MCPSpec(), TestRegistryRecoversToolPanic(), TestRepeatedFailureIgnoresSpillPath(), Registry, NewRegistry() (+13 more)

### Community 97 - "fetch_test.go"
Cohesion: 0.24
Nodes (8): Client, New(), TestGet(), Eval prompt: plan an on-disk response cache, Cache type (dir, TTL) in cache.go, Client.Get in fetch.go uses the cache, Plan: on-disk response cache, TTL option on New

### Community 98 - "test/evals — the rev 21 eval corpus: seven scenarios, each a fixture repo (committed base), prompt.txt, check.sh judged against the base commit (SESSION names the transcript) and optional flags; not in CI, same .env key; harness changes show their delta against BASELINE.md"
Cohesion: 0.22
Nodes (4): Eval corpus scenario definitions, Eval scenarios (bugfix/feature/refactor/vague/largefile/plan/web), test/evals/BASELINE.md — records the measured eval runs; the comparison point for harness or prompt changes, test/evals/config.jsonc (eval harness config)

### Community 99 - "fixture/order.go"
Cohesion: 0.28
Nodes (6): Order, User, NewOrder(), TestOrderEmail(), TestUserEmail(), NewUser()

### Community 103 - "Items"
Cohesion: 0.51
Nodes (4): TestItems(), TestLastOfKind(), Item, Items

### Community 104 - "moca update replaces the installed binary with the latest GitHub release package: checks the newest published release skipping drafts, picks the package for the platform, verifies against checksums, and swaps the binary in place atomically with a symlinked launcher having its target replaced; sessions config and credentials never touched; check mode only reports; a dev or dirty build or one newer reports not newer; a platform without a package like darwin points at make release; runs before config is loaded so a broken config cannot block an update"
Cohesion: 0.67
Nodes (3): Install scripts install.sh for Linux and install.ps1 for Windows download the release package, verify against the release checksums, and install moca to the local bin directory or next to an existing installation; re-running updates in place and never fails because moca is already installed; version and dir flags pin a release or pick the directory, moca update replaces the installed binary with the latest GitHub release package: checks the newest published release skipping drafts, picks the package for the platform, verifies against checksums, and swaps the binary in place atomically with a symlinked launcher having its target replaced; sessions config and credentials never touched; check mode only reports; a dev or dirty build or one newer reports not newer; a platform without a package like darwin points at make release; runs before config is loaded so a broken config cannot block an update, Release workflow: a manual run of a dedicated Release action builds and publishes linux amd64, linux arm64 and windows amd64 packages plus checksums as a GitHub Release exactly what moca update consumes; pushing a version tag deliberately does not trigger it; re-running replaces the release's assets in place and a run from an untagged ref creates a draft release that moca update and the installers ignore until published by hand

### Community 105 - "Create"
Cohesion: 0.08
Nodes (27): TestParseArgsDo(), TestDelete(), TestFind(), TestFindForWorkdirViaSymlink(), TestFindMtimeTieDeterministic(), lockFile(), lockFile(), gitClean() (+19 more)

### Community 107 - "session"
Cohesion: 0.07
Nodes (32): CloseSessionRequest, DeleteSessionRequest, LoadSessionRequest, NewSessionRequest, ResumeSessionRequest, Server, session, SessionCapabilities (+24 more)

### Community 109 - "install.sh script"
Cohesion: 0.67
Nodes (5): die(), fetch(), note(), install.sh script, usage()

### Community 111 - "acp_test.go"
Cohesion: 0.20
Nodes (9): isInterpreter(), getopt(), resolveLong(), main(), argKind, getoptSpec, interpSpec, optHit (+1 more)

### Community 112 - "testOptions"
Cohesion: 0.06
Nodes (83): echoHandler, node, scriptServer, testClient, wireMsg, Review Focus (items owned by this batch), Review Focus (items owned by this batch), Review Focus (items owned by this batch) (+75 more)

### Community 113 - "fixture/slug.go"
Cohesion: 0.40
Nodes (4): Limit(), Make(), TestLimit(), TestMake()

### Community 114 - "CI Workflow (gofmt · vet · build · test)"
Cohesion: 0.40
Nodes (4): CI Workflow (gofmt · vet · build · test), CI gate set (gofmt, go vet, go build, go test -race), go.mod (Go version source for setup-go), SPECS.md §2 — CI gate definition

### Community 116 - "permissions/shell.go"
Cohesion: 0.10
Nodes (29): Jail, resolveDeep(), within(), withinAny(), rtkRunTarget(), rtkShellString(), rtkSub(), rtkTarget() (+21 more)

### Community 117 - "Lookup"
Cohesion: 0.50
Nodes (3): Lookup(), TestLookupBounds(), Valid()

### Community 118 - "Conn"
Cohesion: 0.19
Nodes (13): Conn, message, Task 6: Permissions and cancellation, Task 10: Request cancellation (`$/cancel_request`) and ACP error codes, Task 10: Request cancellation (`$/cancel_request`) and ACP error codes, Task 6: Permissions and cancellation, Task 6: Permissions and cancellation, NewConn() (+5 more)

### Community 119 - "Claude Code frontend-design skill"
Cohesion: 0.50
Nodes (3): AI-generated design default tells, frontend-design Apache 2.0 License, Claude Code frontend-design skill

### Community 120 - "pi mcp-scripting skill"
Cohesion: 0.67
Nodes (4): jev.evaluate reference, jev.evaluate semantic scripting gate, mcpScript tools API, pi mcp-scripting skill

### Community 122 - "evals/run.sh"
Cohesion: 1.00
Nodes (3): fresh_repo(), one(), run.sh script

### Community 140 - "stdioTransport"
Cohesion: 0.09
Nodes (14): proc, cappedWriter, cutRecord(), responseID(), TestResponseID(), KillProcessGroup(), SetProcessGroup(), KillProcessGroup() (+6 more)

### Community 162 - "net/http.Client"
Cohesion: 0.12
Nodes (17): post(), newHTTPError(), retryAfter(), OAuthConfig, Login(), missingScopes(), Revoke(), tokenFrom() (+9 more)

### Community 208 - ".Run"
Cohesion: 0.07
Nodes (29): checkers, applyEdit(), formatDiff(), indentOf(), joinInts(), lineOf(), matchLines(), replaceExact() (+21 more)

### Community 209 - "extractBuiltins"
Cohesion: 0.40
Nodes (5): builtinHash(), ExtractBuiltins(), extractBuiltins(), TestExtractBuiltins(), TestExtractBuiltinsReExtractsOnContentChange()

### Community 212 - ".Run"
Cohesion: 0.16
Nodes (16): TestProxyPlanModeCallFollowsNormalGating(), cutRunes(), errResult(), isTrue(), newProxy(), runP(), TestCutRunesSafe(), TestProxyArgErrors() (+8 more)

### Community 213 - "input_test.go"
Cohesion: 0.30
Nodes (11): NewInput(), TestCollapseUsesExactOffset(), TestCtrlC(), TestEditedMarkerSentLiterally(), TestHistory(), TestHistoryRecallChipsUnsafeContent(), TestMarkerNeverCollidesWithTypedText(), TestPasteChipKeepsContent() (+3 more)

### Community 215 - "Eval baseline — DESIGN rev 21 harness pass"
Cohesion: 0.50
Nodes (4): Eval baseline — DESIGN rev 21 harness pass, Found by the evals (fixed in rev 21), Reading it, Results

### Community 218 - "v0.1.1 (2026-10-08): polish, a credential store in the config dir, slash autocomplete, and a bigger catalog — still one binary, still no framework"
Cohesion: 0.50
Nodes (4): slash login in the TUI: pick a provider then paste an API key masked never echoed never written to the transcript or sign in to a subscription in the browser; while a sign-in is in flight the pasted redirect URL or code from the input box feeds the flow and esc cancels; after a successful ChatGPT sign-in the wizard offers to flip providers openai auth to oauth; slash logout revokes a subscription session best-effort and clears the stored token and key, v0.1.0-alpha (2026-10-07): first release — a minimal token-efficient provider-agnostic coding agent: one Go binary, a Bubble Tea TUI, seven tools, no framework, v0.1.1 (2026-10-08): polish, a credential store in the config dir, slash autocomplete, and a bigger catalog — still one binary, still no framework, v0.2.0-beta (2026-10-09): Ollama support, a TUI review pass with saved slash commands, and self-update from GitHub releases — still one binary, still no framework

### Community 231 - "self-configure/SKILL.md"
Cohesion: 0.29
Nodes (6): How, Installation, Never, Verify, What you can change, When it takes effect

### Community 234 - "create-command/SKILL.md"
Cohesion: 0.40
Nodes (4): File format, Rules, Steps, Where commands live

## Knowledge Gaps
- **173 isolated node(s):** `github.com/adeotek/moca`, `example.com/slug`, `example.com/inventory`, `example.com/codes`, `example.com/fetcher` (+168 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 571 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **149 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `model` connect `model` to `go_pkg_strings`, `newAgentModel`, `newTestModel`, `Parse`, `stdioTransport`, `9.5 ACP server (`moca acp`)`, `Model`, `Agent`, `loginState`, `styleBlock`, `15. Verification status & known items`, `Input`, `renderMarkdown`, `Run`, `RenderStatus`, `14. Agent loop`, `11.3 The classifier (`Env.Judge`)`, `12. Skills & project instructions`, `Sanitize`, `Items`?**
  _High betweenness centrality (0.051) - this node is a cross-community bridge._
- **Are the 8 inferred relationships involving `model` (e.g. with `CLI` and `Model picker (`session/set_config_option`)`) actually correct?**
  _`model` has 8 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/adeotek/moca`, `example.com/slug`, `example.com/inventory` to the rest of the system?**
  _173 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `go_pkg_strings` be split into smaller, more focused modules?**
  _Cohesion score 0.05155659004946174 - nodes in this community are weakly interconnected._
- **Why does `External tools - rtk, graphify, skills ecosystem` connect `External tools - rtk, graphify, skills ecosystem` to `rtk - token-compressed CLI proxy`, `permissions/shell.go`, `README.md - moca`?**
  _High betweenness centrality (0.040) - this node is a cross-community bridge._
- **Should `testing.T` be split into smaller, more focused modules?**
  _Cohesion score 0.03199023199023199 - nodes in this community are weakly interconnected._
- **Why does `README.md - moca` connect `README.md - moca` to `External tools - rtk, graphify, skills ecosystem`, `rtk - token-compressed CLI proxy`, `Release - build packages, publish GitHub Release`, `system-prompt.md`?**
  _High betweenness centrality (0.030) - this node is a cross-community bridge._