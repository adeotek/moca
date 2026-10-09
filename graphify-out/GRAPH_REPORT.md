# Graph Report - moca  (2026-10-09)

## Corpus Check
- 320 files · ~245,942 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 12 file(s) not represented in the graph (top: (none) 6, .jsonc 4, .v2 1)

## Summary
- 3090 nodes · 11902 edges · 188 communities (127 shown, 61 thin omitted)
- Extraction: 89% EXTRACTED · 11% INFERRED · 0% AMBIGUOUS · INFERRED: 1367 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `ffe42442`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- go_pkg_strings
- SPECS.md — moca implemented-state normative spec
- ParseInput
- oauth_test.go
- testing.T
- run
- permissions/shell.go
- newTestModel
- model
- cut_test.go
- .Run
- update/update.go
- newAgentModel
- Parse
- newScript
- NewManager
- moca -p "<prompt>" — one-shot agent run (stdin -p form)
- Input
- anthropic_test.go
- Phase 2: seven tools + agent loop + permissions + skills loader + session schema
- TUI: Bubble Tea + lipgloss, compact/dense, welcome lines
- charm.land/bubbletea/v2.Cmd
- P2 — readability & feedback polish
- openai provider (API key or ChatGPT subscription OAuth)
- /-dropdown autocomplete (built-ins + prompt templates, ≤8 rows, tab/enter/esc)
- NewRegistry
- Effort
- Request
- Model
- opencode-go provider (zen gateway)
- RenderStatus
- v0.2.0-beta (2026-10-09): Ollama, saved / commands, self-update
- Agent
- .Compact
- Sanitize
- tools registry — Tool interface, seven frozen tools, sequential execution in emitted order
- pagerModel
- sync.Mutex
- drained
- anthropic-messages codec (POST /v1/messages, version header, x-api-key)
- Context manager: token-denominated budgets, compaction, recovery
- internal/agent — system prompt, events, loop, compaction + resume, cross-provider transform, Start wiring
- context.Context
- model
- Entry
- Summarize
- .openSessions
- Seven frozen tools for v1 (schema churn = v2 discussion)
- credential store ~/.config/moca/auth.json (0600, atomic writes, cross-process lock)
- DESIGN §12 example config fixture (phase-1 decode gate)
- run
- anthropic provider (api_key only by policy, no OAuth)
- retry_test.go
- v0.1.0-alpha (2026-10-07): first release
- moca update [--check] (newest published GitHub release, in-place swap)
- MCP gating — readOnlyHint && !destructiveHint || approve list || session allow; no name heuristics
- Create
- time.Duration
- registry_test.go
- Config
- Find
- Index
- Plan
- SOURCES.md
- Claude Code skill-creator skill
- renderMarkdown
- Architecture: package layout with strict import direction
- Plan: make moca work like a senior engineer — reliable, consistent, token-efficient
- format.go
- oauth.go
- fixture/main.go
- charm.land/bubbletea/v2.Msg
- system prompt — built once per run, stored in the session, never rebuilt
- Start
- MCP Manager — zero start, lazy ensure, busy-guarded idle stop, replay-once rules
- moca login <provider> / TUI /login: credential wizard (OAuth or masked API key)
- 2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)
- resolveResume
- encoding/json.RawMessage
- .fn
- input_test.go
- v0.1.0-alpha (2026-10-07)
- Agent
- External tools: rtk, graphify, skills ecosystem
- Message
- post
- CI Workflow (gofmt · vet · build · test)
- BuildSystemPrompt
- .Run
- go_pkg_flag
- webFetch
- quick_validate.py
- NewUnjailed
- AGENTS.md — moca
- net/http/httptest.Server
- create-command.md — starter prompt template
- 1.6 Resume replay + /resume picker
- agent events — TextDelta … TurnEnd/SteeringApplied/Compacted/Resumed
- install.sh script
- 1.1 Markdown-lite rendering of responses
- fixture/order.go
- Claude Code frontend-design skill
- pi mcp-scripting skill
- history.go
- allowAll
- StartError
- 1.2 Inline diff preview for edit/write items
- fixture/slug.go
- AllowAll
- askRootChecker
- boundedWriter
- fakeCmds
- Never create a commit without the user's explicit approval
- Lookup
- evals/run.sh
- reference/codes.go
- Item
- bugfix/check.sh
- feature/check.sh
- largefile/check.sh
- plan/check.sh
- 2.3 Richer activity row
- compact.Budget.Trigger() as the red threshold
- history.go / history.jsonl store
- 2.2 One-row item lines
- OpenCode — comparison reference
- Arrange-Act-Assert with xUnit/NSubstitute
- picker.go
- response-cache.md
- refactor/check.sh
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
- .Run
- TestModelPickerRefreshesOllama
- fakeAS
- vague/check.sh
- web/check.sh
- README.md
- errorf
- example.com/codes
- LoadPrompts
- example.com/fetcher
- example.com/inventory
- Usage
- Store
- example.com/shop
- example.com/slug
- example.com/temp
- EntryTokens
- net/http.Client
- Serialize
- LineReader
- reference/order.go
- time.Time
- .RunPlan
- runTUI
- fetch.go
- StartOptions
- retryable
- Eval baseline — DESIGN rev 21 harness pass
- fakeOllama
- DetectVerify
- Compacted
- PlanChanged
- Resumed
- SteeringApplied
- StreamReset
- TextDelta
- ThinkingDelta
- Warning
- YoloChanged
- EnvError
- fakeSnap
- rootChecker

## God Nodes (most connected - your core abstractions)
1. `SPECS.md — moca implemented-state normative spec` - 378 edges
2. `model` - 75 edges
3. `newTestModel()` - 63 edges
4. `run()` - 61 edges
5. `newScript()` - 60 edges
6. `key()` - 44 edges
7. `newAgentModel()` - 43 edges
8. `textTurn()` - 39 edges
9. `Config` - 37 edges
10. `Parse()` - 37 edges

## Surprising Connections (you probably didn't know these)
- `Entry point cmd/moca/main.go; run() is the testable core; sibling subcommand files` --references--> `run()`  [INFERRED]
  AGENTS.md → cmd/moca/main.go
- `Strict import direction + structural interfaces; seven frozen tools with golden schemas` --semantically_similar_to--> `Architecture: package layout with strict import direction`  [INFERRED] [semantically similar]
  AGENTS.md → docs/specs/DESIGN.md
- `Strict config decode: unknown keys rejected; new key = defaults + validation + SPECS update` --references--> `SPECS.md — moca implemented-state normative spec`  [INFERRED]
  AGENTS.md → docs/specs/SPECS.md
