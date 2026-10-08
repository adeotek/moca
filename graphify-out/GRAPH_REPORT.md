# Graph Report - moca  (2026-10-08)

## Corpus Check
- 12 files · ~170,102 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1984 nodes · 6644 edges · 137 communities (94 shown, 43 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 783 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- go_pkg_strings
- oauth_test.go
- testing.T
- newScript
- context.Context
- time.Duration
- Create
- model
- LoadIndex
- frontmatter_test.go
- tools/tool.go
- v0.1.1 credentials move to ~/.config/moca/auth.json (0600)
- fixes_test.go
- Request
- permissions/shell.go
- cli_test.go
- Input
- Compact
- anthropic_test.go
- printed
- registry_test.go
- AppendString
- bridge.go
- retry_test.go
- sync.Mutex
- runLogin
- importer.go
- charm.land/bubbletea/v2.Cmd
- Agent
- permissions/shell_test.go
- Manager
- jail_test.go
- run
- .Compact
- Index
- newTestModel
- app.go
- moca — one Go binary TUI coding agent
- 2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)
- initialize
- auth_test.go
- Model
- prompt.go
- Message
- SOURCES.md
- Claude Code skill-creator skill
- editapply.go
- dropdown.go
- Agent
- runTUI
- Config
- status.go
- fixture/main.go
- §6 Context manager: reserveTokens 16K, keepRecent 20K, token-denominated budgets
- moca README — MO Coding Agent
- session/entry.go
- DESIGN.md — v1 contract (rev 14)
- Registry
- input_test.go
- message_style.go
- v0.1.0-alpha (2026-10-07)
- §14 Ship gate: one unattended session on opencode-go (read→search→edit→test→commit→rtk skill)
- teaapi_test.go
- ToolSpec
- §12 Config: JSONC chosen over TOML; string-literal-aware pre-pass; house dotfiles merge rationale
- loginState
- format.go
- drained
- v0.1.0 seven frozen tools: read/write/edit/shell/search/ls/mcp
- DataDir
- httpTransport
- quick_validate.py
- dropdown_test.go
- NewUnjailed
- AGENTS.md — moca
- §4 Seven tools frozen for v1; schema churn rule (frozen = v2 discussion)
- transform_test.go
- TestLoginCLISwitchesAuthAfterCallback
- tui/trust.go
- startCatalog
- serverEnv
- rtk token-compressed CLI proxy; analyser unwraps rtk <cmd>
- parseIgnore
- ReadTracker
- CI Workflow (gofmt · vet · build · test)
- CostConfig
- §11 TUI: inline immutable scrollback, pager, chips, multi-line paste, steering
- §14 Agent loop: turn contract, control surface, steering queue, length-stop split-work errors, event list
- Messages
- TurnEnd
- rev 13: TUI polish — two-line status bar, rules, full-row bands, /exit
- Claude Code frontend-design skill
- pi mcp-scripting skill
- allowAll
- StartError
- v0.1.0 CLI surface: moca, -p, login/logout, mcp import/index, exit codes 0/1/2/3/130
- v0.1.1 / autocomplete dropdown (built-ins + templates, tab complete)
- findTestFile
- items_test.go
- .loginPanel
- AllowAll
- Skills: SKILL.md in ~/.config/moca/skills or <repo>/.moca/skills; cross-tool compatibility
- run.sh
- boundedWriter
- fakeCmds
- Seven frozen tools with golden schemas
- Opencode-go catalog expansion: 10 new models (deepseek v4.1/v4, kimi-k2.7-code, mimo-v2.6, qwen3.8/3.7)
- Dynamic client registration with issued client id (oaiapp_…)
- Rotating 30-day refresh tokens serialized across processes
- anthropic-messages adapter: body shape, cache_control placement, stream events, stop mapping
- Arrange-Act-Assert with xUnit/NSubstitute
- moca Changelog (release history)
- v0.1.0-alpha — first release: seven tools, Bubble Tea TUI, no framework
- v0.1.0 token-denominated context manager + JSONL sessions with --resume/--continue
- Submitted / commands echo as user-band messages in scrollback
- Burst-aware inline-renderer shrink guard (remembers last flushed frame)
- Thinking blocks print item line at block end with streamed duration
- Welcome line product name in orange #d97706
- Session entry types: session, message, tool_use/result, compaction, model_change, permission_mode, snapshot, error
- Transcript repair: synthetic tool_result on abort/crash
- example.com/totals
- github.com/adeotek/moca

## God Nodes (most connected - your core abstractions)
1. `model` - 62 edges
2. `newScript()` - 46 edges
3. `newTestModel()` - 37 edges
4. `run()` - 36 edges
5. `startTestWith()` - 35 edges
6. `key()` - 33 edges
7. `Config` - 31 edges
8. `ReadFile()` - 29 edges
9. `Parse()` - 28 edges
10. `textTurn()` - 26 edges

## Surprising Connections (you probably didn't know these)
- `moca README — MO Coding Agent` --semantically_similar_to--> `§1 Identity: binary = repo = module github.com/adeotek/moca`  [INFERRED] [semantically similar]
  README.md → docs/specs/DESIGN.md
- `Built-in rtk skill (SKILL.md)` --semantically_similar_to--> `rtk — token-compressed CLI proxy`  [INFERRED] [semantically similar]
  internal/skills/builtin/rtk/SKILL.md → docs/external-tools.md
