# Graph Report - moca  (2026-10-09)

## Corpus Check
- 253 files · ~210,179 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 10 file(s) not represented in the graph (top: (none) 5, .jsonc 3, .v2 1)

## Summary
- 2484 nodes · 9595 edges · 145 communities (102 shown, 43 thin omitted)
- Extraction: 89% EXTRACTED · 11% INFERRED · 0% AMBIGUOUS · INFERRED: 1100 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `9151bcb1`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- start.go
- newTestModel
- auth_test.go
- oauth_test.go
- registry_test.go
- newScript
- model
- testing.T
- context.Context
- Parse
- NewManager
- internal/provider
- Model
- NewDefaultStore
- sync.Mutex
- permissions/shell.go
- CLI surface contract (§3)
- Sanitize
- LoadPrompts
- Manager
- Agent
- runLogin
- Input
- Ask-write root (~/.config/moca/prompts)
- styleBlock
- Request
- Run
- printed
- MCP lazy proxy (internal/mcp)
- Item
- newAgentModel
- Create
- Context manager (token-denominated budgets)
- TUI (Bubble Tea + lipgloss, inline immutable scrollback)
- importer.go
- model
- Interactive Bubble Tea TUI
- .openSessions
- update/update.go
- TUI review & polish plan
- RenderStatus
- cut_test.go
- Message
- External tools: rtk, graphify, skills ecosystem
- 2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)
- v0.1.0-alpha (2026-10-07)
- renderMarkdown
- Architecture: package layout & dependency direction
- Skills system (Agent Skills standard, per-project/global/built-in)
- sessions_test.go
- drained
- internal/skills
- moca minimal coding agent
- Find
- SOURCES.md
- Claude Code skill-creator skill
- create-command.md — starter prompt template
- Provider layer: protocols × providers × auth
- Config
- prepare
- retry_test.go
- build
- .Run
- fixture/main.go
- estimate.go
- The seven frozen tools
- agent.Resume semantics
- internal/agent
- P1 — high value, every session
- pager.go
- Start
- history.go
- parseIgnore
- format.go
- input_test.go
- JSONC config (string-literal-aware pre-pass, trailing commas)
- Lazy MCP proxy (§10.5)
- initialize
- .DiscoverOllama
- pasteReader
- time.Duration
- quick_validate.py
- NewUnjailed
- AGENTS.md — moca
- picker.go
- TestModelPickerRefreshesOllama
- Index
- TurnEnd
- runTUI
- retryable
- Snapshots
- tui/trust.go
- ollamaStart
- CI Workflow (gofmt · vet · build · test)
- README — user-facing overview
- parseArgs
- parseSlash
- time.Time
- 1.2 Inline diff preview for edit/write items
- Claude Code frontend-design skill
- pi mcp-scripting skill
- allowAll
- StartError
- 2.3 Richer activity row
- 1.7 Persistent input history + ctrl+r search
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
- newProxy
- install.sh script

## God Nodes (most connected - your core abstractions)
1. `model` - 73 edges
2. `newTestModel()` - 62 edges
3. `run()` - 52 edges
4. `newScript()` - 46 edges
5. `key()` - 44 edges
6. `newAgentModel()` - 39 edges
7. `sseServer()` - 36 edges
8. `startTestWith()` - 35 edges
9. `Config` - 34 edges
10. `Parse()` - 34 edges

## Surprising Connections (you probably didn't know these)
- `Lens: ~10% surface area, cheap per task, readable in an hour` --semantically_similar_to--> `Minimal, token-efficient, provider-agnostic agent pitch`  [INFERRED] [semantically similar]
  docs/tui-plan.md → README.md
- `Built-in rtk skill (SKILL.md)` --semantically_similar_to--> `rtk — token-compressed CLI proxy`  [INFERRED] [semantically similar]
  internal/skills/builtin/rtk/SKILL.md → docs/external-tools.md
- `Non-goals (v1, probably forever)` --semantically_similar_to--> `Explicit non-goals (§15)`  [INFERRED] [semantically similar]
  README.md → docs/specs/DESIGN.md
- `/create-command starter template (never overwritten)` --semantically_similar_to--> `create-command starter prompt seeded on first TUI run (rev 15)`  [INFERRED] [semantically similar]
  CHANGELOG.md → docs/specs/DESIGN.md
