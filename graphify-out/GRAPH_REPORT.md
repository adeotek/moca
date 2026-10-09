# Graph Report - moca  (2026-10-09)

## Corpus Check
- 255 files · ~213,754 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 10 file(s) not represented in the graph (top: (none) 5, .jsonc 3, .v2 1)

## Summary
- 2795 nodes · 10590 edges · 147 communities (105 shown, 42 thin omitted)
- Extraction: 89% EXTRACTED · 11% INFERRED · 0% AMBIGUOUS · INFERRED: 1206 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `fd872c16`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- start.go
- SPECS.md — moca implemented-state normative spec
- prompt.go
- oauth_test.go
- testing.T
- run
- permissions/shell.go
- newTestModel
- charm.land/bubbletea/v2.Cmd
- cut_test.go
- Env
- update/update.go
- newAgentModel
- AppendString
- newScript
- NewManager
- moca -p "<prompt>" — one-shot agent run (stdin -p form)
- Input
- anthropic_test.go
- shell command analysis: every simple command in the tree checked
- TUI: Bubble Tea + lipgloss, compact/dense, welcome lines
- NewDefaultStore
- P2 — readability & feedback polish
- v0.1.0-alpha (2026-10-07): first release
- /-dropdown autocomplete (built-ins + prompt templates, ≤8 rows, tab/enter/esc)
- ToolSpec
- Agent
- context.Context
- NewRegistry
- opencode-go provider (zen gateway)
- RenderStatus
- v0.2.0-beta (2026-10-09): Ollama, saved / commands, self-update
- Agent
- Phase 2: seven tools + agent loop + permissions + skills loader + session schema
- Sanitize
- tools registry — Tool interface, seven frozen tools, sequential execution in emitted order
- Item
- sync.Mutex
- styleBlock
- Create
- Context manager: token-denominated budgets, compaction, recovery
- internal/agent — system prompt, events, loop, compaction + resume, cross-provider transform, Start wiring
- Manager
- model
- anthropic-messages codec (POST /v1/messages, version header, x-api-key)
- importer.go
- .startSession
- Seven frozen tools for v1 (schema churn = v2 discussion)
- credential store ~/.config/moca/auth.json (0600, atomic writes, cross-process lock)
- DESIGN §12 example config fixture (phase-1 decode gate)
- run
- DESIGN.md — v1 vision/contract (doc/specs)
- retry_test.go
- MCP lazy by design: 0 servers running at session start, ~200 prompt tokens
- moca update [--check] (newest published GitHub release, in-place swap)
- MCP gating — readOnlyHint && !destructiveHint || approve list || session allow; no name heuristics
- Store
- Config
- Resume
- Find
- net/http.Client
- .DiscoverOllama
- ollamaStart
- SOURCES.md
- Claude Code skill-creator skill
- renderMarkdown
- Architecture: package layout with strict import direction
- fakeServer
- initialize
- .Run
- fixture/main.go
- newModel
- system prompt — built once per run, stored in the session, never rebuilt
- Start
- MCP Manager — zero start, lazy ensure, busy-guarded idle stop, replay-once rules
- moca login <provider> / TUI /login: credential wizard (OAuth or masked API key)
- 2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)
- .View
- Result
- GitBranch
- input_test.go
- v0.1.0-alpha (2026-10-07)
- httpTransport
- Phase plan: 7 phases, each one reviewed PR
- Message
- Index
- CI Workflow (gofmt · vet · build · test)
- drained
- .Run
- moca/update_test.go
- decode
- quick_validate.py
- NewUnjailed
- AGENTS.md — moca
- serverEnv
- create-command.md — starter prompt template
- 1.6 Resume replay + /resume picker
- encoding/json.RawMessage
- install.sh script
- 1.1 Markdown-lite rendering of responses
- ollamaRegistry
- Claude Code frontend-design skill
- pi mcp-scripting skill
- .Run
- allowAll
- StartError
- 1.2 Inline diff preview for edit/write items
- Summarizer injection — provider-free compaction over a neutral Entry view
- AllowAll
- askRootChecker
- boundedWriter
- fakeCmds
- Never create a commit without the user's explicit approval
- Compacted
- Resumed
- SteeringApplied
- StreamReset
- TextDelta
- ThinkingDelta
- Warning
- YoloChanged
- EnvError
- 2.3 Richer activity row
- compact.Budget.Trigger() as the red threshold
- history.go / history.jsonl store
- 2.2 One-row item lines
- OpenCode — comparison reference
- Arrange-Act-Assert with xUnit/NSubstitute
- Dynamic client registration: dynamic_agent_client → issued oaiapp_ id; no client secret
- fakeSnap
- rootChecker
- Cache breakpoints: anthropic cache_control positions, ≤4 rolled forward each turn
- DESIGN revision 1: initial draft
- Parallel tool calls execute sequentially in emitted order (no cancellation of rest)
- Serialization for summarization: tool results truncated at 2K chars (COMPACTION NOTE lossy)
- 2.8 Echo !cmd like other input
- 2.5 Notify when unfocused (tui.notify: osc9/bell/off)
- Pi — comparison reference
- 2.7 Quieter first run (hints.json marker)
- 3.1 shift+tab cycles effort
- example.com/totals
- github.com/adeotek/moca
- hints.go

## God Nodes (most connected - your core abstractions)
1. `SPECS.md — moca implemented-state normative spec` - 378 edges
2. `model` - 74 edges
3. `newTestModel()` - 63 edges
4. `run()` - 52 edges
5. `newScript()` - 46 edges
6. `key()` - 44 edges
7. `newAgentModel()` - 39 edges
8. `sseServer()` - 36 edges
9. `startTestWith()` - 35 edges
10. `Config` - 35 edges

## Surprising Connections (you probably didn't know these)
- `Entry point cmd/moca/main.go; run() is the testable core; sibling subcommand files` --references--> `run()`  [INFERRED]
  AGENTS.md → cmd/moca/main.go
- `Strict import direction + structural interfaces; seven frozen tools with golden schemas` --semantically_similar_to--> `Architecture: package layout with strict import direction`  [INFERRED] [semantically similar]
  AGENTS.md → docs/specs/DESIGN.md
- `Built-in rtk skill (SKILL.md)` --semantically_similar_to--> `rtk — token-compressed CLI proxy`  [INFERRED] [semantically similar]
  internal/skills/builtin/rtk/SKILL.md → docs/external-tools.md
- `Strict config decode: unknown keys rejected; new key = defaults + validation + SPECS update` --references--> `SPECS.md — moca implemented-state normative spec`  [INFERRED]
  AGENTS.md → docs/specs/SPECS.md