- `README non-goals (v1, probably forever)` --semantically_similar_to--> `v1 explicit non-goals: subagents, hooks, plan mode, LSP, binary plugins…`  [INFERRED] [semantically similar]
  README.md → docs/specs/DESIGN.md
- `DESIGN revision 17: /sessions picker + lock-guarded session.Delete` --semantically_similar_to--> `/sessions manager: list/switch/ctrl+d delete; single-writer delete refusal`  [INFERRED] [semantically similar]
  docs/specs/DESIGN.md → CHANGELOG.md

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

## Communities (188 total, 61 thin omitted)

### Community 0 - "go_pkg_strings"
Cohesion: 0.10
Nodes (22): TestTeaAPI(), claudeServer, opencodeServer, request, usageError, anthropicEvent, anthropicUsage, completionsChunk (+14 more)

### Community 1 - "SPECS.md — moca implemented-state normative spec"
Cohesion: 0.03
Nodes (103): cmd/moca/main.go (entry, signals, testable run()), cmd/moca/tui.go (TUI path + trust), SPECS.md — moca implemented-state normative spec, agent.StartError (run setup failure → exit 1), charm.land/bubbletea/v2 (TUI framework, with bubbles + lipgloss), required OAuth scope chatgpt.tokens.use.direct, /copy — OSC 52 copy of last assistant text, /cost — in/out/cache breakdown + $cost (+95 more)

### Community 2 - "ParseInput"
Cohesion: 0.47
Nodes (6): Prompt, HelpText(), ParseInput(), TestParseInput(), TestPromptDirsReloadAtRunEnd(), TestLoginCommandParsingAndHelp()

### Community 3 - "oauth_test.go"
Cohesion: 0.19
Nodes (23): browse(), firstURL(), newFakeAS(), runLogin(), TestCallbackPageReportsFailure(), TestLineReaderAbandonedReadKeepsLine(), TestLoginAccessDenied(), TestLoginCallbackWinLeavesNextLineForCaller() (+15 more)

### Community 4 - "testing.T"
Cohesion: 0.04
Nodes (80): ollamaServer(), ollamaStart(), TestOllamaDeclaredWindowIsQuiet(), TestOllamaReportedWindowIsQuiet(), TestOllamaStartErrors(), TestOllamaTinyWindowWarns(), TestStartWithOllamaModel(), TestResumeNotASession() (+72 more)

### Community 5 - "run"
Cohesion: 0.05
Nodes (75): fakeCompletions(), isolate(), TestExitCodes(), TestHelpListsSurface(), TestInterruptedExit130(), TestLoginLogoutCLI(), TestMCPImportCLI(), TestNoApproveSkipsProjectInstructions() (+67 more)

### Community 6 - "permissions/shell.go"
Cohesion: 0.06
Nodes (42): setup, canonical(), Jail, NewJail(), resolveDeep(), setup(), TestJail(), TestJailApprovable() (+34 more)

### Community 7 - "newTestModel"
Cohesion: 0.06
Nodes (59): TestActivityLine(), TestCoalescedShrinkRemembersTheLastFlushedRow(), TestInputBoxGrowsAndShrinks(), TestPagerCloseDefersTheRelease(), TestRefuseRunningWhileOutputInFlight(), TestShrinkIsPaddedUntilTheGuardWindowPasses(), TestShrinkPaddingFitsTheScreen(), TestSpinnerTicksOnlyForTheLiveRun() (+51 more)

### Community 8 - "model"
Cohesion: 0.07
Nodes (15): ShellOutput, firstLineOf(), model, newModel(), Run(), AbbrevEffort(), hintSeen(), markHint() (+7 more)

### Community 9 - "cut_test.go"
Cohesion: 0.30
Nodes (15): FindCut(), a(), call(), res(), TestFindCutKeepsRecentAtValidPoint(), TestFindCutNeverBetweenCallAndResult(), TestFindCutNothingToDo(), TestFindCutSplitTurn() (+7 more)

### Community 10 - ".Run"
Cohesion: 0.07
Nodes (27): applyEdit(), formatDiff(), indentOf(), joinInts(), lineOf(), matchLines(), replaceExact(), replaceFuzzy() (+19 more)

### Community 11 - "update/update.go"
Cohesion: 0.09
Nodes (32): Compare(), comparePre(), ExtractBinary(), FindAsset(), FindChecksums(), IsTemporary(), ParseChecksums(), ParseVersion() (+24 more)

### Community 12 - "newAgentModel"
Cohesion: 0.07
Nodes (52): keyMsg(), TestExitCommandQuits(), TestCompactEscapeCancels(), TestCompactingBlocksRun(), TestCompactNothing(), TestEditItemShowsDiff(), TestApprovalKeysAnswerAfterPause(), TestApprovalKeysIgnoredWhileTyping() (+44 more)

### Community 13 - "Parse"
Cohesion: 0.06
Nodes (44): scanner, span, Parse(), TestAPIKeyOptional(), TestDefaultsApplied(), TestEnvRefs(), TestOAuthAllowedForOpenAI(), TestOAuthRejectedForNonOAuthProvider() (+36 more)

### Community 14 - "newScript"
Cohesion: 0.08
Nodes (77): scriptServer, newScript(), startTest(), startTestWith(), startWith(), TestAbortWritesSyntheticResults(), TestFailedCallDoesNotCancelRest(), TestLengthStopTruncatedToolCall() (+69 more)

### Community 15 - "NewManager"
Cohesion: 0.12
Nodes (40): build(), MCPServer, crashingServer(), shServer(), skipWindows(), TestCallTimeoutStopsWedgedServer(), TestHandshakeTimeout(), TestHTTPAbortedStreamReplayRule() (+32 more)

### Community 16 - "moca -p "<prompt>" — one-shot agent run (stdin -p form)"
Cohesion: 0.07
Nodes (38): cmd/moca/cli.go (flags → Options), cmd/moca/oneshot.go (-p agent path), Agent.Close — stops live MCP servers and closes the session writer, agent.Resume — validate first, repair, restore model/effort, jail at header workdir, /clear — new session file, old stays resumable, resets yolo to startup flag, /exit //q ///quit — quit (cancels in-flight runs), /resume [id8] — picker over this directory’s sessions, /sessions — session manager (delete via lock, open refuses deletion) (+30 more)

### Community 17 - "Input"
Cohesion: 0.13
Nodes (4): lineCount(), hasControl(), Input, paste

