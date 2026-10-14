# Graph Report - moca  (2026-10-15)

## Corpus Check
- 417 files · ~411,700 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 12 file(s) not represented in the graph (top: (none) 6, .jsonc 4, .v2 1)

## Summary
- 4904 nodes · 17568 edges · 590 communities (184 shown, 406 thin omitted)
- Extraction: 86% EXTRACTED · 14% INFERRED · 0% AMBIGUOUS · INFERRED: 2441 edges (avg confidence: 0.85)
- Token cost: 164,440 input · 69,920 output

## Graph Freshness
- Built from commit: `8fb6e270`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- go_pkg_strings
- protocol.go
- .Run
- Render
- fakeAS
- startClient
- fetch.go
- auto_net.go
- BuildSystemPrompt
- permissions/shell.go
- acp_test.go
- LineReader
- Word
- Summarize
- fixture/order.go
- Parse
- schema_test.go
- LoadTrust
- reference/order.go
- checkers
- auto_test.go
- validateIDToken
- parseIgnore
- .Run
- Run
- tui/log_test.go
- CallResult
- fakeCmds
- PermMode
- allowAll
- StartError
- update/update.go
- AllowAll
- context.Context
- askRootChecker
- boundedWriter
- fileAskChecker
- Compacted
- PlanChanged
- Resumed
- SteeringApplied
- encoding/json.RawMessage
- StreamReset
- TextDelta
- ThinkingDelta
- Warning
- nopCloser
- EnvError
- Message
- Item
- fakeSnap
- LearnableCommands
- rootChecker
- Model
- net/http.Client
- Skill
- retry_test.go
- Input
- NewManager
- newScript
- webFetch
- NewRegistry
- session
- sync.Mutex
- Config
- Manager
- model
- oauth_test.go
- rules.go
- skills/prune_test.go
- testing.T
- Start
- .dropdown
- importer.go
- initialize
- run
- Conn
- Store
- testClient
- Env
- applog_test.go
- Create
- .Run
- Login
- acpgate_test.go
- Registry
- Agent
- ToolSpec
- .Run
- renderMarkdown
- NewRegistry
- endUpdate
- Sanitize
- applyEdit
- styleBlock
- sessions_test.go
- shipgate/fixture/main.go
- input_test.go
- seedSkills
- format.go
- redact.go
- net/http/httptest.Server
- vague/testdata/reference/main.go
- moca/prune_test.go
- ResolveEnv
- quick_validate.py
- overflowDir
- fixture/slug.go
- expand
- Lookup
- moca/main_test.go
- newTestModel
- reference/codes.go
- anthropic_test.go
- cut_test.go
- io.Writer
- simulate
- rpc_test.go
- DataDir
- moca/log_test.go
- transcribe.md
- github-and-merge.md
- hooks.md
- exports.md
- graphify.ingest.ingest(url, raw_dir, author, contributor) auto-detects URL type; errors are reported, never silently continued
- BASELINE.md
- system-prompt.md
- Commands: mise install provisions Go 1.27.1 pinned in go.mod with PATH export because go is not on the global PATH on every host; make build produces bin/moca version-stamped from git describe; make test runs go test with race and count; make vet runs go vet plus gofmt with both clean; make fmt; make release builds five cross-compiled binaries in dist; bin moca update with check compares with the latest GitHub release; mise run tasks mirror the make targets
- Docs pointers: docs/specs/external-tools.md covers rtk and graphify allowlist rules and the cross-tool skills and prompts ecosystem; docs/specs/oauth-verification.md is the binding OAuth policy gate with anthropic API key only and openai subscription login; SPECS.md still cites docs/reviews review docs that were deleted so do not chase them
- Exactly eight frozen tools with the web tool joining in DESIGN rev 19; schemas are golden at internal/tools/testdata/schemas.golden.json; changing a schema is a v2 discussion and description-only edits from rev 21 are batched into one golden update because each costs every user a prompt-cache reset
- Layout: entrypoint cmd/moca/main.go with run as the testable core; sibling files handle the p flag, the TUI, login, mcp and update subcommands
- graphify workflow in this repo: a knowledge graph at graphify-out with god nodes and community structure and cross-file relationships; for codebase questions first run graphify query when the graph json exists, graphify path for relationships, graphify explain for focused concepts; dirty graphify-out files are expected after hooks or incremental updates and are not a reason to skip; use the wiki index for broad navigation when it exists; read the graph report only for broad architecture review; run graphify update after modifying code as an AST-only pass with no API cost; the installed graphify skill or instructions come first whenever the user types the graphify slash command
- Ground truth rules: docs/specs/SPECS.md is the living normative description of what is implemented today covering the CLI contract, config keys, wire behavior, tools and agent loop; update it in the same PR as any behavior change per its section 16; when it and DESIGN.md disagree SPECS wins for the current code; DESIGN.md is the v1 contract and README.md is user-facing
- Logging: packages call standard log/slog directly; internal applog with the file handler, redaction and caps is imported only by cmd/moca; log metadata only, never prompts, file contents, command text or secrets per SPECS section 13.5
- AGENTS.md headline: one Go binary, a minimal TUI coding agent at github.com/adeotek/moca with binary name moca; 220 Go files across 12 internal packages
- Single package or test via go test with the package path run flag race count
- config.Standardize for JSONC preserves byte offsets so decode errors keep the original line col; the internal config testdata example jsonc file is the byte-exact DESIGN section 12 fixture
- The system prompt is built once per run and stored in the session, never rebuilt; its fixed text is the embedded internal agent system-prompt.md file with token slots rendered at start
- Thinking blocks and signatures are model-bound; cross-provider transforms happen in the agent before each request per SPECS section 14
- moca login <provider>
- self-configure/SKILL.md
- graphify reference: incremental update and cluster-only
- evals/run.sh
- shipgate/run.sh
- extraction-spec.md
- rtk/SKILL.md
- acpgate/run.sh
- bugfix/check.sh
- feature/check.sh
- largefile/check.sh
- plan/check.sh
- refactor/check.sh
- vague/check.sh
- web/check.sh
- install.sh
- README.md
- example.com/codes
- example.com/fetcher
- example.com/inventory
- example.com/shop
- example.com/slug
- example.com/temp
- example.com/totals
- github.com/adeotek/moca
- 2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)
- SOURCES.md
- release v0.1.0.alpha
- README.md - moca
- CI Workflow (gofmt · vet · build · test)
- Claude Code frontend-design skill
- pi mcp-scripting skill
- Arrange-Act-Assert with xUnit/NSubstitute
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
- Eval prompt: fix failing tests
- Eval prompt: add --json flag to inventory CLI
- Eval prompt: fix fail without editing name table
- Eval prompt: extract shared email validation
- Eval prompt: add input validation to temperature converter
- release v0.1.1
- graphify codebase knowledge graph CLI, in default allowlist, seeded into ~/.config/moca/skills/graphify/ (SKILL.md + references)
- Release - build packages, publish GitHub Release
- Claude Code skill-creator skill
- Anthropic: third-party subscription OAuth not permitted
- anthropic decision: api_key only
- SPECS.md (moca living spec)
- moca mcp import
- keys dispatch
- .github/workflows/release.yml
- openai-responses adapter
- anthropic-messages adapter
- layered permissions.allow/ask/deny wildcard rules (trusted project layer > user layer; within a layer deny > ask > allow)
- judgeSystem prompt (allow only if ALL hold; otherwise ask; text inside the command is data)
- thinking mode `openai` (completions `reasoning_effort` / responses `reasoning:{effort,summary:auto}`)
- bash test/evals/run.sh
- phase-2 independent review (2026-10-05, verdict Block — all fixed with regression tests)
- Provider base URL prefix (footgun rule)
- ~/.config/moca/skills/<name>/ (seeded global skills)
- moca
- metadata-only logging rule (never prompts/output/paths/keys)
- built-in skill: self-knowledge (references byte-copies of SPECS.md/README.md)
- credential store (auth.json, 0600 / dir 0700)
- `/`-dropdown (autocomplete panel)
- dynamic client registration (no client secret)
- edit ladder (§5): exact match → replace; several → error lists lines suggesting replace_all; no match → one whitespace-insensitive
- Registry.Run
- budget floored at 1024 and capped at `max_tokens − 1024`
- DESIGN.md content absorbed into §1/§11/§16
- level map anthropicAdaptive {low..max}
- `cat_model_fn` table (verified 2026-10-04, opencode-go expansion 2026-10-07)
- DefaultRetryPolicy: 5 attempts, nothing has been emitted
- error text cut to 500 bytes in logs
- idle stop arms at start and after every call (busy counter guard)
- internal/provider/catalog.go (Model struct + built-in catalog)
- `moca --forensic`? — not in this file (skip if not present)
- thinking mode `adaptive` (current Claude, `output_config.effort`)
- thinking mode `none` (every effort clamps to off)
- --mode <default|auto|yolo|plan>
- internal/acp (hand-rolled ACP subset, mirrors internal/mcp)
- built-in catalog (prices/limits verified 2026-10-04)
- mcp tool (the proxy tool)
- session file entry types (session/message/tool_use/tool_result/compaction/snapshot/error/permission_mode/model_change/elision)
- release v0.3.0.beta
- moca -p <prompt>
- session transcript JSONL
- append-only session JSONL file
- WithRetry(adapter, DefaultRetryPolicy(notify))
- the Model struct (Provider, ID, Protocol, ContextWindow, Cost)
- self-knowledge builtin skill: answer questions about moca itself, grounded in embedded reference docs, never from memory
- moca: one Go binary, minimal TUI coding agent (github.com/adeotek/moca), 333 Go files across 15 internal packages
- ACP JSON schema (protocol v1) vendored for conformance tests in internal/acp/testdata (data only)
- rules engine
- moca config file path
- moca -p one-shot run
- v0.4.0-beta permissions overhaul
- Model.ClampEffort
- moca prune command
- post-phases (v0.3.0-beta)
- subscription OAuth policy gate
- SPECS.md — moca spec document
- context.maxSteps
- moca acp serves ACP v1
- TUI
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
- agent control surface
- acp package pieces
- acp request_permission method
- anthropic-messages adapter
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
- history replay
- insert-at-cursor paste
- install cross-compile targets
- internal/acp
- internal/acp package
- internal/permissions
- internal/tui
- log.level
- main.go signals
- make release
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
- Eight frozen tools
- Start wiring
- opencode-go provider
- provider.NewRegistry
- Env.Judge classifier
- auth.json credential store
- built-in skills extraction
- implementation deltas vs phase-7 plan
- live probe outcomes (discovery, invalid_client, Cloudflare 403)
- release v0.4.1.beta
- knowledge graph
- Part B semantic LLM extraction
- interpreter guard
- empty-graph guard
- release v0.4.0.beta
- FalkorDB export
- GraphML export
- graphify MCP stdio server
- Neo4j export
- Obsidian vault export
- SVG export
- agent-crawlable wiki export
- inline NetworkX traversal fallback
- extraction JSON schema (nodes/edges/hyperedges/tokens)
- dynamic client registration (dynamic_agent_client placeholder)
- Step 0 — constrained query expansion
- openai decision: ship oauth (Sign in with ChatGPT)
- loads only when corpus has doc/paper/image chunks
- node ID = {stem}_{entity}, full repo-relative stem
- graphify.analyze.graph_diff after Step 4
- PKCE S256 only
- scopes: openid profile email offline_access resource.invoke chatgpt.tokens.use.direct