- `README non-goals (v1, probably forever)` --semantically_similar_to--> `v1 explicit non-goals: subagents, hooks, plan mode, LSP, binary plugins…`  [INFERRED] [semantically similar]
  README.md → docs/specs/DESIGN.md

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **moca phase roadmap 1–7** — docs_specs_specs_phase_1, docs_specs_specs_phase_2, docs_specs_specs_phase_3, docs_specs_specs_phase_4, docs_specs_specs_phase_5, docs_specs_specs_phase_6, docs_specs_specs_phase_7 [EXTRACTED 0.85]
- **TUI polish set: planned in tui-plan, shipped in CHANGELOG** — docs_tui_plan_markdown_lite, docs_tui_plan_context_pressure_color, docs_tui_plan_resume_replay [EXTRACTED 0.90]
- **Cross-tool SKILL.md compatibility ecosystem** — docs_external_tools_skills_ecosystem, internal_skills_builtin_rtk_skill, docs_external_tools_graphify [EXTRACTED 0.95]
- **Ecosystem SKILL.md compatibility corpus (graphify, Claude Code, pi, OpenCode)** — internal_skills_testdata_ecosystem_sources, internal_skills_testdata_ecosystem, internal_skills_testdata_ecosystem_graphify_graphify_vendored, internal_skills_testdata_ecosystem_claude_code_skill_creator_vendored, internal_skills_testdata_ecosystem_claude_code_frontend_design_vendored, internal_skills_testdata_ecosystem_pi_pi_subagents_vendored, internal_skills_testdata_ecosystem_pi_mcp_scripting_vendored, internal_skills_testdata_ecosystem_opencode_dotnet_unit_testing_vendored [EXTRACTED 1.00]
- **Seven frozen tools + golden schemas + registry** — docs_specs_specs_tool_read, docs_specs_specs_tool_write, docs_specs_specs_tool_edit, docs_specs_specs_tool_shell, docs_specs_specs_tool_search, docs_specs_specs_tool_ls, docs_specs_specs_tool_mcp, docs_specs_specs_tools_registry, docs_specs_specs_frozen_schemas [EXTRACTED 1.00]
- **Lazy MCP proxy mechanism** — d_mcp_tool, d_mcp_lazy_design, d_mcp_lazy_lifecycle, d_mcp_discovery_index, d_mcp_server_roster, d_mcp_call_gating, d_mcp_env_hygiene [EXTRACTED 1.00]
- **MCP lazy proxy lifecycle (transport → index → manager → gating)** — docs_specs_specs_mcp_manager, docs_specs_specs_mcp_index, docs_specs_specs_mcp_stdio_transport, docs_specs_specs_mcp_http_transport, docs_specs_specs_mcp_jsonrpc, docs_specs_specs_mcp_gating [EXTRACTED 1.00]
- **rtk allowlist laundering prevention flow** — docs_external_tools_rtk_unwrap, docs_external_tools_internal_permissions_rtk_go, internal_skills_builtin_rtk_skill_prefer_compressed [EXTRACTED 1.00]
- **The seven frozen v1 tools (schemas golden)** — d_read_tool, d_write_tool, d_edit_tool, d_shell_tool, d_search_tool, d_ls_tool, d_mcp_tool [EXTRACTED 1.00]
- **Synthetic frontmatter edge-case fixtures (CRLF, BOM, folding, quoting, rich keys, long description)** — internal_skills_testdata_ecosystem_synthetic_bom_skill, internal_skills_testdata_ecosystem_synthetic_colon_unquoted_skill, internal_skills_testdata_ecosystem_synthetic_crlf_skill, internal_skills_testdata_ecosystem_synthetic_folded_skill, internal_skills_testdata_ecosystem_synthetic_literal_skill, internal_skills_testdata_ecosystem_synthetic_long_description_skill, internal_skills_testdata_ecosystem_synthetic_rich_keys_skill [EXTRACTED 1.00]
- **Human-in-the-loop validation before declaring done** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_claude_code_frontend_design_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_review_validation [INFERRED 0.75]
- **Ship-gate edit→test→commit loop** — d_ship_gate, d_read_tool, d_search_tool, d_edit_tool, d_shell_tool, d_rtk_tool, c_investigation_protocol [INFERRED 0.85]
- **Parallel subagent fanout with parent synthesis** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_graphify_graphify_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_skill [INFERRED 0.85]
- **Skills referencing bundled reference files for progressive disclosure** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_graphify_graphify_skill, internal_skills_testdata_ecosystem_pi_mcp_scripting_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_skill [INFERRED 0.85]

## Communities (147 total, 42 thin omitted)

### Community 0 - "start.go"
Cohesion: 0.12
Nodes (20): request, usageError, anthropicEvent, anthropicUsage, completionsChunk, Credential, ollamaTags, responsesEvent (+12 more)

### Community 1 - "SPECS.md — moca implemented-state normative spec"
Cohesion: 0.03
Nodes (104): cmd/moca/main.go (entry, signals, testable run()), cmd/moca/tui.go (TUI path + trust), SPECS.md — moca implemented-state normative spec, agent events — TextDelta … TurnEnd/SteeringApplied/Compacted/Resumed, agent loop — sequential calls, steering drains, length-turn handling, exit on no-tool answer, charm.land/bubbletea/v2 (TUI framework, with bubbles + lipgloss), /copy — OSC 52 copy of last assistant text, /cost — in/out/cache breakdown + $cost (+96 more)

### Community 2 - "prompt.go"
Cohesion: 0.06
Nodes (56): PromptInput, ServerLine, BuildSystemPrompt(), filterSkills(), oneLine(), TestBuildSystemPrompt(), TestFilterSkillsDropsMissingBuiltins(), TestPromptRTKIsConditional() (+48 more)

### Community 3 - "oauth_test.go"
Cohesion: 0.08
Nodes (35): audContains(), b64d(), jwksKey(), pkce(), randText(), browse(), firstURL(), newFakeAS() (+27 more)

### Community 4 - "testing.T"
Cohesion: 0.04
Nodes (82): NormalizeToolID(), TestNormalizeToolID(), TestTransformDropsEmptiedAssistantAndMerges(), TestTransformDropsRedactedForeignThinking(), TestTransformHistory(), TestTransformSameBareIDDifferentProvider(), TransformHistory(), CapChars() (+74 more)

### Community 5 - "run"
Cohesion: 0.08
Nodes (51): fakeCompletions(), isolate(), TestExitCodes(), TestHelpListsSurface(), TestInterruptedExit130(), TestLoginLogoutCLI(), TestMCPImportCLI(), TestNoApproveSkipsProjectInstructions() (+43 more)

### Community 6 - "permissions/shell.go"
Cohesion: 0.06
Nodes (42): setup, canonical(), Jail, NewJail(), resolveDeep(), setup(), TestJail(), TestJailApprovable() (+34 more)

