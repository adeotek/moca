# Graph Report - moca  (2026-10-06)

## Corpus Check
- 213 files · ~252,207 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 7 file(s) not represented in the graph (top: (none) 4, .jsonc 2, .toml 1)

## Summary
- 1724 nodes · 6402 edges · 93 communities (64 shown, 29 thin omitted)
- Extraction: 88% EXTRACTED · 11% INFERRED · 0% AMBIGUOUS · INFERRED: 736 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `ed1edcc7`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- start.go
- newScript
- permissions/shell.go
- Phase 4 Plan: Context Manager, Compaction, Resume
- retry_test.go
- anthropic_test.go
- testing.T
- Env
- phase-6-rtk-graphify-ecosystem.md
- NewManager
- cli_test.go
- Phase 3 - TUI Shell, Slash Commands, Status Bar, Steering, Model Switching Implementation Plan
- internal/config — JSONC config parsing, defaults, validation
- SetObjectEntry
- stdioTransport
- model
- Registry
- Phase 2 Plan - Seven Tools, Agent Loop, Permissions, Skills, Session Schema
- Phase 4 context compaction & resume review - pass 1 (2026-10-05)
- Create
- newAgentModel
- Phase 3 TUI review - pass 1 (2026-10-05)
- Agent
- Request
- Plan
- Manager
- Review: phase 6 — rtk, graphify, Skills Ecosystem Compatibility (pass 3)
- Start
- time.Duration
- Model
- session/entry.go
- Review: phase 5 — MCP lazy proxy (pass 1)
- Result
- estimate.go
- Sanitize
- initialize
- charm.land/bubbletea/v2.Msg
- Claude Code skill-creator skill
- encoding/json.RawMessage
- Config
- context.Context
- Phase 5 — MCP Lazy Proxy Implementation Plan
- applyEdit
- runOneShot
- ToolSpec
- Review: phase 5 — MCP lazy proxy (pass 2, adversarial quality)
- Low
- ParseInput
- pagerModel
- skills_test.go
- input_test.go
- run
- Index
- NewRegistry
- Run
- PromptInput
- Find
- quick_validate.py
- NewUnjailed
- extractBuiltins
- Item
- Agent loop (§14: turn contract, steering, maxSteps wrap-up)
- .Run
- ReadTracker
- Model/effort switching (SetModel, SetEffort)
- trustModel
- Claude Code frontend-design skill
- pi mcp-scripting skill
- allowAll
- StartError
- CommandChecker
- AllowAll
- boundedWriter
- fakeCmds
- Compacted
- Resumed
- SteeringApplied
- StreamReset
- TextDelta
- ThinkingDelta
- Warning
- YoloChanged
- EnvError
- Arrange-Act-Assert with xUnit/NSubstitute
- fakeSnap
- rootChecker
- config.Version build stamp
- shell allowlist (shell.allow / DefaultShellAllow)
- Agent.Close — stops MCP servers, closes session writer at shutdown
- TestMCPNoSchemasInPromptAndZeroStarts
- github.com/adeotek/moca

## God Nodes (most connected - your core abstractions)
1. `model` - 47 edges
2. `newScript()` - 46 edges
3. `run()` - 37 edges
4. `startTestWith()` - 35 edges
5. `Phase 4 context compaction & resume review - pass 1 (2026-10-05)` - 33 edges
6. `sseServer()` - 32 edges
7. `Agent` - 30 edges
8. `ReadFile()` - 30 edges
9. `Phase 4 Plan: Context Manager, Compaction, Resume` - 30 edges
10. `Config` - 28 edges

## Surprising Connections (you probably didn't know these)
- `No secrets needed — stdlib-only build` --semantically_similar_to--> `Stdlib-only constraint — hand-rolled HTTP+SSE`  [INFERRED] [semantically similar]
  .github/workflows/ci.yml → docs/plans/phase-1-skeleton-providers.md
- `rtk skill allowlist rule (the wrapped command must also be allowlisted)` --conceptually_related_to--> `rtk cannot launder a non-allowlisted command`  [INFERRED]
  internal/skills/builtin/rtk/SKILL.md → docs/plans/phase-6-rtk-graphify-ecosystem.md
- `parseArgs()` --calls--> `ParseEffort()`  [EXTRACTED]
  cmd/moca/cli.go → internal/llm/types.go
- `TestMCPImportCLI()` --calls--> `Parse()`  [EXTRACTED]
  cmd/moca/cli_test.go → internal/config/config.go
