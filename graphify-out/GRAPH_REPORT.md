# Graph Report - moca  (2026-10-09)

## Corpus Check
- 67 files · ~198,967 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 2388 nodes · 9178 edges · 143 communities (102 shown, 41 thin omitted)
- Extraction: 89% EXTRACTED · 11% INFERRED · 0% AMBIGUOUS · INFERRED: 1049 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- start.go
- newTestModel
- testing.T
- oauth_test.go
- registry_test.go
- newScript
- model
- run
- context.Context
- Parse
- NewManager
- internal/provider
- Model
- model
- stdioTransport
- permissions/shell.go
- CLI surface contract (§3)
- Sanitize
- LoadPrompts
- Manager
- Agent
- runLogin
- Input
- Ask-write root (~/.config/moca/prompts)
- NewDefaultStore
- Request
- Run
- Start
- MCP lazy proxy (internal/mcp)
- Item
- newProxy
- Create
- Context manager (token-denominated budgets)
- TUI (Bubble Tea + lipgloss, inline immutable scrollback)
- importer.go
- model
- Interactive Bubble Tea TUI
- ToolSpec
- NewJail
- TUI review & polish plan
- RenderStatus
- cut_test.go
- .Compact
- moca — one Go binary TUI coding agent
- 2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)
- v0.1.0-alpha (2026-10-07)
- renderMarkdown
- Sessions: append-only JSONL transcripts, ids + parentId
- Skills system (Agent Skills standard, per-project/global/built-in)
- initialize
- Entry
- internal/skills
- moca minimal coding agent
- Find
- SOURCES.md
- Claude Code skill-creator skill
- create-command.md — starter prompt template
- Provider layer: protocols × providers × auth
- Config
- post
- retry_test.go
- Registry
- applyEdit
- fixture/main.go
- Message
- The seven frozen tools
- agent.Resume semantics
- internal/agent
- P1 — high value, every session
- .View
- BuildSystemPrompt
- time.Duration
- parseIgnore
- format.go
- input_test.go
- JSONC config (string-literal-aware pre-pass, trailing commas)
- Lazy MCP proxy (§10.5)
- runTUI
- .DiscoverOllama
- estimate.go
- fakeOllama
- quick_validate.py
- NewUnjailed
- AGENTS.md — moca
- Event
- .runResume
- ParseInput
- history.go
- picker.go
- Jail
- Snapshots
- TrustStore
- TestModelPickerRefreshesOllama
- CI Workflow (gofmt · vet · build · test)
- Architecture: package layout & dependency direction
- Serialize
- retryable
- ReadTracker
- 1.2 Inline diff preview for edit/write items
- Claude Code frontend-design skill
- pi mcp-scripting skill
- allowAll
- StartError
- 2.3 Richer activity row
- 1.7 Persistent input history + ctrl+r search
- findTestFile
- AllowAll
- run.sh
- askRootChecker
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
- Seven frozen tools with golden schemas
- EnvError
- Dynamic client registration with issued client id (oaiapp_…)
- Rotating 30-day refresh tokens serialized across processes
- 2.2 One-row item lines
- Arrange-Act-Assert with xUnit/NSubstitute
- fakeSnap
- rootChecker
- AGENTS.md / CLAUDE.md loading (§9.5)
- Sequential execution of parallel tool calls
- Live gates on real OpenCode key
- ls tool
- Parsed shell analysis
- Still-open known items
- unused-a
- unused-b
- Pi — comparison reference
- 2.7 Quieter first run (hints.json marker)
- example.com/totals
- github.com/adeotek/moca

## God Nodes (most connected - your core abstractions)
1. `model` - 73 edges
2. `newTestModel()` - 62 edges
3. `newScript()` - 46 edges
4. `run()` - 45 edges
5. `key()` - 44 edges
6. `sseServer()` - 36 edges
7. `startTestWith()` - 35 edges
8. `Config` - 34 edges
9. `Parse()` - 34 edges
10. `newAgentModel()` - 32 edges

## Surprising Connections (you probably didn't know these)
- `Built-in rtk skill (SKILL.md)` --semantically_similar_to--> `rtk — token-compressed CLI proxy`  [INFERRED] [semantically similar]
  internal/skills/builtin/rtk/SKILL.md → docs/external-tools.md
- `Minimal, token-efficient, provider-agnostic agent pitch` --semantically_similar_to--> `Tight agent loop + disciplined context handling beats features`  [INFERRED] [semantically similar]
  README.md → docs/specs/DESIGN.md
- `Non-goals (v1, probably forever)` --semantically_similar_to--> `Explicit non-goals (§15)`  [INFERRED] [semantically similar]
  README.md → docs/specs/DESIGN.md
- `Release workflow (.github/workflows/release.yml) + moca update (rev 16)` --semantically_similar_to--> `Release workflow shipped (v* tag, checksums.txt)`  [INFERRED] [semantically similar]
  docs/specs/DESIGN.md → CHANGELOG.md