### Community 7 - "newTestModel"
Cohesion: 0.06
Nodes (56): TestActivityLine(), TestCoalescedShrinkRemembersTheLastFlushedRow(), TestInputBoxGrowsAndShrinks(), TestPagerCloseDefersTheRelease(), TestRefuseRunningWhileOutputInFlight(), TestShrinkIsPaddedUntilTheGuardWindowPasses(), TestShrinkPaddingFitsTheScreen(), TestSpinnerTicksOnlyForTheLiveRun() (+48 more)

### Community 8 - "charm.land/bubbletea/v2.Cmd"
Cohesion: 0.11
Nodes (8): ShellOutput, model, TestWelcomeLines(), welcomeText(), AbbrevEffort(), compactDoneMsg, loginDoneMsg, shellDoneMsg

### Community 9 - "cut_test.go"
Cohesion: 0.10
Nodes (31): Kind, Agent, summarizerUnavailable(), Budget, NewBudget(), TestBudget(), FindCut(), a() (+23 more)

### Community 10 - "Env"
Cohesion: 0.11
Nodes (12): TestInvestigationHint(), askOutsideWrite(), CommandChecker, Env, PathChecker, investigationHint(), countFileLines(), countLines() (+4 more)

### Community 11 - "update/update.go"
Cohesion: 0.08
Nodes (35): envOr(), githubToken(), runUpdate(), Compare(), comparePre(), ExtractBinary(), FindAsset(), FindChecksums() (+27 more)

### Community 12 - "newAgentModel"
Cohesion: 0.07
Nodes (54): keyMsg(), TestExitCommandQuits(), TestCompactEscapeCancels(), TestCompactingBlocksRun(), TestCompactNothing(), TestEditItemShowsDiff(), contains(), TestAllowAlwaysRequiresCtrl() (+46 more)

### Community 13 - "AppendString"
Cohesion: 0.13
Nodes (22): scanner, span, AppendString(), concat(), nested(), nestedEntry(), objectHasKey(), objectValueSpan() (+14 more)

### Community 14 - "newScript"
Cohesion: 0.12
Nodes (48): newScript(), startTest(), startTestWith(), startWith(), TestAbortWritesSyntheticResults(), TestFailedCallDoesNotCancelRest(), TestLengthStopTruncatedToolCall(), TestLoopReadEditVerify() (+40 more)

### Community 15 - "NewManager"
Cohesion: 0.16
Nodes (27): crashingServer(), shServer(), skipWindows(), TestCallTimeoutStopsWedgedServer(), TestHandshakeTimeout(), TestHTTPAbortedStreamReplayRule(), TestIndexSaveMergesConcurrentWriters(), TestSearchReportsAndRemembersFailedServer() (+19 more)

### Community 16 - "moca -p "<prompt>" — one-shot agent run (stdin -p form)"
Cohesion: 0.09
Nodes (29): cmd/moca/cli.go (flags → Options), cmd/moca/oneshot.go (-p agent path), Agent.Close — stops live MCP servers and closes the session writer, agent.Resume — validate first, repair, restore model/effort, jail at header workdir, /clear — new session file, old stays resumable, resets yolo to startup flag, /exit //q ///quit — quit (cancels in-flight runs), /resume [id8] — picker over this directory’s sessions, /sessions — session manager (delete via lock, open refuses deletion) (+21 more)

### Community 17 - "Input"
Cohesion: 0.11
Nodes (7): stampOf(), lineCount(), ReadTracker, stamp, Input, padGuard, paste

### Community 18 - "anthropic_test.go"
Cohesion: 0.17
Nodes (35): Adapter, newAnthropic(), keyCred(), mustJSON(), sseServer(), TestAnthropicBodyHygiene(), TestAnthropicBudgetAndNoCache(), TestAnthropicBudgetFollowsEffort() (+27 more)

### Community 19 - "shell command analysis: every simple command in the tree checked"
Cohesion: 0.09
Nodes (27): graphify workflow rules: query/path/explain first, update . after code changes, Failing-test investigation protocol: edit refuses until read+search happened, Built-in rtk skill (embed.FS, extracted to ~/.local/share/moca/builtin-skills/<version>), graphify: codebase-graph CLI (default allowlist; SKILL.md drop-in), MCP server roster: one line per configured server in the system prompt, mvdan.cc/sh/v3/syntax — the single non-UI third-party dependency, v1 explicit non-goals: subagents, hooks, plan mode, LSP, binary plugins…, Phase 6: rtk + graphify + skills ecosystem compatibility (+19 more)

### Community 20 - "TUI: Bubble Tea + lipgloss, compact/dense, welcome lines"
Cohesion: 0.15
Nodes (21): Approvals show the whole command (≤6 lines) + ctrl+o pages all of it, Persistent prompt history (history.jsonl per workdir) + ctrl+r search, Markdown-lite response rendering (bold/code/headings/bullets/fenced blocks), Context pressure: % yellow at 70%, red where auto-compaction fires, Bracketed paste: verbatim, never auto-sent, [paste N lines] chip (alt+p), /copy: last assistant message to clipboard via OSC 52 (SSH-safe), Pager: ctrl+o (latest item) / /show <n>; alt+t for thinking, permission_mode audit entry {yolo}; yolo never restored on resume (+13 more)

### Community 21 - "NewDefaultStore"
Cohesion: 0.12
Nodes (13): NewDefaultStore(), Headless(), OAuthProvider(), TestHeadlessDetectionSSH(), model, providerNames(), providerRow(), TestLoginRowForOllama() (+5 more)

### Community 22 - "P2 — readability & feedback polish"
Cohesion: 0.06
Nodes (29): 0. Fixed in this pass (already on `fix/polish`), 1.1 Markdown-lite rendering of responses — **M**, 1.2 Inline diff preview for `edit`/`write` items — **S/M**, 1.3 Context-pressure color in the status bar — **S**, 1.4 Argument completion in the `/` dropdown + interactive `/model` — **M**, 1.5 `@path` file mentions — **M**, 1.6 Resume shows where you left off; `/resume` picker — **M**, 1.7 Persistent input history + `ctrl+r` search — **S/M** (+21 more)

### Community 23 - "v0.1.0-alpha (2026-10-07): first release"
Cohesion: 0.13
Nodes (24): Default HTTP client bounds response-header wait at 120s, opencode-go [1210] server_error flake retried even on HTTP 400, v0.1.0-alpha (2026-10-07): first release, anthropic-messages protocol adapter (~150 LOC, hand-rolled HTTP+SSE), anthropic provider (api_key only; subscription OAuth not permitted), Mid-stream failure: no silent retry, discard partial, one retry from same request, openai-completions protocol adapter, openai provider (API key or ChatGPT subscription OAuth) (+16 more)

