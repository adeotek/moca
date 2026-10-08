# Graph Report - moca  (2026-10-08)

## Corpus Check
- 84 files · ~168,707 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1903 nodes · 6690 edges · 119 communities (90 shown, 29 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 784 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- go_pkg_strings
- agent_test.go
- Config
- Model
- testing.T
- tools/tool.go
- net/http.Client
- events.go
- permissions/shell.go
- oauth_test.go
- model
- hardening_test.go
- charm.land/bubbletea/v2.Cmd
- Agent
- anthropic_test.go
- tui.go
- read_test.go
- cli_test.go
- Input
- go_pkg_testing
- registry_test.go
- retry_test.go
- newTestModel
- fixes_test.go
- .Compact
- Compact
- shellrun_test.go
- types.go
- bridge.go
- write.go
- frontmatter.go
- Index
- jail.go
- Manager
- auth_test.go
- skills_test.go
- sync.Mutex
- mcp/tool_test.go
- editapply.go
- printed
- styleBlock
- retry.go
- session/entry.go
- context.Context
- Agent loop
- Run
- initialize
- SOURCES.md
- Claude Code skill-creator skill
- dropdown.go
- runTUI
- find.go
- serialize_test.go
- snapshot_test.go
- fixture/main.go
- TestLoginCLISwitchesAuthAfterCallback
- Edit tool ladder (exact → replace_all → whitespace fallback)
- input_test.go
- permissions/yolo.go
- v0.1.0-alpha (2026-10-07)
- key
- shellrun_windows.go
- format.go
- drained
- README.md — moca user-facing overview
- Model catalog (provider/model, contextWindow, cost)
- ignore.go
- tui/trust.go
- Item
- quick_validate.py
- AGENTS.md — moca
- rtk — token-compressed CLI proxy
- MCP lazy proxy (internal/mcp)
- Yolo mode (swaps injected checkers)
- pagerModel
- transform_test.go
- fakeServer
- Skills discovery (project/global/builtin precedence)
- Provider registry (Resolve, credential resolution)
- serverEnv
- readtracker.go
- approvalPrompt
- moca — one Go binary TUI coding agent
- oauth-verification.md
- 1. anthropic — decision: `api_key only` (subscription OAuth not permitted)
- 2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)
- estimate.go
- CI Workflow (gofmt · vet · build · test)
- v0.1.0-alpha release notes
- OpenAI subscription OAuth (Sign in with ChatGPT) implementation
- JSONC pre-pass (config.Standardize, length-preserving)
- retryable
- response
- Claude Code frontend-design skill
- pi mcp-scripting skill
- StartError
- Seven frozen tools with golden schemas
- moca design vision: minimal, token-efficient, provider-agnostic
- run.sh
- fakeCmds
- Cross-provider history transform rationale (thinking bound to producing model)
- Project trust (injected skills/prompts = prompt-injection surface)
- Arrange-Act-Assert with xUnit/NSubstitute
- maxSteps runaway protection (tool_choice: none wrap-up)
- example.com/totals
- github.com/adeotek/moca

## God Nodes (most connected - your core abstractions)
1. `model` - 56 edges
2. `newScript()` - 46 edges
3. `newTestModel()` - 37 edges
4. `run()` - 36 edges
5. `startTestWith()` - 35 edges
6. `key()` - 33 edges
7. `Config` - 31 edges
8. `ReadFile()` - 29 edges
9. `Parse()` - 29 edges
10. `textTurn()` - 26 edges

## Surprising Connections (you probably didn't know these)
- `Cross-tool skills and prompt templates ecosystem` --semantically_similar_to--> `SKILL.md files from pi/Claude Code/OpenCode load unchanged`  [INFERRED] [semantically similar]
  docs/external-tools.md → README.md
- `Built-in rtk skill (SKILL.md)` --semantically_similar_to--> `rtk — token-compressed CLI proxy`  [INFERRED] [semantically similar]
  internal/skills/builtin/rtk/SKILL.md → docs/external-tools.md
- `TestEditRefusesIncompleteInvestigation()` --calls--> `run()`  [INFERRED]
  internal/tools/shell_test.go → cmd/moca/main.go
- `TestShellToolApprovals()` --calls--> `run()`  [INFERRED]
  internal/tools/shell_test.go → cmd/moca/main.go
- `TestShellToolArmsAndClearsInvestigationState()` --calls--> `run()`  [INFERRED]
  internal/tools/shell_test.go → cmd/moca/main.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Cross-tool SKILL.md compatibility ecosystem** — readme_skill_compat, docs_external_tools_skills_ecosystem, internal_skills_builtin_rtk_skill, docs_external_tools_graphify [EXTRACTED 0.95]