- `rev 14: credential store in config dir + /login wizard + / dropdown` --semantically_similar_to--> `v0.1.1 — credential store, /login, / autocomplete, bigger catalog`  [INFERRED] [semantically similar]
  docs/specs/DESIGN.md → CHANGELOG.md
- `TUI /login credential wizard: provider picker, masked entry, OAuth flow bridged by event pipe, auth-flip offer` --semantically_similar_to--> `v0.1.1 TUI /login and /logout commands`  [INFERRED] [semantically similar]
  docs/specs/SPECS.md → CHANGELOG.md
- `TUI /-dropdown autocomplete: prefix-filter, 8-row window, tab/enter/esc semantics` --semantically_similar_to--> `v0.1.1 / autocomplete dropdown (built-ins + templates, tab complete)`  [INFERRED] [semantically similar]
  docs/specs/SPECS.md → CHANGELOG.md

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Cross-tool SKILL.md compatibility ecosystem** — docs_external_tools_skills_ecosystem, internal_skills_builtin_rtk_skill, docs_external_tools_graphify [EXTRACTED 0.95]
- **Ecosystem SKILL.md compatibility corpus (graphify, Claude Code, pi, OpenCode)** — internal_skills_testdata_ecosystem_sources, internal_skills_testdata_ecosystem, internal_skills_testdata_ecosystem_graphify_graphify_vendored, internal_skills_testdata_ecosystem_claude_code_skill_creator_vendored, internal_skills_testdata_ecosystem_claude_code_frontend_design_vendored, internal_skills_testdata_ecosystem_pi_pi_subagents_vendored, internal_skills_testdata_ecosystem_pi_mcp_scripting_vendored, internal_skills_testdata_ecosystem_opencode_dotnet_unit_testing_vendored [EXTRACTED 1.00]
- **Per-provider OAuth policy decisions (phase 7 gate)** — docs_specs_oauth_verification_untolerated_unclear, docs_specs_oauth_verification_anthropic_decision, docs_specs_oauth_verification_openai_decision, docs_specs_oauth_verification_siwc [EXTRACTED 1.00]
- **rtk allowlist laundering prevention flow** — docs_external_tools_rtk_unwrap, docs_external_tools_internal_permissions_rtk_go, internal_skills_builtin_rtk_skill_prefer_compressed [EXTRACTED 1.00]
- **Synthetic frontmatter edge-case fixtures (CRLF, BOM, folding, quoting, rich keys, long description)** — internal_skills_testdata_ecosystem_synthetic_bom_skill, internal_skills_testdata_ecosystem_synthetic_colon_unquoted_skill, internal_skills_testdata_ecosystem_synthetic_crlf_skill, internal_skills_testdata_ecosystem_synthetic_folded_skill, internal_skills_testdata_ecosystem_synthetic_literal_skill, internal_skills_testdata_ecosystem_synthetic_long_description_skill, internal_skills_testdata_ecosystem_synthetic_rich_keys_skill [EXTRACTED 1.00]
- **Human-in-the-loop validation before declaring done** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_claude_code_frontend_design_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_review_validation [INFERRED 0.75]
- **Ship-gate loop: instrumented failing-test investigation backed by three tool/photo state gates** — docs_specs_design_ship_gate, docs_specs_specs_investigation_protocol, docs_specs_specs_shipgate_live_legs [INFERRED 0.85]
- **Parallel subagent fanout with parent synthesis** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_graphify_graphify_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_skill [INFERRED 0.85]
- **Skills referencing bundled reference files for progressive disclosure** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_graphify_graphify_skill, internal_skills_testdata_ecosystem_pi_mcp_scripting_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_skill [INFERRED 0.85]
- **Credential flow: TUI wizard / CLI login → auth.json → store-first per-request resolution** — changelog_v0_1_1_tui_login_logout, docs_specs_specs_credential_store, docs_specs_specs_oauth, docs_specs_specs_providers_registry [INFERRED 0.90]
- **The seven frozen tools shared across all docs** — docs_specs_design_tools_freeze, changelog_v0_1_0_seven_tools, docs_specs_specs_tools [INFERRED 0.90]

## Communities (137 total, 43 thin omitted)

### Community 0 - "go_pkg_strings"
Cohesion: 0.07
Nodes (23): Compacted, Resumed, SteeringApplied, StreamReset, TextDelta, ThinkingDelta, Warning, YoloChanged (+15 more)

### Community 1 - "oauth_test.go"
Cohesion: 0.06
Nodes (49): fakeSIWC(), fakeSIWCOpts(), Store, newHostID(), validateAPIKey(), audContains(), b64d(), OAuthConfig (+41 more)

### Community 2 - "testing.T"
Cohesion: 0.04
Nodes (70): TestResumeNotASession(), CapChars(), TestCapChars(), TestCapCharsRuneSafe(), Parse(), TestAPIKeyOptional(), TestDefaultsApplied(), TestEnvRefs() (+62 more)

### Community 3 - "newScript"
Cohesion: 0.10
Nodes (55): scriptServer, newScript(), startTest(), startTestWith(), startWith(), TestAbortWritesSyntheticResults(), TestFailedCallDoesNotCancelRest(), TestLengthStopTruncatedToolCall() (+47 more)