- `TestMCPImportCLI()` --calls--> `run()`  [INFERRED]
  cmd/moca/cli_test.go → internal/tools/read_test.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Agent Loop Flow** — docs_plans_phase_2_tools_agent_loop_agent_loop, docs_plans_phase_2_tools_agent_loop_sequential_tool_execution, docs_plans_phase_2_tools_agent_loop_max_steps_wrap_up, docs_plans_phase_2_tools_agent_loop_jsonl_session, docs_plans_phase_2_tools_agent_loop_transcript_repair [EXTRACTED 1.00]
- **Ecosystem SKILL.md compatibility corpus (graphify, Claude Code, pi, OpenCode)** — internal_skills_testdata_ecosystem_sources, internal_skills_testdata_ecosystem, internal_skills_testdata_ecosystem_graphify_graphify_vendored, internal_skills_testdata_ecosystem_claude_code_skill_creator_vendored, internal_skills_testdata_ecosystem_claude_code_frontend_design_vendored, internal_skills_testdata_ecosystem_pi_pi_subagents_vendored, internal_skills_testdata_ecosystem_pi_mcp_scripting_vendored, internal_skills_testdata_ecosystem_opencode_dotnet_unit_testing_vendored [EXTRACTED 1.00]
- **MCP lazy proxy architecture (JSON-RPC core, transports, index, manager, proxy tool, importer)** — docs_plans_phase_5_mcp_lazy_proxy_jsonrpc_core, docs_plans_phase_5_mcp_lazy_proxy_start_stdio, docs_plans_phase_5_mcp_lazy_proxy_start_http, internal_mcp_index, docs_plans_phase_5_mcp_lazy_proxy_manager, internal_mcp_proxy_tool, docs_plans_phase_5_mcp_lazy_proxy_importer [EXTRACTED 1.00]
- **Model switching, session events and status reporting flow** — docs_plans_phase_3_tui_model_switching, docs_plans_phase_3_tui_model_change_event, docs_plans_phase_3_tui_cross_provider_history_transform, docs_plans_phase_3_tui_anchored_context_estimate, docs_plans_phase_3_tui_status_bar [EXTRACTED 1.00]
- **OAuth subscription access pipeline** — docs_plans_phase_7_oauth_release_oauth_policy_gate, docs_plans_phase_7_oauth_release_provider_credential_seam, docs_plans_phase_7_oauth_release_token_store, docs_plans_phase_7_oauth_release_cross_process_lock, docs_plans_phase_7_oauth_release_pkce_flow, docs_plans_phase_7_oauth_release_headless_paste, docs_plans_phase_7_oauth_release_moca_login [EXTRACTED 1.00]
- **Permission Checker Injection** — docs_plans_phase_2_tools_agent_loop_tool_interface, docs_plans_phase_2_tools_agent_loop_structural_interfaces, docs_plans_phase_2_tools_agent_loop_path_jail, docs_plans_phase_2_tools_agent_loop_shell_analysis, docs_plans_phase_2_tools_agent_loop_project_trust, docs_plans_phase_2_tools_agent_loop_yolo_mode [EXTRACTED 1.00]
- **Phase-1 CI gate set** — _github_workflows_ci, docs_plans_phase_1_skeleton_providers, _github_workflows_ci_ci_gate_set [EXTRACTED 1.00]
- **Phase 3 TUI review series (passes 1-4)** — docs_reviews_2026_10_05_phase_3_tui_review, docs_reviews_2026_10_05_phase_3_tui_pass_2_review, docs_reviews_2026_10_05_phase_3_tui_pass_3_review, docs_reviews_2026_10_05_phase_3_tui_pass_4_review [EXTRACTED 1.00]
- **Phase 4 compaction pipeline (budget, cut, serialize, summarize, agent trigger)** — internal_compact_budget, internal_compact_cut, internal_compact_serialize, internal_compact_summary, internal_agent_compaction [EXTRACTED 1.00]
- **Phase 4 compaction review series (passes 1-3)** — docs_reviews_2026_10_05_phase_4_context_compaction_resume_review, docs_reviews_2026_10_05_phase_4_context_compaction_resume_pass_2_review, docs_reviews_2026_10_06_phase_4_context_compaction_resume_review [EXTRACTED 1.00]
- **rtk unwrap laundering defense across plan and both review passes** — docs_plans_phase_6_rtk_graphify_ecosystem_rtk_laundering_guarantee, docs_plans_phase_6_rtk_graphify_ecosystem_runner_class_unification, docs_reviews_2026_10_06_phase_6_rtk_graphify_ecosystem_finding_2_case_variant_laundering, docs_reviews_2026_10_06_phase_6_rtk_graphify_ecosystem_pass_2_h1_shell_string, docs_reviews_2026_10_06_phase_6_rtk_graphify_ecosystem_finding_1_windows_rtk_gap [EXTRACTED 1.00]
- **Skill Loading Pipeline** — docs_plans_phase_2_tools_agent_loop_skills_loader, docs_plans_phase_2_tools_agent_loop_builtin_extraction, docs_plans_phase_2_tools_agent_loop_path_jail [EXTRACTED 1.00]
- **Synthetic frontmatter edge-case fixtures (CRLF, BOM, folding, quoting, rich keys, long description)** — internal_skills_testdata_ecosystem_synthetic_bom_skill, internal_skills_testdata_ecosystem_synthetic_colon_unquoted_skill, internal_skills_testdata_ecosystem_synthetic_crlf_skill, internal_skills_testdata_ecosystem_synthetic_folded_skill, internal_skills_testdata_ecosystem_synthetic_literal_skill, internal_skills_testdata_ecosystem_synthetic_long_description_skill, internal_skills_testdata_ecosystem_synthetic_rich_keys_skill [EXTRACTED 1.00]
- **TUI pure-core + glue architecture** — docs_plans_phase_3_tui_pure_core_architecture, docs_plans_phase_3_tui_bubble_tea_glue, docs_plans_phase_3_tui_input_state_machine, docs_plans_phase_3_tui_event_pipe [EXTRACTED 1.00]
- **Human-in-the-loop validation before declaring done** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_claude_code_frontend_design_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_review_validation [INFERRED 0.75]
- **Agent lifecycle: build registers proxy, Close stops servers, session writer teardown** — internal_agent_build, internal_agent_close, docs_plans_phase_5_mcp_lazy_proxy_manager, docs_specs_specs_mcp_manager_lifecycle, docs_specs_specs_session_resume [INFERRED 0.85]
- **Phase 4 compaction pipeline (budget → cut → serialize → summarize → event)** — docs_plans_phase_4_context_compaction_resume_compact_budget, docs_plans_phase_4_context_compaction_resume_compact_entry, docs_plans_phase_4_context_compaction_resume_compact_findcut, docs_plans_phase_4_context_compaction_resume_compact_serialize, docs_plans_phase_4_context_compaction_resume_compact_summarysystem, docs_plans_phase_4_context_compaction_resume_compact_compact, docs_plans_phase_4_context_compaction_resume_agent_maybecompact, docs_plans_phase_4_context_compaction_resume_agent_agent_compact, docs_plans_phase_4_context_compaction_resume_agent_compacted [INFERRED 0.85]
- **JSONC comment-preserving edit family (AppendString, SetObjectEntry, pre-pass)** — docs_specs_specs_config_jsonc, docs_specs_specs_config_append_string, internal_config_set_object_entry, docs_specs_specs_mcp_gating, docs_specs_specs_mcp_import [INFERRED 0.85]
- **Resume / --continue context rebuild flow** — docs_plans_phase_4_context_compaction_resume_session_find, docs_plans_phase_4_context_compaction_resume_session_findforworkdir, docs_plans_phase_4_context_compaction_resume_agent_resume, docs_plans_phase_4_context_compaction_resume_agent_build, docs_plans_phase_4_context_compaction_resume_session_messages, docs_plans_phase_4_context_compaction_resume_session_latestcompaction [INFERRED 0.85]
- **Pass-2→4 review hardening ladder: sentinels, bounded waits, replay rules, embedded-secret rules** — internal_mcp_jsonrpc_errtransportdead, internal_mcp_manager_replayonce, internal_mcp_hardening_test [INFERRED 0.85]
- **Parallel subagent fanout with parent synthesis** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_graphify_graphify_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_skill [INFERRED 0.85]
- **Skills referencing bundled reference files for progressive disclosure** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_graphify_graphify_skill, internal_skills_testdata_ecosystem_pi_mcp_scripting_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_skill [INFERRED 0.85]
- **moca extension model (skills + slash commands + shell allowlist + lazy MCP)** — docs_specs_design_skills, docs_specs_design_external_tools, docs_specs_design_mcp_lazy_proxy, docs_specs_design_permissions, docs_specs_design_explicit_non_goals [INFERRED 0.90]
- **Phase 5 review cycle: pass-1/2/3/4 reviews iterated with fixes landing between passes** — docs_reviews_2026_10_06_phase_5_mcp_lazy_proxy, docs_reviews_2026_10_06_phase_5_mcp_lazy_proxy_pass_2, docs_reviews_2026_10_06_phase_5_mcp_lazy_proxy_pass_3 [INFERRED 0.95]