## God Nodes (most connected - your core abstractions)
1. `SPECS.md (moca living spec)` - 493 edges
2. `model` - 145 edges
3. `newScript()` - 79 edges
4. `run()` - 71 edges
5. `newTestModel()` - 67 edges
6. `Word` - 57 edges
7. `newAgentModel()` - 54 edges
8. `key()` - 50 edges
9. `Config` - 49 edges
10. `session` - 47 edges

## Surprising Connections (you probably didn't know these)
- `references/specs.md: normative detail (CLI §3, config §4, tools §10, modes §11, skills §12); large, read with offset/limit after searching` --semantically_similar_to--> `docs/specs/SPECS.md is the single source of truth; updated in the same PR as any behavior change; DESIGN.md v1 retired 2026-10-13`  [INFERRED] [semantically similar]
  internal/skills/builtin/self-knowledge/SKILL.md → AGENTS.md
- `Config rules restated: strict decode rejects unknown keys, apiKey must be env:VAR, defaultMode is default|auto|yolo (plan is a run flag)` --semantically_similar_to--> `Config decode is strict: unknown keys rejected; adding a key requires defaults + validation + SPECS update`  [INFERRED] [semantically similar]
  internal/skills/builtin/self-configure/SKILL.md → AGENTS.md
- `Skills and prompts from other tools: prompts dir <name>.md becomes /name in the TUI; $ARGUMENTS / $@ / $1..$9 expansion; Claude Code and pi files drop in unchanged` --semantically_similar_to--> `Command file format: optional frontmatter (description shown in /help, argument-hint) then a prompt body with $$ARGUMENTS/$$@, $$1..$$9, $$$$ placeholders`  [AMBIGUOUS] [semantically similar]
  docs/specs/external-tools.md → internal/skills/builtin/create-command/SKILL.md
- `eight frozen tools list` --semantically_similar_to--> `Eight frozen tools`  [INFERRED] [semantically similar]
  README.md → internal/skills/builtin/self-knowledge/references/specs.md
- `rtk and graphify seeded as standard skills` --semantically_similar_to--> `graphify skill (SKILL.md)`  [INFERRED] [semantically similar]
  CHANGELOG.md → internal/skills/seed/graphify/SKILL.md

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **OpenAI dynamic client registration handshake** — docs_specs_oauth_verification_dynamic_client_registration, docs_specs_oauth_verification_agent_name_hint, docs_specs_oauth_verification_ext_agent_host_id, docs_specs_oauth_verification_issued_client_id [EXTRACTED 0.95]
- **--update incremental merge lifecycle** — internal_skills_seed_graphify_references_update_detect_incremental, internal_skills_seed_graphify_references_update_graphify_incremental_file, internal_skills_seed_graphify_references_update_graphify_detect_file, internal_skills_seed_graphify_references_update_build_merge, internal_skills_seed_graphify_references_update_save_manifest [EXTRACTED 0.95]
- **the eight frozen tools and their shared execution surface** — docs_specs_specs_read_tool, docs_specs_specs_write_tool, docs_specs_specs_edit_tool, docs_specs_specs_shell_tool, docs_specs_specs_search_tool, docs_specs_specs_ls_tool, docs_specs_specs_web_tool, docs_specs_specs_mcp_tool, docs_specs_specs_registry_run [EXTRACTED 1.00]
- **the permission-mode surface (modes, rules, auto table, classifier)** — docs_specs_specs_permission_modes_default_auto_yolo, docs_specs_specs_mode_default_auto_yolo_plan, docs_specs_specs_permissions_defaultmode, docs_specs_specs_layered_wildcard_rules_permissions_allow_ask_deny_trusted_project_layer_user_layer_deny_ask_allow, docs_specs_specs_auto_confinement_workdir_temp_writable, docs_specs_specs_auto_deterministic_risk_table_ladder_builtins_read_only_build_test_confined_mutators_project_wrecking_always_ask_unsure, docs_specs_specs_auto_mode_classifier_env_judge, docs_specs_specs_path_jail_workdir_read_only_roots_for_global_skills [EXTRACTED 1.00]
- **the three hand-rolled protocol adapters** — docs_specs_specs_anthropic_messages_adapter, docs_specs_specs_openai_completions_adapter, docs_specs_specs_openai_responses_adapter, docs_specs_specs_three_hand_rolled_protocol_codecs, docs_specs_specs_tool_calls_kept_never_dropped_arguments_as_received, docs_specs_specs_usage_normalization_input_excludes_cache_reads_writes [EXTRACTED 1.00]
- **Ecosystem SKILL.md compatibility corpus (graphify, Claude Code, pi, OpenCode)** — internal_skills_testdata_ecosystem_sources, internal_skills_testdata_ecosystem, internal_skills_testdata_ecosystem_graphify_graphify_vendored, internal_skills_testdata_ecosystem_claude_code_skill_creator_vendored, internal_skills_testdata_ecosystem_claude_code_frontend_design_vendored, internal_skills_testdata_ecosystem_pi_pi_subagents_vendored, internal_skills_testdata_ecosystem_pi_mcp_scripting_vendored, internal_skills_testdata_ecosystem_opencode_dotnet_unit_testing_vendored [EXTRACTED 1.00]
- **permission decision ladder** — changelog_three_permission_modes, changelog_auto_mode, changelog_auto_classifier, changelog_layered_rules [EXTRACTED 1.00]
- **The eight frozen tools share the Registry.Run contract** — internal_skills_builtin_self_knowledge_references_specs_eight_frozen_tools, internal_skills_builtin_self_knowledge_references_specs_read_tool, internal_skills_builtin_self_knowledge_references_specs_write_tool, internal_skills_builtin_self_knowledge_references_specs_edit_tool, internal_skills_builtin_self_knowledge_references_specs_shell_tool, internal_skills_builtin_self_knowledge_references_specs_search_tool, internal_skills_builtin_self_knowledge_references_specs_ls_tool, internal_skills_builtin_self_knowledge_references_specs_the_web_tool, internal_skills_builtin_self_knowledge_references_specs_mcp_tool_the_proxy_tool [EXTRACTED 1.00]
- **Permission modes and plan scope form the agent's enforcement surface** — internal_skills_builtin_self_knowledge_references_specs_permission_modes_applymode, internal_skills_builtin_self_knowledge_references_specs_auto_mode_ladder, internal_skills_builtin_self_knowledge_references_specs_auto_mode_risk_table, internal_skills_builtin_self_knowledge_references_specs_env_judge_classifier, internal_skills_builtin_self_knowledge_references_specs_yolo_mode_swaps_injected_checkers, internal_skills_builtin_self_knowledge_references_specs_trust_store, internal_skills_builtin_self_knowledge_references_specs_plan_mode_agent_mechanism, internal_skills_builtin_self_knowledge_references_specs_rules_engine, internal_skills_builtin_self_knowledge_references_specs_shell_analyser_parsed_commands, internal_skills_builtin_self_knowledge_references_specs_path_jail [EXTRACTED 1.00]
- **Three protocol adapters share one post() + WithRetry contract** — internal_skills_builtin_self_knowledge_references_specs_anthropic_messages_adapter, internal_skills_builtin_self_knowledge_references_specs_openai_completions_adapter, internal_skills_builtin_self_knowledge_references_specs_openai_responses_adapter, internal_skills_builtin_self_knowledge_references_specs_common_post_wire_contract, internal_skills_builtin_self_knowledge_references_specs_withretry_adapter_decorator, internal_skills_builtin_self_knowledge_references_specs_internal_provider [EXTRACTED 1.00]
- **Strict config flow: strict decode governs user and project config, apiKey env:VAR references, verified via moca mcp index** — agents_strict_config_decode, agents_apikey_env_reference, internal_skills_builtin_self_configure_user_config, internal_skills_builtin_self_configure_mcp_index_check, internal_skills_builtin_self_configure_strict_decode_reminders [EXTRACTED 1.00]
- **Synthetic frontmatter edge-case fixtures (CRLF, BOM, folding, quoting, rich keys, long description)** — internal_skills_testdata_ecosystem_synthetic_bom_skill, internal_skills_testdata_ecosystem_synthetic_colon_unquoted_skill, internal_skills_testdata_ecosystem_synthetic_crlf_skill, internal_skills_testdata_ecosystem_synthetic_folded_skill, internal_skills_testdata_ecosystem_synthetic_literal_skill, internal_skills_testdata_ecosystem_synthetic_long_description_skill, internal_skills_testdata_ecosystem_synthetic_rich_keys_skill [EXTRACTED 1.00]
- **moca skill discovery: seeded skills, builtin re-extraction, external copies and prompt templates reach the system prompt through skill files loaded via the read jail** — docs_specs_external_tools_seeded_skills, docs_specs_external_tools_builtin_skills, docs_specs_external_tools_skill_read_jail, docs_specs_external_tools_ecosystem_testdata, internal_skills_builtin_self_knowledge [INFERRED 0.75]
- **Human-in-the-loop validation before declaring done** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_claude_code_frontend_design_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_review_validation [INFERRED 0.75]
- **graph.json rebuild flow: new files, watched changes, commits and merges all refresh graphify-out/graph.json** — internal_skills_seed_graphify_references_transcribe_video_step, internal_skills_seed_graphify_references_add_watch_graphify_ingest, internal_skills_seed_graphify_references_add_watch_graphify_watch, internal_skills_seed_graphify_references_add_watch_graphify_update_pipeline, internal_skills_seed_graphify_references_github_and_merge_graphify_merge_graphs, internal_skills_seed_graphify_references_hooks_post_commit_hook [INFERRED 0.85]
- **Parallel subagent fanout with parent synthesis** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_graphify_graphify_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_skill [INFERRED 0.85]
- **Skills referencing bundled reference files for progressive disclosure** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_graphify_graphify_skill, internal_skills_testdata_ecosystem_pi_mcp_scripting_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_skill [INFERRED 0.85]
- **self-update release chain** — changelog_release_workflow, changelog_install_scripts, changelog_self_update, readme_release_page [INFERRED 0.90]
- **graphify extraction pipeline flow** — internal_skills_seed_graphify_skill_ast_extraction, internal_skills_seed_graphify_skill_semantic_extraction, internal_skills_seed_graphify_skill_community_detection, internal_skills_seed_graphify_skill_bfs_query [INFERRED 0.95]
- **query → answer → save-result → work-memory feedback loop** — internal_skills_seed_graphify_references_query_graphify_query_cli, internal_skills_seed_graphify_references_query_save_result_feedback_loop, internal_skills_seed_graphify_references_query_work_memory_outcomes, internal_skills_seed_graphify_references_query_lessons_md [INFERRED 0.95]
- **Token-efficiency mechanisms implementing the minimal-agent philosophy** — readme_minimal_tui_coding_agent, readme_prompt_caching, readme_diff_shaped_results, readme_token_denominated_compaction, internal_agent_system_prompt_token_discipline, readme_rtk_token_saver, readme_graphify_token_saver [INFERRED 0.95]