- `moca update — self-update vs GitHub release (rev 16)` --semantically_similar_to--> `moca update — self-update shipped`  [INFERRED] [semantically similar]
  docs/specs/DESIGN.md → CHANGELOG.md

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Cross-tool SKILL.md compatibility ecosystem** — docs_external_tools_skills_ecosystem, internal_skills_builtin_rtk_skill, docs_external_tools_graphify [EXTRACTED 0.95]
- **Ecosystem SKILL.md compatibility corpus (graphify, Claude Code, pi, OpenCode)** — internal_skills_testdata_ecosystem_sources, internal_skills_testdata_ecosystem, internal_skills_testdata_ecosystem_graphify_graphify_vendored, internal_skills_testdata_ecosystem_claude_code_skill_creator_vendored, internal_skills_testdata_ecosystem_claude_code_frontend_design_vendored, internal_skills_testdata_ecosystem_pi_pi_subagents_vendored, internal_skills_testdata_ecosystem_pi_mcp_scripting_vendored, internal_skills_testdata_ecosystem_opencode_dotnet_unit_testing_vendored [EXTRACTED 1.00]
- **Per-provider OAuth policy decisions (phase 7 gate)** — docs_specs_oauth_verification_untolerated_unclear, docs_specs_oauth_verification_anthropic_decision, docs_specs_oauth_verification_openai_decision, docs_specs_oauth_verification_siwc [EXTRACTED 1.00]
- **rtk allowlist laundering prevention flow** — docs_external_tools_rtk_unwrap, docs_external_tools_internal_permissions_rtk_go, internal_skills_builtin_rtk_skill_prefer_compressed [EXTRACTED 1.00]
- **Synthetic frontmatter edge-case fixtures (CRLF, BOM, folding, quoting, rich keys, long description)** — internal_skills_testdata_ecosystem_synthetic_bom_skill, internal_skills_testdata_ecosystem_synthetic_colon_unquoted_skill, internal_skills_testdata_ecosystem_synthetic_crlf_skill, internal_skills_testdata_ecosystem_synthetic_folded_skill, internal_skills_testdata_ecosystem_synthetic_literal_skill, internal_skills_testdata_ecosystem_synthetic_long_description_skill, internal_skills_testdata_ecosystem_synthetic_rich_keys_skill [EXTRACTED 1.00]
- **Human-in-the-loop validation before declaring done** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_claude_code_frontend_design_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_review_validation [INFERRED 0.75]
- **Parallel subagent fanout with parent synthesis** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_graphify_graphify_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_skill [INFERRED 0.85]
- **Skills referencing bundled reference files for progressive disclosure** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_graphify_graphify_skill, internal_skills_testdata_ecosystem_pi_mcp_scripting_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_skill [INFERRED 0.85]
- **Agent turn pipeline: transform, request, tools, events** — docs_specs_specs_turn_contract, docs_specs_specs_cross_provider_transform, docs_specs_specs_tools_registry, docs_specs_specs_event_types, docs_specs_specs_retry_policy [EXTRACTED 0.95]
- **Comment-preserving JSONC edit family** — docs_specs_specs_jsonc_standardize, docs_specs_specs_config_append_string, docs_specs_specs_config_set_object_entry, docs_specs_specs_config_set_string [EXTRACTED 0.95]
- **MCP lazy proxy stack** — docs_specs_specs_mcp_protocol, docs_specs_specs_stdio_transport, docs_specs_specs_streamable_http_transport, docs_specs_specs_discovery_index, docs_specs_specs_mcp_manager, docs_specs_specs_mcp_tool [EXTRACTED 0.95]
- **Day-1 extensibility mechanisms (skills, slash commands, allowlist, lazy MCP)** — docs_specs_design_skills, docs_specs_design_prompt_templates, docs_specs_design_shell_command_analysis, docs_specs_design_mcp_lazy_proxy [EXTRACTED 0.90]
- **Permission/security check family (jail, shell analysis, trust, yolo, MCP gating)** — docs_specs_design_path_jail, docs_specs_design_shell_command_analysis, docs_specs_design_project_trust, docs_specs_design_yolo_mode, docs_specs_design_mcp_call_gating [EXTRACTED 0.90]
- **TUI polish set: planned in tui-plan, shipped in CHANGELOG** — docs_tui_plan_markdown_lite, docs_tui_plan_context_pressure_color, docs_tui_plan_resume_replay, changelog_markdown_lite, changelog_context_pressure_feature, changelog_resume_replay_feature [EXTRACTED 0.90]

## Communities (143 total, 41 thin omitted)

### Community 0 - "start.go"
Cohesion: 0.11
Nodes (19): request, usageError, anthropicEvent, anthropicUsage, completionsChunk, Credential, ollamaTags, responsesEvent (+11 more)

### Community 1 - "newTestModel"
Cohesion: 0.04
Nodes (87): TestActivityLine(), TestCoalescedShrinkRemembersTheLastFlushedRow(), TestInputBoxGrowsAndShrinks(), TestPagerCloseDefersTheRelease(), TestRefuseRunningWhileOutputInFlight(), TestShrinkIsPaddedUntilTheGuardWindowPasses(), TestShrinkPaddingFitsTheScreen(), TestSpinnerTicksOnlyForTheLiveRun() (+79 more)