## Communities (93 total, 29 thin omitted)

### Community 0 - "start.go"
Cohesion: 0.09
Nodes (33): Agent loop, Phase 1 skeleton & providers review (2026-10-04), Phase 2 tools & agent loop review (2026-10-05), Review pass 2 (adversarial quality), AppendString(), concat(), nested(), prepEdit() (+25 more)

### Community 1 - "newScript"
Cohesion: 0.07
Nodes (71): scriptServer, newScript(), startTest(), startTestWith(), startWith(), TestAbortWritesSyntheticResults(), TestFailedCallDoesNotCancelRest(), TestLengthStopTruncatedToolCall() (+63 more)

### Community 2 - "permissions/shell.go"
Cohesion: 0.07
Nodes (38): canonical(), Jail, NewJail(), resolveDeep(), setup(), TestJail(), TestJailDanglingSymlinks(), TestJailKeepsNotYetExistingReadOnlyRoot() (+30 more)

### Community 3 - "Phase 4 Plan: Context Manager, Compaction, Resume"
Cohesion: 0.08
Nodes (46): /review-phase slash command, Phase review document (docs/reviews/), Phase 4 Plan: Context Manager, Compaction, Resume, agent.Agent.Compact (manual + automatic compaction), agent.build (shared Start/Resume construction), agent.Compacted event, agent.ErrNothingToCompact, agent.maybeCompact (trigger check + loop guard) (+38 more)

