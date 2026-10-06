# Graph Report - moca  (2026-10-06)

## Corpus Check
- 186 files · ~215,742 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1629 nodes · 6248 edges · 69 communities (61 shown, 8 thin omitted)
- Extraction: 89% EXTRACTED · 11% INFERRED · 0% AMBIGUOUS · INFERRED: 703 edges (avg confidence: 0.85)
- Token cost: 58,411 input · 19,070 output

## Community Hubs (Navigation)
- Agent Core & TUI Runtime
- Provider Tests & Anthropic Client
- Agent Test Harness
- Config Parsing & CLI Entrypoints
- Context Manager & Sessions
- Test Scaffolding & Helpers
- Prompt Assembly & Builtins
- CLI Test Harness
- MCP Manager & Fake Server
- Shell Permissions & Jail
- OAuth, Trust & Roadmap Plans
- Core Types & Helpers
- Config Parse & Edit API
- MCP Transports & Read Tracking
- TUI Model & Key Handling
- Tools, Agent Loop & Skills
- TUI Test Helpers
- Model & Effort State
- LLM Streaming Client
- MCP Import & Secret Rewriting
- Tool Registry & Specs
- Agent Loop Types
- Cut-Point Logic & Tests
- Compaction Engine
- MCP Manager Lifecycle
- Pager Rendering
- Session Store Writer
- Status Formatting
- Context Estimation
- One-Shot & Session Resume
- Approvals & Sanitizing
- Editor Input Handling
- Phase 5 Docs & Reviews
- Session Entry Schema
- Agent Startup Wiring
- Config Load & Defaults
- HTTP Transport & Fake
- Edit Ladder & Diffs
- Event Pipe & Asker
- MCP Lifecycle & Sentinels
- Review Pass 2 Findings
- Review Pass 3 Findings
- Trust Store & Prompt
- Input Tests
- CLI Command Runners
- MCP Index & Client Types
- Review Pass 1 Findings
- Pass-3 Low Findings
- Proxy Tool & Tests
- Yolo & Unjailed Modes
- Review Pass 1 Doc
- Review Pass 2 Doc
- History Transform & Tests
- Proxy Registration & Gating
- Resume Flag Tests
- TUI Run Entry
- Ignore Rules & Globs
- Snapshots & Undo
- Cost Config & Tests
- Model Switching Behaviors
- MCP Lazy-Start Invariants
- Anthropic Stop Reasons
- Allow-All Permission
- Start Error Type
- Command Checker
- AllowAll Checker
- Shell Windows Check
- Module Root

## God Nodes (most connected - your core abstractions)
1. `model` - 47 edges
2. `newScript()` - 45 edges
3. `run()` - 37 edges
4. `startTestWith()` - 34 edges
5. `Phase 4 context compaction & resume review - pass 1 (2026-10-05)` - 33 edges
6. `sseServer()` - 32 edges
7. `Phase 4 Plan: Context Manager, Compaction, Resume` - 30 edges
8. `moca DESIGN.md — v1 contract (rev 11)` - 30 edges
9. `Agent` - 30 edges
10. `ReadFile()` - 30 edges

## Surprising Connections (you probably didn't know these)
- `Token-Compressed CLI Proxy` --semantically_similar_to--> `Token Discipline Principle`  [INFERRED] [semantically similar]
  internal/skills/builtin/rtk/SKILL.md → docs/plans/phase-2-tools-agent-loop.md
- `No secrets needed — stdlib-only build` --semantically_similar_to--> `Stdlib-only constraint — hand-rolled HTTP+SSE`  [INFERRED] [semantically similar]
  .github/workflows/ci.yml → docs/plans/phase-1-skeleton-providers.md
- `TestImportPlan()` --references--> `Secret rewriting on import`  [EXTRACTED]
  internal/mcp/importer_test.go → docs/plans/phase-5-mcp-lazy-proxy.md
- `TestCallSurvivesIdleTimer()` --references--> `One critical section in Manager.Call (idle-timer race)`  [EXTRACTED]
  internal/mcp/manager_test.go → docs/plans/phase-5-mcp-lazy-proxy.md