- `TUI re-reads prompt dirs when a run finishes (rev 15)` --semantically_similar_to--> `/create-command starter template (never overwritten)`  [INFERRED] [semantically similar]
  docs/specs/DESIGN.md → CHANGELOG.md

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Day-1 extensibility mechanisms (skills, slash commands, allowlist, lazy MCP)** — docs_specs_design_skills, docs_specs_design_prompt_templates, docs_specs_design_shell_command_analysis, docs_specs_design_mcp_lazy_proxy [EXTRACTED 0.90]
- **Permission/security check family (jail, shell analysis, trust, yolo, MCP gating)** — docs_specs_design_path_jail, docs_specs_design_shell_command_analysis, docs_specs_design_project_trust, docs_specs_design_yolo_mode, docs_specs_design_mcp_call_gating [EXTRACTED 0.90]
- **TUI polish set: planned in tui-plan, shipped in CHANGELOG** — docs_tui_plan_markdown_lite, docs_tui_plan_context_pressure_color, docs_tui_plan_resume_replay, changelog_markdown_lite, changelog_context_pressure_feature, changelog_resume_replay_feature [EXTRACTED 0.90]
- **Agent turn pipeline: transform, request, tools, events** — docs_specs_specs_turn_contract, docs_specs_specs_cross_provider_transform, docs_specs_specs_tools_registry, docs_specs_specs_event_types, docs_specs_specs_retry_policy [EXTRACTED 0.95]
- **Comment-preserving JSONC edit family** — docs_specs_specs_jsonc_standardize, docs_specs_specs_config_append_string, docs_specs_specs_config_set_object_entry, docs_specs_specs_config_set_string [EXTRACTED 0.95]
- **Cross-tool SKILL.md compatibility ecosystem** — docs_external_tools_skills_ecosystem, internal_skills_builtin_rtk_skill, docs_external_tools_graphify [EXTRACTED 0.95]
- **MCP lazy proxy stack** — docs_specs_specs_mcp_protocol, docs_specs_specs_stdio_transport, docs_specs_specs_streamable_http_transport, docs_specs_specs_discovery_index, docs_specs_specs_mcp_manager, docs_specs_specs_mcp_tool [EXTRACTED 0.95]
- **Ecosystem SKILL.md compatibility corpus (graphify, Claude Code, pi, OpenCode)** — internal_skills_testdata_ecosystem_sources, internal_skills_testdata_ecosystem, internal_skills_testdata_ecosystem_graphify_graphify_vendored, internal_skills_testdata_ecosystem_claude_code_skill_creator_vendored, internal_skills_testdata_ecosystem_claude_code_frontend_design_vendored, internal_skills_testdata_ecosystem_pi_pi_subagents_vendored, internal_skills_testdata_ecosystem_pi_mcp_scripting_vendored, internal_skills_testdata_ecosystem_opencode_dotnet_unit_testing_vendored [EXTRACTED 1.00]
- **Per-provider OAuth policy decisions (phase 7 gate)** — docs_specs_oauth_verification_untolerated_unclear, docs_specs_oauth_verification_anthropic_decision, docs_specs_oauth_verification_openai_decision, docs_specs_oauth_verification_siwc [EXTRACTED 1.00]
- **rtk allowlist laundering prevention flow** — docs_external_tools_rtk_unwrap, docs_external_tools_internal_permissions_rtk_go, internal_skills_builtin_rtk_skill_prefer_compressed [EXTRACTED 1.00]
- **Synthetic frontmatter edge-case fixtures (CRLF, BOM, folding, quoting, rich keys, long description)** — internal_skills_testdata_ecosystem_synthetic_bom_skill, internal_skills_testdata_ecosystem_synthetic_colon_unquoted_skill, internal_skills_testdata_ecosystem_synthetic_crlf_skill, internal_skills_testdata_ecosystem_synthetic_folded_skill, internal_skills_testdata_ecosystem_synthetic_literal_skill, internal_skills_testdata_ecosystem_synthetic_long_description_skill, internal_skills_testdata_ecosystem_synthetic_rich_keys_skill [EXTRACTED 1.00]
- **Human-in-the-loop validation before declaring done** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_claude_code_frontend_design_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_review_validation [INFERRED 0.75]
- **Parallel subagent fanout with parent synthesis** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_graphify_graphify_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_skill [INFERRED 0.85]
- **Skills referencing bundled reference files for progressive disclosure** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_graphify_graphify_skill, internal_skills_testdata_ecosystem_pi_mcp_scripting_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_skill [INFERRED 0.85]

## Communities (145 total, 43 thin omitted)

### Community 0 - "start.go"
Cohesion: 0.11
Nodes (19): request, usageError, anthropicEvent, anthropicUsage, completionsChunk, Credential, ollamaTags, responsesEvent (+11 more)

### Community 1 - "newTestModel"
Cohesion: 0.08
Nodes (37): TestActivityLine(), TestCoalescedShrinkRemembersTheLastFlushedRow(), TestInputBoxGrowsAndShrinks(), TestShrinkIsPaddedUntilTheGuardWindowPasses(), TestShrinkPaddingFitsTheScreen(), TestSpinnerTicksOnlyForTheLiveRun(), TestViewCursorInsideInputBox(), key() (+29 more)

### Community 2 - "auth_test.go"
Cohesion: 0.21
Nodes (16): NewStore(), TestAPIKeyStore(), TestConcurrentRefreshOnce(), TestCredentialForLockWaitHonoursContext(), TestDeleteClearsCorruptStore(), TestHostIDStable(), TestInvalidClient(), TestLockFileWaitsThenAcquires() (+8 more)