### Community 4 - "context.Context"
Cohesion: 0.08
Nodes (22): responseID(), TestResponseID(), errResult(), lockFile(), lockFile(), approve(), TestTestRunCommand(), TestTestRunFailed() (+14 more)

### Community 5 - "time.Duration"
Cohesion: 0.08
Nodes (21): Retry, fakeHTTPServer(), TestHTTPTransport(), post(), newHTTPError(), retryAfter(), DefaultRetryPolicy(), RetryNotice (+13 more)

### Community 6 - "Create"
Cohesion: 0.07
Nodes (31): Find(), FindForWorkdir(), header(), idOf(), listNewestFirst(), TestFind(), TestFindForWorkdirViaSymlink(), TestFindMtimeTieDeterministic() (+23 more)

### Community 7 - "model"
Cohesion: 0.08
Nodes (7): newModel(), Run(), runResult(), TestRunResultMapping(), simulate(), AppOptions, model

### Community 8 - "LoadIndex"
Cohesion: 0.13
Nodes (37): crashingServer(), shServer(), skipWindows(), TestCallTimeoutStopsWedgedServer(), TestHandshakeTimeout(), TestHTTPAbortedStreamReplayRule(), TestIndexSaveMergesConcurrentWriters(), TestSearchReportsAndRemembersFailedServer() (+29 more)

### Community 9 - "frontmatter_test.go"
Cohesion: 0.08
Nodes (30): builtinHash(), ExtractBuiltins(), extractBuiltins(), TestEcosystemSkillsLoadUnchanged(), closingQuote(), ParseFrontmatter(), scalar(), TestFrontmatter() (+22 more)

### Community 10 - "tools/tool.go"
Cohesion: 0.09
Nodes (21): Builtins(), runReg(), TestInvestigationHint(), TestInvestigationRefusal(), TestRegistryInvestigationBanner(), AutoAllow(), Answer, Asker (+13 more)

### Community 11 - "v0.1.1 credentials move to ~/.config/moca/auth.json (0600)"
Cohesion: 0.06
Nodes (30): v0.1.0 anthropic API-key-only policy (phase-7 gate, oauth-verification.md), v0.1.0 day-1 providers: anthropic, opencode-go, openai + custom baseUrl providers, v0.1.0 OpenAI subscription OAuth: Sign in with ChatGPT (PKCE S256, dynamic registration, loopback, JWKS, rotating refresh tokens), v0.1.1 — credential store, /login, / autocomplete, bigger catalog, v0.1.1 credentials move to ~/.config/moca/auth.json (0600), moca login stores API keys (--api-key, silent pipe input); moca logout, v0.1.1 TUI /login and /logout commands, Effort as first-class request parameter (off..max), clamped per model (+22 more)

### Community 12 - "fixes_test.go"
Cohesion: 0.13
Nodes (27): key(), keyMsg(), TestExitCommandQuits(), TestCompactEscapeCancels(), TestCompactingBlocksRun(), TestCompactNothing(), TestThinkingLinePrintsBeforeTheResponse(), openPager() (+19 more)

### Community 13 - "Request"
Cohesion: 0.12
Nodes (18): ToolEnd, ToolStart, ContentBlock, Event, Request, Response, ToolCall, ToolChoice (+10 more)

### Community 14 - "permissions/shell.go"
Cohesion: 0.16
Nodes (17): rtkRunTarget(), rtkShellString(), rtkSub(), rtkTarget(), literalSafe(), numericFlag(), patternSafe(), skipCommand() (+9 more)

### Community 15 - "cli_test.go"
Cohesion: 0.21
Nodes (26): fakeCompletions(), isolate(), TestExitCodes(), TestHelpListsSurface(), TestInterruptedExit130(), TestLoginLogoutCLI(), TestMCPImportCLI(), TestNoApproveSkipsProjectInstructions() (+18 more)

### Community 16 - "Input"
Cohesion: 0.13
Nodes (7): lineCount(), hasControl(), Sanitize(), TestSanitize(), TestSanitizeC1(), Input, paste

### Community 17 - "Compact"
Cohesion: 0.18
Nodes (22): FindCut(), a(), call(), res(), TestFindCutKeepsRecentAtValidPoint(), TestFindCutNeverBetweenCallAndResult(), TestFindCutNothingToDo(), TestFindCutSplitTurn() (+14 more)

### Community 18 - "anthropic_test.go"
Cohesion: 0.24
Nodes (26): Adapter, newAnthropic(), keyCred(), mustJSON(), sseServer(), TestAnthropicBodyHygiene(), TestAnthropicBudgetAndNoCache(), TestAnthropicBudgetFollowsEffort() (+18 more)

### Community 19 - "printed"
Cohesion: 0.13
Nodes (18): TestPagerCloseDefersTheRelease(), TestRefuseRunningWhileOutputInFlight(), TestCompactedAndResumedLines(), TestAllowAlwaysPersists(), ack(), flatten(), printed(), TestBackgroundColorMsgSwitchesVariant() (+10 more)

### Community 20 - "registry_test.go"
Cohesion: 0.20
Nodes (19): NewRegistry(), mustCfg(), TestAPIKeyMissingMentionsLogin(), TestAPIKeyStoreWinsOverEnv(), TestBaseURLPrecedence(), TestBuiltinModelNeedsProtocol(), TestBuiltinProviderNewModelWithModelProtocol(), TestCheckCredentialOAuthDoesNotTouchNetwork() (+11 more)