### Community 4 - "retry_test.go"
Cohesion: 0.31
Nodes (15): Event, fail(), ok(), TestMidStreamFailureResetsAndRetriesOnce(), TestMidStreamRetryBacksOffAndNotifies(), TestNoRetryOn400(), testPolicy(), TestRetryAfterCapped() (+7 more)

### Community 5 - "anthropic_test.go"
Cohesion: 0.14
Nodes (38): SplitModel(), TestSplitModel(), Adapter, newAnthropic(), keyCred(), mustJSON(), sseServer(), TestAnthropicBodyHygiene() (+30 more)

### Community 6 - "testing.T"
Cohesion: 0.05
Nodes (63): NormalizeToolID(), TestNormalizeToolID(), TestTransformDropsEmptiedAssistantAndMerges(), TestTransformDropsRedactedForeignThinking(), TestTransformHistory(), TestTransformSameBareIDDifferentProvider(), TransformHistory(), Parse() (+55 more)

### Community 7 - "Env"
Cohesion: 0.11
Nodes (11): readLine(), approve(), decode(), errorf(), Env, countFileLines(), countLines(), guardedWrite() (+3 more)

### Community 8 - "phase-6-rtk-graphify-ecosystem.md"
Cohesion: 0.10
Nodes (15): rtkRunTarget, rtkSub (flags before the subcommand are skipped), rtkTarget, skipWrapper, unwrap (shell analyser), SKILL.md (builtin rtk), internal/skills/testdata/ecosystem corpus, anthropics/skills frontend-design (+7 more)

### Community 9 - "NewManager"
Cohesion: 0.10
Nodes (47): MCPServer, crashingServer(), shServer(), skipWindows(), TestCallTimeoutStopsWedgedServer(), TestHandshakeTimeout(), TestHTTPAbortedStreamReplayRule(), TestIndexSaveMergesConcurrentWriters() (+39 more)

### Community 10 - "cli_test.go"
Cohesion: 0.12
Nodes (38): fakeCompletions(), isolate(), TestExitCodes(), TestInterruptedExit130(), TestMCPImportCLI(), TestNoApproveSkipsProjectInstructions(), TestOneShotDiscardsStdoutOnReset(), TestOneShotMaxStepsExit3() (+30 more)