### Community 18 - "anthropic_test.go"
Cohesion: 0.16
Nodes (36): newAnthropic(), keyCred(), mustJSON(), sseServer(), TestAnthropicBodyHygiene(), TestAnthropicBudgetAndNoCache(), TestAnthropicBudgetFollowsEffort(), TestAnthropicCredentialHeaders() (+28 more)

### Community 19 - "Phase 2: seven tools + agent loop + permissions + skills loader + session schema"
Cohesion: 0.08
Nodes (33): Docs map: external-tools.md, oauth-verification.md; deleted docs/plans & docs/reviews, graphify workflow rules: query/path/explain first, update . after code changes, Failing-test investigation protocol: edit refuses until read+search happened, ! / !! shell prefixes: user-initiated, bypass model-facing analysis, Per-provider baseUrl override + custom providers (vLLM/LM Studio/Ollama day 1), Built-in rtk skill (embed.FS, extracted to ~/.local/share/moca/builtin-skills/<version>), docs/plans/phase-<n>-*.md task-level plans (deleted; cited as history), Edit-ladder tests (phase 2, non-negotiable: exact/ambiguous/replace_all/CRLF/BOM…) (+25 more)

### Community 20 - "TUI: Bubble Tea + lipgloss, compact/dense, welcome lines"
Cohesion: 0.10
Nodes (32): Thinking blocks/signatures model-bound; cross-provider transforms in agent (SPECS §14), Approvals show the whole command (≤6 lines) + ctrl+o pages all of it, Argument completion for /model, /effort, /login, /logout, /show, /resume, Persistent prompt history (history.jsonl per workdir) + ctrl+r search, Markdown-lite response rendering (bold/code/headings/bullets/fenced blocks), Context pressure: % yellow at 70%, red where auto-compaction fires, Bracketed paste: verbatim, never auto-sent, [paste N lines] chip (alt+p), /copy: last assistant message to clipboard via OSC 52 (SSH-safe) (+24 more)

### Community 21 - "charm.land/bubbletea/v2.Cmd"
Cohesion: 0.11
Nodes (14): NewDefaultStore(), Headless(), OAuthProvider(), TestHeadlessDetectionSSH(), model, providerNames(), providerRow(), TestLoginRowForOllama() (+6 more)

### Community 22 - "P2 — readability & feedback polish"
Cohesion: 0.06
Nodes (29): 0. Fixed in this pass (already on `fix/polish`), 1.1 Markdown-lite rendering of responses — **M**, 1.2 Inline diff preview for `edit`/`write` items — **S/M**, 1.3 Context-pressure color in the status bar — **S**, 1.4 Argument completion in the `/` dropdown + interactive `/model` — **M**, 1.5 `@path` file mentions — **M**, 1.6 Resume shows where you left off; `/resume` picker — **M**, 1.7 Persistent input history + `ctrl+r` search — **S/M** (+21 more)

### Community 23 - "openai provider (API key or ChatGPT subscription OAuth)"
Cohesion: 0.11
Nodes (26): anthropic-messages protocol adapter (~150 LOC, hand-rolled HTTP+SSE), anthropic provider (api_key only; subscription OAuth not permitted), api_key auth mode (stored credential or env: reference), openai-completions protocol adapter, openai provider (API key or ChatGPT subscription OAuth), openai-responses protocol adapter, opencode-go provider (OpenCode Zen, mixed-protocol catalog: glm-5.3, glm-5.3-flash, minimax-m3, kimi-k3…), Phase 7: OAuth providers + upstream graphify PR + v0.1 (+18 more)

### Community 24 - "/-dropdown autocomplete (built-ins + prompt templates, ≤8 rows, tab/enter/esc)"
Cohesion: 0.07
Nodes (27): activity row ⠋ working… (spinner, elapsed, esc to interrupt), Agent.Status() — model, effort, anchored tokens, window, usage, cost, Sub/Hard/Yolo, @path completion (≤5000 indexed files, ranked, text sent as written), /effort [level] (also shift+tab wrap; refused while running), /hard — modelHard + high ↔ saved pair, /show <n> — open any item in the pager, usage-anchored context estimate — anchor + delta after each turn, agent control surface — SetModel/SetEffort/ToggleHard/Models/Status/Steer/AddNote/Undo (mutex-guarded) (+19 more)

### Community 25 - "NewRegistry"
Cohesion: 0.12
Nodes (9): Builtins(), runReg(), TestRegistryInvestigationBanner(), Registry, NewRegistry(), TestRegistryDispatch(), TestSchemasFrozen(), echoTool (+1 more)

### Community 26 - "Effort"
Cohesion: 0.10
Nodes (5): savedModel, Agent, Effort, ParseEffort(), TestParseEffort()

### Community 27 - "Request"
Cohesion: 0.10
Nodes (25): ContentBlock, Event, Request, Response, StopReason, Adapter, anthropicOverflow(), anthropicStop() (+17 more)

### Community 28 - "Model"
Cohesion: 0.12
Nodes (12): ModelOverride, CostConfig, ProviderConfig, IsLocalProvider(), SplitModel(), TestSplitModel(), costM(), Model (+4 more)

### Community 29 - "opencode-go provider (zen gateway)"
Cohesion: 0.11
Nodes (28): opencode.ai Go-tier docs (MissingSessionID, /zen/v1 exemption), level map basicThinking {low,medium,high} (gateway rejects off/minimal: kimi-k2.7-code, mimo-v2.6-*), opencode-go deepseek-v4.1-flash (openai-completions, 1M ctx), opencode-go deepseek-v4-flash (openai-completions, 1M ctx), opencode-go deepseek-v4-flash-vision-exp (vision experiment), opencode-go deepseek-v4-pro (off-peak rate, 1M ctx), level map deepseekProThinking {high,max}, level map deepseekThinking {low,high,max} (+20 more)

### Community 30 - "RenderStatus"
Cohesion: 0.20
Nodes (13): TestStatusWideRunesNeverExceedWidth(), TestStatusLineCarriesTheSetupState(), FmtPercent(), renderLine(), RenderStatus(), TestContextPressure(), TestFmtPercent(), TestRenderStatusFull() (+5 more)

### Community 31 - "v0.2.0-beta (2026-10-09): Ollama, saved / commands, self-update"
Cohesion: 0.14
Nodes (25): Release workflow asset contract with internal/update; install.sh/install.ps1 consumers, Ollama fail-fast: server down says how to start; unpulled model says ollama pull, --resume/--continue replay last 3 turns; new /resume picker, v0.2.0-beta (2026-10-09): Ollama, saved / commands, self-update, Built-in & user-extensible model catalog (window, max out, cost, thinkingMode), /clear starts a new session file (old one stays resumable), Provider-qualified model ids (opencode-go/glm-5.3-flash), Resume: moca --resume <id8|last> / --continue (workdir→session mapping) (+17 more)