### Community 21 - "AppendString"
Cohesion: 0.14
Nodes (19): scanner, span, AppendString(), concat(), nested(), nestedEntry(), objectHasKey(), objectValueSpan() (+11 more)

### Community 23 - "bridge.go"
Cohesion: 0.13
Nodes (13): newAsker(), newEventPipe(), TestEventPipeBuffersUntilStarted(), TestEventPipeNonBlockingAndFIFO(), TestPipeSendKeepsOrderWithEvents(), agentEventMsg, branchMsg, compactDoneMsg (+5 more)

### Community 24 - "retry_test.go"
Cohesion: 0.19
Nodes (13): fail(), ok(), TestMidStreamFailureResetsAndRetriesOnce(), TestMidStreamRetryBacksOffAndNotifies(), TestNoRetryOn400(), testPolicy(), TestRetryAfterCapped(), TestRetryBackoffThenSuccess() (+5 more)

### Community 25 - "sync.Mutex"
Cohesion: 0.12
Nodes (11): callResult, startStdio(), TestStdioWriteEPIPE(), KillProcessGroup(), SetProcessGroup(), KillProcessGroup(), SetProcessGroup(), response (+3 more)

### Community 26 - "runLogin"
Cohesion: 0.16
Nodes (17): printUsage(), loginAPIKey(), runLogin(), yesNo(), confirm(), runMCP(), runMCPImport(), runMCPIndex() (+9 more)

### Community 27 - "importer.go"
Cohesion: 0.15
Nodes (19): TestImportRewritesEmbeddedSecretsInValues(), TestRewriteSecretsValueRuleAlone(), DiscoverSources(), embeddedSecret(), envName(), literalSecretFields(), ParseSource(), Plan() (+11 more)

### Community 28 - "charm.land/bubbletea/v2.Cmd"
Cohesion: 0.23
Nodes (8): NewDefaultStore(), Headless(), OAuthProvider(), TestHeadlessDetectionSSH(), model, providerNames(), providerRow(), loginDoneMsg

### Community 29 - "Agent"
Cohesion: 0.18
Nodes (7): Options, Agent, Outcome, New(), Effort, ParseEffort(), TestParseEffort()

### Community 30 - "permissions/shell_test.go"
Cohesion: 0.14
Nodes (14): setup, TestDefaultHTTPClientBoundsHeaderWait(), build(), defaultHTTPClient(), StartOptions, prepare(), TestRtkUnwrap(), NewShell() (+6 more)

### Community 31 - "Manager"
Cohesion: 0.23
Nodes (6): Rank(), TestRank(), words(), Manager, Hit, state

### Community 32 - "jail_test.go"
Cohesion: 0.15
Nodes (15): canonical(), Jail, NewJail(), resolveDeep(), setup(), TestJail(), TestJailDanglingSymlinks(), TestJailKeepsNotYetExistingReadOnlyRoot() (+7 more)

### Community 33 - "run"
Cohesion: 0.19
Nodes (18): TestLs(), TestReadRefusesNonRegularFile(), run(), testEnv(), TestReadBinaryAndDirAndMissing(), TestReadCaps(), TestReadRefusesHugeFile(), TestReadTracker() (+10 more)

### Community 34 - ".Compact"
Cohesion: 0.18
Nodes (8): Agent, summarizerUnavailable(), Budget, NewBudget(), TestBudget(), Prev, Summarizer, TextOf()

### Community 35 - "Index"
Cohesion: 0.12
Nodes (13): cutRunes(), isTrue(), TestCutRunesSafe(), runePrefix(), runeSuffix(), TestTruncate(), TestTruncateLongLines(), Truncate() (+5 more)

### Community 36 - "newTestModel"
Cohesion: 0.20
Nodes (15): TestActivityLine(), TestCoalescedShrinkRemembersTheLastFlushedRow(), TestInputBoxGrowsAndShrinks(), TestShrinkIsPaddedUntilTheGuardWindowPasses(), TestShrinkPaddingFitsTheScreen(), TestSpinnerTicksOnlyForTheLiveRun(), TestThinkingReadableInPager(), TestViewCursorInsideInputBox() (+7 more)

### Community 37 - "app.go"
Cohesion: 0.12
Nodes (12): approvalPrompt(), firstLineOf(), supportedEfforts(), TestApprovalPromptSanitized(), TestWelcomeLines(), welcomeText(), hintCheckMsg, padGuard (+4 more)

### Community 38 - "moca — one Go binary TUI coding agent"
Cohesion: 0.13
Nodes (12): CI gate (gofmt + vet + build + race test), docs/specs/DESIGN.md — v1 contract/vision, docs/specs/SPECS.md — living normative spec, graphify query/path/explain workflow rules, moca — one Go binary TUI coding agent, Ship gate test/shipgate/run.sh, External tools: rtk, graphify, skills ecosystem, graphify — codebase knowledge graph integration (+4 more)

### Community 39 - "2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)"
Cohesion: 0.12
Nodes (13): apiKey must be an env:VAR reference, 1.1 Terms, 1.2 Client registration, 1.3 Decision, 1. anthropic — decision: `api_key only` (subscription OAuth not permitted), 2.1 Terms, 2.2 Client registration (per the docs), 2.3 Endpoints (verified live 2026-10-06 against `https://auth.openai.com/.well-known/openid-configuration`) (+5 more)

