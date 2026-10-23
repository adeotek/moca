# Graph Report - moca  (2026-10-23)

## Corpus Check
- 529 files · ~594,041 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 33 file(s) not represented in the graph (top: .jsonc 17, (none) 14, .v2 1)

## Summary
- 6018 nodes · 23216 edges · 652 communities (215 shown, 437 thin omitted)
- Extraction: 83% EXTRACTED · 17% INFERRED · 0% AMBIGUOUS · INFERRED: 3871 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `274a2c67`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- go_pkg_strings
- newScript
- model
- startClient
- webFetch
- testEnv
- newAgentModel
- startTestWith
- newTestModel
- Word
- §3 CLI surface (as implemented)
- update/update.go
- §3.5 TUI (moca, no -p)
- Create
- protocol.go
- context.Context
- LoadIndex
- permissions/shell.go
- auto_test.go
- AppendString
- Part B semantic LLM extraction
- SPECS.md — moca normative spec (v0.5.0-beta)
- permission modes applyMode
- PermMode
- §2 Build, test, verify
- Event
- moca (minimal TUI coding agent)
- .Run
- §11 Permissions
- TUI
- §6 Providers & registry
- §5 Model catalog, effort, thinking
- newFakeAS
- §4 Config
- ParseFrontmatter
- §9.5 ACP server (moca acp)
- approvalPanel
- opencode-go provider
- DefaultTheme
- Manager
- sseServer
- release v0.1.0.alpha
- ollamaBaseURL
- permissions/shell.go
- session
- secret.go
- RememberSession
- Release - build packages, publish GitHub Release
- Sanitize
- DataDir
- schema_test.go
- Import direction rule
- .Session
- §13 Sessions & snapshots
- writeCfg
- §14 Agent loop
- Store
- run
- Start
- §15 Verification status & known items
- v0.1.0-alpha (2026-10-07)
- moca — MO Coding Agent
- Message
- anthropic-messages adapter
- context.Context
- §6.5 Subscription OAuth (phase 7)
- release v0.4.0.beta
- §2 Package layout (implemented)
- Run
- judgeAgent
- §10 Tools (eight, frozen)
- system-prompt.md
- compaction budgets
- Eight frozen tools
- Start wiring
- release.yml manual dispatch only
- fakeOllama
- §3.5.1 TUI additions (rev 14)
- moca diagnostic log
- encoding/json.RawMessage
- TestRequestsAreAppendOnly
- registry_test.go
- skills/autoskill_test.go
- ToolCall
- endUpdate
- What You Must Do When Invoked
- rules_test.go
- Message
- §10.5 MCP proxy (phase 5)
- sync.Mutex
- moca/log_test.go
- release v0.4.1.beta
- Agent
- autoskill_prune_test.go
- make build (bin/moca)
- rpc_test.go
- .Run
- Env.Judge classifier
- mcp tool (the proxy tool)
- keys dispatch
- tuiCmd
- Changelog
- skillgate_test.go
- graphify.ingest.ingest(url, raw_dir, author, contributor) auto-detects URL type; errors are reported, never silently continued
- SOURCES.md
- Claude Code skill-creator skill
- provider.NewRegistry
- shipgate/fixture/main.go
- autoskill.go
- .Run
- TextOf
- prefers rtk and graphify token-savers
- MCPServer
- input_test.go
- install.sh
- findTestFile
- fetch_test.go
- encoding/json.RawMessage
- exports.md
- inline NetworkX traversal fallback
- ToolSpec
- Sanitize
- release v0.1.1
- manual Release workflow with checksums
- implementation deltas vs phase-7 plan
- Render
- Open
- startStdio
- .prompt
- graphify-out/
- moca/prune_test.go
- anthropic decision: api_key only
- fixture/order.go
- Login
- Model.ClampEffort
- vague/testdata/reference/main.go
- quick_validate.py
- Registry
- DataDir
- .applyMode
- moca -p one-shot run
- errorf
- compaction budgets
- extraction JSON schema (nodes/edges/hyperedges/tokens)
- Truncate
- rpc.go
- structural interfaces pattern
- dynamic client registration (dynamic_agent_client placeholder)
- Anthropic: third-party subscription OAuth not permitted
- BuildSystemPrompt
- moca prune command
- Step 0 — constrained query expansion
- Serialize
- prompt-cache reset cost
- openai decision: ship oauth (Sign in with ChatGPT)
- README.md - moca
- testing.T
- initialize
- release v0.3.0.beta
- fixture/slug.go
- CI Workflow (gofmt · vet · build · test)
- Client
- TestKeepNameWindows
- live probe outcomes (discovery, invalid_client, Cloudflare 403)
- moca — MO Coding Agent
- loads only when corpus has doc/paper/image chunks
- applyEdit
- Lookup
- self-knowledge builtin skill: answer questions about moca itself, grounded in embedded reference docs, never from memory
- internal/applog
- sync.Mutex
- SPECS.md — moca spec document
- node ID = {stem}_{entity}, full repo-relative stem
- graphify reference: incremental update and cluster-only
- graphify.analyze.graph_diff after Step 4
- Claude Code frontend-design skill
- pi mcp-scripting skill
- evals/run.sh
- moca -p one-shot run
- skillGuard
- make vet (go vet + gofmt -l)
- v0.4.0-beta (2026-10-12)
- seedSkills
- TestKeptTailThinkingFollowsTheRoute
- reference/codes.go
- shipgate/run.sh
- ResolveEnv
- picker.go
- TestRecordNudgeNotFiredAfterARecord
- cmd/moca/main.go (run() testable core)
- Commands: mise install provisions Go 1.27.1 pinned in go.mod with PATH export because go is not on the global PATH on every host; make build produces bin/moca version-stamped from git describe; make test runs go test with race and count; make vet runs go vet plus gofmt with both clean; make fmt; make release builds five cross-compiled binaries in dist; bin moca update with check compares with the latest GitHub release; mise run tasks mirror the make targets
- Item
- moca acp serves ACP v1
- extraction-spec.md
- rtk/SKILL.md
- Arrange-Act-Assert with xUnit/NSubstitute
- acpgate/run.sh
- bugfix/check.sh
- feature/check.sh
- largefile/check.sh
- plan/check.sh
- refactor/check.sh
- vague/check.sh
- web/check.sh
- refusal_filter.sh
- LineReader
- Docs pointers: docs/specs/external-tools.md covers rtk and graphify allowlist rules and the cross-tool skills and prompts ecosystem; docs/specs/oauth-verification.md is the binding OAuth policy gate with anthropic API key only and openai subscription login; SPECS.md still cites docs/reviews review docs that were deleted so do not chase them
- Exactly eight frozen tools with the web tool joining in DESIGN rev 19; schemas are golden at internal/tools/testdata/schemas.golden.json; changing a schema is a v2 discussion and description-only edits from rev 21 are batched into one golden update because each costs every user a prompt-cache reset
- Layout: entrypoint cmd/moca/main.go with run as the testable core; sibling files handle the p flag, the TUI, login, mcp and update subcommands
- github.com/adeotek/moca
- .github/workflows/ci.yml (CI gate)
- graphify workflow in this repo: a knowledge graph at graphify-out with god nodes and community structure and cross-file relationships; for codebase questions first run graphify query when the graph json exists, graphify path for relationships, graphify explain for focused concepts; dirty graphify-out files are expected after hooks or incremental updates and are not a reason to skip; use the wiki index for broad navigation when it exists; read the graph report only for broad architecture review; run graphify update after modifying code as an AST-only pass with no API cost; the installed graphify skill or instructions come first whenever the user types the graphify slash command
- Ground truth rules: docs/specs/SPECS.md is the living normative description of what is implemented today covering the CLI contract, config keys, wire behavior, tools and agent loop; update it in the same PR as any behavior change per its section 16; when it and DESIGN.md disagree SPECS wins for the current code; DESIGN.md is the v1 contract and README.md is user-facing
- Logging: packages call standard log/slog directly; internal applog with the file handler, redaction and caps is imported only by cmd/moca; log metadata only, never prompts, file contents, command text or secrets per SPECS section 13.5
- make release (5 cross-compiled binaries in dist/)
- make test (go test -race)
- AGENTS.md headline: one Go binary, a minimal TUI coding agent at github.com/adeotek/moca with binary name moca; 220 Go files across 12 internal packages
- Single package or test via go test with the package path run flag race count
- config.Standardize for JSONC preserves byte offsets so decode errors keep the original line col; the internal config testdata example jsonc file is the byte-exact DESIGN section 12 fixture
- The system prompt is built once per run and stored in the session, never rebuilt; its fixed text is the embedded internal agent system-prompt.md file with token slots rendered at start
- Thinking blocks and signatures are model-bound; cross-provider transforms happen in the agent before each request per SPECS section 14
- Richer activity row with tool operand, streamed-token estimate and queued steering count; pager search; an approval pending marker; shift-tab cycles effort; a desktop notification or bell for a finished long run or a waiting approval while the terminal is unfocused via tui.notify defaulting to osc9
- apiKey is optional in the config: a key stored with slash login or the CLI wins over the env VAR reference which stays the fallback so an env var set by an external process still works and slash logout restores it; the stored key applies from the running session's next request
- Approvals show the whole command: up to 6 lines under the question with overflow counted and ctrl-o paging all of it; the prompt used to show only the first line so approving python3 hid the heredoc it would run
- At path completes workdir files gitignore-aware; the path is sent and the model reads the file itself
- Credentials live in config moca auth json mode 0600 beside the config file now rather than the data dir with a v0.1 store migrated automatically on first run; API keys and OAuth tokens share the file
- Catalog and tooling in v0.1.1: ten more opencode-go models all live-verified including deepseek and kimi-k2.7-code and mimo and qwen variants; the Go version pinned once in go.mod; make targets mirrored as mise run tasks; make vet now includes the gofmt check; documentation trimmed to the two living specs
- CLI landed in v0.1.0-alpha: moca for the TUI, moca p for one-shot with stdout the final text and exit codes 0 1 2 3 130, model effort approve no-approve yolo no-yolo resume continue version and help flags, moca login and logout, moca mcp import and index; the Makefile does version-stamped builds with git describe into ldflags, make test with race detection, and make release producing five cross-compiled binaries
- Context and sessions landed in v0.1.0-alpha: a token-denominated context manager with reserve and keepRecent budgets, usage-anchored estimates, cut-point rules that never orphan a tool call, structured summaries with cumulative file tracking, overflow compact-and-retry, no-cache-write summary requests; JSONL session transcripts with a single-writer lock and 0600 mode carrying entry ids, thinking blocks, model changes and compaction entries; resume with id8 or last and the continue flag
- Context pressure in the status bar: the percent turns yellow at 70 percent and red with a slash compact hint where auto-compaction fires
- slash copy keeps the last answer when the final turn had no text; failed tool items get a red cross; thinking line count singular; the pager title clamped to one row showing the scroll position
- slash cost uses the bar's compact token format and breaks the input down into fresh, cache read and cache write with subscription for OAuth
- slash create-command — a starter command template written on the first TUI run into the global prompts dir (an existing file is never overwritten); running it has the agent create or update a saved slash command, personal ones in that directory asking to approve the write since it is outside the workdir and once per write never persisted, or project ones in the workdir moca prompts; new commands are usable as soon as the run finishes with no restart
- ctrl c on an empty draft says press ctrl c again to quit; inside the pager it now also clears the textarea so the draft does not come back on the next key
- esc kills a running bang command which now gets the activity row like runs and slash compact
- Exit keeps the final status bar on screen and prints the session id8 with the resume hint
- External tools and the skills ecosystem landed in v0.1.0-alpha: rtk and graphify in the default allowlist with built-in guidance skills and the shell analyser unwrapping rtk so it cannot launder a non-allowlisted command; Agent-Skills compatibility with SKILL.md files written for pi, Claude Code and OpenCode including graphify's proven by a vendored corpus with provenance and SHA-256 enforcement; prompt templates from Claude Code command files and pi templates drop in; graphify install platform moca contributed upstream in Graphify Labs graphify pull request 4174
- The status bar shows the branch of a fresh git init with no commit yet instead of a dash
- Inline edit diffs: a successful edit prints its changed lines in red and green up to 8 under its item line
- Calmer layout: a blank row between tool items and the response, messages hanging their text, one-row item lines, bang command echoes, a per-machine shift-enter hint; scrollback colors follow the terminal profile with NO_COLOR honored and 256-color terminals getting approximated band colors instead of raw truecolor
- Markdown-lite responses: bold, code, headings, bullets, quotes and fenced code blocks in a darker band never word-wrapped instead of raw markdown; slash copy still copies the raw text
- MCP landed in v0.1.0-alpha: the lazy MCP proxy with stdio and streamable HTTP transports, a persisted discovery index, search describe and call in one roughly 200-token tool with server tool lists never entering the prompt, servers starting on first call and stopping after idle; moca mcp import imports Claude Code OpenCode and Pi server configs with secrets rewritten to env references
- moca login provider stores API keys too: providers without an OAuth flow like anthropic print the recorded policy reason then prompt; the prompt shows only on a terminal so echoing a key into moca login stores silently; the api-key flag forces the key path for openai; moca logout clears token and key
- slash model picker marking providers without a key plus argument completion in the slash dropdown for model effort login logout show and resume
- Permissions landed in v0.1.0-alpha: path jail anchored at the session workdir with symlink resolution and read-only skill roots; shell command analysis via mvdan sh with hard-deny and ask ladders, compound-command checking and rtk unwrapping; project trust with approve and no-approve and a trust json store; yolo mode via flag and slash yolo turning every check off between runs
- Persistent prompt history in a per-workdir history jsonl file plus ctrl-r history search with type to filter and enter using the match without sending
- Providers and protocols: three protocol adapters hand-rolled HTTP plus SSE with no SDKs (anthropic-messages, openai-completions, openai-responses); built-in providers anthropic with API key and subscription OAuth not permitted by Anthropic's terms, opencode-go OpenCode Zen with a mixed-protocol catalog including the Go-tier session header, openai with API key and ChatGPT subscription login; custom providers via baseUrl plus protocol covering vLLM, LM Studio, Ollama and any OpenAI-compatible gateway
- Resume picks up where you left off: resume and continue flags replay the last 3 turns into the scrollback; a new slash resume picker lists this directory's other sessions and an id from another directory is refused
- Retry/backoff with jitter honoring retry-after, SSE stall timeout, mid-stream failure rule; effort as a first-class request parameter with per-model clamping; cross-provider history transform with thinking replay bound to the producing model and tool-id normalization; upstream failures the gateway labels server_error are retried even when framed as HTTP 400 like the opencode-go thinking-config flake; the default HTTP client bounds the response-header wait at 120s so a silent provider cannot hang an unattended run
- slash sessions — manage this directory's stored sessions in one place: every session listed with the open one marked current, enter switches to it, ctrl-d deletes it after a y confirm; the open session and sessions held open by another moca process refuse and a file under a live writer is never unlinked; the list stays open and the newest 100 are listed with the title giving the real total
- Seven frozen tools in v0.1.0-alpha: read, write, edit with exact-match-first and one whitespace-tolerant fallback with re-indentation and CRLF/BOM preservation, shell stateless with bash c and pwsh NoProfile and process-group kill, search pure Go ripgrep semantics gitignore-aware, ls, and the lazy mcp proxy
- TUI landed in v0.1.0-alpha: Bubble Tea with a bottom status bar from the first frame showing cwd, git branch, provider model, effort, context size and usage percent, session tokens, cost with the subscription marker, transient retry states and the YOLO state; multi-line input with bracketed paste and collapsed chips; shift enter newline via enhanced keyboard reporting with fallbacks; steering during runs; bang prefixes; approvals allow once, allow always, deny; pager; slash copy via OSC 52; slash commands for model, effort, hard, yolo, clear, compact, cost, undo, copy, show, help plus prompt templates
- TUI polish in v0.1.1: slash autocomplete dropdown listing built-in commands and loaded templates with one-line hints filtered as you type with up down pick tab complete enter run esc dismiss and built-ins winning collision; commands echo as messages in the same band as user messages before their output including refusals; welcome line product name renders orange; thinking blocks print their item line when the block ends with the streamed duration; two-line status bar from the first frame; full-width rules around the input and compact welcome lines; full-row backgrounds for submitted messages and assistant responses; muted tool and thinking lines; exit aliases; the inline-renderer shrink guard became burst-aware so shrinking frames cannot strand stale rows when keystrokes coalesce
- PKCE S256 only
- scopes: openid profile email offline_access resource.invoke chatgpt.tokens.use.direct
- 2026-10-05-phase-2-tools-agent-loop.md
- 2026-10-05-phase-3-tui.md
- 2026-10-05-phase-3-tui-pass-2.md
- 2026-10-05-phase-3-tui-pass-3.md
- 2026-10-05-phase-3-tui-pass-4.md
- 2026-10-05-phase-4-context-compaction-resume.md
- 2026-10-05-phase-4-context-compaction-resume-pass-2.md
- 2026-10-06-phase-5-mcp-lazy-proxy.md
- 2026-10-06-phase-5-mcp-lazy-proxy-pass-2.md
- 2026-10-06-phase-5-mcp-lazy-proxy-pass-3.md
- 2026-10-06-phase-5-mcp-lazy-proxy-pass-4.md
- 2026-10-06-phase-6-rtk-graphify-ecosystem.md
- 2026-10-06-phase-6-rtk-graphify-ecosystem-pass-2.md
- 2026-10-06-phase-6-rtk-graphify-ecosystem-pass-3.md
- 2026-10-06-phase-7-oauth-release.md
- 2026-10-06-phase-7-oauth-release-pass-2.md
- 2026-10-06-phase-7-oauth-release-pass-3.md
- acp availableModes
- acp configOptions
- acp editor config
- acp editors
- acp error codes
- acp event mapping
- acp gate test/acpgate
- acp global flags apply
- acp initialize returns core caps
- acp package pieces
- acp request_permission method
- acp session/list
- acp stopReason mapping
- acp tool call mapping
- branch resolve timing
- CI gate set
- cmd/moca
- Config keys (camelCase)
- contextWindow floor 16384
- deep blue response band
- DESIGN.md retired and deleted 2026-10-13
- docs/reviews review passes
- edit result shape
- error taxonomy mapping
- eval bugfix scenario
- eval feature scenario
- eval largefile scenario
- eval plan scenario
- eval refactor scenario
- eval vague scenario
- eval web scenario
- evalstats aggregator
- everyday coding loop
- full-width dim rule
- git describe stamp
- .git/hooks/pre-push
- .github/workflows/ci.yml
- go test -race -count=1
- history replay
- insert-at-cursor paste
- install cross-compile targets
- install pinned version flag
- install --remove-data
- install replaces moca on PATH dir
- install --uninstall
- internal/acp
- internal/acp package
- internal/permissions
- internal/tui
- log.level
- main.go signals
- make build
- make release
- make test
- make vet
- mcp.servers.<name>
- message_style.go background detection
- moca acp command
- MOCA_API_BASE
- moca logout
- MOCA_UPDATE_API
- moca --version output
- mock SSE server
- no model selected yet notice
- no provider configured notice
- oauth policy record
- -p session slug
- paseo
- paste chips toggle alt+p
- phase branch / chore branch naming
- prune.go
- Purpose and non-goals absorbed from DESIGN.md
- Registry.DiscoverOllama
- Registry.Models sorted for the picker
- ResolveMode
- rev 21
- rev 23
- rev 25
- rev 26
- rev 27
- session append entry types
- slate user band
- status bar shrink order
- structural interface satisfaction
- Threat model absorbed from DESIGN.md
- -TUI maps errors via exitFor
- TurnEnd state after hard stop
- two-line status bar
- update.go
- User-Agent moca/<version>
- version stamp via -ldflags
- vs code acp extension
- web summary line
- web tool
- welcome product name color
- zed
- NewDefaultStore
- fakeCmds
- ACP JSON schema (protocol v1) vendored for conformance tests in internal/acp/testdata (data only)
- allowAll
- AllowAll
- keyboard_test.go
- example.com/codes
- example.com/fetcher
- example.com/inventory
- example.com/shop
- example.com/slug
- example.com/temp
- example.com/totals
- github.com/adeotek/moca
- Eval prompt: fix failing tests
- Eval prompt: add --json flag to inventory CLI
- Eval prompt: fix fail without editing name table
- Eval prompt: extract shared email validation
- Eval prompt: add input validation to temperature converter
- skills-cold/fixture/calc_test.go
- skills/fixture/calc_test.go
- sqliteSidecarGuard
- git flow (main PR-only, phase/N-slug or chore/slug, conventional commits)
- staleResults
- .setMode
- inject/fixture/calc.go
- LoadTrust
- post-phases (v0.3.0-beta)
- scriptServer
- skills-inject/fixture/calc.go
- AGENTS.md CLAUDE.md project instructions
- go install entry
- modelHard escalation
- Non-goals v1
- skills-inject-output/fixture/calc.go
- skills-inject-output/testdata/reference/calc.go
- skills-inject-paste/fixture/calc.go
- inject/testdata/reference/calc.go
- v0.6.0 (2026-10-16)
- Budget
- ThemeConfig
- Plan: "Allow for this session" approval answer
- skills_inject_c_test.sh
- PlanChanged
- Resumed
- SteeringApplied
- protocol_test.go
- overflowDir
- ResolveRecordNudge
- startProjectMCP
- inject/check.sh
- inject/fixture/NOTES.md
- skills_inject_check.sh
- replay/run.sh
- skills/check.sh
- fixture/.moca/skills/moca-testing/SKILL.md
- skills-inject/check.sh
- skills-inject/fixture/README.md
- skills-inject/fixture/scripts/bootstrap.sh
- skills-inject-output/check.sh
- skills-inject-output/fixture/scripts/bootstrap.sh
- skills-inject-paste/check.sh
- skills-inject-paste/fixture/scripts/bootstrap.sh
- reference/.moca/skills/moca-testing/SKILL.md
- skillgate/run.sh
- skills_inject_sourcing_test.sh
- v0.4.1-beta (2026-10-14)
- tui/log_test.go
- v0.2.0-beta (2026-10-09)
- evalcalc
- example.com/inject
- evalinj
- evalinj
- evalinj
- evalcold
- v0.7.2 (2026-10-21)
- expand
- v0.3.0-beta (2026-10-09)
- TrustStore
- ReadTracker
- askRootChecker
- boundedWriter
- fileAskChecker
- Compacted
- Note
- PlanChanged
- Resumed
- SteeringApplied
- StreamReset
- TextDelta
- ThinkingDelta
- UserRefusal
- Warning
- TestMain
- EnvError
- .Run
- fakeSnap
- LearnableCommands
- rootChecker
- CallResult
- anthropic provider
- resolveLong
- openai provider
- web/fixture/README.md
- caseAskChecker

