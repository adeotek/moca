# Graph Report - moca  (2026-10-09)

## Corpus Check
- 257 files · ~210,596 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 10 file(s) not represented in the graph (top: (none) 5, .jsonc 3, .v2 1)

## Summary
- 2769 nodes · 10390 edges · 146 communities (104 shown, 42 thin omitted)
- Extraction: 89% EXTRACTED · 11% INFERRED · 0% AMBIGUOUS · INFERRED: 1187 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `d434f0ea`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- quick_validate.py
- v0.1.0-alpha (2026-10-07)
- start.go
- context.Context
- TestResumeHonoursCompaction
- testRunCommand
- allowAll
- StartError
- findTestFile
- AllowAll
- askRootChecker
- update/update.go
- boundedWriter
- fakeCmds
- Compacted
- Resumed
- SteeringApplied
- StreamReset
- TextDelta
- ThinkingDelta
- Warning
- newAgentModel
- YoloChanged
- EnvError
- TestWelcomeLines
- Parse
- fakeSnap
- rootChecker
- newScript
- example.com/totals
- github.com/adeotek/moca
- NewManager
- Input
- anthropic_test.go
- prompt.go
- NewDefaultStore
- P2 — readability & feedback polish
- ToolSpec
- Agent
- Request
- NewRegistry
- oauth_test.go
- RenderStatus
- Agent
- build
- Sanitize
- tools registry — Tool interface, seven frozen tools, sequential execution in emitted order
- Item
- sync.Mutex
- drained
- ReadFile
- testing.T
- Manager
- model
- importer.go
- time.Duration
- run
- run
- anthropic provider (api_key only by policy, no OAuth)
- retry_test.go
- Model
- Registry
- Resume
- .openSessions
- permissions/shell.go
- Find
- .DiscoverOllama
- ollamaStart
- renderMarkdown
- Config
- initialize
- applyEdit
- newTestModel
- fixture/main.go
- charm.land/bubbletea/v2.Msg
- prepare
- 2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)
- .View
- .Run
- format.go
- model
- input_test.go
- httpTransport
- Messages
- Index
- runTUI
- parseIgnore
- cut_test.go
- LineReader
- NewUnjailed
- AGENTS.md — moca
- serverEnv
- validateIDToken
- install.sh script
- SOURCES.md
- Claude Code frontend-design skill
- pi mcp-scripting skill
- 1.2 Inline diff preview for edit/write items
- 2.3 Richer activity row
- compact.Budget.Trigger() as the red threshold
- history.go / history.jsonl store
- 2.2 One-row item lines
- OpenCode — comparison reference
- Arrange-Act-Assert with xUnit/NSubstitute
- 2.8 Echo !cmd like other input
- 2.5 Notify when unfocused (tui.notify: osc9/bell/off)
- Pi — comparison reference
- 2.7 Quieter first run (hints.json marker)
- 3.1 shift+tab cycles effort
- Claude Code skill-creator skill
- External tools: rtk, graphify, skills ecosystem
- CI Workflow (gofmt · vet · build · test)
- create-command.md — starter prompt template
- 1.6 Resume replay + /resume picker
- 1.1 Markdown-lite rendering of responses
- SPECS.md — moca implemented-state normative spec
- Never create a commit without the user's explicit approval
- Cache breakpoints: anthropic cache_control positions, ≤4 rolled forward each turn
- DESIGN revision 1: initial draft
- Parallel tool calls execute sequentially in emitted order (no cancellation of rest)
- Serialization for summarization: tool results truncated at 2K chars (COMPACTION NOTE lossy)
- moca -p "<prompt>" — one-shot agent run (stdin -p form)
- Phase 2: seven tools + agent loop + permissions + skills loader + session schema
- TUI: Bubble Tea + lipgloss, compact/dense, welcome lines
- openai provider (API key or ChatGPT subscription OAuth)
- /-dropdown autocomplete (built-ins + prompt templates, ≤8 rows, tab/enter/esc)
- opencode-go provider (zen gateway)
- v0.2.0-beta (2026-10-09): Ollama, saved / commands, self-update
- Context manager: token-denominated budgets, compaction, recovery
- internal/agent — system prompt, events, loop, compaction + resume, cross-provider transform, Start wiring
- anthropic-messages codec (POST /v1/messages, version header, x-api-key)
- Seven frozen tools for v1 (schema churn = v2 discussion)
- credential store ~/.config/moca/auth.json (0600, atomic writes, cross-process lock)
- DESIGN §12 example config fixture (phase-1 decode gate)
- v0.1.0-alpha (2026-10-07): first release
- moca update [--check] (newest published GitHub release, in-place swap)
- MCP gating — readOnlyHint && !destructiveHint || approve list || session allow; no name heuristics
- Architecture: package layout with strict import direction
- system prompt — built once per run, stored in the session, never rebuilt
- MCP Manager — zero start, lazy ensure, busy-guarded idle stop, replay-once rules
- moca login <provider> / TUI /login: credential wizard (OAuth or masked API key)
- agent events — TextDelta … TurnEnd/SteeringApplied/Compacted/Resumed

## God Nodes (most connected - your core abstractions)
1. `SPECS.md — moca implemented-state normative spec` - 378 edges
2. `model` - 73 edges
3. `newTestModel()` - 62 edges
4. `run()` - 52 edges
5. `newScript()` - 46 edges
6. `key()` - 44 edges
7. `newAgentModel()` - 39 edges
8. `sseServer()` - 36 edges
9. `startTestWith()` - 35 edges
10. `Parse()` - 34 edges

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

## Communities (146 total, 42 thin omitted)

### Community 81 - "v0.1.0-alpha (2026-10-07)"
Cohesion: 0.18
Nodes (10): CHANGELOG, CLI, Context & sessions, External tools & the skills ecosystem, MCP, OAuth (subscription login), Providers & protocols, Tools & permissions (+2 more)