## Communities (590 total, 406 thin omitted)

### Community 0 - "go_pkg_strings"
Cohesion: 0.11
Nodes (19): request, usageError, anthropicEvent, anthropicUsage, completionsChunk, Credential, ollamaTags, responsesEvent (+11 more)

### Community 10 - "protocol.go"
Cohesion: 0.06
Nodes (54): AgentCapabilities, AuthMethod, AvailableCommand, AvailableCommandInput, AvailableCommandsUpdate, CancelNotification, ClientCapabilities, ClientSessionCapabilities (+46 more)

### Community 100 - ".Run"
Cohesion: 0.15
Nodes (11): shellTool, TestVerifies(), verifies(), approve(), findTestFile(), isPathByte(), TestFindTestFile(), TestTestRunCommand() (+3 more)

### Community 101 - "Render"
Cohesion: 0.14
Nodes (7): jsonItem, main(), Render(), TestRenderTotal(), main(), RenderJSON(), TestRenderJSON()

### Community 104 - "fakeAS"
Cohesion: 0.26
Nodes (3): fakeAS, TestValidateIDToken(), TestValidateIDTokenKidSelection()

### Community 11 - "startClient"
Cohesion: 0.12
Nodes (46): scriptServer, cancelledOutcome(), selected(), startClient(), testOptions(), TestSlashTemplateExpandsAndIsAdvertised(), TestInitializeAdvertisesLoadAndList(), TestListAndLoad() (+38 more)

### Community 111 - "fetch.go"
Cohesion: 0.23
Nodes (8): Client, New(), TestGet(), Plan: on-disk response cache, Cache type (dir, TTL) in cache.go, Client.Get in fetch.go uses the cache, TTL option on New, Eval prompt: plan an on-disk response cache

### Community 112 - "auto_net.go"
Cohesion: 0.32
Nodes (10): cursor, atRead(), curlLongAsk(), curlVerdict(), fileRead(), getVerb(), netURL(), wgetLong() (+2 more)

### Community 117 - "BuildSystemPrompt"
Cohesion: 0.20
Nodes (11): PromptInput, ServerLine, Instruction, BuildSystemPrompt(), oneLine(), renderPrompt(), TestBuildSystemPrompt(), TestPromptRTKIsConditional() (+3 more)

### Community 12 - "permissions/shell.go"
Cohesion: 0.09
Nodes (30): Jail, cmdState, optSpec, Shell, verdictAcc, resolveDeep(), within(), withinAny() (+22 more)

### Community 124 - "acp_test.go"
Cohesion: 0.33
Nodes (7): argKind, getoptSpec, optHit, parsedArgs, getopt(), resolveLong(), main()

### Community 128 - "LineReader"
Cohesion: 0.27
Nodes (8): LineReader, lineResult, LoginIO, pastedResult, parsePasted(), pasteReader(), TestParsePasted(), readLine()

### Community 13 - "Word"
Cohesion: 0.16
Nodes (46): autoVerdict, interpSpec, pathClass, pathFunc, rootGuard, Word, anyNonLiteral(), anyRoot() (+38 more)

### Community 135 - "Summarize"
Cohesion: 0.31
Nodes (8): Metrics, Collect(), fatal(), main(), meanSD(), readResults(), Summarize(), TestSummarize()

### Community 136 - "fixture/order.go"
Cohesion: 0.28
Nodes (6): Order, User, NewOrder(), TestOrderEmail(), TestUserEmail(), NewUser()

### Community 14 - "Parse"
Cohesion: 0.06
Nodes (49): scanner, span, Parse(), TestAPIKeyOptional(), TestDefaultsApplied(), TestEnvRefs(), TestOAuthAllowedForOpenAI(), TestOAuthRejectedForNonOAuthProvider() (+41 more)

### Community 140 - "schema_test.go"
Cohesion: 0.61
Nodes (7): node, check(), jsonType(), loadDefs(), resolve(), TestConformRejectsBadShapes(), typeOK()

### Community 142 - "LoadTrust"
Cohesion: 0.32
Nodes (6): TrustStore, acpTrust(), TestACPTrust(), canonical(), LoadTrust(), TestTrustStore()

### Community 147 - "reference/order.go"
Cohesion: 0.32
Nodes (5): Order, User, NewOrder(), NewUser(), normalizeEmail()

### Community 148 - "checkers"
Cohesion: 0.29
Nodes (4): checkers, Asker, CommandChecker, PathChecker

### Community 15 - "auto_test.go"
Cohesion: 0.12
Nodes (48): Rules, verdict, rules(), newAutoShell(), TestAutoFixRound1(), TestAutoFixRound2(), TestAutoGitSymlinkAndWritableAncestor(), TestAutoLadder() (+40 more)

### Community 152 - "validateIDToken"
Cohesion: 0.38
Nodes (6): idClaims, audContains(), b64d(), jwksKey(), TestJWKSExponentSanity(), validateIDToken()

### Community 153 - "parseIgnore"
Cohesion: 0.38
Nodes (6): ignoreRule, globToRegexp(), ignored(), parseIgnore(), TestIgnore(), ListFiles()

### Community 16 - "Run"
Cohesion: 0.05
Nodes (30): Event, agentEventMsg, AppOptions, eventPipe, historyRec, loginWriter, pagerModel, trustModel (+22 more)

### Community 166 - "tui/log_test.go"
Cohesion: 0.47
Nodes (3): logSink, captureLog(), TestSetupFirstSessionLogged()

### Community 174 - "CallResult"
Cohesion: 0.40
Nodes (4): CallResult, Content, response, rpcError

### Community 18 - "PermMode"
Cohesion: 0.07
Nodes (41): ModeChanged, PermMode, palette, role, segment, StatusInfo, badgeText(), firstLineOf() (+33 more)

### Community 19 - "update/update.go"
Cohesion: 0.08
Nodes (35): Asset, Client, Release, Version, envOr(), githubToken(), runUpdate(), Compare() (+27 more)

### Community 20 - "context.Context"
Cohesion: 0.06
Nodes (27): fakeSum, httpTransport, Unjailed, Discovery, HTTPError, cutUTF8(), isCont(), parseJudge() (+19 more)

### Community 21 - "encoding/json.RawMessage"
Cohesion: 0.12
Nodes (11): echoHandler, Server, buildCommands(), flatten(), mentionPath(), TestBuildCommandsPrecedence(), sessionsDir(), busy() (+3 more)

### Community 22 - "Message"
Cohesion: 0.08
Nodes (29): Kind, Budget, Entry, Prev, Message, Role, ToolResult, use (+21 more)

### Community 24 - "Model"
Cohesion: 0.08
Nodes (22): Status, ModelOverride, CostConfig, ProviderConfig, Effort, Model, effortLevels(), effortOption() (+14 more)

### Community 25 - "net/http.Client"
Cohesion: 0.09
Nodes (23): Summarizer, ContentBlock, Request, Response, StopReason, Adapter, BlockType, Options (+15 more)

### Community 26 - "Skill"
Cohesion: 0.07
Nodes (35): Prompt, Skill, filterSkills(), TestFilterSkillsDropsSeededSkillsWithoutTheirCLI(), TestEcosystemSkillsLoadUnchanged(), closingQuote(), ParseFrontmatter(), scalar() (+27 more)

### Community 27 - "retry_test.go"
Cohesion: 0.12
Nodes (28): Retry, Event, RetryNotice, EventType, logSink, RetryPolicy, scripted, seqAdapter (+20 more)

### Community 28 - "Input"
Cohesion: 0.10
Nodes (8): ReadTracker, stamp, Input, padGuard, paste, stampOf(), lineCount(), hasControl()

### Community 29 - "NewManager"
Cohesion: 0.13
Nodes (33): recTransport, crashingServer(), shServer(), skipWindows(), TestCallTimeoutStopsWedgedServer(), TestHandshakeTimeout(), TestHTTPAbortedStreamReplayRule(), TestIndexSaveMergesConcurrentWriters() (+25 more)

### Community 3 - "newScript"
Cohesion: 0.05
Nodes (108): scriptServer, newScript(), startTest(), startTestWith(), startWith(), TestAbortWritesSyntheticResults(), TestFailedCallDoesNotCancelRest(), TestLengthStopTruncatedToolCall() (+100 more)

### Community 31 - "webFetch"
Cohesion: 0.08
Nodes (27): countingTransport, webResult, webTool, Spill(), cutWeb(), isAlnumByte(), isSpaceByte(), TestWebDataAttrsAreNotLinkAttrs() (+19 more)

### Community 32 - "NewRegistry"
Cohesion: 0.10
Nodes (31): fakeOllama, ollamaShow, modelFromShow(), newFakeOllama(), ollamaRegistry(), TestNoOllamaNoProbe(), TestOllamaChatWithoutKey(), TestOllamaDiscovery() (+23 more)

### Community 36 - "session"
Cohesion: 0.13
Nodes (6): session, newMessageID(), newSession(), summaryOf(), summaryText(), TestSummaryText()

### Community 37 - "sync.Mutex"
Cohesion: 0.09
Nodes (13): logSink, logSink, ring, stdioTransport, logSink, responseID(), TestResponseID(), startStdio() (+5 more)

### Community 38 - "Config"
Cohesion: 0.09
Nodes (19): AutoConfig, ContextConfig, LogConfig, PermissionsConfig, ShellConfig, SnapshotConfig, TUIConfig, WebConfig (+11 more)

### Community 39 - "Manager"
Cohesion: 0.14
Nodes (12): Manager, Annotations, Hit, Index, IndexEntry, IndexTool, state, ConfigHash() (+4 more)

### Community 4 - "model"
Cohesion: 0.07
Nodes (18): compactDoneMsg, InputKind, loginChoice, loginDoneMsg, loginState, loginStep, logoutDoneMsg, model (+10 more)

