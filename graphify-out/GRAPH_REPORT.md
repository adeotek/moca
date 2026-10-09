# Graph Report - moca  (2026-10-09)

## Corpus Check
- 238 files · ~194,797 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 10 file(s) not represented in the graph (top: (none) 5, .jsonc 3, .v2 1)

## Summary
- 2156 nodes · 8688 edges · 113 communities (86 shown, 27 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 1051 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `1e082103`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- start.go
- newFakeAS
- testing.T
- newScript
- context.Context
- retry_test.go
- Create
- Sanitize
- NewManager
- skills_test.go
- ToolSpec
- v0.1.1 credentials move to ~/.config/moca/auth.json (0600)
- newTestModel
- Request
- permissions/shell.go
- run
- Input
- cut_test.go
- anthropic_test.go
- P2 — readability & feedback polish
- NewDefaultStore
- Parse
- model
- Run
- tui.go
- sync.Mutex
- runLogin
- MCPServer
- LoadPrompts
- Agent
- .Run
- Manager
- jail.go
- Registry
- External tools: rtk, graphify, skills ecosystem
- .DiscoverOllama
- Registry
- renderMarkdown
- moca — one Go binary TUI coding agent
- 2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)
- initialize
- post
- Message
- fixes_test.go
- estimate.go
- SOURCES.md
- Claude Code skill-creator skill
- time.Duration
- model
- Model
- runOneShot
- Config
- RenderStatus
- fixture/main.go
- §6 Context manager: reserveTokens 16K, keepRecent 20K, token-denominated budgets
- moca README — MO Coding Agent
- Usage
- Design purpose: tight agent loop + context discipline beats feature richness
- styleBlock
- input_test.go
- ollamaStart
- v0.1.0-alpha (2026-10-07)
- §9 Skills: three sources (project/global/built-in), read-based body load
- go_pkg_charm_land_lipgloss_v2
- Item
- §12 Config: JSONC chosen over TOML; string-literal-aware pre-pass; house dotfiles merge rationale
- DataDir
- format_test.go
- drained
- §4 Seven tools frozen for v1; schema churn rule (frozen = v2 discussion)
- Start
- LineReader
- quick_validate.py
- trust_test.go
- probe/main.go
- AGENTS.md — moca
- TestModelPickerRefreshesOllama
- hints.go
- ReadTracker
- .View
- CostConfig
- StartError
- rtk token-compressed CLI proxy; analyser unwraps rtk <cmd>
- TestWelcomeLines
- CI Workflow (gofmt · vet · build · test)
- §11 TUI: inline immutable scrollback, pager, chips, multi-line paste, steering
- rev 13: TUI polish — two-line status bar, rules, full-row bands, /exit
- Claude Code frontend-design skill
- pi mcp-scripting skill
- allowAll
- v0.1.0 CLI surface: moca, -p, login/logout, mcp import/index, exit codes 0/1/2/3/130
- v0.1.1 / autocomplete dropdown (built-ins + templates, tab complete)
- Skills: SKILL.md in ~/.config/moca/skills or <repo>/.moca/skills; cross-tool compatibility
- run.sh
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
1. `model` - 72 edges
2. `newTestModel()` - 61 edges
3. `newScript()` - 46 edges
4. `run()` - 44 edges
5. `key()` - 44 edges
6. `sseServer()` - 36 edges
7. `startTestWith()` - 35 edges
8. `Config` - 34 edges
9. `Parse()` - 34 edges
10. `newAgentModel()` - 32 edges

## Surprising Connections (you probably didn't know these)
- `moca README — MO Coding Agent` --semantically_similar_to--> `§1 Identity: binary = repo = module github.com/adeotek/moca`  [INFERRED] [semantically similar]
  README.md → docs/specs/DESIGN.md
- `OAuth policy gate: vendor terms must permit third-party use; unclear = not permitted` --semantically_similar_to--> `v0.1.0 anthropic API-key-only policy (phase-7 gate, oauth-verification.md)`  [INFERRED] [semantically similar]
  docs/specs/SPECS.md → CHANGELOG.md