### Community 0 - "start.go"
Cohesion: 0.11
Nodes (21): KillProcessGroup(), request, usageError, anthropicEvent, anthropicUsage, completionsChunk, Credential, ollamaTags (+13 more)

### Community 10 - "context.Context"
Cohesion: 0.08
Nodes (22): CallResult, lockFile(), lockFile(), approve(), TestInvestigationHint(), TestInvestigationRefusal(), askOutsideWrite(), decode() (+14 more)

### Community 100 - "TestResumeHonoursCompaction"
Cohesion: 0.50
Nodes (4): userText(), TestLiveEntriesMissingFirstKeptFallback(), mustJSONString(), TestResumeHonoursCompaction()

### Community 103 - "testRunCommand"
Cohesion: 0.50
Nodes (4): TestTestRunCommand(), TestTestRunFailed(), testRunCommand(), testRunFailed()

### Community 107 - "findTestFile"
Cohesion: 0.67
Nodes (3): findTestFile(), isPathByte(), TestFindTestFile()

### Community 11 - "update/update.go"
Cohesion: 0.08
Nodes (35): envOr(), githubToken(), runUpdate(), Compare(), comparePre(), ExtractBinary(), FindAsset(), FindChecksums() (+27 more)

### Community 12 - "newAgentModel"
Cohesion: 0.09
Nodes (42): keyMsg(), TestExitCommandQuits(), TestCompactEscapeCancels(), TestCompactingBlocksRun(), TestCompactNothing(), TestEditItemShowsDiff(), TestAllowAlwaysRequiresCtrl(), TestApprovalKeysAnswerAfterPause() (+34 more)

### Community 13 - "Parse"
Cohesion: 0.07
Nodes (41): scanner, span, Parse(), TestAPIKeyOptional(), TestDefaultsApplied(), TestEnvRefs(), TestOAuthAllowedForOpenAI(), TestOAuthRejectedForNonOAuthProvider() (+33 more)

### Community 14 - "newScript"
Cohesion: 0.13
Nodes (41): scriptServer, newScript(), startTest(), startTestWith(), startWith(), TestAbortWritesSyntheticResults(), TestFailedCallDoesNotCancelRest(), TestLengthStopTruncatedToolCall() (+33 more)

### Community 15 - "NewManager"
Cohesion: 0.12
Nodes (39): MCPServer, crashingServer(), shServer(), skipWindows(), TestCallTimeoutStopsWedgedServer(), TestHandshakeTimeout(), TestHTTPAbortedStreamReplayRule(), TestIndexSaveMergesConcurrentWriters() (+31 more)

### Community 17 - "Input"
Cohesion: 0.08
Nodes (13): stampOf(), appendHistory(), loadHistory(), rewriteHistory(), lineCount(), TestHistoryPersistence(), hasControl(), ReadTracker (+5 more)

### Community 18 - "anthropic_test.go"
Cohesion: 0.17
Nodes (36): Adapter, newAnthropic(), keyCred(), mustJSON(), sseServer(), TestAnthropicBodyHygiene(), TestAnthropicBudgetAndNoCache(), TestAnthropicBudgetFollowsEffort() (+28 more)

### Community 2 - "prompt.go"
Cohesion: 0.06
Nodes (58): PromptInput, ServerLine, BuildSystemPrompt(), filterSkills(), GitState(), oneLine(), Platform(), TestBuildSystemPrompt() (+50 more)

### Community 21 - "NewDefaultStore"
Cohesion: 0.11
Nodes (14): NewDefaultStore(), Headless(), OAuthProvider(), TestHeadlessDetectionSSH(), model, providerNames(), providerRow(), TestLoginRowForOllama() (+6 more)

### Community 22 - "P2 — readability & feedback polish"
Cohesion: 0.06
Nodes (29): 0. Fixed in this pass (already on `fix/polish`), 1.1 Markdown-lite rendering of responses — **M**, 1.2 Inline diff preview for `edit`/`write` items — **S/M**, 1.3 Context-pressure color in the status bar — **S**, 1.4 Argument completion in the `/` dropdown + interactive `/model` — **M**, 1.5 `@path` file mentions — **M**, 1.6 Resume shows where you left off; `/resume` picker — **M**, 1.7 Persistent input history + `ctrl+r` search — **S/M** (+21 more)

### Community 25 - "ToolSpec"
Cohesion: 0.08
Nodes (17): ToolSpec, TestProxySpecFrozen(), Builtins(), MCPSpec(), Registry, NewRegistry(), TestRegistryDispatch(), TestSchemasFrozen() (+9 more)

### Community 26 - "Agent"
Cohesion: 0.10
Nodes (10): Status, Agent, MessageChars(), RequestChars(), TestEstimate(), Tokens(), UsageTokens(), Message (+2 more)

### Community 27 - "Request"
Cohesion: 0.11
Nodes (21): ContentBlock, Event, Request, ToolResult, anthropicOverflow(), isDialFailure(), ollamaUnreachable(), openaiOverflow() (+13 more)

### Community 28 - "NewRegistry"
Cohesion: 0.13
Nodes (28): newFakeOllama(), ollamaRegistry(), TestNoOllamaNoProbe(), TestOllamaChatWithoutKey(), TestOllamaDiscovery(), TestOllamaDiscoveryVerifyConcurrent(), TestOllamaKeylessAndOptionalKey(), TestOllamaServerDown() (+20 more)

### Community 3 - "oauth_test.go"
Cohesion: 0.06
Nodes (44): fakeSIWC(), fakeSIWCOpts(), Store, newHostID(), validateAPIKey(), OAuthConfig, Login(), missingScopes() (+36 more)

### Community 30 - "RenderStatus"
Cohesion: 0.11
Nodes (23): AutoAllow(), Answer, Question, approvalPanel(), approvalPrompt(), TestApprovalPromptSanitized(), diffPreview(), TestDiffPreview() (+15 more)