- **Ecosystem SKILL.md compatibility corpus (graphify, Claude Code, pi, OpenCode)** — internal_skills_testdata_ecosystem_sources, internal_skills_testdata_ecosystem, internal_skills_testdata_ecosystem_graphify_graphify_vendored, internal_skills_testdata_ecosystem_claude_code_skill_creator_vendored, internal_skills_testdata_ecosystem_claude_code_frontend_design_vendored, internal_skills_testdata_ecosystem_pi_pi_subagents_vendored, internal_skills_testdata_ecosystem_pi_mcp_scripting_vendored, internal_skills_testdata_ecosystem_opencode_dotnet_unit_testing_vendored [EXTRACTED 1.00]
- **Per-provider OAuth policy decisions (phase 7 gate)** — docs_specs_oauth_verification_untolerated_unclear, docs_specs_oauth_verification_anthropic_decision, docs_specs_oauth_verification_openai_decision, docs_specs_oauth_verification_siwc, changelog_openai_subscription_oauth [EXTRACTED 1.00]
- **rtk allowlist laundering prevention flow** — changelog_rtk_allowlist_unwrap, docs_external_tools_rtk_unwrap, docs_external_tools_internal_permissions_rtk_go, internal_skills_builtin_rtk_skill_prefer_compressed [EXTRACTED 1.00]
- **Synthetic frontmatter edge-case fixtures (CRLF, BOM, folding, quoting, rich keys, long description)** — internal_skills_testdata_ecosystem_synthetic_bom_skill, internal_skills_testdata_ecosystem_synthetic_colon_unquoted_skill, internal_skills_testdata_ecosystem_synthetic_crlf_skill, internal_skills_testdata_ecosystem_synthetic_folded_skill, internal_skills_testdata_ecosystem_synthetic_literal_skill, internal_skills_testdata_ecosystem_synthetic_long_description_skill, internal_skills_testdata_ecosystem_synthetic_rich_keys_skill [EXTRACTED 1.00]
- **Context-lean pipeline (budgets → compaction → anchored estimate → lazy MCP → collapsed thinking)** — docs_specs_specs_compaction, docs_specs_specs_compaction_loop_guard, docs_specs_specs_context_estimate_anchor, docs_specs_specs_mcp_lazy_proxy, docs_specs_specs_tui [INFERRED 0.75]
- **Human-in-the-loop validation before declaring done** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_claude_code_frontend_design_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_review_validation [INFERRED 0.75]
- **Frozen-text contract protecting the prompt cache (schemas, system prompt, tool list, JSONC edits)** — docs_specs_specs_frozen_schemas_golden, docs_specs_specs_seven_frozen_tools, docs_specs_specs_system_prompt_built_once, docs_specs_specs_edit_tool_ladder, docs_specs_specs_jsonc_edit_helpers [INFERRED 0.85]
- **Permission stack steering the agent loop (jail, shell analysis, trust, MCP gating, yolo swap)** — docs_specs_specs_path_jail, docs_specs_specs_shell_command_analysis, docs_specs_specs_rtk_unwrap, docs_specs_specs_trust_store, docs_specs_specs_mcp_gating, docs_specs_specs_yolo_mode [INFERRED 0.85]
- **Parallel subagent fanout with parent synthesis** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_graphify_graphify_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_skill [INFERRED 0.85]
- **Skills referencing bundled reference files for progressive disclosure** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_graphify_graphify_skill, internal_skills_testdata_ecosystem_pi_mcp_scripting_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_skill [INFERRED 0.85]

## Communities (119 total, 29 thin omitted)

### Community 0 - "go_pkg_strings"
Cohesion: 0.07
Nodes (48): Compacted, Resumed, SteeringApplied, EnvError, MCPConfig, MCPServer, ModelOverride, ProviderConfig (+40 more)

### Community 1 - "agent_test.go"
Cohesion: 0.05
Nodes (94): allowAll, scriptServer, newScript(), startTest(), startTestWith(), startWith(), TestAbortWritesSyntheticResults(), TestFailedCallDoesNotCancelRest() (+86 more)

### Community 2 - "Config"
Cohesion: 0.06
Nodes (43): printUsage(), TestExitForOAuthSentinels(), loginAPIKey(), runLogin(), runLogout(), yesNo(), confirm(), runMCP() (+35 more)

### Community 3 - "Model"
Cohesion: 0.06
Nodes (19): savedModel, Status, Agent, SplitModel(), TestSplitModel(), Model, Registry, supportedEfforts() (+11 more)