- `SIWC login flow: dynamic registration, PKCE S256, loopback callback, JWKS ID-token validation, RS256, entropy checks` --semantically_similar_to--> `v0.1.0 OpenAI subscription OAuth: Sign in with ChatGPT (PKCE S256, dynamic registration, loopback, JWKS, rotating refresh tokens)`  [INFERRED] [semantically similar]
  docs/specs/SPECS.md → CHANGELOG.md
- `rev 14: credential store in config dir + /login wizard + / dropdown` --semantically_similar_to--> `v0.1.1 — credential store, /login, / autocomplete, bigger catalog`  [INFERRED] [semantically similar]
  docs/specs/DESIGN.md → CHANGELOG.md
- `TUI /login credential wizard: provider picker, masked entry, OAuth flow bridged by event pipe, auth-flip offer` --semantically_similar_to--> `v0.1.1 TUI /login and /logout commands`  [INFERRED] [semantically similar]
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

## Communities (113 total, 27 thin omitted)

### Community 0 - "start.go"
Cohesion: 0.13
Nodes (21): Compacted, Resumed, SteeringApplied, EnvError, claudeServer, opencodeServer, request, usageError (+13 more)

### Community 1 - "newFakeAS"
Cohesion: 0.06
Nodes (45): fakeSIWC(), fakeSIWCOpts(), Store, newHostID(), validateAPIKey(), audContains(), b64d(), OAuthConfig (+37 more)

### Community 2 - "testing.T"
Cohesion: 0.05
Nodes (70): TestResumeNotASession(), TestEffortRankOrdered(), TestUsageAdd(), TestCallToolNullArgsSentAsEmptyObject(), TestHelperProcess(), NewStore(), TestAPIKeyStore(), TestConcurrentRefreshOnce() (+62 more)

### Community 3 - "newScript"
Cohesion: 0.08
Nodes (69): scriptServer, newScript(), startTest(), startTestWith(), startWith(), TestAbortWritesSyntheticResults(), TestFailedCallDoesNotCancelRest(), TestLengthStopTruncatedToolCall() (+61 more)

### Community 4 - "context.Context"
Cohesion: 0.08
Nodes (23): CallResult, errResult(), lockFile(), lockFile(), approve(), TestInvestigationHint(), TestInvestigationRefusal(), decode() (+15 more)

### Community 5 - "retry_test.go"
Cohesion: 0.29
Nodes (16): Retry, DefaultRetryPolicy(), RetryNotice, fail(), TestMidStreamFailureResetsAndRetriesOnce(), TestMidStreamRetryBacksOffAndNotifies(), TestNoRetryOn400(), testPolicy() (+8 more)

### Community 6 - "Create"
Cohesion: 0.08
Nodes (25): TestFind(), TestFindForWorkdirViaSymlink(), TestFindMtimeTieDeterministic(), lockFile(), lockFile(), gitClean(), Snapshots, NewSnapshots() (+17 more)

### Community 7 - "Sanitize"
Cohesion: 0.08
Nodes (20): AutoAllow(), Answer, Question, approvalPanel(), approvalPrompt(), TestApprovalPromptSanitized(), diffPreview(), plural() (+12 more)

### Community 8 - "NewManager"
Cohesion: 0.13
Nodes (36): build(), crashingServer(), shServer(), skipWindows(), TestCallTimeoutStopsWedgedServer(), TestHandshakeTimeout(), TestHTTPAbortedStreamReplayRule(), TestIndexSaveMergesConcurrentWriters() (+28 more)

### Community 9 - "skills_test.go"
Cohesion: 0.23
Nodes (10): builtinHash(), ExtractBuiltins(), extractBuiltins(), TestEcosystemSkillsLoadUnchanged(), Discover(), mkSkill(), TestDiscoverPrecedence(), TestDiscoverReportsSymlinkedDir() (+2 more)

### Community 10 - "ToolSpec"
Cohesion: 0.12
Nodes (10): ToolSpec, TestProxySpecFrozen(), MCPSpec(), echoTool, editTool, lsTool, readTool, searchTool (+2 more)