### Community 32 - "Agent"
Cohesion: 0.14
Nodes (11): Options, ToolEnd, ToolStart, Agent, Outcome, New(), Response, ToolCall (+3 more)

### Community 33 - "build"
Cohesion: 0.08
Nodes (19): TurnEnd, Result, build(), EnvRefs(), StopReason, Usage, anthropicStop(), Entry (+11 more)

### Community 34 - "Sanitize"
Cohesion: 0.11
Nodes (11): choiceRow(), masked(), model, TestWindowRange(), model, windowRange(), Sanitize(), TestSanitize() (+3 more)

### Community 35 - "tools registry — Tool interface, seven frozen tools, sequential execution in emitted order"
Cohesion: 0.11
Nodes (21): run.sh script, frozen tool schemas — golden schemas.golden.json (v2 discussion to change), tools registry — Tool interface, seven frozen tools, sequential execution in emitted order, Jail.Approvable — resolves refused writes to the ask-write root, ask-write root ~/.config/moca/prompts (write/edit ask, never persistent), !cmd / !!cmd user-initiated shell (30 s, ShellEnv, 30K truncation; esc kills), config.EnvRefs — every referenced variable (shell hygiene input), edit ladder — exact match → replace_all → whitespace-insensitive re-indent → error (+13 more)

### Community 36 - "Item"
Cohesion: 0.14
Nodes (7): TestMatchRanges(), matchRanges(), newPager(), TestTeaAPI(), Item, Items, pagerModel

### Community 37 - "sync.Mutex"
Cohesion: 0.12
Nodes (11): TestStdioStdoutFatalAlwaysSurfaces(), callResult, startStdio(), TestStdioWriteEPIPE(), KillProcessGroup(), SetProcessGroup(), SetProcessGroup(), response (+3 more)

### Community 38 - "drained"
Cohesion: 0.18
Nodes (19): TestThinkingLinePrintsBeforeTheResponse(), drained(), newLoginModel(), TestLoginMarkedBackAndCancel(), TestLoginOAuthDeclinedFlipAndCancel(), TestLoginOAuthFlowAndAuthFlip(), TestLoginRefusedWhileRunning(), TestLoginStoresAPIKey() (+11 more)

### Community 39 - "ReadFile"
Cohesion: 0.16
Nodes (15): lockFile(), lockFile(), Create(), Writer, lastNewline(), newID(), Open(), ReadFile() (+7 more)

### Community 4 - "testing.T"
Cohesion: 0.04
Nodes (67): NormalizeToolID(), TestNormalizeToolID(), TestTransformDropsEmptiedAssistantAndMerges(), TestTransformDropsRedactedForeignThinking(), TestTransformHistory(), TestTransformSameBareIDDifferentProvider(), TransformHistory(), CapChars() (+59 more)

### Community 42 - "Manager"
Cohesion: 0.20
Nodes (8): ConfigHash(), Rank(), TestIndexPersistAndHash(), TestRank(), words(), Manager, Hit, state

### Community 43 - "model"
Cohesion: 0.16
Nodes (12): firstLineOf(), clampRunes(), model, matchCommands(), TestClampRunes(), mentionMatches(), rankFile(), TestMentionRanking() (+4 more)

### Community 45 - "importer.go"
Cohesion: 0.15
Nodes (19): TestImportRewritesEmbeddedSecretsInValues(), TestRewriteSecretsValueRuleAlone(), DiscoverSources(), embeddedSecret(), envName(), literalSecretFields(), ParseSource(), Plan() (+11 more)

### Community 46 - "time.Duration"
Cohesion: 0.11
Nodes (12): post(), newHTTPError(), retryAfter(), TestRetryAfterFormats(), TestRetryAfterParsed(), TestFmtElapsed(), fmtElapsed(), fmtThinkDuration() (+4 more)

### Community 5 - "run"
Cohesion: 0.07
Nodes (60): fakeCompletions(), isolate(), TestExitCodes(), TestHelpListsSurface(), TestInterruptedExit130(), TestLoginLogoutCLI(), TestMCPImportCLI(), TestNoApproveSkipsProjectInstructions() (+52 more)

### Community 50 - "run"
Cohesion: 0.22
Nodes (15): parseArgs(), printUsage(), TestParseArgs(), loginAPIKey(), runLogin(), runLogout(), yesNo(), run() (+7 more)

### Community 51 - "anthropic provider (api_key only by policy, no OAuth)"
Cohesion: 0.13
Nodes (13): moca — MO Coding Agent (binary = repo = module path), level map anthropicAdaptive {low,medium,high,max} (off/minimal clamp up), default base URLs (api.anthropic.com, api.openai.com/v1, opencode.ai/zen/go), anthropic claude-haiku-4-5 (200k ctx, budget), config.OAuthProviders pinned against provider.oauthProviders by a test, anthropic claude-opus-5-5 (1M ctx, adaptive), anthropic provider (api_key only by policy, no OAuth), openai provider (OAuth only on openai-responses) (+5 more)

### Community 52 - "retry_test.go"
Cohesion: 0.29
Nodes (16): Retry, DefaultRetryPolicy(), RetryNotice, fail(), TestMidStreamFailureResetsAndRetriesOnce(), TestMidStreamRetryBacksOffAndNotifies(), TestNoRetryOn400(), testPolicy() (+8 more)

### Community 56 - "Model"
Cohesion: 0.17
Nodes (6): savedModel, Effort, ParseEffort(), TestParseEffort(), Model, supportedEfforts()

### Community 57 - "Registry"
Cohesion: 0.17
Nodes (5): discoverLocal(), IsLocalProvider(), SplitModel(), TestSplitModel(), Registry

### Community 58 - "Resume"
Cohesion: 0.17
Nodes (14): Resume(), catalogConfig(), lastModelChange(), startCatalog(), TestResumeAfterCrashMidTool(), TestResumeEffortFlagOverridesStored(), TestResumeMissingWorkdir(), TestResumeNeverRestoresYolo() (+6 more)