### Community 3 - "oauth_test.go"
Cohesion: 0.05
Nodes (51): TestDefaultHTTPClientBoundsHeaderWait(), defaultHTTPClient(), Store, newHostID(), validateAPIKey(), NewLineReader(), audContains(), b64d() (+43 more)

### Community 4 - "registry_test.go"
Cohesion: 0.07
Nodes (67): SplitModel(), TestResolveEnv(), TestSplitModel(), ResolveEnv(), TestKeepNameWindows(), FilterEnv(), keepName(), serverEnv() (+59 more)

### Community 5 - "newScript"
Cohesion: 0.09
Nodes (61): scriptServer, newScript(), startTest(), startTestWith(), startWith(), TestAbortWritesSyntheticResults(), TestFailedCallDoesNotCancelRest(), TestLengthStopTruncatedToolCall() (+53 more)

### Community 6 - "model"
Cohesion: 0.11
Nodes (11): ShellOutput, model, TestWelcomeLines(), welcomeText(), AbbrevEffort(), hintSeen(), markHint(), compactDoneMsg (+3 more)

### Community 7 - "testing.T"
Cohesion: 0.05
Nodes (92): fakeCompletions(), isolate(), TestExitCodes(), TestHelpListsSurface(), TestInterruptedExit130(), TestLoginLogoutCLI(), TestMCPImportCLI(), TestNoApproveSkipsProjectInstructions() (+84 more)

### Community 8 - "context.Context"
Cohesion: 0.07
Nodes (30): CallResult, errResult(), lockFile(), lockFile(), approve(), findTestFile(), isPathByte(), TestFindTestFile() (+22 more)

### Community 9 - "Parse"
Cohesion: 0.06
Nodes (42): scanner, span, Parse(), TestAPIKeyOptional(), TestDefaultsApplied(), TestEnvRefs(), TestOAuthAllowedForOpenAI(), TestOAuthRejectedForNonOAuthProvider() (+34 more)

### Community 10 - "NewManager"
Cohesion: 0.12
Nodes (34): runMCPIndex(), crashingServer(), shServer(), skipWindows(), TestCallTimeoutStopsWedgedServer(), TestHandshakeTimeout(), TestHTTPAbortedStreamReplayRule(), TestIndexSaveMergesConcurrentWriters() (+26 more)

### Community 11 - "internal/provider"
Cohesion: 0.07
Nodes (28): anthropic-messages adapter, Thinking budget (Model.BudgetTokens), Config defaults, Config keys (camelCase), Custom providers (vLLM/LM Studio), Effort clamping (Model.ClampEffort), --model / --effort flags, History replay rules (+20 more)

### Community 12 - "Model"
Cohesion: 0.09
Nodes (8): savedModel, Status, Agent, Effort, ParseEffort(), TestParseEffort(), Model, supportedEfforts()

### Community 13 - "NewDefaultStore"
Cohesion: 0.11
Nodes (12): NewDefaultStore(), OAuthProvider(), model, providerNames(), providerRow(), TestLoginRowForOllama(), loginChoice, loginDoneMsg (+4 more)

### Community 14 - "sync.Mutex"
Cohesion: 0.09
Nodes (16): fakeSum, responseID(), TestResponseID(), readEvents(), callResult, startStdio(), TestStdioWriteEPIPE(), KillProcessGroup() (+8 more)

### Community 15 - "permissions/shell.go"
Cohesion: 0.06
Nodes (42): setup, canonical(), Jail, NewJail(), resolveDeep(), setup(), TestJail(), TestJailApprovable() (+34 more)

### Community 16 - "CLI surface contract (§3)"
Cohesion: 0.10
Nodes (16): config.SetString, Credential store (auth.json 0600, flock, rotation), --approve/--no-approve/--yolo flags, JSONC config with strict decode, config.Standardize JSONC pre-pass, moca login command, `/login`/`/logout` TUI wizard, moca logout command (+8 more)

### Community 17 - "Sanitize"
Cohesion: 0.10
Nodes (11): TextOf(), firstLineOf(), clampRunes(), TestClampRunes(), model, TestToolArgSummary(), isUserTurn(), Sanitize() (+3 more)

### Community 18 - "LoadPrompts"
Cohesion: 0.08
Nodes (32): builtinHash(), ExtractBuiltins(), extractBuiltins(), TestEcosystemSkillsLoadUnchanged(), closingQuote(), ParseFrontmatter(), scalar(), TestFrontmatter() (+24 more)

### Community 19 - "Manager"
Cohesion: 0.29
Nodes (3): ConfigHash(), Manager, state

### Community 20 - "Agent"
Cohesion: 0.09
Nodes (19): Options, ToolEnd, ToolStart, Result, Agent, Outcome, New(), Response (+11 more)

### Community 21 - "runLogin"
Cohesion: 0.17
Nodes (16): printUsage(), loginAPIKey(), runLogin(), runLogout(), yesNo(), main(), run(), confirm() (+8 more)