### Community 11 - "v0.1.1 credentials move to ~/.config/moca/auth.json (0600)"
Cohesion: 0.06
Nodes (30): v0.1.0 anthropic API-key-only policy (phase-7 gate, oauth-verification.md), v0.1.0 day-1 providers: anthropic, opencode-go, openai + custom baseUrl providers, v0.1.0 OpenAI subscription OAuth: Sign in with ChatGPT (PKCE S256, dynamic registration, loopback, JWKS, rotating refresh tokens), v0.1.1 — credential store, /login, / autocomplete, bigger catalog, v0.1.1 credentials move to ~/.config/moca/auth.json (0600), moca login stores API keys (--api-key, silent pipe input); moca logout, v0.1.1 TUI /login and /logout commands, Effort as first-class request parameter (off..max), clamped per model (+22 more)

### Community 12 - "newTestModel"
Cohesion: 0.04
Nodes (86): TestActivityLine(), TestCoalescedShrinkRemembersTheLastFlushedRow(), TestInputBoxGrowsAndShrinks(), TestPagerCloseDefersTheRelease(), TestRefuseRunningWhileOutputInFlight(), TestShrinkIsPaddedUntilTheGuardWindowPasses(), TestShrinkPaddingFitsTheScreen(), TestSpinnerTicksOnlyForTheLiveRun() (+78 more)

### Community 13 - "Request"
Cohesion: 0.16
Nodes (15): Event, Request, anthropicOverflow(), openaiOverflow(), priorReasoningText(), replaysVerbatim(), toolInput(), ok() (+7 more)

### Community 14 - "permissions/shell.go"
Cohesion: 0.19
Nodes (15): rtkRunTarget(), rtkShellString(), rtkSub(), rtkTarget(), Shell, literalSafe(), numericFlag(), patternSafe() (+7 more)

### Community 15 - "run"
Cohesion: 0.08
Nodes (49): fakeCompletions(), isolate(), TestExitCodes(), TestHelpListsSurface(), TestInterruptedExit130(), TestLoginLogoutCLI(), TestMCPImportCLI(), TestNoApproveSkipsProjectInstructions() (+41 more)

### Community 16 - "Input"
Cohesion: 0.06
Nodes (27): canonical(), Find(), FindForWorkdir(), Info, header(), headerAndPreview(), idOf(), List() (+19 more)

### Community 17 - "cut_test.go"
Cohesion: 0.09
Nodes (33): Agent, summarizerUnavailable(), Budget, NewBudget(), TestBudget(), FindCut(), a(), call() (+25 more)

### Community 18 - "anthropic_test.go"
Cohesion: 0.17
Nodes (36): Adapter, newAnthropic(), keyCred(), mustJSON(), sseServer(), TestAnthropicBodyHygiene(), TestAnthropicBudgetAndNoCache(), TestAnthropicBudgetFollowsEffort() (+28 more)

### Community 19 - "P2 — readability & feedback polish"
Cohesion: 0.07
Nodes (26): 0. Fixed in this pass (already on `fix/polish`), 1.1 Markdown-lite rendering of responses — **M**, 1.2 Inline diff preview for `edit`/`write` items — **S/M**, 1.3 Context-pressure color in the status bar — **S**, 1.4 Argument completion in the `/` dropdown + interactive `/model` — **M**, 1.5 `@path` file mentions — **M**, 1.6 Resume shows where you left off; `/resume` picker — **M**, 1.7 Persistent input history + `ctrl+r` search — **S/M** (+18 more)

### Community 20 - "NewDefaultStore"
Cohesion: 0.11
Nodes (12): NewDefaultStore(), OAuthProvider(), model, providerNames(), providerRow(), TestLoginRowForOllama(), loginChoice, loginDoneMsg (+4 more)

### Community 21 - "Parse"
Cohesion: 0.07
Nodes (41): scanner, span, Parse(), TestAPIKeyOptional(), TestDefaultsApplied(), TestEnvRefs(), TestOAuthAllowedForOpenAI(), TestOAuthRejectedForNonOAuthProvider() (+33 more)

### Community 22 - "model"
Cohesion: 0.13
Nodes (3): model, AbbrevEffort(), compactDoneMsg

### Community 23 - "Run"
Cohesion: 0.12
Nodes (15): Event, Asker, newModel(), Run(), newAsker(), newEventPipe(), TestEventPipeBuffersUntilStarted(), TestEventPipeNonBlockingAndFIFO() (+7 more)