### Community 2 - "testing.T"
Cohesion: 0.04
Nodes (80): ollamaServer(), ollamaStart(), TestOllamaDeclaredWindowIsQuiet(), TestOllamaReportedWindowIsQuiet(), TestOllamaStartErrors(), TestOllamaTinyWindowWarns(), TestStartWithOllamaModel(), TestResumeNotASession() (+72 more)

### Community 3 - "oauth_test.go"
Cohesion: 0.06
Nodes (48): TestDefaultHTTPClientBoundsHeaderWait(), defaultHTTPClient(), Store, newHostID(), validateAPIKey(), audContains(), b64d(), OAuthConfig (+40 more)

### Community 4 - "registry_test.go"
Cohesion: 0.07
Nodes (66): ModelOverride, discoverLocal(), CostConfig, ProviderConfig, IsLocalProvider(), SplitModel(), TestSplitModel(), Adapter (+58 more)

### Community 5 - "newScript"
Cohesion: 0.09
Nodes (61): scriptServer, newScript(), startTest(), startTestWith(), startWith(), TestAbortWritesSyntheticResults(), TestFailedCallDoesNotCancelRest(), TestLengthStopTruncatedToolCall() (+53 more)

### Community 6 - "model"
Cohesion: 0.11
Nodes (10): ConfigFile(), ShellOutput, model, TestWelcomeLines(), welcomeText(), AbbrevEffort(), hintSeen(), markHint() (+2 more)

### Community 7 - "run"
Cohesion: 0.07
Nodes (52): fakeCompletions(), isolate(), TestExitCodes(), TestHelpListsSurface(), TestInterruptedExit130(), TestLoginLogoutCLI(), TestMCPImportCLI(), TestNoApproveSkipsProjectInstructions() (+44 more)

### Community 8 - "context.Context"
Cohesion: 0.07
Nodes (27): CallResult, lockFile(), lockFile(), approve(), TestInvestigationHint(), TestInvestigationRefusal(), TestTestRunCommand(), TestTestRunFailed() (+19 more)

### Community 9 - "Parse"
Cohesion: 0.07
Nodes (41): scanner, span, Parse(), TestAPIKeyOptional(), TestDefaultsApplied(), TestEnvRefs(), TestOAuthAllowedForOpenAI(), TestOAuthRejectedForNonOAuthProvider() (+33 more)

### Community 10 - "NewManager"
Cohesion: 0.12
Nodes (40): MCPServer, crashingServer(), shServer(), skipWindows(), TestCallTimeoutStopsWedgedServer(), TestHandshakeTimeout(), TestHTTPAbortedStreamReplayRule(), TestIndexSaveMergesConcurrentWriters() (+32 more)

### Community 11 - "internal/provider"
Cohesion: 0.07
Nodes (28): anthropic-messages adapter, Thinking budget (Model.BudgetTokens), Config defaults, Config keys (camelCase), Custom providers (vLLM/LM Studio), Effort clamping (Model.ClampEffort), --model / --effort flags, History replay rules (+20 more)

### Community 12 - "Model"
Cohesion: 0.09
Nodes (8): savedModel, Status, Agent, Effort, ParseEffort(), TestParseEffort(), Model, supportedEfforts()

### Community 13 - "model"
Cohesion: 0.10
Nodes (13): Headless(), OAuthProvider(), TestHeadlessDetectionSSH(), model, providerNames(), providerRow(), TestLoginRowForOllama(), loginChoice (+5 more)

### Community 14 - "stdioTransport"
Cohesion: 0.09
Nodes (14): fakeSum, responseID(), TestResponseID(), readEvents(), callResult, KillProcessGroup(), SetProcessGroup(), KillProcessGroup() (+6 more)

### Community 15 - "permissions/shell.go"
Cohesion: 0.15
Nodes (17): rtkRunTarget(), rtkShellString(), rtkSub(), rtkTarget(), Shell, literalSafe(), numericFlag(), patternSafe() (+9 more)

### Community 16 - "CLI surface contract (§3)"
Cohesion: 0.10
Nodes (16): config.SetString, Credential store (auth.json 0600, flock, rotation), --approve/--no-approve/--yolo flags, JSONC config with strict decode, config.Standardize JSONC pre-pass, moca login command, `/login`/`/logout` TUI wizard, moca logout command (+8 more)

### Community 17 - "Sanitize"
Cohesion: 0.11
Nodes (17): AutoAllow(), Answer, Question, approvalPanel(), approvalPrompt(), TestApprovalPromptSanitized(), diffPreview(), TestDiffPreview() (+9 more)

### Community 18 - "LoadPrompts"
Cohesion: 0.09
Nodes (26): builtinHash(), ExtractBuiltins(), extractBuiltins(), TestEcosystemSkillsLoadUnchanged(), closingQuote(), ParseFrontmatter(), scalar(), TestFrontmatter() (+18 more)

### Community 19 - "Manager"
Cohesion: 0.15
Nodes (8): Rank(), TestRank(), words(), Manager, Hit, Index, IndexEntry, state

### Community 20 - "Agent"
Cohesion: 0.14
Nodes (11): Options, ToolEnd, ToolStart, Agent, Outcome, New(), Response, ToolCall (+3 more)