### Community 22 - "Input"
Cohesion: 0.15
Nodes (3): lineCount(), Input, paste

### Community 23 - "Ask-write root (~/.config/moca/prompts)"
Cohesion: 0.11
Nodes (19): tools.ApprovablePaths contract, Approvals, trust and yolo model, CI gate (.github/workflows/ci.yml), `/`-dropdown autocomplete, Inline edit diffs, edit tool, Path jail, Phase-7 ship-gate checker + live legs (+11 more)

### Community 24 - "styleBlock"
Cohesion: 0.31
Nodes (10): respMsgStyle(), styleBlock(), styleBlockLead(), TestCommandEchoesAsUserMessage(), TestMessageBackgroundsCoverTheFullRow(), TestResponseCommitUsesBackground(), TestScrollbackMessageBackgrounds(), TestStyleBlockExpandsTabs() (+2 more)

### Community 25 - "Request"
Cohesion: 0.14
Nodes (18): ContentBlock, Event, Request, ToolResult, anthropicOverflow(), openaiOverflow(), priorReasoningText(), replaysVerbatim() (+10 more)

### Community 26 - "Run"
Cohesion: 0.14
Nodes (12): Event, newModel(), Run(), newAsker(), newEventPipe(), TestEventPipeBuffersUntilStarted(), TestEventPipeNonBlockingAndFIFO(), TestPipeSendKeepsOrderWithEvents() (+4 more)

### Community 27 - "printed"
Cohesion: 0.15
Nodes (19): TestPagerCloseDefersTheRelease(), TestRefuseRunningWhileOutputInFlight(), TestThinkingReadableInPager(), TestCompactedAndResumedLines(), TestAllowAlwaysPersists(), ack(), printed(), settle() (+11 more)

### Community 28 - "MCP lazy proxy (internal/mcp)"
Cohesion: 0.13
Nodes (20): Interactive approval prompt, config.AppendString (comment-preserving JSONC edit), config.SetObjectEntry, Persisted MCP discovery index (mcp-index.json), Markdown-lite response rendering, MCP gating (read-only / approve list / session allow), mcp import (source discovery, secret rewriting), moca mcp import subcommand (+12 more)

### Community 30 - "newAgentModel"
Cohesion: 0.11
Nodes (25): TestExitCommandQuits(), TestCompactEscapeCancels(), TestCompactingBlocksRun(), TestCompactNothing(), TestEditItemShowsDiff(), TestThinkingLinePrintsBeforeTheResponse(), TestApprovalKeysAnswerAfterPause(), TestEnterAndCtrlCDuringApproval() (+17 more)

### Community 31 - "Create"
Cohesion: 0.09
Nodes (24): TestDelete(), TestListAll(), TestFind(), TestFindMtimeTieDeterministic(), TestInWorkdir(), lockFile(), lockFile(), NewSnapshots() (+16 more)

### Community 32 - "Context manager (token-denominated budgets)"
Cohesion: 0.14
Nodes (13): Context-pressure status bar colors shipped, Context manager (token-denominated budgets), reserveTokens 16K / keepRecentTokens 20K budgets, Overflow compact-and-retry, -p output contract + exit codes 0/1/2/3/130, Project trust (--approve/--no-approve, trust.json), Retry policy: backoff + SSE stall timeout + mid-stream rule, Status bar (two lines: version/cwd/branch + model/effort/ctx/tokens/cost) (+5 more)

### Community 33 - "TUI (Bubble Tea + lipgloss, inline immutable scrollback)"
Cohesion: 0.10
Nodes (19): esc kills a running !cmd (process group), Unfocused-run notifications shipped (tui.notify), ! / !! shell prefixes (user-initiated, bypass analysis), shift+enter newline via enhanced keyboard reporting, Steering: input during a run lands after tool results, Thinking blocks rendered collapsed (pager-expanded), TUI (Bubble Tea + lipgloss, inline immutable scrollback), 2.1 Turn spacing and hanging indent — **S** (+11 more)

### Community 34 - "importer.go"
Cohesion: 0.17
Nodes (20): MCPServer, TestImportRewritesEmbeddedSecretsInValues(), TestRewriteSecretsValueRuleAlone(), DiscoverSources(), embeddedSecret(), envName(), literalSecretFields(), ParseSource() (+12 more)

### Community 35 - "model"
Cohesion: 0.17
Nodes (10): model, matchCommands(), model, mentionMatches(), mentionQuery(), rankFile(), TestMentionQuery(), TestMentionRanking() (+2 more)

### Community 36 - "Interactive Bubble Tea TUI"
Cohesion: 0.14
Nodes (19): Activity row (live estimated tokens), Terminal color profile handling, Compaction budgets (NewBudget, FindCut, keepRecent), Usage-anchored context estimate, Context-pressure color in status bar, Agent control surface (SetModel/SetEffort/ToggleHard/Status/Steer), Event pipe (FIFO) bridge/threading, Agent event types (+11 more)