### Community 42 - "oauth_test.go"
Cohesion: 0.17
Nodes (22): syncBuf, browse(), firstURL(), newFakeAS(), runLogin(), TestCallbackPageReportsFailure(), TestLineReaderAbandonedReadKeepsLine(), TestLoginAccessDenied() (+14 more)

### Community 45 - "rules.go"
Cohesion: 0.17
Nodes (20): Decision, matcher, Rule, RuleSet, globWord(), matchWords(), ParseRule(), lit() (+12 more)

### Community 47 - "skills/prune_test.go"
Cohesion: 0.15
Nodes (23): Pruned, DetectVerify(), fileContains(), makeTargets(), TestDetectVerify(), builtinHash(), ExtractBuiltins(), extractBuiltins() (+15 more)

### Community 5 - "testing.T"
Cohesion: 0.03
Nodes (92): ShellOutput, evt, shellDoneMsg, parseArgs(), TestParseArgs(), TestParseArgsExecuteUsageErrors(), ptr(), TestPlanNeverResolvesWider() (+84 more)

### Community 50 - "Start"
Cohesion: 0.12
Nodes (22): setup, StartOptions, Dir, promptDirs(), TestDefaultHTTPClientBoundsHeaderWait(), planMode(), GitState(), Platform() (+14 more)

### Community 53 - ".dropdown"
Cohesion: 0.11
Nodes (19): dropItem, dropState, TestSkillSubmissionStartsARun(), matchCommands(), contains(), count(), dropNames(), TestDropdownListsCommandsAndPrompts() (+11 more)

### Community 54 - "importer.go"
Cohesion: 0.15
Nodes (21): MCPConfig, MCPServer, claudeServer, EnvVar, opencodeServer, Source, TestImportRewritesEmbeddedSecretsInValues(), TestRewriteSecretsValueRuleAlone() (+13 more)

### Community 57 - "initialize"
Cohesion: 0.14
Nodes (15): client, headerLog, Tool, transport, initialize(), TestHTTPCloseBounded(), TestListToolsRepeatedCursor(), startHTTP() (+7 more)

### Community 6 - "run"
Cohesion: 0.04
Nodes (87): lockedBuf, TestACPBadFlags(), TestACPServesOnlyJSONRPC(), fakeCompletions(), isolate(), TestExitCodes(), TestHelpListsSurface(), TestInterruptedExit130() (+79 more)

### Community 61 - "Conn"
Cohesion: 0.18
Nodes (7): Conn, Deferred, Handler, message, Options, NewConn(), Serve()

### Community 64 - "Store"
Cohesion: 0.19
Nodes (8): Store, storeFile, newHostID(), validateAPIKey(), File format, Rules, Steps, Where commands live

### Community 66 - "testClient"
Cohesion: 0.22
Nodes (10): RPCError, testClient, wireMsg, newSessionFull(), TestModeFlagSetsInitialMode(), TestNewSessionAdvertisesModesAndModels(), TestSetMode(), TestSetModel() (+2 more)

### Community 67 - "Env"
Cohesion: 0.17
Nodes (8): ToolEnd, Env, Result, Snapshotter, trackFailure(), decode(), errorf(), runRecovered()

### Community 68 - "applog_test.go"
Cohesion: 0.16
Nodes (16): Level, wc, Discard(), Open(), ParseLevel(), prune(), keepDefault(), TestCappedWriterRecordAndFileCaps() (+8 more)

### Community 7 - "Create"
Cohesion: 0.05
Nodes (52): Info, Snapshots, Writer, fileInfo, Header, SnapshotRec, TestExitForOAuthSentinels(), TestParseArgsExecute() (+44 more)

### Community 74 - ".Run"
Cohesion: 0.15
Nodes (17): ProxyTool, TestProxyPlanModeCallFollowsNormalGating(), cutRunes(), errResult(), isTrue(), NewTool(), newProxy(), runP() (+9 more)

### Community 75 - "Login"
Cohesion: 0.18
Nodes (13): OAuthConfig, Refresher, Token, fakeSIWC(), fakeSIWCOpts(), Login(), missingScopes(), pkce() (+5 more)

### Community 78 - "acpgate_test.go"
Cohesion: 0.19
Nodes (9): proc, cappedWriter, cutRecord(), kinds(), provider(), shell(), start(), TestACPGate() (+1 more)

### Community 79 - "Registry"
Cohesion: 0.16
Nodes (9): Registry, CredentialFunc, IsLocalProvider(), anyProviderUsable(), For /graphify explain, For /graphify path, graphify reference: query, path, explain, Step 0 — Constrained query expansion (REQUIRED before traversal) (+1 more)

### Community 8 - "Agent"
Cohesion: 0.06
Nodes (27): Options, savedModel, ToolStart, TurnEnd, Result, Agent, Outcome, ToolCall (+19 more)

### Community 80 - "ToolSpec"
Cohesion: 0.16
Nodes (9): ToolSpec, Registry, lsTool, mcpStub, readTool, searchTool, Tool, TestProxySpecFrozen() (+1 more)

### Community 82 - ".Run"
Cohesion: 0.14
Nodes (10): ApprovablePaths, editTool, writeTool, planAllows(), isDocPath(), noteChange(), askOutsideWrite(), countFileLines() (+2 more)

### Community 83 - "renderMarkdown"
Cohesion: 0.19
Nodes (12): mdState, codeBandStyle(), fenceLang(), renderInline(), renderMarkdown(), plainMD(), TestMarkdownBlocks(), TestMarkdownFence() (+4 more)

### Community 87 - "NewRegistry"
Cohesion: 0.14
Nodes (9): echoTool, spillFailTool, TestRegistryRecoversToolPanic(), TestRepeatedFailureHintAndReset(), TestRepeatedFailureIgnoresSpillPath(), runReg(), TestRegistryInvestigationBanner(), NewRegistry() (+1 more)

### Community 89 - "endUpdate"
Cohesion: 0.19
Nodes (14): toolInput, absPath(), capHead(), capTail(), endUpdate(), parseInput(), startUpdate(), call() (+6 more)

### Community 9 - "Sanitize"
Cohesion: 0.06
Nodes (29): Answer, Question, approvalMsg, approvalOption, approvalState, Item, Items, AutoAllow() (+21 more)

### Community 99 - "applyEdit"
Cohesion: 0.23
Nodes (14): hunk, applyEdit(), formatDiff(), indentOf(), joinInts(), lineOf(), matchLines(), replaceExact() (+6 more)

### Community 107 - "styleBlock"
Cohesion: 0.31
Nodes (10): respMsgStyle(), styleBlock(), styleBlockLead(), TestCommandEchoesAsUserMessage(), TestMessageBackgroundsCoverTheFullRow(), TestResponseCommitUsesBackground(), TestScrollbackMessageBackgrounds(), TestStyleBlockExpandsTabs() (+2 more)

### Community 108 - "sessions_test.go"
Cohesion: 0.41
Nodes (11): sessionsDir(), openSessionPicker(), pickRow(), seedSession(), sessionIDOf(), TestSessionsCurrentCannotBeDeleted(), TestSessionsDeleteFlow(), TestSessionsDeleteRefusedForOpenSession() (+3 more)

### Community 109 - "shipgate/fixture/main.go"
Cohesion: 0.22
Nodes (7): Sum(), TestTotals(), ParseList(), Average(), Max(), TestMaxAndAverage(), main()

### Community 116 - "input_test.go"
Cohesion: 0.30
Nodes (11): NewInput(), TestCollapseUsesExactOffset(), TestCtrlC(), TestEditedMarkerSentLiterally(), TestHistory(), TestHistoryRecallChipsUnsafeContent(), TestMarkerNeverCollidesWithTypedText(), TestPasteChipKeepsContent() (+3 more)

### Community 119 - "seedSkills"
Cohesion: 0.31
Nodes (10): SeedSkills(), seedSkills(), readFile(), seedFSFixture(), TestEmbeddedSeedSkills(), TestSeedSkillsCreatesMissing(), TestSeedSkillsFillsOnlyTheMissingOne(), TestSeedSkillsLeavesExistingAlone() (+2 more)

### Community 122 - "format.go"
Cohesion: 0.27
Nodes (9): TestGitStatusUsesNoOptionalLocks(), FmtTokens(), FmtWindow(), GitBranch(), gitStatusArgs(), TestFormatters(), TestGitBranch(), TestGitBranchUnborn() (+1 more)

### Community 127 - "redact.go"
Cohesion: 0.29
Nodes (6): TestSegments(), cutString(), isSecretKey(), replaceAttr(), safeURL(), segments()

### Community 129 - "net/http/httptest.Server"
Cohesion: 0.29
Nodes (8): ollamaServer(), ollamaStart(), TestOllamaDeclaredWindowIsQuiet(), TestOllamaReportedWindowIsQuiet(), TestOllamaStartErrors(), TestOllamaTinyWindowWarns(), TestStartWithOllamaModel(), abortingHTTPServer()

### Community 130 - "vague/testdata/reference/main.go"
Cohesion: 0.29
Nodes (7): TestCollect(), CToF(), FToC(), TestRoundTrip(), main(), fail(), main()

### Community 131 - "moca/prune_test.go"
Cohesion: 0.47
Nodes (8): execPrune(), pruneEnv(), TestRunPruneDryRunRemovesNothing(), TestRunPruneIgnoresBrokenConfig(), TestRunPruneNothingToDo(), TestRunPruneRejectsUnknownArgs(), TestRunPruneRemovesOlderBuiltinDirs(), TestUsageListsPrune()

### Community 137 - "ResolveEnv"
Cohesion: 0.25
Nodes (9): webSearchKey(), TestResolveEnv(), ResolveEnv(), TestKeepNameWindows(), FilterEnv(), keepName(), serverEnv(), TestFilterEnv() (+1 more)

### Community 163 - "overflowDir"
Cohesion: 0.47
Nodes (6): TestPruneOverflow(), TestSpillDirsAreRandom(), newestModTime(), newSpillDir(), overflowDir(), pruneOverflow()

### Community 167 - "fixture/slug.go"
Cohesion: 0.40
Nodes (4): Limit(), Make(), TestLimit(), TestMake()

### Community 172 - "expand"
Cohesion: 0.50
Nodes (5): arg(), braceExpr(), closingBrace(), expand(), slice()

### Community 175 - "Lookup"
Cohesion: 0.50
Nodes (3): Lookup(), TestLookupBounds(), Valid()

### Community 2 - "newTestModel"
Cohesion: 0.04
Nodes (109): NewDefaultStore(), TestActivityLine(), TestCoalescedShrinkRemembersTheLastFlushedRow(), TestInputBoxGrowsAndShrinks(), TestPagerCloseDefersTheRelease(), TestRefuseRunningWhileOutputInFlight(), TestShrinkIsPaddedUntilTheGuardWindowPasses(), TestShrinkPaddingFitsTheScreen() (+101 more)