### Community 21 - "runLogin"
Cohesion: 0.15
Nodes (20): parseArgs(), printUsage(), TestParseArgs(), loginAPIKey(), runLogin(), runLogout(), yesNo(), run() (+12 more)

### Community 22 - "Input"
Cohesion: 0.13
Nodes (4): lineCount(), Input, padGuard, paste

### Community 23 - "Ask-write root (~/.config/moca/prompts)"
Cohesion: 0.11
Nodes (19): tools.ApprovablePaths contract, Approvals, trust and yolo model, CI gate (.github/workflows/ci.yml), `/`-dropdown autocomplete, Inline edit diffs, edit tool, Path jail, Phase-7 ship-gate checker + live legs (+11 more)

### Community 24 - "NewDefaultStore"
Cohesion: 0.18
Nodes (20): NewDefaultStore(), TestThinkingLinePrintsBeforeTheResponse(), drained(), newLoginModel(), TestLoginMarkedBackAndCancel(), TestLoginOAuthDeclinedFlipAndCancel(), TestLoginOAuthFlowAndAuthFlip(), TestLoginRefusedWhileRunning() (+12 more)

### Community 25 - "Request"
Cohesion: 0.14
Nodes (17): Kind, Entry, ContentBlock, Request, ToolResult, anthropicOverflow(), openaiOverflow(), priorReasoningText() (+9 more)

### Community 26 - "Run"
Cohesion: 0.12
Nodes (15): Event, Asker, newModel(), Run(), newAsker(), newEventPipe(), TestEventPipeBuffersUntilStarted(), TestEventPipeNonBlockingAndFIFO() (+7 more)

### Community 27 - "Start"
Cohesion: 0.14
Nodes (21): setup, main(), runMCPIndex(), GitState(), Platform(), build(), StartOptions, prepare() (+13 more)

### Community 28 - "MCP lazy proxy (internal/mcp)"
Cohesion: 0.13
Nodes (20): Interactive approval prompt, config.AppendString (comment-preserving JSONC edit), config.SetObjectEntry, Persisted MCP discovery index (mcp-index.json), Markdown-lite response rendering, MCP gating (read-only / approve list / session allow), mcp import (source discovery, secret rewriting), moca mcp import subcommand (+12 more)

### Community 29 - "Item"
Cohesion: 0.15
Nodes (7): TestMatchRanges(), matchRanges(), newPager(), TestTeaAPI(), Item, Items, pagerModel

### Community 30 - "newProxy"
Cohesion: 0.11
Nodes (19): cutRunes(), errResult(), isTrue(), newProxy(), runP(), TestCutRunesSafe(), TestProxyArgErrors(), TestProxyGating() (+11 more)

### Community 31 - "Create"
Cohesion: 0.15
Nodes (14): lockFile(), lockFile(), Create(), Writer, lastNewline(), newID(), Open(), Slug() (+6 more)

### Community 32 - "Context manager (token-denominated budgets)"
Cohesion: 0.10
Nodes (18): Context-pressure status bar colors shipped, moca update — self-update shipped, Release workflow shipped (v* tag, checksums.txt), CLI surface (§12.5): TUI, -p one-shot, login, mcp, update, Context manager (token-denominated budgets), reserveTokens 16K / keepRecentTokens 20K budgets, Overflow compact-and-retry, -p output contract + exit codes 0/1/2/3/130 (+10 more)

### Community 33 - "TUI (Bubble Tea + lipgloss, inline immutable scrollback)"
Cohesion: 0.10
Nodes (19): esc kills a running !cmd (process group), Unfocused-run notifications shipped (tui.notify), ! / !! shell prefixes (user-initiated, bypass analysis), shift+enter newline via enhanced keyboard reporting, Steering: input during a run lands after tool results, Thinking blocks rendered collapsed (pager-expanded), TUI (Bubble Tea + lipgloss, inline immutable scrollback), 2.1 Turn spacing and hanging indent — **S** (+11 more)

### Community 34 - "importer.go"
Cohesion: 0.15
Nodes (19): TestImportRewritesEmbeddedSecretsInValues(), TestRewriteSecretsValueRuleAlone(), DiscoverSources(), embeddedSecret(), envName(), literalSecretFields(), ParseSource(), Plan() (+11 more)

### Community 35 - "model"
Cohesion: 0.17
Nodes (11): firstLineOf(), clampRunes(), model, matchCommands(), TestClampRunes(), mentionMatches(), rankFile(), TestToolArgSummary() (+3 more)

### Community 36 - "Interactive Bubble Tea TUI"
Cohesion: 0.14
Nodes (19): Activity row (live estimated tokens), Terminal color profile handling, Compaction budgets (NewBudget, FindCut, keepRecent), Usage-anchored context estimate, Context-pressure color in status bar, Agent control surface (SetModel/SetEffort/ToggleHard/Status/Steer), Event pipe (FIFO) bridge/threading, Agent event types (+11 more)

### Community 37 - "ToolSpec"
Cohesion: 0.11
Nodes (11): ToolSpec, TestProxySpecFrozen(), MCPSpec(), echoTool, editTool, lsTool, mcpStub, readTool (+3 more)