## God Nodes (most connected - your core abstractions)
1. `model` - 156 edges
2. `newScript()` - 131 edges
3. `newTestModel()` - 78 edges
4. `textTurn()` - 77 edges
5. `testEnv()` - 66 edges
6. `§3.5 TUI (moca, no -p)` - 66 edges
7. `startClient()` - 60 edges
8. `Agent` - 60 edges
9. `Word` - 60 edges
10. `§3 CLI surface (as implemented)` - 60 edges

## Surprising Connections (you probably didn't know these)
- `Decisions (user)` --references--> `a()`  [INFERRED]
  docs/plans/2026-10-22-session-allow.md → internal/compact/cut_test.go
- `eight frozen tools list` --semantically_similar_to--> `Eight frozen tools`  [INFERRED] [semantically similar]
  README.md → internal/skills/builtin/self-knowledge/references/specs.md
- `Tests (TDD: write each failing test first)` --references--> `TestPermissionAllowSessionRemembersExactCommand()`  [INFERRED]
  docs/plans/2026-10-22-session-allow.md → internal/acp/permission_test.go
- `Tests (TDD: write each failing test first)` --references--> `TestSessionAllowScope()`  [INFERRED]
  docs/plans/2026-10-22-session-allow.md → internal/agent/sessionallow_test.go
- `Design` --references--> `build()`  [INFERRED]
  docs/plans/2026-10-22-session-allow.md → internal/agent/start.go

## Import Cycles
- None detected.

## Communities (652 total, 437 thin omitted)

### Community 0 - "go_pkg_strings"
Cohesion: 0.10
Nodes (25): liveResult, readArgs, readPending, usageRec, TestTeaAPI(), claudeServer, opencodeServer, request (+17 more)

### Community 1 - "newScript"
Cohesion: 0.09
Nodes (65): newScript(), startTest(), TestAbortWritesSyntheticResults(), TestFailedCallDoesNotCancelRest(), TestLengthStopTruncatedToolCall(), TestLoopReadEditVerify(), TestMaxStepsWrapUp(), TestMaxStepsWrapUpRepairsStrayCalls() (+57 more)

### Community 2 - "model"
Cohesion: 0.07
Nodes (19): OAuthProvider(), AbbrevEffort(), AbbrevHome(), providerNames(), providerRow(), modeNotice(), TestLoginRowForOllama(), TestResumeFlowAndReplay() (+11 more)

### Community 3 - "startClient"
Cohesion: 0.07
Nodes (80): RPCError, scriptServer, testClient, wireMsg, managedOptions(), TestACPBareRememberIsAnsweredNotForwarded(), TestACPRememberIntercepted(), TestACPRememberIsReservedWhenOff() (+72 more)

### Community 4 - "webFetch"
Cohesion: 0.10
Nodes (27): Spill(), cutWeb(), isAlnumByte(), isSpaceByte(), redirectSource(), TestRedirectSourceIsTheLastHop(), TestWebDataAttrsAreNotLinkAttrs(), TestWebHTMLConverters() (+19 more)

### Community 5 - "testEnv"
Cohesion: 0.05
Nodes (80): TestAllowSessionLearnsOnlyWhereTheRulesCan(), Builtins(), TestColdArmingNeedsAVerificationShapedCommand(), TestEditTrailingBlankPastEOFNoPanic(), TestRunRecordsExecutedCommands(), TestLs(), spilledPath(), TestSpillWritesFullOutput() (+72 more)

### Community 6 - "newAgentModel"
Cohesion: 0.07
Nodes (53): NewDefaultStore(), keyMsg(), TestExitCommandQuits(), TestCompactEscapeCancels(), TestCompactingBlocksRun(), TestCompactNothing(), TestEditItemShowsDiff(), TestThinkingLinePrintsBeforeTheResponse() (+45 more)

### Community 7 - "startTestWith"
Cohesion: 0.08
Nodes (49): startTestWith(), TestMCPNoSchemasInPromptAndZeroStarts(), TestModelCanReadGlobalSkillSupportFiles(), bigToolTurns(), summaryBodies(), TestAutoCompactionShrinksContext(), TestAutoCompactionSkipsWhenSummarizerUnavailable(), TestFailedCompactionStillCountsSpentUsage() (+41 more)

### Community 8 - "newTestModel"
Cohesion: 0.04
Nodes (95): Tests (TDD: write each failing test first), TestSelfManagedUnknownKeyRejected(), withPosition(), LoadProject(), ProjectConfigFile(), TestLoadProject(), TestLoadProjectMCP(), TestLoadProjectRejectsTrailingContent() (+87 more)

### Community 9 - "Word"
Cohesion: 0.12
Nodes (59): anyNonLiteral(), anyRoot(), buildWithWrites(), hasOpt(), longIs(), mutatorArgs(), mutatorPath(), mutatorVerdict() (+51 more)