### Community 4 - "testing.T"
Cohesion: 0.05
Nodes (54): Parse(), TestAPIKeyOptional(), TestDefaultsApplied(), TestEnvRefs(), TestOAuthAllowedForOpenAI(), TestOAuthRejectedForNonOAuthProvider(), TestResolveEnv(), TestRetentionZeroMeansForever() (+46 more)

### Community 5 - "tools/tool.go"
Cohesion: 0.08
Nodes (24): Builtins(), approve(), runReg(), TestInvestigationHint(), TestInvestigationRefusal(), TestRegistryInvestigationBanner(), decode(), errorf() (+16 more)

### Community 6 - "net/http.Client"
Cohesion: 0.07
Nodes (26): TestDefaultHTTPClientBoundsHeaderWait(), defaultHTTPClient(), post(), Store, newHostID(), validateAPIKey(), newHTTPError(), retryAfter() (+18 more)

### Community 7 - "events.go"
Cohesion: 0.11
Nodes (15): StreamReset, TextDelta, ThinkingDelta, Warning, YoloChanged, Event, findTestFile(), isPathByte() (+7 more)

### Community 8 - "permissions/shell.go"
Cohesion: 0.10
Nodes (26): setup, rtkRunTarget(), rtkShellString(), rtkSub(), rtkTarget(), TestRtkUnwrap(), literalSafe(), NewShell() (+18 more)

### Community 9 - "oauth_test.go"
Cohesion: 0.13
Nodes (27): browse(), firstURL(), newFakeAS(), runLogin(), TestCallbackPageReportsFailure(), TestHeadlessDetectionSSH(), TestJWKSExponentSanity(), TestLineReaderAbandonedReadKeepsLine() (+19 more)

### Community 10 - "model"
Cohesion: 0.12
Nodes (4): TestFmtElapsed(), fmtElapsed(), model, shellDoneMsg

### Community 11 - "hardening_test.go"
Cohesion: 0.17
Nodes (29): runMCPIndex(), abortingHTTPServer(), crashingServer(), shServer(), skipWindows(), TestCallTimeoutStopsWedgedServer(), TestCallToolNullArgsSentAsEmptyObject(), TestHandshakeTimeout() (+21 more)

### Community 12 - "charm.land/bubbletea/v2.Cmd"
Cohesion: 0.14
Nodes (10): OAuthProvider(), choiceRow(), model, masked(), providerNames(), providerRow(), loginChoice, loginState (+2 more)

### Community 13 - "Agent"
Cohesion: 0.12
Nodes (12): Options, ToolEnd, ToolStart, Agent, Outcome, New(), Effort, Response (+4 more)

### Community 14 - "anthropic_test.go"
Cohesion: 0.22
Nodes (27): Adapter, newAnthropic(), keyCred(), mustJSON(), sseServer(), TestAnthropicBodyHygiene(), TestAnthropicBudgetAndNoCache(), TestAnthropicBudgetFollowsEffort() (+19 more)

### Community 15 - "tui.go"
Cohesion: 0.17
Nodes (7): main(), TestUnjailedAndAllowAll(), hashFile(), TestEcosystemSourcesHashes(), Instruction, LoadInstructions(), TestLoadInstructions()

### Community 16 - "read_test.go"
Cohesion: 0.16
Nodes (20): TestLs(), TestReadRefusesNonRegularFile(), run(), testEnv(), TestReadBinaryAndDirAndMissing(), TestReadCaps(), TestReadRefusesHugeFile(), TestReadTracker() (+12 more)

### Community 17 - "cli_test.go"
Cohesion: 0.25
Nodes (24): parseArgs(), fakeCompletions(), isolate(), TestExitCodes(), TestHelpListsSurface(), TestInterruptedExit130(), TestLoginLogoutCLI(), TestMCPImportCLI() (+16 more)

### Community 18 - "Input"
Cohesion: 0.14
Nodes (7): lineCount(), hasControl(), Sanitize(), TestSanitize(), TestSanitizeC1(), Input, paste

### Community 19 - "go_pkg_testing"
Cohesion: 0.12
Nodes (12): PromptInput, ServerLine, BuildSystemPrompt(), filterSkills(), GitState(), oneLine(), Platform(), TestBuildSystemPrompt() (+4 more)

### Community 20 - "registry_test.go"
Cohesion: 0.21
Nodes (23): main(), AuthFile(), MigrateLegacyStore(), NewDefaultStore(), TestMigrateLegacyStore(), NewRegistry(), mustCfg(), TestAPIKeyMissingMentionsLogin() (+15 more)