- `TestProxySpecFrozen()` --references--> `Frozen MCPSpec schema (phase-2 golden)`  [EXTRACTED]
  internal/mcp/tool_test.go → docs/plans/phase-5-mcp-lazy-proxy.md

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Agent Loop Flow** — docs_plans_phase_2_tools_agent_loop_agent_loop, docs_plans_phase_2_tools_agent_loop_sequential_tool_execution, docs_plans_phase_2_tools_agent_loop_max_steps_wrap_up, docs_plans_phase_2_tools_agent_loop_jsonl_session, docs_plans_phase_2_tools_agent_loop_transcript_repair [EXTRACTED 1.00]
- **Ecosystem skill loading pipeline** — docs_plans_phase_6_rtk_graphify_ecosystem_ecosystem_corpus, docs_plans_phase_6_rtk_graphify_ecosystem_frontmatter_parser, docs_plans_phase_6_rtk_graphify_ecosystem_skills, docs_plans_phase_6_rtk_graphify_ecosystem_builtin_rtk_skill, docs_plans_phase_6_rtk_graphify_ecosystem_graphify [EXTRACTED 1.00]
- **Model switching, session events and status reporting flow** — docs_plans_phase_3_tui_model_switching, docs_plans_phase_3_tui_model_change_event, docs_plans_phase_3_tui_cross_provider_history_transform, docs_plans_phase_3_tui_anchored_context_estimate, docs_plans_phase_3_tui_status_bar [EXTRACTED 1.00]
- **OAuth subscription access pipeline** — docs_plans_phase_7_oauth_release_oauth_policy_gate, docs_plans_phase_7_oauth_release_provider_credential_seam, docs_plans_phase_7_oauth_release_token_store, docs_plans_phase_7_oauth_release_cross_process_lock, docs_plans_phase_7_oauth_release_pkce_flow, docs_plans_phase_7_oauth_release_headless_paste, docs_plans_phase_7_oauth_release_moca_login [EXTRACTED 1.00]
- **Permission Checker Injection** — docs_plans_phase_2_tools_agent_loop_tool_interface, docs_plans_phase_2_tools_agent_loop_structural_interfaces, docs_plans_phase_2_tools_agent_loop_path_jail, docs_plans_phase_2_tools_agent_loop_shell_analysis, docs_plans_phase_2_tools_agent_loop_project_trust, docs_plans_phase_2_tools_agent_loop_yolo_mode [EXTRACTED 1.00]
- **Phase-1 CI gate set** — _github_workflows_ci, docs_plans_phase_1_skeleton_providers, _github_workflows_ci_ci_gate_set [EXTRACTED 1.00]
- **Phase 3 TUI review series (passes 1-4)** — docs_reviews_2026_10_05_phase_3_tui_review, docs_reviews_2026_10_05_phase_3_tui_pass_2_review, docs_reviews_2026_10_05_phase_3_tui_pass_3_review, docs_reviews_2026_10_05_phase_3_tui_pass_4_review [EXTRACTED 1.00]
- **Phase 4 compaction pipeline (budget, cut, serialize, summarize, agent trigger)** — internal_compact_budget, internal_compact_cut, internal_compact_serialize, internal_compact_summary, internal_agent_compaction [EXTRACTED 1.00]
- **Phase 4 compaction review series (passes 1-3)** — docs_reviews_2026_10_05_phase_4_context_compaction_resume_review, docs_reviews_2026_10_05_phase_4_context_compaction_resume_pass_2_review, docs_reviews_2026_10_06_phase_4_context_compaction_resume_review [EXTRACTED 1.00]
- **rtk unwrap and classification chain** — docs_plans_phase_6_rtk_graphify_ecosystem_rtk_subcommand_table, docs_plans_phase_6_rtk_graphify_ecosystem_rtktarget, docs_plans_phase_6_rtk_graphify_ecosystem_shell_unwrap, docs_plans_phase_6_rtk_graphify_ecosystem_rtk_never_launder [EXTRACTED 1.00]
- **Skill Loading Pipeline** — docs_plans_phase_2_tools_agent_loop_skills_loader, docs_plans_phase_2_tools_agent_loop_builtin_extraction, internal_skills_builtin_rtk_skill_rtk, docs_plans_phase_2_tools_agent_loop_path_jail [EXTRACTED 1.00]
- **TUI pure-core + glue architecture** — docs_plans_phase_3_tui_pure_core_architecture, docs_plans_phase_3_tui_bubble_tea_glue, docs_plans_phase_3_tui_input_state_machine, docs_plans_phase_3_tui_event_pipe [EXTRACTED 1.00]
- **Phase 4 compaction pipeline (budget → cut → serialize → summarize → event)** — docs_plans_phase_4_context_compaction_resume_compact_budget, docs_plans_phase_4_context_compaction_resume_compact_entry, docs_plans_phase_4_context_compaction_resume_compact_findcut, docs_plans_phase_4_context_compaction_resume_compact_serialize, docs_plans_phase_4_context_compaction_resume_compact_summarysystem, docs_plans_phase_4_context_compaction_resume_compact_compact, docs_plans_phase_4_context_compaction_resume_agent_maybecompact, docs_plans_phase_4_context_compaction_resume_agent_agent_compact, docs_plans_phase_4_context_compaction_resume_agent_compacted [INFERRED 0.85]
- **Resume / --continue context rebuild flow** — docs_plans_phase_4_context_compaction_resume_session_find, docs_plans_phase_4_context_compaction_resume_session_findforworkdir, docs_plans_phase_4_context_compaction_resume_agent_resume, docs_plans_phase_4_context_compaction_resume_agent_build, docs_plans_phase_4_context_compaction_resume_session_messages, docs_plans_phase_4_context_compaction_resume_session_latestcompaction [INFERRED 0.85]
- **moca extension model (skills + slash commands + shell allowlist + lazy MCP)** — docs_specs_design_skills, docs_specs_design_external_tools, docs_specs_design_mcp_lazy_proxy, docs_specs_design_permissions, docs_specs_design_explicit_non_goals [INFERRED 0.90]
- **Lazy lifecycle flow: persisted index feeds deferred server start, idle stop and restart through the Manager** — internal_mcp_manager, internal_mcp_index, internal_mcp_stdio, internal_mcp_http, concept_mcp_lazy_proxy, concept_mcp_idle_timeout [EXTRACTED 1.00]
- **Import pipeline: discover sources, parse shapes, rewrite secrets, preview, write config** — internal_mcp_importer, concept_mcp_secret_redaction, cmd_moca_mcp, internal_config_edit_setobjectentry [EXTRACTED 1.00]
- **Phase 5 review cycle: pass-1/2/3/4 reviews iterated with fixes landing between passes** — docs_reviews_2026_10_06_phase_5_mcp_lazy_proxy, docs_reviews_2026_10_06_phase_5_mcp_lazy_proxy_pass_2, docs_reviews_2026_10_06_phase_5_mcp_lazy_proxy_pass_3, docs_reviews_2026_10_06_phase_5_mcp_lazy_proxy_pass_4 [INFERRED 0.95]
- **The MCP lazy-proxy stack: transports, client, manager, proxy tool, index** — internal_mcp_jsonrpc, internal_mcp_stdio, internal_mcp_http, internal_mcp_client, internal_mcp_manager, internal_mcp_index, internal_mcp_tool [EXTRACTED 1.00]
- **MCP import pipeline: DiscoverSources → RewriteSecrets → SetObjectEntry write** — internal_mcp_importer, internal_config_setobjectentry, internal_mcp_importer_secrets_never_copied [EXTRACTED 1.00]
- **Pass-2→4 review hardening ladder: sentinels, bounded waits, replay rules, embedded-secret rules** — internal_mcp_jsonrpc_errtransportdead, internal_mcp_manager_defaulthttptimeout, internal_mcp_manager_replayonce, internal_mcp_hardening_test [INFERRED 0.85]