### Community 59 - ".openSessions"
Cohesion: 0.19
Nodes (11): Delete(), TestDelete(), TestListAll(), Info, List(), ListAll(), TestListPreviewsAndFilters(), fmtAge() (+3 more)

### Community 6 - "permissions/shell.go"
Cohesion: 0.06
Nodes (42): setup, canonical(), Jail, NewJail(), resolveDeep(), setup(), TestJail(), TestJailApprovable() (+34 more)

### Community 60 - "Find"
Cohesion: 0.17
Nodes (16): canonical(), Find(), FindForWorkdir(), header(), headerAndPreview(), idOf(), InWorkdir(), list() (+8 more)

### Community 61 - ".DiscoverOllama"
Cohesion: 0.15
Nodes (12): ModelOverride, CostConfig, ProviderConfig, costM(), TestCostOf(), Registry, normalizeOllamaURL(), ollamaBaseURL() (+4 more)

### Community 62 - "ollamaStart"
Cohesion: 0.17
Nodes (11): ollamaServer(), ollamaStart(), TestOllamaDeclaredWindowIsQuiet(), TestOllamaReportedWindowIsQuiet(), TestOllamaStartErrors(), TestOllamaTinyWindowWarns(), TestStartWithOllamaModel(), abortingHTTPServer() (+3 more)

### Community 65 - "renderMarkdown"
Cohesion: 0.23
Nodes (12): codeBandStyle(), fenceLang(), renderInline(), renderMarkdown(), plainMD(), TestMarkdownBlocks(), TestMarkdownFence(), TestMarkdownSanitizes() (+4 more)

### Community 67 - "Config"
Cohesion: 0.18
Nodes (10): ContextConfig, MCPConfig, ShellConfig, SnapshotConfig, TUIConfig, Default(), Config, Load() (+2 more)

### Community 68 - "initialize"
Cohesion: 0.22
Nodes (9): initialize(), TestHTTPCloseBounded(), startHTTP(), fakeHTTPServer(), TestHTTPNotifyStatusChecked(), TestHTTPTransport(), TestStdioEarlyExit(), headerLog (+1 more)

### Community 69 - "applyEdit"
Cohesion: 0.23
Nodes (14): applyEdit(), formatDiff(), indentOf(), joinInts(), lineOf(), matchLines(), replaceExact(), replaceFuzzy() (+6 more)

### Community 7 - "newTestModel"
Cohesion: 0.06
Nodes (57): TestActivityLine(), TestCoalescedShrinkRemembersTheLastFlushedRow(), TestInputBoxGrowsAndShrinks(), TestPagerCloseDefersTheRelease(), TestRefuseRunningWhileOutputInFlight(), TestShrinkIsPaddedUntilTheGuardWindowPasses(), TestShrinkPaddingFitsTheScreen(), TestSpinnerTicksOnlyForTheLiveRun() (+49 more)

### Community 70 - "fixture/main.go"
Cohesion: 0.21
Nodes (7): Sum(), TestTotals(), ParseList(), Average(), Max(), TestMaxAndAverage(), main()

### Community 71 - "charm.land/bubbletea/v2.Msg"
Cohesion: 0.21
Nodes (8): Event, Asker, newAsker(), agentEvent(), flatten(), TestFenceSurvivesCommitLive(), agentEventMsg, eventPipe

### Community 73 - "prepare"
Cohesion: 0.23
Nodes (13): main(), TestDefaultHTTPClientBoundsHeaderWait(), defaultHTTPClient(), prepare(), TestPathsHonourXDG(), AuthFile(), ConfigDir(), ConfigFile() (+5 more)

### Community 76 - "2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)"
Cohesion: 0.17
Nodes (12): 1.1 Terms, 1.2 Client registration, 1.3 Decision, 1. anthropic — decision: `api_key only` (subscription OAuth not permitted), 2.1 Terms, 2.2 Client registration (per the docs), 2.3 Endpoints (verified live 2026-10-06 against `https://auth.openai.com/.well-known/openid-configuration`), 2.4 Inference contract for subscription traffic (open-source flow docs) (+4 more)

### Community 78 - ".Run"
Cohesion: 0.20
Nodes (10): cutRunes(), errResult(), isTrue(), TestCutRunesSafe(), runePrefix(), runeSuffix(), TestTruncate(), TestTruncateLongLines() (+2 more)

### Community 79 - "format.go"
Cohesion: 0.24
Nodes (11): TestGitStatusUsesNoOptionalLocks(), AbbrevEffort(), AbbrevHome(), FmtTokens(), FmtWindow(), GitBranch(), gitStatusArgs(), TestFormatters() (+3 more)

### Community 8 - "model"
Cohesion: 0.10
Nodes (11): ShellOutput, model, newModel(), Run(), hintSeen(), markHint(), AppOptions, compactDoneMsg (+3 more)

### Community 80 - "input_test.go"
Cohesion: 0.30
Nodes (11): NewInput(), TestCollapseUsesExactOffset(), TestCtrlC(), TestEditedMarkerSentLiterally(), TestHistory(), TestHistoryRecallChipsUnsafeContent(), TestMarkerNeverCollidesWithTypedText(), TestPasteChipKeepsContent() (+3 more)

### Community 82 - "httpTransport"
Cohesion: 0.25
Nodes (5): fakeSum, responseID(), TestResponseID(), readEvents(), httpTransport

### Community 84 - "Messages"
Cohesion: 0.25
Nodes (10): Role, appendUser(), LatestCompaction(), Messages(), Repair(), TestMessagesFromCompaction(), TestMessagesRebuild(), TestRepair() (+2 more)

### Community 85 - "Index"
Cohesion: 0.20
Nodes (6): Annotations, client, Index, IndexEntry, IndexTool, Tool

### Community 87 - "runTUI"
Cohesion: 0.27
Nodes (8): TestExitForOAuthSentinels(), exitFor(), firstLine(), resolveResume(), runOneShot(), decideTrust(), hasProjectResources(), runTUI()