### Community 24 - "tui.go"
Cohesion: 0.23
Nodes (8): hashFile(), TestEcosystemSourcesHashes(), closingQuote(), ParseFrontmatter(), scalar(), TestFrontmatter(), TestFrontmatterBlockAndQuoteEdges(), TestFrontmatterBlockIndicatorsAndComments()

### Community 25 - "sync.Mutex"
Cohesion: 0.12
Nodes (10): responseID(), TestResponseID(), callResult, startStdio(), TestStdioWriteEPIPE(), httpTransport, response, ring (+2 more)

### Community 26 - "runLogin"
Cohesion: 0.21
Nodes (14): parseArgs(), printUsage(), TestParseArgs(), runLogin(), runLogout(), yesNo(), run(), confirm() (+6 more)

### Community 27 - "MCPServer"
Cohesion: 0.15
Nodes (18): MCPServer, TestImportRewritesEmbeddedSecretsInValues(), TestRewriteSecretsValueRuleAlone(), DiscoverSources(), embeddedSecret(), envName(), literalSecretFields(), ParseSource() (+10 more)

### Community 28 - "LoadPrompts"
Cohesion: 0.19
Nodes (13): ExpandPrompt(), Prompt, LoadPrompts(), TestEcosystemPromptsLoad(), TestExpandPrompt(), TestLoadPrompts(), HelpText(), ParseInput() (+5 more)

### Community 29 - "Agent"
Cohesion: 0.14
Nodes (9): Options, savedModel, Agent, Outcome, New(), Response, ToolChoice, CommandChecker (+1 more)

### Community 30 - ".Run"
Cohesion: 0.21
Nodes (6): isTrue(), Annotations, Index, IndexEntry, IndexTool, ProxyTool

### Community 31 - "Manager"
Cohesion: 0.20
Nodes (8): ConfigHash(), Rank(), TestIndexPersistAndHash(), TestRank(), words(), Manager, Hit, state

### Community 32 - "jail.go"
Cohesion: 0.14
Nodes (19): setup, Jail, NewJail(), resolveDeep(), setup(), TestJail(), TestJailDanglingSymlinks(), TestJailKeepsNotYetExistingReadOnlyRoot() (+11 more)

### Community 33 - "Registry"
Cohesion: 0.18
Nodes (8): TestDefaultHTTPClientBoundsHeaderWait(), defaultHTTPClient(), discoverLocal(), StartOptions, prepare(), SplitModel(), TestSplitModel(), Registry

### Community 34 - "External tools: rtk, graphify, skills ecosystem"
Cohesion: 0.32
Nodes (5): External tools: rtk, graphify, skills ecosystem, internal/permissions/rtk.go — captured rtk subcommand classes, rtk — token-compressed CLI proxy, Cross-tool skills and prompt templates ecosystem, Built-in rtk skill (SKILL.md)

### Community 35 - ".DiscoverOllama"
Cohesion: 0.16
Nodes (10): Registry, isDialFailure(), normalizeOllamaURL(), ollamaBaseURL(), ollamaRoot(), ollamaUnreachable(), TestNormalizeOllamaURL(), TestOllamaBaseURLPrecedence() (+2 more)

### Community 36 - "Registry"
Cohesion: 0.21
Nodes (7): Builtins(), runReg(), TestRegistryInvestigationBanner(), Registry, NewRegistry(), TestSchemasFrozen(), Tool

### Community 37 - "renderMarkdown"
Cohesion: 0.23
Nodes (12): codeBandStyle(), fenceLang(), renderInline(), renderMarkdown(), plainMD(), TestMarkdownBlocks(), TestMarkdownFence(), TestMarkdownSanitizes() (+4 more)

### Community 38 - "moca — one Go binary TUI coding agent"
Cohesion: 0.22
Nodes (7): CI gate (gofmt + vet + build + race test), docs/specs/DESIGN.md — v1 contract/vision, docs/specs/SPECS.md — living normative spec, graphify query/path/explain workflow rules, moca — one Go binary TUI coding agent, Ship gate test/shipgate/run.sh, graphify — codebase knowledge graph integration

### Community 39 - "2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)"
Cohesion: 0.12
Nodes (13): apiKey must be an env:VAR reference, 1.1 Terms, 1.2 Client registration, 1.3 Decision, 1. anthropic — decision: `api_key only` (subscription OAuth not permitted), 2.1 Terms, 2.2 Client registration (per the docs), 2.3 Endpoints (verified live 2026-10-06 against `https://auth.openai.com/.well-known/openid-configuration`) (+5 more)