### Community 21 - "retry_test.go"
Cohesion: 0.19
Nodes (13): fail(), ok(), TestMidStreamFailureResetsAndRetriesOnce(), TestMidStreamRetryBacksOffAndNotifies(), TestNoRetryOn400(), testPolicy(), TestRetryAfterCapped(), TestRetryBackoffThenSuccess() (+5 more)

### Community 22 - "newTestModel"
Cohesion: 0.15
Nodes (19): TestActivityLine(), newTestModel(), TestApprovalPreservesDraft(), TestExitCommandQuits(), TestInputRulesAndNoPrefix(), TestPasteDoesNotSend(), TestRunDoneClearsTransient(), TestStreamingCommitsCompletedLines() (+11 more)

### Community 23 - "fixes_test.go"
Cohesion: 0.15
Nodes (20): keyMsg(), openPager(), TestAllowAlwaysMCPPersists(), TestAllowAlwaysRequiresCtrl(), TestApprovalKeysAnswerAfterPause(), TestApprovalKeysIgnoredWhileTyping(), TestEnterAndCtrlCDuringApproval(), TestGitStatusUsesNoOptionalLocks() (+12 more)

### Community 24 - ".Compact"
Cohesion: 0.17
Nodes (10): Kind, Agent, summarizerUnavailable(), Budget, NewBudget(), TestBudget(), EntryTokens(), Entry (+2 more)

### Community 25 - "Compact"
Cohesion: 0.27
Nodes (16): Result, FindCut(), a(), call(), res(), TestFindCutKeepsRecentAtValidPoint(), TestFindCutNeverBetweenCallAndResult(), TestFindCutNothingToDo() (+8 more)

### Community 26 - "shellrun_test.go"
Cohesion: 0.15
Nodes (16): ShellOutput, runePrefix(), runeSuffix(), RunShell(), ShellEnv(), TestRunShellBasics(), TestRunShellBoundsCapture(), TestRunShellInvalidUTF8() (+8 more)

### Community 27 - "types.go"
Cohesion: 0.16
Nodes (15): ContentBlock, Event, Request, StopReason, ToolResult, anthropicOverflow(), anthropicStop(), openaiOverflow() (+7 more)

### Community 28 - "bridge.go"
Cohesion: 0.14
Nodes (13): Asker, newAsker(), newEventPipe(), TestEventPipeBuffersUntilStarted(), TestEventPipeNonBlockingAndFIFO(), flatten(), agentEventMsg, branchMsg (+5 more)

### Community 29 - "write.go"
Cohesion: 0.13
Nodes (9): ToolSpec, MCPSpec(), TestRegistryDispatch(), countFileLines(), countLines(), guardedWrite(), echoTool, lsTool (+1 more)

### Community 30 - "frontmatter.go"
Cohesion: 0.19
Nodes (12): closingQuote(), ParseFrontmatter(), scalar(), TestFrontmatter(), TestFrontmatterBlockAndQuoteEdges(), TestFrontmatterBlockIndicatorsAndComments(), HelpText(), ParseInput() (+4 more)

### Community 31 - "Index"
Cohesion: 0.12
Nodes (12): CallResult, Rank(), TestRank(), words(), Annotations, client, Content, Hit (+4 more)

### Community 32 - "jail.go"
Cohesion: 0.20
Nodes (12): canonical(), Jail, NewJail(), resolveDeep(), setup(), TestJail(), TestJailDanglingSymlinks(), TestJailKeepsNotYetExistingReadOnlyRoot() (+4 more)

### Community 33 - "Manager"
Cohesion: 0.29
Nodes (3): ConfigHash(), Manager, state

### Community 34 - "auth_test.go"
Cohesion: 0.21
Nodes (16): NewStore(), TestAPIKeyStore(), TestConcurrentRefreshOnce(), TestCredentialForLockWaitHonoursContext(), TestDeleteClearsCorruptStore(), TestHostIDStable(), TestInvalidClient(), TestLockFileWaitsThenAcquires() (+8 more)

### Community 35 - "skills_test.go"
Cohesion: 0.19
Nodes (12): builtinHash(), ExtractBuiltins(), extractBuiltins(), TestEcosystemSkillsLoadUnchanged(), Discover(), Skill, mkSkill(), TestDiscoverPrecedence() (+4 more)

### Community 36 - "sync.Mutex"
Cohesion: 0.20
Nodes (6): responseID(), TestResponseID(), startStdio(), TestStdioEarlyExit(), ring, stdioTransport