### Community 32 - "Agent"
Cohesion: 0.15
Nodes (11): Options, ToolStart, Agent, Outcome, New(), userText(), verifyNudge(), withPlanEnvelope() (+3 more)

### Community 33 - ".Compact"
Cohesion: 0.21
Nodes (7): Agent, summarizerUnavailable(), Budget, NewBudget(), TestBudget(), Prev, Summarizer

### Community 34 - "Sanitize"
Cohesion: 0.11
Nodes (17): AutoAllow(), Answer, Question, approvalPanel(), approvalPrompt(), TestApprovalPromptSanitized(), diffPreview(), TestDiffPreview() (+9 more)

### Community 35 - "tools registry — Tool interface, seven frozen tools, sequential execution in emitted order"
Cohesion: 0.11
Nodes (21): Jail.Approvable — resolves refused writes to the ask-write root, ask-write root ~/.config/moca/prompts (write/edit ask, never persistent), !cmd / !!cmd user-initiated shell (30 s, ShellEnv, 30K truncation; esc kills), config.EnvRefs — every referenced variable (shell hygiene input), edit ladder — exact match → replace_all → whitespace-insensitive re-indent → error, frozen tool schemas — golden schemas.golden.json (v2 discussion to change), inline edit diffs — changed lines indented, ≤8 rows then N more, mcp proxy tool — byte-identical MCPSpec, search/describe/call rendering, 30K truncate (+13 more)

### Community 36 - "pagerModel"
Cohesion: 0.20
Nodes (4): TestMatchRanges(), matchRanges(), newPager(), pagerModel

### Community 37 - "sync.Mutex"
Cohesion: 0.09
Nodes (15): responseID(), TestResponseID(), callResult, startStdio(), TestStdioEarlyExit(), TestStdioWriteEPIPE(), KillProcessGroup(), SetProcessGroup() (+7 more)

### Community 38 - "drained"
Cohesion: 0.16
Nodes (21): TestThinkingLinePrintsBeforeTheResponse(), drained(), newLoginModel(), TestLoginMarkedBackAndCancel(), TestLoginOAuthDeclinedFlipAndCancel(), TestLoginOAuthFlowAndAuthFlip(), TestLoginRefusedWhileRunning(), TestLoginStoresAPIKey() (+13 more)

### Community 39 - "anthropic-messages codec (POST /v1/messages, version header, x-api-key)"
Cohesion: 0.12
Nodes (20): anthropic stream flow (message_start/content_block_*/message_delta/stop), anthropic cache breakpoints — system end + last tool + last eligible block, openai-completions thinking — reasoning_content; history thinking never sent, cross-provider transform — thinking replay bound by Model == request model, ErrContextOverflow (prompt too long / context_length_exceeded), subscription wire use — bearer, store:false, no max_output_tokens, tools in one namespace group, overflow recovery — errOverflow (nothing persisted), one compact-and-retry, then surface, anthropic-messages codec (POST /v1/messages, version header, x-api-key) (+12 more)

### Community 40 - "Context manager: token-denominated budgets, compaction, recovery"
Cohesion: 0.17
Nodes (20): /sessions manager: list/switch/ctrl+d delete; single-writer delete refusal, JSONL sessions: single-writer flock, 0600, compaction entry (summary, firstKeptEntryId, tokensBefore, usage), Compaction loop guard: stop with error if still above trigger, Context manager: token-denominated budgets, compaction, recovery, JSONL append-only session transcripts (0600 file, 0700 dir, id+parentId chain), keepRecentTokens (default 20K kept verbatim), Overflow recovery: one compact-and-retry attempt (plain length stop ≠ overflow) (+12 more)

### Community 41 - "internal/agent — system prompt, events, loop, compaction + resume, cross-provider transform, Start wiring"
Cohesion: 0.14
Nodes (22): agent build() — registry, jail, checkers, env, snapshots, MCP manager, agent (shared with Resume), /compact — manual compaction on cancellable context (compacting… transient), cmd/moca entrypoint package, compact.Budget — window/reserve/keepRecent, small windows scale to window/4, context pressure coloring — yellow 70%, red at trigger from compact.Budget.Trigger(), internal/agent — system prompt, events, loop, compaction + resume, cross-provider transform, Start wiring, internal/compact — budgets, cut points, lossy serialization, structured summaries (no provider import), internal/config — paths, JSONC pre-pass, typed config + validation, env indirection (+14 more)

### Community 42 - "context.Context"
Cohesion: 0.10
Nodes (18): TestExitForOAuthSentinels(), exitFor(), firstLine(), runOneShot(), envOr(), githubToken(), runUpdate(), Rank() (+10 more)

### Community 43 - "model"
Cohesion: 0.12
Nodes (16): clampRunes(), model, matchCommands(), parseSlash(), slashQuery(), TestClampRunes(), TestSlashQuery(), model (+8 more)

### Community 44 - "Entry"
Cohesion: 0.15
Nodes (10): Entry, ModelChange, gitClean(), Snapshots, NewSnapshots(), Elision, ErrorInfo, PermissionMode (+2 more)

### Community 45 - "Summarize"
Cohesion: 0.31
Nodes (8): Metrics, Collect(), fatal(), main(), meanSD(), readResults(), Summarize(), TestSummarize()

### Community 46 - ".openSessions"
Cohesion: 0.19
Nodes (11): Delete(), TestDelete(), TestListAll(), Info, List(), ListAll(), TestListPreviewsAndFilters(), fmtAge() (+3 more)

### Community 47 - "Seven frozen tools for v1 (schema churn = v2 discussion)"
Cohesion: 0.11
Nodes (18): Seven frozen tools; schemas golden in internal/tools/testdata/schemas.golden.json, Inline edit diffs: changed lines (red/green, ≤8) under the item line, edit tool: path + old_string/new_string + replace_all, ls tool: one level, dirs suffixed /, hidden opt-in, mvdan.cc/sh/v3/syntax — the single non-UI third-party dependency, Read-before-write + unchanged-on-disk guard (mtime + size + hash), Output redirection targets must pass the path jail (literal targets only), DESIGN revision 9: trailing-comma fix, read-only roots, mcp describe, shell stateless (+10 more)