### Community 38 - "NewJail"
Cohesion: 0.19
Nodes (17): NewJail(), setup(), TestJail(), TestJailApprovable(), TestJailDanglingSymlinks(), TestJailKeepsNotYetExistingReadOnlyRoot(), TestJailResolvedPathIsCanonical(), TestJailTildeExpands() (+9 more)

### Community 39 - "TUI review & polish plan"
Cohesion: 0.11
Nodes (14): @path file mentions shipped, Ben — owner / steering requests, moca — MO Coding Agent, 0. Fixed in this pass (already on `fix/polish`), 2. Deliberately out of scope, 3. Suggested sequencing (as planned; all but 2.6 and 3.2 landed together on this branch), 1.5 @path file mentions, PR sequencing A–D + resize investigation (+6 more)

### Community 40 - "RenderStatus"
Cohesion: 0.18
Nodes (12): TestStatusWideRunesNeverExceedWidth(), FmtPercent(), renderLine(), RenderStatus(), TestContextPressure(), TestFmtPercent(), TestRenderStatusFull(), TestRenderStatusNarrow() (+4 more)

### Community 41 - "cut_test.go"
Cohesion: 0.33
Nodes (14): FindCut(), a(), call(), res(), TestFindCutKeepsRecentAtValidPoint(), TestFindCutNeverBetweenCallAndResult(), TestFindCutNothingToDo(), TestFindCutSplitTurn() (+6 more)

### Community 42 - ".Compact"
Cohesion: 0.21
Nodes (7): Agent, summarizerUnavailable(), Budget, NewBudget(), TestBudget(), Prev, Summarizer

### Community 43 - "moca — one Go binary TUI coding agent"
Cohesion: 0.13
Nodes (12): CI gate (gofmt + vet + build + race test), docs/specs/DESIGN.md — v1 contract/vision, docs/specs/SPECS.md — living normative spec, graphify query/path/explain workflow rules, moca — one Go binary TUI coding agent, Ship gate test/shipgate/run.sh, External tools: rtk, graphify, skills ecosystem, graphify — codebase knowledge graph integration (+4 more)

### Community 44 - "2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)"
Cohesion: 0.12
Nodes (13): apiKey must be an env:VAR reference, 1.1 Terms, 1.2 Client registration, 1.3 Decision, 1. anthropic — decision: `api_key only` (subscription OAuth not permitted), 2.1 Terms, 2.2 Client registration (per the docs), 2.3 Endpoints (verified live 2026-10-06 against `https://auth.openai.com/.well-known/openid-configuration`) (+5 more)

### Community 45 - "v0.1.0-alpha (2026-10-07)"
Cohesion: 0.12
Nodes (15): CHANGELOG, CLI, Context & sessions, External tools & the skills ecosystem, Markdown-lite responses shipped, MCP, OAuth (subscription login), Providers & protocols (+7 more)

### Community 46 - "renderMarkdown"
Cohesion: 0.20
Nodes (12): codeBandStyle(), fenceLang(), renderInline(), renderMarkdown(), plainMD(), TestMarkdownBlocks(), TestMarkdownFence(), TestMarkdownSanitizes() (+4 more)

### Community 47 - "Sessions: append-only JSONL transcripts, ids + parentId"
Cohesion: 0.12
Nodes (14): Exit keeps the final status bar + resume hint, /model picker + argument completion shipped, Resume replay + /resume picker shipped, Cache breakpoints (≤4, rolled forward), Prompt-cache discipline, Resume: --resume / --continue, Sessions: append-only JSONL transcripts, ids + parentId, Transcript repair (synthetic tool_result on abort/crash) (+6 more)

### Community 48 - "Skills system (Agent Skills standard, per-project/global/built-in)"
Cohesion: 0.14
Nodes (10): Explicit non-goals (§15), graphify — codebase-graph CLI + drop-in skill, maxSteps runaway guard (default 40, tool_choice:none wrap-up), rtk — token-compressed CLI proxy + built-in skill, shell command analysis via mvdan.cc/sh, Hard-deny + ask-every-time sets (rm ask-every-time), Ship gate (§14): one unattended session on opencode-go, Skills system (Agent Skills standard, per-project/global/built-in) (+2 more)

### Community 49 - "initialize"
Cohesion: 0.15
Nodes (14): TestResolveEnv(), ResolveEnv(), initialize(), TestHTTPCloseBounded(), TestKeepNameWindows(), startHTTP(), TestHTTPNotifyStatusChecked(), FilterEnv() (+6 more)

### Community 50 - "Entry"
Cohesion: 0.14
Nodes (11): TurnEnd, Result, StopReason, Usage, anthropicStop(), Entry, ModelChange, Compaction (+3 more)

### Community 51 - "internal/skills"
Cohesion: 0.20
Nodes (13): Built-in rtk skill + PATH filtering, Builtin skills embedding and extraction, Ecosystem compatibility corpus, graphify query/explain CLI integration, LoadInstructions (AGENTS.md / CLAUDE.md), Phase-6 rtk/graphify gate, internal/skills, rtk/graphify/skills ecosystem compatibility (+5 more)