## Communities (69 total, 8 thin omitted)

### Community 0 - "Agent Core & TUI Runtime"
Cohesion: 0.06
Nodes (85): Compacted, Agent loop, Resumed, SteeringApplied, StreamReset, TextDelta, ThinkingDelta, Warning (+77 more)

### Community 1 - "Provider Tests & Anthropic Client"
Cohesion: 0.07
Nodes (64): Retry, SplitModel(), TestResolveEnv(), TestSplitModel(), ResolveEnv(), Event, Adapter, newAnthropic() (+56 more)

### Community 2 - "Agent Test Harness"
Cohesion: 0.11
Nodes (53): scriptServer, newScript(), startTest(), startTestWith(), startWith(), TestAbortWritesSyntheticResults(), TestFailedCallDoesNotCancelRest(), TestLengthStopTruncatedToolCall() (+45 more)

### Community 3 - "Config Parsing & CLI Entrypoints"
Cohesion: 0.06
Nodes (47): CI Workflow (gofmt · vet · build · test), CI gate set (gofmt, go vet, go build, go test -race), go.mod (Go version source for setup-go), SPECS.md §2 — CI gate definition, Phase 1 — Skeleton, Protocol Adapters, Streaming, JSONC Config (implementation plan), Adapter interface — Stream(ctx, req, emit), anthropic-messages adapter, ClampEffort / DefaultEffort (+39 more)