### Community 48 - "credential store ~/.config/moca/auth.json (0600, atomic writes, cross-process lock)"
Cohesion: 0.12
Nodes (18): credential store ~/.config/moca/auth.json (0600, atomic writes, cross-process lock), Registry.CheckCredential — network-free login check (/model, resume), /model [id] — cross-provider switch (refused while running, forfeits prompt cache), config.SetObjectEntry — insert key: rawJSON (mcp import), config.SetString — replace a string in place, last-duplicate wins, non-string refused, mcp.DiscoverSources — claude/opencode/pi config discovery paths, moca login --api-key (prompt for a key instead of OAuth), moca login --no-browser (headless OAuth: URL + pasted code) (+10 more)

### Community 49 - "DESIGN §12 example config fixture (phase-1 decode gate)"
Cohesion: 0.18
Nodes (18): config.Standardize preserves byte offsets; example.jsonc is byte-exact DESIGN §12 fixture, Secrets: apiKey must be env:VAR; .env and *.key gitignored, never committed, Strict config decode: unknown keys rejected; new key = defaults + validation + SPECS update, moca CLI surface: TUI, -p, login/logout, mcp import/index, --version (§12.5), DESIGN §12 example config fixture (phase-1 decode gate), Config: ~/.config/moca/config.jsonc, typed struct, fail-fast validation, env:VAR secret indirection (the one indirection syntax everywhere), -p exit codes: 0 completed · 1 runtime · 2 config/usage · 3 maxSteps · 130 interrupted (+10 more)

### Community 50 - "run"
Cohesion: 0.24
Nodes (13): Entry point cmd/moca/main.go; run() is the testable core; sibling subcommand files, parseArgs(), printUsage(), TestParseArgs(), loginAPIKey(), runLogin(), runLogout(), yesNo() (+5 more)

### Community 51 - "anthropic provider (api_key only by policy, no OAuth)"
Cohesion: 0.13
Nodes (13): moca — MO Coding Agent (binary = repo = module path), DESIGN.md — v1 vision/contract (doc/specs), level map anthropicAdaptive {low,medium,high,max} (off/minimal clamp up), default base URLs (api.anthropic.com, api.openai.com/v1, opencode.ai/zen/go), anthropic claude-haiku-4-5 (200k ctx, budget), config.OAuthProviders pinned against provider.oauthProviders by a test, anthropic claude-opus-5-5 (1M ctx, adaptive), anthropic provider (api_key only by policy, no OAuth) (+5 more)

### Community 52 - "retry_test.go"
Cohesion: 0.29
Nodes (16): Retry, DefaultRetryPolicy(), RetryNotice, fail(), TestMidStreamFailureResetsAndRetriesOnce(), TestMidStreamRetryBacksOffAndNotifies(), TestNoRetryOn400(), testPolicy() (+8 more)

### Community 53 - "v0.1.0-alpha (2026-10-07): first release"
Cohesion: 0.22
Nodes (18): Default HTTP client bounds response-header wait at 120s, opencode-go [1210] server_error flake retried even on HTTP 400, v0.1.0-alpha (2026-10-07): first release, MCP call gating: default-deny via readOnlyHint annotations / per-server approve lists, mcp config block: idleTimeout scalar beside mcp.servers map + approve lists, Persisted MCP discovery index (~/.local/share/moca/mcp-index.json, configHash-invalidated), MCP env hygiene: filtered subprocess env (PATH/HOME/USER/LANG/TERM/TMPDIR/XDG_*), moca mcp import (Claude Code/OpenCode/Pi configs; secrets rewritten to env:) + mcp index (+10 more)

### Community 54 - "moca update [--check] (newest published GitHub release, in-place swap)"
Cohesion: 0.14
Nodes (17): cmd/moca/update.go (moca update subcommand), checksums.txt sha256 verification (missing warns; mismatch refuses), moca update --check flag (report only, never writes), git describe version stamp (-ldflags -X config.Version), install-script gate — fresh/update/mismatch refusals for install.sh + functional pwsh run, internal/update — GitHub release fetch, semver compare, checksum verify, atomic replace, make build (bin/moca, version-stamped from git describe), make release (dist/ cross-compiled, darwin included) (+9 more)

### Community 55 - "MCP gating — readOnlyHint && !destructiveHint || approve list || session allow; no name heuristics"
Cohesion: 0.12
Nodes (17): TUI approval prompt (allow <cmd>? a once / ctrl+a always / d deny; 700 ms typing pause; esc denies), config.AppendString — comment-preserving array append (allow lists), 32-entry shell.allow default list (DESIGN §12), hard-deny class — sudo/doas/dd/shred/chown/mkfs* (+ su), never overridable, config key: mcp.servers.<name>.approve (allow list, "*" trusts the server), MCP gating — readOnlyHint && !destructiveHint || approve list || session allow; no name heuristics, mvdan.cc/sh/v3 (only non-UI dependency, shell parsing), phase-5 live legs — real model drives mcp search/describe/call + gating (+9 more)

### Community 56 - "Create"
Cohesion: 0.17
Nodes (13): lockFile(), lockFile(), Create(), Writer, lastNewline(), newID(), Open(), TestCreateAppendRead() (+5 more)

### Community 57 - "time.Duration"
Cohesion: 0.24
Nodes (7): TestFmtElapsed(), fmtElapsed(), fmtThinkDuration(), Options, HTTPError, Item, Items

### Community 58 - "registry_test.go"
Cohesion: 0.25
Nodes (20): TestNoOllamaNoProbe(), TestOllamaVerify(), NewRegistry(), mustCfg(), TestAPIKeyMissingMentionsLogin(), TestAPIKeyStoreWinsOverEnv(), TestBaseURLPrecedence(), TestBuiltinModelNeedsProtocol() (+12 more)

### Community 59 - "Config"
Cohesion: 0.14
Nodes (12): ContextConfig, MCPConfig, ShellConfig, SnapshotConfig, TUIConfig, WebConfig, WebSearchConfig, Default() (+4 more)

### Community 60 - "Find"
Cohesion: 0.17
Nodes (16): canonical(), Find(), FindForWorkdir(), header(), headerAndPreview(), idOf(), InWorkdir(), list() (+8 more)

### Community 61 - "Index"
Cohesion: 0.25
Nodes (5): Annotations, Index, IndexEntry, IndexTool, Tool

### Community 62 - "Plan"
Cohesion: 0.14
Nodes (17): TestImportRewritesEmbeddedSecretsInValues(), TestRewriteSecretsValueRuleAlone(), DiscoverSources(), embeddedSecret(), envName(), literalSecretFields(), ParseSource(), Plan() (+9 more)

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