### Community 37 - ".openSessions"
Cohesion: 0.22
Nodes (9): Delete(), Info, List(), ListAll(), TestListPreviewsAndFilters(), fmtAge(), model, sessionHint() (+1 more)

### Community 38 - "update/update.go"
Cohesion: 0.08
Nodes (35): envOr(), githubToken(), runUpdate(), Compare(), comparePre(), ExtractBinary(), FindAsset(), FindChecksums() (+27 more)

### Community 39 - "TUI review & polish plan"
Cohesion: 0.17
Nodes (9): @path file mentions shipped, 0. Fixed in this pass (already on `fix/polish`), 2. Deliberately out of scope, 3. Suggested sequencing (as planned; all but 2.6 and 3.2 landed together on this branch), 1.5 @path file mentions, PR sequencing A–D + resize investigation, TUI review & polish plan, Slash commands + prompt templates + /create-command (+1 more)

### Community 40 - "RenderStatus"
Cohesion: 0.11
Nodes (23): AutoAllow(), Answer, Question, approvalPanel(), approvalPrompt(), TestApprovalPromptSanitized(), diffPreview(), TestDiffPreview() (+15 more)

### Community 41 - "cut_test.go"
Cohesion: 0.09
Nodes (32): Kind, Agent, summarizerUnavailable(), Budget, NewBudget(), TestBudget(), FindCut(), a() (+24 more)

### Community 42 - "Message"
Cohesion: 0.16
Nodes (16): NormalizeToolID(), TestNormalizeToolID(), TestTransformDropsEmptiedAssistantAndMerges(), TestTransformDropsRedactedForeignThinking(), TestTransformHistory(), TestTransformSameBareIDDifferentProvider(), TransformHistory(), Message (+8 more)

### Community 43 - "External tools: rtk, graphify, skills ecosystem"
Cohesion: 0.13
Nodes (12): CI gate (gofmt + vet + build + race test), docs/specs/DESIGN.md — v1 contract/vision, docs/specs/SPECS.md — living normative spec, graphify query/path/explain workflow rules, moca — one Go binary TUI coding agent, Ship gate test/shipgate/run.sh, External tools: rtk, graphify, skills ecosystem, graphify — codebase knowledge graph integration (+4 more)

### Community 44 - "2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)"
Cohesion: 0.12
Nodes (13): apiKey must be an env:VAR reference, 1.1 Terms, 1.2 Client registration, 1.3 Decision, 1. anthropic — decision: `api_key only` (subscription OAuth not permitted), 2.1 Terms, 2.2 Client registration (per the docs), 2.3 Endpoints (verified live 2026-10-06 against `https://auth.openai.com/.well-known/openid-configuration`) (+5 more)

### Community 45 - "v0.1.0-alpha (2026-10-07)"
Cohesion: 0.12
Nodes (15): CHANGELOG, CLI, Context & sessions, External tools & the skills ecosystem, Markdown-lite responses shipped, MCP, OAuth (subscription login), Providers & protocols (+7 more)

### Community 46 - "renderMarkdown"
Cohesion: 0.19
Nodes (14): agentEvent(), codeBandStyle(), fenceLang(), renderInline(), renderMarkdown(), plainMD(), TestFenceSurvivesCommitLive(), TestMarkdownBlocks() (+6 more)

### Community 47 - "Architecture: package layout & dependency direction"
Cohesion: 0.13
Nodes (13): Exit keeps the final status bar + resume hint, Resume replay + /resume picker shipped, agent package — the loop, Architecture: package layout & dependency direction, Cache breakpoints (≤4, rolled forward), llm leaf package (imports nothing internal), Prompt-cache discipline, Resume: --resume / --continue (+5 more)

### Community 48 - "Skills system (Agent Skills standard, per-project/global/built-in)"
Cohesion: 0.14
Nodes (10): Explicit non-goals (§15), graphify — codebase-graph CLI + drop-in skill, maxSteps runaway guard (default 40, tool_choice:none wrap-up), rtk — token-compressed CLI proxy + built-in skill, shell command analysis via mvdan.cc/sh, Hard-deny + ask-every-time sets (rm ask-every-time), Ship gate (§14): one unattended session on opencode-go, Skills system (Agent Skills standard, per-project/global/built-in) (+2 more)

### Community 49 - "sessions_test.go"
Cohesion: 0.25
Nodes (16): keyMsg(), TestAllowAlwaysRequiresCtrl(), TestApprovalKeysIgnoredWhileTyping(), TestPasteInsertsAtCursor(), sessionsDir(), openSessionPicker(), pickRow(), seedSession() (+8 more)

### Community 50 - "drained"
Cohesion: 0.47
Nodes (9): drained(), newLoginModel(), TestLoginMarkedBackAndCancel(), TestLoginOAuthDeclinedFlipAndCancel(), TestLoginOAuthFlowAndAuthFlip(), TestLoginRefusedWhileRunning(), TestLoginStoresAPIKey(), TestLogoutClearsStoredCredential() (+1 more)