### Community 23 - "anthropic_test.go"
Cohesion: 0.16
Nodes (36): newAnthropic(), keyCred(), mustJSON(), sseServer(), TestAnthropicBodyHygiene(), TestAnthropicBudgetAndNoCache(), TestAnthropicBudgetFollowsEffort(), TestAnthropicCredentialHeaders() (+28 more)

### Community 43 - "cut_test.go"
Cohesion: 0.17
Nodes (23): FindCut(), a(), call(), res(), TestFindCutKeepsRecentAtValidPoint(), TestFindCutNeverBetweenCallAndResult(), TestFindCutNothingToDo(), TestFindCutSplitTurn() (+15 more)

### Community 51 - "io.Writer"
Cohesion: 0.16
Nodes (17): runACP(), printUsage(), startLog(), loginAPIKey(), runLogin(), runLogout(), yesNo(), run() (+9 more)

### Community 77 - "simulate"
Cohesion: 0.16
Nodes (18): TestExitCommandQuits(), TestEditItemShowsDiff(), simulate(), TestResumeLastIsNewestOtherHere(), TestShiftTabCyclesEffort(), setupCfg(), setupEnv(), setupModel() (+10 more)

### Community 84 - "rpc_test.go"
Cohesion: 0.33
Nodes (14): pipePeer(), readMsg(), TestCallCascadesCancel(), TestCallFailsOnEOF(), TestCancelRequest(), TestConcurrentDispatch(), TestConcurrentWritesAreWholeLines(), TestDeferredRunsAfterResponse() (+6 more)

### Community 88 - "DataDir"
Cohesion: 0.18
Nodes (13): main(), runMCPIndex(), decideTrust(), hasProjectResources(), modeWithoutProject(), runTUI(), TestStateDir(), AuthFile() (+5 more)

### Community 97 - "moca/log_test.go"
Cohesion: 0.27
Nodes (13): logRun(), logRunCfg(), logRunIn(), readLog(), sessionID8(), TestLogDebugViaEnv(), TestLogFileForOneShot(), TestLogInvalidEnvIsUsageError() (+5 more)