### Community 52 - "moca minimal coding agent"
Cohesion: 0.14
Nodes (10): Compaction summary request (cheap model, lossy serialization), Context manager (token budgets, compaction, resume), Go 1.27.1 pinned in go.mod toolchain directive, make release — cross-compiled binaries, maxSteps wrap-up request with tool_choice:none, make targets mirrored as mise tasks, moca minimal coding agent, Phase-4 live compaction/resume gates (+2 more)

### Community 53 - "Find"
Cohesion: 0.17
Nodes (15): canonical(), Find(), FindForWorkdir(), header(), headerAndPreview(), idOf(), List(), listNewestFirst() (+7 more)

### Community 54 - "SOURCES.md"
Cohesion: 0.19
Nodes (7): internal/skills/testdata/ecosystem corpus, anthropics/skills frontend-design, anthropics/skills skill-creator, graphify skill (graphify install, claude platform; byte-identical to pi variant), adeotek/dotfiles opencode dotnet-unit-testing skill, npm pi-mcp-adapter mcp-scripting skill, npm pi-subagents skill

### Community 55 - "Claude Code skill-creator skill"
Cohesion: 0.16
Nodes (13): Skill description (trigger) optimization, Skill eval/iterate loop (with-skill vs baseline), skill-creator Apache 2.0 License (Anthropic), Claude Code skill-creator skill, SKILL.md anatomy + bundled resources layout, Constrained query expansion against graph vocab, Progressive Disclosure (three-level skill loading), graphify query/path/explain reference (+5 more)

### Community 56 - "create-command.md — starter prompt template"
Cohesion: 0.22
Nodes (10): /create-command starter template (never overwritten), Ask-write root for ~/.config/moca/prompts/ (rev 15), create-command starter prompt seeded on first TUI run (rev 15), Prompt templates / slash commands (§10 mechanism 2), / dropdown autocomplete (rev 14), TUI re-reads prompt dirs when a run finishes (rev 15), create-command.md — starter prompt template, Personal commands dir {{prompts_dir}} (~/.config/moca/prompts/) (+2 more)

### Community 57 - "Provider layer: protocols × providers × auth"
Cohesion: 0.21
Nodes (12): New provider ollama (local/LAN server, model discovery), Cross-provider history transform (thinking/signatures/tool ids), Effort as a first-class request parameter, Built-in model catalog (thinkingMode, thinkingLevelMap, cost), OAuth verification (docs/specs/oauth-verification.md, verified 2026-10-06), anthropic provider, Provider layer: protocols × providers × auth, openai provider (+4 more)

### Community 58 - "Config"
Cohesion: 0.18
Nodes (10): ContextConfig, MCPConfig, ShellConfig, SnapshotConfig, TUIConfig, Default(), Config, Load() (+2 more)

### Community 59 - "post"
Cohesion: 0.21
Nodes (9): fakeHTTPServer(), TestHTTPTransport(), post(), newHTTPError(), retryAfter(), TestRetryAfterFormats(), TestRetryAfterParsed(), headerLog (+1 more)

### Community 60 - "retry_test.go"
Cohesion: 0.44
Nodes (12): fail(), TestMidStreamFailureResetsAndRetriesOnce(), TestMidStreamRetryBacksOffAndNotifies(), TestNoRetryOn400(), testPolicy(), TestRetryAfterCapped(), TestRetryBackoffThenSuccess(), TestRetryGivesUpAfterFive() (+4 more)

### Community 61 - "Registry"
Cohesion: 0.19
Nodes (8): Builtins(), runReg(), TestRegistryInvestigationBanner(), Registry, NewRegistry(), TestRegistryDispatch(), TestSchemasFrozen(), Tool

### Community 62 - "applyEdit"
Cohesion: 0.23
Nodes (14): applyEdit(), formatDiff(), indentOf(), joinInts(), lineOf(), matchLines(), replaceExact(), replaceFuzzy() (+6 more)

### Community 63 - "fixture/main.go"
Cohesion: 0.21
Nodes (7): Sum(), TestTotals(), ParseList(), Average(), Max(), TestMaxAndAverage(), main()

### Community 64 - "Message"
Cohesion: 0.27
Nodes (10): Message, Role, TextOf(), appendUser(), LatestCompaction(), Messages(), TestMessagesFromCompaction(), TestMessagesRebuild() (+2 more)

### Community 65 - "The seven frozen tools"
Cohesion: 0.19
Nodes (12): Failing-test investigation protocol enforced by the edit tool, Lazy MCP proxy shipped, rtk/graphify allowlist + skills ecosystem compatibility, v0.1.0-alpha release (2026-10-07), Edit-ladder tests (non-negotiable, phase 2), Pre-edit snapshots + /undo, The seven frozen tools, edit tool (exact-match first, whitespace fallback, re-indent) (+4 more)

### Community 66 - "agent.Resume semantics"
Cohesion: 0.21
Nodes (12): Generalized dropdown (@path, ctrl+r history), --resume/--continue flags, Append-only JSONL sessions with pre-edit snapshots and undo, Persistent prompt history (history.jsonl), Resume replay of last turns, session.Find / FindForWorkdir lookup, session.Messages rebuild (compaction-aware), session.Repair (synthetic interrupted results) (+4 more)

### Community 67 - "internal/agent"
Cohesion: 0.42
Nodes (13): internal/agent, cmd/moca package, internal/compact, internal/config, internal/llm, internal/mcp, internal/permissions, internal/session (+5 more)