### Community 24 - "/-dropdown autocomplete (built-ins + prompt templates, ≤8 rows, tab/enter/esc)"
Cohesion: 0.07
Nodes (27): activity row ⠋ working… (spinner, elapsed, esc to interrupt), Agent.Status() — model, effort, anchored tokens, window, usage, cost, Sub/Hard/Yolo, @path completion (≤5000 indexed files, ranked, text sent as written), /effort [level] (also shift+tab wrap; refused while running), /hard — modelHard + high ↔ saved pair, /show <n> — open any item in the pager, usage-anchored context estimate — anchor + delta after each turn, agent control surface — SetModel/SetEffort/ToggleHard/Models/Status/Steer/AddNote/Undo (mutex-guarded) (+19 more)

### Community 25 - "ToolSpec"
Cohesion: 0.14
Nodes (11): ToolSpec, TestProxySpecFrozen(), Builtins(), MCPSpec(), Registry, NewRegistry(), TestRegistryDispatch(), TestSchemasFrozen() (+3 more)

### Community 26 - "Agent"
Cohesion: 0.13
Nodes (4): userText(), Agent, mustJSONString(), TestResumeHonoursCompaction()

### Community 27 - "context.Context"
Cohesion: 0.08
Nodes (34): ToolEnd, ToolStart, ContentBlock, Event, Request, Response, StopReason, ToolCall (+26 more)

### Community 28 - "NewRegistry"
Cohesion: 0.19
Nodes (20): TestNoOllamaNoProbe(), TestOllamaVerify(), NewRegistry(), mustCfg(), TestAPIKeyMissingMentionsLogin(), TestAPIKeyStoreWinsOverEnv(), TestBaseURLPrecedence(), TestBuiltinModelNeedsProtocol() (+12 more)

### Community 29 - "opencode-go provider (zen gateway)"
Cohesion: 0.08
Nodes (37): opencode.ai Go-tier docs (MissingSessionID, /zen/v1 exemption), level map anthropicAdaptive {low,medium,high,max} (off/minimal clamp up), level map basicThinking {low,medium,high} (gateway rejects off/minimal: kimi-k2.7-code, mimo-v2.6-*), opencode-go deepseek-v4.1-flash (openai-completions, 1M ctx), opencode-go deepseek-v4-flash (openai-completions, 1M ctx), opencode-go deepseek-v4-flash-vision-exp (vision experiment), opencode-go deepseek-v4-pro (off-peak rate, 1M ctx), level map deepseekProThinking {high,max} (+29 more)

### Community 30 - "RenderStatus"
Cohesion: 0.12
Nodes (18): TestStatusWideRunesNeverExceedWidth(), AbbrevHome(), FmtTokens(), FmtWindow(), TestFormatters(), trimZero(), TestStatusLineCarriesTheSetupState(), FmtPercent() (+10 more)

### Community 31 - "v0.2.0-beta (2026-10-09): Ollama, saved / commands, self-update"
Cohesion: 0.14
Nodes (25): Release workflow asset contract with internal/update; install.sh/install.ps1 consumers, Ollama fail-fast: server down says how to start; unpulled model says ollama pull, --resume/--continue replay last 3 turns; new /resume picker, v0.2.0-beta (2026-10-09): Ollama, saved / commands, self-update, Built-in & user-extensible model catalog (window, max out, cost, thinkingMode), /clear starts a new session file (old one stays resumable), Provider-qualified model ids (opencode-go/glm-5.3-flash), Resume: moca --resume <id8|last> / --continue (workdir→session mapping) (+17 more)

### Community 32 - "Agent"
Cohesion: 0.09
Nodes (16): Options, savedModel, Status, Result, Agent, Outcome, New(), Effort (+8 more)

### Community 33 - "Phase 2: seven tools + agent loop + permissions + skills loader + session schema"
Cohesion: 0.14
Nodes (19): /sessions manager: list/switch/ctrl+d delete; single-writer delete refusal, JSONL sessions: single-writer flock, 0600, ! / !! shell prefixes: user-initiated, bypass model-facing analysis, Per-provider baseUrl override + custom providers (vLLM/LM Studio/Ollama day 1), Edit-ladder tests (phase 2, non-negotiable: exact/ambiguous/replace_all/CRLF/BOM…), JSONL append-only session transcripts (0600 file, 0700 dir, id+parentId chain), Path jail anchored at session-start workdir (symlinks resolved, EvalSymlinks), Phase 2: seven tools + agent loop + permissions + skills loader + session schema (+11 more)

### Community 34 - "Sanitize"
Cohesion: 0.10
Nodes (18): AutoAllow(), Answer, Question, approvalPanel(), approvalPrompt(), TestApprovalPromptSanitized(), diffPreview(), TestDiffPreview() (+10 more)