### Community 40 - "initialize"
Cohesion: 0.11
Nodes (18): TestResolveEnv(), ResolveEnv(), initialize(), TestHTTPCloseBounded(), TestListToolsRepeatedCursor(), startHTTP(), TestHTTPNotifyStatusChecked(), FilterEnv() (+10 more)

### Community 41 - "post"
Cohesion: 0.11
Nodes (13): abortingHTTPServer(), fakeHTTPServer(), TestHTTPTransport(), post(), newHTTPError(), retryAfter(), modelFromShow(), TestRetryAfterFormats() (+5 more)

### Community 42 - "Message"
Cohesion: 0.13
Nodes (17): ToolEnd, ToolStart, Kind, Entry, ContentBlock, Message, ToolCall, ToolResult (+9 more)

### Community 43 - "fixes_test.go"
Cohesion: 0.07
Nodes (73): StreamReset, TextDelta, ThinkingDelta, Warning, YoloChanged, NormalizeToolID(), TestNormalizeToolID(), TestTransformDropsEmptiedAssistantAndMerges() (+65 more)

### Community 44 - "estimate.go"
Cohesion: 0.36
Nodes (6): EntryTokens(), MessageChars(), RequestChars(), TestEstimate(), Tokens(), UsageTokens()

### Community 45 - "SOURCES.md"
Cohesion: 0.19
Nodes (7): internal/skills/testdata/ecosystem corpus, anthropics/skills frontend-design, anthropics/skills skill-creator, graphify skill (graphify install, claude platform; byte-identical to pi variant), adeotek/dotfiles opencode dotnet-unit-testing skill, npm pi-mcp-adapter mcp-scripting skill, npm pi-subagents skill

### Community 46 - "Claude Code skill-creator skill"
Cohesion: 0.16
Nodes (13): Skill description (trigger) optimization, Skill eval/iterate loop (with-skill vs baseline), skill-creator Apache 2.0 License (Anthropic), Claude Code skill-creator skill, SKILL.md anatomy + bundled resources layout, Constrained query expansion against graph vocab, Progressive Disclosure (three-level skill loading), graphify query/path/explain reference (+5 more)

### Community 47 - "time.Duration"
Cohesion: 0.24
Nodes (6): TestFmtElapsed(), fmtElapsed(), fmtThinkDuration(), Options, HTTPError, retrying

### Community 48 - "model"
Cohesion: 0.13
Nodes (15): firstLineOf(), clampRunes(), model, matchCommands(), TestClampRunes(), model, mentionMatches(), mentionQuery() (+7 more)

### Community 49 - "Model"
Cohesion: 0.10
Nodes (7): Status, Agent, Effort, ParseEffort(), TestParseEffort(), Model, supportedEfforts()

### Community 50 - "runOneShot"
Cohesion: 0.40
Nodes (5): TestExitForOAuthSentinels(), exitFor(), firstLine(), resolveResume(), runOneShot()

### Community 51 - "Config"
Cohesion: 0.14
Nodes (12): ContextConfig, MCPConfig, ShellConfig, SnapshotConfig, TUIConfig, Default(), Config, ProviderConfig (+4 more)

### Community 52 - "RenderStatus"
Cohesion: 0.13
Nodes (18): TestStatusWideRunesNeverExceedWidth(), AbbrevHome(), FmtTokens(), FmtWindow(), TestFormatters(), trimZero(), FmtPercent(), renderLine() (+10 more)

### Community 53 - "fixture/main.go"
Cohesion: 0.21
Nodes (7): Sum(), TestTotals(), ParseList(), Average(), Max(), TestMaxAndAverage(), main()