### Community 11 - "Phase 3 - TUI Shell, Slash Commands, Status Bar, Steering, Model Switching Implementation Plan"
Cohesion: 0.08
Nodes (30): allow-always persistence (shell.allow), Interactive approval prompt (blocking Asker), Bubble Tea glue (internal/tui/app.go), Bubble Tea v2 (charm.land/bubbletea/v2), LoadPrompts / ExpandPrompt (internal/skills/prompts.go), Numbered collapsible tool/thinking items + pager, ParseInput command parsing (internal/tui/commands.go), Paste chips [paste N lines #K] (+22 more)

### Community 12 - "internal/config — JSONC config parsing, defaults, validation"
Cohesion: 0.08
Nodes (30): CI Workflow (gofmt · vet · build · test), CI gate set (gofmt, go vet, go build, go test -race), go.mod (Go version source for setup-go), SPECS.md §2 — CI gate definition, Phase 1 — Skeleton, Protocol Adapters, Streaming, JSONC Config (implementation plan), Adapter interface — Stream(ctx, req, emit), anthropic-messages adapter, ClampEffort / DefaultEffort (+22 more)

### Community 13 - "SetObjectEntry"
Cohesion: 0.24
Nodes (7): scanner, span, nestedEntry(), objectHasKey(), SetObjectEntry(), TestSetObjectEntry(), writeEdited()

### Community 14 - "stdioTransport"
Cohesion: 0.14
Nodes (7): gitClean(), KillProcessGroup(), SetProcessGroup(), KillProcessGroup(), SetProcessGroup(), ring, stdioTransport

### Community 15 - "model"
Cohesion: 0.20
Nodes (3): println(), printlnContent(), model

### Community 16 - "Registry"
Cohesion: 0.17
Nodes (7): Builtins(), Registry, NewRegistry(), TestRegistryDispatch(), TestSchemasFrozen(), echoTool, Tool

### Community 17 - "Phase 2 Plan - Seven Tools, Agent Loop, Permissions, Skills, Session Schema"
Cohesion: 0.10
Nodes (26): Agent Loop (model - tools - model), AGENTS.md / CLAUDE.md Project Instructions, Embedded Built-in Skills Extraction (ExtractBuiltins), DESIGN.md rev 11 Specification, Edit Ladder (exact / whitespace-insensitive / replace_all), Pure-Go Gitignore Matcher, Append-Only JSONL Session Transcript, maxSteps Wrap-Up (tool_choice none) (+18 more)

### Community 18 - "Phase 4 context compaction & resume review - pass 1 (2026-10-05)"
Cohesion: 0.06
Nodes (47): Kind, Context compaction, Crash repair of unanswered tool calls, Phase 4 context compaction & resume review - pass 2 (2026-10-05), Phase 4 context compaction & resume review - pass 1 (2026-10-05), Phase 4 context compaction & resume review - pass 3 (2026-10-06), Cut-point selection (FindCut), Agent (+39 more)

### Community 19 - "Create"
Cohesion: 0.13
Nodes (14): lockFile(), lockFile(), Snapshots, NewSnapshots(), Create(), Writer, lastNewline(), newID() (+6 more)

### Community 20 - "newAgentModel"
Cohesion: 0.19
Nodes (21): key(), keyMsg(), TestCompactEscapeCancels(), TestCompactingBlocksRun(), TestCompactNothing(), TestCompactedAndResumedLines(), TestAllowAlwaysMCPPersists(), TestAllowAlwaysPersists() (+13 more)

### Community 21 - "Phase 3 TUI review - pass 1 (2026-10-05)"
Cohesion: 0.05
Nodes (41): Approval prompt flow, Phase 3 TUI review - pass 2 (2026-10-05), Phase 3 TUI review - pass 3 (2026-10-05), Phase 3 TUI review - pass 4 (2026-10-05), Phase 3 TUI review - pass 1 (2026-10-05), TUI event pipe with pre-start buffering, Exit code mapping (exitFor), Hard mode model switching (+33 more)

### Community 22 - "Agent"
Cohesion: 0.14
Nodes (11): Options, ToolEnd, ToolStart, Agent, Outcome, New(), Response, ToolCall (+3 more)

### Community 23 - "Request"
Cohesion: 0.12
Nodes (20): ContentBlock, Request, ToolResult, post(), anthropicOverflow(), newHTTPError(), openaiOverflow(), priorReasoningText() (+12 more)

### Community 24 - "Plan"
Cohesion: 0.14
Nodes (17): TestImportRewritesEmbeddedSecretsInValues(), TestRewriteSecretsValueRuleAlone(), DiscoverSources(), embeddedSecret(), envName(), literalSecretFields(), ParseSource(), Plan() (+9 more)

### Community 26 - "Review: phase 6 — rtk, graphify, Skills Ecosystem Compatibility (pass 3)"
Cohesion: 0.13
Nodes (14): 1. `rtkSelf` classifies native-utility proxies as plain `rtk`, so `rtk find -exec …` (and `rg --pre`, `ast-grep -U`, `tree -o`) bypass a user's allowlist — Confirmed, 2. Mutating rtk-own subcommands (`init -g`, `trust -y`, `config`, `learn --write-rules`) are allowed with no prompt and write outside the jail — Confirmed (analyser behaviour); effects per `--help`, not executed, 3. Frontmatter block-scalar headers with chomping/indent indicators or a trailing comment (`>+`, `|+`, `>2`, `> # note`) silently become the description — Confirmed, Checks run, Earlier findings, Findings, High, Low (+6 more)

### Community 27 - "Start"
Cohesion: 0.15
Nodes (18): setup, GitState(), Platform(), build(), StartOptions, prepare(), Start(), TestPathsHonourXDG() (+10 more)

### Community 28 - "time.Duration"
Cohesion: 0.19
Nodes (10): Retry, http2Transient(), retryable(), DefaultRetryPolicy(), RetryNotice, TestRetryable(), TestRetryableTransportErrors(), Options (+2 more)

### Community 29 - "Model"
Cohesion: 0.09
Nodes (8): savedModel, Status, Agent, Effort, ParseEffort(), TestParseEffort(), Model, supportedEfforts()

### Community 30 - "session/entry.go"
Cohesion: 0.16
Nodes (13): TurnEnd, Result, StopReason, Usage, anthropicStop(), Entry, ModelChange, Compaction (+5 more)

### Community 31 - "Review: phase 5 — MCP lazy proxy (pass 1)"
Cohesion: 0.11
Nodes (18): 1. `Manager.Call`'s dead-transport retry on an HTTP transport can restart the same server concurrently with itself across `Search`/`Describe`/`Call` from different goroutines — Plausible, 1. The retry predicate string-matches error text; a server error message containing the substring "exited" (or "session expired") spuriously restarts a healthy server, and any in-flight sibling calls on the same transport are failed — Confirmed, 2. `Index.LoadIndex` silently ignores a corrupt file's parse error, so a partially-parsed index can serve stale entries — Plausible, 2. The stdio transport closes the process at Close (`stdin` close, then kill), but `Call`'s write-error retry branch can return `ctx.Err()` without ever surfacing the exit error, skipping the exactly-once retry the plan promises — Confirmed, 3. `envName` can collide two distinct env/headers keys onto one export variable — Plausible, 4. `Run`'s search output truncates descriptions by bytes, not runes — Plausible (edge), 5. `stdioTransport.Close`'s `<-t.done` waits for the child exit unboundedly after `SIGKILL` — Plausible (edge), Checks run (+10 more)

### Community 32 - "Result"
Cohesion: 0.18
Nodes (10): cutRunes(), errResult(), isTrue(), TestCutRunesSafe(), runePrefix(), runeSuffix(), TestTruncate(), TestTruncateLongLines() (+2 more)

### Community 33 - "estimate.go"
Cohesion: 0.46
Nodes (5): MessageChars(), RequestChars(), TestEstimate(), Tokens(), UsageTokens()

### Community 34 - "Sanitize"
Cohesion: 0.16
Nodes (10): ShellOutput, AutoAllow(), Answer, Question, approvalPrompt(), firstLineOf(), TestApprovalPromptSanitized(), Sanitize() (+2 more)

### Community 35 - "initialize"
Cohesion: 0.14
Nodes (16): TestResolveEnv(), ResolveEnv(), initialize(), abortingHTTPServer(), TestHTTPCloseBounded(), TestKeepNameWindows(), startHTTP(), fakeHTTPServer() (+8 more)

### Community 36 - "charm.land/bubbletea/v2.Msg"
Cohesion: 0.18
Nodes (8): Event, Asker, newAsker(), newEventPipe(), TestEventPipeBuffersUntilStarted(), TestEventPipeNonBlockingAndFIFO(), agentEventMsg, eventPipe

### Community 37 - "Claude Code skill-creator skill"
Cohesion: 0.16
Nodes (13): Skill description (trigger) optimization, Skill eval/iterate loop (with-skill vs baseline), skill-creator Apache 2.0 License (Anthropic), Claude Code skill-creator skill, SKILL.md anatomy + bundled resources layout, Constrained query expansion against graph vocab, Progressive Disclosure (three-level skill loading), graphify query/path/explain reference (+5 more)

### Community 38 - "encoding/json.RawMessage"
Cohesion: 0.18
Nodes (8): CallResult, callResult, client, Content, response, rpcError, Tool, transport

### Community 39 - "Config"
Cohesion: 0.12
Nodes (14): ContextConfig, MCPConfig, ModelOverride, ProviderConfig, ShellConfig, SnapshotConfig, Default(), Config (+6 more)

### Community 40 - "context.Context"
Cohesion: 0.24
Nodes (5): fakeSum, responseID(), TestResponseID(), readEvents(), httpTransport

### Community 41 - "Phase 5 — MCP Lazy Proxy Implementation Plan"
Cohesion: 0.07
Nodes (36): Phase 5 — MCP Lazy Proxy Implementation Plan, internal/mcp client — initialize, paged listTools, callTool, internal/mcp importer — DiscoverSources/ParseSource/RewriteSecrets/Plan, internal/mcp jsonrpc.go — request/response/rpcError types, transport interface, internal/mcp Manager — lazy lifecycle, idle stop, IndexAll, internal/mcp ProxyTool — Spec/run/Allowed gating, internal/mcp sse.go readEvents — minimal SSE reader, internal/mcp startHTTP — streamable HTTP transport (+28 more)

### Community 42 - "applyEdit"
Cohesion: 0.23
Nodes (14): applyEdit(), formatDiff(), indentOf(), joinInts(), lineOf(), matchLines(), replaceExact(), replaceFuzzy() (+6 more)

### Community 43 - "runOneShot"
Cohesion: 0.24
Nodes (9): exitFor(), firstLine(), resolveResume(), runOneShot(), decideTrust(), hasProjectResources(), runTUI(), Slug() (+1 more)

### Community 44 - "ToolSpec"
Cohesion: 0.20
Nodes (6): ToolSpec, MCPSpec(), editTool, lsTool, shellTool, writeTool

### Community 45 - "Review: phase 5 — MCP lazy proxy (pass 2, adversarial quality)"
Cohesion: 0.10
Nodes (21): 1. A streamable-HTTP server that opens an SSE response and stalls hangs `mcp` calls indefinitely — Confirmed, 1. `Notify` returns success for any HTTP status, masking a dead session at the worst moment — Confirmed, 2. `envName` collisions: two distinct keys can map to the same exported variable — Confirmed, 2. `moca mcp import` copies literal secrets that live outside `env`/`headers` — Confirmed, 3. A connection-aborted SSE response skips the designed restart-once — Confirmed, 3. Search-result description truncation splits UTF-8 runes — Confirmed, 4. `Manager.Close` concurrent with a failing in-flight call respawns a server after shutdown — Confirmed by trace, 5. Error paths of both transports have no unit tests — Confirmed (+13 more)

### Community 46 - "Low"
Cohesion: 0.09
Nodes (23): 1. stdio exit handling races `cmd.Wait()` against the stdout reader: real replies and the server's own diagnostics can be lost — Confirmed, 1. The retry-once replays a `tools/call` that may already have executed — Confirmed (plan-prescribed behaviour), 2. An explicit `"args": null` is sent to the server as `"arguments": null` — Confirmed, 2. `moca mcp import` still copies literal credentials held in `env`/`headers` values whose key is not secret-shaped, and prints them in the preview — Confirmed, 3. HTTP transport `Close` has no bound short of the 15-minute client timeout, and runs under the server lock — Confirmed, 3. `Search` hides a failing server's error and re-spawns it on every search — Confirmed, 4. stdio handshake/calls and the pagination loop have no bound other than ctx — Confirmed by reading, 5. `Index.Save` is not safe across concurrent moca processes — Plausible (+15 more)

### Community 47 - "ParseInput"
Cohesion: 0.40
Nodes (6): Prompt, HelpText(), ParseInput(), TestParseInput(), InputKind, Parsed

### Community 48 - "pagerModel"
Cohesion: 0.20
Nodes (3): newPager(), TestTeaAPI(), pagerModel

### Community 49 - "skills_test.go"
Cohesion: 0.15
Nodes (15): TestEcosystemSkillsLoadUnchanged(), closingQuote(), ParseFrontmatter(), scalar(), TestFrontmatter(), TestFrontmatterBlockAndQuoteEdges(), TestFrontmatterBlockIndicatorsAndComments(), LoadPrompts() (+7 more)

### Community 50 - "input_test.go"
Cohesion: 0.30
Nodes (11): NewInput(), TestCollapseUsesExactOffset(), TestCtrlC(), TestEditedMarkerSentLiterally(), TestHistory(), TestHistoryRecallChipsUnsafeContent(), TestMarkerNeverCollidesWithTypedText(), TestPasteChipKeepsContent() (+3 more)

### Community 51 - "run"
Cohesion: 0.35
Nodes (8): parseArgs(), TestParseArgs(), main(), run(), confirm(), runMCP(), runMCPImport(), runMCPIndex()

### Community 52 - "Index"
Cohesion: 0.18
Nodes (8): Rank(), TestRank(), words(), Annotations, Hit, Index, IndexEntry, IndexTool

### Community 53 - "NewRegistry"
Cohesion: 0.33
Nodes (11): NewRegistry(), mustCfg(), TestBaseURLPrecedence(), TestBuiltinModelNeedsProtocol(), TestBuiltinProviderNewModelWithModelProtocol(), TestCustomProviderAndLazyMissingKey(), TestNoSessionHeaderOutsideOpenCodeGo(), TestOpenCodeGoSessionHeader() (+3 more)

### Community 54 - "Run"
Cohesion: 0.28
Nodes (6): newModel(), Run(), runResult(), TestRunResultMapping(), AppOptions, compactDoneMsg

### Community 55 - "PromptInput"
Cohesion: 0.22
Nodes (9): PromptInput, ServerLine, BuildSystemPrompt(), oneLine(), TestBuildSystemPrompt(), Instruction, LoadInstructions(), TestLoadInstructions() (+1 more)

### Community 56 - "Find"
Cohesion: 0.22
Nodes (10): Find(), FindForWorkdir(), header(), idOf(), listNewestFirst(), TestFind(), TestFindForWorkdirViaSymlink(), TestFindMtimeTieDeterministic() (+2 more)

### Community 58 - "NewUnjailed"
Cohesion: 0.29
Nodes (4): Agent, NewUnjailed(), TestUnjailedAndAllowAll(), Unjailed

### Community 59 - "extractBuiltins"
Cohesion: 0.67
Nodes (3): builtinHash(), extractBuiltins(), TestExtractBuiltinsReExtractsOnContentChange()

### Community 64 - "Agent loop (§14: turn contract, steering, maxSteps wrap-up)"
Cohesion: 0.40
Nodes (4): Agent loop (§14: turn contract, steering, maxSteps wrap-up), Context manager compaction (budgets, cut points, structured summary, overflow recovery), Cross-provider thinking replay transform + NormalizeToolID, Session resume (--resume/--continue, repair, single-writer flock)

### Community 65 - ".Run"
Cohesion: 0.24
Nodes (6): globToRegexp(), ignored(), parseIgnore(), TestIgnore(), ignoreRule, searchTool

### Community 66 - "ReadTracker"
Cohesion: 0.60
Nodes (3): stampOf(), ReadTracker, stamp

### Community 68 - "Model/effort switching (SetModel, SetEffort)"
Cohesion: 0.50
Nodes (4): /hard toggle (modelHard + high effort), model_change session entry, Model/effort switching (SetModel, SetEffort), NormalizeToolID (deterministic tool-call id normalization)

### Community 70 - "Claude Code frontend-design skill"
Cohesion: 0.50
Nodes (3): AI-generated design default tells, frontend-design Apache 2.0 License, Claude Code frontend-design skill

### Community 71 - "pi mcp-scripting skill"
Cohesion: 0.67
Nodes (4): jev.evaluate reference, jev.evaluate semantic scripting gate, mcpScript tools API, pi mcp-scripting skill

## Ambiguous Edges - Review These
- `writer.go` → `Single-writer session locking`  [AMBIGUOUS]
  docs/reviews/2026-10-06-phase-4-context-compaction-resume.md · relation: conceptually_related_to
- `Model catalog (builtinCatalog)` → `Thinking modes none|budget|adaptive|openai`  [AMBIGUOUS]
  docs/plans/phase-1-skeleton-providers.md · relation: conceptually_related_to

## Knowledge Gaps
- **135 isolated node(s):** `usageError`, `github.com/adeotek/moca`, `claudeServer`, `opencodeServer`, `request` (+130 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 260 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **29 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **What is the exact relationship between `writer.go` and `Single-writer session locking`?**
  _Edge tagged AMBIGUOUS (relation: conceptually_related_to) - confidence is low._
- **Why does `moca DESIGN.md — v1 contract (rev 11)` connect `Phase 4 Plan: Context Manager, Compaction, Resume` to `start.go`, `phase-6-rtk-graphify-ecosystem.md`?**
  _High betweenness centrality (0.079) - this node is a cross-community bridge._
- **What connects `usageError`, `github.com/adeotek/moca`, `claudeServer` to the rest of the system?**
  _135 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `start.go` be split into smaller, more focused modules?**
  _Cohesion score 0.093734335839599 - nodes in this community are weakly interconnected._
- **What is the exact relationship between `Model catalog (builtinCatalog)` and `Thinking modes none|budget|adaptive|openai`?**
  _Edge tagged AMBIGUOUS (relation: conceptually_related_to) - confidence is low._
- **Why does `Review pass 2 (adversarial quality)` connect `start.go` to `Phase 5 — MCP Lazy Proxy Implementation Plan`, `Phase 4 Plan: Context Manager, Compaction, Resume`, `Review: phase 5 — MCP lazy proxy (pass 2, adversarial quality)`?**
  _High betweenness centrality (0.045) - this node is a cross-community bridge._
- **Should `newScript` be split into smaller, more focused modules?**
  _Cohesion score 0.07450444292549556 - nodes in this community are weakly interconnected._