### Community 67 - "Plan: make moca work like a senior engineer — reliable, consistent, token-efficient"
Cohesion: 0.15
Nodes (12): Current state (audited 2026-10-09, branch `chore/polish`), Goal, Out of scope, Outcome (2026-10-09), Phase A — measurement (do this first; everything else is judged by it), Phase B — token efficiency (cheapest wins first), Phase C — reliability (move "please do X" from the prompt into the harness), Phase D — senior-engineer working style (prompt, judged by evals) (+4 more)

### Community 68 - "format.go"
Cohesion: 0.12
Nodes (14): TestWelcomeLines(), welcomeText(), TestGitStatusUsesNoOptionalLocks(), AbbrevHome(), FmtTokens(), FmtWindow(), GitBranch(), gitStatusArgs() (+6 more)

### Community 69 - "oauth.go"
Cohesion: 0.17
Nodes (9): audContains(), b64d(), jwksKey(), missingScopes(), pkce(), randText(), TestPKCEUniqueness(), validateIDToken() (+1 more)

### Community 70 - "fixture/main.go"
Cohesion: 0.21
Nodes (7): Sum(), TestTotals(), ParseList(), Average(), Max(), TestMaxAndAverage(), main()

### Community 71 - "charm.land/bubbletea/v2.Msg"
Cohesion: 0.14
Nodes (8): newAsker(), newEventPipe(), TestEventPipeBuffersUntilStarted(), TestEventPipeNonBlockingAndFIFO(), TestPipeSendKeepsOrderWithEvents(), flatten(), eventPipe, trustModel

### Community 72 - "system prompt — built once per run, stored in the session, never rebuilt"
Cohesion: 0.17
Nodes (13): AGENTS.md — project instructions file, builtin skills — embed, ExtractBuiltins into content-hash dirs (currently rtk), CLAUDE.md fallback instructions file, vendored ecosystem corpus internal/skills/testdata/ecosystem (pi/claude/opencode/graphify skills), ExtractBuiltins(dataDir) — temp dir + rename, race-safe, agent.filterSkills — omit builtins whose CLI is missing from PATH, LoadInstructions — global AGENTS.md, workdir AGENTS.md (trusted) else CLAUDE.md, 32K cap, phase-6 live legs — sharpened rtk description drives rtk test, graphify answer (+5 more)

### Community 73 - "Start"
Cohesion: 0.15
Nodes (19): main(), TestDefaultHTTPClientBoundsHeaderWait(), GitState(), Platform(), defaultHTTPClient(), overflowDir(), prepare(), Start() (+11 more)

### Community 74 - "MCP Manager — zero start, lazy ensure, busy-guarded idle stop, replay-once rules"
Cohesion: 0.15
Nodes (12): config key: mcp.servers.<name> (command|url, args, env, headers), errNotSent — request never left: replay-safe, errTransportDead sentinel (errors.Is-visible), MCP streamable HTTP transport — Mcp-Session-Id echo, SSE responses, 15m client, persisted MCP index mcp-index.json (configHash, no schemas, merged saves), JSON-RPC 2.0 core — id routing, ping, paged tools/list, transports interface, MCP Manager — zero start, lazy ensure, busy-guarded idle stop, replay-once rules, MCP stdio transport — line-wise, 4KiB ring, exit drain ≤500ms, dead-sentinel errors (+4 more)

### Community 75 - "moca login <provider> / TUI /login: credential wizard (OAuth or masked API key)"
Cohesion: 0.32
Nodes (12): auth.json moved to config dir; v0.1 store auto-migrated, +10 opencode-go catalog models (deepseek v4.1, kimi-k2.7-code, mimo-v2.6, qwen3.x…), v0.1.1 (2026-10-08): polish + credential store + autocomplete, moca login <provider> / TUI /login: credential wizard (OAuth or masked API key), moca logout <provider> / TUI /logout: clear a stored credential, oauth auth mode (moca login; auth.json 0600, auto-refresh), DESIGN revision 14: credential store + /login//logout + / autocomplete, DESIGN revision 15: slash-command management: seeded /create-command + ask-write root (+4 more)

### Community 76 - "2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)"
Cohesion: 0.17
Nodes (12): 1.1 Terms, 1.2 Client registration, 1.3 Decision, 1. anthropic — decision: `api_key only` (subscription OAuth not permitted), 2.1 Terms, 2.2 Client registration (per the docs), 2.3 Endpoints (verified live 2026-10-06 against `https://auth.openai.com/.well-known/openid-configuration`), 2.4 Inference contract for subscription traffic (open-source flow docs) (+4 more)

### Community 77 - "resolveResume"
Cohesion: 0.40
Nodes (5): TestParseArgsDo(), oneShotSlug(), resolveResume(), Slug(), TestSlug()

### Community 78 - "encoding/json.RawMessage"
Cohesion: 0.07
Nodes (14): ToolSpec, CallResult, TestProxySpecFrozen(), MCPSpec(), readLine(), TestRegistryRecoversToolPanic(), Content, editTool (+6 more)

### Community 80 - "input_test.go"
Cohesion: 0.30
Nodes (11): NewInput(), TestCollapseUsesExactOffset(), TestCtrlC(), TestEditedMarkerSentLiterally(), TestHistory(), TestHistoryRecallChipsUnsafeContent(), TestMarkerNeverCollidesWithTypedText(), TestPasteChipKeepsContent() (+3 more)

### Community 81 - "v0.1.0-alpha (2026-10-07)"
Cohesion: 0.18
Nodes (10): CHANGELOG, CLI, Context & sessions, External tools & the skills ecosystem, MCP, OAuth (subscription login), Providers & protocols, Tools & permissions (+2 more)

### Community 83 - "External tools: rtk, graphify, skills ecosystem"
Cohesion: 0.22
Nodes (7): External tools: rtk, graphify, skills ecosystem, graphify — codebase knowledge graph integration, internal/permissions/rtk.go — captured rtk subcommand classes, rtk — token-compressed CLI proxy, Cross-tool skills and prompt templates ecosystem, Built-in rtk skill (SKILL.md), External tools: rtk + graphify allowlisted with built-in skills

### Community 84 - "Message"
Cohesion: 0.23
Nodes (12): Message, Role, TextOf(), appendUser(), ElidedIDs(), LatestCompaction(), Messages(), TestMessagesFromCompaction() (+4 more)

### Community 85 - "post"
Cohesion: 0.11
Nodes (13): post(), newHTTPError(), retryAfter(), Registry, normalizeOllamaURL(), ollamaBaseURL(), ollamaRoot(), TestNormalizeOllamaURL() (+5 more)