### Community 40 - "initialize"
Cohesion: 0.14
Nodes (13): CallResult, initialize(), TestHTTPCloseBounded(), TestListToolsRepeatedCursor(), startHTTP(), TestHTTPNotifyStatusChecked(), dial(), TestStdioEarlyExit() (+5 more)

### Community 41 - "auth_test.go"
Cohesion: 0.21
Nodes (16): NewStore(), TestAPIKeyStore(), TestConcurrentRefreshOnce(), TestCredentialForLockWaitHonoursContext(), TestDeleteClearsCorruptStore(), TestHostIDStable(), TestInvalidClient(), TestLockFileWaitsThenAcquires() (+8 more)

### Community 42 - "Model"
Cohesion: 0.19
Nodes (3): savedModel, Status, Model

### Community 43 - "prompt.go"
Cohesion: 0.22
Nodes (11): PromptInput, ServerLine, BuildSystemPrompt(), filterSkills(), GitState(), oneLine(), Platform(), TestBuildSystemPrompt() (+3 more)

### Community 44 - "Message"
Cohesion: 0.23
Nodes (13): Kind, EntryTokens(), Entry, MessageChars(), RequestChars(), TestEstimate(), Tokens(), UsageTokens() (+5 more)

### Community 45 - "SOURCES.md"
Cohesion: 0.19
Nodes (7): internal/skills/testdata/ecosystem corpus, anthropics/skills frontend-design, anthropics/skills skill-creator, graphify skill (graphify install, claude platform; byte-identical to pi variant), adeotek/dotfiles opencode dotnet-unit-testing skill, npm pi-mcp-adapter mcp-scripting skill, npm pi-subagents skill

### Community 46 - "Claude Code skill-creator skill"
Cohesion: 0.16
Nodes (13): Skill description (trigger) optimization, Skill eval/iterate loop (with-skill vs baseline), skill-creator Apache 2.0 License (Anthropic), Claude Code skill-creator skill, SKILL.md anatomy + bundled resources layout, Constrained query expansion against graph vocab, Progressive Disclosure (three-level skill loading), graphify query/path/explain reference (+5 more)

### Community 47 - "editapply.go"
Cohesion: 0.30
Nodes (14): applyEdit(), formatDiff(), indentOf(), joinInts(), lineOf(), matchLines(), replaceExact(), replaceFuzzy() (+6 more)

### Community 48 - "dropdown.go"
Cohesion: 0.26
Nodes (8): clampRunes(), model, matchCommands(), slashQuery(), TestClampRunes(), TestSlashQuery(), dropItem, dropState

### Community 50 - "runTUI"
Cohesion: 0.18
Nodes (12): parseArgs(), TestExitForOAuthSentinels(), TestParseArgs(), runLogout(), exitFor(), firstLine(), resolveResume(), runOneShot() (+4 more)

### Community 51 - "Config"
Cohesion: 0.18
Nodes (10): ContextConfig, MCPConfig, MCPServer, ShellConfig, SnapshotConfig, Default(), Config, Load() (+2 more)

### Community 52 - "status.go"
Cohesion: 0.24
Nodes (11): TestStatusWideRunesNeverExceedWidth(), FmtPercent(), renderLine(), RenderStatus(), TestFmtPercent(), TestRenderStatusFull(), TestRenderStatusNarrow(), TestRenderStatusVariants() (+3 more)

### Community 53 - "fixture/main.go"
Cohesion: 0.21
Nodes (7): Sum(), TestTotals(), ParseList(), Average(), Max(), TestMaxAndAverage(), main()

### Community 54 - "§6 Context manager: reserveTokens 16K, keepRecent 20K, token-denominated budgets"
Cohesion: 0.18
Nodes (11): Cache breakpoints: cache_control placement, ≤4, rolled forward, §6 Context manager: reserveTokens 16K, keepRecent 20K, token-denominated budgets, maxSteps 40: runaway guard, wrap-up via tool_choice none, Overflow recovery: one compact-and-retry; length-with-room ≠ overflow, Path jail anchored at session-start workdir, symlink-resolved; read-only skill roots, Project trust: --approve/--no-approve, trust.json; cloned repo prompt injection rationale, rev 7: compaction rewritten token-denominated; session schema; trust; steering, Structured summary format: Goal/Constraints/Progress/Key Decisions/Next Steps + cumulative file lists (+3 more)

### Community 55 - "moca README — MO Coding Agent"
Cohesion: 0.15
Nodes (13): §12.5 CLI surface: moca, -p flags, login/logout, mcp import/index, --version, §1 Identity: binary = repo = module github.com/adeotek/moca, §15 Explicit non-goals v1: no plugins/sandboxing/MCP OAuth etc., Phase plan 1–7 (each = one PR) with per-phase gates, §3 CLI surface as implemented (--config, --resume rules, login/logout detail, --help), §3 Exit codes 0/1/2/3/130 mapped per failure class (incl. OAuth invalid_grant → 2), §9 -p run semantics: trust defaults no, slug, wrap-up exit 3, Development: mise pins Go 1.27.1; make build/test/release; mise run targets (+5 more)

### Community 56 - "session/entry.go"
Cohesion: 0.29
Nodes (10): Result, Usage, Entry, ModelChange, Compaction, ErrorInfo, Header, PermissionMode (+2 more)

