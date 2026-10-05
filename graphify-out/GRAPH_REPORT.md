# Graph Report - moca  (2026-10-06)

## Corpus Check
- 162 files · ~187,087 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 5 file(s) not represented in the graph (top: (none) 3, .jsonc 1, .toml 1)

## Summary
- 1384 nodes · 5096 edges · 82 communities (59 shown, 23 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 588 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- TUI and Agent Core
- Compaction Wiring and Phase-4 Review
- MCP Lazy Proxy and Gating
- Config, CLI Entry and rtk Ecosystem
- Design Contract and Context Manager
- Shared Test Helpers and Parsing
- Agent Test Harness
- Shell Analysis and Path Jail
- CLI Wiring and Config Loading
- CLI Test Suite
- TUI Glue and OAuth Plans
- Session Writer and Snapshots
- TUI Model and Commands
- Provider Adapter Tests
- Phase-2 Plan and Tool Loop
- TUI Compact Test Fixtures
- HTTP Streaming Adapters
- Retry Policy and SSE Events
- Session Startup Wiring
- Agent Core Loop
- Pager and Item Model
- Input Editor
- Phase-3 Reviews and TUI Input
- Approvals and Sanitizing
- Agent Control and Steering
- Token Estimation
- TUI Run Loop and Events
- Provider Registry
- Edit Tool and Fuzzy Apply
- Resume Path and Tests
- Status Bar Rendering
- Skills Discovery and Frontmatter
- Input Editor Tests
- Usage and Status Events
- Protocol Decoding and Errors
- Registry Base URL Tests
- Formatting Helpers
- Ignore Rule Parsing
- Tool Specs and Test Tools
- Adapter Streaming and Retry
- Effort Levels and Clamping
- Turn Loop and Recovery
- System Prompt Building
- Session Entry Schema
- Tool Registry
- Yolo and Unjailed Mode
- Model Catalog and Cost Tests
- Run Results and Approvals
- Transcript Rebuild
- Slash Command Parsing
- Tool Env and Guarded Writes
- Read Tool
- Read Tracking and Stamps
- Write Tool and Line Counting
- Trust Flow and Review Findings
- Shell Lexer
- Model Switching Concepts
- Process Group Control
- HTTP Error Taxonomy
- Truncation Helpers
- Provider Capability Concepts
- Sanitize Tests
- Allow-All Test Doubles
- Start Error Type
- Allow-All Type
- Bounded Writer
- Fake Command Checker
- Compacted Event
- Resumed Event
- Steering Event
- StreamReset Event
- TextDelta Event
- ThinkingDelta Event
- Warning Event
- YoloChanged Event
- EnvError Type
- Windows Shell Checks
- Shell Output Messages
- Fake Snapshotter
- Root Path Checker
- Module Root

## God Nodes (most connected - your core abstractions)
1. `model` - 47 edges
2. `newScript()` - 44 edges
3. `run()` - 36 edges
4. `startTestWith()` - 33 edges
5. `Phase 4 context compaction & resume review - pass 1 (2026-10-05)` - 33 edges
6. `sseServer()` - 32 edges
7. `Phase 4 Plan: Context Manager, Compaction, Resume` - 31 edges
8. `Agent` - 29 edges
9. `ReadFile()` - 29 edges
10. `keyCred()` - 27 edges

## Surprising Connections (you probably didn't know these)
- `Token-Compressed CLI Proxy` --semantically_similar_to--> `Token Discipline Principle`  [INFERRED] [semantically similar]
  internal/skills/builtin/rtk/SKILL.md → docs/plans/phase-2-tools-agent-loop.md
- `No secrets needed — stdlib-only build` --semantically_similar_to--> `Stdlib-only constraint — hand-rolled HTTP+SSE`  [INFERRED] [semantically similar]
  .github/workflows/ci.yml → docs/plans/phase-1-skeleton-providers.md
- `rtk proxy Raw-Output Fallback` --conceptually_related_to--> `Parsed Shell Command Analyser (shell-analysis ladder)`  [INFERRED]
  internal/skills/builtin/rtk/SKILL.md → docs/plans/phase-2-tools-agent-loop.md
- `parseArgs()` --calls--> `ParseEffort()`  [EXTRACTED]
  cmd/moca/cli.go → internal/llm/types.go
- `TestOneShotStreamsToStdout()` --calls--> `run()`  [INFERRED]
  cmd/moca/cli_test.go → internal/tools/read_test.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Agent Loop Flow** — docs_plans_phase_2_tools_agent_loop_agent_loop, docs_plans_phase_2_tools_agent_loop_sequential_tool_execution, docs_plans_phase_2_tools_agent_loop_max_steps_wrap_up, docs_plans_phase_2_tools_agent_loop_jsonl_session, docs_plans_phase_2_tools_agent_loop_transcript_repair [EXTRACTED 1.00]
- **Permission Checker Injection** — docs_plans_phase_2_tools_agent_loop_tool_interface, docs_plans_phase_2_tools_agent_loop_structural_interfaces, docs_plans_phase_2_tools_agent_loop_path_jail, docs_plans_phase_2_tools_agent_loop_shell_analysis, docs_plans_phase_2_tools_agent_loop_project_trust, docs_plans_phase_2_tools_agent_loop_yolo_mode [EXTRACTED 1.00]
- **Skill Loading Pipeline** — docs_plans_phase_2_tools_agent_loop_skills_loader, docs_plans_phase_2_tools_agent_loop_builtin_extraction, internal_skills_builtin_rtk_skill_rtk, docs_plans_phase_2_tools_agent_loop_path_jail [EXTRACTED 1.00]
- **Phase-1 CI gate set** — _github_workflows_ci, docs_plans_phase_1_skeleton_providers, _github_workflows_ci_ci_gate_set [EXTRACTED 1.00]
- **rtk unwrap and classification chain** — docs_plans_phase_6_rtk_graphify_ecosystem_rtk_subcommand_table, docs_plans_phase_6_rtk_graphify_ecosystem_rtktarget, docs_plans_phase_6_rtk_graphify_ecosystem_shell_unwrap, docs_plans_phase_6_rtk_graphify_ecosystem_rtk_never_launder [EXTRACTED 1.00]
- **Ecosystem skill loading pipeline** — docs_plans_phase_6_rtk_graphify_ecosystem_ecosystem_corpus, docs_plans_phase_6_rtk_graphify_ecosystem_frontmatter_parser, docs_plans_phase_6_rtk_graphify_ecosystem_skills, docs_plans_phase_6_rtk_graphify_ecosystem_builtin_rtk_skill, docs_plans_phase_6_rtk_graphify_ecosystem_graphify [EXTRACTED 1.00]
- **TUI pure-core + glue architecture** — docs_plans_phase_3_tui_pure_core_architecture, docs_plans_phase_3_tui_bubble_tea_glue, docs_plans_phase_3_tui_input_state_machine, docs_plans_phase_3_tui_event_pipe [EXTRACTED 1.00]
- **OAuth subscription access pipeline** — docs_plans_phase_7_oauth_release_oauth_policy_gate, docs_plans_phase_7_oauth_release_provider_credential_seam, docs_plans_phase_7_oauth_release_token_store, docs_plans_phase_7_oauth_release_cross_process_lock, docs_plans_phase_7_oauth_release_pkce_flow, docs_plans_phase_7_oauth_release_headless_paste, docs_plans_phase_7_oauth_release_moca_login [EXTRACTED 1.00]
- **Model switching, session events and status reporting flow** — docs_plans_phase_3_tui_model_switching, docs_plans_phase_3_tui_model_change_event, docs_plans_phase_3_tui_cross_provider_history_transform, docs_plans_phase_3_tui_anchored_context_estimate, docs_plans_phase_3_tui_status_bar [EXTRACTED 1.00]
- **Phase 4 compaction pipeline (budget → cut → serialize → summarize → event)** — docs_plans_phase_4_context_compaction_resume_compact_budget, docs_plans_phase_4_context_compaction_resume_compact_entry, docs_plans_phase_4_context_compaction_resume_compact_findcut, docs_plans_phase_4_context_compaction_resume_compact_serialize, docs_plans_phase_4_context_compaction_resume_compact_summarysystem, docs_plans_phase_4_context_compaction_resume_compact_compact, docs_plans_phase_4_context_compaction_resume_agent_maybecompact, docs_plans_phase_4_context_compaction_resume_agent_agent_compact, docs_plans_phase_4_context_compaction_resume_agent_compacted [INFERRED 0.85]
- **Resume / --continue context rebuild flow** — docs_plans_phase_4_context_compaction_resume_session_find, docs_plans_phase_4_context_compaction_resume_session_findforworkdir, docs_plans_phase_4_context_compaction_resume_agent_resume, docs_plans_phase_4_context_compaction_resume_agent_build, docs_plans_phase_4_context_compaction_resume_session_messages, docs_plans_phase_4_context_compaction_resume_session_latestcompaction [INFERRED 0.85]
- **moca extension model (skills + slash commands + shell allowlist + lazy MCP)** — docs_specs_design_skills, docs_specs_design_external_tools, docs_specs_design_mcp_lazy_proxy, docs_specs_design_permissions, docs_specs_design_explicit_non_goals [INFERRED 0.90]
- **MCP lazy lifecycle & persisted discovery** — docs_plans_phase_5_mcp_lazy_proxy_manager, docs_plans_phase_5_mcp_lazy_proxy_lazy_lifecycle, docs_plans_phase_5_mcp_lazy_proxy_index, docs_plans_phase_5_mcp_lazy_proxy_mcp_index_json, docs_plans_phase_5_mcp_lazy_proxy_config_hash [EXTRACTED 1.00]
- **Secret-free MCP import pipeline** — docs_plans_phase_5_mcp_lazy_proxy_importer, docs_plans_phase_5_mcp_lazy_proxy_rewrite_secrets, docs_specs_specs_env_indirection, docs_plans_phase_5_mcp_lazy_proxy_set_object_entry, docs_plans_phase_5_mcp_lazy_proxy_mcp_import_cmd [EXTRACTED 1.00]
- **Frozen schema + allow-always approval flow across phases** — docs_plans_phase_5_mcp_lazy_proxy_frozen_mcp_schema, docs_plans_phase_5_mcp_lazy_proxy_tool, docs_plans_phase_5_mcp_lazy_proxy_tui_allow_always, docs_specs_specs_append_string, docs_specs_specs_tui_approvals [INFERRED 0.85]
- **Phase 4 compaction pipeline (budget, cut, serialize, summarize, agent trigger)** — internal_compact_budget, internal_compact_cut, internal_compact_serialize, internal_compact_summary, internal_agent_compaction [EXTRACTED 1.00]
- **Phase 3 TUI review series (passes 1-4)** — docs_reviews_2026_10_05_phase_3_tui_review, docs_reviews_2026_10_05_phase_3_tui_pass_2_review, docs_reviews_2026_10_05_phase_3_tui_pass_3_review, docs_reviews_2026_10_05_phase_3_tui_pass_4_review [EXTRACTED 1.00]
- **Phase 4 compaction review series (passes 1-3)** — docs_reviews_2026_10_05_phase_4_context_compaction_resume_review, docs_reviews_2026_10_05_phase_4_context_compaction_resume_pass_2_review, docs_reviews_2026_10_06_phase_4_context_compaction_resume_review [EXTRACTED 1.00]

## Communities (82 total, 23 thin omitted)

### Community 0 - "TUI and Agent Core"
Cohesion: 0.11
Nodes (17): Agent loop, Phase 1 skeleton & providers review (2026-10-04), Phase 2 tools & agent loop review (2026-10-05), usageError, Symlink-resolving path jail, anthropicEvent, anthropicUsage, completionsChunk (+9 more)

### Community 1 - "Compaction Wiring and Phase-4 Review"
Cohesion: 0.07
Nodes (44): Context compaction, Crash repair of unanswered tool calls, Phase 4 context compaction & resume review - pass 2 (2026-10-05), Phase 4 context compaction & resume review - pass 1 (2026-10-05), Phase 4 context compaction & resume review - pass 3 (2026-10-06), Cut-point selection (FindCut), Agent, summarizerUnavailable() (+36 more)

### Community 2 - "MCP Lazy Proxy and Gating"
Cohesion: 0.05
Nodes (53): agent.build MCP wiring + Agent.Close, MCP tool annotations (readOnlyHint/destructiveHint), MCP client (client.go: initialize/listTools/callTool), ConfigHash (sha256 of server config fields), FilterEnv/serverEnv (stdio env allowlist), streamable HTTP transport (http.go), MCP config importer (importer.go), Persisted MCP discovery index (index.go) (+45 more)

### Community 3 - "Config, CLI Entry and rtk Ecosystem"
Cohesion: 0.06
Nodes (47): CI Workflow (gofmt · vet · build · test), CI gate set (gofmt, go vet, go build, go test -race), go.mod (Go version source for setup-go), SPECS.md §2 — CI gate definition, Phase 1 — Skeleton, Protocol Adapters, Streaming, JSONC Config (implementation plan), Adapter interface — Stream(ctx, req, emit), anthropic-messages adapter, ClampEffort / DefaultEffort (+39 more)

### Community 4 - "Design Contract and Context Manager"
Cohesion: 0.08
Nodes (48): /review-phase slash command, Phase review document (docs/reviews/), Phase 4 Plan: Context Manager, Compaction, Resume, agent.Agent.Compact (manual + automatic compaction), agent.build (shared Start/Resume construction), agent.Compacted event, agent.ErrNothingToCompact, agent.maybeCompact (trigger check + loop guard) (+40 more)

### Community 5 - "Shared Test Helpers and Parsing"
Cohesion: 0.07
Nodes (47): NormalizeToolID(), TestNormalizeToolID(), TestTransformDropsEmptiedAssistantAndMerges(), TestTransformDropsRedactedForeignThinking(), TestTransformHistory(), TestTransformSameBareIDDifferentProvider(), TransformHistory(), Parse() (+39 more)

### Community 6 - "Agent Test Harness"
Cohesion: 0.13
Nodes (44): scriptServer, newScript(), startTest(), startTestWith(), startWith(), TestAbortWritesSyntheticResults(), TestFailedCallDoesNotCancelRest(), TestLengthStopTruncatedToolCall() (+36 more)

### Community 7 - "Shell Analysis and Path Jail"
Cohesion: 0.08
Nodes (30): canonical(), Jail, NewJail(), resolveDeep(), setup(), TestJail(), TestJailDanglingSymlinks(), TestJailResolvedPathIsCanonical() (+22 more)

### Community 8 - "CLI Wiring and Config Loading"
Cohesion: 0.08
Nodes (30): parseArgs(), TestParseArgs(), main(), run(), exitFor(), firstLine(), resolveResume(), runOneShot() (+22 more)

### Community 9 - "CLI Test Suite"
Cohesion: 0.12
Nodes (37): fakeCompletions(), isolate(), TestExitCodes(), TestInterruptedExit130(), TestNoApproveSkipsProjectInstructions(), TestOneShotDiscardsStdoutOnReset(), TestOneShotMaxStepsExit3(), TestOneShotNonCompletionStopsExitNonZero() (+29 more)

### Community 10 - "TUI Glue and OAuth Plans"
Cohesion: 0.08
Nodes (30): allow-always persistence (shell.allow), Interactive approval prompt (blocking Asker), Bubble Tea glue (internal/tui/app.go), Bubble Tea v2 (charm.land/bubbletea/v2), LoadPrompts / ExpandPrompt (internal/skills/prompts.go), Numbered collapsible tool/thinking items + pager, ParseInput command parsing (internal/tui/commands.go), Paste chips [paste N lines #K] (+22 more)

### Community 11 - "Session Writer and Snapshots"
Cohesion: 0.08
Nodes (25): Find(), FindForWorkdir(), header(), idOf(), listNewestFirst(), TestFind(), TestFindForWorkdirViaSymlink(), TestFindMtimeTieDeterministic() (+17 more)

### Community 12 - "TUI Model and Commands"
Cohesion: 0.18
Nodes (4): println(), printlnContent(), compactDoneMsg, model

### Community 13 - "Provider Adapter Tests"
Cohesion: 0.20
Nodes (32): newAnthropic(), keyCred(), mustJSON(), sseServer(), TestAnthropicBodyHygiene(), TestAnthropicBudgetAndNoCache(), TestAnthropicBudgetFollowsEffort(), TestAnthropicCredentialHeaders() (+24 more)

### Community 14 - "Phase-2 Plan and Tool Loop"
Cohesion: 0.09
Nodes (29): Agent Loop (model - tools - model), AGENTS.md / CLAUDE.md Project Instructions, Embedded Built-in Skills Extraction (ExtractBuiltins), DESIGN.md rev 11 Specification, Edit Ladder (exact / whitespace-insensitive / replace_all), Pure-Go Gitignore Matcher, Append-Only JSONL Session Transcript, maxSteps Wrap-Up (tool_choice none) (+21 more)

### Community 15 - "TUI Compact Test Fixtures"
Cohesion: 0.12
Nodes (30): key(), keyMsg(), newTestModel(), TestApprovalPreservesDraft(), TestPasteDoesNotSend(), TestRunDoneClearsTransient(), TestStreamingCommitsCompletedLines(), TestThinkingCollapsedOnTurnEnd() (+22 more)

### Community 16 - "HTTP Streaming Adapters"
Cohesion: 0.13
Nodes (19): ContentBlock, Request, Response, StopReason, ToolResult, post(), anthropicOverflow(), anthropicStop() (+11 more)

### Community 17 - "Retry Policy and SSE Events"
Cohesion: 0.23
Nodes (19): Retry, Event, DefaultRetryPolicy(), RetryNotice, fail(), ok(), TestMidStreamFailureResetsAndRetriesOnce(), TestMidStreamRetryBacksOffAndNotifies() (+11 more)

### Community 18 - "Session Startup Wiring"
Cohesion: 0.14
Nodes (18): setup, GitState(), Platform(), build(), StartOptions, prepare(), Start(), TestEnvRefs() (+10 more)

### Community 19 - "Agent Core Loop"
Cohesion: 0.17
Nodes (7): Options, Agent, Outcome, New(), CommandChecker, PathChecker, runDoneMsg

### Community 20 - "Pager and Item Model"
Cohesion: 0.16
Nodes (5): newPager(), TestTeaAPI(), Item, Items, pagerModel

### Community 22 - "Phase-3 Reviews and TUI Input"
Cohesion: 0.19
Nodes (14): Phase 3 TUI review - pass 2 (2026-10-05), Phase 3 TUI review - pass 3 (2026-10-05), Phase 3 TUI review - pass 1 (2026-10-05), TUI event pipe with pre-start buffering, Exit code mapping (exitFor), Hard mode model switching, internal/tui/control.go, JSONC-preserving config edit (AppendString) (+6 more)

### Community 23 - "Approvals and Sanitizing"
Cohesion: 0.15
Nodes (9): AutoAllow(), Answer, Question, approvalPrompt(), firstLineOf(), TestApprovalPromptSanitized(), Sanitize(), approvalMsg (+1 more)

### Community 24 - "Agent Control and Steering"
Cohesion: 0.17
Nodes (3): userText(), TestLiveEntriesMissingFirstKeptFallback(), Agent

### Community 25 - "Token Estimation"
Cohesion: 0.22
Nodes (12): Kind, EntryTokens(), Entry, MessageChars(), RequestChars(), TestEstimate(), Tokens(), UsageTokens() (+4 more)

### Community 26 - "TUI Run Loop and Events"
Cohesion: 0.16
Nodes (11): Event, Asker, newModel(), Run(), newAsker(), newEventPipe(), TestEventPipeBuffersUntilStarted(), TestEventPipeNonBlockingAndFIFO() (+3 more)

### Community 27 - "Provider Registry"
Cohesion: 0.20
Nodes (6): TestResolveEnv(), ResolveEnv(), Model, Registry, supportedEfforts(), CredentialFunc

### Community 28 - "Edit Tool and Fuzzy Apply"
Cohesion: 0.30
Nodes (14): applyEdit(), formatDiff(), indentOf(), joinInts(), lineOf(), matchLines(), replaceExact(), replaceFuzzy() (+6 more)

### Community 29 - "Resume Path and Tests"
Cohesion: 0.20
Nodes (12): Resume(), catalogConfig(), lastModelChange(), startCatalog(), TestResumeEffortFlagOverridesStored(), TestResumeMissingWorkdir(), TestResumeNeverRestoresYolo(), TestResumeNotASession() (+4 more)

### Community 30 - "Status Bar Rendering"
Cohesion: 0.23
Nodes (8): TestStatusWideRunesNeverExceedWidth(), RenderStatus(), TestRenderStatusFull(), TestRenderStatusNarrow(), TestRenderStatusVariants(), truncCells(), Status bar rendering and width budget, StatusInfo

### Community 31 - "Skills Discovery and Frontmatter"
Cohesion: 0.20
Nodes (12): builtinHash(), ExtractBuiltins(), ParseFrontmatter(), TestFrontmatter(), LoadPrompts(), TestLoadPrompts(), Discover(), mkSkill() (+4 more)

### Community 32 - "Input Editor Tests"
Cohesion: 0.30
Nodes (11): NewInput(), TestCollapseUsesExactOffset(), TestCtrlC(), TestEditedMarkerSentLiterally(), TestHistory(), TestHistoryRecallChipsUnsafeContent(), TestMarkerNeverCollidesWithTypedText(), TestPasteChipKeepsContent() (+3 more)

### Community 33 - "Usage and Status Events"
Cohesion: 0.18
Nodes (5): Status, TurnEnd, fakeSum, Result, Usage

### Community 34 - "Protocol Decoding and Errors"
Cohesion: 0.29
Nodes (3): decode(), errorf(), Result

### Community 35 - "Registry Base URL Tests"
Cohesion: 0.33
Nodes (11): NewRegistry(), mustCfg(), TestBaseURLPrecedence(), TestBuiltinModelNeedsProtocol(), TestBuiltinProviderNewModelWithModelProtocol(), TestCustomProviderAndLazyMissingKey(), TestNoSessionHeaderOutsideOpenCodeGo(), TestOpenCodeGoSessionHeader() (+3 more)

### Community 36 - "Formatting Helpers"
Cohesion: 0.27
Nodes (10): TestGitStatusUsesNoOptionalLocks(), AbbrevEffort(), AbbrevHome(), FmtTokens(), FmtWindow(), GitBranch(), gitStatusArgs(), TestFormatters() (+2 more)

### Community 37 - "Ignore Rule Parsing"
Cohesion: 0.24
Nodes (6): globToRegexp(), ignored(), parseIgnore(), TestIgnore(), ignoreRule, searchTool

### Community 38 - "Tool Specs and Test Tools"
Cohesion: 0.22
Nodes (5): ToolSpec, MCPSpec(), echoTool, lsTool, mcpStub

### Community 39 - "Adapter Streaming and Retry"
Cohesion: 0.22
Nodes (7): Adapter, http2Transient(), retryable(), TestNonSSEOKResponseIsNotRetried(), TestRetryable(), TestRetryableTransportErrors(), retrying

### Community 40 - "Effort Levels and Clamping"
Cohesion: 0.28
Nodes (4): savedModel, Effort, ParseEffort(), TestParseEffort()

### Community 41 - "Turn Loop and Recovery"
Cohesion: 0.31
Nodes (4): ToolEnd, ToolStart, ToolCall, ToolChoice

### Community 42 - "System Prompt Building"
Cohesion: 0.22
Nodes (9): PromptInput, ServerLine, BuildSystemPrompt(), oneLine(), TestBuildSystemPrompt(), Instruction, LoadInstructions(), TestLoadInstructions() (+1 more)

### Community 43 - "Session Entry Schema"
Cohesion: 0.42
Nodes (8): Entry, ModelChange, Compaction, ErrorInfo, Header, PermissionMode, SnapshotRec, ToolUse

### Community 44 - "Tool Registry"
Cohesion: 0.36
Nodes (6): Builtins(), Registry, NewRegistry(), TestRegistryDispatch(), TestSchemasFrozen(), Tool

### Community 45 - "Yolo and Unjailed Mode"
Cohesion: 0.29
Nodes (4): Agent, NewUnjailed(), TestUnjailedAndAllowAll(), Unjailed

### Community 46 - "Model Catalog and Cost Tests"
Cohesion: 0.25
Nodes (6): costM(), TestBuiltinCatalogSane(), TestClampEffort(), TestCostOf(), TestDefaultEffort(), TestMaxTokensAndBudget()

### Community 47 - "Run Results and Approvals"
Cohesion: 0.29
Nodes (4): approve(), runResult(), TestRunResultMapping(), shellTool

### Community 48 - "Transcript Rebuild"
Cohesion: 0.39
Nodes (7): LatestCompaction(), Messages(), Repair(), TestMessagesFromCompaction(), TestMessagesRebuild(), TestRepair(), txt()

### Community 49 - "Slash Command Parsing"
Cohesion: 0.29
Nodes (8): ExpandPrompt(), Prompt, TestExpandPrompt(), HelpText(), ParseInput(), TestParseInput(), InputKind, Parsed

### Community 50 - "Tool Env and Guarded Writes"
Cohesion: 0.29
Nodes (4): Env, guardedWrite(), editTool, Snapshotter

### Community 52 - "Read Tracking and Stamps"
Cohesion: 0.47
Nodes (4): NewReadTracker(), stampOf(), ReadTracker, stamp

### Community 53 - "Write Tool and Line Counting"
Cohesion: 0.33
Nodes (3): countFileLines(), countLines(), writeTool

### Community 54 - "Trust Flow and Review Findings"
Cohesion: 0.60
Nodes (3): Approval prompt flow, Phase 3 TUI review - pass 4 (2026-10-05), Trust decision order

### Community 56 - "Model Switching Concepts"
Cohesion: 0.50
Nodes (4): /hard toggle (modelHard + high effort), model_change session entry, Model/effort switching (SetModel, SetEffort), NormalizeToolID (deterministic tool-call id normalization)

### Community 57 - "Process Group Control"
Cohesion: 0.40
Nodes (4): killProcessGroup(), setProcessGroup(), killProcessGroup(), setProcessGroup()

### Community 58 - "HTTP Error Taxonomy"
Cohesion: 0.40
Nodes (4): newHTTPError(), TestRetryAfterFormats(), TestRetryAfterParsed(), HTTPError

### Community 59 - "Truncation Helpers"
Cohesion: 0.40
Nodes (5): runePrefix(), runeSuffix(), TestTruncate(), TestTruncateLongLines(), Truncate()

### Community 60 - "Provider Capability Concepts"
Cohesion: 0.50
Nodes (3): Protocol adapters (anthropic-messages, openai-completions, openai-responses), Provider registry & model catalog, Retry policy & error taxonomy

### Community 61 - "Sanitize Tests"
Cohesion: 0.50
Nodes (3): hasControl(), TestSanitize(), TestSanitizeC1()

## Ambiguous Edges - Review These
- `writer.go` → `Single-writer session locking`  [AMBIGUOUS]
  docs/reviews/2026-10-06-phase-4-context-compaction-resume.md · relation: conceptually_related_to
- `Model catalog (builtinCatalog)` → `Thinking modes none|budget|adaptive|openai`  [AMBIGUOUS]
  docs/plans/phase-1-skeleton-providers.md · relation: conceptually_related_to
- `MCP config importer (importer.go)` → `pi-mcp-adapter (external Pi MCP config source)`  [AMBIGUOUS]
  docs/plans/phase-5-mcp-lazy-proxy.md · relation: references

## Knowledge Gaps
- **48 isolated node(s):** `usageError`, `github.com/adeotek/moca`, `verdict`, `Shell`, `Credential` (+43 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 153 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **23 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **What is the exact relationship between `writer.go` and `Single-writer session locking`?**
  _Edge tagged AMBIGUOUS (relation: conceptually_related_to) - confidence is low._
- **What is the exact relationship between `Model catalog (builtinCatalog)` and `Thinking modes none|budget|adaptive|openai`?**
  _Edge tagged AMBIGUOUS (relation: conceptually_related_to) - confidence is low._
- **What is the exact relationship between `MCP config importer (importer.go)` and `pi-mcp-adapter (external Pi MCP config source)`?**
  _Edge tagged AMBIGUOUS (relation: references) - confidence is low._
- **Why does `model` connect `TUI Model and Commands` to `TUI and Agent Core`, `TUI Compact Test Fixtures`, `Session Startup Wiring`, `Agent Core Loop`, `Pager and Item Model`, `Input Editor`, `Approvals and Sanitizing`, `TUI Run Loop and Events`, `Status Bar Rendering`?**
  _High betweenness centrality (0.033) - this node is a cross-community bridge._
- **Why does `Agent` connect `Agent Core Loop` to `TUI and Agent Core`, `Usage and Status Events`, `Adapter Streaming and Retry`, `Effort Levels and Clamping`, `Turn Loop and Recovery`, `Session Writer and Snapshots`, `Session Entry Schema`, `TUI Model and Commands`, `TUI Run Loop and Events`, `Provider Registry`?**
  _High betweenness centrality (0.016) - this node is a cross-community bridge._
- **Why does `StartOptions` connect `Session Startup Wiring` to `TUI and Agent Core`, `CLI Wiring and Config Loading`, `TUI Model and Commands`, `HTTP Streaming Adapters`, `TUI Run Loop and Events`, `Resume Path and Tests`?**
  _High betweenness centrality (0.011) - this node is a cross-community bridge._
- **What connects `usageError`, `github.com/adeotek/moca`, `verdict` to the rest of the system?**
  _48 weakly-connected nodes found - possible documentation gaps or missing edges._