### Community 120 - "transcribe.md"
Cohesion: 0.22
Nodes (8): graphify reference: transcribe video and audio, Step 2.5 - Transcribe video / audio files (only if video files detected), graphify.transcribe.transcribe_all(video_files, initial_prompt=prompt), Token reduction benchmark (graphify benchmark) gated on total_words > 5000 from .graphify_detect.json, Step 2.5: transcribe detected video files (skip entirely when detect returns zero video); transcripts join the docs list before semantic subagents (Step 3B), Whisper transcription: GRAPHIFY_WHISPER_MODEL (default base) and GRAPHIFY_WHISPER_PROMPT must be exported for the child process, graphify-out/.graphify_detect.json holds detect output incl. total_words and per-type file lists, graphify-out/.graphify_transcripts.json: written from Python, not a shell redirect, because Whisper progress on stdout would corrupt the JSON (#1392)

### Community 123 - "github-and-merge.md"
Cohesion: 0.27
Nodes (8): graphify reference: GitHub clone and cross-repo merge, Step 0 - Clone GitHub repo(s) (only if a GitHub URL was given), graphify clone <github-url> [--branch]: clones into ~/.graphify/repos/<owner>/<repo> and reuses existing clones on repeat runs, graphify merge-graphs: merge per-repo or per-subfolder graph.json files into one cross-repo graph, Fast path: once graphify-out/graph.json exists, any codebase question runs graphify query directly on the merged graph - no re-extraction, no size gate, ~/.graphify/repos/<owner>/<repo> clone layout; merged graph nodes carry a repo attribute so you can filter by origin, Multiple local subfolders: run the CLI directly per subfolder (graphify extract ./core/, ./service/, ./platform/ with --backend choice), then merge at project root, moca's own knowledge graph workflow: graphify query for codebase questions, path for relationships, explain for concepts, update . after modifying code (AST-only, no API cost)

### Community 165 - "hooks.md"
Cohesion: 0.33
Nodes (5): For git commit hook, For native CLAUDE.md integration, graphify reference: commit hook and native CLAUDE.md integration, graphify claude install/uninstall: writes a ## graphify section to CLAUDE.md so graphify is always-on in Claude Code sessions, Project CLAUDE.md receives the ## graphify section (check the graph before codebase answers, rebuild after code changes)

### Community 81 - "exports.md"
Cohesion: 0.12
Nodes (16): graphify reference: extra exports and benchmark, Step 6b - Wiki (only if --wiki flag), Step 7 - Neo4j export (only if --neo4j or --neo4j-push flag), Step 7a - FalkorDB export (only if --falkordb or --falkordb-push flag), Step 7b - SVG export (only if --svg flag), Step 7c - GraphML export (only if --graphml flag), Step 7d - MCP server (only if --mcp flag), Step 8 - Token reduction benchmark (only if total_words > 5000) (+8 more)

### Community 94 - "graphify.ingest.ingest(url, raw_dir, author, contributor) auto-detects URL type; errors are reported, never silently continued"
Cohesion: 0.16
Nodes (13): For /graphify add, For --watch, graphify reference: add a URL and watch a folder, graphify.ingest.ingest(url, raw_dir, author, contributor) auto-detects URL type; errors are reported, never silently continued, graphify.watch background watcher: --debounce 3, reads the trusted graphify-out/.graphify_root so a metacharacter-bearing scan root can never be re-interpreted, graphify hook install/uninstall/status: post-commit hook detects changes via git diff HEAD~1, re-runs AST extraction on changed code files, rebuilds graph.json and GRAPH_REPORT.md; doc/image changes ignored, Code-only changes (.py/.ts/.go/?) re-run AST extraction + rebuild + cluster immediately, no LLM; graph.json and GRAPH_REPORT.md updated automatically, /graphify --update pipeline: merges a newly saved file into the existing graph (LLM semantic re-extraction) (+5 more)

### Community 141 - "BASELINE.md"
Cohesion: 0.29
Nodes (6): Eval baseline — DESIGN rev 21 harness pass, Found by the evals (fixed in rev 21), Reading it, Results, go run ./test/evals/evalstats -summary: compares labels and produces the summary, Eval corpus rev 21 (not in CI): seven scenarios, test/evals/run.sh, metrics in gitignored test/evals/results/, compared against BASELINE.md

### Community 49 - "system-prompt.md"
Cohesion: 0.10
Nodes (14): Token discipline, Environment, Tools, Working style, Web tool (fetch + search, keyless), Long outputs are cut and the full text saved to a file named in the result: read or search that file instead of re-running, mcp tool: search, then describe, then call the listed MCP servers, shell is stateless: every call starts in the workdir; use cd dir && cmd in one call (+6 more)

### Community 103 - "moca login <provider>"
Cohesion: 0.17
Nodes (11): --api-key, ~/.config/moca/auth.json (credential store), missing model error (exit 2), moca login key prompt (stdin only), moca login <provider>, moca logout <provider>, --no-browser, openai ships OAuth via Sign in with ChatGPT (+3 more)

### Community 154 - "self-configure/SKILL.md"
Cohesion: 0.29
Nodes (6): How, Installation, Never, Verify, What you can change, When it takes effect

### Community 181 - "graphify reference: incremental update and cluster-only"
Cohesion: 0.50
Nodes (3): For --cluster-only, For --update (incremental re-extraction), graphify reference: incremental update and cluster-only

### Community 185 - "evals/run.sh"
Cohesion: 1.00
Nodes (3): fresh_repo(), one(), run.sh script

### Community 40 - "install.sh"
Cohesion: 0.10
Nodes (19): die(), fetch(), note(), install.sh script, usage(), internal/update, test/shipgate ship gate, checksums.txt (+11 more)

### Community 46 - "README.md"
Cohesion: 0.10
Nodes (25): release notes, Ollama provider, Ollama failure modes: a server that is down fails at once with how to start it (no retry ladder); a model never pulled says ollama pull name; a key via slash login ollama is optional for servers behind an authenticating proxy; nothing is probed unless you opt in, Eval prompt: latest stable Go release -> GO_VERSION.txt, approval prompt as arrow-key picker, Ollama provider local and LAN, skills invokable from the / picker, moca acp Agent Client Protocol server (+17 more)

### Community 91 - "2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)"
Cohesion: 0.13
Nodes (12): 1.1 Terms, 1.2 Client registration, 1.3 Decision, 1. anthropic — decision: `api_key only` (subscription OAuth not permitted), 2.1 Terms, 2.2 Client registration (per the docs), 2.3 Endpoints (verified live 2026-10-06 against `https://auth.openai.com/.well-known/openid-configuration`), 2.4 Inference contract for subscription traffic (open-source flow docs) (+4 more)

### Community 95 - "SOURCES.md"
Cohesion: 0.19
Nodes (7): anthropics/skills frontend-design, anthropics/skills skill-creator, graphify skill (graphify install, claude platform; byte-identical to pi variant), adeotek/dotfiles opencode dotnet-unit-testing skill, npm pi-mcp-adapter mcp-scripting skill, npm pi-subagents skill, internal/skills/testdata/ecosystem corpus

### Community 44 - "release v0.1.0.alpha"
Cohesion: 0.10
Nodes (24): Agent-Skills SKILL.md compatibility, graphify install --platform moca (upstream PR), Anthropic API-key-only policy gate, cross-provider history transform, custom providers via baseUrl and protocol, graphify allowlist and guidance skill, Makefile and mise mirrors, OpenAI subscription OAuth PKCE (+16 more)

### Community 90 - "README.md - moca"
Cohesion: 0.21
Nodes (13): First-run setup mode, Prompt caching, Diff-shaped tool results, Do mode (--do//do), config.jsonc (JSONC, $XDG_CONFIG_HOME honored), MCP lazy ~200-token proxy tool, Minimal TUI coding agent, Provider auth modes (anthropic: API key only; openai: subscription OAuth) (+5 more)

### Community 168 - "CI Workflow (gofmt · vet · build · test)"
Cohesion: 0.40
Nodes (4): go.mod (Go version source for setup-go), CI gate set (gofmt, go vet, go build, go test -race), CI Workflow (gofmt · vet · build · test), SPECS.md §2 — CI gate definition

### Community 183 - "Claude Code frontend-design skill"
Cohesion: 0.50
Nodes (3): AI-generated design default tells, frontend-design Apache 2.0 License, Claude Code frontend-design skill

### Community 184 - "pi mcp-scripting skill"
Cohesion: 0.67
Nodes (4): jev.evaluate semantic scripting gate, mcpScript tools API, jev.evaluate reference, pi mcp-scripting skill

### Community 59 - "release v0.1.1"
Cohesion: 0.13
Nodes (18): slash login in the TUI: pick a provider then paste an API key masked never echoed never written to the transcript or sign in to a subscription in the browser; while a sign-in is in flight the pasted redirect URL or code from the input box feeds the flow and esc cancels; after a successful ChatGPT sign-in the wizard offers to flip providers openai auth to oauth; slash logout revokes a subscription session best-effort and clears the stored token and key, moca update replaces the installed binary with the latest GitHub release package: checks the newest published release skipping drafts, picks the package for the platform, verifies against checksums, and swaps the binary in place atomically with a symlinked launcher having its target replaced; sessions config and credentials never touched; check mode only reports; a dev or dirty build or one newer reports not newer; a platform without a package like darwin points at make release; runs before config is loaded so a broken config cannot block an update, credentials in auth.json 0600, dollar-brace bash expansion in prompt templates, Go version pinned in go.mod, install.sh and install.ps1, TUI /login and /logout, ten more opencode-go catalog models (+10 more)

### Community 62 - "graphify codebase knowledge graph CLI, in default allowlist, seeded into ~/.config/moca/skills/graphify/ (SKILL.md + references)"
Cohesion: 0.12
Nodes (17): graphify (codebase knowledge graph), rtk (token-compressed CLI output), Graphify-Labs/graphify PR #4174 (graphify install --platform moca, contributed upstream), internal/permissions/rtk.go captures subcommand classes from the real CLI (rtk 0.51.0), internal/skills/testdata/ecosystem corpus proves cross-tool SKILL.md compatibility (provenance and licenses in SOURCES.md), graphify codebase knowledge graph CLI, in default allowlist, seeded into ~/.config/moca/skills/graphify/ (SKILL.md + references), rtk token-compressed CLI proxy, in default allowlist, ships seeded skill teaching rtk <cmd> variants, rtk subcommands writing config/agent hooks (init, config, trust, untrust, learn, telemetry) ask once (+9 more)

### Community 76 - "Release - build packages, publish GitHub Release"
Cohesion: 0.19
Nodes (16): Release - build packages, publish GitHub Release, build job - package matrix targets, release job - publish the GitHub Release, Download artifacts with merge-multiple, Upload package artifacts, Asset naming moca-<version>-<os>-<arch>.tar.gz|zip, Build matrix (linux/amd64, linux/arm64, windows/amd64), sha256sum checksums.txt generation (+8 more)

### Community 96 - "Claude Code skill-creator skill"
Cohesion: 0.16
Nodes (13): Skill description (trigger) optimization, Skill eval/iterate loop (with-skill vs baseline), SKILL.md anatomy + bundled resources layout, Constrained query expansion against graph vocab, Progressive Disclosure (three-level skill loading), Self-improving save-result/reflect loop, Fresh-context reviewer disposition, skill-creator Apache 2.0 License (Anthropic) (+5 more)

### Community 150 - "Anthropic: third-party subscription OAuth not permitted"
Cohesion: 0.29
Nodes (5): Claude legal-and-compliance usage policy (primary source), 2026-01-09 server-side credential checks / account bans, 2026-02-18/19 clause formalized in Claude Code legal docs, OpenCode removes Claude subscription support, corroborating enforcement timeline 2026-01 to 2026-09

### Community 133 - "anthropic decision: api_key only"
Cohesion: 0.25
Nodes (6): Sign in with ChatGPT open-source token sharing, partner directory open-source integrations (OpenClaw/OpenCode/Pi/T3), config validation rejects providers.anthropic.auth=oauth, moca, OpenClaw (listed partner), OpenCode (listed partner)

### Community 1 - "SPECS.md (moca living spec)"
Cohesion: 0.01
Nodes (253): 30K-truncated MCP rendering, 60 s handshake timeout + 15 min tools/call timeout, 884 top-level tests across 17 Go packages (go test ./... -race), a length-stopped turn's calls all get the "cut off — split the work" error, access tokens ~1 h, ACP error codes (-32602 / -32002 / -32800 / -32000 / -32603 / -32700), ACP loads the config once at startup, ACP method authenticate (not supported: run `moca login`) (+245 more)

### Community 110 - "moca mcp import"
Cohesion: 0.20
Nodes (11): config.SetObjectEntry (insert a key), credential sniffing (Bearer/Basic, known prefixes, password pairs, URL userinfo), importer discovery (~/.claude.json, .mcp.json, opencode config, Pi files), `moca mcp import` uses config.SetObjectEntry, `${VAR}`/`$VAR` → env:VAR, mcp import discovery sources, mcp import server shapes, moca mcp import (+3 more)

### Community 118 - "keys dispatch"
Cohesion: 0.18
Nodes (10): append-only JSONL sessions, alt+t thinking pager, ctrl+c double-press quit, ctrl+o pager on latest item, esc interrupt semantics, keys dispatch, ReportAllKeysAsEscapeCodes, shift+tab mode cycle (+2 more)

### Community 126 - ".github/workflows/release.yml"
Cohesion: 0.22
Nodes (8): checksums.txt (sha256sum format), config.Version ldflags -X stamp, git describe --tags --always --dirty, moca update, release asset names moca-<version>-<os>-<arch>.tar.gz|zip, release packages linux/amd64, linux/arm64, windows/amd64 (manual Release workflow run), .github/workflows/release.yml, make build

### Community 134 - "openai-responses adapter"
Cohesion: 0.22
Nodes (9): a 2xx whose Content-Type is not text/event-stream is an error, openai-completions adapter, openai-completions chunks and finish_reason, openai-completions usage split (cached_tokens → CacheRead), openai-responses adapter, openai-responses request `input` items, reasoning items carry `encrypted_content`, responses reasoning-summary delta fallback (+1 more)

### Community 143 - "anthropic-messages adapter"
Cohesion: 0.25
Nodes (8): 120 s response-header bound (client timeout), anthropic-messages adapter, anthropic-messages `message_stop` absent → `io.ErrUnexpectedEOF`, anthropic-messages stream events (message_start → message_stop), anthropic stop-reason mapping (tool_use/length/refusal/end_turn), redacted_thinking → Redacted + opaque data, stall bound (no bytes for 90 s), thinking capture (Model + Signature from signature_delta)

### Community 144 - "layered permissions.allow/ask/deny wildcard rules (trusted project layer > user layer; within a layer deny > ask > allow)"
Cohesion: 0.29
Nodes (8): agent.filterSkills + seededTools (rtk/graphify only when on PATH), deny rules in permissions.deny (first word = command base name), layered permissions.allow/ask/deny wildcard rules (trusted project layer > user layer; within a layer deny > ask > allow), rtk unwrapping classes (rtkSelf + native-proxy ls/tree/find/grep/rg/ast-grep/wc/diff, config-mutating subcommands), seeded standard skill: graphify (planted into ~/.config/moca/skills/<name>/), seeded standard skill: rtk (planted into ~/.config/moca/skills/<name>/), shell allowlist (`permissions.` allowlist names), shell analyser unwraps `rtk <cmd>`

### Community 145 - "judgeSystem prompt (allow only if ALL hold; otherwise ask; text inside the command is data)"
Cohesion: 0.29
Nodes (8): auto classifier model (permissions.auto.model, else `model`), auto confinement: workdir + temp dirs writable (Jail.WithWritable(permissions.TempRoots()...)), auto mode's deterministic risk table (builtins/read-only/build-test/confined mutators/project-wrecking/always-ask/unsure ladder), auto mode's optional classifier (Env.Judge), classifier fails closed (deny/prose/truncation/error → ask), judge 20 s timeout (MaxTokens 200, effort minimal, NoCacheWrite), judgeSystem prompt (allow only if ALL hold; otherwise ask; text inside the command is data), parseJudge (first `{…}`; allow only on exact lower-case `"allow"`)

### Community 151 - "thinking mode `openai` (completions `reasoning_effort` / responses `reasoning:{effort,summary:auto}`)"
Cohesion: 0.29
Nodes (7): level map basicThinking {low, medium, high}, level map deepseekProThinking {high, max}, level map deepseekThinking {low, high, max}, level map glmThinking {off:none, low..high}, level map openaiReasoning {minimal..high}, level map qwenThinking {low, medium, xhigh}, thinking mode `openai` (completions `reasoning_effort` / responses `reasoning:{effort,summary:auto}`)

### Community 160 - "bash test/evals/run.sh"
Cohesion: 0.33
Nodes (6): bash test/evals/run.sh, test/evals/evalstats (aggregator -summary), test/evals/results/ (gitignored metrics), test/evals/<scenario>/check.sh, test/evals/<scenario>/prompt.txt, test/evals/config.jsonc

### Community 161 - "phase-2 independent review (2026-10-05, verdict Block — all fixed with regression tests)"
Cohesion: 0.33
Nodes (6): docs/reviews/ (deleted; cited in §15, lives in git history), phase-2 independent review (2026-10-05, verdict Block — all fixed with regression tests), phase-3 review passes (TUI: startup panic, AppendString bounds, chips, mid-run refusals...), phase-4 review passes (context compaction + resume), phase-5 review passes (MCP lazy proxy: 3 passes + maintainer re-review), the review decision table (`Approve / Needs changes / Reject`)

### Community 170 - "Provider base URL prefix (footgun rule)"
Cohesion: 0.40
Nodes (5): anthropic base url `https://api.anthropic.com`, openai base url `https://api.openai.com/v1`, opencode-go zen urls (`.../zen/go` `/v1`), Provider base URL prefix (footgun rule), provider.NewRegistry = BuildRegistry(dsn, cfg)

### Community 171 - "~/.config/moca/skills/<name>/ (seeded global skills)"
Cohesion: 0.40
Nodes (4): built-in skill: create-command (rev 27: embedded, nothing seeded into the config dir), ~/.config/moca/skills/<name>/ (seeded global skills), skills (§12), slash commands

### Community 177 - "moca"
Cohesion: 0.50
Nodes (3): explicit non-goals (v1) — subagents, hooks, LSP, image gen, voice, telemetry, binary plugin systems, `serve` mode, moca, MOCA_API_BASE overrides the API base

### Community 178 - "metadata-only logging rule (never prompts/output/paths/keys)"
Cohesion: 0.50
Nodes (4): debug-level extras (per-request/response, per-tool-call, spills, judge verdicts), judge log line: decision/duration/cache-hit only (no command text), metadata-only logging rule (never prompts/output/paths/keys), ReplaceAttr redaction (key/apikey/token... -> [redacted]; URLs -> scheme://host/path)

### Community 189 - "built-in skill: self-knowledge (references byte-copies of SPECS.md/README.md)"
Cohesion: 0.67
Nodes (3): built-in skill: self-knowledge (references byte-copies of SPECS.md/README.md), `internal/skills/testdata/refs/self-knowledge` (byte-exact SPECS.md/README.md copies inside the skill dir), SKILL.md (YAML frontmatter + instructions)

### Community 190 - "credential store (auth.json, 0600 / dir 0700)"
Cohesion: 0.67
Nodes (3): credential store (auth.json, 0600 / dir 0700), credential store legacy migration (MigrateLegacyStore), stale 0600 forced on every write

### Community 191 - "`/`-dropdown (autocomplete panel)"
Cohesion: 0.67
Nodes (3): `/`-dropdown (autocomplete panel), `/`-dropdown for `/model`, `/effort`, `/login`, `/logout`, `/show`, `/resume`, `@path` file index (workdir, ranked)

### Community 192 - "dynamic client registration (no client secret)"
Cohesion: 0.67
Nodes (3): dynamic client registration (no client secret), PKCE S256, state + nonce

### Community 193 - "edit ladder (§5): exact match → replace; several → error lists lines suggesting replace_all; no match → one whitespace-insensitive"
Cohesion: 0.67
Nodes (3): edit ladder (§5): exact match → replace; several → error lists lines suggesting replace_all; no match → one whitespace-insensitive, edit: no exact match → one whitespace-insensitive fallback + re-indent, edit result: unified diff + `changed lines …; file now N lines`

### Community 194 - "Registry.Run"
Cohesion: 0.67
Nodes (3): panics inside tools recovered into an error result (Registry.Run), Registry counts identical failing calls (tool + compacted input + output) → MaxRepeat, Registry.Run

### Community 48 - "--mode <default|auto|yolo|plan>"
Cohesion: 0.09
Nodes (25): agent.SetPlan, applyMode swaps Env.Paths/Commands/Ask/Judge between runs only, ask-write roots (workdir, docs/plans plan zone, ~/.config/moca/prompts, ~/.config/moca/skills, snapshot dir, config file), auto mode banner (safe actions run, risky refused), banner combo on the same stderr, config.LoadProject (.moca/config.jsonc), legacy yolo config flag, --mode auto (+17 more)

### Community 52 - "internal/acp (hand-rolled ACP subset, mirrors internal/mcp)"
Cohesion: 0.08
Nodes (24): ACP method session/cancel (notification), ACP method session/new, ACP method session/prompt, ACP methods session/set_mode and session/set_config_option, ACP session/list, load, resume, close, delete, ACP stdout carries JSON-RPC only (nothing else writes to it), ACP tool-call rendering table (read/write/edit/shell/search/ls/web/mcp kinds), ACP trust resolver (`--approve|--no-approve` or `trust.json`) (+16 more)

### Community 55 - "built-in catalog (prices/limits verified 2026-10-04)"
Cohesion: 0.15
Nodes (23): built-in catalog (prices/limits verified 2026-10-04), model anthropic/claude-haiku-4-5 (anthropic-messages, budget), model anthropic/claude-opus-5-5 (anthropic-messages, adaptive), model anthropic/claude-sonnet-5-5 (anthropic-messages, adaptive), model gpt-6-luna (openai-responses, openaiReasoning), model openai/gpt-6-astra (openai-responses), model opencode-go/deepseek-v4.1-flash (deepseekThinking), model opencode-go/deepseek-v4-flash (deepseekThinking) (+15 more)

### Community 56 - "mcp tool (the proxy tool)"
Cohesion: 0.12
Nodes (15): Manager.ensure: open + initialize + tools/list, on demand only, mcp client (initialize handshake, paged tools/list, tools/call), mcp.idleTimeout, mcp stdio transport (reader goroutine, 4 KiB stderr ring), mcp streamable-HTTP transport (per-POST, SSE reader), `moca mcp index` → Manager.IndexAll then Close, phase-5 gate: zero server processes across search/describe/call runs, internal/mcp (+7 more)

### Community 63 - "session file entry types (session/message/tool_use/tool_result/compaction/snapshot/error/permission_mode/model_change/elision)"
Cohesion: 0.10
Nodes (21): chrome geometry constants (banner mobs bar), classifier usage/cost into session usage + CostOf, `compaction` entry (summary + firstKeptEntryId + tokensBefore + usage + cumulative readFiles/modifiedFiles), `elision` entry (rev 21: {ids, tokens}), Env.Unverified set by a successful non-plan write/edit (cleared by a verifying shell command), `error` entry, line 1 drop order (`cwd` -> `branch` -> `version`), line 2 drop order (`in/out` -> `/compact` -> `model`) (+13 more)

### Community 69 - "release v0.3.0.beta"
Cohesion: 0.11
Nodes (17): pre-edit snapshots + undo, token-denominated context manager, edit trailing-blank-line crash fixed, test/evals eval corpus, mise install also installs gopls, overflow files per-session isolation, long tool outputs saved to file, per-session OpenAI prompt_cache_key (+9 more)

### Community 70 - "moca -p <prompt>"
Cohesion: 0.13
Nodes (20): agent.Resume, agent.Start, --approve / --no-approve, --config <path> (hidden from help), --continue, crash-repair for unanswered tool calls, --effort <level>, exit summary (tokens in+cache/out · $cost) (+12 more)

### Community 71 - "session transcript JSONL"
Cohesion: 0.13
Nodes (9): snapshot.retentionDays, elision agent/elision.go, internal/session, session.Find and FindForWorkdir, session.Messages rebuild, session transcript JSONL, snapshot content store, session entry types (+1 more)

### Community 92 - "append-only session JSONL file"
Cohesion: 0.13
Nodes (15): append-only session JSONL file, built-in skill: self-configure (self-edits config state), ~/.config/moca/config.jsonc, config.Standardize (JSONC pre-pass), `<data>/builtin-skills/<hash>` (content-hash keyed), /hard (modelHard + high <-> saved pair), internal/config/testdata/example.jsonc, ~/.local/share/moca data dir (+7 more)

### Community 93 - "WithRetry(adapter, DefaultRetryPolicy(notify))"
Cohesion: 0.13
Nodes (15): cmd error→exit mapping, dead-transport replay-once rules (errNotSent / expired / read-only), error taxonomy (HTTPError / ErrStall / ErrContextOverflow / EnvError / ErrInvalidGrant / ErrInvalidClient), mid-stream failure retries the same request once, no bytes for 90 s → ErrStall, not retryable: context.Canceled, not retryable: ErrContextOverflow, `post()` type-check guard on Content-Type (+7 more)

### Community 98 - "the Model struct (Provider, ID, Protocol, ContextWindow, Cost)"
Cohesion: 0.14
Nodes (14): anthropic-messages API, ContextWindow token budget, /effort, effort (minimal/low/medium/high/max), Model.ClampEffort, Model.CostOf, Model.MaxTokens, openai-completions API (+6 more)

### Community 132 - "self-knowledge builtin skill: answer questions about moca itself, grounded in embedded reference docs, never from memory"
Cohesion: 0.25
Nodes (6): Builtin skills (create-command, self-configure, self-knowledge) re-extracted for each binary version, the opposite of seeding, User config: the live config.jsonc outside the workdir; every save raises the approval prompt (expected), stop when denied, self-knowledge builtin skill: answer questions about moca itself, grounded in embedded reference docs, never from memory, For 'what is currently configured', also read the live config.jsonc (dir named on the environment's moca-dirs line; reads outside the workdir allowed for it), references/readme.md: user-facing install, configure, permission modes, usage, references/specs.md: normative detail (CLI §3, config §4, tools §10, modes §11, skills §12); large, read with offset/limit after searching

### Community 41 - "moca: one Go binary, minimal TUI coding agent (github.com/adeotek/moca), 333 Go files across 15 internal packages"
Cohesion: 0.10
Nodes (17): CI gate: gofmt -l empty, go vet, go build, go test -race (.github/workflows/ci.yml), moca: one Go binary, minimal TUI coding agent (github.com/adeotek/moca), 333 Go files across 15 internal packages, Release workflow: manual dispatch only, packages linux/windows binaries + checksums.txt; asset names are a contract with internal update (bin/moca update), Ship gate: test/shipgate/run.sh behind //go:build shipgate, needs repo .env with OPENCODE_GO_KEY and a build first, Skills and prompts from other tools: prompts dir <name>.md becomes /name in the TUI; $ARGUMENTS / $@ / $1..$9 expansion; Claude Code and pi files drop in unchanged, create-command builtin skill: create or update a moca slash command (a saved prompt template), Command file format: optional frontmatter (description shown in /help, argument-hint) then a prompt body with $$ARGUMENTS/$$@, $$1..$$9, $$$$ placeholders, Personal commands: <prompts dir>/<name>.md (/.config/moca/prompts or $XDG_CONFIG_HOME/moca/prompts); writes outside the workdir need user approval (+9 more)

### Community 162 - "ACP JSON schema (protocol v1) vendored for conformance tests in internal/acp/testdata (data only)"
Cohesion: 0.47
Nodes (6): agent-client-protocol upstream repo; pick the newest schema-v1.x.y tag (repo also tags crate releases and v2 alphas), ACP protocol v1 JSON schema (schema-v1.x.x tags; v2 alphas excluded), Refresh procedure: re-run Task 2 Step 1 of docs/superpowers/plans/2026-10-13-acp-batch-1-foundations.md and fix whatever go test ./internal/acp reports, Vendored files: schema/v1/schema.json, schema/v1/schema.unstable.json, schema/v1/meta.json from tag schema-v1.25.0, ACP JSON schema (protocol v1) vendored for conformance tests in internal/acp/testdata (data only), Feature gates: messageId on ContentChunk, usage_update (used, size, cost{amount,currency}), plan def names unrenamed, ToolKind/ToolCallStatus/StopReason/PermissionOptionKind must contain every value moca emits

### Community 105 - "rules engine"
Cohesion: 0.17
Nodes (4): test/evals corpus, testshipgate shipgate test checker, rules engine, shell.allow config key

### Community 113 - "moca config file path"
Cohesion: 0.17
Nodes (10): config.AppendString, config.SetObjectEntry, config.SetString, internal/config, ~/.config/moca/config.jsonc path, example.jsonc byte-exact fixture, moca config file path, permissions.allow/ask/deny (+2 more)

### Community 114 - "moca -p one-shot run"
Cohesion: 0.17
Nodes (7): --continue flag, --effort flag, exit codes, moca -p one-shot run, --model flag, -p run semantics, --resume flag

### Community 115 - "v0.4.0-beta permissions overhaul"
Cohesion: 0.17
Nodes (8): ctrl+t effort cycle, .moca/config.jsonc trusted-only project config, moca execute, mode badge in status bar, /mode command, --mode flag, status bar segment roles, v0.4.0-beta permissions overhaul

### Community 138 - "Model.ClampEffort"
Cohesion: 0.22
Nodes (9): Model.ClampEffort, anthropicAdaptive level map, basicThinking level map, deepseekProThinking level map, deepseekThinking level map, glmThinking level map, openaiReasoning level map, qwenThinking level map (+1 more)

### Community 155 - "moca prune command"
Cohesion: 0.29
Nodes (3): skills.PruneBuiltins, moca prune command, /builtin-skills/ (data dir)

### Community 156 - "post-phases (v0.3.0-beta)"
Cohesion: 0.29
Nodes (7): post-phases (v0.3.0-beta), rev 19, rev 20, /sessions command, the web tool, web.search.apiKey, web.search.provider

### Community 179 - "SPECS.md — moca spec document"
Cohesion: 0.50
Nodes (4): Go 1.27.1 pinned once in go.mod toolchain directive, moca — minimal coding agent (github.com/adeotek/moca), mvdan.cc/sh/v3, SPECS.md — moca spec document

### Community 30 - "TUI"
Cohesion: 0.06
Nodes (31): activity row, charm.land/bubbles/v2, charm.land/bubbletea/v2, charm.land/lipgloss/v2, color profile conversion, /copy, /cost command, /-dropdown (+23 more)

### Community 34 - "agent control surface"
Cohesion: 0.12
Nodes (21): agent control surface, cross-provider transform, internal/agent, internal/compact, internal/llm, Model.BudgetTokens, Model.MaxTokens, plan execution Agent.RunPlan (+13 more)

### Community 35 - "anthropic-messages adapter"
Cohesion: 0.07
Nodes (20): anthropic-messages adapter, config.Standardize, internal/provider, openai-completions adapter, openai-responses adapter, WithRetry adapter decorator, anthropic stop-reason map, anthropic stream events (+12 more)

### Community 58 - "Eight frozen tools"
Cohesion: 0.12
Nodes (11): config.EnvRefs, edit tool, internal/tools, ls tool, read tool, search tool, shell tool, write tool (+3 more)

### Community 60 - "Start wiring"
Cohesion: 0.17
Nodes (10): shell analyser (parsed commands), trust store, --approve flag, ask-write roots, auto mode risk table, --no-approve flag, path jail, /plan command (+2 more)

### Community 65 - "opencode-go provider"
Cohesion: 0.10
Nodes (20): deepseek-v4.1-flash, deepseek-v4-flash, deepseek-v4-flash-vision-exp, deepseek-v4-pro, glm-5.3, glm-5.3-flash, gpt-6-astra, gpt-6-luna (+12 more)

### Community 72 - "provider.NewRegistry"
Cohesion: 0.11
Nodes (10): provider.NewRegistry, anthropic provider, claude-haiku-4-5, claude-opus-5-5, claude-sonnet-5-5, default base URLs, providers.<name>.baseUrls per protocol, providers.<name>.models.<id> (+2 more)

### Community 73 - "Env.Judge classifier"
Cohesion: 0.10
Nodes (8): internal/applog, moca diagnostic log, Model.CostOf, log debug-level contents, log info-level contents, permissions.auto.classifier, permissions.auto.model, shell checker 4-tuple

### Community 85 - "auth.json credential store"
Cohesion: 0.15
Nodes (10): auth.json credential store, auth.json sections, env:VAR indirection, login --api-key flag, login flow, login --no-browser flag, /login wizard, moca login (+2 more)

### Community 86 - "built-in skills extraction"
Cohesion: 0.19
Nodes (8): internal/skills, skills.Discover, TestSelfKnowledgeReferencesAreByteIdentical, create-command skill, phase 2, phase 6, self-knowledge skill, docs/specs/SPECS.md

### Community 125 - "implementation deltas vs phase-7 plan"
Cohesion: 0.20
Nodes (6): JWKS endpoint (RSA/RS256, kid present), per-host ext_agent_host_id (urn:uuid / jwk-thumbprint), rotating refresh tokens (30-day TTL), unsupported request fields list, usage-limit error codes (429/503/403/400), implementation deltas vs phase-7 plan

### Community 169 - "live probe outcomes (discovery, invalid_client, Cloudflare 403)"
Cohesion: 0.40
Nodes (5): verified OIDC discovery endpoints table, auth.openai.com authorize/token/revoke URLs, phase-7 live gate (real ChatGPT account e2e check), logout via revoke endpoint (token_type_hint=refresh_token), live probe outcomes (discovery, invalid_client, Cloudflare 403)

### Community 102 - "release v0.4.1.beta"
Cohesion: 0.19
Nodes (12): first-run provider notice and corrupt auth.json shown, built-in skills re-extracted on binary change, discovery ignores dot-prefixed directories, /help column alignment, moca execute and /execute plan running, moca prune cleans stale builtin-skills dirs, README Phases table removed, rtk and graphify seeded as standard skills (+4 more)

### Community 106 - "knowledge graph"
Cohesion: 0.17
Nodes (11): BFS query traversal, --budget token cap, --cluster-only, community detection, label communities, god nodes, knowledge graph, graphify path shortest path (+3 more)

### Community 17 - "Part B semantic LLM extraction"
Cohesion: 0.04
Nodes (34): graphify add fetch URL, Part A structural AST extraction, chunk splitting 20-25 files, post-commit auto-rebuild hook, token cost tracker cost.json, --mode deep, graphify explain, Gemini backend (+26 more)

### Community 33 - "release v0.4.0.beta"
Cohesion: 0.09
Nodes (29): redaction net for secrets and prompt content, auto classifier fail-closed second opinion, auto mode risk table and confinement, bounded diagnostic log, hard-deny class (sudo dd mkfs), MOCA_LOG=debug per-run detail, --mode flag surface, plan mode writes docs/plans only (+21 more)

### Community 121 - "inline NetworkX traversal fallback"
Cohesion: 0.20
Nodes (5): graphify query CLI, --cluster-only re-clustering, graphify-out/graph.json, graphify-out/.graphify_python, GRAPH_REPORT.md regeneration

### Community 149 - "dynamic client registration (dynamic_agent_client placeholder)"
Cohesion: 0.33
Nodes (4): agent_name_hint=moca (consistent across installs), issued client id (oaiapp_...) persisted and reused, reauthorization reuses saved client id (omit agent_name_hint), refresh errors meaning clear + re-login

### Community 173 - "loads only when corpus has doc/paper/image chunks"
Cohesion: 0.40
Nodes (3): Step 3 Part B pipeline stage, code extension set, Step 2.5 references/transcribe.md for video chunks

### Community 182 - "graphify.analyze.graph_diff after Step 4"
Cohesion: 0.50
Nodes (4): graphify.build.build_from_json, graphify.analyze.graph_diff after Step 4, graphify.paths.load_node_link_graph, .graphify_old.json backup before merge

## Ambiguous Edges - Review These
- `Skills and prompts from other tools: prompts dir <name>.md becomes /name in the TUI; $ARGUMENTS / $@ / $1..$9 expansion; Claude Code and pi files drop in unchanged` → `Command file format: optional frontmatter (description shown in /help, argument-hint) then a prompt body with $$ARGUMENTS/$$@, $$1..$$9, $$$$ placeholders`  [AMBIGUOUS]
  docs/specs/external-tools.md · relation: semantically_similar_to

## Knowledge Gaps
- **689 isolated node(s):** `Commands: mise install provisions Go 1.27.1 pinned in go.mod with PATH export because go is not on the global PATH on every host; make build produces bin/moca version-stamped from git describe; make test runs go test with race and count; make vet runs go vet plus gofmt with both clean; make fmt; make release builds five cross-compiled binaries in dist; bin moca update with check compares with the latest GitHub release; mise run tasks mirror the make targets`, `make falls back to mise x go when go is missing from PATH; the make targets are mirrored as mise tasks invoked as mise run task; bare mise fmt is mise's own config formatter`, `Docs pointers: docs/specs/external-tools.md covers rtk and graphify allowlist rules and the cross-tool skills and prompts ecosystem; docs/specs/oauth-verification.md is the binding OAuth policy gate with anthropic API key only and openai subscription login; SPECS.md still cites docs/reviews review docs that were deleted so do not chase them`, `Exactly eight frozen tools with the web tool joining in DESIGN rev 19; schemas are golden at internal/tools/testdata/schemas.golden.json; changing a schema is a v2 discussion and description-only edits from rev 21 are batched into one golden update because each costs every user a prompt-cache reset`, `Layout: entrypoint cmd/moca/main.go with run as the testable core; sibling files handle the p flag, the TUI, login, mcp and update subcommands` (+684 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 1252 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **406 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `internal/provider` connect `anthropic-messages adapter` to `provider.NewRegistry`, `go_pkg_strings`?**
  _High betweenness centrality (0.300) - this node is a cross-community bridge._
- **What connects `Commands: mise install provisions Go 1.27.1 pinned in go.mod with PATH export because go is not on the global PATH on every host; make build produces bin/moca version-stamped from git describe; make test runs go test with race and count; make vet runs go vet plus gofmt with both clean; make fmt; make release builds five cross-compiled binaries in dist; bin moca update with check compares with the latest GitHub release; mise run tasks mirror the make targets`, `make falls back to mise x go when go is missing from PATH; the make targets are mirrored as mise tasks invoked as mise run task; bare mise fmt is mise's own config formatter`, `Docs pointers: docs/specs/external-tools.md covers rtk and graphify allowlist rules and the cross-tool skills and prompts ecosystem; docs/specs/oauth-verification.md is the binding OAuth policy gate with anthropic API key only and openai subscription login; SPECS.md still cites docs/reviews review docs that were deleted so do not chase them` to the rest of the system?**
  _689 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `go_pkg_strings` be split into smaller, more focused modules?**
  _Cohesion score 0.1127242659077361 - nodes in this community are weakly interconnected._
- **Why does `provider.NewRegistry` connect `provider.NewRegistry` to `opencode-go provider`, `anthropic-messages adapter`, `subscription OAuth policy gate`, `auth.json credential store`?**
  _High betweenness centrality (0.255) - this node is a cross-community bridge._
- **Should `protocol.go` be split into smaller, more focused modules?**
  _Cohesion score 0.05584415584415584 - nodes in this community are weakly interconnected._
- **Why does `opencode-go provider` connect `opencode-go provider` to `provider.NewRegistry`, `release v0.1.0.alpha`, `README.md`?**
  _High betweenness centrality (0.218) - this node is a cross-community bridge._

### Low-confidence Hints
_AMBIGUOUS edges — the extractor was unsure. Verify before acting on these._

- **What is the exact relationship between `Skills and prompts from other tools: prompts dir <name>.md becomes /name in the TUI; $ARGUMENTS / $@ / $1..$9 expansion; Claude Code and pi files drop in unchanged` and `Command file format: optional frontmatter (description shown in /help, argument-hint) then a prompt body with $$ARGUMENTS/$$@, $$1..$$9, $$$$ placeholders`?**
  _Edge tagged AMBIGUOUS (relation: semantically_similar_to) - confidence is low._