### Community 57 - "DESIGN.md — v1 contract (rev 14)"
Cohesion: 0.17
Nodes (8): §2 Architecture: internal packages (llm, agent, provider, tools, compact, session, tui, config, skills, permissions, mcp), DESIGN.md — v1 contract (rev 14), §2 Build/test/verify: make targets, version stamping, CI gate, shipgate build tag, §16 Maintenance: SPECS updated at end of every phase/behavior PR, SPECS.md — current implemented state (living, normative), §2 Package layout (implemented) + dependency directions incl. UI deps, Docs set: SPECS.md (current), DESIGN.md (v1 contract), external-tools.md, Token-efficiency levers: prompt caching, windowed reads, diff-shaped results, token compaction

### Community 58 - "Registry"
Cohesion: 0.26
Nodes (3): SplitModel(), TestSplitModel(), Registry

### Community 59 - "input_test.go"
Cohesion: 0.30
Nodes (11): NewInput(), TestCollapseUsesExactOffset(), TestCtrlC(), TestEditedMarkerSentLiterally(), TestHistory(), TestHistoryRecallChipsUnsafeContent(), TestMarkerNeverCollidesWithTypedText(), TestPasteChipKeepsContent() (+3 more)

### Community 60 - "message_style.go"
Cohesion: 0.27
Nodes (4): model, respMsgStyle(), styleBlock(), userMsgStyle()

### Community 61 - "v0.1.0-alpha (2026-10-07)"
Cohesion: 0.18
Nodes (10): Changelog, CLI, Context & sessions, External tools & the skills ecosystem, MCP, OAuth (subscription login), Providers & protocols, Tools & permissions (+2 more)