### Community 37 - "mcp/tool_test.go"
Cohesion: 0.24
Nodes (13): cutRunes(), errResult(), isTrue(), NewTool(), newProxy(), runP(), TestCutRunesSafe(), TestProxyArgErrors() (+5 more)

### Community 38 - "editapply.go"
Cohesion: 0.29
Nodes (14): applyEdit(), formatDiff(), indentOf(), joinInts(), lineOf(), matchLines(), replaceExact(), replaceFuzzy() (+6 more)

### Community 39 - "printed"
Cohesion: 0.22
Nodes (14): TestPagerCloseDefersTheRelease(), TestRefuseRunningWhileOutputInFlight(), TestCompactEscapeCancels(), TestCompactingBlocksRun(), TestCompactNothing(), TestCompactedAndResumedLines(), TestAllowAlwaysPersists(), ack() (+6 more)

### Community 40 - "styleBlock"
Cohesion: 0.22
Nodes (8): model, respMsgStyle(), styleBlock(), TestMessageBackgroundsCoverTheFullRow(), TestScrollbackMessageBackgrounds(), TestStyleBlockExpandsTabs(), TestStyleBlockWithoutWidth(), userMsgStyle()

### Community 41 - "retry.go"
Cohesion: 0.21
Nodes (8): Retry, DefaultRetryPolicy(), RetryNotice, WithRetry(), Options, HTTPError, retrying, RetryPolicy

### Community 42 - "session/entry.go"
Cohesion: 0.22
Nodes (11): TurnEnd, Usage, Entry, ModelChange, Compaction, ErrorInfo, Header, PermissionMode (+3 more)

### Community 43 - "context.Context"
Cohesion: 0.20
Nodes (7): fakeSum, readEvents(), lockFile(), lockFile(), runResult(), TestRunResultMapping(), httpTransport