### Community 88 - "parseIgnore"
Cohesion: 0.24
Nodes (7): globToRegexp(), ignored(), parseIgnore(), TestIgnore(), ListFiles(), model, ignoreRule

### Community 9 - "cut_test.go"
Cohesion: 0.09
Nodes (33): Kind, Agent, summarizerUnavailable(), Budget, NewBudget(), TestBudget(), FindCut(), a() (+25 more)

### Community 90 - "LineReader"
Cohesion: 0.25
Nodes (7): LineReader, parsePasted(), pasteReader(), TestParsePasted(), readLine(), lineResult, pastedResult

### Community 92 - "NewUnjailed"
Cohesion: 0.29
Nodes (4): Agent, NewUnjailed(), TestUnjailedAndAllowAll(), Unjailed

### Community 93 - "AGENTS.md — moca"
Cohesion: 0.25
Nodes (7): AGENTS.md — moca, Commands, Conventions & gotchas, Docs, graphify, Ground truth (read before changing behavior), Layout & dependency rules

### Community 94 - "serverEnv"
Cohesion: 0.29
Nodes (8): TestResolveEnv(), ResolveEnv(), TestKeepNameWindows(), FilterEnv(), keepName(), serverEnv(), TestFilterEnv(), TestServerEnvExplicitEntryReplacesInherited()

### Community 97 - "validateIDToken"
Cohesion: 0.40
Nodes (5): audContains(), b64d(), jwksKey(), validateIDToken(), idClaims

### Community 98 - "install.sh script"
Cohesion: 0.67
Nodes (5): die(), fetch(), note(), install.sh script, usage()

### Community 63 - "SOURCES.md"
Cohesion: 0.19
Nodes (7): anthropics/skills frontend-design, anthropics/skills skill-creator, graphify skill (graphify install, claude platform; byte-identical to pi variant), adeotek/dotfiles opencode dotnet-unit-testing skill, npm pi-mcp-adapter mcp-scripting skill, npm pi-subagents skill, internal/skills/testdata/ecosystem corpus

### Community 101 - "Claude Code frontend-design skill"
Cohesion: 0.50
Nodes (3): AI-generated design default tells, frontend-design Apache 2.0 License, Claude Code frontend-design skill

### Community 102 - "pi mcp-scripting skill"
Cohesion: 0.67
Nodes (4): jev.evaluate semantic scripting gate, mcpScript tools API, jev.evaluate reference, pi mcp-scripting skill

### Community 106 - "1.2 Inline diff preview for edit/write items"
Cohesion: 0.67
Nodes (3): Claude Code — comparison reference, diffPreview(detail, width, maxLines), 1.2 Inline diff preview for edit/write items

### Community 64 - "Claude Code skill-creator skill"
Cohesion: 0.16
Nodes (13): Skill description (trigger) optimization, Skill eval/iterate loop (with-skill vs baseline), SKILL.md anatomy + bundled resources layout, Constrained query expansion against graph vocab, Progressive Disclosure (three-level skill loading), Self-improving save-result/reflect loop, Fresh-context reviewer disposition, skill-creator Apache 2.0 License (Anthropic) (+5 more)

### Community 83 - "External tools: rtk, graphify, skills ecosystem"
Cohesion: 0.22
Nodes (7): graphify — codebase knowledge graph integration, internal/permissions/rtk.go — captured rtk subcommand classes, rtk — token-compressed CLI proxy, Cross-tool skills and prompt templates ecosystem, External tools: rtk, graphify, skills ecosystem, Built-in rtk skill (SKILL.md), External tools: rtk + graphify allowlisted with built-in skills