### Community 54 - "§6 Context manager: reserveTokens 16K, keepRecent 20K, token-denominated budgets"
Cohesion: 0.18
Nodes (11): Cache breakpoints: cache_control placement, ≤4, rolled forward, §6 Context manager: reserveTokens 16K, keepRecent 20K, token-denominated budgets, maxSteps 40: runaway guard, wrap-up via tool_choice none, Overflow recovery: one compact-and-retry; length-with-room ≠ overflow, Path jail anchored at session-start workdir, symlink-resolved; read-only skill roots, Project trust: --approve/--no-approve, trust.json; cloned repo prompt injection rationale, rev 7: compaction rewritten token-denominated; session schema; trust; steering, Structured summary format: Goal/Constraints/Progress/Key Decisions/Next Steps + cumulative file lists (+3 more)

### Community 55 - "moca README — MO Coding Agent"
Cohesion: 0.12
Nodes (17): §12.5 CLI surface: moca, -p flags, login/logout, mcp import/index, --version, §1 Identity: binary = repo = module github.com/adeotek/moca, DESIGN.md — v1 contract (rev 14), §15 Explicit non-goals v1: no plugins/sandboxing/MCP OAuth etc., Phase plan 1–7 (each = one PR) with per-phase gates, §3 CLI surface as implemented (--config, --resume rules, login/logout detail, --help), §3 Exit codes 0/1/2/3/130 mapped per failure class (incl. OAuth invalid_grant → 2), §16 Maintenance: SPECS updated at end of every phase/behavior PR (+9 more)

### Community 56 - "Usage"
Cohesion: 0.17
Nodes (7): TurnEnd, fakeSum, Result, StopReason, Usage, readEvents(), anthropicStop()

### Community 57 - "Design purpose: tight agent loop + context discipline beats feature richness"
Cohesion: 0.29
Nodes (4): §2 Architecture: internal packages (llm, agent, provider, tools, compact, session, tui, config, skills, permissions, mcp), §2 Build/test/verify: make targets, version stamping, CI gate, shipgate build tag, §2 Package layout (implemented) + dependency directions incl. UI deps, Token-efficiency levers: prompt caching, windowed reads, diff-shaped results, token compaction

### Community 58 - "styleBlock"
Cohesion: 0.31
Nodes (10): respMsgStyle(), styleBlock(), styleBlockLead(), TestCommandEchoesAsUserMessage(), TestMessageBackgroundsCoverTheFullRow(), TestResponseCommitUsesBackground(), TestScrollbackMessageBackgrounds(), TestStyleBlockExpandsTabs() (+2 more)

### Community 59 - "input_test.go"
Cohesion: 0.30
Nodes (11): NewInput(), TestCollapseUsesExactOffset(), TestCtrlC(), TestEditedMarkerSentLiterally(), TestHistory(), TestHistoryRecallChipsUnsafeContent(), TestMarkerNeverCollidesWithTypedText(), TestPasteChipKeepsContent() (+3 more)

### Community 60 - "ollamaStart"
Cohesion: 0.36
Nodes (6): ollamaServer(), ollamaStart(), TestOllamaReportedWindowIsQuiet(), TestOllamaStartErrors(), TestOllamaTinyWindowWarns(), TestStartWithOllamaModel()

### Community 61 - "v0.1.0-alpha (2026-10-07)"
Cohesion: 0.18
Nodes (10): Changelog, CLI, Context & sessions, External tools & the skills ecosystem, MCP, OAuth (subscription login), Providers & protocols, Tools & permissions (+2 more)

