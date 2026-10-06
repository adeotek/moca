# Graph Report - moca  (2026-10-06)

## Corpus Check
- 45 files · ~248,506 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1706 nodes · 6378 edges · 97 communities (68 shown, 29 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 735 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- Entry Points & Phase Reviews
- Agent Test Harness
- Path Jail & Shell Syntax
- Review Pipeline & Compaction API
- Retry, Errors & Events
- Anthropic Adapter & Model Resolution
- History Transform & Catalog Tests
- Tool Results & Call Plumbing
- Phase 6: rtk & Ecosystem
- MCP Lifecycle & Hardening Tests
- CLI & Tool Test Suites
- TUI Interaction & Prompt Templates
- CI Gates & Provider Adapters
- Config Parsing & Defaults
- Process & JSON-RPC Primitives
- TUI Model Plumbing
- Tool Registry & Specs
- Agent Loop & Tooling Design
- Compaction Cut & Serialization
- Session Locks & Snapshots
- TUI Compact & Approval Tests
- Input Handling & Sanitizing
- Agent Core Methods
- Protocol Types & Codecs
- MCP Import & Secret Rewriting
- MCP Index & Ranking
- Compaction Budget & Triggers
- Agent Startup & Paths
- Status Bar Formatting
- Model & Effort Control
- Usage Accounting & Session Entries
- Phase 5 Pass-1 Review
- Proxy Tool & Rune-Safe Cuts
- Token Estimation & Rebuilds
- Approval UI & Output Safety
- MCP HTTP Transport Tests
- Event Bridge & Askers
- Skill-Creator & Graphify Skills
- Agent Control Surface
- Config Model & Validation
- MCP Proxy Plan & Reviews
- MCP Transports & Importer
- Edit Application Internals
- Front-End Entry & Trust
- Spec Concepts: Tools & Permissions
- Phase 5 Pass-2 Review
- Phase 5 Pass-3 Review
- Prompt Templates & Commands
- TUI Pager
- Built-in Skills & Discovery
- Input State Machine Tests
- CLI Argument Parsing
- MCP Client & Index Entries
- Provider Registry Tests
- Phase 5 Pass-3 Fix List
- System Prompt & Instructions
- Session Finding
- Skill Validator Script
- Yolo Overrides
- Phase 5 Pass-2 Sections
- Shell Env Filtering
- Resume Behavior Tests
- TUI Transcript Items
- Summarizer Serialization
- Spec: Loop & Context
- Gitignore Matcher
- Read Tracking
- Model Overrides & Costing
- Model Switching Design
- Trust Prompt Model
- Frontend-Design Skill
- pi mcp-scripting Skill
- AllowAll Test Helper
- Start Errors
- Command Checker Interface
- AllowAll Checker
- Bounded Output Writer
- Shell Test Fakes
- Compacted Event
- Resumed Event
- Steering Event
- StreamReset Event
- TextDelta Event
- ThinkingDelta Event
- Warning Event
- YoloChanged Event
- Env Errors
- dotnet-unit-testing Skill
- Write Guard Test Fakes
- Root Checker Fakes
- Version Build Stamp
- Shell Allowlist Concept
- Agent Close
- MCP No-Schema Test
- Go Module Path

## God Nodes (most connected - your core abstractions)
1. `model` - 47 edges
2. `newScript()` - 46 edges
3. `run()` - 37 edges
4. `startTestWith()` - 35 edges
5. `Phase 4 context compaction & resume review - pass 1 (2026-10-05)` - 33 edges
6. `sseServer()` - 32 edges
7. `Phase 4 Plan: Context Manager, Compaction, Resume` - 30 edges
8. `Agent` - 30 edges
9. `ReadFile()` - 30 edges
10. `Config` - 28 edges

## Surprising Connections (you probably didn't know these)
- `No secrets needed — stdlib-only build` --semantically_similar_to--> `Stdlib-only constraint — hand-rolled HTTP+SSE`  [INFERRED] [semantically similar]
  .github/workflows/ci.yml → docs/plans/phase-1-skeleton-providers.md
- `rtk skill allowlist rule (the wrapped command must also be allowlisted)` --conceptually_related_to--> `rtk cannot launder a non-allowlisted command`  [INFERRED]
  internal/skills/builtin/rtk/SKILL.md → docs/plans/phase-6-rtk-graphify-ecosystem.md
- `Phase 1 — Skeleton, Protocol Adapters, Streaming, JSONC Config (implementation plan)` --references--> `go.mod (Go version source for setup-go)`  [EXTRACTED]
  docs/plans/phase-1-skeleton-providers.md → .github/workflows/ci.yml
- `Phase review document (docs/reviews/)` --references--> `moca DESIGN.md — v1 contract (rev 11)`  [INFERRED]
  .claude/commands/review-phase.md → docs/specs/DESIGN.md
- `/review-phase slash command` --references--> `Phase 4 Plan: Context Manager, Compaction, Resume`  [INFERRED]
  .claude/commands/review-phase.md → docs/plans/phase-4-context-compaction-resume.md

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Agent Loop Flow** — docs_plans_phase_2_tools_agent_loop_agent_loop, docs_plans_phase_2_tools_agent_loop_sequential_tool_execution, docs_plans_phase_2_tools_agent_loop_max_steps_wrap_up, docs_plans_phase_2_tools_agent_loop_jsonl_session, docs_plans_phase_2_tools_agent_loop_transcript_repair [EXTRACTED 1.00]
- **Model switching, session events and status reporting flow** — docs_plans_phase_3_tui_model_switching, docs_plans_phase_3_tui_model_change_event, docs_plans_phase_3_tui_cross_provider_history_transform, docs_plans_phase_3_tui_anchored_context_estimate, docs_plans_phase_3_tui_status_bar [EXTRACTED 1.00]
- **OAuth subscription access pipeline** — docs_plans_phase_7_oauth_release_oauth_policy_gate, docs_plans_phase_7_oauth_release_provider_credential_seam, docs_plans_phase_7_oauth_release_token_store, docs_plans_phase_7_oauth_release_cross_process_lock, docs_plans_phase_7_oauth_release_pkce_flow, docs_plans_phase_7_oauth_release_headless_paste, docs_plans_phase_7_oauth_release_moca_login [EXTRACTED 1.00]
- **Permission Checker Injection** — docs_plans_phase_2_tools_agent_loop_tool_interface, docs_plans_phase_2_tools_agent_loop_structural_interfaces, docs_plans_phase_2_tools_agent_loop_path_jail, docs_plans_phase_2_tools_agent_loop_shell_analysis, docs_plans_phase_2_tools_agent_loop_project_trust, docs_plans_phase_2_tools_agent_loop_yolo_mode [EXTRACTED 1.00]
- **Phase-1 CI gate set** — _github_workflows_ci, docs_plans_phase_1_skeleton_providers, _github_workflows_ci_ci_gate_set [EXTRACTED 1.00]
- **Phase 3 TUI review series (passes 1-4)** — docs_reviews_2026_10_05_phase_3_tui_review, docs_reviews_2026_10_05_phase_3_tui_pass_2_review, docs_reviews_2026_10_05_phase_3_tui_pass_3_review, docs_reviews_2026_10_05_phase_3_tui_pass_4_review [EXTRACTED 1.00]
- **Phase 4 compaction pipeline (budget, cut, serialize, summarize, agent trigger)** — internal_compact_budget, internal_compact_cut, internal_compact_serialize, internal_compact_summary, internal_agent_compaction [EXTRACTED 1.00]
- **Phase 4 compaction review series (passes 1-3)** — docs_reviews_2026_10_05_phase_4_context_compaction_resume_review, docs_reviews_2026_10_05_phase_4_context_compaction_resume_pass_2_review, docs_reviews_2026_10_06_phase_4_context_compaction_resume_review [EXTRACTED 1.00]
- **Skill Loading Pipeline** — docs_plans_phase_2_tools_agent_loop_skills_loader, docs_plans_phase_2_tools_agent_loop_builtin_extraction, docs_plans_phase_2_tools_agent_loop_path_jail [EXTRACTED 1.00]
- **TUI pure-core + glue architecture** — docs_plans_phase_3_tui_pure_core_architecture, docs_plans_phase_3_tui_bubble_tea_glue, docs_plans_phase_3_tui_input_state_machine, docs_plans_phase_3_tui_event_pipe [EXTRACTED 1.00]
- **Phase 4 compaction pipeline (budget → cut → serialize → summarize → event)** — docs_plans_phase_4_context_compaction_resume_compact_budget, docs_plans_phase_4_context_compaction_resume_compact_entry, docs_plans_phase_4_context_compaction_resume_compact_findcut, docs_plans_phase_4_context_compaction_resume_compact_serialize, docs_plans_phase_4_context_compaction_resume_compact_summarysystem, docs_plans_phase_4_context_compaction_resume_compact_compact, docs_plans_phase_4_context_compaction_resume_agent_maybecompact, docs_plans_phase_4_context_compaction_resume_agent_agent_compact, docs_plans_phase_4_context_compaction_resume_agent_compacted [INFERRED 0.85]
- **Resume / --continue context rebuild flow** — docs_plans_phase_4_context_compaction_resume_session_find, docs_plans_phase_4_context_compaction_resume_session_findforworkdir, docs_plans_phase_4_context_compaction_resume_agent_resume, docs_plans_phase_4_context_compaction_resume_agent_build, docs_plans_phase_4_context_compaction_resume_session_messages, docs_plans_phase_4_context_compaction_resume_session_latestcompaction [INFERRED 0.85]
- **Pass-2→4 review hardening ladder: sentinels, bounded waits, replay rules, embedded-secret rules** — internal_mcp_jsonrpc_errtransportdead, internal_mcp_manager_replayonce, internal_mcp_hardening_test [INFERRED 0.85]
- **moca extension model (skills + slash commands + shell allowlist + lazy MCP)** — docs_specs_design_skills, docs_specs_design_external_tools, docs_specs_design_mcp_lazy_proxy, docs_specs_design_permissions, docs_specs_design_explicit_non_goals [INFERRED 0.90]
- **Phase 5 review cycle: pass-1/2/3/4 reviews iterated with fixes landing between passes** — docs_reviews_2026_10_06_phase_5_mcp_lazy_proxy, docs_reviews_2026_10_06_phase_5_mcp_lazy_proxy_pass_2, docs_reviews_2026_10_06_phase_5_mcp_lazy_proxy_pass_3 [INFERRED 0.95]
- **MCP lazy proxy architecture (JSON-RPC core, transports, index, manager, proxy tool, importer)** — docs_plans_phase_5_mcp_lazy_proxy_jsonrpc_core, docs_plans_phase_5_mcp_lazy_proxy_start_stdio, docs_plans_phase_5_mcp_lazy_proxy_start_http, internal_mcp_index, docs_plans_phase_5_mcp_lazy_proxy_manager, internal_mcp_proxy_tool, docs_plans_phase_5_mcp_lazy_proxy_importer [EXTRACTED 1.00]
- **JSONC comment-preserving edit family (AppendString, SetObjectEntry, pre-pass)** — docs_specs_specs_config_jsonc, docs_specs_specs_config_append_string, internal_config_set_object_entry, docs_specs_specs_mcp_gating, docs_specs_specs_mcp_import [INFERRED 0.85]
- **Agent lifecycle: build registers proxy, Close stops servers, session writer teardown** — internal_agent_build, internal_agent_close, docs_plans_phase_5_mcp_lazy_proxy_manager, docs_specs_specs_mcp_manager_lifecycle, docs_specs_specs_session_resume [INFERRED 0.85]
- **Ecosystem SKILL.md compatibility corpus (graphify, Claude Code, pi, OpenCode)** — internal_skills_testdata_ecosystem_sources, internal_skills_testdata_ecosystem, internal_skills_testdata_ecosystem_graphify_graphify_vendored, internal_skills_testdata_ecosystem_claude_code_skill_creator_vendored, internal_skills_testdata_ecosystem_claude_code_frontend_design_vendored, internal_skills_testdata_ecosystem_pi_pi_subagents_vendored, internal_skills_testdata_ecosystem_pi_mcp_scripting_vendored, internal_skills_testdata_ecosystem_opencode_dotnet_unit_testing_vendored [EXTRACTED 1.00]
- **Synthetic frontmatter edge-case fixtures (CRLF, BOM, folding, quoting, rich keys, long description)** — internal_skills_testdata_ecosystem_synthetic_bom_skill, internal_skills_testdata_ecosystem_synthetic_colon_unquoted_skill, internal_skills_testdata_ecosystem_synthetic_crlf_skill, internal_skills_testdata_ecosystem_synthetic_folded_skill, internal_skills_testdata_ecosystem_synthetic_literal_skill, internal_skills_testdata_ecosystem_synthetic_long_description_skill, internal_skills_testdata_ecosystem_synthetic_rich_keys_skill [EXTRACTED 1.00]
- **rtk unwrap laundering defense across plan and both review passes** — docs_plans_phase_6_rtk_graphify_ecosystem_rtk_laundering_guarantee, docs_plans_phase_6_rtk_graphify_ecosystem_runner_class_unification, docs_reviews_2026_10_06_phase_6_rtk_graphify_ecosystem_finding_2_case_variant_laundering, docs_reviews_2026_10_06_phase_6_rtk_graphify_ecosystem_pass_2_h1_shell_string, docs_reviews_2026_10_06_phase_6_rtk_graphify_ecosystem_finding_1_windows_rtk_gap [EXTRACTED 1.00]
- **Skills referencing bundled reference files for progressive disclosure** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_graphify_graphify_skill, internal_skills_testdata_ecosystem_pi_mcp_scripting_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_skill [INFERRED 0.85]
- **Parallel subagent fanout with parent synthesis** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_graphify_graphify_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_skill [INFERRED 0.85]
- **Human-in-the-loop validation before declaring done** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_claude_code_frontend_design_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_review_validation [INFERRED 0.75]

## Communities (97 total, 29 thin omitted)

### Community 0 - "Entry Points & Phase Reviews"
Cohesion: 0.08
Nodes (50): Agent loop, Approval prompt flow, Context compaction, Crash repair of unanswered tool calls, Phase 1 skeleton & providers review (2026-10-04), Phase 2 tools & agent loop review (2026-10-05), Phase 3 TUI review - pass 2 (2026-10-05), Phase 3 TUI review - pass 3 (2026-10-05) (+42 more)

### Community 1 - "Agent Test Harness"
Cohesion: 0.11
Nodes (54): scriptServer, newScript(), startTest(), startTestWith(), startWith(), TestAbortWritesSyntheticResults(), TestFailedCallDoesNotCancelRest(), TestLengthStopTruncatedToolCall() (+46 more)

### Community 2 - "Path Jail & Shell Syntax"
Cohesion: 0.07
Nodes (39): canonical(), Jail, NewJail(), resolveDeep(), setup(), TestJail(), TestJailDanglingSymlinks(), TestJailKeepsNotYetExistingReadOnlyRoot() (+31 more)

### Community 3 - "Review Pipeline & Compaction API"
Cohesion: 0.08
Nodes (46): /review-phase slash command, Phase review document (docs/reviews/), Phase 4 Plan: Context Manager, Compaction, Resume, agent.Agent.Compact (manual + automatic compaction), agent.build (shared Start/Resume construction), agent.Compacted event, agent.ErrNothingToCompact, agent.maybeCompact (trigger check + loop guard) (+38 more)

### Community 4 - "Retry, Errors & Events"
Cohesion: 0.09
Nodes (36): Retry, Event, http2Transient(), newHTTPError(), retryable(), DefaultRetryPolicy(), RetryNotice, fail() (+28 more)

### Community 5 - "Anthropic Adapter & Model Resolution"
Cohesion: 0.13
Nodes (40): SplitModel(), TestSplitModel(), Adapter, newAnthropic(), keyCred(), mustJSON(), sseServer(), TestAnthropicBodyHygiene() (+32 more)

### Community 6 - "History Transform & Catalog Tests"
Cohesion: 0.06
Nodes (46): TestResumeNotASession(), NormalizeToolID(), TestNormalizeToolID(), TestTransformDropsEmptiedAssistantAndMerges(), TestTransformDropsRedactedForeignThinking(), TestTransformHistory(), TestTransformSameBareIDDifferentProvider(), TransformHistory() (+38 more)

### Community 7 - "Tool Results & Call Plumbing"
Cohesion: 0.09
Nodes (16): ToolEnd, CallResult, readLine(), approve(), decode(), errorf(), Env, PathChecker (+8 more)

### Community 8 - "Phase 6: rtk & Ecosystem"
Cohesion: 0.10
Nodes (15): rtkRunTarget, rtkSub (flags before the subcommand are skipped), rtkTarget, skipWrapper, unwrap (shell analyser), SKILL.md (builtin rtk), internal/skills/testdata/ecosystem corpus, anthropics/skills frontend-design (+7 more)

### Community 9 - "MCP Lifecycle & Hardening Tests"
Cohesion: 0.12
Nodes (39): MCPServer, crashingServer(), shServer(), skipWindows(), TestCallTimeoutStopsWedgedServer(), TestHandshakeTimeout(), TestHTTPAbortedStreamReplayRule(), TestIndexSaveMergesConcurrentWriters() (+31 more)

### Community 10 - "CLI & Tool Test Suites"
Cohesion: 0.12
Nodes (38): fakeCompletions(), isolate(), TestExitCodes(), TestInterruptedExit130(), TestMCPImportCLI(), TestNoApproveSkipsProjectInstructions(), TestOneShotDiscardsStdoutOnReset(), TestOneShotMaxStepsExit3() (+30 more)

### Community 11 - "TUI Interaction & Prompt Templates"
Cohesion: 0.08
Nodes (30): allow-always persistence (shell.allow), Interactive approval prompt (blocking Asker), Bubble Tea glue (internal/tui/app.go), Bubble Tea v2 (charm.land/bubbletea/v2), LoadPrompts / ExpandPrompt (internal/skills/prompts.go), Numbered collapsible tool/thinking items + pager, ParseInput command parsing (internal/tui/commands.go), Paste chips [paste N lines #K] (+22 more)

### Community 12 - "CI Gates & Provider Adapters"
Cohesion: 0.08
Nodes (30): CI Workflow (gofmt · vet · build · test), CI gate set (gofmt, go vet, go build, go test -race), go.mod (Go version source for setup-go), SPECS.md §2 — CI gate definition, Phase 1 — Skeleton, Protocol Adapters, Streaming, JSONC Config (implementation plan), Adapter interface — Stream(ctx, req, emit), anthropic-messages adapter, ClampEffort / DefaultEffort (+22 more)

### Community 13 - "Config Parsing & Defaults"
Cohesion: 0.07
Nodes (32): scanner, span, Parse(), TestDefaultsApplied(), TestEnvRefs(), TestRetentionZeroMeansForever(), TestSpecExampleDecodesIntact(), TestSyntaxErrorReportsLineCol() (+24 more)

### Community 14 - "Process & JSON-RPC Primitives"
Cohesion: 0.08
Nodes (15): fakeSum, responseID(), TestResponseID(), readEvents(), callResult, gitClean(), KillProcessGroup(), SetProcessGroup() (+7 more)

### Community 15 - "TUI Model Plumbing"
Cohesion: 0.18
Nodes (4): println(), printlnContent(), compactDoneMsg, model

### Community 16 - "Tool Registry & Specs"
Cohesion: 0.08
Nodes (17): ToolSpec, TestProxySpecFrozen(), Builtins(), MCPSpec(), Registry, NewRegistry(), TestRegistryDispatch(), TestSchemasFrozen() (+9 more)

### Community 17 - "Agent Loop & Tooling Design"
Cohesion: 0.10
Nodes (26): Agent Loop (model - tools - model), AGENTS.md / CLAUDE.md Project Instructions, Embedded Built-in Skills Extraction (ExtractBuiltins), DESIGN.md rev 11 Specification, Edit Ladder (exact / whitespace-insensitive / replace_all), Pure-Go Gitignore Matcher, Append-Only JSONL Session Transcript, maxSteps Wrap-Up (tool_choice none) (+18 more)

### Community 18 - "Compaction Cut & Serialization"
Cohesion: 0.18
Nodes (22): FindCut(), a(), call(), res(), TestFindCutKeepsRecentAtValidPoint(), TestFindCutNeverBetweenCallAndResult(), TestFindCutNothingToDo(), TestFindCutSplitTurn() (+14 more)

### Community 19 - "Session Locks & Snapshots"
Cohesion: 0.13
Nodes (14): lockFile(), lockFile(), Snapshots, NewSnapshots(), Create(), Writer, lastNewline(), newID() (+6 more)

### Community 20 - "TUI Compact & Approval Tests"
Cohesion: 0.19
Nodes (21): key(), keyMsg(), TestCompactEscapeCancels(), TestCompactingBlocksRun(), TestCompactNothing(), TestCompactedAndResumedLines(), TestAllowAlwaysMCPPersists(), TestAllowAlwaysPersists() (+13 more)

### Community 21 - "Input Handling & Sanitizing"
Cohesion: 0.15
Nodes (6): lineCount(), hasControl(), TestSanitize(), TestSanitizeC1(), Input, paste

### Community 22 - "Agent Core Methods"
Cohesion: 0.19
Nodes (8): Options, ToolStart, Agent, Outcome, New(), ToolCall, ToolChoice, runDoneMsg

### Community 23 - "Protocol Types & Codecs"
Cohesion: 0.17
Nodes (14): ContentBlock, Request, Response, StopReason, ToolResult, post(), anthropicOverflow(), anthropicStop() (+6 more)

### Community 24 - "MCP Import & Secret Rewriting"
Cohesion: 0.16
Nodes (18): TestImportRewritesEmbeddedSecretsInValues(), TestRewriteSecretsValueRuleAlone(), DiscoverSources(), embeddedSecret(), envName(), literalSecretFields(), ParseSource(), Plan() (+10 more)

### Community 25 - "MCP Index & Ranking"
Cohesion: 0.23
Nodes (6): Rank(), TestRank(), words(), Manager, Hit, state

### Community 26 - "Compaction Budget & Triggers"
Cohesion: 0.19
Nodes (9): Kind, Agent, summarizerUnavailable(), Budget, NewBudget(), TestBudget(), EntryTokens(), Entry (+1 more)

### Community 27 - "Agent Startup & Paths"
Cohesion: 0.16
Nodes (17): setup, GitState(), Platform(), build(), StartOptions, prepare(), Start(), TestPathsHonourXDG() (+9 more)

### Community 28 - "Status Bar Formatting"
Cohesion: 0.16
Nodes (17): TestGitStatusUsesNoOptionalLocks(), TestStatusWideRunesNeverExceedWidth(), AbbrevEffort(), AbbrevHome(), FmtTokens(), FmtWindow(), GitBranch(), gitStatusArgs() (+9 more)

### Community 29 - "Model & Effort Control"
Cohesion: 0.15
Nodes (7): savedModel, Status, Effort, ParseEffort(), TestParseEffort(), Model, supportedEfforts()

### Community 30 - "Usage Accounting & Session Entries"
Cohesion: 0.17
Nodes (12): TurnEnd, Result, Usage, Entry, ModelChange, header(), Compaction, ErrorInfo (+4 more)

### Community 31 - "Phase 5 Pass-1 Review"
Cohesion: 0.11
Nodes (18): 1. `Manager.Call`'s dead-transport retry on an HTTP transport can restart the same server concurrently with itself across `Search`/`Describe`/`Call` from different goroutines — Plausible, 1. The retry predicate string-matches error text; a server error message containing the substring "exited" (or "session expired") spuriously restarts a healthy server, and any in-flight sibling calls on the same transport are failed — Confirmed, 2. `Index.LoadIndex` silently ignores a corrupt file's parse error, so a partially-parsed index can serve stale entries — Plausible, 2. The stdio transport closes the process at Close (`stdin` close, then kill), but `Call`'s write-error retry branch can return `ctx.Err()` without ever surfacing the exit error, skipping the exactly-once retry the plan promises — Confirmed, 3. `envName` can collide two distinct env/headers keys onto one export variable — Plausible, 4. `Run`'s search output truncates descriptions by bytes, not runes — Plausible (edge), 5. `stdioTransport.Close`'s `<-t.done` waits for the child exit unboundedly after `SIGKILL` — Plausible (edge), Checks run (+10 more)

### Community 32 - "Proxy Tool & Rune-Safe Cuts"
Cohesion: 0.15
Nodes (15): cutRunes(), errResult(), isTrue(), newProxy(), runP(), TestCutRunesSafe(), TestProxyArgErrors(), TestProxyGating() (+7 more)

### Community 33 - "Token Estimation & Rebuilds"
Cohesion: 0.24
Nodes (13): MessageChars(), RequestChars(), TestEstimate(), Tokens(), UsageTokens(), Message, Role, appendUser() (+5 more)

### Community 34 - "Approval UI & Output Safety"
Cohesion: 0.16
Nodes (10): ShellOutput, AutoAllow(), Answer, Question, approvalPrompt(), firstLineOf(), TestApprovalPromptSanitized(), Sanitize() (+2 more)

### Community 35 - "MCP HTTP Transport Tests"
Cohesion: 0.18
Nodes (10): initialize(), abortingHTTPServer(), TestHTTPCloseBounded(), startHTTP(), fakeHTTPServer(), TestHTTPNotifyStatusChecked(), TestHTTPTransport(), TestStdioEarlyExit() (+2 more)

### Community 36 - "Event Bridge & Askers"
Cohesion: 0.16
Nodes (10): Event, Asker, newModel(), Run(), newAsker(), newEventPipe(), TestEventPipeBuffersUntilStarted(), TestEventPipeNonBlockingAndFIFO() (+2 more)

### Community 37 - "Skill-Creator & Graphify Skills"
Cohesion: 0.16
Nodes (13): Skill description (trigger) optimization, Skill eval/iterate loop (with-skill vs baseline), skill-creator Apache 2.0 License (Anthropic), Claude Code skill-creator skill, SKILL.md anatomy + bundled resources layout, Constrained query expansion against graph vocab, Progressive Disclosure (three-level skill loading), graphify query/path/explain reference (+5 more)

### Community 39 - "Config Model & Validation"
Cohesion: 0.18
Nodes (9): ContextConfig, MCPConfig, ShellConfig, SnapshotConfig, Default(), Config, Load(), TestLoadMissingFileIsDefault() (+1 more)

### Community 40 - "MCP Proxy Plan & Reviews"
Cohesion: 0.21
Nodes (12): Phase 5 — MCP Lazy Proxy Implementation Plan, internal/mcp client — initialize, paged listTools, callTool, internal/mcp jsonrpc.go — request/response/rpcError types, transport interface, internal/mcp Manager — lazy lifecycle, idle stop, IndexAll, internal/mcp sse.go readEvents — minimal SSE reader, Review pass 2 (adversarial quality), moca SPECS.md (current implemented state), MCP JSON-RPC 2.0 protocol handling (initialize, tools/list paging, tools/call) (+4 more)

### Community 41 - "MCP Transports & Importer"
Cohesion: 0.16
Nodes (13): internal/mcp importer — DiscoverSources/ParseSource/RewriteSecrets/Plan, internal/mcp startHTTP — streamable HTTP transport, internal/mcp startStdio + FilterEnv/serverEnv, env:VAR lazy credential indirection, MCP persisted discovery index (mcp-index.json, ConfigHash, word-token Rank), `moca mcp import` importer (DiscoverSources, secret rewriting, SetObjectEntry writes), `moca mcp index` subcommand (IndexAll over configured servers), MCP lazy proxy (§10.5) (+5 more)

### Community 42 - "Edit Application Internals"
Cohesion: 0.23
Nodes (14): applyEdit(), formatDiff(), indentOf(), joinInts(), lineOf(), matchLines(), replaceExact(), replaceFuzzy() (+6 more)

### Community 43 - "Front-End Entry & Trust"
Cohesion: 0.19
Nodes (10): exitFor(), firstLine(), resolveResume(), runOneShot(), decideTrust(), hasProjectResources(), runTUI(), Slug() (+2 more)

### Community 44 - "Spec Concepts: Tools & Permissions"
Cohesion: 0.15
Nodes (12): internal/mcp ProxyTool — Spec/run/Allowed gating, config.AppendString — comment-preserving JSONC string-array append, JSONC config pre-pass + comment-preserving edits (AppendString/SetObjectEntry), config.SetObjectEntry — JSONC object-key insert (phase 5), MCP call gating (readOnlyHint/destructiveHint, approve lists, allow-always persistence), The `mcp` proxy tool (search/describe/call rendering, 30K truncation, frozen spec), Path jail (canonical root + read-only roots, symlink resolution), Parsed shell analysis incl. rtk unwrap (phase 6) (+4 more)

### Community 45 - "Phase 5 Pass-2 Review"
Cohesion: 0.15
Nodes (13): 1. A streamable-HTTP server that opens an SSE response and stalls hangs `mcp` calls indefinitely — Confirmed, 1. `Notify` returns success for any HTTP status, masking a dead session at the worst moment — Confirmed, 2. `envName` collisions: two distinct keys can map to the same exported variable — Confirmed, 2. `moca mcp import` copies literal secrets that live outside `env`/`headers` — Confirmed, 3. A connection-aborted SSE response skips the designed restart-once — Confirmed, 3. Search-result description truncation splits UTF-8 runes — Confirmed, 4. `Manager.Close` concurrent with a failing in-flight call respawns a server after shutdown — Confirmed by trace, 5. Error paths of both transports have no unit tests — Confirmed (+5 more)

### Community 46 - "Phase 5 Pass-3 Review"
Cohesion: 0.15
Nodes (13): 1. stdio exit handling races `cmd.Wait()` against the stdout reader: real replies and the server's own diagnostics can be lost — Confirmed, 2. `moca mcp import` still copies literal credentials held in `env`/`headers` values whose key is not secret-shaped, and prints them in the preview — Confirmed, 3. HTTP transport `Close` has no bound short of the 15-minute client timeout, and runs under the server lock — Confirmed, Checks run, Earlier findings, Findings, Medium, Plan conformance (+5 more)

### Community 47 - "Prompt Templates & Commands"
Cohesion: 0.18
Nodes (13): ExpandPrompt(), Prompt, LoadPrompts(), TestEcosystemPromptsLoad(), TestExpandPrompt(), TestLoadPrompts(), HelpText(), ParseInput() (+5 more)

### Community 48 - "TUI Pager"
Cohesion: 0.18
Nodes (3): newPager(), TestTeaAPI(), pagerModel

### Community 49 - "Built-in Skills & Discovery"
Cohesion: 0.24
Nodes (9): builtinHash(), extractBuiltins(), TestEcosystemSkillsLoadUnchanged(), Discover(), mkSkill(), TestDiscoverPrecedence(), TestDiscoverReportsSymlinkedDir(), TestExtractBuiltins() (+1 more)

### Community 50 - "Input State Machine Tests"
Cohesion: 0.30
Nodes (11): NewInput(), TestCollapseUsesExactOffset(), TestCtrlC(), TestEditedMarkerSentLiterally(), TestHistory(), TestHistoryRecallChipsUnsafeContent(), TestMarkerNeverCollidesWithTypedText(), TestPasteChipKeepsContent() (+3 more)

### Community 51 - "CLI Argument Parsing"
Cohesion: 0.35
Nodes (8): parseArgs(), TestParseArgs(), main(), run(), confirm(), runMCP(), runMCPImport(), runMCPIndex()

### Community 52 - "MCP Client & Index Entries"
Cohesion: 0.20
Nodes (6): Annotations, client, Index, IndexEntry, IndexTool, Tool

### Community 53 - "Provider Registry Tests"
Cohesion: 0.33
Nodes (11): NewRegistry(), mustCfg(), TestBaseURLPrecedence(), TestBuiltinModelNeedsProtocol(), TestBuiltinProviderNewModelWithModelProtocol(), TestCustomProviderAndLazyMissingKey(), TestNoSessionHeaderOutsideOpenCodeGo(), TestOpenCodeGoSessionHeader() (+3 more)

### Community 54 - "Phase 5 Pass-3 Fix List"
Cohesion: 0.20
Nodes (10): 1. The retry-once replays a `tools/call` that may already have executed — Confirmed (plan-prescribed behaviour), 2. An explicit `"args": null` is sent to the server as `"arguments": null` — Confirmed, 3. `Search` hides a failing server's error and re-spawns it on every search — Confirmed, 4. stdio handshake/calls and the pagination loop have no bound other than ctx — Confirmed by reading, 5. `Index.Save` is not safe across concurrent moca processes — Plausible, 6. A negative `mcp.idleTimeout` is accepted by validation — Confirmed by reading, 7. Test gaps and tests that cannot fail — Confirmed, 8. `FilterEnv` matches names case-sensitively, so Windows stdio servers get almost no environment — Plausible (+2 more)

### Community 55 - "System Prompt & Instructions"
Cohesion: 0.22
Nodes (9): PromptInput, ServerLine, BuildSystemPrompt(), oneLine(), TestBuildSystemPrompt(), Instruction, LoadInstructions(), TestLoadInstructions() (+1 more)

### Community 56 - "Session Finding"
Cohesion: 0.25
Nodes (9): Find(), FindForWorkdir(), idOf(), listNewestFirst(), TestFind(), TestFindForWorkdirViaSymlink(), TestFindMtimeTieDeterministic(), TestFindNoSessions() (+1 more)

### Community 58 - "Yolo Overrides"
Cohesion: 0.29
Nodes (4): Agent, NewUnjailed(), TestUnjailedAndAllowAll(), Unjailed

### Community 59 - "Phase 5 Pass-2 Sections"
Cohesion: 0.25
Nodes (8): Checks run, Earlier findings, Plan conformance (brief — pass 1's area), Review Focus, Review: phase 5 — MCP lazy proxy (pass 2, adversarial quality), Suggested order of fixes, Summary, Verdict

### Community 60 - "Shell Env Filtering"
Cohesion: 0.29
Nodes (8): TestResolveEnv(), ResolveEnv(), TestKeepNameWindows(), FilterEnv(), keepName(), serverEnv(), TestFilterEnv(), TestServerEnvExplicitEntryReplacesInherited()

### Community 61 - "Resume Behavior Tests"
Cohesion: 0.38
Nodes (6): catalogConfig(), lastModelChange(), startCatalog(), TestResumeEffortFlagOverridesStored(), TestResumeRestoresHardMode(), TestResumeRestoresModelAndEffortFromModelChange()

### Community 63 - "Summarizer Serialization"
Cohesion: 0.33
Nodes (5): CapChars(), TestCapChars(), TestCapCharsRuneSafe(), Summarizer, TextOf()

### Community 64 - "Spec: Loop & Context"
Cohesion: 0.40
Nodes (4): Agent loop (§14: turn contract, steering, maxSteps wrap-up), Context manager compaction (budgets, cut points, structured summary, overflow recovery), Cross-provider thinking replay transform + NormalizeToolID, Session resume (--resume/--continue, repair, single-writer flock)

### Community 65 - "Gitignore Matcher"
Cohesion: 0.40
Nodes (5): globToRegexp(), ignored(), parseIgnore(), TestIgnore(), ignoreRule

### Community 66 - "Read Tracking"
Cohesion: 0.47
Nodes (4): NewReadTracker(), stampOf(), ReadTracker, stamp

### Community 67 - "Model Overrides & Costing"
Cohesion: 0.40
Nodes (5): ModelOverride, ProviderConfig, CostConfig, costM(), TestCostOf()

### Community 68 - "Model Switching Design"
Cohesion: 0.50
Nodes (4): /hard toggle (modelHard + high effort), model_change session entry, Model/effort switching (SetModel, SetEffort), NormalizeToolID (deterministic tool-call id normalization)

### Community 70 - "Frontend-Design Skill"
Cohesion: 0.50
Nodes (3): AI-generated design default tells, frontend-design Apache 2.0 License, Claude Code frontend-design skill

### Community 71 - "pi mcp-scripting Skill"
Cohesion: 0.67
Nodes (4): jev.evaluate reference, jev.evaluate semantic scripting gate, mcpScript tools API, pi mcp-scripting skill

## Ambiguous Edges - Review These
- `Single-writer session locking` → `writer.go`  [AMBIGUOUS]
  docs/reviews/2026-10-06-phase-4-context-compaction-resume.md · relation: conceptually_related_to
- `Model catalog (builtinCatalog)` → `Thinking modes none|budget|adaptive|openai`  [AMBIGUOUS]
  docs/plans/phase-1-skeleton-providers.md · relation: conceptually_related_to

## Knowledge Gaps
- **125 isolated node(s):** `1. A streamable-HTTP server that opens an SSE response and stalls hangs `mcp` calls indefinitely — Confirmed`, `1. `Notify` returns success for any HTTP status, masking a dead session at the worst moment — Confirmed`, `2. `envName` collisions: two distinct keys can map to the same exported variable — Confirmed`, `2. `moca mcp import` copies literal secrets that live outside `env`/`headers` — Confirmed`, `3. A connection-aborted SSE response skips the designed restart-once — Confirmed` (+120 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 249 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **29 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **What is the exact relationship between `Single-writer session locking` and `writer.go`?**
  _Edge tagged AMBIGUOUS (relation: conceptually_related_to) - confidence is low._
- **Why does `Review pass 2 (adversarial quality)` connect `MCP Proxy Plan & Reviews` to `Review Pipeline & Compaction API`, `Entry Points & Phase Reviews`, `Phase 5 Pass-2 Sections`, `MCP Import & Secret Rewriting`?**
  _High betweenness centrality (0.057) - this node is a cross-community bridge._
- **What connects `1. A streamable-HTTP server that opens an SSE response and stalls hangs `mcp` calls indefinitely — Confirmed`, `1. `Notify` returns success for any HTTP status, masking a dead session at the worst moment — Confirmed`, `2. `envName` collisions: two distinct keys can map to the same exported variable — Confirmed` to the rest of the system?**
  _125 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Entry Points & Phase Reviews` be split into smaller, more focused modules?**
  _Cohesion score 0.0801600290961993 - nodes in this community are weakly interconnected._
- **What is the exact relationship between `Model catalog (builtinCatalog)` and `Thinking modes none|budget|adaptive|openai`?**
  _Edge tagged AMBIGUOUS (relation: conceptually_related_to) - confidence is low._
- **Why does `moca DESIGN.md — v1 contract (rev 11)` connect `Review Pipeline & Compaction API` to `MCP Proxy Plan & Reviews`?**
  _High betweenness centrality (0.052) - this node is a cross-community bridge._
- **Should `Agent Test Harness` be split into smaller, more focused modules?**
  _Cohesion score 0.10526315789473684 - nodes in this community are weakly interconnected._