### Community 44 - "Agent loop"
Cohesion: 0.18
Nodes (13): Token-denominated budgets (reserve/keepRecent, never orphan tool results), Session schema (entry ids + parentId chain, Pi's proven shape), Steering during runs (queued after tool batch), Transcript repair on abort/crash, Agent loop, Append-only JSONL transcript, Compaction (token budgets, cut points, structured summary), Compaction loop guard (no compaction twice in a row) (+5 more)

### Community 45 - "Run"
Cohesion: 0.16
Nodes (6): newModel(), Run(), TestWelcomeLines(), welcomeText(), AppOptions, compactDoneMsg

### Community 46 - "initialize"
Cohesion: 0.20
Nodes (8): initialize(), TestHTTPCloseBounded(), startHTTP(), fakeHTTPServer(), TestHTTPNotifyStatusChecked(), TestHTTPTransport(), headerLog, transport

### Community 47 - "SOURCES.md"
Cohesion: 0.19
Nodes (7): internal/skills/testdata/ecosystem corpus, anthropics/skills frontend-design, anthropics/skills skill-creator, graphify skill (graphify install, claude platform; byte-identical to pi variant), adeotek/dotfiles opencode dotnet-unit-testing skill, npm pi-mcp-adapter mcp-scripting skill, npm pi-subagents skill

### Community 48 - "Claude Code skill-creator skill"
Cohesion: 0.16
Nodes (13): Skill description (trigger) optimization, Skill eval/iterate loop (with-skill vs baseline), skill-creator Apache 2.0 License (Anthropic), Claude Code skill-creator skill, SKILL.md anatomy + bundled resources layout, Constrained query expansion against graph vocab, Progressive Disclosure (three-level skill loading), graphify query/path/explain reference (+5 more)

### Community 49 - "dropdown.go"
Cohesion: 0.26
Nodes (8): clampRunes(), model, matchCommands(), slashQuery(), TestClampRunes(), TestSlashQuery(), dropItem, dropState

### Community 50 - "runTUI"
Cohesion: 0.20
Nodes (11): decideTrust(), hasProjectResources(), runTUI(), LoadTrust(), TestTrustStore(), ExpandPrompt(), Prompt, LoadPrompts() (+3 more)

### Community 51 - "find.go"
Cohesion: 0.27
Nodes (10): Find(), FindForWorkdir(), header(), idOf(), listNewestFirst(), TestFind(), TestFindForWorkdirViaSymlink(), TestFindMtimeTieDeterministic() (+2 more)

### Community 52 - "serialize_test.go"
Cohesion: 0.22
Nodes (11): callLine(), CapChars(), safeTail(), Serialize(), TestCapChars(), TestCapCharsRuneSafe(), TestSerialize(), TestSerializeRuneSafeAndEmptyText() (+3 more)

### Community 53 - "snapshot_test.go"
Cohesion: 0.21
Nodes (9): gitClean(), Snapshots, NewSnapshots(), Prune(), newSnap(), TestGitCleanSkipsSnapshot(), TestPrune(), TestSnapshotUndoEditAndCreate() (+1 more)

### Community 54 - "fixture/main.go"
Cohesion: 0.22
Nodes (7): Sum(), TestTotals(), ParseList(), Average(), Max(), TestMaxAndAverage(), main()

### Community 55 - "TestLoginCLISwitchesAuthAfterCallback"
Cohesion: 0.21
Nodes (8): authURLFrom(), fakeSIWC(), fakeSIWCOpts(), TestLoginCLISwitchesAuthAfterCallback(), TestLoginTokenEndpointIsBounded(), waitFor(), lockedBuf, syncBuf

### Community 56 - "Edit tool ladder (exact → replace_all → whitespace fallback)"
Cohesion: 0.18
Nodes (10): Edit-tool contract ladder (re-indentation, CRLF/BOM rules), Phase plan (7 PRs, each reviewed and gated), Pre-edit snapshots + /undo (skipped for clean tracked git files), Seven tools frozen for v1 (schema churn rule), Ship gate (one unattended session: read → search → edit → green test → commit), Edit tool ladder (exact → replace_all → whitespace fallback), ReadTracker (mtime/size/sha256 per read), Seven frozen tools (read/write/edit/shell/search/ls/mcp) (+2 more)

### Community 57 - "input_test.go"
Cohesion: 0.30
Nodes (11): NewInput(), TestCollapseUsesExactOffset(), TestCtrlC(), TestEditedMarkerSentLiterally(), TestHistory(), TestHistoryRecallChipsUnsafeContent(), TestMarkerNeverCollidesWithTypedText(), TestPasteChipKeepsContent() (+3 more)

### Community 58 - "permissions/yolo.go"
Cohesion: 0.22
Nodes (4): Agent, NewUnjailed(), AllowAll, Unjailed

### Community 59 - "v0.1.0-alpha (2026-10-07)"
Cohesion: 0.18
Nodes (10): Changelog, CLI, Context & sessions, External tools & the skills ecosystem, MCP, OAuth (subscription login), Providers & protocols, Tools & permissions (+2 more)

### Community 60 - "key"
Cohesion: 0.31
Nodes (8): TestCoalescedShrinkRemembersTheLastFlushedRow(), TestInputBoxGrowsAndShrinks(), TestShrinkIsPaddedUntilTheGuardWindowPasses(), TestShrinkPaddingFitsTheScreen(), TestSpinnerTicksOnlyForTheLiveRun(), TestThinkingReadableInPager(), TestViewCursorInsideInputBox(), key()

### Community 61 - "shellrun_windows.go"
Cohesion: 0.27
Nodes (4): KillProcessGroup(), SetProcessGroup(), KillProcessGroup(), SetProcessGroup()

### Community 62 - "format.go"
Cohesion: 0.31
Nodes (9): AbbrevEffort(), AbbrevHome(), FmtTokens(), FmtWindow(), GitBranch(), gitStatusArgs(), TestFormatters(), TestGitBranch() (+1 more)

### Community 63 - "drained"
Cohesion: 0.47
Nodes (9): drained(), newLoginModel(), TestLoginMarkedBackAndCancel(), TestLoginOAuthDeclinedFlipAndCancel(), TestLoginOAuthFlowAndAuthFlip(), TestLoginRefusedWhileRunning(), TestLoginStoresAPIKey(), TestLogoutClearsStoredCredential() (+1 more)

### Community 64 - "README.md — moca user-facing overview"
Cohesion: 0.28
Nodes (6): Lazy MCP proxy (~200-token single tool), External tools: rtk, graphify, skills ecosystem, Cross-tool skills and prompt templates ecosystem, JSONC config at ~/.config/moca/config.jsonc, README.md — moca user-facing overview, SKILL.md files from pi/Claude Code/OpenCode load unchanged

### Community 65 - "Model catalog (provider/model, contextWindow, cost)"
Cohesion: 0.25
Nodes (9): Retry policy (backoff, stall timeout, mid-stream rule), Status bar (2-line, live usage/cost, red YOLO first), Cross-provider history transform (thinking replay, tool-id normalization), Effort clamping (ClampEffort), Model catalog (provider/model, contextWindow, cost), Three protocol adapters (anthropic-messages, openai-completions, openai-responses), Retry policy (backoff schedule, stall, mid-stream retry), Two-line status bar (event-driven) (+1 more)

### Community 66 - "ignore.go"
Cohesion: 0.33
Nodes (5): globToRegexp(), ignored(), parseIgnore(), TestIgnore(), ignoreRule

### Community 70 - "AGENTS.md — moca"
Cohesion: 0.25
Nodes (7): AGENTS.md — moca, Commands, Conventions & gotchas, Docs, graphify, Ground truth (read before changing behavior), Layout & dependency rules

### Community 71 - "rtk — token-compressed CLI proxy"
Cohesion: 0.29
Nodes (6): Credential store at ~/.config/moca/auth.json (0600), rtk unwrapping in the shell analyser so rtk cannot launder commands, v0.1.1 release notes, internal/permissions/rtk.go — captured rtk subcommand classes, rtk — token-compressed CLI proxy, Built-in rtk skill (SKILL.md)

### Community 72 - "MCP lazy proxy (internal/mcp)"
Cohesion: 0.29
Nodes (6): Lazy MCP proxy (§10.5: transports, index, import), Explicit non-goals (v1): subagents, plugins, sandboxing, MCP OAuth…, Persisted MCP discovery index (mcp-index.json, config hash), MCP lazy proxy (internal/mcp), MCP lifecycle Manager (lazy start, idle stop, replay-once), Package dependency direction (llm/config leaf → agent → tui)

### Community 73 - "Yolo mode (swaps injected checkers)"
Cohesion: 0.32
Nodes (7): Path jail (session-start anchored, symlink escape refused), Shell command analysis (every simple command checked), Yolo mode (explicit opt-in, sandboxed environments), MCP call gating (annotation-based, default-deny), Path jail (symlink-resolved, anchored at workdir), Shell command analysis (mvdan.cc/sh, wrapper unwrap), Yolo mode (swaps injected checkers)

### Community 75 - "transform_test.go"
Cohesion: 0.39
Nodes (7): NormalizeToolID(), TestNormalizeToolID(), TestTransformDropsEmptiedAssistantAndMerges(), TestTransformDropsRedactedForeignThinking(), TestTransformHistory(), TestTransformSameBareIDDifferentProvider(), TransformHistory()

### Community 76 - "fakeServer"
Cohesion: 0.32
Nodes (8): TestListToolsRepeatedCursor(), TestServerErrorTextDoesNotRestart(), TestUnknownServerAndTool(), dial(), fakeServer(), TestStdioListAndCall(), TestStdioPaged(), TestStdioWriteEPIPE()

### Community 77 - "Skills discovery (project/global/builtin precedence)"
Cohesion: 0.29
Nodes (6): Ecosystem compatibility as hard requirement (pi/claude-code/opencode skills load unchanged), Built-in rtk skill (CLI-gated, extraction by content hash), Prompt templates (/name, $ARGUMENTS expansion), rtk unwrap in shell analysis, Skills discovery (project/global/builtin precedence), Slash commands + /-dropdown autocomplete

### Community 78 - "Provider registry (Resolve, credential resolution)"
Cohesion: 0.33
Nodes (5): Provider layer: protocols × providers × auth (Pi's proven decomposition), Credential store (~/.config/moca/auth.json, 0600), env:VAR indirection (secret literals rejected), OpenAI subscription OAuth (SIWC, Responses route), Provider registry (Resolve, credential resolution)

### Community 79 - "serverEnv"
Cohesion: 0.33
Nodes (7): ResolveEnv(), TestKeepNameWindows(), FilterEnv(), keepName(), serverEnv(), TestFilterEnv(), TestServerEnvExplicitEntryReplacesInherited()

### Community 80 - "readtracker.go"
Cohesion: 0.52
Nodes (4): NewReadTracker(), stampOf(), ReadTracker, stamp

### Community 81 - "approvalPrompt"
Cohesion: 0.33
Nodes (7): AutoAllow(), Answer, Question, approvalPrompt(), firstLineOf(), TestApprovalPromptSanitized(), approvalMsg

### Community 82 - "moca — one Go binary TUI coding agent"
Cohesion: 0.33
Nodes (5): CI gate (gofmt + vet + build + race test), docs/specs/DESIGN.md — v1 contract/vision, docs/specs/SPECS.md — living normative spec, moca — one Go binary TUI coding agent, Ship gate test/shipgate/run.sh

### Community 84 - "1. anthropic — decision: `api_key only` (subscription OAuth not permitted)"
Cohesion: 0.33
Nodes (6): 1.1 Terms, 1.2 Client registration, 1.3 Decision, 1. anthropic — decision: `api_key only` (subscription OAuth not permitted), 3. Implementation deltas vs. the phase plan (recorded for the plan's Implementation notes), OAuth verification record (phase 7 — §3 policy gate)

### Community 85 - "2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)"
Cohesion: 0.33
Nodes (6): 2.1 Terms, 2.2 Client registration (per the docs), 2.3 Endpoints (verified live 2026-10-06 against `https://auth.openai.com/.well-known/openid-configuration`), 2.4 Inference contract for subscription traffic (open-source flow docs), 2.5 Decision, 2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)

### Community 86 - "estimate.go"
Cohesion: 0.60
Nodes (5): MessageChars(), RequestChars(), TestEstimate(), Tokens(), UsageTokens()

### Community 87 - "CI Workflow (gofmt · vet · build · test)"
Cohesion: 0.40
Nodes (4): CI Workflow (gofmt · vet · build · test), CI gate set (gofmt, go vet, go build, go test -race), go.mod (Go version source for setup-go), SPECS.md §2 — CI gate definition

### Community 88 - "v0.1.0-alpha release notes"
Cohesion: 0.40
Nodes (4): graphify query/path/explain workflow rules, graphify install --platform moca upstream PR #4174, v0.1.0-alpha release notes, graphify — codebase knowledge graph integration

### Community 89 - "OpenAI subscription OAuth (Sign in with ChatGPT) implementation"
Cohesion: 0.40
Nodes (4): OpenAI subscription OAuth (Sign in with ChatGPT) implementation, Dynamic client registration with issued client id (oaiapp_…), Rotating 30-day refresh tokens serialized across processes, Sign in with ChatGPT (SIWC) open-source token sharing

### Community 90 - "JSONC pre-pass (config.Standardize, length-preserving)"
Cohesion: 0.50
Nodes (3): JSONC comment-preserving edits (AppendString/SetObjectEntry/SetString), JSONC pre-pass (config.Standardize, length-preserving), moca mcp import (secret redaction to env:VAR)

### Community 91 - "retryable"
Cohesion: 0.40
Nodes (5): http2Transient(), retryable(), TestRetryableServerError400(), TestRetryable(), TestRetryableTransportErrors()

### Community 92 - "response"
Cohesion: 0.50
Nodes (3): callResult, response, rpcError

### Community 93 - "Claude Code frontend-design skill"
Cohesion: 0.50
Nodes (3): AI-generated design default tells, frontend-design Apache 2.0 License, Claude Code frontend-design skill

### Community 94 - "pi mcp-scripting skill"
Cohesion: 0.67
Nodes (4): jev.evaluate reference, jev.evaluate semantic scripting gate, mcpScript tools API, pi mcp-scripting skill

## Ambiguous Edges - Review These
- `Context estimate anchor (usage-anchored, chars/4 delta)` → `Persisted MCP discovery index (mcp-index.json, config hash)`  [AMBIGUOUS]
  docs/specs/SPECS.md · relation: conceptually_related_to

## Knowledge Gaps
- **86 isolated node(s):** `request`, `completionsChunk`, `Credential`, `claudeServer`, `opencodeServer` (+81 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 247 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **29 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **What is the exact relationship between `Context estimate anchor (usage-anchored, chars/4 delta)` and `Persisted MCP discovery index (mcp-index.json, config hash)`?**
  _Edge tagged AMBIGUOUS (relation: conceptually_related_to) - confidence is low._
- **Why does `model` connect `model` to `go_pkg_strings`, `agent_test.go`, `Model`, `Item`, `session/entry.go`, `pagerModel`, `charm.land/bubbletea/v2.Cmd`, `Run`, `approvalPrompt`, `dropdown.go`, `bridge.go`?**
  _High betweenness centrality (0.044) - this node is a cross-community bridge._
- **What connects `request`, `completionsChunk`, `Credential` to the rest of the system?**
  _86 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `go_pkg_strings` be split into smaller, more focused modules?**
  _Cohesion score 0.06962714704650189 - nodes in this community are weakly interconnected._
- **Why does `tuiCmd()` connect `drained` to `go_pkg_strings`, `charm.land/bubbletea/v2.Cmd`, `testing.T`?**
  _High betweenness centrality (0.022) - this node is a cross-community bridge._
- **Should `agent_test.go` be split into smaller, more focused modules?**
  _Cohesion score 0.05224089635854342 - nodes in this community are weakly interconnected._
- **Why does `Config` connect `Config` to `go_pkg_strings`, `agent_test.go`, `Model`, `testing.T`, `permissions/shell.go`, `hardening_test.go`, `charm.land/bubbletea/v2.Cmd`, `Agent`, `runTUI`, `registry_test.go`?**
  _High betweenness centrality (0.019) - this node is a cross-community bridge._