### Community 86 - "CI Workflow (gofmt · vet · build · test)"
Cohesion: 0.22
Nodes (9): CI Workflow (gofmt · vet · build · test), CI gate set (gofmt, go vet, go build, go test -race), go.mod (Go version source for setup-go), SPECS.md §2 — CI gate definition, CI gate: gofmt, go vet, go build, go test -race (.github/workflows/ci.yml), make build/test/vet/fmt/release; single-package test invocation, mise provisions Go 1.27.1 (pinned in go.mod); make falls back to mise x go, Ship gate: make build && bash test/shipgate/run.sh (//go:build shipgate tag) (+1 more)

### Community 87 - "BuildSystemPrompt"
Cohesion: 0.22
Nodes (9): PromptInput, ServerLine, BuildSystemPrompt(), oneLine(), TestBuildSystemPrompt(), TestPromptRTKIsConditional(), Instruction, LoadInstructions() (+1 more)

### Community 88 - ".Run"
Cohesion: 0.19
Nodes (8): globToRegexp(), ignored(), parseIgnore(), TestIgnore(), ListFiles(), TestListFiles(), ignoreRule, searchTool

### Community 89 - "go_pkg_flag"
Cohesion: 0.09
Nodes (14): jsonItem, TestCollect(), main(), Render(), TestRenderTotal(), main(), RenderJSON(), TestRenderJSON() (+6 more)

### Community 90 - "webFetch"
Cohesion: 0.14
Nodes (20): Spill(), cutWeb(), isAlnumByte(), isSpaceByte(), TestWebHTMLConverters(), webClean(), webDisplayURL(), webDoJSON() (+12 more)

### Community 92 - "NewUnjailed"
Cohesion: 0.29
Nodes (4): Agent, NewUnjailed(), TestUnjailedAndAllowAll(), Unjailed

### Community 93 - "AGENTS.md — moca"
Cohesion: 0.25
Nodes (7): AGENTS.md — moca, Commands, Conventions & gotchas, Docs, graphify, Ground truth (read before changing behavior), Layout & dependency rules

### Community 94 - "net/http/httptest.Server"
Cohesion: 0.11
Nodes (18): webSearchKey(), TestResolveEnv(), ResolveEnv(), initialize(), abortingHTTPServer(), TestHTTPCloseBounded(), TestKeepNameWindows(), startHTTP() (+10 more)

### Community 95 - "create-command.md — starter prompt template"
Cohesion: 0.33
Nodes (3): create-command.md — starter prompt template, Project commands dir .moca/prompts/, Personal commands dir {{prompts_dir}} (~/.config/moca/prompts/)

### Community 96 - "1.6 Resume replay + /resume picker"
Cohesion: 0.33
Nodes (6): agent package (Resume, History, SetEffort), 1.4 Argument completion in the / dropdown + interactive /model, /model picker marks (no key) via CheckCredential, picker type shared by login /model /resume, 1.6 Resume replay + /resume picker, session package (List, FindForWorkdir)

### Community 97 - "agent events — TextDelta … TurnEnd/SteeringApplied/Compacted/Resumed"
Cohesion: 0.22
Nodes (9): agent events — TextDelta … TurnEnd/SteeringApplied/Compacted/Resumed, agent loop — sequential calls, steering drains, length-turn handling, exit on no-tool answer, eventPipe — FIFO pipe bridging agent events into Bubble Tea, llm.Request, NormalizeToolID — deterministic ids on use and result, phase-3 gate — scripted tmux TUI session (status bar, steering, approvals, chips), resume replay — agent.Resumed banner + last 3 user turns replayed, runDoneMsg — completion rides the event pipe, cannot overtake a delta (+1 more)

### Community 98 - "install.sh script"
Cohesion: 0.67
Nodes (5): die(), fetch(), note(), install.sh script, usage()

### Community 99 - "1.1 Markdown-lite rendering of responses"
Cohesion: 0.40
Nodes (4): ansi.Wrap (terminal ANSI wrapping), ansi.Wrap span re-open fix (targeted toggles), 1.1 Markdown-lite rendering of responses, styleBlock (pre-styled spans variant)

### Community 100 - "fixture/order.go"
Cohesion: 0.28
Nodes (6): Order, User, NewOrder(), TestOrderEmail(), TestUserEmail(), NewUser()

### Community 101 - "Claude Code frontend-design skill"
Cohesion: 0.50
Nodes (3): AI-generated design default tells, frontend-design Apache 2.0 License, Claude Code frontend-design skill

### Community 102 - "pi mcp-scripting skill"
Cohesion: 0.67
Nodes (4): jev.evaluate reference, jev.evaluate semantic scripting gate, mcpScript tools API, pi mcp-scripting skill

### Community 103 - "history.go"
Cohesion: 0.53
Nodes (5): appendHistory(), loadHistory(), rewriteHistory(), TestHistoryPersistence(), historyRec

### Community 106 - "1.2 Inline diff preview for edit/write items"
Cohesion: 0.67
Nodes (3): Claude Code — comparison reference, diffPreview(detail, width, maxLines), 1.2 Inline diff preview for edit/write items

### Community 107 - "fixture/slug.go"
Cohesion: 0.40
Nodes (4): Limit(), Make(), TestLimit(), TestMake()

### Community 113 - "Lookup"
Cohesion: 0.50
Nodes (3): Lookup(), TestLookupBounds(), Valid()

### Community 114 - "evals/run.sh"
Cohesion: 1.00
Nodes (3): fresh_repo(), one(), run.sh script

### Community 129 - "picker.go"
Cohesion: 0.25
Nodes (6): modelPickItems(), TestWindowRange(), model, windowRange(), pickItem, pickState

### Community 146 - ".Run"
Cohesion: 0.11
Nodes (16): TestVerifies(), verifies(), approve(), findTestFile(), isPathByte(), TestFindTestFile(), TestTestRunCommand(), TestTestRunFailed() (+8 more)

### Community 147 - "TestModelPickerRefreshesOllama"
Cohesion: 0.33
Nodes (6): contains(), count(), dropNames(), TestDropdownListsCommandsAndPrompts(), TestModelPickerRefreshesOllama(), TestShowCompletionNewestFirst()

### Community 148 - "fakeAS"
Cohesion: 0.20
Nodes (5): TestJWKSExponentSanity(), TestRevoke(), TestValidateIDToken(), TestValidateIDTokenKidSelection(), fakeAS

### Community 152 - "errorf"
Cohesion: 0.09
Nodes (24): ToolEnd, TestProxyPlanModeCallFollowsNormalGating(), cutRunes(), errResult(), isTrue(), newProxy(), runP(), TestCutRunesSafe() (+16 more)

### Community 154 - "LoadPrompts"
Cohesion: 0.08
Nodes (29): filterSkills(), TestFilterSkillsDropsMissingBuiltins(), builtinHash(), ExtractBuiltins(), extractBuiltins(), TestEcosystemSkillsLoadUnchanged(), closingQuote(), ParseFrontmatter() (+21 more)

### Community 157 - "Usage"
Cohesion: 0.16
Nodes (8): Status, TurnEnd, Result, MessageChars(), RequestChars(), TestEstimate(), UsageTokens(), Usage

### Community 158 - "Store"
Cohesion: 0.30
Nodes (4): Store, newHostID(), validateAPIKey(), storeFile

### Community 162 - "EntryTokens"
Cohesion: 0.22
Nodes (10): Kind, callKey(), Agent, staleResults(), TestStaleResults(), EntryTokens(), Entry, Tokens() (+2 more)

### Community 163 - "net/http.Client"
Cohesion: 0.42
Nodes (6): OAuthConfig, Login(), Revoke(), tokenFrom(), Refresher, Token

### Community 164 - "Serialize"
Cohesion: 0.25
Nodes (8): callLine(), safeTail(), Serialize(), TestSerialize(), TestSerializeRuneSafeAndEmptyText(), TestTrackFiles(), TestTrackFilesNormalizesPaths(), TrackFiles()

### Community 165 - "LineReader"
Cohesion: 0.28
Nodes (8): LineReader, NewLineReader(), parsePasted(), pasteReader(), TestParsePasted(), lineResult, LoginIO, pastedResult

### Community 166 - "reference/order.go"
Cohesion: 0.32
Nodes (5): Order, User, NewOrder(), NewUser(), normalizeEmail()

### Community 167 - "time.Time"
Cohesion: 0.38
Nodes (4): stampOf(), ReadTracker, stamp, padGuard

### Community 168 - ".RunPlan"
Cohesion: 0.40
Nodes (4): doNudge(), doPrompt(), Agent, uncheckedSteps()

### Community 169 - "runTUI"
Cohesion: 0.40
Nodes (4): decideTrust(), hasProjectResources(), runTUI(), RunTrustPrompt()

### Community 170 - "fetch.go"
Cohesion: 0.50
Nodes (3): Client, New(), TestGet()

### Community 171 - "StartOptions"
Cohesion: 0.40
Nodes (4): planMode(), discoverLocal(), StartOptions, Asker

### Community 172 - "retryable"
Cohesion: 0.40
Nodes (5): http2Transient(), retryable(), TestRetryableServerError400(), TestRetryable(), TestRetryableTransportErrors()

### Community 173 - "Eval baseline — DESIGN rev 21 harness pass"
Cohesion: 0.40
Nodes (4): Eval baseline — DESIGN rev 21 harness pass, Found by the evals (fixed in rev 21), Reading it, Results

### Community 174 - "fakeOllama"
Cohesion: 0.50
Nodes (3): modelFromShow(), fakeOllama, ollamaShow

### Community 175 - "DetectVerify"
Cohesion: 0.50
Nodes (4): DetectVerify(), fileContains(), makeTargets(), TestDetectVerify()

## Knowledge Gaps
- **141 isolated node(s):** `github.com/adeotek/moca`, `Agent`, `Shell`, `example.com/slug`, `example.com/inventory` (+136 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 421 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **61 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `SPECS.md — moca implemented-state normative spec` connect `SPECS.md — moca implemented-state normative spec` to `go_pkg_strings`, `agent events — TextDelta … TurnEnd/SteeringApplied/Compacted/Resumed`, `tools registry — Tool interface, seven frozen tools, sequential execution in emitted order`, `permissions/shell.go`, `anthropic-messages codec (POST /v1/messages, version header, x-api-key)`, `system prompt — built once per run, stored in the session, never rebuilt`, `internal/agent — system prompt, events, loop, compaction + resume, cross-provider transform, Start wiring`, `MCP Manager — zero start, lazy ensure, busy-guarded idle stop, replay-once rules`, `moca -p "<prompt>" — one-shot agent run (stdin -p form)`, `DESIGN §12 example config fixture (phase-1 decode gate)`, `credential store ~/.config/moca/auth.json (0600, atomic writes, cross-process lock)`, `anthropic provider (api_key only by policy, no OAuth)`, `TUI: Bubble Tea + lipgloss, compact/dense, welcome lines`, `External tools: rtk, graphify, skills ecosystem`, `moca update [--check] (newest published GitHub release, in-place swap)`, `MCP gating — readOnlyHint && !destructiveHint || approve list || session allow; no name heuristics`, `/-dropdown autocomplete (built-ins + prompt templates, ≤8 rows, tab/enter/esc)`, `opencode-go provider (zen gateway)`?**
  _High betweenness centrality (0.318) - this node is a cross-community bridge._
- **What connects `github.com/adeotek/moca`, `Agent`, `Shell` to the rest of the system?**
  _141 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `go_pkg_strings` be split into smaller, more focused modules?**
  _Cohesion score 0.10140809011776754 - nodes in this community are weakly interconnected._
- **Why does `v0.2.0-beta (2026-10-09): Ollama, saved / commands, self-update` connect `v0.2.0-beta (2026-10-09): Ollama, saved / commands, self-update` to `Context manager: token-denominated budgets, compaction, recovery`, `moca login <provider> / TUI /login: credential wizard (OAuth or masked API key)`, `Seven frozen tools for v1 (schema churn = v2 discussion)`, `anthropic provider (api_key only by policy, no OAuth)`, `TUI: Bubble Tea + lipgloss, compact/dense, welcome lines`?**
  _High betweenness centrality (0.038) - this node is a cross-community bridge._
- **Should `SPECS.md — moca implemented-state normative spec` be split into smaller, more focused modules?**
  _Cohesion score 0.0289395070948469 - nodes in this community are weakly interconnected._
- **Why does `Version stamp from git describe --tags --always --dirty` connect `v0.2.0-beta (2026-10-09): Ollama, saved / commands, self-update` to `go_pkg_strings`?**
  _High betweenness centrality (0.036) - this node is a cross-community bridge._
- **Should `testing.T` be split into smaller, more focused modules?**
  _Cohesion score 0.03901319563970167 - nodes in this community are weakly interconnected._