### Community 10 - "§3 CLI surface (as implemented)"
Cohesion: 0.03
Nodes (59): Lookup (session.Find/FindForWorkdir): --resume last = the cwd's newest by mtime, i.e. FindForWorkdir (Find(dir, ref, workdir); stable sort; equal mtimes keep directory-name order); --resume <id8> = file-name suffix match (8 hex enforced; the match must belong to the cwd), a miss lists the 5 most recent ids; --continue = newest whose header workdir equals the canonical cwd (Abs + EvalSymlinks, the jail's comparison); headers are read from the first line only, /mode [default|auto|yolo|plan] — v0.4.0-beta; rev 27: the same four words as --mode — no argument prints the current state and the shift+tab cycle; an argument sets it — refused while running (truncated in source), cliModes = default|auto|yolo|plan (--mode must name one of the four, exit 2 otherwise; the pre-release --plan/--yolo/--no-yolo flags are removed — --mode plan ≡ --plan, --mode yolo ≡ --yolo, --mode default ≡ --no-yolo, and an explicit flag still overrides the config's yolo), §3 CLI surface (as implemented), Exit code 0 — completed, Exit code 1 — provider/runtime error, a final length/refusal stop (`response truncated (token limit reached)` / `model refused to answer`; the partial answer is still printed), or a failed run setup (data dir, session files, skills — agent.StartError), Exit code 130 — interrupted (SIGINT and SIGTERM cancel in-flight work), Exit code 2 — config, model or usage error (incl. unset env: vars, and an OAuth credential the user must fix with moca login — ErrInvalidGrant/ErrInvalidClient) (+51 more)

### Community 11 - "update/update.go"
Cohesion: 0.07
Nodes (42): envOr(), githubToken(), runUpdate(), Fast path: once graphify-out/graph.json exists, any codebase question runs graphify query directly on the merged graph - no re-extraction, no size gate, graphify clone <github-url> [--branch]: clones into ~/.graphify/repos/<owner>/<repo> and reuses existing clones on repeat runs, graphify merge-graphs: merge per-repo or per-subfolder graph.json files into one cross-repo graph, graphify reference: GitHub clone and cross-repo merge, ~/.graphify/repos/<owner>/<repo> clone layout; merged graph nodes carry a repo attribute so you can filter by origin (+34 more)

### Community 12 - "§3.5 TUI (moca, no -p)"
Cohesion: 0.04
Nodes (52): The activity row is replaced by the approval prompt, marked with a yellow ?, while a question is pending, or by the /-dropdown panel while one is open, Inline Bubble Tea v2 TUI (no alt-screen except the pager), /compact — manual compaction entry point in the TUI, /copy — copies the raw markdown of the last assistant response, /cost, /execute — plan execution entry point in the TUI (rev 21), /hard — modelHard + high ↔ saved pair, /help, /exit, /clear — help, quit, and start a new session (the open session refuses deletion — /clear starts a new one) (+44 more)

### Community 13 - "Create"
Cohesion: 0.06
Nodes (47): resolveResume(), Delete(), TestDelete(), TestListAll(), canonical(), Find(), FindForWorkdir(), Info (+39 more)

### Community 14 - "protocol.go"
Cohesion: 0.05
Nodes (56): AgentCapabilities, AuthMethod, AvailableCommand, AvailableCommandInput, AvailableCommandsUpdate, CancelNotification, ClientCapabilities, ClientSessionCapabilities (+48 more)

### Community 15 - "context.Context"
Cohesion: 0.07
Nodes (34): dropAdapter, fakeSum, cutUTF8(), isCont(), parseJudge(), TestParseJudge(), Lock(), Lock() (+26 more)

### Community 16 - "LoadIndex"
Cohesion: 0.17
Nodes (24): crashingServer(), TestCallTimeoutStopsWedgedServer(), TestHandshakeTimeout(), TestHTTPAbortedStreamReplayRule(), TestIndexSaveMergesConcurrentWriters(), TestSearchReportsUnsavedIndex(), TestStdioCrashMidCallReplayRule(), TestStdioDeadBetweenCallsRestarts() (+16 more)

### Community 17 - "permissions/shell.go"
Cohesion: 0.10
Nodes (26): Jail, resolveDeep(), within(), withinAny(), confinementShell(), mustAllow(), mustRefuse(), TestDashDashOperands() (+18 more)

### Community 18 - "auto_test.go"
Cohesion: 0.09
Nodes (61): TestPermissionsAllowKeepsFullRules(), TestShellAllowStillFiltersBareNames(), rules(), newAutoShell(), TestAutoFixRound1(), TestAutoFixRound2(), TestAutoGitSymlinkAndWritableAncestor(), TestAutoLadder() (+53 more)

### Community 19 - "AppendString"
Cohesion: 0.10
Nodes (27): scanner, span, AppendString(), concat(), nested(), nestedEntry(), objectHasKey(), objectValueSpan() (+19 more)

### Community 20 - "Part B semantic LLM extraction"
Cohesion: 0.08
Nodes (15): /graphify path, graphify reflect --if-stale, graphify-out/reflections/LESSONS.md, save-result feedback loop, graphify.build.build_merge, graphify.detect.detect_incremental, graphify-out/.graphify_detect.json, graphify-out/.graphify_extract.json (+7 more)

### Community 21 - "SPECS.md — moca normative spec (v0.5.0-beta)"
Cohesion: 0.05
Nodes (45): SPECS.md — moca normative spec (v0.5.0-beta), Scope notes — the path jail does not apply to MCP tools (a server's own scope is its own); MCP calls/results are ordinary tool calls/results for sessions, snapshots and compaction; no sampling, elicitation, MCP OAuth or legacy SSE (§16 non-goals), Built-in skill `create-command` - the rev-15 first-run seeding is gone; rev 27: nothing is written into the config dir; a previously seeded create-command.md is user-owned and wins over the skill, Built-ins follow the binary: a new build is a new content hash, so the next run extracts and reads a new directory (older ones stay until moca prune, sect 3); rtk and graphify are NOT built-in - they are seeded standard skills; the prompt's skills list drops the seeded tool skills rtk/graphify whose CLI is missing from PATH (agent.filterSkills + seededTools - matched by name for the builtin and global sources, so a user's own copy is hidden too; a trusted project's own skill is never filtered), and the token-discipline 'Prefer rtk-prefixed ...' line is included only when rtk is on PATH (PromptInput.RTK) - moca never advertises a tool the shell cannot run (the first live ship-gate run on a host without rtk derailed on `rtk: command not found` and never read the failing test); the skill's guidance is scoped to command output: for files it defers to read/search/ls, because shell reads (cat, rtk read, ...) do not satisfy the edit guard's tracked-read requirement (the third live ship-gate run drifted to shell cat/ls and never opened the failing test), Built-in skill `self-configure`, Built-in skill `self-knowledge` - with references/specs.md + references/readme.md: byte copies of docs/specs/SPECS.md/README.md, pinned byte-identical by TestSelfKnowledgeReferencesAreByteIdentical, so a docs edit must re-copy them in the same commit, Instructions - LoadInstructions: global AGENTS.md, then (trusted only) workdir/AGENTS.md, else workdir/CLAUDE.md; each capped at 32K chars with [... truncated at 32K chars], Prompt templates (phase 3; expansion forms since v0.4.0-beta are a bash parameter-expansion subset, a superset of pi's $ARGUMENTS) - skills.LoadPrompts(dirs) reads <dir>/<name>.md (optional frontmatter description/argument-hint; first dir wins a collision; sorted); ExpandPrompt(body, args) / the exported SplitArgs substitute: $1..$9, $@, $ARGUMENTS, $$ (a literal $), ${N} (any N >= 1 - ${10} works, while $10 stays bash: $1+0), ${@}, ${ARGUMENTS}, the default/alternate forms ${X:-word} / ${X-word} / ${X:+word} / ${X+word} (X = N, @ or ARGUMENTS; word may itself contain $1/${2}-style references, one level like bash; ${2-x} vs ${2:-x} differ because an explicitly quoted empty argument is an argument), and the slices ${@:N} / ${@:N:L} (${@:0} treats 0 as 1, pi parity); arguments split shell-like with SplitArgs (quotes group, "" is an empty argument); argument values are never re-expanded; any other ${...} - ${HOME}, an unterminated ${ - is kept verbatim, so shell snippets in templates survive (pinned: TestExpandPromptBraceForms, TestSplitArgs, TestExpandPrompt unchanged for $ARGUMENTS/$@/$1..$9/$$) (+37 more)

### Community 22 - "permission modes applyMode"
Cohesion: 0.09
Nodes (13): --approve flag, ask-write roots, auto mode risk table, --no-approve flag, path jail, permissions.auto.classifier, permissions.auto.model, phase 2 (+5 more)

### Community 23 - "PermMode"
Cohesion: 0.06
Nodes (41): ModeChanged, PermMode, badgeText(), firstLineOf(), TestGitStatusUsesNoOptionalLocks(), TestStatusWideRunesNeverExceedWidth(), FmtTokens(), FmtWindow() (+33 more)

### Community 24 - "§2 Build, test, verify"
Cohesion: 0.05
Nodes (44): One logfmt file per TUI / -p / acp process: $XDG_STATE_HOME/moca/logs/<YYYY-MM-DD>-<pid>.log (else ~/.local/state/moca/logs/), file 0600, dir 0700; the one-shot subcommands (login, logout, mcp, update, prune) do not log; files older than snapshot.retentionDays are pruned at start (0 keeps all), internal/update/ — moca update: GitHub release fetch, semver compare over the describe stamp, checksum verify, atomic binary replace (leaf — no internal imports), make build && bash test/acpgate/run.sh — the ACP gate, hermetic (no key, no network), §2 Build, test, verify, Go pinned from go.mod, no secrets required, The ship-gate package is behind //go:build shipgate, so CI's plain go test skips it, CI (.github/workflows/ci.yml) — gofmt check, go vet, go build, go test ./... -race on every PR/push to main, manually, eval scenario bugfix — two bugs behind failing tests, tests untouched (+36 more)

### Community 25 - "Event"
Cohesion: 0.15
Nodes (23): Retry, captureLog(), logPolicy(), TestProviderLogsFinalFailureAtInfo(), TestProviderLogsRetryAndResponse(), DefaultRetryPolicy(), RetryNotice, fail() (+15 more)

### Community 26 - "moca (minimal TUI coding agent)"
Cohesion: 0.18
Nodes (14): moca (minimal TUI coding agent), Skills and prompts from other tools: prompts dir <name>.md becomes /name in the TUI; $ARGUMENTS / $@ / $1..$9 expansion; Claude Code and pi files drop in unchanged, create-command builtin skill: create or update a moca slash command (a saved prompt template), Command file format: optional frontmatter (description shown in /help, argument-hint) then a prompt body with $$ARGUMENTS/$$@, $$1..$$9, $$$$ placeholders, Personal commands: <prompts dir>/<name>.md (/.config/moca/prompts or $XDG_CONFIG_HOME/moca/prompts); writes outside the workdir need user approval, Project commands: <workdir>/.moca/prompts/<name>.md, loaded when the project is trusted and shared with the repo, self-configure builtin skill: manage moca's own configuration, prompts, skills and installation at the user's request, A new prompt or skill is usable when the run ends (/, /help reload); a system-prompt listing refresh needs a new session; config changes apply to the next launch (+6 more)

### Community 27 - ".Run"
Cohesion: 0.08
Nodes (22): MCP, Permissions, TUI, v0.5.1-beta (2026-10-15), mutatesOnly(), normCommand(), TestVerifies(), verifies() (+14 more)

### Community 28 - "§11 Permissions"
Cohesion: 0.05
Nodes (40): Confinement - auto's path jail is Jail.WithWritable(permissions.TempRoots()...): the workdir plus the temp dirs (os.TempDir(), /tmp when different, $TMPDIR; TempRoots canonicalizes) are writable in addition to the workdir - built once as Shell.AutoView(jail) sharing the same Rules, so allow-always learned in auto applies after switching back to default too (TestAutoViewSharesLearnedRules, TestJailWithWritable); the read/write/edit tools run on this auto jail; outside the confinement they refuse exactly as in default mode (the ask-write prompt roots keep working); shell path arguments outside the confinement are an ask-every-time, not a refusal, GNU option parsing (follow-up 5b, auto_getopt.go) - cp, mv, ln, touch and mkdir are parsed with their real GNU option specs: a value-taking letter swallows the rest of its cluster, -- ends options, long-option abbreviations must be unique, optional-argument long options, permutation allowed; -t/--target-directory counts as a destination only for cp, mv and ln (every other mutator is positional-only, so chmod -t . asks again); unknown, ambiguous, repeated or valueless options are unsure; install stays unsure; also: git branch|tag --sort -l is unsure (creates a ref), touch -r FILE reads FILE (confined-read check) instead of treating it as written; unknown options elsewhere are a safe "not modeled -> unsure/ask" - Windows is the same on its best-effort path (TestShellWindowsRules, TestAutoWindowsMatchesUnix), Guards and parsing rules (the security fix rounds; pinned by TestAutoLadder, TestAutoRmGuardsProjectRoot, TestAutoTable, TestAutoFixRound1, TestAutoFixRound2, TestAutoGitSymlinkAndWritableAncestor, TestAutoWindowsMatchesUnix, TestDefaultModeHardening): after a cd/pushd/popd git is unsure (cwd unknown -> a relative operand cannot be confined); a redirect, tee, time -o or mutator operand with any .git path component (case-folded, any depth) asks every time; curl -f ... | bash asks for the bash end; a time -o FILE redirect is jail-checked like a redirect; rg --hostname-bin asks; make -f -, go -toolexec/-vettool/-exec, npm --script-shell and similar program-executing flags are unsure, Ladder (per classified command, after hard-deny/refused, which run exactly as in default mode; a rule match decides before the table - an explicit allow beats the auto table and an explicit deny beats it, TestAutoRulesStillDecide): builtins -> allow; read-only tools -> allow; project build/test -> allow; confined mutators -> allow iff every path operand is literal and confined, ask otherwise; the project-wrecking tier and the always-ask tier -> ask every time (CanAlways:false); everything else -> unsure -> the classifier (11.3); in default mode unsure is always empty, §11.2 Auto mode — confinement and the deterministic risk table, git push, git reset --hard, git clean, working-tree git checkout --/git restore (git restore -sS/--source --staged included), git rebase, git filter-*; curl/wget with upload/data/method/unsafe-output flags; plain curl/wget (downloads only) are allowed, but curl -w %output{%e2%80a6} and unknown wget options ask / are unsure; ssh scp rsync sftp nc ncat socat; npm publish, cargo publish, pip install without a confined --target, global installs (npm i -g, brew, apt*, dnf, yum); fetch-and-run (npx, bunx, pnpm/yarn dlx, uvx, pipx run, npm exec/x); docker run push, kubectl apply delete, terraform apply destroy; kill,kill,killall, systemctl, crontab; chmod -R; an interpreter with no script file or with -c/-e; any pipeline whose consumer is sh bash zsh python* node perl ruby reading a script from stdin -> ask (every time), Auto risk table class `build/test` - go build test vet fmt list env version mod tidy mod download; make (bare or a target); npm test, npm run *, pytest, cargo build test check, mise run * -> allow, Auto risk table class `builtins` - cd echo pwd test -> allow (+32 more)

### Community 29 - "TUI"
Cohesion: 0.05
Nodes (37): activity row, agent control surface, charm.land/bubbles/v2, charm.land/bubbletea/v2, charm.land/lipgloss/v2, /clear command, color profile conversion, /copy (+29 more)

### Community 30 - "§6 Providers & registry"
Cohesion: 0.05
Nodes (39): internal/provider/ — SSE reader, error taxonomy, catalog, 3 protocol codecs, replay helpers, retry wrapper, registry (incl. CheckCredential for model switches), providers.ollama (local provider, §6), internal/provider/catalog.go — Model{Provider, ID, Protocol, ContextWindow, MaxOutput, Cost(USD/M input/output/cacheRead/cacheWrite), ThinkingMode, ThinkingLevelMap}, Precedence: baseUrls[protocol] > baseUrl > built-in default, Default base URLs — anthropic → https://api.anthropic.com; openai → https://api.openai.com/v1; opencode-go → anthropic-messages https://opencode.ai/zen/go, openai-* https://opencode.ai/zen/go/v1, every request from every provider identifies the client as User-Agent: moca/<version>, Model overrides merge, not replace: a config entry for a catalog model changes only the fields it sets; a new model under a custom provider needs protocol + contextWindow (config validation); a new model under a built-in provider needs protocol (model-level or provider-level) — otherwise NewRegistry fails fast with the exact key path to add, Registry rule: an oauth provider resolves only for models whose protocol is openai-responses (the error names the protocol) (+31 more)

### Community 31 - "§5 Model catalog, effort, thinking"
Cohesion: 0.05
Nodes (38): --effort <level> — off|minimal|low|medium|high|xhigh|max (validated; clamped per model), Built-in catalog (prices/limits verified 2026-10-04, opencode-go expansion rows 2026-10-07 — re-verify against the sources listed in the file's comment when vendor pages change; cost = standard rates; the deepseek rows use the off-peak rate), Effort clamping (Model.ClampEffort): nearest supported level at or below the request; if none is below, the lowest supported; off on a model that cannot disable → lowest supported, Cost (Model.CostOf(usage)): Σ tokens × rate / 1e6, Level map basicThinking = {low, medium, high} (models whose gateway rejects off/minimal — kimi-k2.7-code, mimo-v2.6-*), Level map deepseekProThinking = {high, max}, Level map deepseekThinking = {low, high, max}, Level map glmThinking = {off:"none", low, medium, high} (+30 more)

### Community 32 - "newFakeAS"
Cohesion: 0.09
Nodes (30): audContains(), b64d(), jwksKey(), browse(), firstURL(), newFakeAS(), runLogin(), TestCallbackPageReportsFailure() (+22 more)

### Community 33 - "§4 Config"
Cohesion: 0.05
Nodes (35): internal/config/ — version, paths, JSONC pre-pass, typed config + validation, env indirection, AppendString (JSONC-preserving edit), Resolution order: an explicit flag wins; else a trusted project file's permissions.defaultMode; else the user config's resolved mode (permissions.defaultMode, else the legacy yolo flag, else default), §4 Config, Path ~/.config/moca/config.jsonc ($XDG_CONFIG_HOME honoured); data dir ~/.local/share/moca ($XDG_DATA_HOME), Custom providers (vLLM/LM Studio/Ollama…): any name + baseUrl + protocol + models with contextWindow; usable through the same registry path, Defaults: built-in providers anthropic/opencode-go/openai with auth: api_key and env:ANTHROPIC_API_KEY / env:OPENCODE_API_KEY / env:OPENAI_API_KEY, Defaults: permissions.defaultMode empty (→ mode default unless legacy yolo), permissions.auto.classifier on; mcp.idleTimeout 600; context 16384/20000/40; snapshot.retentionDays 30; web.search.provider tavily; tui.notify osc9; yolo false, shell.allow defaults to the 32-entry DefaultShellAllow list (internal/config/config.go) (+27 more)

### Community 34 - "ParseFrontmatter"
Cohesion: 0.25
Nodes (8): TestBodyOffset(), BodyOffset(), closingQuote(), ParseFrontmatter(), scalar(), TestFrontmatter(), TestFrontmatterBlockAndQuoteEdges(), TestFrontmatterBlockIndicatorsAndComments()

### Community 35 - "§9.5 ACP server (moca acp)"
Cohesion: 0.05
Nodes (36): moca acp — serve the Agent Client Protocol on stdio (Zed, VS Code, Paseo, …) — §9.5, availableModes: default, auto, yolo, plan (plan = SetMode(default) + SetPlan(true); any other also clears plan); the initial currentModeId is the resolved mode (--mode > trusted project defaultMode > config), Config is loaded once at startup (a parse failure exits 2); a missing model is not fatal — session/new reports it; the file log starts as for the other modes (§13.5; metadata only: the method, the session id8, trust decisions, session open/close/delete and prompt-failure lines — never prompt text, file contents, command text or keys), Errors: malformed JSON line -32700 (id null; the connection stays up); unknown method -32601 (notifications ignored, logged); bad params / bad cwd -32602; unknown sessionId -32002; request cancelled -32800 (a cancelled session/prompt answers stopReason: "cancelled" instead); auth/model missing or unusable -32000; internal failure -32603 with the error text, Gates: bash test/acpgate/run.sh drives the built binary (child process, scripted provider, two processes on one data dir): text turn, approval, cancel, stdin EOF exit 0 with an empty stderr, reload + replay from a second process, close, delete, and a scan that the API key never reaches stdout, stderr or the debug log; the Zed and ACP TCK manual gates are open items (§15; both are outside CI), Non-goals: client fs/terminal delegation; client-supplied MCP servers; image/audio prompts; authenticate/logout flows; additionalDirectories; terminal tool content; elicitation; plan updates; TUI built-ins over ACP; steering a running prompt; every unstable method (session/fork, providers/*, nes/*, document/*, mcp/message); the schema is pinned under internal/acp/testdata/ (SOURCE.md records the tag and the feature gates: messageId and usage_update are in the stable schema), moca acp serves the Agent Client Protocol (v1, protocolVersion: 1) on stdin/stdout so ACP clients (Zed, VS Code ACP extensions, Paseo) drive moca; editor config is a command line: "agent_servers": {"moca": {"command": "moca", "args": ["acp"]}}, Permissions (session/request_permission): toolCall {toolCallId: <current tool call>, title: "<Kind>: <Subject>", rawInput: {detail}}; options allow-once ("Allow once", allow_once), allow-always ("Always allow", allow_always, only when the question can be remembered — never for rm) and reject_once ("Deny"); selected maps to allow once / allow always / deny; cancelled, an error response or a closed connection map to deny; requests never time out: session/cancel and stdin EOF are the only ways out — a timeout would silently deny under clients without auto-accept (+28 more)

### Community 36 - "approvalPanel"
Cohesion: 0.13
Nodes (13): Design, TestAllowAlwaysIsIgnoredWhenTheQuestionForbadeIt(), TestUnrecognisedAnswerFailsClosed(), AutoAllow(), Answer, Question, TestAnswerString(), TestQuestionSessionKey() (+5 more)

### Community 37 - "opencode-go provider"
Cohesion: 0.11
Nodes (17): deepseek-v4.1-flash, deepseek-v4-flash, deepseek-v4-flash-vision-exp, deepseek-v4-pro, glm-5.3, glm-5.3-flash, grok-4.7, kimi-k2.7-code (+9 more)

### Community 38 - "DefaultTheme"
Cohesion: 0.06
Nodes (40): TestThemeForUserLayer(), themeFor(), TestApprovalPromptSanitized(), TestWelcomeLines(), approvalPanel(), btoi(), TestApprovalShowsItsOptionsOnce(), diffPreview() (+32 more)

### Community 39 - "Manager"
Cohesion: 0.16
Nodes (9): Rank(), TestRank(), words(), Manager, startFailClass(), Hit, Index, IndexEntry (+1 more)

### Community 40 - "sseServer"
Cohesion: 0.14
Nodes (39): anthropicStream(), TestAnthropicCorruptStreamIsAnError(), newAnthropic(), keyCred(), mustJSON(), sseServer(), TestAnthropicBodyHygiene(), TestAnthropicBudgetAndNoCache() (+31 more)

### Community 41 - "release v0.1.0.alpha"
Cohesion: 0.09
Nodes (24): Anthropic API-key-only policy gate, token-denominated context manager, cross-provider history transform, custom providers via baseUrl and protocol, graphify allowlist and guidance skill, Makefile and mise mirrors, OpenAI subscription OAuth PKCE, long tool outputs saved to file (+16 more)

### Community 42 - "ollamaBaseURL"
Cohesion: 0.18
Nodes (9): ModelOverride, CostConfig, ProviderConfig, costM(), TestCostOf(), normalizeOllamaURL(), ollamaBaseURL(), TestNormalizeOllamaURL() (+1 more)

### Community 43 - "permissions/shell.go"
Cohesion: 0.13
Nodes (23): rtkRunTarget(), rtkShellString(), rtkSub(), rtkTarget(), caseFolds(), foldWithin(), headName(), literalSafe() (+15 more)

### Community 44 - "session"
Cohesion: 0.16
Nodes (6): session, newMessageID(), newSession(), summaryOf(), summaryText(), TestSummaryText()

### Community 45 - "secret.go"
Cohesion: 0.15
Nodes (19): literalSecretFields(), Embedded(), Key(), Match(), onlyVarRef(), schemeProse(), TestEmbedded(), TestKey() (+11 more)

### Community 46 - "RememberSession"
Cohesion: 0.27
Nodes (8): RememberSession(), TestRememberSessionCancelledContextDenies(), TestRememberSessionNilStaysNil(), TestRememberSessionNoSession(), TestRememberSessionRecordsOnlySessionAnswers(), TestRememberSessionTiers(), TestWebFetchRedirectHopToSessionHostStillAsks(), TestWebFetchSessionAllow()

### Community 47 - "Release - build packages, publish GitHub Release"
Cohesion: 0.19
Nodes (16): Release - build packages, publish GitHub Release, Download artifacts with merge-multiple, Upload package artifacts, Asset naming moca-<version>-<os>-<arch>.tar.gz|zip, build job - package matrix targets, Build matrix (linux/amd64, linux/arm64, windows/amd64), sha256sum checksums.txt generation, gh release create/upload (clobber updates) (+8 more)

### Community 48 - "Sanitize"
Cohesion: 0.12
Nodes (7): dayDiff(), TestDayDiff(), lineCount(), hasControl(), Input, padGuard, paste

### Community 49 - "DataDir"
Cohesion: 0.17
Nodes (15): dirTrusted(), runConfig(), runConfigCheck(), main(), TestAgentReachesItsOwnConfig(), TestPathsHonourXDG(), TestStateDir(), AuthFile() (+7 more)

### Community 50 - "schema_test.go"
Cohesion: 0.17
Nodes (17): node, TestConfigSamplePrintsTheTemplate(), check(), jsonType(), loadDefs(), resolve(), TestConformRejectsBadShapes(), typeOK() (+9 more)

### Community 51 - "Import direction rule"
Cohesion: 0.07
Nodes (25): CommandChecker 4-tuple (need, askEveryTime, unsure, err), Env.Judge (auto mode classifier), internal/acp, internal/agent, internal/config, internal/config/testdata/example.jsonc, internal/llm, internal/mcp (+17 more)

### Community 52 - ".Session"
Cohesion: 0.13
Nodes (43): startWith(), autoBody(), autoFile(), editArgs(), hasNote(), managedConfig(), runTool(), seedAutoSkill() (+35 more)

### Community 53 - "§13 Sessions & snapshots"
Cohesion: 0.07
Nodes (30): Logging - the judge's own only log line is slog.Debug('judge', decision, ms, cached[, error='provider']): decision, duration, cache-hit and the error class only - never the command or the reason text (the metadata-only rule; the analyser's refusals log no text either); its usage/cost book into the session's Status() totals (a.usage.Add(resp.Usage) + CostOf, same goroutine as the tools so no extra lock) - one extra small request per new unsure command (TestJudgeAccountsCost), §13.5 Diagnostic log, Architecture: the standard log/slog default logger; internal/applog (imported only by cmd/moca) installs a discard handler first, then the file handler; other packages call slog directly - no internal import edge, Bounds: records <= 16 KB; the file stops at 10 MB after one `log truncated: size cap reached` line; a write error turns logging off silently; an open failure prints one stderr warning and the run continues; logging never changes an exit code, debug adds: effective config keys; each request/response (model, counts, ttfb, duration, stop, usage); each step's tool names; each tool call (duration, is-error, bytes); spills; approvals asked (command name + answer); auto-mode classifier verdicts (judge lines: decision/duration/cache-hit/error class - never the command or the reason text); analyser refusals (no text); web fetch/search metadata; each MCP call, info level logs: process start/exit; session start/resume (id8, workdir, model, effort, yolo, plan, mode); provider retries (attempt, wait, cause class) and final failures; run end (outcome, steps, duration, usage, cost); nudges; user-visible warnings; compaction, overflow recovery, elision; recovered tool panics with stack; spill failures; MCP start/ready/exit (status only)/idle stop/start failure/dead-expired-timed-out calls; setup-mode first session; login results, Level: config log.level - info (default) | debug | off; MOCA_LOG overrides it for one run (invalid -> exit 2); off creates nothing, Metadata only, every level: never prompts, model output, tool inputs/outputs, file contents, shell command text, search or web-search queries, MCP server stderr or credential values; session-scoped lines carry session=<id8> - the transcript holds the content (+22 more)

### Community 54 - "writeCfg"
Cohesion: 0.10
Nodes (34): TestACPBadFlags(), TestACPServesOnlyJSONRPC(), fakeCompletions(), isolate(), TestExitCodes(), TestInterruptedExit130(), TestLoginLogoutCLI(), TestMCPImportCLI() (+26 more)

### Community 55 - "§14 Agent loop"
Cohesion: 0.07
Nodes (28): Lifecycle (Manager) — zero servers start at session start (index present or not), and no server process is spawned except through the manager's ensure (start + initialize + tools/list), which only search (index miss), describe, call and `moca mcp index` reach; per server: search serves from a valid index entry or starts the server once to (re)index; describe/call ensure (start + initialize + listTools + index refresh) on demand; the idle timer (mcp.idleTimeout, default 600 s) arms at start and after every call; it stops a server only with no call in flight (busy counter under the server lock guard, a call holds busy across the RPC and re-arms the timer at its end — a timer firing between two calls can never kill one); a stopped server stays discoverable via the index and restarts on demand, Registry.Run counts identical failing calls (tool + compacted input + output): from the 3rd, the result carries `[hint: this exact call has failed N times with the same result — retrying will not help; change approach, or stop and explain what is blocking you]`; MaxRepeat feeds the agent's stop at 5 (§14), Plan mode (rev 20) — with Env.Plan set: write/edit reach only <workdir>/docs/plans/*.md (non-.md and everything else → `refused: plan mode only writes docs/plans/*.md …`; the ask-write roots stay closed — nothing is asked), and a successful plan-file write/edit sets Env.PlanWrote; everything else stays available, under the normal rules: read/search/ls/web as usual, and shell + mcp calls under the permission ladder / the proxy's read-only/approve gating (Ben's follow-up, 2026-10-09: planning needs git log, builds and docs lookups); the failing-test investigation banner/refusal is suppressed while plan mode is on (plan runs inspect, they don't fix); the write confinement is not a permission: --yolo does not lift it, Plan mode (rev 20; rev 26/27 for the mode pairing and the flag) - --mode plan (TUI/-p) or the TUI /plan toggle sets Env.Plan (the tools enforce it, sect 10); SetPlan writes a permission_mode entry ({yolo, plan, mode} - both flags + the mode) and emits PlanChanged (the TUI refreshes its blue Plan badge and prints the notice); never restored on resume - the resuming process's flag decides, like the mode - and combined only with default (rev 26); the prompt side - the request-only envelope and the one-bounded-nudge guarantee - is sect 14; edit applies the plan-zone check before the ask-write approval, like write, so plan mode never asks; a run ended by the maxSteps wrap-up (tool-less, so it cannot write the file) emits the same deliverable-missing warning, Trust store - ~/.local/share/moca/trust.json (0600, atomic write; keys are canonical paths): Lookup/Set; trust gates <workdir>/.moca/** and AGENTS.md/CLAUDE.md; -p rule: explicit flag > yolo > no (saved decisions are ignored in -p); the TUI uses the saved decisions and asks once on first open of an untrusted workdir with project resources, §14 Agent loop, Overflow recovery: a turn whose stream fails with provider.ErrContextOverflow, or stops length with the input within reserve of the window, becomes an internal errOverflow (nothing persisted) and is retried once after a compaction as a fresh turn; a failed compaction or a second overflow surfaces the original error; Compacted{TokensBefore,TokensAfter} event; the TUI prints `compacted: N -> M tokens` and sets a `compacting...` transient while a manual /compact is in flight, Summary request: the cheap model (not /hard), effort minimal clamped, NoCacheWrite, max_tokens = min(4096, MaxOutput), no tools; the serialized input ([User]: / [Assistant thinking]: / [Assistant tool calls]: name(k=v,...) / [Tool result]:, tool results capped at 2K chars, header COMPACTION NOTE (lossy)) is capped to the summarizer model's window budget at 3 chars/token (CapChars drops the oldest lines with [... earlier history omitted]); a summarizer that still overflows is retried once with the newer half of the payload; tokens spent by a compaction that then fails (e.g. the first of two split-turn calls) are still booked on an error entry's usage/cost; output format: Goal / Constraints & Preferences / Progress (done / in-progress / blocked) / Key Decisions / Next Steps / Critical Context + cumulative <read-files>/<modified-files> lists (a repeated compaction updates the previous summary - find-style continuity from firstKeptEntryId, lists accumulate) (+20 more)

### Community 56 - "Store"
Cohesion: 0.12
Nodes (14): Store, newHostID(), validateAPIKey(), File format, Rules, Steps, Where commands live, How (+6 more)

### Community 57 - "run"
Cohesion: 0.13
Nodes (23): runACP(), printUsage(), startLog(), loginAPIKey(), runLogin(), runLogout(), yesNo(), run() (+15 more)

### Community 58 - "Start"
Cohesion: 0.06
Nodes (38): setup, promptDirs(), planMode(), filterSkills(), GitState(), Platform(), TestFilterSkillsDropsSeededSkillsWithoutTheirCLI(), headlessRun() (+30 more)

### Community 59 - "§15 Verification status & known items"
Cohesion: 0.07
Nodes (28): Gating — a call runs without asking iff the tool declares readOnlyHint == true && destructiveHint != true, or it is named in mcp.servers.<name>.approve (the list may contain "*" to trust the whole server), or it was allowed-always earlier in this session; otherwise the asker is consulted (tools.Question{Kind:"mcp", Subject:"server/tool", Detail: <args>, CanAlways:true}); deny or no asker (-p without --yolo) → error `refused: <server>/<tool> is not marked read-only and is not in mcp.servers.<server>.approve; the user did not approve it`; AllowOnce runs once; AllowAlways adds a session allow and (TUI ctrl+a) persists the tool name into mcp.servers.<server>.approve via config.AppendString, updating the /clear restart copy too; no name-pattern heuristics; yolo needs no MCP-specific code (Env.Ask is tools.AutoAllow → every gated call runs, also in -p), ACP gate (2026-10-14, re-run green 2026-10-15): make build && bash test/acpgate/run.sh green on the built binary (text turn, approval, session/cancel, stdin-EOF exit 0 with an empty stderr, session/load replay from a second process on the same data dir, close, delete, API-key leak scan over stdout, stderr and the log), Install-script gate (2026-10-09, scratch moca_install_gate/) - against a local fake release server: install.sh fresh install (--dir), re-run updates in place, no--dir updates the moca on PATH in place, --version 9.9.9 (no v) hits the /releases/tags route, a checksum mismatch exits non-zero with nothing replaced and no temp leftovers, and a clean re-run recovers; install.ps1 ran functionally under pwsh with only its Windows-API bits forked out, Live gates (2026-10-05, real OpenCode key): phase-1 smoke (11/11) and phase-2 gates stay green; phase 3 live: minimax-m3 (anthropic-messages, thinking + tool calls) -> /model -> glm-5.3-flash (openai-completions) -> continue -> /model back -> continue, all clean (no 400), thinking blocks stamped opencode-go/minimax-m3 with signatures, 2 model_change entries, real usage/cost in the bar, Phase-4 live gates (2026-10-05, real OpenCode key, contextWindow: 32768 declared on glm-5.3-flash as the small-window stand-in): TUI long session on the real repo - status bar climbed to 56%, compacted: 31199 -> 14776 tokens, then compacted: 26009 -> 8312 tokens, context dropped to 33%, and after both compactions the model recalled all three files (in order) - session file: 2 compaction entries, cumulative readFiles, structured summaries; loop guard reproduced live on an indivisible batch (two 50K-char reads in one turn -> guard error naming both entries, exit 1 - by design); abrupt mid-tool shell sleep 90 stop + --continue -> resumed banner (3 messages), follow-up request served, session file holds exactly one synthetic interrupted result, Phase-3 gate (2026-10-05) - mock-driven TUI session in a real tmux terminal (scripted openai-completions server): status bar from the first frame (repo | main | fake/m | off | ctx 128k), branch switch via the shell tool reflected in the bar, streamed text committed to scrollback, numbered thinking/tool items, /help /cost /model (list + cross-model switch) /effort (clamp note) /hard (both directions restore) /copy /show /compact, missing-key switch refused naming OPENAI_API_KEY with the model unchanged, no-enhanced-keyboard hint + alt+enter newline, 120-line bracketed paste -> chip -> alt+p expand -> send with all 120 lines intact, steering verified in the captured request payload, approvals (a ran; A appended python3 to shell.allow with the file's comment and trailing comma byte-preserved + .bak; rm prompted without [A] and deny kept the file), /yolo on/off incl. a command running with no prompt and the 60-column bar keeping YOLO, !touch local, /undo removing a created file, esc interrupting a running tool ([interrupted], queued steering returned to the editor), /clear starting a new session file; fixed two real bugs (event-pipe deadlock on /yolo; styled prints mangled by Sanitize), Phase-5 gate (2026-10-06, scratch moca_gate5/, real MCP servers, scripted openai-completions provider) - moca mcp index against the real @modelcontextprotocol/server-filesystem (stdio, 14 tools) and the real context7 server over streamable HTTP (2 tools); scripted -p request capture: first request carries exactly the 7 frozen tools, no server tool name/schema anywhere in it (hits appear only in the tool result of the next request); search-only run: index-served with zero server processes ever observed; describe+call run: the process appears during the call and is gone when moca exits (Close); tmux TUI: 0 at start, 0 after a search, 2 during/after a call, 0 after 30 s idle (idleTimeout: 30); gating ladder in -p: no approve -> refused; approve write_file -> written; --yolo -> written; live HTTP search/describe/call through context7; import: real host configs with no MCP servers -> correct nothing-to-import, then a scratch-HOME run with realistic claude/opencode/pi files: preview + write rewrote every secret to env:MOCA_MCP_* (no ghp_/Bearer/BSA_/lin_ value anywhere; PLAIN_VAR stayed literal), skipped sse/disabled/duplicate names with notes, and the second run re-parsed the config and skipped everything as already configured, Phase-5 live legs closed (2026-10-06, real OpenCode Go glm-5.3-flash, repo .env key; scratch moca_gate5live/) - a real model drove the full ladder in -p: mcp search read file (14) -> mcp describe fs/read_text_file -> mcp fs/read_text_file, returning the file's contents with zero server processes before/after; gating: write_file -> refused (no file created), then with approve write_file the model wrote out.txt = gate-ok and read it back to confirm; a model-free moca mcp index reap check observed 0 processes within 1 s; a TUI-driven real-model MCP ladder pass remains untested (+20 more)

### Community 60 - "v0.1.0-alpha (2026-10-07)"
Cohesion: 0.12
Nodes (16): CLI, Context & sessions, External tools & the skills ecosystem, Front-end parity (#43, #44), MCP, OAuth (subscription login), Providers & protocols, Self-managed project skills (+8 more)

### Community 61 - "moca — MO Coding Agent"
Cohesion: 0.09
Nodes (22): AutoConfig, ContextConfig, LogConfig, PermissionsConfig, ShellConfig, SkillsConfig, SnapshotConfig, WebConfig (+14 more)

### Community 62 - "Message"
Cohesion: 0.05
Nodes (60): Status, TurnEnd, Kind, Result, readWindow, summarizerUnavailable(), callKey(), shouldElide() (+52 more)

### Community 63 - "anthropic-messages adapter"
Cohesion: 0.07
Nodes (21): anthropic-messages adapter, anthropic stop-reason map, anthropic stream events, anthropic system as one text block, anthropic thinking capture, common post() wire contract, completions stream chunks, config.Standardize (+13 more)

### Community 64 - "context.Context"
Cohesion: 0.19
Nodes (19): builtinHash(), ExtractBuiltins(), extractBuiltins(), PruneBuiltins(), pruneBuiltins(), exists(), mkTree(), pruneFixture() (+11 more)

### Community 65 - "§6.5 Subscription OAuth (phase 7)"
Cohesion: 0.08
Nodes (25): The OAuth row starts provider.Login in a command goroutine with the output bridged into the scrollback through the event pipe (so progress lines and the completion can never swap order); while it runs the panel says so, enter with text in the input box hands the pasted redirect URL/code to the flow (a blocked pipe write is spawned, released when the flow ends or is cancelled), and esc cancels the flow; after a successful OAuth login whose provider is not configured auth: "oauth", the wizard asks [y/N] to flip providers.<p>.auth (config.SetString, comment-preserving) and reloads the config so the next /clear (or restart) picks the new mode up; n/esc leaves it unchanged, moca login stores the credential in ~/.config/moca/auth.json (§6.5), moca login <provider> — store a credential for the provider (§6.5), For api_key providers resolution is store-first: a key stored in auth.json (§6.5, /login or moca login) wins over the config's env:VAR reference; the error when neither exists names the variable and both login entry points, Validation: model ids must be provider/<model> (split at first /); provider must exist; an apiKey literal → rejected (must be env:), an omitted apiKey → accepted (a store-only provider, resolved from /login); custom providers need baseUrl(+baseUrls) and protocol; model-level protocol must be one of the three; custom models must declare contextWindow; contextWindow < 16384 refused; context values and retentionDays >= 0 checked; each MCP server must have exactly one of command/url; mcp.idleTimeout must not be negative (0 = the default 600); tui.notify must be osc9, bell or off; auth: "oauth" is rejected for a provider the §3 policy gate refuses (anthropic — the error carries the recorded reason, §6.5) and for any provider without an implemented OAuth flow (config.OAuthProviders, pinned against provider.oauthProviders by a test — "no OAuth support for this provider; set \"api_key\""), anthropic — api_key only (Anthropic does not permit third-party clients to use Claude subscription OAuth; providers.anthropic.auth: "oauth" is a config error naming the record), Known limitation: the client_id is registered per login (no shared app client id yet); no Anthropic OAuth by policy, Credential store (internal/provider/auth.go) — ~/.config/moca/auth.json (file 0600, dir 0700; the write forces 0600 even when a stale tmp file exists, and the rename deliberately replaces a symlink rather than following it) (+17 more)

### Community 66 - "release v0.4.0.beta"
Cohesion: 0.08
Nodes (33): redaction net for secrets and prompt content, approval prompt as arrow-key picker, auto classifier fail-closed second opinion, auto mode risk table and confinement, bounded diagnostic log, hard-deny class (sudo dd mkfs), MOCA_LOG=debug per-run detail, --mode flag surface (+25 more)

### Community 67 - "§2 Package layout (implemented)"
Cohesion: 0.08
Nodes (23): Built-ins - internal/skills/builtin/** embedded (embed); ExtractBuiltins(dataDir) materializes them into <data>/builtin-skills/<content hash> (unique temp dir + rename; a lost rename race is success) so the model can read them (builtin dir is a read-only jail root); extraction is content-hash keyed, so changed built-in content re-extracts on the next run (MapFS test); currently exactly three (TestExtractBuiltins) - see below, cmd/moca/ — main.go (entry, signals), cli.go (flags → Options), oneshot.go (-p agent path), tui.go (TUI path + trust), update.go (moca update), prune.go (moca prune), agent imports everything, cmd/moca imports agent + config + tui + acp + mcp + update, llm/config/update import nothing internal, mcp imports tools + config + llm, tools/permissions/session/skills/compact are mutually independent; structural interfaces, provider imports llm + config (+15 more)

### Community 68 - "Run"
Cohesion: 0.07
Nodes (20): Event, Run(), newAsker(), newEventPipe(), TestEventPipeBuffersUntilStarted(), TestEventPipeNonBlockingAndFIFO(), TestPipeSendKeepsOrderWithEvents(), TestTrustPromptCancel() (+12 more)

### Community 69 - "judgeAgent"
Cohesion: 0.25
Nodes (7): judgeAgent(), TestJudgeAccountsCost(), TestJudgeCachesAllowOnly(), TestJudgeFailsClosed(), TestJudgeNoCacheForAskOrError(), TestJudgeRequestCarriesNoToolOutput(), TestJudgeTimeoutFailsClosed()

### Community 70 - "§10 Tools (eight, frozen)"
Cohesion: 0.10
Nodes (23): Approvals (§10 default ladder; §11.0 for the layered-rules/auto versions): allowlisted-but-unapproved → ask (AllowAlways persists via Commands.Allow → Rules.Learn); rm → ask every time (CanAlways:false, never allowlistable); a write/edit outside the workdir under the global prompt templates asks once, never persistently (Kind:"write", CanAlways:false); Ask == nil (a -p run) = deny, Overflow files (rev 21) — a shell, search, web fetch or mcp call output over its cap is cut for the model, and the full text is written to <data>/overflow/<process dir>/<id8>-<kind>-*.txt (0600, dirs 0700; tools.Spill; the process dir is 16 random hex chars chosen at session start); the result ends with `[full output (N lines, size) saved to <path> — read it with offset/limit or search it; do not re-run the command to see it]`; only this process's dir is a read-only jail root — other sessions' saved outputs (other projects, possibly secrets) stay unreadable; a resumed session gets a new dir; entries older than snapshot.retentionDays are removed at startup; no SpillDir (tests) or a failed write → the cut output stands alone; MCP call results cap at 12K (was 30K), Run state (rev 21, tools/runstate.go) — Env.BeginRun (called by Agent.Run) resets PlanWrote, Edited, Unverified, MaxRepeat and the failure counts; a successful non-plan write/edit sets Edited, names the file in Unverified unless it is a docs file (.md .markdown .txt .rst .adoc), and clears the failure counts (the state they ran against is gone); a shell command verifies when any pipeline segment's command is not inspect-only (cat ls head tail grep rg find echo git sed awk diff …, after VAR=/rtk/sudo/time prefixes) — it clears Unverified, The shell tool's checker is the 4-tuple Check(cmd) (need, every, unsure []string, err): need = ask-once, every = ask-every-time, unsure = the auto table's judge tier (auto mode only) — the shell tool passes unsure to env.Judge when one is installed (an allow clears the tier, anything else lands in need), then walks need and every as before, edit — the edit ladder (§5): exact unique match → replace; several exact matches → error listing the lines + suggests replace_all; replace_all replaces every exact occurrence (never falls back to fuzzy); no exact match → one whitespace-insensitive match accepted and new_string re-indented to the file's indentation; none/ambiguous → model-facing error (mentions N| prefixes when the text looks like pasted read output); empty old_string → "use write"; identical strings → error; CRLF and BOM preserved; a whitespace-fallback match whose trailing blank old line lands on the phantom element after the file's final newline keeps that element out of the hunk (rev 21 fix: formatDiff indexed past EOF and panicked after the write had landed); non-UTF-8 refused; result: unified diff + `changed lines …; file now N lines`; Summary `path [+a −d]`; while a failing test run is unresolved it refuses before applying, naming the missing investigation step, ls — one directory level, sorted; dirs /, symlinks @; 1000-entry cap; hidden entries only with hidden:true, read — N|content lines + [lines A-B of TOTAL] footer (+ `use offset=B+1 to continue` when more remain); without limit the window is 400 lines (rev 21 — the footer then says `use offset=N to continue, or search for what you need`); limit raises it up to 2000 lines / 50K chars per call; lines >2000 chars truncated with [… line truncated]; NUL in the first 8K → refused with size + MIME guess; a directory → "use ls"; non-regular files (FIFOs, devices) and files >256 MB are refused; the window is streamed with a bounded reader (no whole-file load); every successful read records mtime/size/sha256 (ReadTracker, streamed hash); reading any *_test.go sets Env.TestSeen and clears Env.FailingTest, search — RE2 regex; respects .gitignore/.ignore from the workdir down to the search path (deeper files override); files >4 MB are skipped with a visible note; skips hidden + binary; glob matched against the basename (or the relative path when it contains /); 100 hits shown with a visible note ([… 100 of N hits shown …]), up to 5000 collected and spilled to an overflow file (rev 21); files_only; output path:line:text (text cut at 300 chars); sets Env.Searched (investigation protocol) (+15 more)

### Community 71 - "system-prompt.md"
Cohesion: 0.08
Nodes (19): Environment, Long outputs are cut and the full text saved to a file named in the result: read or search that file instead of re-running, mcp tool: search, then describe, then call the listed MCP servers, shell is stateless: every call starts in the workdir; use cd dir && cmd in one call, Token discipline, Tools, web tool: op fetch (markdown|text|html) or op search; use instead of shell curl for pages and web lookups, Workdir jail: all relative paths resolve to {{workdir}}; file tools cannot leave it (reading user-level config/skills dirs allowed) (+11 more)

### Community 72 - "compaction budgets"
Cohesion: 0.26
Nodes (16): FindCut(), a(), call(), res(), TestFindCutKeepsRecentAtValidPoint(), TestFindCutNeverBetweenCallAndResult(), TestFindCutNothingToDo(), TestFindCutSplitTurn() (+8 more)

### Community 73 - "Eight frozen tools"
Cohesion: 0.11
Nodes (11): config.EnvRefs, edit tool, Eight frozen tools, internal/tools, ls tool, markdown-lite renderer, read tool, run state (tools/runstate.go) (+3 more)

### Community 74 - "Start wiring"
Cohesion: 0.10
Nodes (11): docs/specs/SPECS.md, internal/skills, phase 6, rules engine, self-knowledge skill, shell.allow config key, shell analyser (parsed commands), skills.Discover (+3 more)

### Community 75 - "release.yml manual dispatch only"
Cohesion: 0.10
Nodes (17): auth.json credential store, auth.json sections, checksums.txt, internal/update, login --api-key flag, login flow, login --no-browser flag, /login wizard (+9 more)

### Community 76 - "fakeOllama"
Cohesion: 0.22
Nodes (4): modelFromShow(), countingTransport, fakeOllama, ollamaShow

### Community 77 - "§3.5.1 TUI additions (rev 14)"
Cohesion: 0.09
Nodes (22): Schemas are frozen since phase 2 (internal/tools/testdata/schemas.golden.json; changing one is a v2 discussion — the web tool was added as rev 19); eight tools: read, write, edit, shell, search, ls, web, mcp, Rebuild (session.Messages) - with a compaction entry present, the output starts with a user message '[Summary of earlier conversation]\n<summary>' (a kept user message merges into it) followed by entries from firstKeptEntryId onward; without one, the whole transcript rebuilds as before: an assistant message + its tool_use entries (by messageId) form one assistant message (calls after text/thinking); consecutive user-side entries merge into one user message (results first, append order); non-conversation entries are skipped; a tool_result named by any elision entry is rebuilt with its content replaced by session.ElidedStub ([superseded: this call ran again later (or the file was rewritten) - the newer result is below]); the transcript keeps the original, internal/tools/ — Tool interface + registry, the 7 tools, ReadTracker, shell runner, ignore matcher, Eval corpus (rev 21, test/evals/) — seven scenarios: fixture repo (committed as base), prompt.txt, check.sh (judged against the base commit), optional flags; SESSION names the transcript, Activity row: `⠋ shell go test ./…… 12s · ↓1.2k tok · 1 queued · esc to interrupt` — the running tool's operand (command first line / path / pattern, clamped), a streamed-token estimate (chars/4; runs only) and the number of queued steering messages (agent.PendingSteering), @path: when the draft's last word starts with @, the panel lists matching workdir files (gitignore-aware via tools.ListFiles, ≤ 5000 files indexed lazily on the first @ and re-indexed after a turn or a ! command; ranked basename prefix → path-segment prefix → substring, shallower first, ≤ 50 rows); tab/enter insert @path (enter never sends while the list is open); text is sent as written — the model reads the file itself; an email address (@ not at a word start) never opens it, Colors: scrollback text is converted to the terminal's color profile before printing (tea.Println bypasses the renderer's own downsampling); detected at startup like Bubble Tea does; NO_COLOR drops every color (bold/faint stay; › still marks user lines), 256/16-color terminals get approximations of the band colors, a non-terminal gets plain text, Context pressure: the status bar's percent field turns yellow from 70% of the window and red from the auto-compaction trigger (compact.Budget.Trigger(): window − reserve), where line 2 also says /compact (dropped before in/out when narrow) (+14 more)

### Community 78 - "moca diagnostic log"
Cohesion: 0.12
Nodes (13): lazy MCP proxy, config.AppendString, config.SetObjectEntry, config.SetString, env:VAR indirection, internal/config, mcp import discovery sources, mcp import server shapes (+5 more)

### Community 79 - "encoding/json.RawMessage"
Cohesion: 0.16
Nodes (8): Server, sessionsDir(), busy(), invalidParams(), notFound(), decode(), TestLookupFlushesHeldUpdates(), ACP: held updates flush before the next request's notifications

### Community 80 - "TestRequestsAreAppendOnly"
Cohesion: 0.25
Nodes (12): snapshotBodies(), TestRequestsAreAppendOnly(), TestRequestsAreAppendOnlyAfterCompaction(), Harness (A1-A3), blocksOf(), FirstDivergence(), roleOf(), Strip() (+4 more)

### Community 81 - "registry_test.go"
Cohesion: 0.11
Nodes (30): IsLocalProvider(), newFakeOllama(), ollamaRegistry(), TestNoOllamaNoProbe(), TestOllamaChatWithoutKey(), TestOllamaDiscovery(), TestOllamaDiscoveryVerifyConcurrent(), TestOllamaKeylessAndOptionalKey() (+22 more)

### Community 82 - "skills/autoskill_test.go"
Cohesion: 0.16
Nodes (34): autoSkillRaw(), TestValidateRequiresTheDescriptionToNameACitedCommand(), TestValidateStoredListsAPreReleaseDescription(), TestValidateChecksOnlyAddedLinesForCitedCommands(), TestValidateRefusesACitedCommandTheSessionNeverRan(), TestValidateTrustsUserRulesForCitedCommands(), TestNegationFirstExemptsAWarningFromClauseC(), TestTrailingNegationIsFineForAProvenCommand() (+26 more)

### Community 83 - "ToolCall"
Cohesion: 0.18
Nodes (16): ToolEnd, ToolStart, toolCall(), ToolCall, TestExecutedSegments(), cmdOf(), ExecutedFromCall(), ExecutedSegments() (+8 more)

### Community 84 - "endUpdate"
Cohesion: 0.23
Nodes (11): toolInput, TestEditUpdateCapsBothDiffSides(), absPath(), capHead(), capTail(), endUpdate(), parseInput(), startUpdate() (+3 more)

### Community 85 - "What You Must Do When Invoked"
Cohesion: 0.08
Nodes (24): For /graphify add and --watch, For /graphify query, For the commit hook and native CLAUDE.md integration, For --update and --cluster-only, /graphify, Honesty Rules, Interpreter guard for subcommands, Part A - Structural extraction for code files (+16 more)

### Community 86 - "rules_test.go"
Cohesion: 0.15
Nodes (17): ParseRule(), lit(), matchWithin(), TestLearnIsOneLiteralWord(), TestLearnPathRecordsExactHead(), TestParseRuleCollapsesStars(), TestParseRuleErrors(), TestRuleCase() (+9 more)

### Community 87 - "Message"
Cohesion: 0.12
Nodes (13): agent events list, cross-provider transform, elision agent/elision.go, internal/agent, internal/llm, internal/session, session entry types, session.Find and FindForWorkdir (+5 more)

### Community 88 - "§10.5 MCP proxy (phase 5)"
Cohesion: 0.11
Nodes (16): internal/mcp — no SDK, hand-rolled JSON-RPC 2.0; pieces: the message core + transport interface (jsonrpc.go), the stdio transport, the streamable-HTTP transport + SSE reader, the client (handshake, paged tools/list, tools/call), the persisted index, the lifecycle Manager, the mcp proxy tool and the importer, Streamable HTTP transport — one POST per message (JSON body, Accept: application/json, text/event-stream, configured headers with env:VAR resolution); Mcp-Session-Id captured from the initialize response and echoed on every later request; MCP-Protocol-Version sent once negotiated; a JSON response is matched by id; a text/event-stream response is read until a data: event carries the response with our id (server→client requests/notifications and other ids are ignored); 202 is accepted for notifications; a 404 once a session exists → session expired; non-2xx → error with a ≤4 KiB body (the status is checked on notifications too); Close sends DELETE (best-effort, bounded to 3 s — it runs under the server lock); without an injected client the manager builds &http.Client{Timeout: 15m} (defaultHTTPTimeout) — the TUI's run context is unbounded, so a server that accepts a POST, answers text/event-stream and wedges must not hang a call forever, moca mcp import [--yes] — discovery (mcp.DiscoverSources): ~/.claude.json (top-level mcpServers + projects[<cwd>].mcpServers), <cwd>/.mcp.json, ~/.config/opencode/opencode.{json,jsonc} (mcp), and Pi's files per pi-mcp-adapter's documented layout with the Claude-compatible shape: ~/.config/mcp/mcp.json, ~/.agents/mcp.json, ~/.agents/mcp/mcp.json, ~/.pi/agent/mcp.json, <cwd>/.pi/mcp.json; shapes: claude {command,args,env} | {type:http|streamable-http, url, headers}; type:"sse" skipped (legacy SSE unsupported, noted); enabled:false / disabled:true skipped; opencode {type:"local", command:[cmd, …], environment} | {type:"remote", url, headers}, First source wins a duplicate name; names already configured are skipped; notes explain every skip; the preview prints each server's JSONC block + the export lines + notes; without --yes a [y/N] confirm on stdin; accepted servers are written one by one with config.SetObjectEntry(…, ["mcp","servers"], name, <compact JSON>), and the summary separates written from already-present-kept, moca mcp index — builds a Manager over the configured servers and calls IndexAll (starts each once, indexes it, then Close stops them), printing `name: N tools` per success and the error to stderr otherwise; exit 1 if any server failed; both mcp subcommands load config but need no model (they route before the model check), Index (~/.local/share/moca/mcp-index.json, 0600, unique temp file + atomic rename; Save merges with what is on disk so concurrent moca processes keep each other's servers) — {server: {configHash, tools:[{name, description, annotations}], updatedAt}}; tool schemas are not persisted (describe fetches them live); ConfigHash = sha256 over command/args/env/url/headers only, so editing description/approve never re-indexes; missing/corrupt file → empty (search then starts servers to index them); refreshed on every server start; an entry is valid only while the hash matches, Protocol — initialize (protocolVersion 2025-06-18, clientInfo {moca, <version>}) then the notifications/initialized notification; tools/list follows nextCursor until exhausted (a repeated cursor or more than 100 pages is an error); tools/call sends arguments ({} when omitted or null), §10.5 MCP proxy (phase 5) (+8 more)

### Community 89 - "sync.Mutex"
Cohesion: 0.12
Nodes (16): Still open: a live moca login openai round-trip (needs a ChatGPT account - the flow is covered by the fake-AS suite and the real endpoints were probed 2026-10-06); direct api.anthropic.com / api.openai.com API-key endpoints; multi-turn thinking replay + prompt caching verification; glm --effort off behavioral verification; the opencode-go x-opencode-session header is still per-process (fine for routing, per-conversation rebinding remains open); a TUI-driven real-model MCP ladder pass; graphify install --platform moca pending upstream merge of Graphify-Labs/graphify 4174; intermittent Go-tier upstream 400s on glm-5.3-flash (no tokens spent; now auto-retried - vendor-side); the release workflow (manual trigger only) awaits its first run after merge, and the Windows replace path is cross-compile-checked only (GOOS=windows go build); the Zed and ACP TCK manual gates for moca acp (the TCK is an experimental out-of-CI harness - the hermetic test/acpgate and the pinned-schema conformance suites are what ships verified today), Go 1.27.1 pinned once in go.mod toolchain directive (mise reads it), Module github.com/adeotek/moca, binary moca, No frameworks, no binary plugin system (non-goals are §16), Not implemented: released graphify install --platform moca (PR pending upstream), Not implemented: a live moca login openai round-trip (needs a ChatGPT account), §1 Overview, Phase 1: streaming via three hand-rolled protocol codecs, JSONC config, catalog, retry, registry (+8 more)

### Community 90 - "moca/log_test.go"
Cohesion: 0.22
Nodes (17): logRun(), logRunCfg(), logRunCfgIn(), logRunIn(), readLog(), sessionID8(), TestLogDebugViaEnv(), TestLogFileForOneShot() (+9 more)

### Community 91 - "release v0.4.1.beta"
Cohesion: 0.17
Nodes (14): first-run provider notice and corrupt auth.json shown, built-in skills re-extracted on binary change, discovery ignores dot-prefixed directories, /help column alignment, moca execute and /execute plan running, moca prune cleans stale builtin-skills dirs, seeded skill hidden when CLI not on PATH, README Phases table removed (+6 more)

### Community 92 - "Agent"
Cohesion: 0.06
Nodes (22): Options, RunOpts, savedModel, steerMsg, Agent, Outcome, New(), progressNudge() (+14 more)

### Community 93 - "autoskill_prune_test.go"
Cohesion: 0.26
Nodes (20): hygieneSummary(), containsStr(), PrunedSkills, PruneAutoSkills(), rewriteBody(), splitBodyLines(), day(), TestPruneAutoAtomicRewriteKeepsMode() (+12 more)

### Community 94 - "make build (bin/moca)"
Cohesion: 0.11
Nodes (17): binary moca, //go:build shipgate tag, config.Standardize (JSONC), Go 1.27.1 (pinned), gopls, make build (bin/moca), make fmt (go fmt ./...), mise (+9 more)

### Community 95 - "rpc_test.go"
Cohesion: 0.25
Nodes (18): echoHandler, pipePeer(), readMsg(), TestCallCascadesCancel(), TestCallFailsOnEOF(), TestCancelRequest(), TestConcurrentDispatch(), TestConcurrentWritesAreWholeLines() (+10 more)

### Community 96 - ".Run"
Cohesion: 0.18
Nodes (17): TestProxyToolRejectsAMisspelledField(), TestSearchReportsAndRemembersFailedServer(), ConfigHash(), TestIndexPersistAndHash(), TestProxyPlanModeCallFollowsNormalGating(), cutRunes(), errResult(), isTrue() (+9 more)

### Community 97 - "Env.Judge classifier"
Cohesion: 0.11
Nodes (12): ~/.config/moca/config.jsonc path, example.jsonc byte-exact fixture, internal/applog, log debug-level contents, log info-level contents, moca config file path, moca diagnostic log, /model command (+4 more)

### Community 98 - "mcp tool (the proxy tool)"
Cohesion: 0.19
Nodes (8): internal/mcp, mcp lazy proxy package, mcp tool call rendering, mcp tool describe rendering, mcp tool search rendering, mcp tool (the proxy tool), moca mcp index, phase 5

### Community 99 - "keys dispatch"
Cohesion: 0.10
Nodes (15): alt+t thinking pager, ctrl+c double-press quit, ctrl+o pager on latest item, ctrl+t effort cycle, esc interrupt semantics, keys dispatch, .moca/config.jsonc trusted-only project config, moca execute (+7 more)

### Community 100 - "tuiCmd"
Cohesion: 0.06
Nodes (49): Prompt, SkillMessage(), TestSkillMessage(), countLines(), newModel(), newManagedModel(), TestAutoSkillsNotSlashCommands(), TestHelpListsRememberAndAutoSkills() (+41 more)

### Community 101 - "Changelog"
Cohesion: 0.09
Nodes (22): A description that hides the command is refused (#52), A rule may only cite a command the run actually ran (#52), Agent, Agent, Agent, Also in this release, Catalog & tooling, Changelog (+14 more)

### Community 102 - "skillgate_test.go"
Cohesion: 0.11
Nodes (49): proc, KillProcessGroup(), SetProcessGroup(), KillProcessGroup(), SetProcessGroup(), gate, proc, provider (+41 more)

### Community 103 - "graphify.ingest.ingest(url, raw_dir, author, contributor) auto-detects URL type; errors are reported, never silently continued"
Cohesion: 0.11
Nodes (18): Code-only changes (.py/.ts/.go/?) re-run AST extraction + rebuild + cluster immediately, no LLM; graph.json and GRAPH_REPORT.md updated automatically, For /graphify add, For --watch, graphify.ingest.ingest(url, raw_dir, author, contributor) auto-detects URL type; errors are reported, never silently continued, graphify reference: add a URL and watch a folder, /graphify --update pipeline: merges a newly saved file into the existing graph (LLM semantic re-extraction), graphify.watch background watcher: --debounce 3, reads the trusted graphify-out/.graphify_root so a metacharacter-bearing scan root can never be re-interpreted, arXiv ingest: abstract + metadata saved as .md (+10 more)

### Community 104 - "SOURCES.md"
Cohesion: 0.19
Nodes (7): internal/skills/testdata/ecosystem corpus, anthropics/skills frontend-design, anthropics/skills skill-creator, graphify skill (graphify install, claude platform; byte-identical to pi variant), adeotek/dotfiles opencode dotnet-unit-testing skill, npm pi-mcp-adapter mcp-scripting skill, npm pi-subagents skill

### Community 105 - "Claude Code skill-creator skill"
Cohesion: 0.16
Nodes (13): Skill description (trigger) optimization, Skill eval/iterate loop (with-skill vs baseline), skill-creator Apache 2.0 License (Anthropic), Claude Code skill-creator skill, SKILL.md anatomy + bundled resources layout, Constrained query expansion against graph vocab, Progressive Disclosure (three-level skill loading), graphify query/path/explain reference (+5 more)

### Community 106 - "provider.NewRegistry"
Cohesion: 0.14
Nodes (6): default base URLs, provider.NewRegistry, providers.<name>.baseUrls per protocol, providers.<name>.models.<id>, providers.ollama entry, x-opencode-session header

### Community 107 - "shipgate/fixture/main.go"
Cohesion: 0.21
Nodes (7): Sum(), TestTotals(), ParseList(), Average(), Max(), TestMaxAndAverage(), main()

### Community 108 - "autoskill.go"
Cohesion: 0.14
Nodes (30): citedCommand(), clauseBefore(), commandClaim(), CountAutoSkills(), countRules(), TestCommandClaim(), instructed(), IsEmptyBody() (+22 more)

### Community 109 - ".Run"
Cohesion: 0.13
Nodes (8): isDocPath(), noteChange(), TestRegistryRecoversToolPanic(), TestRepeatedFailureHintAndReset(), TestRepeatedFailureIgnoresSpillPath(), failTool, panicTool, spillFailTool

### Community 110 - "TextOf"
Cohesion: 0.12
Nodes (20): Summarizer, TextOf(), Options, Result, Turn, TestReplayIsDeterministic(), fatal(), main() (+12 more)

### Community 112 - "MCPServer"
Cohesion: 0.12
Nodes (27): MCPServer, TestImportRewritesEmbeddedSecretsInValues(), TestRewriteSecretsValueRuleAlone(), DiscoverSources(), envName(), envRefs(), ParseSource(), Plan() (+19 more)

### Community 113 - "input_test.go"
Cohesion: 0.30
Nodes (11): NewInput(), TestCollapseUsesExactOffset(), TestCtrlC(), TestEditedMarkerSentLiterally(), TestHistory(), TestHistoryRecallChipsUnsafeContent(), TestMarkerNeverCollidesWithTypedText(), TestPasteChipKeepsContent() (+3 more)

### Community 114 - "install.sh"
Cohesion: 0.27
Nodes (9): bin/moca update --check, .github/workflows/release.yml (manual dispatch), internal/update, release asset naming contract (moca-<version>-<os>-<arch>), die(), fetch(), note(), install.sh script (+1 more)

### Community 115 - "findTestFile"
Cohesion: 0.11
Nodes (29): CacheBreak, Comparison, Metrics, Ratio, ratios, ScenarioDelta, totals, Compare() (+21 more)

### Community 116 - "fetch_test.go"
Cohesion: 0.23
Nodes (8): Client, New(), TestGet(), Eval prompt: plan an on-disk response cache, Cache type (dir, TTL) in cache.go, Client.Get in fetch.go uses the cache, Plan: on-disk response cache, TTL option on New

### Community 117 - "encoding/json.RawMessage"
Cohesion: 0.09
Nodes (24): checkers, planAllows(), TestPlanModeSuppressesInvestigationProtocol(), TestSamePathFoldsWhereTheFilesystemDoes(), TestInvestigationHint(), TestInvestigationRefusal(), TestShellOwnBreakageDoesNotArmInvestigation(), CanonicalSkillPath() (+16 more)

### Community 118 - "exports.md"
Cohesion: 0.11
Nodes (17): claude_desktop_config.json wiring: set command to the absolute interpreter path from .graphify_python (Claude Desktop cannot run $(...)), FalkorDB export: OpenCypher files one statement at a time; prefer --falkordb-push (falkordb://localhost:6379); MERGE idempotent, graphify-out/.graphify_labels.json is consumed by the wiki export; wiki runs before Step 9 (cleanup) so the file still exists, graphify reference: extra exports and benchmark, GraphML export (graphify export graphml), graphify.serve stdio MCP server exposing query_graph, get_node, get_neighbors, get_community, god_nodes, graph_stats, shortest_path, Neo4j export: Cypher file or --neo4j-push to bolt:// instance; MERGE makes re-runs idempotent, Step 6b - Wiki (only if --wiki flag) (+9 more)

### Community 119 - "inline NetworkX traversal fallback"
Cohesion: 0.20
Nodes (5): graphify-out/graph.json, graphify-out/.graphify_python, graphify query CLI, --cluster-only re-clustering, GRAPH_REPORT.md regeneration

### Community 120 - "ToolSpec"
Cohesion: 0.12
Nodes (10): ToolSpec, TestProxySpecFrozen(), MCPSpec(), Registry, editTool, lsTool, readTool, Tool (+2 more)

### Community 121 - "Sanitize"
Cohesion: 0.06
Nodes (28): TestSkillSubmissionStartsARun(), clampRunes(), matchCommands(), contains(), count(), dropNames(), TestClampRunes(), TestDropdownListsCommandsAndPrompts() (+20 more)

### Community 122 - "release v0.1.1"
Cohesion: 0.15
Nodes (13): credentials in auth.json 0600, dollar-brace bash expansion in prompt templates, Go version pinned in go.mod, TUI /login and /logout, slash login in the TUI: pick a provider then paste an API key masked never echoed never written to the transcript or sign in to a subscription in the browser; while a sign-in is in flight the pasted redirect URL or code from the input box feeds the flow and esc cancels; after a successful ChatGPT sign-in the wizard offers to flip providers openai auth to oauth; slash logout revokes a subscription session best-effort and clears the stored token and key, ten more opencode-go catalog models, / autocomplete dropdown, stored key wins over env reference (+5 more)

### Community 123 - "manual Release workflow with checksums"
Cohesion: 0.22
Nodes (13): install.sh and install.ps1, moca update replaces the installed binary with the latest GitHub release package: checks the newest published release skipping drafts, picks the package for the platform, verifies against checksums, and swaps the binary in place atomically with a symlinked launcher having its target replaced; sessions config and credentials never touched; check mode only reports; a dev or dirty build or one newer reports not newer; a platform without a package like darwin points at make release; runs before config is loaded so a broken config cannot block an update, Ollama failure modes: a server that is down fails at once with how to start it (no retry ladder); a model never pulled says ollama pull name; a key via slash login ollama is optional for servers behind an authenticating proxy; nothing is probed unless you opt in, Ollama provider local and LAN, manual Release workflow with checksums, moca update self-update from GitHub releases, --uninstall flag in install scripts, moca update and --check (+5 more)

### Community 124 - "implementation deltas vs phase-7 plan"
Cohesion: 0.20
Nodes (6): per-host ext_agent_host_id (urn:uuid / jwk-thumbprint), implementation deltas vs phase-7 plan, JWKS endpoint (RSA/RS256, kid present), rotating refresh tokens (30-day TTL), unsupported request fields list, usage-limit error codes (429/503/403/400)

### Community 125 - "Render"
Cohesion: 0.14
Nodes (7): jsonItem, main(), Render(), TestRenderTotal(), main(), RenderJSON(), TestRenderJSON()

### Community 126 - "Open"
Cohesion: 0.10
Nodes (23): Level, nopCloser, wc, Discard(), Open(), ParseLevel(), prune(), keepDefault() (+15 more)

### Community 127 - "startStdio"
Cohesion: 0.24
Nodes (12): shServer(), skipWindows(), TestListToolsRepeatedCursor(), TestStdioGrandchildHoldingStdoutDoesNotHangExit(), TestStdioReplyThenExitKeepsReply(), TestStdioStdoutFatalAlwaysSurfaces(), startStdio(), dial() (+4 more)

### Community 128 - ".prompt"
Cohesion: 0.24
Nodes (7): flatten(), mentionPath(), splitCommand(), textBlocks(), mentionsRemembering(), RememberRequest(), TestRememberRequestText()

### Community 129 - "graphify-out/"
Cohesion: 0.25
Nodes (9): docs/specs/external-tools.md, god nodes & community structure, graphify, graphify-out/, graphify-out/GRAPH_REPORT.md, graphify-out/wiki/index.md, graphify query/path/explain commands, graphify update . (+1 more)

### Community 130 - "moca/prune_test.go"
Cohesion: 0.35
Nodes (10): formatBytes(), execPrune(), pruneEnv(), TestFormatBytes(), TestRunPruneDryRunRemovesNothing(), TestRunPruneIgnoresBrokenConfig(), TestRunPruneNothingToDo(), TestRunPruneRejectsUnknownArgs() (+2 more)

### Community 131 - "anthropic decision: api_key only"
Cohesion: 0.25
Nodes (6): config validation rejects providers.anthropic.auth=oauth, moca, OpenClaw (listed partner), OpenCode (listed partner), partner directory open-source integrations (OpenClaw/OpenCode/Pi/T3), Sign in with ChatGPT open-source token sharing

### Community 132 - "fixture/order.go"
Cohesion: 0.28
Nodes (6): Order, User, NewOrder(), TestOrderEmail(), TestUserEmail(), NewUser()

### Community 133 - "Login"
Cohesion: 0.12
Nodes (19): fakeSIWC(), fakeSIWCOpts(), TestDefaultHTTPClientBoundsHeaderWait(), defaultHTTPClient(), newHTTPError(), retryAfter(), OAuthConfig, Login() (+11 more)

### Community 134 - "Model.ClampEffort"
Cohesion: 0.22
Nodes (9): anthropicAdaptive level map, basicThinking level map, deepseekProThinking level map, deepseekThinking level map, glmThinking level map, Model.ClampEffort, openaiReasoning level map, qwenThinking level map (+1 more)

### Community 135 - "vague/testdata/reference/main.go"
Cohesion: 0.29
Nodes (7): TestCollect(), CToF(), FToC(), TestRoundTrip(), main(), fail(), main()

### Community 137 - "Registry"
Cohesion: 0.18
Nodes (8): Registry, For /graphify explain, For /graphify path, graphify reference: query, path, explain, Step 0 — Constrained query expansion (REQUIRED before traversal), Step 1 — Traversal, anyProviderUsable(), CredentialFunc

### Community 138 - "DataDir"
Cohesion: 0.23
Nodes (10): TestExitForOAuthSentinels(), TestProjectResourcesTriggerTheTrustPrompt(), exitFor(), firstLine(), oneShotSlug(), runOneShot(), decideTrust(), hasProjectResources() (+2 more)

### Community 139 - ".applyMode"
Cohesion: 0.28
Nodes (7): graphify-out/.graphify_detect.json holds detect output incl. total_words and per-type file lists, graphify reference: transcribe video and audio, Step 2.5 - Transcribe video / audio files (only if video files detected), graphify.transcribe.transcribe_all(video_files, initial_prompt=prompt), graphify-out/.graphify_transcripts.json: written from Python, not a shell redirect, because Whisper progress on stdout would corrupt the JSON (#1392), Step 2.5: transcribe detected video files (skip entirely when detect returns zero video); transcripts join the docs list before semantic subagents (Step 3B), Whisper transcription: GRAPHIFY_WHISPER_MODEL (default base) and GRAPHIFY_WHISPER_PROMPT must be exported for the child process

### Community 140 - "moca -p one-shot run"
Cohesion: 0.51
Nodes (4): TestItems(), TestLastOfKind(), Item, Items

### Community 141 - "errorf"
Cohesion: 0.11
Nodes (12): globToRegexp(), ignored(), parseIgnore(), TestIgnore(), ListFiles(), TestListFiles(), readLine(), decode() (+4 more)

### Community 142 - "compaction budgets"
Cohesion: 0.19
Nodes (11): Checkpoints: maybeCompact before every new user prompt (start of Run) and after every completed tool batch (after steering); when the estimate is over the trigger it compacts, and if the estimate is still over it stops the run with an error naming the largest live entries (`context still over budget after compaction (N > trigger); largest entries: tool_result for read(...) approx Nk tokens; fixed overhead (system prompt + tools) approx Nk tokens` - the overhead clause appears when it dominates the estimate) - never compacts twice in a row; an indivisible over-trigger tool batch (calls+results must stay together) is the intended case: the guard names it instead of dropping fresh tool data; a summarizer that cannot run at all (cheap-model credential unset, unknown cheap model) skips the automatic checkpoint with `warning: auto-compaction skipped: ...` - the run continues on the active model; a real provider overflow still surfaces the original error through the retry path, /compact command, compaction summary format, context.keepRecentTokens, context pressure colors, context.reserveTokens, internal/compact, Model.BudgetTokens (+3 more)

### Community 144 - "Truncate"
Cohesion: 0.32
Nodes (5): Order, User, NewOrder(), NewUser(), normalizeEmail()

### Community 145 - "rpc.go"
Cohesion: 0.16
Nodes (8): Conn, Deferred, Handler, message, Options, guardPanic(), NewConn(), Serve()

### Community 146 - "structural interfaces pattern"
Cohesion: 0.29
Nodes (6): internal/compact, internal/permissions, internal/session, internal/skills, internal/tools, internal/tools !windows tests

### Community 147 - "dynamic client registration (dynamic_agent_client placeholder)"
Cohesion: 0.33
Nodes (4): agent_name_hint=moca (consistent across installs), issued client id (oaiapp_...) persisted and reused, reauthorization reuses saved client id (omit agent_name_hint), refresh errors meaning clear + re-login

### Community 148 - "Anthropic: third-party subscription OAuth not permitted"
Cohesion: 0.29
Nodes (5): 2026-02-18/19 clause formalized in Claude Code legal docs, 2026-01-09 server-side credential checks / account bans, Claude legal-and-compliance usage policy (primary source), corroborating enforcement timeline 2026-01 to 2026-09, OpenCode removes Claude subscription support

### Community 149 - "BuildSystemPrompt"
Cohesion: 0.16
Nodes (14): PromptInput, SelfManagedInput, ServerLine, TestPromptBlockAndArtifactMarker(), BuildSystemPrompt(), oneLine(), renderPrompt(), selfManagedProbe() (+6 more)

### Community 150 - "moca prune command"
Cohesion: 0.29
Nodes (3): /builtin-skills/ (data dir), moca prune command, skills.PruneBuiltins

### Community 152 - "Serialize"
Cohesion: 0.25
Nodes (8): callLine(), safeTail(), Serialize(), TestSerialize(), TestSerializeRuneSafeAndEmptyText(), TestTrackFiles(), TestTrackFilesNormalizesPaths(), TrackFiles()

### Community 153 - "prompt-cache reset cost"
Cohesion: 0.47
Nodes (3): internal/agent/system-prompt.md (embedded), internal/tools/testdata/schemas.golden.json, {{token}} slots

### Community 155 - "README.md - moca"
Cohesion: 0.18
Nodes (15): README.md - moca, config.jsonc, competes on cost per task, Diff-shaped tool results, Do mode (--do//do), First-run setup mode, config.jsonc (JSONC, $XDG_CONFIG_HOME honored), MCP lazy ~200-token proxy tool (+7 more)

### Community 156 - "testing.T"
Cohesion: 0.02
Nodes (160): parseArgs(), TestHelpListsSurface(), TestParseArgs(), TestParseArgsExecute(), TestParseArgsExecuteUsageErrors(), TestSubcommandBeforeConfig(), ptr(), TestExecuteIgnoresConfiguredPlan() (+152 more)

### Community 157 - "initialize"
Cohesion: 0.15
Nodes (13): initialize(), TestHTTPCloseBounded(), startHTTP(), fakeHTTPServer(), TestHTTPNotifyStatusChecked(), TestHTTPTransport(), TestStdioEarlyExit(), Annotations (+5 more)

### Community 158 - "release v0.3.0.beta"
Cohesion: 0.11
Nodes (18): edit trailing-blank-line crash fixed, test/evals eval corpus, mise install also installs gopls, overflow files per-session isolation, plan mode writes docs/plans only, per-session OpenAI prompt_cache_key, read returns 400 lines default, runs end verified or explained (+10 more)

### Community 159 - "fixture/slug.go"
Cohesion: 0.40
Nodes (4): Limit(), Make(), TestLimit(), TestMake()

### Community 160 - "CI Workflow (gofmt · vet · build · test)"
Cohesion: 0.40
Nodes (4): CI Workflow (gofmt · vet · build · test), CI gate set (gofmt, go vet, go build, go test -race), go.mod (Go version source for setup-go), SPECS.md §2 — CI gate definition

### Community 161 - "Client"
Cohesion: 0.21
Nodes (6): baseName(), globWord(), matchWords(), Decision, matcher, Rule

### Community 162 - "TestKeepNameWindows"
Cohesion: 0.11
Nodes (17): Spec: external tools - rtk, graphify, and the skills/prompts ecosystem, internal/skills/testdata/ecosystem corpus proves cross-tool SKILL.md compatibility (provenance and licenses in SOURCES.md), graphify codebase knowledge graph CLI, in default allowlist, seeded into ~/.config/moca/skills/graphify/ (SKILL.md + references), Graphify-Labs/graphify PR #4174 (graphify install --platform moca, contributed upstream), internal/permissions/rtk.go captures subcommand classes from the real CLI (rtk 0.51.0), rtk token-compressed CLI proxy, in default allowlist, ships seeded skill teaching rtk <cmd> variants, rtk subcommands writing config/agent hooks (init, config, trust, untrust, learn, telemetry) ask once, rtk subcommands proxying a native binary (ls, tree, find, grep, rg, ast-grep, wc, diff) checked as that binary too (rtk find -exec runs; rg --pre, ast-grep -U, tree -o) (+9 more)

### Community 163 - "live probe outcomes (discovery, invalid_client, Cloudflare 403)"
Cohesion: 0.40
Nodes (5): verified OIDC discovery endpoints table, live probe outcomes (discovery, invalid_client, Cloudflare 403), auth.openai.com authorize/token/revoke URLs, phase-7 live gate (real ChatGPT account e2e check), logout via revoke endpoint (token_type_hint=refresh_token)

### Community 164 - "moca — MO Coding Agent"
Cohesion: 0.12
Nodes (14): Configure, Debugging / logs, Development, Docs, Editors (ACP), Inspecting the config, Install, moca — MO Coding Agent (+6 more)

### Community 165 - "loads only when corpus has doc/paper/image chunks"
Cohesion: 0.40
Nodes (3): Step 3 Part B pipeline stage, code extension set, Step 2.5 references/transcribe.md for video chunks

### Community 166 - "applyEdit"
Cohesion: 0.23
Nodes (14): applyEdit(), formatDiff(), indentOf(), joinInts(), lineOf(), matchLines(), replaceExact(), replaceFuzzy() (+6 more)

### Community 167 - "Lookup"
Cohesion: 0.50
Nodes (3): Lookup(), TestLookupBounds(), Valid()

### Community 168 - "self-knowledge builtin skill: answer questions about moca itself, grounded in embedded reference docs, never from memory"
Cohesion: 0.25
Nodes (6): Builtin skills (create-command, self-configure, self-knowledge) re-extracted for each binary version, the opposite of seeding, User config: the live config.jsonc outside the workdir; every save raises the approval prompt (expected), stop when denied, self-knowledge builtin skill: answer questions about moca itself, grounded in embedded reference docs, never from memory, For 'what is currently configured', also read the live config.jsonc (dir named on the environment's moca-dirs line; reads outside the workdir allowed for it), references/readme.md: user-facing install, configure, permission modes, usage, references/specs.md: normative detail (CLI §3, config §4, tools §10, modes §11, skills §12); large, read with offset/limit after searching

### Community 169 - "internal/applog"
Cohesion: 0.50
Nodes (3): internal/applog, standard log/slog, SPECS §13.5 (log privacy)

### Community 170 - "sync.Mutex"
Cohesion: 0.08
Nodes (12): logSink, cappedWriter, cutRecord(), responseID(), TestResponseID(), httpTransport, logSink, ring (+4 more)

### Community 171 - "SPECS.md — moca spec document"
Cohesion: 0.50
Nodes (4): Go 1.27.1 pinned once in go.mod toolchain directive, moca — minimal coding agent (github.com/adeotek/moca), mvdan.cc/sh/v3, SPECS.md — moca spec document

### Community 173 - "graphify reference: incremental update and cluster-only"
Cohesion: 0.50
Nodes (3): For --cluster-only, For --update (incremental re-extraction), graphify reference: incremental update and cluster-only

### Community 174 - "graphify.analyze.graph_diff after Step 4"
Cohesion: 0.50
Nodes (4): graphify.build.build_from_json, graphify.analyze.graph_diff after Step 4, graphify.paths.load_node_link_graph, .graphify_old.json backup before merge

### Community 175 - "Claude Code frontend-design skill"
Cohesion: 0.50
Nodes (3): AI-generated design default tells, frontend-design Apache 2.0 License, Claude Code frontend-design skill

### Community 176 - "pi mcp-scripting skill"
Cohesion: 0.67
Nodes (4): jev.evaluate reference, jev.evaluate semantic scripting gate, mcpScript tools API, pi mcp-scripting skill

### Community 177 - "evals/run.sh"
Cohesion: 0.70
Nodes (3): fresh_repo(), one(), run.sh script

### Community 178 - "moca -p one-shot run"
Cohesion: 0.18
Nodes (7): --continue flag, --effort flag, exit codes, moca -p one-shot run, --model flag, -p run semantics, --resume flag

### Community 179 - "skillGuard"
Cohesion: 0.22
Nodes (9): isAutoSkill(), refusingAutoGuard(), skillGuard(), containsWord(), Named(), TestNamed(), wordByte(), isSkillFile() (+1 more)

### Community 180 - "make vet (go vet + gofmt -l)"
Cohesion: 0.67
Nodes (3): go vet ./..., gofmt -l . must be empty, make vet (go vet + gofmt -l)

### Community 183 - "v0.4.0-beta (2026-10-12)"
Cohesion: 0.18
Nodes (10): Diagnostics, Docs, Installer, Permissions, Sessions, Skills — self-management, Templates & the small stuff, TUI (+2 more)

### Community 184 - "seedSkills"
Cohesion: 0.38
Nodes (10): SeedSkills(), seedSkills(), readFile(), seedFSFixture(), TestEmbeddedSeedSkills(), TestSeedSkillsCreatesMissing(), TestSeedSkillsFillsOnlyTheMissingOne(), TestSeedSkillsLeavesExistingAlone() (+2 more)

### Community 185 - "TestKeptTailThinkingFollowsTheRoute"
Cohesion: 0.07
Nodes (36): TestKeptTailThinkingFollowsTheRoute(), checkPrefix(), normalizeBody(), requestFor(), thinkingHistory(), Model, TestBuiltinCatalogSane(), TestHistoryBoundIsAdaptiveOnly() (+28 more)

### Community 188 - "ResolveEnv"
Cohesion: 0.25
Nodes (9): webSearchKey(), TestResolveEnv(), ResolveEnv(), TestKeepNameWindows(), FilterEnv(), keepName(), serverEnv(), TestFilterEnv() (+1 more)

### Community 189 - "picker.go"
Cohesion: 0.24
Nodes (7): modelPickItems(), fullWidthRules(), TestEveryPanelOpensWithTheSeparatorRule(), TestInputRulesUseAccentOrange(), TestPickerSeparatorRule(), pickItem, pickState

### Community 190 - "TestRecordNudgeNotFiredAfterARecord"
Cohesion: 0.31
Nodes (10): writeProjectConfig(), anyRequestHas(), nudgeKinds(), startManagedDefault(), TestRecordNudgeDisabledFallsBackToHint(), TestRecordNudgeFiresAtEndOfRun(), TestRecordNudgeNotFiredAfterARecord(), TestRecordNudgePlanModeNeverFires() (+2 more)

### Community 213 - "refusal_filter.sh"
Cohesion: 0.47
Nodes (4): carries_injection(), refusal_filter.sh script, check(), refusal_filter_test.sh script

### Community 544 - "ACP JSON schema (protocol v1) vendored for conformance tests in internal/acp/testdata (data only)"
Cohesion: 0.47
Nodes (6): ACP protocol v1 JSON schema (schema-v1.x.x tags; v2 alphas excluded), Refresh procedure: re-run Task 2 Step 1 of docs/superpowers/plans/2026-10-13-acp-batch-1-foundations.md and fix whatever go test ./internal/acp reports, Vendored files: schema/v1/schema.json, schema/v1/schema.unstable.json, schema/v1/meta.json from tag schema-v1.25.0, ACP JSON schema (protocol v1) vendored for conformance tests in internal/acp/testdata (data only), Feature gates: messageId on ContentChunk, usage_update (used, size, cost{amount,currency}), plan def names unrenamed, ToolKind/ToolCallStatus/StopReason/PermissionOptionKind must contain every value moca emits, agent-client-protocol upstream repo; pick the newest schema-v1.x.y tag (repo also tags crate releases and v2 alphas)

### Community 548 - "keyboard_test.go"
Cohesion: 0.50
Nodes (3): decodeKey(), TestShiftedSymbolReachesTheDraft(), TestViewRequestsAlternateKeys()

### Community 565 - "sqliteSidecarGuard"
Cohesion: 0.32
Nodes (8): firstOperand(), guardWordOf(), hasWord(), resolveGuardPath(), sqliteSidecarGuard(), unparseableSidecar(), walHeaderOK(), guardWord

### Community 566 - "git flow (main PR-only, phase/N-slug or chore/slug, conventional commits)"
Cohesion: 0.22
Nodes (7): DESIGN.md v1 contract (retired), docs/reviews/ review docs (deleted), docs/specs/SPECS.md, .git/hooks/pre-push (blocks pushes to main), SPECS §16 (living spec rule), conventional commits, one version bump per PR

### Community 568 - ".setMode"
Cohesion: 0.38
Nodes (4): SessionMode, modeByID(), modeIDs(), CanonicalMode()

### Community 569 - "inject/fixture/calc.go"
Cohesion: 0.48
Nodes (5): main(), Mean(), Sum(), TestMean(), TestSum()

### Community 570 - "LoadTrust"
Cohesion: 0.33
Nodes (7): acpTrust(), TestACPTrust(), LoadTrust(), TestTrustStoreSetMergesAcrossProcesses(), TestTrustStoreSetRefusesToClobberMalformedFile(), TestTrustStoreSetRemainsUsableAfterAnError(), TestTrustStore()

### Community 572 - "post-phases (v0.3.0-beta)"
Cohesion: 0.29
Nodes (7): post-phases (v0.3.0-beta), rev 19, rev 20, /sessions command, the web tool, web.search.apiKey, web.search.provider

### Community 574 - "skills-inject/fixture/calc.go"
Cohesion: 0.50
Nodes (3): main(), Sum(), TestSum()

### Community 579 - "skills-inject-output/fixture/calc.go"
Cohesion: 0.50
Nodes (3): main(), Sum(), TestSum()

### Community 580 - "skills-inject-output/testdata/reference/calc.go"
Cohesion: 0.50
Nodes (3): main(), Sum(), TestSum()

### Community 581 - "skills-inject-paste/fixture/calc.go"
Cohesion: 0.50
Nodes (3): main(), Sum(), TestSum()

### Community 582 - "inject/testdata/reference/calc.go"
Cohesion: 1.00
Nodes (3): main(), Mean(), Sum()

### Community 583 - "v0.6.0 (2026-10-16)"
Cohesion: 0.33
Nodes (6): TestGLMEffortMapsMatchTheThinkingOnlyServingVariants(), Agent, Permissions, Providers, TUI, v0.6.0 (2026-10-16)

### Community 584 - "Budget"
Cohesion: 0.47
Nodes (3): Budget, NewBudget(), TestBudget()

### Community 585 - "ThemeConfig"
Cohesion: 0.40
Nodes (6): ThemeColors, TUIConfig, ThemeConfig, validateColor(), validateTheme(), validateThemeColors()

### Community 586 - "Plan: "Allow for this session" approval answer"
Cohesion: 0.29
Nodes (6): Builder's calls (can be vetoed), Context, Decisions (user), Plan: "Allow for this session" approval answer, Verification (at implementation time), TestSelfKnowledgeReferencesAreByteIdentical()

### Community 587 - "skills_inject_c_test.sh"
Cohesion: 0.83
Nodes (3): expect(), skills_inject_c_test.sh script, transcript()

### Community 591 - "protocol_test.go"
Cohesion: 0.33
Nodes (5): TestFlatten(), strp(), TestMethodsInPinnedMeta(), TestProtocolShapesConform(), flatten()

### Community 592 - "overflowDir"
Cohesion: 0.47
Nodes (6): TestPruneOverflow(), TestSpillDirsAreRandom(), newestModTime(), newSpillDir(), overflowDir(), pruneOverflow()

### Community 593 - "ResolveRecordNudge"
Cohesion: 0.67
Nodes (4): ResolveRecordNudge(), TestResolveRecordNudgeTable(), The end-of-run recording nudge, v0.7.1 (2026-10-19)

### Community 594 - "startProjectMCP"
Cohesion: 0.53
Nodes (5): keys(), startProjectMCP(), TestProjectMCPServersJoinTheUserLayer(), TestProjectMCPServersNeedTrust(), warnings()

### Community 612 - "v0.4.1-beta (2026-10-14)"
Cohesion: 0.33
Nodes (6): Build, CLI, Docs, Skills, TUI, v0.4.1-beta (2026-10-14)

### Community 613 - "tui/log_test.go"
Cohesion: 0.47
Nodes (3): captureLog(), TestSetupFirstSessionLogged(), logSink

### Community 614 - "v0.2.0-beta (2026-10-09)"
Cohesion: 0.40
Nodes (5): Ollama (local or LAN), Self-update & release packages, TUI features, TUI fixes, v0.2.0-beta (2026-10-09)

### Community 622 - "v0.7.2 (2026-10-21)"
Cohesion: 0.22
Nodes (8): A review round, not a feature, Auto by default, and the strict mode is called `ask` (2026-10-21), Robustness: the trust store, run state, and the streams (#42), Shell confinement and rule specificity (#39, #40), Skills: provenance, replay, hygiene (#45, #46), The measurement, and a checker that failed open, Truth of the recording nudge (#41), v0.7.2 (2026-10-21)

### Community 623 - "expand"
Cohesion: 0.50
Nodes (5): arg(), braceExpr(), closingBrace(), expand(), slice()

### Community 624 - "v0.3.0-beta (2026-10-09)"
Cohesion: 0.40
Nodes (5): Harness (measured), Plan mode, TUI, v0.3.0-beta (2026-10-09), Web

### Community 626 - "ReadTracker"
Cohesion: 0.60
Nodes (3): stampOf(), ReadTracker, stamp

### Community 646 - "CallResult"
Cohesion: 0.40
Nodes (4): CallResult, Content, response, rpcError

### Community 647 - "anthropic provider"
Cohesion: 0.50
Nodes (4): anthropic provider, claude-haiku-4-5, claude-opus-5-5, claude-sonnet-5-5

### Community 648 - "resolveLong"
Cohesion: 0.67
Nodes (3): resolveLong(), argKind, getoptSpec

### Community 649 - "openai provider"
Cohesion: 0.67
Nodes (3): gpt-6-astra, gpt-6-luna, openai provider

## Ambiguous Edges - Review These
- `Skills and prompts from other tools: prompts dir <name>.md becomes /name in the TUI; $ARGUMENTS / $@ / $1..$9 expansion; Claude Code and pi files drop in unchanged` → `Command file format: optional frontmatter (description shown in /help, argument-hint) then a prompt body with $$ARGUMENTS/$$@, $$1..$$9, $$$$ placeholders`  [AMBIGUOUS]
  docs/specs/external-tools.md · relation: semantically_similar_to
- `docs/reviews/ review docs (deleted)` → `docs/specs/SPECS.md`  [AMBIGUOUS]
  AGENTS.md · relation: cites_deleted_docs

## Knowledge Gaps
- **877 isolated node(s):** `github.com/adeotek/moca`, `example.com/slug`, `example.com/inventory`, `example.com/inject`, `example.com/codes` (+872 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 1596 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **437 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `README.md - moca` connect `README.md - moca` to `TestKeepNameWindows`, `system-prompt.md`, `release v0.1.0.alpha`, `Release - build packages, publish GitHub Release`, `SPECS.md — moca normative spec (v0.5.0-beta)`, `release v0.1.1`, `manual Release workflow with checksums`, `release v0.3.0.beta`?**
  _High betweenness centrality (0.297) - this node is a cross-community bridge._
- **What connects `github.com/adeotek/moca`, `example.com/slug`, `example.com/inventory` to the rest of the system?**
  _877 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `go_pkg_strings` be split into smaller, more focused modules?**
  _Cohesion score 0.10087661756813167 - nodes in this community are weakly interconnected._
- **Why does `graphify codebase knowledge graph CLI, in default allowlist, seeded into ~/.config/moca/skills/graphify/ (SKILL.md + references)` connect `TestKeepNameWindows` to `update/update.go`, `.applyMode`, `exports.md`, `graphify.ingest.ingest(url, raw_dir, author, contributor) auto-detects URL type; errors are reported, never silently continued`?**
  _High betweenness centrality (0.293) - this node is a cross-community bridge._
- **Should `newScript` be split into smaller, more focused modules?**
  _Cohesion score 0.0877431026684758 - nodes in this community are weakly interconnected._

### Low-confidence Hints
_AMBIGUOUS edges — the extractor was unsure. Verify before acting on these._

- **What is the exact relationship between `Skills and prompts from other tools: prompts dir <name>.md becomes /name in the TUI; $ARGUMENTS / $@ / $1..$9 expansion; Claude Code and pi files drop in unchanged` and `Command file format: optional frontmatter (description shown in /help, argument-hint) then a prompt body with $$ARGUMENTS/$$@, $$1..$$9, $$$$ placeholders`?**
  _Edge tagged AMBIGUOUS (relation: semantically_similar_to) - confidence is low._
- **What is the exact relationship between `docs/reviews/ review docs (deleted)` and `docs/specs/SPECS.md`?**
  _Edge tagged AMBIGUOUS (relation: cites_deleted_docs) - confidence is low._