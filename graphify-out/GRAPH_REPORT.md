# Graph Report - moca  (2026-10-06)

## Corpus Check
- 237 files · ~286,012 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 9 file(s) not represented in the graph (top: (none) 5, .jsonc 3, .toml 1)

## Summary
- 1986 nodes · 7292 edges · 107 communities (77 shown, 30 thin omitted)
- Extraction: 89% EXTRACTED · 11% INFERRED · 0% AMBIGUOUS · INFERRED: 799 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `ddada17f`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- start.go
- newScript
- permissions/shell.go
- Phase 4 Plan: Context Manager, Compaction, Resume
- startStdio
- anthropic_test.go
- testing.T
- context.Context
- phase-6-rtk-graphify-ecosystem.md
- NewManager
- runLogin
- Phase 3 - TUI Shell, Slash Commands, Status Bar, Steering, Model Switching Implementation Plan
- internal/config — JSONC config parsing, defaults, validation
- Parse
- sync.Mutex
- model
- run
- Phase 2 Plan - Seven Tools, Agent Loop, Permissions, Skills, Session Schema
- cut_test.go
- Create
- newAgentModel
- Phase 3 TUI review - pass 1 (2026-10-05)
- ToolCall
- internal/mcp Manager — lazy lifecycle, idle stop, IndexAll
- MCPServer
- Manager
- Review: phase 6 — rtk, graphify, Skills Ecosystem Compatibility (pass 3)
- Start
- Low
- Agent
- session/entry.go
- Low
- .Run
- Message
- Sanitize
- Review: phase 5 — MCP lazy proxy (pass 1)
- Run
- Claude Code skill-creator skill
- Index
- Config
- Review: phase 5 — MCP lazy proxy (pass 2, adversarial quality)
- MCP lazy proxy (§10.5)
- applyEdit
- oauth_test.go
- ToolSpec
- Low
- Review: phase 5 — MCP lazy proxy (pass 3, post-fix re-review)
- skills_test.go
- Item
- LoadPrompts
- input_test.go
- retry_test.go
- EnvError
- Request
- Review: phase 7 — OAuth Providers, Upstream graphify PR, v0.1 (pass 2, adversarial)
- Compacted
- Resumed
- quick_validate.py
- NewUnjailed
- SteeringApplied
- StreamReset
- Phase 5 — MCP Lazy Proxy Implementation Plan
- Agent
- TextDelta
- Agent loop (§14: turn contract, steering, maxSteps wrap-up)
- parseIgnore
- ReadTracker
- ThinkingDelta
- Model/effort switching (SetModel, SetEffort)
- Warning
- Claude Code frontend-design skill
- pi mcp-scripting skill
- allowAll
- StartError
- YoloChanged
- AllowAll
- boundedWriter
- fakeCmds
- fakeSnap
- rootChecker
- format.go
- Review: phase 7 — OAuth Providers, Upstream graphify PR, v0.1 (pass 3, post-fix)
- Review: phase 7 — OAuth Providers, Upstream graphify PR, v0.1 (pass 1)
- .Compact
- Input
- auth_test.go
- registry_test.go
- Arrange-Act-Assert with xUnit/NSubstitute
- Writer
- fixture/main.go
- config.Version build stamp
- shell allowlist (shell.allow / DefaultShellAllow)
- Agent.Close — stops MCP servers, closes session writer at shutdown
- TestMCPNoSchemasInPromptAndZeroStarts
- github.com/adeotek/moca
- 2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)
- Usage
- v0.1.0 — unreleased (pending the §14 ship gate's live runs and the tag)
- runOneShot
- PromptInput
- LineReader
- trustModel
- txt
- run.sh
- example.com/totals

## God Nodes (most connected - your core abstractions)
1. `model` - 47 edges
2. `newScript()` - 46 edges
3. `run()` - 40 edges
4. `startTestWith()` - 35 edges
5. `sseServer()` - 34 edges
6. `Phase 4 context compaction & resume review - pass 1 (2026-10-05)` - 33 edges
7. `ReadFile()` - 31 edges
8. `Agent` - 30 edges
9. `Config` - 30 edges
10. `Phase 4 Plan: Context Manager, Compaction, Resume` - 30 edges

## Surprising Connections (you probably didn't know these)
- `No secrets needed — stdlib-only build` --semantically_similar_to--> `Stdlib-only constraint — hand-rolled HTTP+SSE`  [INFERRED] [semantically similar]
  .github/workflows/ci.yml → docs/plans/phase-1-skeleton-providers.md
- `rtk skill allowlist rule (the wrapped command must also be allowlisted)` --conceptually_related_to--> `rtk cannot launder a non-allowlisted command`  [INFERRED]
  internal/skills/builtin/rtk/SKILL.md → docs/plans/phase-6-rtk-graphify-ecosystem.md
- `parseArgs()` --calls--> `ParseEffort()`  [EXTRACTED]
  cmd/moca/cli.go → internal/llm/types.go
- `TestHelpListsSurface()` --calls--> `run()`  [INFERRED]
  cmd/moca/cli_test.go → internal/tools/read_test.go
- `TestMCPImportCLI()` --calls--> `Parse()`  [EXTRACTED]
  cmd/moca/cli_test.go → internal/config/config.go

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

## Communities (107 total, 30 thin omitted)

### Community 0 - "start.go"
Cohesion: 0.09
Nodes (36): Agent loop, Context compaction, Crash repair of unanswered tool calls, Phase 1 skeleton & providers review (2026-10-04), Phase 2 tools & agent loop review (2026-10-05), Phase 4 context compaction & resume review - pass 2 (2026-10-05), Phase 4 context compaction & resume review - pass 1 (2026-10-05), Phase 4 context compaction & resume review - pass 3 (2026-10-06) (+28 more)

### Community 1 - "newScript"
Cohesion: 0.08
Nodes (66): scriptServer, newScript(), startTest(), startTestWith(), startWith(), TestAbortWritesSyntheticResults(), TestFailedCallDoesNotCancelRest(), TestLengthStopTruncatedToolCall() (+58 more)

### Community 2 - "permissions/shell.go"
Cohesion: 0.07
Nodes (41): canonical(), Jail, NewJail(), resolveDeep(), setup(), TestJail(), TestJailDanglingSymlinks(), TestJailKeepsNotYetExistingReadOnlyRoot() (+33 more)

### Community 3 - "Phase 4 Plan: Context Manager, Compaction, Resume"
Cohesion: 0.08
Nodes (46): /review-phase slash command, Phase review document (docs/reviews/), Phase 4 Plan: Context Manager, Compaction, Resume, agent.Agent.Compact (manual + automatic compaction), agent.build (shared Start/Resume construction), agent.Compacted event, agent.ErrNothingToCompact, agent.maybeCompact (trigger check + loop guard) (+38 more)

### Community 4 - "startStdio"
Cohesion: 0.13
Nodes (21): TestResolveEnv(), ResolveEnv(), initialize(), shServer(), skipWindows(), TestHTTPCloseBounded(), TestKeepNameWindows(), TestStdioGrandchildHoldingStdoutDoesNotHangExit() (+13 more)

### Community 5 - "anthropic_test.go"
Cohesion: 0.13
Nodes (39): SplitModel(), TestSplitModel(), Adapter, newAnthropic(), keyCred(), mustJSON(), sseServer(), TestAnthropicBodyHygiene() (+31 more)

### Community 6 - "testing.T"
Cohesion: 0.05
Nodes (54): NormalizeToolID(), TestNormalizeToolID(), TestTransformDropsEmptiedAssistantAndMerges(), TestTransformDropsRedactedForeignThinking(), TestTransformHistory(), TestTransformSameBareIDDifferentProvider(), TransformHistory(), CapChars() (+46 more)

### Community 7 - "context.Context"
Cohesion: 0.12
Nodes (13): CallResult, lockFile(), lockFile(), approve(), decode(), errorf(), Env, Result (+5 more)

### Community 8 - "phase-6-rtk-graphify-ecosystem.md"
Cohesion: 0.10
Nodes (15): rtkRunTarget, rtkSub (flags before the subcommand are skipped), rtkTarget, skipWrapper, unwrap (shell analyser), SKILL.md (builtin rtk), internal/skills/testdata/ecosystem corpus, anthropics/skills frontend-design (+7 more)

### Community 9 - "NewManager"
Cohesion: 0.13
Nodes (34): crashingServer(), TestCallTimeoutStopsWedgedServer(), TestHandshakeTimeout(), TestHTTPAbortedStreamReplayRule(), TestIndexSaveMergesConcurrentWriters(), TestListToolsRepeatedCursor(), TestSearchReportsAndRemembersFailedServer(), TestSearchReportsUnsavedIndex() (+26 more)

### Community 10 - "runLogin"
Cohesion: 0.19
Nodes (16): parseArgs(), printUsage(), TestParseArgs(), runLogin(), runLogout(), yesNo(), main(), run() (+8 more)

### Community 11 - "Phase 3 - TUI Shell, Slash Commands, Status Bar, Steering, Model Switching Implementation Plan"
Cohesion: 0.08
Nodes (30): allow-always persistence (shell.allow), Interactive approval prompt (blocking Asker), Bubble Tea glue (internal/tui/app.go), Bubble Tea v2 (charm.land/bubbletea/v2), LoadPrompts / ExpandPrompt (internal/skills/prompts.go), Numbered collapsible tool/thinking items + pager, ParseInput command parsing (internal/tui/commands.go), Paste chips [paste N lines #K] (+22 more)

### Community 12 - "internal/config — JSONC config parsing, defaults, validation"
Cohesion: 0.08
Nodes (30): CI Workflow (gofmt · vet · build · test), CI gate set (gofmt, go vet, go build, go test -race), go.mod (Go version source for setup-go), SPECS.md §2 — CI gate definition, Phase 1 — Skeleton, Protocol Adapters, Streaming, JSONC Config (implementation plan), Adapter interface — Stream(ctx, req, emit), anthropic-messages adapter, ClampEffort / DefaultEffort (+22 more)

### Community 13 - "Parse"
Cohesion: 0.07
Nodes (38): scanner, span, Parse(), TestDefaultsApplied(), TestEnvRefs(), TestOAuthAllowedForOpenAI(), TestOAuthRejectedForNonOAuthProvider(), TestRetentionZeroMeansForever() (+30 more)

### Community 14 - "sync.Mutex"
Cohesion: 0.10
Nodes (12): responseID(), TestResponseID(), callResult, KillProcessGroup(), SetProcessGroup(), KillProcessGroup(), SetProcessGroup(), httpTransport (+4 more)

### Community 15 - "model"
Cohesion: 0.18
Nodes (4): println(), printlnContent(), compactDoneMsg, model

### Community 16 - "run"
Cohesion: 0.08
Nodes (44): fakeCompletions(), isolate(), TestExitCodes(), TestHelpListsSurface(), TestInterruptedExit130(), TestLoginLogoutCLI(), TestMCPImportCLI(), TestNoApproveSkipsProjectInstructions() (+36 more)

### Community 17 - "Phase 2 Plan - Seven Tools, Agent Loop, Permissions, Skills, Session Schema"
Cohesion: 0.10
Nodes (26): Agent Loop (model - tools - model), AGENTS.md / CLAUDE.md Project Instructions, Embedded Built-in Skills Extraction (ExtractBuiltins), DESIGN.md rev 11 Specification, Edit Ladder (exact / whitespace-insensitive / replace_all), Pure-Go Gitignore Matcher, Append-Only JSONL Session Transcript, maxSteps Wrap-Up (tool_choice none) (+18 more)

### Community 18 - "cut_test.go"
Cohesion: 0.18
Nodes (22): FindCut(), a(), call(), res(), TestFindCutKeepsRecentAtValidPoint(), TestFindCutNeverBetweenCallAndResult(), TestFindCutNothingToDo(), TestFindCutSplitTurn() (+14 more)

### Community 19 - "Create"
Cohesion: 0.13
Nodes (19): Find(), FindForWorkdir(), idOf(), listNewestFirst(), TestFind(), TestFindForWorkdirViaSymlink(), TestFindMtimeTieDeterministic(), TestFindNoSessions() (+11 more)

### Community 20 - "newAgentModel"
Cohesion: 0.19
Nodes (21): key(), keyMsg(), TestCompactEscapeCancels(), TestCompactingBlocksRun(), TestCompactNothing(), TestCompactedAndResumedLines(), TestAllowAlwaysMCPPersists(), TestAllowAlwaysPersists() (+13 more)

### Community 21 - "Phase 3 TUI review - pass 1 (2026-10-05)"
Cohesion: 0.16
Nodes (17): Approval prompt flow, Phase 3 TUI review - pass 2 (2026-10-05), Phase 3 TUI review - pass 3 (2026-10-05), Phase 3 TUI review - pass 4 (2026-10-05), Phase 3 TUI review - pass 1 (2026-10-05), TUI event pipe with pre-start buffering, Exit code mapping (exitFor), Hard mode model switching (+9 more)

### Community 22 - "ToolCall"
Cohesion: 0.19
Nodes (6): ToolEnd, ToolStart, Outcome, ToolCall, ToolChoice, runDoneMsg

### Community 23 - "internal/mcp Manager — lazy lifecycle, idle stop, IndexAll"
Cohesion: 0.15
Nodes (13): internal/mcp Manager — lazy lifecycle, idle stop, IndexAll, internal/mcp ProxyTool — Spec/run/Allowed gating, config.AppendString — comment-preserving JSONC string-array append, MCP call gating (readOnlyHint/destructiveHint, approve lists, allow-always persistence), MCP Manager lifecycle (lazy ensure, idle stop, busy counter, restart-once replay), The `mcp` proxy tool (search/describe/call rendering, 30K truncation, frozen spec), Path jail (canonical root + read-only roots, symlink resolution), Parsed shell analysis incl. rtk unwrap (phase 6) (+5 more)

### Community 24 - "MCPServer"
Cohesion: 0.14
Nodes (19): MCPConfig, MCPServer, TestImportRewritesEmbeddedSecretsInValues(), TestRewriteSecretsValueRuleAlone(), DiscoverSources(), embeddedSecret(), envName(), literalSecretFields() (+11 more)

### Community 25 - "Manager"
Cohesion: 0.20
Nodes (8): ConfigHash(), Rank(), TestIndexPersistAndHash(), TestRank(), words(), Manager, Hit, state

### Community 26 - "Review: phase 6 — rtk, graphify, Skills Ecosystem Compatibility (pass 3)"
Cohesion: 0.13
Nodes (14): 1. `rtkSelf` classifies native-utility proxies as plain `rtk`, so `rtk find -exec …` (and `rg --pre`, `ast-grep -U`, `tree -o`) bypass a user's allowlist — Confirmed, 2. Mutating rtk-own subcommands (`init -g`, `trust -y`, `config`, `learn --write-rules`) are allowed with no prompt and write outside the jail — Confirmed (analyser behaviour); effects per `--help`, not executed, 3. Frontmatter block-scalar headers with chomping/indent indicators or a trailing comment (`>+`, `|+`, `>2`, `> # note`) silently become the description — Confirmed, Checks run, Earlier findings, Findings, High, Low (+6 more)

### Community 27 - "Start"
Cohesion: 0.17
Nodes (16): setup, GitState(), Platform(), build(), StartOptions, prepare(), Start(), TestPathsHonourXDG() (+8 more)

### Community 28 - "Low"
Cohesion: 0.20
Nodes (10): 1. The retry-once replays a `tools/call` that may already have executed — Confirmed (plan-prescribed behaviour), 2. An explicit `"args": null` is sent to the server as `"arguments": null` — Confirmed, 3. `Search` hides a failing server's error and re-spawns it on every search — Confirmed, 4. stdio handshake/calls and the pagination loop have no bound other than ctx — Confirmed by reading, 5. `Index.Save` is not safe across concurrent moca processes — Plausible, 6. A negative `mcp.idleTimeout` is accepted by validation — Confirmed by reading, 7. Test gaps and tests that cannot fail — Confirmed, 8. `FilterEnv` matches names case-sensitively, so Windows stdio servers get almost no environment — Plausible (+2 more)

### Community 29 - "Agent"
Cohesion: 0.12
Nodes (11): Options, savedModel, Agent, New(), Effort, ParseEffort(), TestParseEffort(), Model (+3 more)

### Community 30 - "session/entry.go"
Cohesion: 0.33
Nodes (9): Entry, ModelChange, header(), Compaction, ErrorInfo, Header, PermissionMode, SnapshotRec (+1 more)

### Community 31 - "Low"
Cohesion: 0.20
Nodes (10): 1. `Manager.Call`'s dead-transport retry on an HTTP transport can restart the same server concurrently with itself across `Search`/`Describe`/`Call` from different goroutines — Plausible, 1. The retry predicate string-matches error text; a server error message containing the substring "exited" (or "session expired") spuriously restarts a healthy server, and any in-flight sibling calls on the same transport are failed — Confirmed, 2. `Index.LoadIndex` silently ignores a corrupt file's parse error, so a partially-parsed index can serve stale entries — Plausible, 2. The stdio transport closes the process at Close (`stdin` close, then kill), but `Call`'s write-error retry branch can return `ctx.Err()` without ever surfacing the exit error, skipping the exactly-once retry the plan promises — Confirmed, 3. `envName` can collide two distinct env/headers keys onto one export variable — Plausible, 4. `Run`'s search output truncates descriptions by bytes, not runes — Plausible (edge), 5. `stdioTransport.Close`'s `<-t.done` waits for the child exit unboundedly after `SIGKILL` — Plausible (edge), Findings (+2 more)

### Community 32 - ".Run"
Cohesion: 0.20
Nodes (10): cutRunes(), errResult(), isTrue(), TestCutRunesSafe(), runePrefix(), runeSuffix(), TestTruncate(), TestTruncateLongLines() (+2 more)

### Community 33 - "Message"
Cohesion: 0.33
Nodes (9): Kind, EntryTokens(), Entry, MessageChars(), RequestChars(), TestEstimate(), Tokens(), UsageTokens() (+1 more)

### Community 34 - "Sanitize"
Cohesion: 0.13
Nodes (13): ShellOutput, AutoAllow(), Answer, Question, approvalPrompt(), firstLineOf(), TestApprovalPromptSanitized(), hasControl() (+5 more)

### Community 35 - "Review: phase 5 — MCP lazy proxy (pass 1)"
Cohesion: 0.25
Nodes (8): Checks run, Earlier findings, Plan conformance, Review Focus, Review: phase 5 — MCP lazy proxy (pass 1), Suggested order of fixes, Summary, Verdict

### Community 36 - "Run"
Cohesion: 0.14
Nodes (13): Event, Asker, newModel(), Run(), runResult(), newAsker(), newEventPipe(), TestEventPipeBuffersUntilStarted() (+5 more)

### Community 37 - "Claude Code skill-creator skill"
Cohesion: 0.16
Nodes (13): Skill description (trigger) optimization, Skill eval/iterate loop (with-skill vs baseline), skill-creator Apache 2.0 License (Anthropic), Claude Code skill-creator skill, SKILL.md anatomy + bundled resources layout, Constrained query expansion against graph vocab, Progressive Disclosure (three-level skill loading), graphify query/path/explain reference (+5 more)

### Community 38 - "Index"
Cohesion: 0.20
Nodes (6): Annotations, client, Index, IndexEntry, IndexTool, Tool

### Community 39 - "Config"
Cohesion: 0.13
Nodes (13): ContextConfig, ModelOverride, ProviderConfig, ShellConfig, SnapshotConfig, Default(), Config, CostConfig (+5 more)

### Community 40 - "Review: phase 5 — MCP lazy proxy (pass 2, adversarial quality)"
Cohesion: 0.25
Nodes (8): Checks run, Earlier findings, Plan conformance (brief — pass 1's area), Review Focus, Review: phase 5 — MCP lazy proxy (pass 2, adversarial quality), Suggested order of fixes, Summary, Verdict

### Community 41 - "MCP lazy proxy (§10.5)"
Cohesion: 0.14
Nodes (15): internal/mcp importer — DiscoverSources/ParseSource/RewriteSecrets/Plan, internal/mcp startHTTP — streamable HTTP transport, internal/mcp startStdio + FilterEnv/serverEnv, JSONC config pre-pass + comment-preserving edits (AppendString/SetObjectEntry), config.SetObjectEntry — JSONC object-key insert (phase 5), env:VAR lazy credential indirection, MCP persisted discovery index (mcp-index.json, ConfigHash, word-token Rank), `moca mcp import` importer (DiscoverSources, secret rewriting, SetObjectEntry writes) (+7 more)

### Community 42 - "applyEdit"
Cohesion: 0.23
Nodes (14): applyEdit(), formatDiff(), indentOf(), joinInts(), lineOf(), matchLines(), replaceExact(), replaceFuzzy() (+6 more)

### Community 43 - "oauth_test.go"
Cohesion: 0.06
Nodes (47): fakeSIWC(), newHostID(), audContains(), b64d(), OAuthConfig, jwksKey(), Login(), missingScopes() (+39 more)

### Community 44 - "ToolSpec"
Cohesion: 0.08
Nodes (17): ToolSpec, TestProxySpecFrozen(), Builtins(), MCPSpec(), Registry, NewRegistry(), TestRegistryDispatch(), TestSchemasFrozen() (+9 more)

### Community 45 - "Low"
Cohesion: 0.15
Nodes (13): 1. A streamable-HTTP server that opens an SSE response and stalls hangs `mcp` calls indefinitely — Confirmed, 1. `Notify` returns success for any HTTP status, masking a dead session at the worst moment — Confirmed, 2. `envName` collisions: two distinct keys can map to the same exported variable — Confirmed, 2. `moca mcp import` copies literal secrets that live outside `env`/`headers` — Confirmed, 3. A connection-aborted SSE response skips the designed restart-once — Confirmed, 3. Search-result description truncation splits UTF-8 runes — Confirmed, 4. `Manager.Close` concurrent with a failing in-flight call respawns a server after shutdown — Confirmed by trace, 5. Error paths of both transports have no unit tests — Confirmed (+5 more)

### Community 46 - "Review: phase 5 — MCP lazy proxy (pass 3, post-fix re-review)"
Cohesion: 0.15
Nodes (13): 1. stdio exit handling races `cmd.Wait()` against the stdout reader: real replies and the server's own diagnostics can be lost — Confirmed, 2. `moca mcp import` still copies literal credentials held in `env`/`headers` values whose key is not secret-shaped, and prints them in the preview — Confirmed, 3. HTTP transport `Close` has no bound short of the 15-minute client timeout, and runs under the server lock — Confirmed, Checks run, Earlier findings, Findings, Medium, Plan conformance (+5 more)

### Community 47 - "skills_test.go"
Cohesion: 0.23
Nodes (10): builtinHash(), ExtractBuiltins(), extractBuiltins(), TestEcosystemSkillsLoadUnchanged(), Discover(), mkSkill(), TestDiscoverPrecedence(), TestDiscoverReportsSymlinkedDir() (+2 more)

### Community 48 - "Item"
Cohesion: 0.16
Nodes (5): newPager(), TestTeaAPI(), Item, Items, pagerModel

### Community 49 - "LoadPrompts"
Cohesion: 0.20
Nodes (12): ExpandPrompt(), Prompt, LoadPrompts(), TestEcosystemPromptsLoad(), TestExpandPrompt(), TestLoadPrompts(), HelpText(), ParseInput() (+4 more)

### Community 50 - "input_test.go"
Cohesion: 0.30
Nodes (11): NewInput(), TestCollapseUsesExactOffset(), TestCtrlC(), TestEditedMarkerSentLiterally(), TestHistory(), TestHistoryRecallChipsUnsafeContent(), TestMarkerNeverCollidesWithTypedText(), TestPasteChipKeepsContent() (+3 more)

### Community 51 - "retry_test.go"
Cohesion: 0.08
Nodes (36): Retry, Event, fakeHTTPServer(), TestHTTPTransport(), post(), http2Transient(), newHTTPError(), retryable() (+28 more)

### Community 53 - "Request"
Cohesion: 0.14
Nodes (18): ContentBlock, Request, Response, StopReason, ToolResult, anthropicOverflow(), anthropicStop(), openaiOverflow() (+10 more)

### Community 54 - "Review: phase 7 — OAuth Providers, Upstream graphify PR, v0.1 (pass 2, adversarial)"
Cohesion: 0.08
Nodes (23): 10. Login declines write both callback responses in the first-come race, but no test covers the "paste arrives while callback write is in flight" ordering — Confirmed by reading; no probe (dedup is correct, the missing test is the gap), 1. `randText`'s failure path silently returns an empty state/nonce — and no test pins the entropy of state/nonce — Confirmed (mutation survived the whole provider suite), 2. `cmd/moca`'s exit-2 mapping for `ErrInvalidGrant`/`ErrInvalidClient` is untested — mutation survived, Confirmed, 3. A pre-existing `<path>.tmp` file's mode is inherited through the store's rename — a rotated token can land at 0644 — Confirmed (probe), 4. The registered client-id check trusts the callback's `client_id` echoed over the open redirect — mutation-proven load-bearing but the spec-recommended rejection is fragile on reauth — Rejected CallerId is Confirmed (behaviour verified against fakeAS only), 5. `SetString` on a config containing duplicate provider objects silently mutates the *first* occurrence while every consumer reads the *last* — Confirmed (probe), 6. `SetString` accepts a JSON-keyed value `auth` of any type and overwrites it (mutating 42→"oauth"), bypassing the "string" promise in its name — Confirmed (probe), 7. `SetObjectEntry`'s missing-key fallback into a contain-alias file can still produce a symlinked `auth.json` under the store's path — Low, spec-widening (+15 more)

### Community 58 - "NewUnjailed"
Cohesion: 0.29
Nodes (4): Agent, NewUnjailed(), TestUnjailedAndAllowAll(), Unjailed

### Community 61 - "Phase 5 — MCP Lazy Proxy Implementation Plan"
Cohesion: 0.27
Nodes (9): Phase 5 — MCP Lazy Proxy Implementation Plan, internal/mcp client — initialize, paged listTools, callTool, internal/mcp jsonrpc.go — request/response/rpcError types, transport interface, internal/mcp sse.go readEvents — minimal SSE reader, Review pass 2 (adversarial quality), moca SPECS.md (current implemented state), MCP JSON-RPC 2.0 protocol handling (initialize, tools/list paging, tools/call), Retry/error taxonomy (DefaultRetryPolicy, mid-stream reset retry, stall timeout) (+1 more)

### Community 64 - "Agent loop (§14: turn contract, steering, maxSteps wrap-up)"
Cohesion: 0.40
Nodes (4): Agent loop (§14: turn contract, steering, maxSteps wrap-up), Context manager compaction (budgets, cut points, structured summary, overflow recovery), Cross-provider thinking replay transform + NormalizeToolID, Session resume (--resume/--continue, repair, single-writer flock)

### Community 65 - "parseIgnore"
Cohesion: 0.40
Nodes (5): globToRegexp(), ignored(), parseIgnore(), TestIgnore(), ignoreRule

### Community 66 - "ReadTracker"
Cohesion: 0.47
Nodes (4): NewReadTracker(), stampOf(), ReadTracker, stamp

### Community 68 - "Model/effort switching (SetModel, SetEffort)"
Cohesion: 0.50
Nodes (4): /hard toggle (modelHard + high effort), model_change session entry, Model/effort switching (SetModel, SetEffort), NormalizeToolID (deterministic tool-call id normalization)

### Community 70 - "Claude Code frontend-design skill"
Cohesion: 0.50
Nodes (3): AI-generated design default tells, frontend-design Apache 2.0 License, Claude Code frontend-design skill

### Community 71 - "pi mcp-scripting skill"
Cohesion: 0.67
Nodes (4): jev.evaluate reference, jev.evaluate semantic scripting gate, mcpScript tools API, pi mcp-scripting skill

### Community 80 - "format.go"
Cohesion: 0.14
Nodes (18): TestGitStatusUsesNoOptionalLocks(), TestStatusWideRunesNeverExceedWidth(), AbbrevEffort(), AbbrevHome(), FmtTokens(), FmtWindow(), GitBranch(), gitStatusArgs() (+10 more)

### Community 81 - "Review: phase 7 — OAuth Providers, Upstream graphify PR, v0.1 (pass 3, post-fix)"
Cohesion: 0.10
Nodes (20): 1. The ship-gate checker can pass while the fixture's tests still fail — Confirmed (probe), 2. `Registry.CheckCredential` now does a network refresh on the TUI's synchronous `/model` path, with no ctx and no timeout — Confirmed (probe + trace), 3. `moca login` can swallow the answer to its own "Switch it now? [y/N]" prompt — Confirmed (probe), 4. The refresh path can block uncancellably: lock wait ignores ctx, token endpoint has no timeout — Confirmed (probe for the lock; trace for the client), 5. A transient token-endpoint failure is not retried, and a still-valid access token is not used — Confirmed (trace), 6. SPECS and the plan's notes misdescribe four behaviours — Confirmed, 7. CHANGELOG still presents `v0.1.0` as a dated, released version — Confirmed, 8. Untested paths around login: Review Focus 5's wait-then-paste, the open-failure message, and `runLogin`'s success path — Confirmed (absence) (+12 more)

### Community 82 - "Review: phase 7 — OAuth Providers, Upstream graphify PR, v0.1 (pass 1)"
Cohesion: 0.10
Nodes (19): 1. `Store.write` publishes a pre-existing `<path>.tmp`'s mode through the rename — the 0600 token-store guarantee can be broken — Confirmed (probe), 2. `randText` swallows the entropy error — state and nonce can silently become the empty string, fail-opening both anti-replay checks — Confirmed (code trace; all existing tests pass either way), 3. The `ErrInvalidGrant`/`ErrInvalidClient` → exit 2 mapping has no test — Confirmed (absence), 4. `auth: "oauth"` on a provider that has no OAuth (e.g. `opencode-go`) passes config validation and fails at request time with exit 1, though the remedy is a config edit — Confirmed (trace), 5. README status line previewing "v0.1.0" while the tag is explicitly deferred — Confirmed (docs/code wrinkle), 6. `run.sh` leaks its three temp dirs per run, and the checker accepts any `_test.go`/any-search before the edit (looser than §14's wording) — Confirmed (hygiene + checker-scope notes), Checkpoint adaptation note, Checks run (+11 more)

### Community 83 - ".Compact"
Cohesion: 0.19
Nodes (8): Agent, summarizerUnavailable(), Budget, NewBudget(), TestBudget(), Prev, Summarizer, TextOf()

### Community 85 - "auth_test.go"
Cohesion: 0.23
Nodes (15): NewStore(), TestConcurrentRefreshOnce(), TestCredentialForLockWaitHonoursContext(), TestDeleteClearsCorruptStore(), TestHostIDStable(), TestInvalidClient(), TestLockFileWaitsThenAcquires(), TestNotLoggedIn() (+7 more)

### Community 86 - "registry_test.go"
Cohesion: 0.33
Nodes (15): NewRegistry(), mustCfg(), TestBaseURLPrecedence(), TestBuiltinModelNeedsProtocol(), TestBuiltinProviderNewModelWithModelProtocol(), TestCheckCredentialOAuthDoesNotTouchNetwork(), TestCustomProviderAndLazyMissingKey(), TestNoSessionHeaderOutsideOpenCodeGo() (+7 more)

### Community 88 - "Writer"
Cohesion: 0.15
Nodes (7): lockFile(), lockFile(), gitClean(), Snapshots, NewSnapshots(), Writer, lastNewline()

### Community 89 - "fixture/main.go"
Cohesion: 0.21
Nodes (7): Sum(), TestTotals(), ParseList(), Average(), Max(), TestMaxAndAverage(), main()

### Community 97 - "2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)"
Cohesion: 0.15
Nodes (12): 1.1 Terms, 1.2 Client registration, 1.3 Decision, 1. anthropic — decision: `api_key only` (subscription OAuth not permitted), 2.1 Terms, 2.2 Client registration (per the docs), 2.3 Endpoints (verified live 2026-10-06 against `https://auth.openai.com/.well-known/openid-configuration`), 2.4 Inference contract for subscription traffic (open-source flow docs) (+4 more)

### Community 98 - "Usage"
Cohesion: 0.17
Nodes (6): Status, TurnEnd, fakeSum, Result, Usage, readEvents()

### Community 99 - "v0.1.0 — unreleased (pending the §14 ship gate's live runs and the tag)"
Cohesion: 0.18
Nodes (10): Changelog, CLI, Context & sessions, External tools & the skills ecosystem, MCP, OAuth (subscription login), Providers & protocols, Tools & permissions (+2 more)

### Community 100 - "runOneShot"
Cohesion: 0.27
Nodes (8): TestExitForOAuthSentinels(), exitFor(), firstLine(), resolveResume(), runOneShot(), decideTrust(), hasProjectResources(), runTUI()

### Community 101 - "PromptInput"
Cohesion: 0.22
Nodes (9): PromptInput, ServerLine, BuildSystemPrompt(), oneLine(), TestBuildSystemPrompt(), Instruction, LoadInstructions(), TestLoadInstructions() (+1 more)

### Community 102 - "LineReader"
Cohesion: 0.25
Nodes (7): LineReader, parsePasted(), pasteReader(), TestParsePasted(), readLine(), lineResult, pastedResult

### Community 104 - "txt"
Cohesion: 0.50
Nodes (4): Role, TestMessagesRebuild(), TestRepair(), txt()

## Ambiguous Edges - Review These
- `writer.go` → `Single-writer session locking`  [AMBIGUOUS]
  docs/reviews/2026-10-06-phase-4-context-compaction-resume.md · relation: conceptually_related_to
- `Model catalog (builtinCatalog)` → `Thinking modes none|budget|adaptive|openai`  [AMBIGUOUS]
  docs/plans/phase-1-skeleton-providers.md · relation: conceptually_related_to

## Knowledge Gaps
- **205 isolated node(s):** `usageError`, `github.com/adeotek/moca`, `claudeServer`, `opencodeServer`, `request` (+200 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 345 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **30 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **What is the exact relationship between `writer.go` and `Single-writer session locking`?**
  _Edge tagged AMBIGUOUS (relation: conceptually_related_to) - confidence is low._
- **Why does `moca DESIGN.md — v1 contract (rev 11)` connect `Phase 4 Plan: Context Manager, Compaction, Resume` to `phase-6-rtk-graphify-ecosystem.md`, `Phase 5 — MCP Lazy Proxy Implementation Plan`?**
  _High betweenness centrality (0.073) - this node is a cross-community bridge._
- **What connects `usageError`, `github.com/adeotek/moca`, `claudeServer` to the rest of the system?**
  _205 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `start.go` be split into smaller, more focused modules?**
  _Cohesion score 0.08932806324110672 - nodes in this community are weakly interconnected._
- **What is the exact relationship between `Model catalog (builtinCatalog)` and `Thinking modes none|budget|adaptive|openai`?**
  _Edge tagged AMBIGUOUS (relation: conceptually_related_to) - confidence is low._
- **Why does `Review pass 2 (adversarial quality)` connect `Phase 5 — MCP Lazy Proxy Implementation Plan` to `Review: phase 5 — MCP lazy proxy (pass 2, adversarial quality)`, `start.go`, `Phase 4 Plan: Context Manager, Compaction, Resume`?**
  _High betweenness centrality (0.042) - this node is a cross-community bridge._
- **Should `newScript` be split into smaller, more focused modules?**
  _Cohesion score 0.081377151799687 - nodes in this community are weakly interconnected._