### Community 62 - "§9 Skills: three sources (project/global/built-in), read-based body load"
Cohesion: 0.18
Nodes (12): graphify install --platform moca upstream PR (Graphify-Labs/graphify#4174), v0.1.0 rtk/graphify allowlist, Agent-Skills compatibility, vendored corpus, §9.5 Project instructions (AGENTS.md): global then workdir, 32K cap, §8 Sessions: JSONL append-only, 0600, entry ids/parentId chain, resume/--continue, §9 Skills: three sources (project/global/built-in), read-based body load, §13 System prompt: built once per session, stored, never rebuilt (cache discipline), §14 Agent loop: turn contract, control surface, steering queue, length-stop split-work errors, event list, Context estimate anchor: anchor = usage tokens per turn, invalidated on model change/compaction (+4 more)

### Community 63 - "go_pkg_charm_land_lipgloss_v2"
Cohesion: 0.16
Nodes (5): TestMatchRanges(), matchRanges(), newPager(), TestTeaAPI(), pagerModel

### Community 65 - "§12 Config: JSONC chosen over TOML; string-literal-aware pre-pass; house dotfiles merge rationale"
Cohesion: 0.22
Nodes (9): §12 Config: JSONC chosen over TOML; string-literal-aware pre-pass; house dotfiles merge rationale, MCP call gating: annotation-based default-deny + per-server approve, moca mcp import: reads Claude Code/OpenCode/Pi configs, secrets rewritten to env: refs, §10.5 Lazy MCP proxy (rev 3): ~200-token tool, no schemas in prompt, lazy lifecycle, persisted index, rev 3: MCP in v1, lazy by design (pi-mcp-adapter measured results rationale), §4 Config: strict decode, unknown keys rejected, line:col errors, config.AppendString / SetObjectEntry / SetString: comment-preserving JSONC edits (.bak + atomic rename), §4 Config keys (camelCase): providers, shell.allow, mcp, snapshot, context, yolo + defaults (+1 more)

### Community 66 - "DataDir"
Cohesion: 0.22
Nodes (13): main(), runMCPIndex(), decideTrust(), hasProjectResources(), runTUI(), TestPathsHonourXDG(), AuthFile(), ConfigDir() (+5 more)

### Community 67 - "format_test.go"
Cohesion: 0.40
Nodes (5): TestGitStatusUsesNoOptionalLocks(), GitBranch(), gitStatusArgs(), TestGitBranch(), TestGitBranchUnborn()

### Community 68 - "drained"
Cohesion: 0.47
Nodes (9): drained(), newLoginModel(), TestLoginMarkedBackAndCancel(), TestLoginOAuthDeclinedFlipAndCancel(), TestLoginOAuthFlowAndAuthFlip(), TestLoginRefusedWhileRunning(), TestLoginStoresAPIKey(), TestLogoutClearsStoredCredential() (+1 more)

### Community 69 - "§4 Seven tools frozen for v1; schema churn rule (frozen = v2 discussion)"
Cohesion: 0.11
Nodes (20): v0.1.0 failing-test investigation protocol (edit refuses until read+search done), v0.1.0 lazy MCP proxy: stdio+HTTP, persisted index, ~200-token tool, mcp import, v0.1.0 permissions: path jail, mvdan.cc/sh analysis, trust.json, yolo mode, v0.1.0 seven frozen tools: read/write/edit/shell/search/ls/mcp, v0.1.0 pre-edit snapshots + /undo (skipped for clean tracked files), §5 Edit-tool contract: uniqueness, re-indentation, CRLF/BOM, model-facing errors, §14 Ship gate: one unattended session on opencode-go (read→search→edit→test→commit→rtk skill), Tool edit: exact-first with whitespace-tolerant fallback, replace_all (+12 more)

### Community 70 - "Start"
Cohesion: 0.12
Nodes (17): PromptInput, ServerLine, BuildSystemPrompt(), filterSkills(), GitState(), oneLine(), Platform(), TestBuildSystemPrompt() (+9 more)

### Community 71 - "LineReader"
Cohesion: 0.22
Nodes (9): loginAPIKey(), LineReader, parsePasted(), pasteReader(), TestParsePasted(), readLine(), lineResult, LoginIO (+1 more)

### Community 73 - "trust_test.go"
Cohesion: 0.33
Nodes (4): canonical(), LoadTrust(), TestTrustStore(), TrustStore

### Community 74 - "probe/main.go"
Cohesion: 0.12
Nodes (8): Agent, main(), Shell, winName(), NewUnjailed(), TestUnjailedAndAllowAll(), AllowAll, Unjailed

### Community 75 - "AGENTS.md — moca"
Cohesion: 0.25
Nodes (7): AGENTS.md — moca, Commands, Conventions & gotchas, Docs, graphify, Ground truth (read before changing behavior), Layout & dependency rules

### Community 76 - "TestModelPickerRefreshesOllama"
Cohesion: 0.33
Nodes (6): contains(), count(), dropNames(), TestDropdownListsCommandsAndPrompts(), TestModelPickerRefreshesOllama(), TestShowCompletionNewestFirst()

### Community 78 - "ReadTracker"
Cohesion: 0.47
Nodes (4): NewReadTracker(), stampOf(), ReadTracker, stamp

### Community 80 - "CostConfig"
Cohesion: 0.50
Nodes (4): ModelOverride, CostConfig, costM(), TestCostOf()

### Community 82 - "rtk token-compressed CLI proxy; analyser unwraps rtk <cmd>"
Cohesion: 0.33
Nodes (5): §10 External tools & extensibility: skills + slash commands + allowlist (no plugins), rtk token-compressed CLI proxy; analyser unwraps rtk <cmd>, Shell command analysis via mvdan.cc/sh: every simple command, wrappers, redirects, hard-deny set, rm ask-every-time, §11 Permissions: canonicalized jail, hard-deny/refused/builtin classes, case-insensitive matching, Windows best-effort, trust store, yolo checkers, rtk unwrap ladder (phase 6): runners, proxies, config-mutating subcommands; rtk.go capture

### Community 85 - "CI Workflow (gofmt · vet · build · test)"
Cohesion: 0.40
Nodes (4): CI Workflow (gofmt · vet · build · test), CI gate set (gofmt, go vet, go build, go test -race), go.mod (Go version source for setup-go), SPECS.md §2 — CI gate definition

### Community 87 - "§11 TUI: inline immutable scrollback, pager, chips, multi-line paste, steering"
Cohesion: 0.50
Nodes (5): §11 TUI: inline immutable scrollback, pager, chips, multi-line paste, steering, §3.5 TUI: layout, bands, status bar, keys, pager, chips, runs, commands, !cmd / !!cmd shell prefixes from the input box, README slash commands list + prompt templates + // escape, README TUI keys: enter, shift+enter, esc, ctrl+o, alt+t, alt+p, a/ctrl+a/d, dropdown keys

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

### Community 102 - "Skills: SKILL.md in ~/.config/moca/skills or <repo>/.moca/skills; cross-tool compatibility"
Cohesion: 0.67
Nodes (3): External tools: rtk + graphify allowlisted with built-in skills, Project instructions: AGENTS.md / CLAUDE.md, trusted projects only, Skills: SKILL.md in ~/.config/moca/skills or <repo>/.moca/skills; cross-tool compatibility

## Ambiguous Edges - Review These
- `v0.1.0 OpenAI subscription OAuth: Sign in with ChatGPT (PKCE S256, dynamic registration, loopback, JWKS, rotating refresh tokens)` → `v0.1.0 permissions: path jail, mvdan.cc/sh analysis, trust.json, yolo mode`  [AMBIGUOUS]
  CHANGELOG.md · relation: conceptually_related_to

## Knowledge Gaps
- **156 isolated node(s):** `usageError`, `github.com/adeotek/moca`, `claudeServer`, `opencodeServer`, `request` (+151 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 305 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **27 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **What is the exact relationship between `v0.1.0 OpenAI subscription OAuth: Sign in with ChatGPT (PKCE S256, dynamic registration, loopback, JWKS, rotating refresh tokens)` and `v0.1.0 permissions: path jail, mvdan.cc/sh analysis, trust.json, yolo mode`?**
  _Edge tagged AMBIGUOUS (relation: conceptually_related_to) - confidence is low._
- **Why does `model` connect `model` to `start.go`, `Registry`, `Item`, `renderMarkdown`, `Sanitize`, `.View`, `Input`, `model`, `NewDefaultStore`, `RenderStatus`, `Run`, `Agent`, `go_pkg_charm_land_lipgloss_v2`?**
  _High betweenness centrality (0.029) - this node is a cross-community bridge._
- **What connects `usageError`, `github.com/adeotek/moca`, `claudeServer` to the rest of the system?**
  _156 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `start.go` be split into smaller, more focused modules?**
  _Cohesion score 0.12783018867924528 - nodes in this community are weakly interconnected._
- **Why does `Input` connect `Input` to `input_test.go`, `model`?**
  _High betweenness centrality (0.015) - this node is a cross-community bridge._
- **Should `newFakeAS` be split into smaller, more focused modules?**
  _Cohesion score 0.058126619770455384 - nodes in this community are weakly interconnected._
- **Why does `Env` connect `context.Context` to `Registry`, `NewManager`, `fixes_test.go`, `ReadTracker`, `run`, `Run`, `Agent`, `.Run`?**
  _High betweenness centrality (0.013) - this node is a cross-community bridge._