### Community 68 - "P1 — high value, every session"
Cohesion: 0.15
Nodes (13): 1.1 Markdown-lite rendering of responses — **M**, 1.2 Inline diff preview for `edit`/`write` items — **S/M**, 1.3 Context-pressure color in the status bar — **S**, 1.4 Argument completion in the `/` dropdown + interactive `/model` — **M**, 1.5 `@path` file mentions — **M**, 1.6 Resume shows where you left off; `/resume` picker — **M**, 1.7 Persistent input history + `ctrl+r` search — **S/M**, 1. Proposed improvements (priority order) (+5 more)

### Community 70 - "BuildSystemPrompt"
Cohesion: 0.17
Nodes (12): PromptInput, ServerLine, BuildSystemPrompt(), filterSkills(), oneLine(), TestBuildSystemPrompt(), TestFilterSkillsDropsMissingBuiltins(), TestPromptRTKIsConditional() (+4 more)

### Community 71 - "time.Duration"
Cohesion: 0.24
Nodes (8): Retry, DefaultRetryPolicy(), RetryNotice, TestFmtElapsed(), fmtElapsed(), fmtThinkDuration(), retrying, RetryPolicy

### Community 72 - "parseIgnore"
Cohesion: 0.20
Nodes (10): globToRegexp(), ignored(), parseIgnore(), TestIgnore(), ListFiles(), TestListFiles(), model, mentionQuery() (+2 more)

### Community 73 - "format.go"
Cohesion: 0.26
Nodes (10): TestGitStatusUsesNoOptionalLocks(), AbbrevHome(), FmtTokens(), FmtWindow(), GitBranch(), gitStatusArgs(), TestFormatters(), TestGitBranch() (+2 more)

### Community 74 - "input_test.go"
Cohesion: 0.30
Nodes (11): NewInput(), TestCollapseUsesExactOffset(), TestCtrlC(), TestEditedMarkerSentLiterally(), TestHistory(), TestHistoryRecallChipsUnsafeContent(), TestMarkerNeverCollidesWithTypedText(), TestPasteChipKeepsContent() (+3 more)

### Community 75 - "JSONC config (string-literal-aware pre-pass, trailing commas)"
Cohesion: 0.22
Nodes (9): apiKey optional; stored key wins over env: reference, / autocomplete dropdown + command echo, v0.1.1 /login wizard + credential store in auth.json, v0.1.1 release (2026-10-08), api_key auth mode with env:VAR indirection, Credential store in ~/.config/moca/auth.json (rev 14), JSONC config (string-literal-aware pre-pass, trailing commas), /login credential wizard + /logout (rev 14) (+1 more)