### Community 51 - "internal/skills"
Cohesion: 0.20
Nodes (13): Built-in rtk skill + PATH filtering, Builtin skills embedding and extraction, Ecosystem compatibility corpus, graphify query/explain CLI integration, LoadInstructions (AGENTS.md / CLAUDE.md), Phase-6 rtk/graphify gate, internal/skills, rtk/graphify/skills ecosystem compatibility (+5 more)

### Community 52 - "moca minimal coding agent"
Cohesion: 0.14
Nodes (10): Compaction summary request (cheap model, lossy serialization), Context manager (token budgets, compaction, resume), Go 1.27.1 pinned in go.mod toolchain directive, make release — cross-compiled binaries, maxSteps wrap-up request with tool_choice:none, make targets mirrored as mise tasks, moca minimal coding agent, Phase-4 live compaction/resume gates (+2 more)

### Community 53 - "Find"
Cohesion: 0.23
Nodes (13): canonical(), Find(), FindForWorkdir(), header(), headerAndPreview(), idOf(), InWorkdir(), list() (+5 more)

### Community 54 - "SOURCES.md"
Cohesion: 0.19
Nodes (7): internal/skills/testdata/ecosystem corpus, anthropics/skills frontend-design, anthropics/skills skill-creator, graphify skill (graphify install, claude platform; byte-identical to pi variant), adeotek/dotfiles opencode dotnet-unit-testing skill, npm pi-mcp-adapter mcp-scripting skill, npm pi-subagents skill

### Community 55 - "Claude Code skill-creator skill"
Cohesion: 0.16
Nodes (13): Skill description (trigger) optimization, Skill eval/iterate loop (with-skill vs baseline), skill-creator Apache 2.0 License (Anthropic), Claude Code skill-creator skill, SKILL.md anatomy + bundled resources layout, Constrained query expansion against graph vocab, Progressive Disclosure (three-level skill loading), graphify query/path/explain reference (+5 more)

### Community 56 - "create-command.md — starter prompt template"
Cohesion: 0.24
Nodes (9): /create-command starter template (never overwritten), Ask-write root for ~/.config/moca/prompts/ (rev 15), create-command starter prompt seeded on first TUI run (rev 15), Path jail (session-start anchored, symlink-resolved), Prompt templates / slash commands (§10 mechanism 2), Read-only roots for global/built-in skill dirs, TUI re-reads prompt dirs when a run finishes (rev 15), create-command.md — starter prompt template (+1 more)

### Community 57 - "Provider layer: protocols × providers × auth"
Cohesion: 0.23
Nodes (11): New provider ollama (local/LAN server, model discovery), Cross-provider history transform (thinking/signatures/tool ids), Effort as a first-class request parameter, Built-in model catalog (thinkingMode, thinkingLevelMap, cost), anthropic provider, Provider layer: protocols × providers × auth, openai provider, opencode-go provider (+3 more)

### Community 58 - "Config"
Cohesion: 0.17
Nodes (10): ContextConfig, MCPConfig, ShellConfig, SnapshotConfig, TUIConfig, Default(), Config, IsLocalProvider() (+2 more)

### Community 59 - "prepare"
Cohesion: 0.22
Nodes (13): discoverLocal(), StartOptions, prepare(), TestPathsHonourXDG(), AuthFile(), ConfigDir(), ConfigFile(), DataDir() (+5 more)

### Community 60 - "retry_test.go"
Cohesion: 0.29
Nodes (16): Retry, DefaultRetryPolicy(), RetryNotice, fail(), TestMidStreamFailureResetsAndRetriesOnce(), TestMidStreamRetryBacksOffAndNotifies(), TestNoRetryOn400(), testPolicy() (+8 more)

### Community 61 - "build"
Cohesion: 0.09
Nodes (17): build(), ToolSpec, TestProxySpecFrozen(), Builtins(), MCPSpec(), ShellEnv(), TestShellEnv(), Registry (+9 more)

### Community 62 - ".Run"
Cohesion: 0.15
Nodes (17): applyEdit(), formatDiff(), indentOf(), joinInts(), lineOf(), matchLines(), replaceExact(), replaceFuzzy() (+9 more)

### Community 63 - "fixture/main.go"
Cohesion: 0.21
Nodes (7): Sum(), TestTotals(), ParseList(), Average(), Max(), TestMaxAndAverage(), main()

### Community 64 - "estimate.go"
Cohesion: 0.46
Nodes (5): MessageChars(), RequestChars(), TestEstimate(), Tokens(), UsageTokens()

### Community 65 - "The seven frozen tools"
Cohesion: 0.21
Nodes (11): Failing-test investigation protocol enforced by the edit tool, Lazy MCP proxy shipped, rtk/graphify allowlist + skills ecosystem compatibility, v0.1.0-alpha release (2026-10-07), Pre-edit snapshots + /undo, The seven frozen tools, edit tool (exact-match first, whitespace fallback, re-indent), ls tool (+3 more)