### Community 62 - "§14 Ship gate: one unattended session on opencode-go (read→search→edit→test→commit→rtk skill)"
Cohesion: 0.20
Nodes (11): graphify install --platform moca upstream PR (Graphify-Labs/graphify#4174), v0.1.0 rtk/graphify allowlist, Agent-Skills compatibility, vendored corpus, §9.5 Project instructions (AGENTS.md): global then workdir, 32K cap, §8 Sessions: JSONL append-only, 0600, entry ids/parentId chain, resume/--continue, §14 Ship gate: one unattended session on opencode-go (read→search→edit→test→commit→rtk skill), §9 Skills: three sources (project/global/built-in), read-based body load, §13 System prompt: built once per session, stored, never rebuilt (cache discipline), Still-open items: live login round-trip,_DIRECT vendor endpoints, per-process session header, upstream PR merge (+3 more)

### Community 63 - "teaapi_test.go"
Cohesion: 0.20
Nodes (3): newPager(), TestTeaAPI(), pagerModel

### Community 64 - "ToolSpec"
Cohesion: 0.22
Nodes (6): ToolSpec, TestProxySpecFrozen(), MCPSpec(), echoTool, lsTool, writeTool

### Community 65 - "§12 Config: JSONC chosen over TOML; string-literal-aware pre-pass; house dotfiles merge rationale"
Cohesion: 0.22
Nodes (9): §12 Config: JSONC chosen over TOML; string-literal-aware pre-pass; house dotfiles merge rationale, MCP call gating: annotation-based default-deny + per-server approve, moca mcp import: reads Claude Code/OpenCode/Pi configs, secrets rewritten to env: refs, §10.5 Lazy MCP proxy (rev 3): ~200-token tool, no schemas in prompt, lazy lifecycle, persisted index, rev 3: MCP in v1, lazy by design (pi-mcp-adapter measured results rationale), §4 Config: strict decode, unknown keys rejected, line:col errors, config.AppendString / SetObjectEntry / SetString: comment-preserving JSONC edits (.bak + atomic rename), §4 Config keys (camelCase): providers, shell.allow, mcp, snapshot, context, yolo + defaults (+1 more)

### Community 66 - "loginState"
Cohesion: 0.22
Nodes (4): loginChoice, loginState, loginStep, loginWriter

### Community 67 - "format.go"
Cohesion: 0.31
Nodes (9): AbbrevEffort(), AbbrevHome(), FmtTokens(), FmtWindow(), GitBranch(), gitStatusArgs(), TestFormatters(), TestGitBranch() (+1 more)

### Community 68 - "drained"
Cohesion: 0.47
Nodes (9): drained(), newLoginModel(), TestLoginMarkedBackAndCancel(), TestLoginOAuthDeclinedFlipAndCancel(), TestLoginOAuthFlowAndAuthFlip(), TestLoginRefusedWhileRunning(), TestLoginStoresAPIKey(), TestLogoutClearsStoredCredential() (+1 more)

### Community 69 - "v0.1.0 seven frozen tools: read/write/edit/shell/search/ls/mcp"
Cohesion: 0.25
Nodes (9): v0.1.0 failing-test investigation protocol (edit refuses until read+search done), v0.1.0 lazy MCP proxy: stdio+HTTP, persisted index, ~200-token tool, mcp import, v0.1.0 permissions: path jail, mvdan.cc/sh analysis, trust.json, yolo mode, v0.1.0 seven frozen tools: read/write/edit/shell/search/ls/mcp, v0.1.0 pre-edit snapshots + /undo (skipped for clean tracked files), Investigation protocol (SPECS §10): Env.FailingTest/Searched gates drive per-tool banners and edit refusal, ReadTracker: mtime/size/sha256 recorded; *_test.go read sets Env.TestSeen, §10 Tools (seven, frozen): sequential execution, ReadTracker, golden schemas (+1 more)

### Community 70 - "DataDir"
Cohesion: 0.33
Nodes (9): main(), TestPathsHonourXDG(), AuthFile(), ConfigDir(), ConfigFile(), DataDir(), home(), MigrateLegacyStore() (+1 more)

### Community 71 - "httpTransport"
Cohesion: 0.33
Nodes (3): fakeSum, readEvents(), httpTransport

### Community 73 - "dropdown_test.go"
Cohesion: 0.31
Nodes (8): contains(), count(), dropNames(), TestDropdownEscDismissesUntilTheWordChanges(), TestDropdownHiddenWhenEnterWouldBeRefused(), TestDropdownKeysCompleteAndSend(), TestDropdownListsCommandsAndPrompts(), TestDropdownRendersCursorAndLegend()

### Community 74 - "NewUnjailed"
Cohesion: 0.29
Nodes (4): Agent, NewUnjailed(), TestUnjailedAndAllowAll(), Unjailed

### Community 75 - "AGENTS.md — moca"
Cohesion: 0.25
Nodes (7): AGENTS.md — moca, Commands, Conventions & gotchas, Docs, graphify, Ground truth (read before changing behavior), Layout & dependency rules

### Community 76 - "§4 Seven tools frozen for v1; schema churn rule (frozen = v2 discussion)"
Cohesion: 0.32
Nodes (7): §5 Edit-tool contract: uniqueness, re-indentation, CRLF/BOM, model-facing errors, Tool edit: exact-first with whitespace-tolerant fallback, replace_all, Tool ls: one level, dirs suffixed /, hidden opt-in, Tool read: 2000 lines/50K caps, 2000-char line cap, binary refusal, Tool search: pure-Go ripgrep semantics, .gitignore-aware, RE2, 200-hit cap, Tool shell: stateless bash -c/pwsh, stdin null, process-group kill, Tool write: new files allowed; existing needs prior read + unchanged on disk

### Community 77 - "transform_test.go"
Cohesion: 0.39
Nodes (7): NormalizeToolID(), TestNormalizeToolID(), TestTransformDropsEmptiedAssistantAndMerges(), TestTransformDropsRedactedForeignThinking(), TestTransformHistory(), TestTransformSameBareIDDifferentProvider(), TransformHistory()

### Community 78 - "TestLoginCLISwitchesAuthAfterCallback"
Cohesion: 0.38
Nodes (5): authURLFrom(), TestLoginCLISwitchesAuthAfterCallback(), TestLoginTokenEndpointIsBounded(), waitFor(), lockedBuf

### Community 80 - "startCatalog"
Cohesion: 0.38
Nodes (6): catalogConfig(), lastModelChange(), startCatalog(), TestResumeEffortFlagOverridesStored(), TestResumeRestoresHardMode(), TestResumeRestoresModelAndEffortFromModelChange()

### Community 81 - "serverEnv"
Cohesion: 0.33
Nodes (7): ResolveEnv(), TestKeepNameWindows(), FilterEnv(), keepName(), serverEnv(), TestFilterEnv(), TestServerEnvExplicitEntryReplacesInherited()

### Community 82 - "rtk token-compressed CLI proxy; analyser unwraps rtk <cmd>"
Cohesion: 0.33
Nodes (5): §10 External tools & extensibility: skills + slash commands + allowlist (no plugins), rtk token-compressed CLI proxy; analyser unwraps rtk <cmd>, Shell command analysis via mvdan.cc/sh: every simple command, wrappers, redirects, hard-deny set, rm ask-every-time, §11 Permissions: canonicalized jail, hard-deny/refused/builtin classes, case-insensitive matching, Windows best-effort, trust store, yolo checkers, rtk unwrap ladder (phase 6): runners, proxies, config-mutating subcommands; rtk.go capture

### Community 83 - "parseIgnore"
Cohesion: 0.40
Nodes (5): globToRegexp(), ignored(), parseIgnore(), TestIgnore(), ignoreRule

### Community 84 - "ReadTracker"
Cohesion: 0.47
Nodes (4): NewReadTracker(), stampOf(), ReadTracker, stamp

### Community 85 - "CI Workflow (gofmt · vet · build · test)"
Cohesion: 0.40
Nodes (4): CI Workflow (gofmt · vet · build · test), CI gate set (gofmt, go vet, go build, go test -race), go.mod (Go version source for setup-go), SPECS.md §2 — CI gate definition

### Community 86 - "CostConfig"
Cohesion: 0.40
Nodes (5): ModelOverride, ProviderConfig, CostConfig, costM(), TestCostOf()

### Community 87 - "§11 TUI: inline immutable scrollback, pager, chips, multi-line paste, steering"
Cohesion: 0.50
Nodes (5): §11 TUI: inline immutable scrollback, pager, chips, multi-line paste, steering, §3.5 TUI: layout, bands, status bar, keys, pager, chips, runs, commands, !cmd / !!cmd shell prefixes from the input box, README slash commands list + prompt templates + // escape, README TUI keys: enter, shift+enter, esc, ctrl+o, alt+t, alt+p, a/ctrl+a/d, dropdown keys

### Community 88 - "§14 Agent loop: turn contract, control surface, steering queue, length-stop split-work errors, event list"
Cohesion: 0.40
Nodes (5): §14 Agent loop: turn contract, control surface, steering queue, length-stop split-work errors, event list, Context estimate anchor: anchor = usage tokens per turn, invalidated on model change/compaction, agent.Resume: validation-first, repair, restore model/effort/snapshots/totals, shared prepare()/build(), Single-writer flock per session file; concurrent open refuses with exit 1, §13 Sessions & snapshots: transcript types, Find/FindForWorkdir, rebuild, repair, snapshot store/undo/prune

### Community 89 - "Messages"
Cohesion: 0.60
Nodes (4): LatestCompaction(), Messages(), TestMessagesFromCompaction(), TestMessagesRebuild()

### Community 90 - "TurnEnd"
Cohesion: 0.50
Nodes (3): TurnEnd, StopReason, anthropicStop()

### Community 91 - "rev 13: TUI polish — two-line status bar, rules, full-row bands, /exit"
Cohesion: 0.50
Nodes (4): Two-line status bar: version·cwd·branch / provider/model·effort·ctx·in/out·cost, rev 13: TUI polish — two-line status bar, rules, full-row bands, /exit, TUI activity row / approvals rendering with 700ms typing pause, Sanitize: C0/C1 control rendering before terminal output

### Community 92 - "Claude Code frontend-design skill"
Cohesion: 0.50
Nodes (3): AI-generated design default tells, frontend-design Apache 2.0 License, Claude Code frontend-design skill

### Community 93 - "pi mcp-scripting skill"
Cohesion: 0.67
Nodes (4): jev.evaluate reference, jev.evaluate semantic scripting gate, mcpScript tools API, pi mcp-scripting skill

### Community 96 - "v0.1.0 CLI surface: moca, -p, login/logout, mcp import/index, exit codes 0/1/2/3/130"
Cohesion: 0.67
Nodes (3): v0.1.0 CLI surface: moca, -p, login/logout, mcp import/index, exit codes 0/1/2/3/130, v0.1.0 three protocol adapters, hand-rolled HTTP+SSE, no SDKs, v0.1.0 TUI: status bar, bracketed paste, steering, approvals, slash commands

### Community 97 - "v0.1.1 / autocomplete dropdown (built-ins + templates, tab complete)"
Cohesion: 0.67
Nodes (3): v0.1.1 / autocomplete dropdown (built-ins + templates, tab complete), Prompt templates: /name with $ARGUMENTS/$@/$1..$9/$$; Claude Code + pi compatible, TUI /-dropdown autocomplete: prefix-filter, 8-row window, tab/enter/esc semantics

### Community 98 - "findTestFile"
Cohesion: 0.67
Nodes (3): findTestFile(), isPathByte(), TestFindTestFile()

### Community 102 - "Skills: SKILL.md in ~/.config/moca/skills or <repo>/.moca/skills; cross-tool compatibility"
Cohesion: 0.67
Nodes (3): External tools: rtk + graphify allowlisted with built-in skills, Project instructions: AGENTS.md / CLAUDE.md, trusted projects only, Skills: SKILL.md in ~/.config/moca/skills or <repo>/.moca/skills; cross-tool compatibility

## Ambiguous Edges - Review These
- `v0.1.0 OpenAI subscription OAuth: Sign in with ChatGPT (PKCE S256, dynamic registration, loopback, JWKS, rotating refresh tokens)` → `v0.1.0 permissions: path jail, mvdan.cc/sh analysis, trust.json, yolo mode`  [AMBIGUOUS]
  CHANGELOG.md · relation: conceptually_related_to

## Knowledge Gaps
- **132 isolated node(s):** `claudeServer`, `opencodeServer`, `request`, `usageError`, `completionsChunk` (+127 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 308 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **43 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **What is the exact relationship between `v0.1.0 OpenAI subscription OAuth: Sign in with ChatGPT (PKCE S256, dynamic registration, loopback, JWKS, rotating refresh tokens)` and `v0.1.0 permissions: path jail, mvdan.cc/sh analysis, trust.json, yolo mode`?**
  _Edge tagged AMBIGUOUS (relation: conceptually_related_to) - confidence is low._
- **Why does `model` connect `model` to `loginState`, `newTestModel`, `app.go`, `time.Duration`, `fixes_test.go`, `Input`, `printed`, `.Update`, `charm.land/bubbletea/v2.Cmd`?**
  _High betweenness centrality (0.039) - this node is a cross-community bridge._
- **What connects `claudeServer`, `opencodeServer`, `request` to the rest of the system?**
  _132 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `go_pkg_strings` be split into smaller, more focused modules?**
  _Cohesion score 0.06976744186046512 - nodes in this community are weakly interconnected._
- **Why does `Input` connect `Input` to `input_test.go`?**
  _High betweenness centrality (0.023) - this node is a cross-community bridge._
- **Should `oauth_test.go` be split into smaller, more focused modules?**
  _Cohesion score 0.057813911472448055 - nodes in this community are weakly interconnected._
- **Why does `Agent` connect `Agent` to `go_pkg_strings`, `Model`, `tools/tool.go`, `Request`, `anthropic_test.go`, `session/entry.go`, `sync.Mutex`?**
  _High betweenness centrality (0.016) - this node is a cross-community bridge._