### Community 35 - "tools registry — Tool interface, seven frozen tools, sequential execution in emitted order"
Cohesion: 0.10
Nodes (22): Ship gate: make build && bash test/shipgate/run.sh (//go:build shipgate tag), Jail.Approvable — resolves refused writes to the ask-write root, ask-write root ~/.config/moca/prompts (write/edit ask, never persistent), !cmd / !!cmd user-initiated shell (30 s, ShellEnv, 30K truncation; esc kills), config.EnvRefs — every referenced variable (shell hygiene input), edit ladder — exact match → replace_all → whitespace-insensitive re-indent → error, frozen tool schemas — golden schemas.golden.json (v2 discussion to change), inline edit diffs — changed lines indented, ≤8 rows then N more (+14 more)

### Community 36 - "Item"
Cohesion: 0.15
Nodes (7): TestMatchRanges(), matchRanges(), newPager(), TestTeaAPI(), Item, Items, pagerModel

### Community 37 - "sync.Mutex"
Cohesion: 0.16
Nodes (8): startStdio(), TestStdioWriteEPIPE(), KillProcessGroup(), SetProcessGroup(), KillProcessGroup(), SetProcessGroup(), ring, stdioTransport

### Community 38 - "styleBlock"
Cohesion: 0.31
Nodes (10): respMsgStyle(), styleBlock(), styleBlockLead(), TestCommandEchoesAsUserMessage(), TestMessageBackgroundsCoverTheFullRow(), TestResponseCommitUsesBackground(), TestScrollbackMessageBackgrounds(), TestStyleBlockExpandsTabs() (+2 more)

### Community 39 - "Create"
Cohesion: 0.11
Nodes (18): lockFile(), lockFile(), gitClean(), Snapshots, NewSnapshots(), Create(), Writer, lastNewline() (+10 more)

### Community 40 - "Context manager: token-denominated budgets, compaction, recovery"
Cohesion: 0.20
Nodes (16): compaction entry (summary, firstKeptEntryId, tokensBefore, usage), Compaction loop guard: stop with error if still above trigger, Context manager: token-denominated budgets, compaction, recovery, -p exit codes: 0 completed · 1 runtime · 2 config/usage · 3 maxSteps · 130 interrupted, keepRecentTokens (default 20K kept verbatim), maxSteps (default 40): max agentic iterations per run, maxSteps wrap-up request with tool_choice: none (never removes tools), Overflow recovery: one compact-and-retry attempt (plain length stop ≠ overflow) (+8 more)

### Community 41 - "internal/agent — system prompt, events, loop, compaction + resume, cross-provider transform, Start wiring"
Cohesion: 0.15
Nodes (19): agent build() — registry, jail, checkers, env, snapshots, MCP manager, agent (shared with Resume), /compact — manual compaction on cancellable context (compacting… transient), cmd/moca entrypoint package, compact.Budget — window/reserve/keepRecent, small windows scale to window/4, context pressure coloring — yellow 70%, red at trigger from compact.Budget.Trigger(), internal/agent — system prompt, events, loop, compaction + resume, cross-provider transform, Start wiring, internal/compact — budgets, cut points, lossy serialization, structured summaries (no provider import), internal/config — paths, JSONC pre-pass, typed config + validation, env indirection (+11 more)

### Community 42 - "Manager"
Cohesion: 0.25
Nodes (5): ConfigHash(), TestIndexPersistAndHash(), Manager, Options, state

### Community 43 - "model"
Cohesion: 0.16
Nodes (12): firstLineOf(), clampRunes(), model, matchCommands(), TestClampRunes(), mentionMatches(), rankFile(), TestMentionRanking() (+4 more)

### Community 44 - "anthropic-messages codec (POST /v1/messages, version header, x-api-key)"
Cohesion: 0.12
Nodes (20): anthropic stream flow (message_start/content_block_*/message_delta/stop), anthropic cache breakpoints — system end + last tool + last eligible block, openai-completions thinking — reasoning_content; history thinking never sent, cross-provider transform — thinking replay bound by Model == request model, ErrContextOverflow (prompt too long / context_length_exceeded), subscription wire use — bearer, store:false, no max_output_tokens, tools in one namespace group, overflow recovery — errOverflow (nothing persisted), one compact-and-retry, then surface, anthropic-messages codec (POST /v1/messages, version header, x-api-key) (+12 more)

### Community 45 - "importer.go"
Cohesion: 0.16
Nodes (20): MCPServer, TestImportRewritesEmbeddedSecretsInValues(), TestRewriteSecretsValueRuleAlone(), DiscoverSources(), embeddedSecret(), envName(), literalSecretFields(), ParseSource() (+12 more)

### Community 46 - ".startSession"
Cohesion: 0.15
Nodes (8): modelPickItems(), TestWindowRange(), model, windowRange(), model, noProviderNotice(), pickItem, pickState

### Community 47 - "Seven frozen tools for v1 (schema churn = v2 discussion)"
Cohesion: 0.18
Nodes (10): Seven frozen tools; schemas golden in internal/tools/testdata/schemas.golden.json, Inline edit diffs: changed lines (red/green, ≤8) under the item line, edit tool: path + old_string/new_string + replace_all, ls tool: one level, dirs suffixed /, hidden opt-in, Read-before-write + unchanged-on-disk guard (mtime + size + hash), search tool: pure-Go ripgrep semantics, .gitignore/.ignore aware, 200-hit cap, Seven frozen tools for v1 (schema churn = v2 discussion), Edit whitespace-tolerant fallback with re-indentation (indentation delta) (+2 more)

### Community 48 - "credential store ~/.config/moca/auth.json (0600, atomic writes, cross-process lock)"
Cohesion: 0.07
Nodes (31): agent.StartError (run setup failure → exit 1), credential store ~/.config/moca/auth.json (0600, atomic writes, cross-process lock), required OAuth scope chatgpt.tokens.use.direct, Registry.CheckCredential — network-free login check (/model, resume), /model [id] — cross-provider switch (refused while running, forfeits prompt cache), config.EnvError (unset env:VAR, first-request, exit 2), config.SetObjectEntry — insert key: rawJSON (mcp import), config.SetString — replace a string in place, last-duplicate wins, non-string refused (+23 more)

### Community 49 - "DESIGN §12 example config fixture (phase-1 decode gate)"
Cohesion: 0.14
Nodes (22): config.Standardize preserves byte offsets; example.jsonc is byte-exact DESIGN §12 fixture, Secrets: apiKey must be env:VAR; .env and *.key gitignored, never committed, Strict config decode: unknown keys rejected; new key = defaults + validation + SPECS update, Thinking blocks/signatures model-bound; cross-provider transforms in agent (SPECS §14), Argument completion for /model, /effort, /login, /logout, /show, /resume, api_key auth mode (stored credential or env: reference), DESIGN §12 example config fixture (phase-1 decode gate), Config: ~/.config/moca/config.jsonc, typed struct, fail-fast validation (+14 more)

### Community 50 - "run"
Cohesion: 0.09
Nodes (31): Entry point cmd/moca/main.go; run() is the testable core; sibling subcommand files, parseArgs(), printUsage(), TestExitForOAuthSentinels(), TestParseArgs(), loginAPIKey(), runLogin(), runLogout() (+23 more)

### Community 51 - "DESIGN.md — v1 vision/contract (doc/specs)"
Cohesion: 0.32
Nodes (4): moca — MO Coding Agent (binary = repo = module path), DESIGN.md — v1 vision/contract (doc/specs), moca README — project overview, Status: v0.2.0-beta (self-update, saved / commands, Ollama)

### Community 52 - "retry_test.go"
Cohesion: 0.16
Nodes (23): Retry, newHTTPError(), retryAfter(), DefaultRetryPolicy(), RetryNotice, fail(), TestMidStreamFailureResetsAndRetriesOnce(), TestMidStreamRetryBacksOffAndNotifies() (+15 more)

### Community 53 - "MCP lazy by design: 0 servers running at session start, ~200 prompt tokens"
Cohesion: 0.31
Nodes (13): MCP call gating: default-deny via readOnlyHint annotations / per-server approve lists, mcp config block: idleTimeout scalar beside mcp.servers map + approve lists, Persisted MCP discovery index (~/.local/share/moca/mcp-index.json, configHash-invalidated), MCP env hygiene: filtered subprocess env (PATH/HOME/USER/LANG/TERM/TMPDIR/XDG_*), MCP HTTP headers with env: indirection (never literal values), moca mcp import (Claude Code/OpenCode/Pi configs; secrets rewritten to env:) + mcp index, MCP lazy lifecycle: start on first describe/call, stop after mcp.idleTimeout (600s), Path jail does NOT apply to MCP tools (server tool defines its own scope) (+5 more)

### Community 54 - "moca update [--check] (newest published GitHub release, in-place swap)"
Cohesion: 0.15
Nodes (17): cmd/moca/update.go (moca update subcommand), checksums.txt sha256 verification (missing warns; mismatch refuses), moca update --check flag (report only, never writes), git describe version stamp (-ldflags -X config.Version), install-script gate — fresh/update/mismatch refusals for install.sh + functional pwsh run, internal/update — GitHub release fetch, semver compare, checksum verify, atomic replace, make build (bin/moca, version-stamped from git describe), make release (dist/ cross-compiled, darwin included) (+9 more)

### Community 55 - "MCP gating — readOnlyHint && !destructiveHint || approve list || session allow; no name heuristics"
Cohesion: 0.12
Nodes (17): TUI approval prompt (allow <cmd>? a once / ctrl+a always / d deny; 700 ms typing pause; esc denies), config.AppendString — comment-preserving array append (allow lists), 32-entry shell.allow default list (DESIGN §12), hard-deny class — sudo/doas/dd/shred/chown/mkfs* (+ su), never overridable, config key: mcp.servers.<name>.approve (allow list, "*" trusts the server), MCP gating — readOnlyHint && !destructiveHint || approve list || session allow; no name heuristics, mvdan.cc/sh/v3 (only non-UI dependency, shell parsing), phase-5 live legs — real model drives mcp search/describe/call + gating (+9 more)

### Community 56 - "Store"
Cohesion: 0.28
Nodes (5): Store, newHostID(), validateAPIKey(), Refresher, storeFile

### Community 57 - "Config"
Cohesion: 0.08
Nodes (22): ContextConfig, MCPConfig, ModelOverride, ShellConfig, SnapshotConfig, TUIConfig, discoverLocal(), Default() (+14 more)

### Community 58 - "Resume"
Cohesion: 0.22
Nodes (10): scriptServer, Resume(), catalogConfig(), lastModelChange(), startCatalog(), TestResumeEffortFlagOverridesStored(), TestResumeNotASession(), TestResumeRestoresHardMode() (+2 more)

### Community 59 - "Find"
Cohesion: 0.10
Nodes (27): Delete(), TestDelete(), TestListAll(), canonical(), Find(), FindForWorkdir(), Info, header() (+19 more)

### Community 60 - "net/http.Client"
Cohesion: 0.33
Nodes (8): fakeSIWC(), fakeSIWCOpts(), OAuthConfig, Login(), missingScopes(), Revoke(), tokenFrom(), Token

### Community 61 - ".DiscoverOllama"
Cohesion: 0.12
Nodes (11): Registry, modelFromShow(), normalizeOllamaURL(), ollamaBaseURL(), ollamaRoot(), TestNormalizeOllamaURL(), TestOllamaBaseURLPrecedence(), countingTransport (+3 more)

### Community 62 - "ollamaStart"
Cohesion: 0.25
Nodes (8): ollamaServer(), ollamaStart(), TestOllamaDeclaredWindowIsQuiet(), TestOllamaReportedWindowIsQuiet(), TestOllamaStartErrors(), TestOllamaTinyWindowWarns(), TestStartWithOllamaModel(), abortingHTTPServer()

### Community 63 - "SOURCES.md"
Cohesion: 0.19
Nodes (7): internal/skills/testdata/ecosystem corpus, anthropics/skills frontend-design, anthropics/skills skill-creator, graphify skill (graphify install, claude platform; byte-identical to pi variant), adeotek/dotfiles opencode dotnet-unit-testing skill, npm pi-mcp-adapter mcp-scripting skill, npm pi-subagents skill

### Community 64 - "Claude Code skill-creator skill"
Cohesion: 0.16
Nodes (13): Skill description (trigger) optimization, Skill eval/iterate loop (with-skill vs baseline), skill-creator Apache 2.0 License (Anthropic), Claude Code skill-creator skill, SKILL.md anatomy + bundled resources layout, Constrained query expansion against graph vocab, Progressive Disclosure (three-level skill loading), graphify query/path/explain reference (+5 more)

### Community 65 - "renderMarkdown"
Cohesion: 0.15
Nodes (16): Event, agentEvent(), codeBandStyle(), fenceLang(), renderInline(), renderMarkdown(), plainMD(), TestFenceSurvivesCommitLive() (+8 more)

### Community 66 - "Architecture: package layout with strict import direction"
Cohesion: 0.21
Nodes (12): Strict import direction + structural interfaces; seven frozen tools with golden schemas, internal/agent: the agent loop (user msg → model → tools → results), internal/compact: token accounting, truncation, summarization, cache markers, internal/config: single JSONC config, typed, validated, internal/llm: leaf Message/ContentBlock/ToolCall/Usage/Request types, internal/mcp: lazy MCP proxy (stdio + streamable HTTP, discovery index, lifecycle), internal/permissions: path jail, shell command analysis + allowlist, project trust, internal/provider: protocol adapters, provider configs, model catalog, auth, retry (+4 more)

### Community 67 - "fakeServer"
Cohesion: 0.26
Nodes (12): TestListToolsRepeatedCursor(), dial(), fakeServer(), TestStdioListAndCall(), TestStdioPaged(), NewTool(), newProxy(), runP() (+4 more)

### Community 68 - "initialize"
Cohesion: 0.20
Nodes (9): initialize(), TestHTTPCloseBounded(), startHTTP(), fakeHTTPServer(), TestHTTPNotifyStatusChecked(), TestHTTPTransport(), TestStdioEarlyExit(), headerLog (+1 more)

### Community 69 - ".Run"
Cohesion: 0.15
Nodes (17): applyEdit(), formatDiff(), indentOf(), joinInts(), lineOf(), matchLines(), replaceExact(), replaceFuzzy() (+9 more)

### Community 70 - "fixture/main.go"
Cohesion: 0.21
Nodes (7): Sum(), TestTotals(), ParseList(), Average(), Max(), TestMaxAndAverage(), main()

### Community 71 - "newModel"
Cohesion: 0.13
Nodes (16): Asker, newModel(), Run(), newAsker(), newEventPipe(), TestEventPipeBuffersUntilStarted(), TestEventPipeNonBlockingAndFIFO(), TestPipeSendKeepsOrderWithEvents() (+8 more)

### Community 72 - "system prompt — built once per run, stored in the session, never rebuilt"
Cohesion: 0.17
Nodes (13): AGENTS.md — project instructions file, builtin skills — embed, ExtractBuiltins into content-hash dirs (currently rtk), CLAUDE.md fallback instructions file, vendored ecosystem corpus internal/skills/testdata/ecosystem (pi/claude/opencode/graphify skills), ExtractBuiltins(dataDir) — temp dir + rename, race-safe, agent.filterSkills — omit builtins whose CLI is missing from PATH, LoadInstructions — global AGENTS.md, workdir AGENTS.md (trusted) else CLAUDE.md, 32K cap, phase-6 live legs — sharpened rtk description drives rtk test, graphify answer (+5 more)

### Community 73 - "Start"
Cohesion: 0.14
Nodes (21): TestDefaultHTTPClientBoundsHeaderWait(), GitState(), Platform(), build(), defaultHTTPClient(), StartOptions, prepare(), Start() (+13 more)

### Community 74 - "MCP Manager — zero start, lazy ensure, busy-guarded idle stop, replay-once rules"
Cohesion: 0.15
Nodes (12): config key: mcp.servers.<name> (command|url, args, env, headers), errNotSent — request never left: replay-safe, errTransportDead sentinel (errors.Is-visible), MCP streamable HTTP transport — Mcp-Session-Id echo, SSE responses, 15m client, persisted MCP index mcp-index.json (configHash, no schemas, merged saves), JSON-RPC 2.0 core — id routing, ping, paged tools/list, transports interface, MCP Manager — zero start, lazy ensure, busy-guarded idle stop, replay-once rules, MCP stdio transport — line-wise, 4KiB ring, exit drain ≤500ms, dead-sentinel errors (+4 more)

### Community 75 - "moca login <provider> / TUI /login: credential wizard (OAuth or masked API key)"
Cohesion: 0.25
Nodes (15): auth.json moved to config dir; v0.1 store auto-migrated, +10 opencode-go catalog models (deepseek v4.1, kimi-k2.7-code, mimo-v2.6, qwen3.x…), v0.1.1 (2026-10-08): polish + credential store + autocomplete, moca CLI surface: TUI, -p, login/logout, mcp import/index, --version (§12.5), moca login <provider> / TUI /login: credential wizard (OAuth or masked API key), moca logout <provider> / TUI /logout: clear a stored credential, oauth auth mode (moca login; auth.json 0600, auto-refresh), moca -p one-shot mode (prompt also via stdin, -p -) (+7 more)

### Community 76 - "2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)"
Cohesion: 0.17
Nodes (12): 1.1 Terms, 1.2 Client registration, 1.3 Decision, 1. anthropic — decision: `api_key only` (subscription OAuth not permitted), 2.1 Terms, 2.2 Client registration (per the docs), 2.3 Endpoints (verified live 2026-10-06 against `https://auth.openai.com/.well-known/openid-configuration`), 2.4 Inference contract for subscription traffic (open-source flow docs) (+4 more)

### Community 78 - "Result"
Cohesion: 0.31
Nodes (6): cutRunes(), errResult(), isTrue(), TestCutRunesSafe(), Result, ProxyTool

### Community 79 - "GitBranch"
Cohesion: 0.40
Nodes (5): TestGitStatusUsesNoOptionalLocks(), GitBranch(), gitStatusArgs(), TestGitBranch(), TestGitBranchUnborn()

### Community 80 - "input_test.go"
Cohesion: 0.30
Nodes (11): NewInput(), TestCollapseUsesExactOffset(), TestCtrlC(), TestEditedMarkerSentLiterally(), TestHistory(), TestHistoryRecallChipsUnsafeContent(), TestMarkerNeverCollidesWithTypedText(), TestPasteChipKeepsContent() (+3 more)

### Community 81 - "v0.1.0-alpha (2026-10-07)"
Cohesion: 0.18
Nodes (10): CHANGELOG, CLI, Context & sessions, External tools & the skills ecosystem, MCP, OAuth (subscription login), Providers & protocols, Tools & permissions (+2 more)

### Community 82 - "httpTransport"
Cohesion: 0.25
Nodes (5): fakeSum, responseID(), TestResponseID(), readEvents(), httpTransport

### Community 83 - "Phase plan: 7 phases, each one reviewed PR"
Cohesion: 0.15
Nodes (11): Docs map: external-tools.md, oauth-verification.md; deleted docs/plans & docs/reviews, docs/plans/phase-<n>-*.md task-level plans (deleted; cited as history), Phase plan: 7 phases, each one reviewed PR, External tools: rtk, graphify, skills ecosystem, graphify — codebase knowledge graph integration, internal/permissions/rtk.go — captured rtk subcommand classes, rtk — token-compressed CLI proxy, Cross-tool skills and prompt templates ecosystem (+3 more)

### Community 84 - "Message"
Cohesion: 0.13
Nodes (20): TurnEnd, MessageChars(), RequestChars(), TestEstimate(), Tokens(), UsageTokens(), Message, Role (+12 more)

### Community 85 - "Index"
Cohesion: 0.22
Nodes (6): Rank(), TestRank(), words(), Hit, Index, IndexEntry

### Community 86 - "CI Workflow (gofmt · vet · build · test)"
Cohesion: 0.25
Nodes (8): CI Workflow (gofmt · vet · build · test), CI gate set (gofmt, go vet, go build, go test -race), go.mod (Go version source for setup-go), SPECS.md §2 — CI gate definition, CI gate: gofmt, go vet, go build, go test -race (.github/workflows/ci.yml), make build/test/vet/fmt/release; single-package test invocation, mise provisions Go 1.27.1 (pinned in go.mod); make falls back to mise x go, Development: mise install, make build/test/release, mise run fallback

### Community 87 - "drained"
Cohesion: 0.40
Nodes (10): TestThinkingLinePrintsBeforeTheResponse(), drained(), newLoginModel(), TestLoginMarkedBackAndCancel(), TestLoginOAuthDeclinedFlipAndCancel(), TestLoginOAuthFlowAndAuthFlip(), TestLoginRefusedWhileRunning(), TestLoginStoresAPIKey() (+2 more)

### Community 88 - ".Run"
Cohesion: 0.15
Nodes (10): globToRegexp(), ignored(), parseIgnore(), TestIgnore(), ListFiles(), model, mentionQuery(), TestMentionQuery() (+2 more)

### Community 89 - "moca/update_test.go"
Cohesion: 0.42
Nodes (9): fakeReleases(), setVersion(), TestRunUpdateCheck(), TestRunUpdateFetchFails(), TestRunUpdateLocalNewer(), TestRunUpdateMissingPlatformPackage(), TestRunUpdateNoVersionStamp(), TestRunUpdateUpToDate() (+1 more)

### Community 90 - "decode"
Cohesion: 0.15
Nodes (5): readLine(), decode(), errorf(), lsTool, readTool

### Community 92 - "NewUnjailed"
Cohesion: 0.29
Nodes (4): Agent, NewUnjailed(), TestUnjailedAndAllowAll(), Unjailed

### Community 93 - "AGENTS.md — moca"
Cohesion: 0.25
Nodes (7): AGENTS.md — moca, Commands, Conventions & gotchas, Docs, graphify, Ground truth (read before changing behavior), Layout & dependency rules

### Community 94 - "serverEnv"
Cohesion: 0.29
Nodes (8): TestResolveEnv(), ResolveEnv(), TestKeepNameWindows(), FilterEnv(), keepName(), serverEnv(), TestFilterEnv(), TestServerEnvExplicitEntryReplacesInherited()

### Community 95 - "create-command.md — starter prompt template"
Cohesion: 0.33
Nodes (3): create-command.md — starter prompt template, Project commands dir .moca/prompts/, Personal commands dir {{prompts_dir}} (~/.config/moca/prompts/)

### Community 96 - "1.6 Resume replay + /resume picker"
Cohesion: 0.33
Nodes (6): agent package (Resume, History, SetEffort), 1.4 Argument completion in the / dropdown + interactive /model, /model picker marks (no key) via CheckCredential, picker type shared by login /model /resume, 1.6 Resume replay + /resume picker, session package (List, FindForWorkdir)

### Community 97 - "encoding/json.RawMessage"
Cohesion: 0.16
Nodes (10): CallResult, callResult, Annotations, client, Content, IndexTool, response, rpcError (+2 more)

### Community 98 - "install.sh script"
Cohesion: 0.67
Nodes (5): die(), fetch(), note(), install.sh script, usage()

### Community 99 - "1.1 Markdown-lite rendering of responses"
Cohesion: 0.40
Nodes (4): ansi.Wrap (terminal ANSI wrapping), ansi.Wrap span re-open fix (targeted toggles), 1.1 Markdown-lite rendering of responses, styleBlock (pre-styled spans variant)

### Community 100 - "ollamaRegistry"
Cohesion: 0.33
Nodes (8): newFakeOllama(), ollamaRegistry(), TestOllamaChatWithoutKey(), TestOllamaDiscovery(), TestOllamaDiscoveryVerifyConcurrent(), TestOllamaKeylessAndOptionalKey(), TestOllamaServerDown(), TestOllamaUnauthorizedAndWrongServer()

### Community 101 - "Claude Code frontend-design skill"
Cohesion: 0.50
Nodes (3): AI-generated design default tells, frontend-design Apache 2.0 License, Claude Code frontend-design skill

### Community 102 - "pi mcp-scripting skill"
Cohesion: 0.67
Nodes (4): jev.evaluate reference, jev.evaluate semantic scripting gate, mcpScript tools API, pi mcp-scripting skill

### Community 103 - ".Run"
Cohesion: 0.12
Nodes (14): approve(), findTestFile(), isPathByte(), TestFindTestFile(), TestTestRunCommand(), TestTestRunFailed(), testRunCommand(), testRunFailed() (+6 more)

### Community 106 - "1.2 Inline diff preview for edit/write items"
Cohesion: 0.67
Nodes (3): Claude Code — comparison reference, diffPreview(detail, width, maxLines), 1.2 Inline diff preview for edit/write items

### Community 107 - "Summarizer injection — provider-free compaction over a neutral Entry view"
Cohesion: 0.33
Nodes (7): compaction entry — summary, firstKeptEntryId, tokensBefore, usage, cumulative read/modified lists, llm.Message, NoCacheWrite — compaction strips every cache_control, session.Messages rebuild — assistant batches by message id, merged user messages, skip non-conversation, Summarizer injection — provider-free compaction over a neutral Entry view, structured summary — Goal/Constraints/Progress/Key Decisions/Next Steps/Critical Context + cumulative file lists, summary request shape — cheap model, effort minimal, NoCacheWrite, max_tokens ≤4096, payload capped 3 chars/token

### Community 129 - "Dynamic client registration: dynamic_agent_client → issued oaiapp_ id; no client secret"
Cohesion: 0.33
Nodes (6): Dynamic client registration: dynamic_agent_client → issued oaiapp_ id; no client secret, ext_agent_host_id: stable opaque per-host identity (not a credential), Live probes 2026-10-06: discovery/JWKS/token endpoint results; interactive flow via phase-7 live gate, Verified OpenAI OAuth endpoints, PKCE S256, loopback 127.0.0.1 callback, JWKS, Sign in with ChatGPT open-source token sharing: permitted for open-source tools, Rotating 30-day refresh tokens; refreshes serialized across processes

## Knowledge Gaps
- **134 isolated node(s):** `github.com/adeotek/moca`, `Shell`, `example.com/totals`, `anthropics/skills frontend-design`, `anthropics/skills skill-creator` (+129 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 356 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **42 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `SPECS.md — moca implemented-state normative spec` connect `SPECS.md — moca implemented-state normative spec` to `start.go`, `tools registry — Tool interface, seven frozen tools, sequential execution in emitted order`, `permissions/shell.go`, `system prompt — built once per run, stored in the session, never rebuilt`, `internal/agent — system prompt, events, loop, compaction + resume, cross-provider transform, Start wiring`, `MCP Manager — zero start, lazy ensure, busy-guarded idle stop, replay-once rules`, `Summarizer injection — provider-free compaction over a neutral Entry view`, `anthropic-messages codec (POST /v1/messages, version header, x-api-key)`, `moca -p "<prompt>" — one-shot agent run (stdin -p form)`, `DESIGN §12 example config fixture (phase-1 decode gate)`, `credential store ~/.config/moca/auth.json (0600, atomic writes, cross-process lock)`, `DESIGN.md — v1 vision/contract (doc/specs)`, `Phase plan: 7 phases, each one reviewed PR`, `moca update [--check] (newest published GitHub release, in-place swap)`, `MCP gating — readOnlyHint && !destructiveHint || approve list || session allow; no name heuristics`, `/-dropdown autocomplete (built-ins + prompt templates, ≤8 rows, tab/enter/esc)`, `opencode-go provider (zen gateway)`?**
  _High betweenness centrality (0.318) - this node is a cross-community bridge._
- **What connects `github.com/adeotek/moca`, `Shell`, `example.com/totals` to the rest of the system?**
  _134 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `start.go` be split into smaller, more focused modules?**
  _Cohesion score 0.11775204101006266 - nodes in this community are weakly interconnected._
- **Why does `Version stamp from git describe --tags --always --dirty` connect `v0.2.0-beta (2026-10-09): Ollama, saved / commands, self-update` to `start.go`?**
  _High betweenness centrality (0.043) - this node is a cross-community bridge._
- **Should `SPECS.md — moca implemented-state normative spec` be split into smaller, more focused modules?**
  _Cohesion score 0.028388278388278388 - nodes in this community are weakly interconnected._
- **Why does `model` connect `charm.land/bubbletea/v2.Cmd` to `start.go`, `Agent`, `Sanitize`, `renderMarkdown`, `Item`, `newModel`, `Start`, `model`, `.View`, `.startSession`, `Input`, `NewDefaultStore`, `Config`, `RenderStatus`?**
  _High betweenness centrality (0.039) - this node is a cross-community bridge._
- **Should `prompt.go` be split into smaller, more focused modules?**
  _Cohesion score 0.06049213943950786 - nodes in this community are weakly interconnected._