### Community 66 - "agent.Resume semantics"
Cohesion: 0.21
Nodes (12): Generalized dropdown (@path, ctrl+r history), --resume/--continue flags, Append-only JSONL sessions with pre-edit snapshots and undo, Persistent prompt history (history.jsonl), Resume replay of last turns, session.Find / FindForWorkdir lookup, session.Messages rebuild (compaction-aware), session.Repair (synthetic interrupted results) (+4 more)

### Community 67 - "internal/agent"
Cohesion: 0.42
Nodes (13): internal/agent, cmd/moca package, internal/compact, internal/config, internal/llm, internal/mcp, internal/permissions, internal/session (+5 more)

### Community 68 - "P1 — high value, every session"
Cohesion: 0.15
Nodes (13): 1.1 Markdown-lite rendering of responses — **M**, 1.2 Inline diff preview for `edit`/`write` items — **S/M**, 1.3 Context-pressure color in the status bar — **S**, 1.4 Argument completion in the `/` dropdown + interactive `/model` — **M**, 1.5 `@path` file mentions — **M**, 1.6 Resume shows where you left off; `/resume` picker — **M**, 1.7 Persistent input history + `ctrl+r` search — **S/M**, 1. Proposed improvements (priority order) (+5 more)

### Community 69 - "pager.go"
Cohesion: 0.18
Nodes (5): TestMatchRanges(), matchRanges(), newPager(), TestTeaAPI(), pagerModel

### Community 70 - "Start"
Cohesion: 0.12
Nodes (17): PromptInput, ServerLine, BuildSystemPrompt(), filterSkills(), GitState(), oneLine(), Platform(), TestBuildSystemPrompt() (+9 more)

### Community 71 - "history.go"
Cohesion: 0.36
Nodes (6): appendHistory(), loadHistory(), rewriteHistory(), TestHistoryPersistence(), hasControl(), historyRec

### Community 72 - "parseIgnore"
Cohesion: 0.32
Nodes (7): globToRegexp(), ignored(), parseIgnore(), TestIgnore(), ListFiles(), TestListFiles(), ignoreRule

### Community 73 - "format.go"
Cohesion: 0.26
Nodes (10): TestGitStatusUsesNoOptionalLocks(), AbbrevHome(), FmtTokens(), FmtWindow(), GitBranch(), gitStatusArgs(), TestFormatters(), TestGitBranch() (+2 more)

### Community 74 - "input_test.go"
Cohesion: 0.30
Nodes (11): NewInput(), TestCollapseUsesExactOffset(), TestCtrlC(), TestEditedMarkerSentLiterally(), TestHistory(), TestHistoryRecallChipsUnsafeContent(), TestMarkerNeverCollidesWithTypedText(), TestPasteChipKeepsContent() (+3 more)

### Community 75 - "JSONC config (string-literal-aware pre-pass, trailing commas)"
Cohesion: 0.10
Nodes (19): apiKey optional; stored key wins over env: reference, / autocomplete dropdown + command echo, v0.1.1 /login wizard + credential store in auth.json, moca update — self-update shipped, /model picker + argument completion shipped, Release workflow shipped (v* tag, checksums.txt), v0.1.1 release (2026-10-08), api_key auth mode with env:VAR indirection (+11 more)