### Community 86 - "CI Workflow (gofmt · vet · build · test)"
Cohesion: 0.22
Nodes (9): go.mod (Go version source for setup-go), CI gate set (gofmt, go vet, go build, go test -race), CI Workflow (gofmt · vet · build · test), SPECS.md §2 — CI gate definition, mise provisions Go 1.27.1 (pinned in go.mod); make falls back to mise x go, CI gate: gofmt, go vet, go build, go test -race (.github/workflows/ci.yml), make build/test/vet/fmt/release; single-package test invocation, Ship gate: make build && bash test/shipgate/run.sh (//go:build shipgate tag) (+1 more)

### Community 95 - "create-command.md — starter prompt template"
Cohesion: 0.33
Nodes (3): Project commands dir .moca/prompts/, Personal commands dir {{prompts_dir}} (~/.config/moca/prompts/), create-command.md — starter prompt template

### Community 96 - "1.6 Resume replay + /resume picker"
Cohesion: 0.33
Nodes (6): agent package (Resume, History, SetEffort), 1.4 Argument completion in the / dropdown + interactive /model, /model picker marks (no key) via CheckCredential, picker type shared by login /model /resume, 1.6 Resume replay + /resume picker, session package (List, FindForWorkdir)

### Community 99 - "1.1 Markdown-lite rendering of responses"
Cohesion: 0.40
Nodes (4): ansi.Wrap (terminal ANSI wrapping), ansi.Wrap span re-open fix (targeted toggles), 1.1 Markdown-lite rendering of responses, styleBlock (pre-styled spans variant)

### Community 1 - "SPECS.md — moca implemented-state normative spec"
Cohesion: 0.03
Nodes (103): cmd/moca/main.go (entry, signals, testable run()), cmd/moca/tui.go (TUI path + trust), llm.ContentBlock (text/thinking/tool_use/tool_result), llm.Event (stream events), llm.ToolCall, llm.ToolResult, llm.Usage (input/cacheRead/cacheWrite/output), Model type (Provider, ID, Protocol, ContextWindow, MaxOutput, Cost, ThinkingMode) (+95 more)

### Community 16 - "moca -p "<prompt>" — one-shot agent run (stdin -p form)"
Cohesion: 0.07
Nodes (38): cmd/moca/cli.go (flags → Options), cmd/moca/oneshot.go (-p agent path), llm.Message, Agent.Close — stops live MCP servers and closes the session writer, agent.Resume — validate first, repair, restore model/effort, jail at header workdir, /clear — new session file, old stays resumable, resets yolo to startup flag, /exit //q ///quit — quit (cancels in-flight runs), /resume [id8] — picker over this directory’s sessions (+30 more)

### Community 19 - "Phase 2: seven tools + agent loop + permissions + skills loader + session schema"
Cohesion: 0.08
Nodes (33): graphify workflow rules: query/path/explain first, update . after code changes, Failing-test investigation protocol: edit refuses until read+search happened, ! / !! shell prefixes: user-initiated, bypass model-facing analysis, Per-provider baseUrl override + custom providers (vLLM/LM Studio/Ollama day 1), Built-in rtk skill (embed.FS, extracted to ~/.local/share/moca/builtin-skills/<version>), graphify: codebase-graph CLI (default allowlist; SKILL.md drop-in), MCP server roster: one line per configured server in the system prompt, Path jail anchored at session-start workdir (symlinks resolved, EvalSymlinks) (+25 more)

### Community 20 - "TUI: Bubble Tea + lipgloss, compact/dense, welcome lines"
Cohesion: 0.10
Nodes (32): Thinking blocks/signatures model-bound; cross-provider transforms in agent (SPECS §14), Approvals show the whole command (≤6 lines) + ctrl+o pages all of it, Argument completion for /model, /effort, /login, /logout, /show, /resume, Persistent prompt history (history.jsonl per workdir) + ctrl+r search, Markdown-lite response rendering (bold/code/headings/bullets/fenced blocks), Context pressure: % yellow at 70%, red where auto-compaction fires, Bracketed paste: verbatim, never auto-sent, [paste N lines] chip (alt+p), /copy: last assistant message to clipboard via OSC 52 (SSH-safe) (+24 more)

### Community 23 - "openai provider (API key or ChatGPT subscription OAuth)"
Cohesion: 0.11
Nodes (26): anthropic-messages protocol adapter (~150 LOC, hand-rolled HTTP+SSE), anthropic provider (api_key only; subscription OAuth not permitted), api_key auth mode (stored credential or env: reference), openai-completions protocol adapter, openai provider (API key or ChatGPT subscription OAuth), openai-responses protocol adapter, opencode-go provider (OpenCode Zen, mixed-protocol catalog: glm-5.3, glm-5.3-flash, minimax-m3, kimi-k3…), shell tool: stateless bash -c / pwsh -NoProfile, 30s→300s timeout, process-group kill, 30K truncation (+18 more)

### Community 24 - "/-dropdown autocomplete (built-ins + prompt templates, ≤8 rows, tab/enter/esc)"
Cohesion: 0.07
Nodes (27): model catalog (internal/provider/catalog.go), activity row ⠋ working… (spinner, elapsed, esc to interrupt), Agent.Status() — model, effort, anchored tokens, window, usage, cost, Sub/Hard/Yolo, @path completion (≤5000 indexed files, ranked, text sent as written), /effort [level] (also shift+tab wrap; refused while running), /hard — modelHard + high ↔ saved pair, /show <n> — open any item in the pager, usage-anchored context estimate — anchor + delta after each turn (+19 more)

### Community 29 - "opencode-go provider (zen gateway)"
Cohesion: 0.11
Nodes (28): level map basicThinking {low,medium,high} (gateway rejects off/minimal: kimi-k2.7-code, mimo-v2.6-*), opencode-go deepseek-v4.1-flash (openai-completions, 1M ctx), opencode-go deepseek-v4-flash (openai-completions, 1M ctx), opencode-go deepseek-v4-flash-vision-exp (vision experiment), opencode-go deepseek-v4-pro (off-peak rate, 1M ctx), level map deepseekProThinking {high,max}, level map deepseekThinking {low,high,max}, opencode-go glm-5.3 (1M ctx, $1.4/$4.4) (+20 more)

### Community 31 - "v0.2.0-beta (2026-10-09): Ollama, saved / commands, self-update"
Cohesion: 0.14
Nodes (25): Ollama fail-fast: server down says how to start; unpulled model says ollama pull, --resume/--continue replay last 3 turns; new /resume picker, Built-in & user-extensible model catalog (window, max out, cost, thinkingMode), /clear starts a new session file (old one stays resumable), Provider-qualified model ids (opencode-go/glm-5.3-flash), Resume: moca --resume <id8|last> / --continue (workdir→session mapping), install.sh: download, checksum-verify, install in place, install.ps1: Windows package install (+17 more)

### Community 40 - "Context manager: token-denominated budgets, compaction, recovery"
Cohesion: 0.17
Nodes (20): /sessions manager: list/switch/ctrl+d delete; single-writer delete refusal, JSONL sessions: single-writer flock, 0600, compaction entry (summary, firstKeptEntryId, tokensBefore, usage), Compaction loop guard: stop with error if still above trigger, Context manager: token-denominated budgets, compaction, recovery, JSONL append-only session transcripts (0600 file, 0700 dir, id+parentId chain), keepRecentTokens (default 20K kept verbatim), Overflow recovery: one compact-and-retry attempt (plain length stop ≠ overflow) (+12 more)

### Community 41 - "internal/agent — system prompt, events, loop, compaction + resume, cross-provider transform, Start wiring"
Cohesion: 0.14
Nodes (22): cmd/moca entrypoint package, internal/agent — system prompt, events, loop, compaction + resume, cross-provider transform, Start wiring, internal/compact — budgets, cut points, lossy serialization, structured summaries (no provider import), internal/config — paths, JSONC pre-pass, typed config + validation, env indirection, internal/llm — leaf types (imports nothing internal), internal/mcp — lazy MCP proxy: JSON-RPC core, transports, index, manager, proxy tool, importer, internal/permissions — path jail, parsed shell analysis, trust store, yolo checkers, internal/provider — SSE reader, error taxonomy, catalog, 3 codecs, replay, retry, registry (+14 more)

### Community 44 - "anthropic-messages codec (POST /v1/messages, version header, x-api-key)"
Cohesion: 0.12
Nodes (20): anthropic stream flow (message_start/content_block_*/message_delta/stop), anthropic cache breakpoints — system end + last tool + last eligible block, openai-completions thinking — reasoning_content; history thinking never sent, cross-provider transform — thinking replay bound by Model == request model, ErrContextOverflow (prompt too long / context_length_exceeded), subscription wire use — bearer, store:false, no max_output_tokens, tools in one namespace group, overflow recovery — errOverflow (nothing persisted), one compact-and-retry, then surface, anthropic-messages codec (POST /v1/messages, version header, x-api-key) (+12 more)

### Community 47 - "Seven frozen tools for v1 (schema churn = v2 discussion)"
Cohesion: 0.11
Nodes (18): Seven frozen tools; schemas golden in internal/tools/testdata/schemas.golden.json, Inline edit diffs: changed lines (red/green, ≤8) under the item line, edit tool: path + old_string/new_string + replace_all, ls tool: one level, dirs suffixed /, hidden opt-in, mvdan.cc/sh/v3/syntax — the single non-UI third-party dependency, Read-before-write + unchanged-on-disk guard (mtime + size + hash), Output redirection targets must pass the path jail (literal targets only), rm is ask-every-time: never allowlistable, one-key approval in TUI, refused in -p (+10 more)

### Community 48 - "credential store ~/.config/moca/auth.json (0600, atomic writes, cross-process lock)"
Cohesion: 0.12
Nodes (18): credential store ~/.config/moca/auth.json (0600, atomic writes, cross-process lock), Registry.CheckCredential — network-free login check (/model, resume), /model [id] — cross-provider switch (refused while running, forfeits prompt cache), config.SetObjectEntry — insert key: rawJSON (mcp import), config.SetString — replace a string in place, last-duplicate wins, non-string refused, mcp.DiscoverSources — claude/opencode/pi config discovery paths, moca login --api-key (prompt for a key instead of OAuth), moca login --no-browser (headless OAuth: URL + pasted code) (+10 more)

### Community 49 - "DESIGN §12 example config fixture (phase-1 decode gate)"
Cohesion: 0.18
Nodes (18): config.Standardize preserves byte offsets; example.jsonc is byte-exact DESIGN §12 fixture, Secrets: apiKey must be env:VAR; .env and *.key gitignored, never committed, Strict config decode: unknown keys rejected; new key = defaults + validation + SPECS update, Config: ~/.config/moca/config.jsonc, typed struct, fail-fast validation, env:VAR secret indirection (the one indirection syntax everywhere), -p exit codes: 0 completed · 1 runtime · 2 config/usage · 3 maxSteps · 130 interrupted, maxSteps (default 40): max agentic iterations per run, maxSteps wrap-up request with tool_choice: none (never removes tools) (+10 more)

### Community 53 - "v0.1.0-alpha (2026-10-07): first release"
Cohesion: 0.22
Nodes (18): Default HTTP client bounds response-header wait at 120s, opencode-go [1210] server_error flake retried even on HTTP 400, MCP call gating: default-deny via readOnlyHint annotations / per-server approve lists, mcp config block: idleTimeout scalar beside mcp.servers map + approve lists, Persisted MCP discovery index (~/.local/share/moca/mcp-index.json, configHash-invalidated), MCP env hygiene: filtered subprocess env (PATH/HOME/USER/LANG/TERM/TMPDIR/XDG_*), moca mcp import (Claude Code/OpenCode/Pi configs; secrets rewritten to env:) + mcp index, MCP lazy lifecycle: start on first describe/call, stop after mcp.idleTimeout (600s) (+10 more)

### Community 54 - "moca update [--check] (newest published GitHub release, in-place swap)"
Cohesion: 0.14
Nodes (17): cmd/moca/update.go (moca update subcommand), internal/update — GitHub release fetch, semver compare, checksum verify, atomic replace, install.ps1 — Windows installer/updater, install.sh — Linux installer/updater (latest release or --version), checksums.txt sha256 verification (missing warns; mismatch refuses), moca update --check flag (report only, never writes), git describe version stamp (-ldflags -X config.Version), install-script gate — fresh/update/mismatch refusals for install.sh + functional pwsh run (+9 more)

### Community 55 - "MCP gating — readOnlyHint && !destructiveHint || approve list || session allow; no name heuristics"
Cohesion: 0.12
Nodes (17): TUI approval prompt (allow <cmd>? a once / ctrl+a always / d deny; 700 ms typing pause; esc denies), config.AppendString — comment-preserving array append (allow lists), 32-entry shell.allow default list (DESIGN §12), hard-deny class — sudo/doas/dd/shred/chown/mkfs* (+ su), never overridable, config key: mcp.servers.<name>.approve (allow list, "*" trusts the server), MCP gating — readOnlyHint && !destructiveHint || approve list || session allow; no name heuristics, mvdan.cc/sh/v3 (only non-UI dependency, shell parsing), phase-5 live legs — real model drives mcp search/describe/call + gating (+9 more)

### Community 66 - "Architecture: package layout with strict import direction"
Cohesion: 0.21
Nodes (12): Strict import direction + structural interfaces; seven frozen tools with golden schemas, internal/agent: the agent loop (user msg → model → tools → results), internal/compact: token accounting, truncation, summarization, cache markers, internal/config: single JSONC config, typed, validated, internal/llm: leaf Message/ContentBlock/ToolCall/Usage/Request types, internal/mcp: lazy MCP proxy (stdio + streamable HTTP, discovery index, lifecycle), internal/permissions: path jail, shell command analysis + allowlist, project trust, internal/provider: protocol adapters, provider configs, model catalog, auth, retry (+4 more)

### Community 72 - "system prompt — built once per run, stored in the session, never rebuilt"
Cohesion: 0.17
Nodes (13): vendored ecosystem corpus internal/skills/testdata/ecosystem (pi/claude/opencode/graphify skills), builtin skills — embed, ExtractBuiltins into content-hash dirs (currently rtk), ExtractBuiltins(dataDir) — temp dir + rename, race-safe, agent.filterSkills — omit builtins whose CLI is missing from PATH, LoadInstructions — global AGENTS.md, workdir AGENTS.md (trusted) else CLAUDE.md, 32K cap, phase-6 live legs — sharpened rtk description drives rtk test, graphify answer, rtk builtin skill — verified against the real CLI, SKILL.md frontmatter parser — YAML subset (quotes, block scalars, BOM/CRLF) (+5 more)

### Community 74 - "MCP Manager — zero start, lazy ensure, busy-guarded idle stop, replay-once rules"
Cohesion: 0.15
Nodes (12): config key: mcp.servers.<name> (command|url, args, env, headers), errNotSent — request never left: replay-safe, errTransportDead sentinel (errors.Is-visible), MCP streamable HTTP transport — Mcp-Session-Id echo, SSE responses, 15m client, persisted MCP index mcp-index.json (configHash, no schemas, merged saves), JSON-RPC 2.0 core — id routing, ping, paged tools/list, transports interface, MCP Manager — zero start, lazy ensure, busy-guarded idle stop, replay-once rules, MCP stdio transport — line-wise, 4KiB ring, exit drain ≤500ms, dead-sentinel errors (+4 more)

### Community 75 - "moca login <provider> / TUI /login: credential wizard (OAuth or masked API key)"
Cohesion: 0.32
Nodes (12): auth.json moved to config dir; v0.1 store auto-migrated, +10 opencode-go catalog models (deepseek v4.1, kimi-k2.7-code, mimo-v2.6, qwen3.x…), moca login <provider> / TUI /login: credential wizard (OAuth or masked API key), moca logout <provider> / TUI /logout: clear a stored credential, oauth auth mode (moca login; auth.json 0600, auto-refresh), Slash commands from prompt templates (~/.config/moca/prompts/<name>.md → /name), ~/.config/moca/auth.json credential store (0600, keys beside OAuth tokens), /create-command: seeded starter template; agent writes saved slash commands (+4 more)

### Community 89 - "agent events — TextDelta … TurnEnd/SteeringApplied/Compacted/Resumed"
Cohesion: 0.22
Nodes (9): llm.Request, agent events — TextDelta … TurnEnd/SteeringApplied/Compacted/Resumed, agent loop — sequential calls, steering drains, length-turn handling, exit on no-tool answer, eventPipe — FIFO pipe bridging agent events into Bubble Tea, NormalizeToolID — deterministic ids on use and result, phase-3 gate — scripted tmux TUI session (status bar, steering, approvals, chips), resume replay — agent.Resumed banner + last 3 user turns replayed, runDoneMsg — completion rides the event pipe, cannot overtake a delta (+1 more)

## Knowledge Gaps
- **199 isolated node(s):** `request`, `usageError`, `completionsChunk`, `Credential`, `ollamaTags` (+194 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 355 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **42 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `SPECS.md — moca implemented-state normative spec` connect `SPECS.md — moca implemented-state normative spec` to `start.go`, `tools registry — Tool interface, seven frozen tools, sequential execution in emitted order`, `permissions/shell.go`, `system prompt — built once per run, stored in the session, never rebuilt`, `internal/agent — system prompt, events, loop, compaction + resume, cross-provider transform, Start wiring`, `MCP Manager — zero start, lazy ensure, busy-guarded idle stop, replay-once rules`, `anthropic-messages codec (POST /v1/messages, version header, x-api-key)`, `credential store ~/.config/moca/auth.json (0600, atomic writes, cross-process lock)`, `moca -p "<prompt>" — one-shot agent run (stdin -p form)`, `DESIGN §12 example config fixture (phase-1 decode gate)`, `External tools: rtk, graphify, skills ecosystem`, `anthropic provider (api_key only by policy, no OAuth)`, `TUI: Bubble Tea + lipgloss, compact/dense, welcome lines`, `moca update [--check] (newest published GitHub release, in-place swap)`, `MCP gating — readOnlyHint && !destructiveHint || approve list || session allow; no name heuristics`, `/-dropdown autocomplete (built-ins + prompt templates, ≤8 rows, tab/enter/esc)`, `agent events — TextDelta … TurnEnd/SteeringApplied/Compacted/Resumed`, `opencode-go provider (zen gateway)`?**
  _High betweenness centrality (0.319) - this node is a cross-community bridge._
- **What connects `request`, `usageError`, `completionsChunk` to the rest of the system?**
  _199 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `start.go` be split into smaller, more focused modules?**
  _Cohesion score 0.11118099748409058 - nodes in this community are weakly interconnected._
- **Why does `Thinking blocks/signatures model-bound; cross-provider transforms in agent (SPECS §14)` connect `TUI: Bubble Tea + lipgloss, compact/dense, welcome lines` to `SPECS.md — moca implemented-state normative spec`?**
  _High betweenness centrality (0.041) - this node is a cross-community bridge._
- **Should `context.Context` be split into smaller, more focused modules?**
  _Cohesion score 0.08163265306122448 - nodes in this community are weakly interconnected._
- **Why does `Cross-provider history transform (thinking bound to producing model, tool-id normalization)` connect `TUI: Bubble Tea + lipgloss, compact/dense, welcome lines` to `v0.1.0-alpha (2026-10-07): first release`?**
  _High betweenness centrality (0.041) - this node is a cross-community bridge._
- **Should `update/update.go` be split into smaller, more focused modules?**
  _Cohesion score 0.08333333333333333 - nodes in this community are weakly interconnected._