### Community 4 - "Context Manager & Sessions"
Cohesion: 0.08
Nodes (46): /review-phase slash command, Phase review document (docs/reviews/), Phase 4 Plan: Context Manager, Compaction, Resume, agent.Agent.Compact (manual + automatic compaction), agent.build (shared Start/Resume construction), agent.Compacted event, agent.ErrNothingToCompact, agent.maybeCompact (trigger check + loop guard) (+38 more)

### Community 5 - "Test Scaffolding & Helpers"
Cohesion: 0.07
Nodes (46): TestResumeNotASession(), CapChars(), TestCapChars(), TestCapCharsRuneSafe(), TestTrackFiles(), TestTrackFilesNormalizesPaths(), TrackFiles(), TestEffortRankOrdered() (+38 more)

### Community 6 - "Prompt Assembly & Builtins"
Cohesion: 0.10
Nodes (30): PromptInput, ServerLine, BuildSystemPrompt(), oneLine(), TestBuildSystemPrompt(), builtinHash(), ExtractBuiltins(), ParseFrontmatter() (+22 more)

### Community 7 - "CLI Test Harness"
Cohesion: 0.11
Nodes (39): fakeCompletions(), isolate(), TestExitCodes(), TestInterruptedExit130(), TestMCPImportCLI(), TestNoApproveSkipsProjectInstructions(), TestOneShotDiscardsStdoutOnReset(), TestOneShotMaxStepsExit3() (+31 more)

### Community 8 - "MCP Manager & Fake Server"
Cohesion: 0.12
Nodes (39): MCPServer, crashingServer(), shServer(), skipWindows(), TestCallTimeoutStopsWedgedServer(), TestHandshakeTimeout(), TestHTTPAbortedStreamReplayRule(), TestIndexSaveMergesConcurrentWriters() (+31 more)

### Community 9 - "Shell Permissions & Jail"
Cohesion: 0.09
Nodes (27): setup, Jail, NewJail(), resolveDeep(), setup(), TestJail(), TestJailDanglingSymlinks(), TestJailResolvedPathIsCanonical() (+19 more)