### Community 76 - "Lazy MCP proxy (§10.5)"
Cohesion: 0.17
Nodes (11): Edit-ladder tests (non-negotiable, phase 2), MCP call gating: annotation-based, default-deny, Lazy MCP proxy (§10.5), mcp import literal-secret rewriting to env:VAR, Phase plan (7 phases, one PR each), Pi — architectural reference (compaction, MCP adapter), Hand-rolled protocol adapters (anthropic-messages, openai-completions, openai-responses), Structured summary format (Pi's proven shape) (+3 more)

### Community 77 - "initialize"
Cohesion: 0.20
Nodes (9): initialize(), TestHTTPCloseBounded(), startHTTP(), fakeHTTPServer(), TestHTTPNotifyStatusChecked(), TestHTTPTransport(), TestStdioEarlyExit(), headerLog (+1 more)

### Community 78 - ".DiscoverOllama"
Cohesion: 0.12
Nodes (15): ModelOverride, CostConfig, ProviderConfig, costM(), TestCostOf(), Registry, isDialFailure(), normalizeOllamaURL() (+7 more)

### Community 79 - "pasteReader"
Cohesion: 0.67
Nodes (4): parsePasted(), pasteReader(), TestParsePasted(), pastedResult

### Community 80 - "time.Duration"
Cohesion: 0.14
Nodes (10): post(), newHTTPError(), retryAfter(), TestRetryAfterFormats(), TestRetryAfterParsed(), TestFmtElapsed(), fmtElapsed(), fmtThinkDuration() (+2 more)

### Community 82 - "NewUnjailed"
Cohesion: 0.29
Nodes (4): Agent, NewUnjailed(), TestUnjailedAndAllowAll(), Unjailed

### Community 83 - "AGENTS.md — moca"
Cohesion: 0.25
Nodes (7): AGENTS.md — moca, Commands, Conventions & gotchas, Docs, graphify, Ground truth (read before changing behavior), Layout & dependency rules

### Community 84 - "picker.go"
Cohesion: 0.22
Nodes (7): choiceRow(), masked(), TestWindowRange(), model, windowRange(), pickItem, pickState

### Community 85 - "TestModelPickerRefreshesOllama"
Cohesion: 0.33
Nodes (6): contains(), count(), dropNames(), TestDropdownListsCommandsAndPrompts(), TestModelPickerRefreshesOllama(), TestShowCompletionNewestFirst()

### Community 86 - "Index"
Cohesion: 0.14
Nodes (10): Rank(), TestRank(), words(), Annotations, client, Hit, Index, IndexEntry (+2 more)

### Community 87 - "TurnEnd"
Cohesion: 0.50
Nodes (3): TurnEnd, StopReason, anthropicStop()

### Community 88 - "runTUI"
Cohesion: 0.27
Nodes (8): TestExitForOAuthSentinels(), exitFor(), firstLine(), resolveResume(), runOneShot(), decideTrust(), hasProjectResources(), runTUI()

### Community 89 - "retryable"
Cohesion: 0.29
Nodes (6): http2Transient(), retryable(), TestRetryableServerError400(), TestRetryable(), TestRetryableTransportErrors(), retrying

### Community 90 - "Snapshots"
Cohesion: 0.40
Nodes (3): gitClean(), Snapshots, SnapshotRec

### Community 92 - "ollamaStart"
Cohesion: 0.11
Nodes (19): ollamaServer(), ollamaStart(), TestOllamaDeclaredWindowIsQuiet(), TestOllamaReportedWindowIsQuiet(), TestOllamaStartErrors(), TestOllamaTinyWindowWarns(), TestStartWithOllamaModel(), abortingHTTPServer() (+11 more)

### Community 93 - "CI Workflow (gofmt · vet · build · test)"
Cohesion: 0.40
Nodes (4): CI Workflow (gofmt · vet · build · test), CI gate set (gofmt, go vet, go build, go test -race), go.mod (Go version source for setup-go), SPECS.md §2 — CI gate definition

### Community 94 - "README — user-facing overview"
Cohesion: 0.20
Nodes (8): Ben — owner / steering requests, moca — MO Coding Agent, OAuth verification (docs/specs/oauth-verification.md, verified 2026-10-06), config.jsonc quick-start (model, modelHard, providers), Docs pointers to SPECS/DESIGN/external-tools, Extend: skills, MCP, project instructions, external tools, Install (go install or releases page), README — user-facing overview

### Community 95 - "parseArgs"
Cohesion: 0.50
Nodes (3): parseArgs(), TestParseArgs(), Options

### Community 96 - "parseSlash"
Cohesion: 0.50
Nodes (4): parseSlash(), slashQuery(), TestSlashQuery(), TestParseSlash()

### Community 97 - "time.Time"
Cohesion: 0.31
Nodes (4): stampOf(), ReadTracker, stamp, padGuard

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

### Community 143 - "newProxy"
Cohesion: 0.16
Nodes (16): cutRunes(), isTrue(), NewTool(), newProxy(), runP(), TestCutRunesSafe(), TestProxyArgErrors(), TestProxyGating() (+8 more)

### Community 144 - "install.sh script"
Cohesion: 0.67
Nodes (5): die(), fetch(), note(), install.sh script, usage()

## Knowledge Gaps
- **188 isolated node(s):** `usageError`, `github.com/adeotek/moca`, `claudeServer`, `opencodeServer`, `request` (+183 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 357 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **43 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `model` connect `model` to `start.go`, `time.Time`, `model`, `pager.go`, `RenderStatus`, `NewDefaultStore`, `renderMarkdown`, `Sanitize`, `Agent`, `picker.go`, `Input`, `Run`, `prepare`, `Item`?**
  _High betweenness centrality (0.034) - this node is a cross-community bridge._
- **What connects `usageError`, `github.com/adeotek/moca`, `claudeServer` to the rest of the system?**
  _188 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `start.go` be split into smaller, more focused modules?**
  _Cohesion score 0.10528604884576781 - nodes in this community are weakly interconnected._
- **Why does `Writer` connect `Create` to `start.go`, `sync.Mutex`, `Agent`, `Snapshots`, `build`?**
  _High betweenness centrality (0.017) - this node is a cross-community bridge._
- **Should `newTestModel` be split into smaller, more focused modules?**
  _Cohesion score 0.07692307692307693 - nodes in this community are weakly interconnected._
- **Why does `Agent` connect `Agent` to `start.go`, `registry_test.go`, `model`, `context.Context`, `Model`, `sync.Mutex`, `Run`, `prepare`?**
  _High betweenness centrality (0.014) - this node is a cross-community bridge._
- **Should `oauth_test.go` be split into smaller, more focused modules?**
  _Cohesion score 0.05292702485966319 - nodes in this community are weakly interconnected._