### Community 76 - "Lazy MCP proxy (§10.5)"
Cohesion: 0.18
Nodes (10): MCP call gating: annotation-based, default-deny, Lazy MCP proxy (§10.5), mcp import literal-secret rewriting to env:VAR, Phase plan (7 phases, one PR each), Pi — architectural reference (compaction, MCP adapter), Hand-rolled protocol adapters (anthropic-messages, openai-completions, openai-responses), Structured summary format (Pi's proven shape), mcp lazy proxy tool (~200-token schema) (+2 more)

### Community 77 - "runTUI"
Cohesion: 0.27
Nodes (8): TestExitForOAuthSentinels(), exitFor(), firstLine(), resolveResume(), runOneShot(), decideTrust(), hasProjectResources(), runTUI()

### Community 78 - ".DiscoverOllama"
Cohesion: 0.24
Nodes (7): Registry, normalizeOllamaURL(), ollamaBaseURL(), ollamaRoot(), TestNormalizeOllamaURL(), TestOllamaBaseURLPrecedence(), Discovery

### Community 79 - "estimate.go"
Cohesion: 0.42
Nodes (6): EntryTokens(), MessageChars(), RequestChars(), TestEstimate(), Tokens(), UsageTokens()

### Community 80 - "fakeOllama"
Cohesion: 0.22
Nodes (4): modelFromShow(), countingTransport, fakeOllama, ollamaShow

### Community 82 - "NewUnjailed"
Cohesion: 0.29
Nodes (4): Agent, NewUnjailed(), TestUnjailedAndAllowAll(), Unjailed

### Community 83 - "AGENTS.md — moca"
Cohesion: 0.25
Nodes (7): AGENTS.md — moca, Commands, Conventions & gotchas, Docs, graphify, Ground truth (read before changing behavior), Layout & dependency rules

### Community 84 - "Event"
Cohesion: 0.25
Nodes (6): Event, isDialFailure(), ollamaUnreachable(), ok(), EventType, ollamaGuard

### Community 85 - ".runResume"
Cohesion: 0.39
Nodes (5): Info, fmtAge(), model, sessionHint(), sessionsDir()

### Community 86 - "ParseInput"
Cohesion: 0.32
Nodes (8): Prompt, HelpText(), ParseInput(), TestParseInput(), TestPromptDirsReloadAtRunEnd(), TestLoginCommandParsingAndHelp(), InputKind, Parsed

### Community 87 - "history.go"
Cohesion: 0.36
Nodes (6): appendHistory(), loadHistory(), rewriteHistory(), TestHistoryPersistence(), hasControl(), historyRec

### Community 88 - "picker.go"
Cohesion: 0.29
Nodes (5): TestWindowRange(), model, windowRange(), pickItem, pickState

### Community 89 - "Jail"
Cohesion: 0.43
Nodes (3): Jail, resolveDeep(), within()

### Community 90 - "Snapshots"
Cohesion: 0.29
Nodes (4): gitClean(), Snapshots, NewSnapshots(), SnapshotRec

### Community 91 - "TrustStore"
Cohesion: 0.40
Nodes (4): canonical(), LoadTrust(), TestTrustStore(), TrustStore

### Community 92 - "TestModelPickerRefreshesOllama"
Cohesion: 0.33
Nodes (6): contains(), count(), dropNames(), TestDropdownListsCommandsAndPrompts(), TestModelPickerRefreshesOllama(), TestShowCompletionNewestFirst()

### Community 93 - "CI Workflow (gofmt · vet · build · test)"
Cohesion: 0.40
Nodes (4): CI Workflow (gofmt · vet · build · test), CI gate set (gofmt, go vet, go build, go test -race), go.mod (Go version source for setup-go), SPECS.md §2 — CI gate definition

### Community 94 - "Architecture: package layout & dependency direction"
Cohesion: 0.40
Nodes (5): agent package — the loop, Architecture: package layout & dependency direction, llm leaf package (imports nothing internal), Path jail (session-start anchored, symlink-resolved), Read-only roots for global/built-in skill dirs

### Community 95 - "Serialize"
Cohesion: 0.50
Nodes (5): callLine(), safeTail(), Serialize(), TestSerialize(), TestSerializeRuneSafeAndEmptyText()

### Community 96 - "retryable"
Cohesion: 0.40
Nodes (5): http2Transient(), retryable(), TestRetryableServerError400(), TestRetryable(), TestRetryableTransportErrors()

### Community 97 - "ReadTracker"
Cohesion: 0.60
Nodes (3): stampOf(), ReadTracker, stamp

### Community 98 - "1.2 Inline diff preview for edit/write items"
Cohesion: 0.50
Nodes (4): Inline edit diffs shipped, Claude Code — comparison reference, diffPreview(detail, width, maxLines), 1.2 Inline diff preview for edit/write items

### Community 99 - "Claude Code frontend-design skill"
Cohesion: 0.50
Nodes (3): AI-generated design default tells, frontend-design Apache 2.0 License, Claude Code frontend-design skill

### Community 100 - "pi mcp-scripting skill"
Cohesion: 0.67
Nodes (4): jev.evaluate reference, jev.evaluate semantic scripting gate, mcpScript tools API, pi mcp-scripting skill

### Community 103 - "2.3 Richer activity row"
Cohesion: 0.67
Nodes (3): Richer activity row + pager search shipped, 2.3 Richer activity row, agent.PendingSteering() count

### Community 104 - "1.7 Persistent input history + ctrl+r search"
Cohesion: 0.67
Nodes (3): Persistent prompt history + ctrl+r shipped, history.go / history.jsonl store, 1.7 Persistent input history + ctrl+r search

### Community 105 - "findTestFile"
Cohesion: 0.67
Nodes (3): findTestFile(), isPathByte(), TestFindTestFile()

## Knowledge Gaps
- **190 isolated node(s):** `0. Fixed in this pass (already on `fix/polish`)`, `1.1 Markdown-lite rendering of responses — **M**`, `1.2 Inline diff preview for `edit`/`write` items — **S/M**`, `1.3 Context-pressure color in the status bar — **S**`, `1.4 Argument completion in the `/` dropdown + interactive `/model` — **M**` (+185 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 354 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **41 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `model` connect `model` to `start.go`, `model`, `.View`, `RenderStatus`, `model`, `renderMarkdown`, `Sanitize`, `Agent`, `Input`, `picker.go`, `Run`, `Start`, `Item`?**
  _High betweenness centrality (0.027) - this node is a cross-community bridge._
- **What connects `0. Fixed in this pass (already on `fix/polish`)`, `1.1 Markdown-lite rendering of responses — **M**`, `1.2 Inline diff preview for `edit`/`write` items — **S/M**` to the rest of the system?**
  _190 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `start.go` be split into smaller, more focused modules?**
  _Cohesion score 0.10934706429235501 - nodes in this community are weakly interconnected._
- **Why does `Agent` connect `Agent` to `start.go`, `registry_test.go`, `model`, `context.Context`, `Model`, `Entry`, `Manager`, `Run`, `Create`?**
  _High betweenness centrality (0.015) - this node is a cross-community bridge._
- **Should `newTestModel` be split into smaller, more focused modules?**
  _Cohesion score 0.042776998597475455 - nodes in this community are weakly interconnected._
- **Why does `fakeAS` connect `oauth_test.go` to `testing.T`, `Manager`?**
  _High betweenness centrality (0.011) - this node is a cross-community bridge._
- **Should `testing.T` be split into smaller, more focused modules?**
  _Cohesion score 0.04063084736701417 - nodes in this community are weakly interconnected._