### Community 10 - "OAuth, Trust & Roadmap Plans"
Cohesion: 0.08
Nodes (30): allow-always persistence (shell.allow), Interactive approval prompt (blocking Asker), Bubble Tea glue (internal/tui/app.go), Bubble Tea v2 (charm.land/bubbletea/v2), LoadPrompts / ExpandPrompt (internal/skills/prompts.go), Numbered collapsible tool/thinking items + pager, ParseInput command parsing (internal/tui/commands.go), Paste chips [paste N lines #K] (+22 more)

### Community 11 - "Core Types & Helpers"
Cohesion: 0.12
Nodes (12): CallResult, readLine(), approve(), decode(), errorf(), Env, Result, countFileLines() (+4 more)

### Community 12 - "Config Parse & Edit API"
Cohesion: 0.08
Nodes (31): scanner, span, Parse(), TestDefaultsApplied(), TestEnvRefs(), TestRetentionZeroMeansForever(), TestSpecExampleDecodesIntact(), TestSyntaxErrorReportsLineCol() (+23 more)

### Community 13 - "MCP Transports & Read Tracking"
Cohesion: 0.09
Nodes (14): fakeSum, responseID(), TestResponseID(), readEvents(), callResult, NewReadTracker(), stampOf(), httpTransport (+6 more)

### Community 14 - "TUI Model & Key Handling"
Cohesion: 0.18
Nodes (4): println(), printlnContent(), compactDoneMsg, model

### Community 15 - "Tools, Agent Loop & Skills"
Cohesion: 0.09
Nodes (29): Agent Loop (model - tools - model), AGENTS.md / CLAUDE.md Project Instructions, Embedded Built-in Skills Extraction (ExtractBuiltins), DESIGN.md rev 11 Specification, Edit Ladder (exact / whitespace-insensitive / replace_all), Pure-Go Gitignore Matcher, Append-Only JSONL Session Transcript, maxSteps Wrap-Up (tool_choice none) (+21 more)

### Community 16 - "TUI Test Helpers"
Cohesion: 0.11
Nodes (31): key(), keyMsg(), newTestModel(), TestApprovalPreservesDraft(), TestPasteDoesNotSend(), TestRunDoneClearsTransient(), TestStreamingCommitsCompletedLines(), TestThinkingCollapsedOnTurnEnd() (+23 more)

### Community 17 - "Model & Effort State"
Cohesion: 0.09
Nodes (8): savedModel, Status, Agent, Effort, ParseEffort(), TestParseEffort(), Model, supportedEfforts()

### Community 18 - "LLM Streaming Client"
Cohesion: 0.11
Nodes (21): ContentBlock, Request, ToolResult, post(), anthropicOverflow(), newHTTPError(), openaiOverflow(), priorReasoningText() (+13 more)

### Community 19 - "MCP Import & Secret Rewriting"
Cohesion: 0.10
Nodes (25): Secret rewriting on import, config.AppendString (comment-preserving JSONC edit), config.SetObjectEntry (JSONC object entry insert, phase 5), TestImportRewritesEmbeddedSecretsInValues(), TestRewriteSecretsValueRuleAlone(), DiscoverSources(), embeddedSecret(), envName() (+17 more)

### Community 20 - "Tool Registry & Specs"
Cohesion: 0.09
Nodes (16): ToolSpec, Builtins(), MCPSpec(), Registry, NewRegistry(), TestRegistryDispatch(), TestSchemasFrozen(), echoTool (+8 more)

### Community 21 - "Agent Loop Types"
Cohesion: 0.13
Nodes (11): Options, ToolEnd, ToolStart, Agent, Outcome, New(), Response, ToolCall (+3 more)

### Community 22 - "Cut-Point Logic & Tests"
Cohesion: 0.21
Nodes (19): FindCut(), a(), call(), res(), TestFindCutKeepsRecentAtValidPoint(), TestFindCutNeverBetweenCallAndResult(), TestFindCutNothingToDo(), TestFindCutSplitTurn() (+11 more)

### Community 23 - "Compaction Engine"
Cohesion: 0.16
Nodes (11): Kind, Agent, summarizerUnavailable(), Budget, NewBudget(), TestBudget(), EntryTokens(), Entry (+3 more)

### Community 24 - "MCP Manager Lifecycle"
Cohesion: 0.23
Nodes (6): Rank(), TestRank(), words(), Manager, Hit, state

### Community 25 - "Pager Rendering"
Cohesion: 0.16
Nodes (5): newPager(), TestTeaAPI(), Item, Items, pagerModel

### Community 26 - "Session Store Writer"
Cohesion: 0.18
Nodes (12): lockFile(), lockFile(), Create(), Writer, lastNewline(), newID(), Open(), TestCreateAppendRead() (+4 more)

### Community 27 - "Status Formatting"
Cohesion: 0.17
Nodes (16): TestGitStatusUsesNoOptionalLocks(), TestStatusWideRunesNeverExceedWidth(), AbbrevEffort(), AbbrevHome(), FmtTokens(), FmtWindow(), GitBranch(), gitStatusArgs() (+8 more)

### Community 28 - "Context Estimation"
Cohesion: 0.22
Nodes (13): MessageChars(), RequestChars(), TestEstimate(), Tokens(), UsageTokens(), Message, Role, appendUser() (+5 more)

### Community 29 - "One-Shot & Session Resume"
Cohesion: 0.14
Nodes (16): exitFor(), firstLine(), resolveResume(), runOneShot(), Find(), FindForWorkdir(), header(), idOf() (+8 more)

### Community 30 - "Approvals & Sanitizing"
Cohesion: 0.15
Nodes (11): AutoAllow(), Answer, Question, approvalPrompt(), firstLineOf(), TestApprovalPromptSanitized(), hasControl(), Sanitize() (+3 more)

### Community 32 - "Phase 5 Docs & Reviews"
Cohesion: 0.20
Nodes (13): Phase 5 — MCP Lazy Proxy — Implementation Plan, Review pass 2 (adversarial quality), Review pass 4 (maintainer re-review of fix commit 9179ee0), moca SPECS.md (current implemented state), internal/config (JSONC config), internal/llm (leaf types), internal/mcp package (lazy MCP proxy), MCP idle timeout (mcp.idleTimeout default 600s, busy counter) (+5 more)

### Community 33 - "Session Entry Schema"
Cohesion: 0.22
Nodes (10): Result, Usage, Entry, ModelChange, Compaction, ErrorInfo, Header, PermissionMode (+2 more)

### Community 34 - "Agent Startup Wiring"
Cohesion: 0.21
Nodes (13): GitState(), Platform(), build(), StartOptions, prepare(), Start(), TestPathsHonourXDG(), ConfigDir() (+5 more)

### Community 35 - "Config Load & Defaults"
Cohesion: 0.18
Nodes (9): ContextConfig, MCPConfig, ShellConfig, SnapshotConfig, Default(), Config, Load(), TestLoadMissingFileIsDefault() (+1 more)

### Community 36 - "HTTP Transport & Fake"
Cohesion: 0.22
Nodes (9): initialize(), TestHTTPCloseBounded(), startHTTP(), fakeHTTPServer(), TestHTTPNotifyStatusChecked(), TestHTTPTransport(), TestStdioEarlyExit(), headerLog (+1 more)

### Community 37 - "Edit Ladder & Diffs"
Cohesion: 0.23
Nodes (14): applyEdit(), formatDiff(), indentOf(), joinInts(), lineOf(), matchLines(), replaceExact(), replaceFuzzy() (+6 more)

### Community 38 - "Event Pipe & Asker"
Cohesion: 0.18
Nodes (6): Event, Asker, newAsker(), agentEventMsg, eventPipe, trustModel

### Community 39 - "MCP Lifecycle & Sentinels"
Cohesion: 0.18
Nodes (11): Idle timeout lifecycle, MCP lazy proxy (zero servers at session start), internal/agent (loop, Start wiring), Agent.Close — stops MCP servers, closes session writer, Agent.Close (stops live MCP servers, closes session writer), errTransportDead sentinel, Manager.closed flag (no respawn after Close), defaultHTTPTimeout 15m bounded HTTP client (+3 more)

### Community 40 - "Review Pass 2 Findings"
Cohesion: 0.15
Nodes (13): 1. A streamable-HTTP server that opens an SSE response and stalls hangs `mcp` calls indefinitely — Confirmed, 1. `Notify` returns success for any HTTP status, masking a dead session at the worst moment — Confirmed, 2. `envName` collisions: two distinct keys can map to the same exported variable — Confirmed, 2. `moca mcp import` copies literal secrets that live outside `env`/`headers` — Confirmed, 3. A connection-aborted SSE response skips the designed restart-once — Confirmed, 3. Search-result description truncation splits UTF-8 runes — Confirmed, 4. `Manager.Close` concurrent with a failing in-flight call respawns a server after shutdown — Confirmed by trace, 5. Error paths of both transports have no unit tests — Confirmed (+5 more)

### Community 41 - "Review Pass 3 Findings"
Cohesion: 0.15
Nodes (13): 1. stdio exit handling races `cmd.Wait()` against the stdout reader: real replies and the server's own diagnostics can be lost — Confirmed, 2. `moca mcp import` still copies literal credentials held in `env`/`headers` values whose key is not secret-shaped, and prints them in the preview — Confirmed, 3. HTTP transport `Close` has no bound short of the 15-minute client timeout, and runs under the server lock — Confirmed, Checks run, Earlier findings, Findings, Medium, Plan conformance (+5 more)

### Community 42 - "Trust Store & Prompt"
Cohesion: 0.20
Nodes (8): decideTrust(), hasProjectResources(), runTUI(), canonical(), LoadTrust(), TestTrustStore(), RunTrustPrompt(), TrustStore

### Community 43 - "Input Tests"
Cohesion: 0.30
Nodes (11): NewInput(), TestCollapseUsesExactOffset(), TestCtrlC(), TestEditedMarkerSentLiterally(), TestHistory(), TestHistoryRecallChipsUnsafeContent(), TestMarkerNeverCollidesWithTypedText(), TestPasteChipKeepsContent() (+3 more)

### Community 44 - "CLI Command Runners"
Cohesion: 0.35
Nodes (8): parseArgs(), TestParseArgs(), main(), run(), confirm(), runMCP(), runMCPImport(), runMCPIndex()

### Community 45 - "MCP Index & Client Types"
Cohesion: 0.20
Nodes (6): Annotations, client, Index, IndexEntry, IndexTool, Tool

### Community 46 - "Review Pass 1 Findings"
Cohesion: 0.20
Nodes (10): 1. `Manager.Call`'s dead-transport retry on an HTTP transport can restart the same server concurrently with itself across `Search`/`Describe`/`Call` from different goroutines — Plausible, 1. The retry predicate string-matches error text; a server error message containing the substring "exited" (or "session expired") spuriously restarts a healthy server, and any in-flight sibling calls on the same transport are failed — Confirmed, 2. `Index.LoadIndex` silently ignores a corrupt file's parse error, so a partially-parsed index can serve stale entries — Plausible, 2. The stdio transport closes the process at Close (`stdin` close, then kill), but `Call`'s write-error retry branch can return `ctx.Err()` without ever surfacing the exit error, skipping the exactly-once retry the plan promises — Confirmed, 3. `envName` can collide two distinct env/headers keys onto one export variable — Plausible, 4. `Run`'s search output truncates descriptions by bytes, not runes — Plausible (edge), 5. `stdioTransport.Close`'s `<-t.done` waits for the child exit unboundedly after `SIGKILL` — Plausible (edge), Findings (+2 more)

### Community 47 - "Pass-3 Low Findings"
Cohesion: 0.20
Nodes (10): 1. The retry-once replays a `tools/call` that may already have executed — Confirmed (plan-prescribed behaviour), 2. An explicit `"args": null` is sent to the server as `"arguments": null` — Confirmed, 3. `Search` hides a failing server's error and re-spawns it on every search — Confirmed, 4. stdio handshake/calls and the pagination loop have no bound other than ctx — Confirmed by reading, 5. `Index.Save` is not safe across concurrent moca processes — Plausible, 6. A negative `mcp.idleTimeout` is accepted by validation — Confirmed by reading, 7. Test gaps and tests that cannot fail — Confirmed, 8. `FilterEnv` matches names case-sensitively, so Windows stdio servers get almost no environment — Plausible (+2 more)

### Community 48 - "Proxy Tool & Tests"
Cohesion: 0.31
Nodes (8): errResult(), isTrue(), newProxy(), runP(), TestProxyArgErrors(), TestProxyGating(), TestProxySearchDescribeCall(), ProxyTool

### Community 49 - "Yolo & Unjailed Modes"
Cohesion: 0.29
Nodes (4): Agent, NewUnjailed(), TestUnjailedAndAllowAll(), Unjailed

### Community 50 - "Review Pass 1 Doc"
Cohesion: 0.25
Nodes (8): Checks run, Earlier findings, Plan conformance, Review Focus, Review: phase 5 — MCP lazy proxy (pass 1), Suggested order of fixes, Summary, Verdict

### Community 51 - "Review Pass 2 Doc"
Cohesion: 0.25
Nodes (8): Checks run, Earlier findings, Plan conformance (brief — pass 1's area), Review Focus, Review: phase 5 — MCP lazy proxy (pass 2, adversarial quality), Suggested order of fixes, Summary, Verdict

### Community 52 - "History Transform & Tests"
Cohesion: 0.39
Nodes (7): NormalizeToolID(), TestNormalizeToolID(), TestTransformDropsEmptiedAssistantAndMerges(), TestTransformDropsRedactedForeignThinking(), TestTransformHistory(), TestTransformSameBareIDDifferentProvider(), TransformHistory()

### Community 53 - "Proxy Registration & Gating"
Cohesion: 0.33
Nodes (6): MCP tool gating (readOnlyHint / approve list), agent.build — register proxy when servers exist, ProxyTool (the mcp proxy tool), TestProxySpecFrozen(), Frozen MCPSpec schema (phase-2 golden), TUI allow-always for the mcp kind

### Community 54 - "Resume Flag Tests"
Cohesion: 0.38
Nodes (6): catalogConfig(), lastModelChange(), startCatalog(), TestResumeEffortFlagOverridesStored(), TestResumeRestoresHardMode(), TestResumeRestoresModelAndEffortFromModelChange()

### Community 55 - "TUI Run Entry"
Cohesion: 0.38
Nodes (5): newModel(), Run(), runResult(), TestRunResultMapping(), AppOptions

### Community 56 - "Ignore Rules & Globs"
Cohesion: 0.40
Nodes (5): globToRegexp(), ignored(), parseIgnore(), TestIgnore(), ignoreRule

### Community 57 - "Snapshots & Undo"
Cohesion: 0.33
Nodes (3): gitClean(), Snapshots, NewSnapshots()

### Community 58 - "Cost Config & Tests"
Cohesion: 0.40
Nodes (5): ModelOverride, ProviderConfig, CostConfig, costM(), TestCostOf()

### Community 59 - "Model Switching Behaviors"
Cohesion: 0.50
Nodes (4): /hard toggle (modelHard + high effort), model_change session entry, Model/effort switching (SetModel, SetEffort), NormalizeToolID (deterministic tool-call id normalization)

### Community 60 - "MCP Lazy-Start Invariants"
Cohesion: 0.50
Nodes (3): TestMCPNoSchemasInPromptAndZeroStarts, Manager.ensure (start + initialize + tools/list, lazy spawn point), tools.MCPSpec() frozen ~200-token schema

### Community 61 - "Anthropic Stop Reasons"
Cohesion: 0.50
Nodes (3): TurnEnd, StopReason, anthropicStop()

## Ambiguous Edges - Review These
- `Single-writer session locking` → `writer.go`  [AMBIGUOUS]
  docs/reviews/2026-10-06-phase-4-context-compaction-resume.md · relation: conceptually_related_to
- `Model catalog (builtinCatalog)` → `Thinking modes none|budget|adaptive|openai`  [AMBIGUOUS]
  docs/plans/phase-1-skeleton-providers.md · relation: conceptually_related_to
- `mcp/sse.go` → `Transports limited to stdio + streamable HTTP`  [AMBIGUOUS]
  docs/plans/phase-5-mcp-lazy-proxy.md · relation: conceptually_related_to
- `Review pass 4 (maintainer re-review of fix commit 9179ee0)` → `config.SetObjectEntry (JSONC object entry insert, phase 5)`  [AMBIGUOUS]
  docs/reviews/2026-10-06-phase-5-mcp-lazy-proxy-pass-4.md · relation: references

## Knowledge Gaps
- **99 isolated node(s):** `1. stdio exit handling races `cmd.Wait()` against the stdout reader: real replies and the server's own diagnostics can be lost — Confirmed`, `1. The retry-once replays a `tools/call` that may already have executed — Confirmed (plan-prescribed behaviour)`, `2. An explicit `"args": null` is sent to the server as `"arguments": null` — Confirmed`, `2. `moca mcp import` still copies literal credentials held in `env`/`headers` values whose key is not secret-shaped, and prints them in the preview — Confirmed`, `3. HTTP transport `Close` has no bound short of the 15-minute client timeout, and runs under the server lock — Confirmed` (+94 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 212 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **8 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **What is the exact relationship between `Single-writer session locking` and `writer.go`?**
  _Edge tagged AMBIGUOUS (relation: conceptually_related_to) - confidence is low._
- **Why does `moca DESIGN.md — v1 contract (rev 11)` connect `Context Manager & Sessions` to `Phase 5 Docs & Reviews`?**
  _High betweenness centrality (0.073) - this node is a cross-community bridge._
- **What connects `1. stdio exit handling races `cmd.Wait()` against the stdout reader: real replies and the server's own diagnostics can be lost — Confirmed`, `1. The retry-once replays a `tools/call` that may already have executed — Confirmed (plan-prescribed behaviour)`, `2. An explicit `"args": null` is sent to the server as `"arguments": null` — Confirmed` to the rest of the system?**
  _99 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Agent Core & TUI Runtime` be split into smaller, more focused modules?**
  _Cohesion score 0.05637690776376908 - nodes in this community are weakly interconnected._
- **What is the exact relationship between `Model catalog (builtinCatalog)` and `Thinking modes none|budget|adaptive|openai`?**
  _Edge tagged AMBIGUOUS (relation: conceptually_related_to) - confidence is low._
- **Why does `Review pass 2 (adversarial quality)` connect `Phase 5 Docs & Reviews` to `Agent Core & TUI Runtime`, `Context Manager & Sessions`, `MCP Lifecycle & Sentinels`, `MCP Import & Secret Rewriting`, `Review Pass 2 Doc`?**
  _High betweenness centrality (0.055) - this node is a cross-community bridge._
- **Should `Provider Tests & Anthropic Client` be split into smaller, more focused modules?**
  _Cohesion score 0.0710868079289132 - nodes in this community are weakly interconnected._