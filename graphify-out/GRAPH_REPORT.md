# Graph Report - moca  (2026-10-08)

## Corpus Check
- 215 files · ~157,791 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 10 file(s) not represented in the graph (top: (none) 5, .jsonc 3, .v2 1)

## Summary
- 1725 nodes · 7029 edges · 102 communities (74 shown, 28 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 826 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `cebd816c`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- start.go
- newScript
- permissions/shell.go
- moca DESIGN.md — v1 contract (rev 11)
- retry_test.go
- anthropic_test.go
- testing.T
- decode
- SOURCES.md
- Registry
- runLogin
- auth_test.go
- CI Workflow (gofmt · vet · build · test)
- AppendString
- sync.Mutex
- model
- cli_test.go
- NewManager
- cut_test.go
- Create
- newTestModel
- registry_test.go
- Agent
- MCP call gating (readOnlyHint/destructiveHint, approve lists, allow-always persistence)
- CostConfig
- Manager
- ToolSpec
- Start
- AllowAll
- Agent
- importer.go
- Run
- newProxy
- Message
- Sanitize
- boundedWriter
- charm.land/bubbletea/v2.Msg
- Claude Code skill-creator skill
- session/entry.go
- Config
- Compacted
- MCP lazy proxy (§10.5)
- .Run
- oauth_test.go
- Resumed
- Index
- SteeringApplied
- EnvError
- pagerModel
- initialize
- input_test.go
- Env
- extractBuiltins
- Request
- AGENTS.md — moca
- Usage
- NewUnjailed
- quick_validate.py
- ToolCall
- encoding/json.RawMessage
- serverEnv
- Retry/error taxonomy (DefaultRetryPolicy, mid-stream reset retry, stall timeout)
- runOneShot
- context.Context
- Agent loop (§14: turn contract, steering, maxSteps wrap-up)
- .Run
- ReadTracker
- Item
- external-tools.md
- LineReader
- Claude Code frontend-design skill
- pi mcp-scripting skill
- allowAll
- StartError
- approvalPrompt
- newHTTPError
- .Run
- fakeCmds
- StreamReset
- TextDelta
- format.go
- countingTransport
- ThinkingDelta
- retryable
- Input
- parseArgs
- Warning
- Arrange-Act-Assert with xUnit/NSubstitute
- .Run
- fixture/main.go
- config.AppendString
- YoloChanged
- fakeSnap
- Agent.Close — stops MCP servers, closes session writer at shutdown
- rootChecker
- github.com/adeotek/moca
- 2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)
- v0.1.0-alpha (2026-10-07)
- trustModel
- BuildSystemPrompt
- run.sh
- example.com/totals

## God Nodes (most connected - your core abstractions)
1. `model` - 54 edges
2. `newScript()` - 46 edges
3. `run()` - 44 edges
4. `startTestWith()` - 35 edges
5. `sseServer()` - 34 edges
6. `ReadFile()` - 31 edges
7. `newTestModel()` - 31 edges
8. `Agent` - 30 edges
9. `Config` - 30 edges
10. `Parse()` - 28 edges

## Surprising Connections (you probably didn't know these)
- `parseArgs()` --calls--> `ParseEffort()`  [EXTRACTED]
  cmd/moca/cli.go → internal/llm/types.go
- `TestHelpListsSurface()` --calls--> `run()`  [INFERRED]
  cmd/moca/cli_test.go → internal/tools/read_test.go
- `TestMCPImportCLI()` --calls--> `Parse()`  [EXTRACTED]
  cmd/moca/cli_test.go → internal/config/config.go
- `TestMCPImportCLI()` --calls--> `run()`  [INFERRED]
  cmd/moca/cli_test.go → internal/tools/read_test.go
- `TestOneShotStreamsToStdout()` --calls--> `run()`  [INFERRED]
  cmd/moca/cli_test.go → internal/tools/read_test.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Ecosystem SKILL.md compatibility corpus (graphify, Claude Code, pi, OpenCode)** — internal_skills_testdata_ecosystem_sources, internal_skills_testdata_ecosystem, internal_skills_testdata_ecosystem_graphify_graphify_vendored, internal_skills_testdata_ecosystem_claude_code_skill_creator_vendored, internal_skills_testdata_ecosystem_claude_code_frontend_design_vendored, internal_skills_testdata_ecosystem_pi_pi_subagents_vendored, internal_skills_testdata_ecosystem_pi_mcp_scripting_vendored, internal_skills_testdata_ecosystem_opencode_dotnet_unit_testing_vendored [EXTRACTED 1.00]
- **Synthetic frontmatter edge-case fixtures (CRLF, BOM, folding, quoting, rich keys, long description)** — internal_skills_testdata_ecosystem_synthetic_bom_skill, internal_skills_testdata_ecosystem_synthetic_colon_unquoted_skill, internal_skills_testdata_ecosystem_synthetic_crlf_skill, internal_skills_testdata_ecosystem_synthetic_folded_skill, internal_skills_testdata_ecosystem_synthetic_literal_skill, internal_skills_testdata_ecosystem_synthetic_long_description_skill, internal_skills_testdata_ecosystem_synthetic_rich_keys_skill [EXTRACTED 1.00]
- **Human-in-the-loop validation before declaring done** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_claude_code_frontend_design_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_review_validation [INFERRED 0.75]
- **Parallel subagent fanout with parent synthesis** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_graphify_graphify_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_skill [INFERRED 0.85]
- **Skills referencing bundled reference files for progressive disclosure** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_graphify_graphify_skill, internal_skills_testdata_ecosystem_pi_mcp_scripting_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_skill [INFERRED 0.85]
- **moca extension model (skills + slash commands + shell allowlist + lazy MCP)** — docs_specs_design_skills, docs_specs_design_external_tools, docs_specs_design_mcp_lazy_proxy, docs_specs_design_permissions, docs_specs_design_explicit_non_goals [INFERRED 0.90]

## Communities (102 total, 28 thin omitted)

### Community 0 - "start.go"
Cohesion: 0.11
Nodes (14): request, usageError, anthropicEvent, anthropicUsage, completionsChunk, Credential, responsesEvent, use (+6 more)

### Community 1 - "newScript"
Cohesion: 0.08
Nodes (63): scriptServer, newScript(), startTest(), startTestWith(), startWith(), TestAbortWritesSyntheticResults(), TestFailedCallDoesNotCancelRest(), TestLengthStopTruncatedToolCall() (+55 more)

### Community 2 - "permissions/shell.go"
Cohesion: 0.07
Nodes (40): canonical(), Jail, NewJail(), resolveDeep(), setup(), TestJail(), TestJailDanglingSymlinks(), TestJailKeepsNotYetExistingReadOnlyRoot() (+32 more)

### Community 3 - "moca DESIGN.md — v1 contract (rev 11)"
Cohesion: 0.18
Nodes (20): moca DESIGN.md — v1 contract (rev 11), Architecture and dependency direction (§2), CLI surface (§12.5), Config: JSONC, typed, fail-fast (§12), Context manager: budgets, compaction, overflow (§6), Edit-tool contract (§5), Explicit non-goals (§15), External tools & extensibility (§10) (+12 more)

### Community 4 - "retry_test.go"
Cohesion: 0.27
Nodes (16): Retry, DefaultRetryPolicy(), RetryNotice, fail(), TestMidStreamFailureResetsAndRetriesOnce(), TestMidStreamRetryBacksOffAndNotifies(), TestNoRetryOn400(), testPolicy() (+8 more)

### Community 5 - "anthropic_test.go"
Cohesion: 0.14
Nodes (39): SplitModel(), TestSplitModel(), Adapter, newAnthropic(), keyCred(), mustJSON(), sseServer(), TestAnthropicBodyHygiene() (+31 more)

### Community 6 - "testing.T"
Cohesion: 0.05
Nodes (61): TestResumeNotASession(), NormalizeToolID(), TestNormalizeToolID(), TestTransformDropsEmptiedAssistantAndMerges(), TestTransformDropsRedactedForeignThinking(), TestTransformHistory(), TestTransformSameBareIDDifferentProvider(), TransformHistory() (+53 more)

### Community 7 - "decode"
Cohesion: 0.14
Nodes (6): readLine(), decode(), errorf(), lsTool, mcpStub, readTool

### Community 8 - "SOURCES.md"
Cohesion: 0.19
Nodes (7): internal/skills/testdata/ecosystem corpus, anthropics/skills frontend-design, anthropics/skills skill-creator, graphify skill (graphify install, claude platform; byte-identical to pi variant), adeotek/dotfiles opencode dotnet-unit-testing skill, npm pi-mcp-adapter mcp-scripting skill, npm pi-subagents skill

### Community 9 - "Registry"
Cohesion: 0.21
Nodes (7): Builtins(), runReg(), TestRegistryInvestigationBanner(), Registry, NewRegistry(), TestSchemasFrozen(), Tool

### Community 10 - "runLogin"
Cohesion: 0.23
Nodes (13): printUsage(), runLogin(), runLogout(), yesNo(), main(), run(), confirm(), runMCP() (+5 more)

### Community 11 - "auth_test.go"
Cohesion: 0.23
Nodes (15): NewStore(), TestConcurrentRefreshOnce(), TestCredentialForLockWaitHonoursContext(), TestDeleteClearsCorruptStore(), TestHostIDStable(), TestInvalidClient(), TestLockFileWaitsThenAcquires(), TestNotLoggedIn() (+7 more)

### Community 12 - "CI Workflow (gofmt · vet · build · test)"
Cohesion: 0.40
Nodes (4): CI Workflow (gofmt · vet · build · test), CI gate set (gofmt, go vet, go build, go test -race), go.mod (Go version source for setup-go), SPECS.md §2 — CI gate definition

### Community 13 - "AppendString"
Cohesion: 0.10
Nodes (27): scanner, span, AppendString(), concat(), nested(), nestedEntry(), objectHasKey(), objectValueSpan() (+19 more)

### Community 14 - "sync.Mutex"
Cohesion: 0.15
Nodes (9): startStdio(), TestStdioEarlyExit(), TestStdioWriteEPIPE(), KillProcessGroup(), SetProcessGroup(), KillProcessGroup(), SetProcessGroup(), ring (+1 more)

### Community 15 - "model"
Cohesion: 0.15
Nodes (3): model, TestWelcomeLines(), welcomeText()

### Community 16 - "cli_test.go"
Cohesion: 0.09
Nodes (48): fakeCompletions(), isolate(), TestExitCodes(), TestHelpListsSurface(), TestInterruptedExit130(), TestLoginLogoutCLI(), TestMCPImportCLI(), TestNoApproveSkipsProjectInstructions() (+40 more)

### Community 17 - "NewManager"
Cohesion: 0.13
Nodes (33): crashingServer(), shServer(), skipWindows(), TestCallTimeoutStopsWedgedServer(), TestHandshakeTimeout(), TestHTTPAbortedStreamReplayRule(), TestIndexSaveMergesConcurrentWriters(), TestListToolsRepeatedCursor() (+25 more)

### Community 18 - "cut_test.go"
Cohesion: 0.08
Nodes (37): Kind, Result, Agent, summarizerUnavailable(), Budget, NewBudget(), TestBudget(), FindCut() (+29 more)

### Community 19 - "Create"
Cohesion: 0.07
Nodes (30): Find(), FindForWorkdir(), idOf(), listNewestFirst(), TestFind(), TestFindForWorkdirViaSymlink(), TestFindMtimeTieDeterministic(), TestFindNoSessions() (+22 more)

### Community 20 - "newTestModel"
Cohesion: 0.06
Nodes (57): TestActivityLine(), TestInputBoxGrowsAndShrinks(), TestPagerCloseDefersTheRelease(), TestRefuseRunningWhileOutputInFlight(), TestShrinkIsPaddedUntilTheGuardWindowPasses(), TestShrinkPaddingFitsTheScreen(), TestSpinnerTicksOnlyForTheLiveRun(), TestThinkingReadableInPager() (+49 more)

### Community 21 - "registry_test.go"
Cohesion: 0.30
Nodes (15): NewRegistry(), mustCfg(), TestBaseURLPrecedence(), TestBuiltinModelNeedsProtocol(), TestBuiltinProviderNewModelWithModelProtocol(), TestCheckCredentialOAuthDoesNotTouchNetwork(), TestCustomProviderAndLazyMissingKey(), TestNoSessionHeaderOutsideOpenCodeGo() (+7 more)

### Community 22 - "Agent"
Cohesion: 0.10
Nodes (13): Options, savedModel, Agent, Outcome, New(), Effort, ParseEffort(), TestParseEffort() (+5 more)

### Community 23 - "MCP call gating (readOnlyHint/destructiveHint, approve lists, allow-always persistence)"
Cohesion: 0.20
Nodes (9): config.AppendString — comment-preserving JSONC string-array append, JSONC config pre-pass + comment-preserving edits (AppendString/SetObjectEntry), config.SetObjectEntry — JSONC object-key insert (phase 5), MCP call gating (readOnlyHint/destructiveHint, approve lists, allow-always persistence), `moca mcp import` importer (DiscoverSources, secret rewriting, SetObjectEntry writes), Path jail (canonical root + read-only roots, symlink resolution), Parsed shell analysis incl. rtk unwrap (phase 6), Skills ecosystem compatibility (vendored corpus, SKILL.md parsers, prompt templates) (+1 more)

### Community 24 - "CostConfig"
Cohesion: 0.40
Nodes (5): ModelOverride, ProviderConfig, CostConfig, costM(), TestCostOf()

### Community 25 - "Manager"
Cohesion: 0.29
Nodes (3): ConfigHash(), Manager, state

### Community 26 - "ToolSpec"
Cohesion: 0.20
Nodes (6): ToolSpec, TestProxySpecFrozen(), MCPSpec(), TestRegistryDispatch(), echoTool, searchTool

### Community 27 - "Start"
Cohesion: 0.17
Nodes (16): setup, GitState(), Platform(), build(), StartOptions, prepare(), Start(), TestPathsHonourXDG() (+8 more)

### Community 30 - "importer.go"
Cohesion: 0.17
Nodes (20): MCPServer, TestImportRewritesEmbeddedSecretsInValues(), TestRewriteSecretsValueRuleAlone(), DiscoverSources(), embeddedSecret(), envName(), literalSecretFields(), ParseSource() (+12 more)

### Community 31 - "Run"
Cohesion: 0.19
Nodes (10): Prompt, newModel(), Run(), HelpText(), ParseInput(), TestParseInput(), AppOptions, compactDoneMsg (+2 more)

### Community 32 - "newProxy"
Cohesion: 0.43
Nodes (8): NewTool(), newProxy(), runP(), TestProxyArgErrors(), TestProxyGating(), TestProxySearchDescribeCall(), TestProxySearchDescriptionRuneSafe(), ProxyTool

### Community 33 - "Message"
Cohesion: 0.24
Nodes (13): MessageChars(), RequestChars(), TestEstimate(), Tokens(), UsageTokens(), Message, Role, appendUser() (+5 more)

### Community 34 - "Sanitize"
Cohesion: 0.16
Nodes (9): ShellOutput, TestFmtElapsed(), fmtElapsed(), model, hasControl(), Sanitize(), TestSanitize(), TestSanitizeC1() (+1 more)

### Community 36 - "charm.land/bubbletea/v2.Msg"
Cohesion: 0.18
Nodes (10): Event, Asker, newAsker(), newEventPipe(), TestEventPipeBuffersUntilStarted(), TestEventPipeNonBlockingAndFIFO(), TestPipeSendKeepsOrderWithEvents(), flatten() (+2 more)

### Community 37 - "Claude Code skill-creator skill"
Cohesion: 0.16
Nodes (13): Skill description (trigger) optimization, Skill eval/iterate loop (with-skill vs baseline), skill-creator Apache 2.0 License (Anthropic), Claude Code skill-creator skill, SKILL.md anatomy + bundled resources layout, Constrained query expansion against graph vocab, Progressive Disclosure (three-level skill loading), graphify query/path/explain reference (+5 more)

### Community 38 - "session/entry.go"
Cohesion: 0.29
Nodes (10): Entry, ModelChange, header(), Compaction, ErrorInfo, Header, PermissionMode, SnapshotRec (+2 more)

### Community 39 - "Config"
Cohesion: 0.21
Nodes (8): ContextConfig, MCPConfig, ShellConfig, SnapshotConfig, Default(), Config, Load(), TestLoadMissingFileIsDefault()

### Community 41 - "MCP lazy proxy (§10.5)"
Cohesion: 0.17
Nodes (11): moca SPECS.md (current implemented state), env:VAR lazy credential indirection, MCP persisted discovery index (mcp-index.json, ConfigHash, word-token Rank), `moca mcp index` subcommand (IndexAll over configured servers), MCP JSON-RPC 2.0 protocol handling (initialize, tools/list paging, tools/call), MCP lazy proxy (§10.5), MCP Manager lifecycle (lazy ensure, idle stop, busy counter, restart-once replay), The `mcp` proxy tool (search/describe/call rendering, 30K truncation, frozen spec) (+3 more)

### Community 42 - ".Run"
Cohesion: 0.14
Nodes (18): applyEdit(), formatDiff(), indentOf(), joinInts(), lineOf(), matchLines(), replaceExact(), replaceFuzzy() (+10 more)

### Community 43 - "oauth_test.go"
Cohesion: 0.05
Nodes (52): fakeSIWC(), fakeSIWCOpts(), TestDefaultHTTPClientBoundsHeaderWait(), defaultHTTPClient(), newHostID(), audContains(), b64d(), OAuthConfig (+44 more)

### Community 45 - "Index"
Cohesion: 0.22
Nodes (6): Rank(), TestRank(), words(), Hit, Index, IndexEntry

### Community 48 - "pagerModel"
Cohesion: 0.20
Nodes (3): newPager(), TestTeaAPI(), pagerModel

### Community 49 - "initialize"
Cohesion: 0.22
Nodes (8): initialize(), TestHTTPCloseBounded(), startHTTP(), fakeHTTPServer(), TestHTTPNotifyStatusChecked(), TestHTTPTransport(), headerLog, transport

### Community 50 - "input_test.go"
Cohesion: 0.30
Nodes (11): NewInput(), TestCollapseUsesExactOffset(), TestCtrlC(), TestEditedMarkerSentLiterally(), TestHistory(), TestHistoryRecallChipsUnsafeContent(), TestMarkerNeverCollidesWithTypedText(), TestPasteChipKeepsContent() (+3 more)

### Community 51 - "Env"
Cohesion: 0.20
Nodes (9): cutRunes(), errResult(), isTrue(), TestCutRunesSafe(), TestInvestigationHint(), Env, Result, investigationHint() (+1 more)

### Community 52 - "extractBuiltins"
Cohesion: 0.40
Nodes (5): builtinHash(), ExtractBuiltins(), extractBuiltins(), TestExtractBuiltins(), TestExtractBuiltinsReExtractsOnContentChange()

### Community 53 - "Request"
Cohesion: 0.15
Nodes (16): ContentBlock, Request, ToolResult, post(), anthropicOverflow(), openaiOverflow(), priorReasoningText(), replaysVerbatim() (+8 more)

### Community 54 - "AGENTS.md — moca"
Cohesion: 0.25
Nodes (7): AGENTS.md — moca, Commands, Conventions & gotchas, Docs, graphify, Ground truth (read before changing behavior), Layout & dependency rules

### Community 55 - "Usage"
Cohesion: 0.20
Nodes (5): Status, TurnEnd, StopReason, Usage, anthropicStop()

### Community 56 - "NewUnjailed"
Cohesion: 0.29
Nodes (4): Agent, NewUnjailed(), TestUnjailedAndAllowAll(), Unjailed

### Community 58 - "ToolCall"
Cohesion: 0.17
Nodes (9): ToolEnd, ToolStart, Event, Response, ToolCall, ToolChoice, ok(), EventType (+1 more)

### Community 59 - "encoding/json.RawMessage"
Cohesion: 0.25
Nodes (6): callResult, Annotations, IndexTool, response, rpcError, Tool

### Community 60 - "serverEnv"
Cohesion: 0.29
Nodes (8): TestResolveEnv(), ResolveEnv(), TestKeepNameWindows(), FilterEnv(), keepName(), serverEnv(), TestFilterEnv(), TestServerEnvExplicitEntryReplacesInherited()

### Community 62 - "runOneShot"
Cohesion: 0.27
Nodes (8): TestExitForOAuthSentinels(), exitFor(), firstLine(), resolveResume(), runOneShot(), decideTrust(), hasProjectResources(), runTUI()

### Community 63 - "context.Context"
Cohesion: 0.12
Nodes (13): runMCPIndex(), fakeSum, CallResult, responseID(), TestResponseID(), readEvents(), lockFile(), lockFile() (+5 more)

### Community 64 - "Agent loop (§14: turn contract, steering, maxSteps wrap-up)"
Cohesion: 0.40
Nodes (4): Agent loop (§14: turn contract, steering, maxSteps wrap-up), Context manager compaction (budgets, cut points, structured summary, overflow recovery), Cross-provider thinking replay transform + NormalizeToolID, Session resume (--resume/--continue, repair, single-writer flock)

### Community 65 - ".Run"
Cohesion: 0.32
Nodes (5): globToRegexp(), ignored(), parseIgnore(), TestIgnore(), ignoreRule

### Community 66 - "ReadTracker"
Cohesion: 0.47
Nodes (4): NewReadTracker(), stampOf(), ReadTracker, stamp

### Community 68 - "external-tools.md"
Cohesion: 0.39
Nodes (3): SKILL.md (builtin rtk), graphify (codebase knowledge graph), rtk (token-compressed CLI proxy)

### Community 69 - "LineReader"
Cohesion: 0.33
Nodes (6): LineReader, parsePasted(), pasteReader(), TestParsePasted(), lineResult, pastedResult

### Community 70 - "Claude Code frontend-design skill"
Cohesion: 0.50
Nodes (3): AI-generated design default tells, frontend-design Apache 2.0 License, Claude Code frontend-design skill

### Community 71 - "pi mcp-scripting skill"
Cohesion: 0.67
Nodes (4): jev.evaluate reference, jev.evaluate semantic scripting gate, mcpScript tools API, pi mcp-scripting skill

### Community 74 - "approvalPrompt"
Cohesion: 0.33
Nodes (7): AutoAllow(), Answer, Question, approvalPrompt(), firstLineOf(), TestApprovalPromptSanitized(), approvalMsg

### Community 75 - "newHTTPError"
Cohesion: 0.33
Nodes (5): newHTTPError(), retryAfter(), TestRetryAfterFormats(), TestRetryAfterParsed(), HTTPError

### Community 76 - ".Run"
Cohesion: 0.33
Nodes (3): countFileLines(), countLines(), writeTool

### Community 80 - "format.go"
Cohesion: 0.13
Nodes (21): TestGitStatusUsesNoOptionalLocks(), TestStatusWideRunesNeverExceedWidth(), AbbrevEffort(), AbbrevHome(), FmtTokens(), FmtWindow(), GitBranch(), gitStatusArgs() (+13 more)

### Community 83 - "retryable"
Cohesion: 0.40
Nodes (5): http2Transient(), retryable(), TestRetryableServerError400(), TestRetryable(), TestRetryableTransportErrors()

### Community 84 - "Input"
Cohesion: 0.17
Nodes (3): lineCount(), Input, paste

### Community 85 - "parseArgs"
Cohesion: 0.50
Nodes (3): parseArgs(), TestParseArgs(), Options

### Community 88 - ".Run"
Cohesion: 0.12
Nodes (14): approve(), findTestFile(), isPathByte(), TestFindTestFile(), TestTestRunCommand(), TestTestRunFailed(), testRunCommand(), testRunFailed() (+6 more)

### Community 89 - "fixture/main.go"
Cohesion: 0.21
Nodes (7): Sum(), TestTotals(), ParseList(), Average(), Max(), TestMaxAndAverage(), main()

### Community 97 - "2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)"
Cohesion: 0.15
Nodes (12): 1.1 Terms, 1.2 Client registration, 1.3 Decision, 1. anthropic — decision: `api_key only` (subscription OAuth not permitted), 2.1 Terms, 2.2 Client registration (per the docs), 2.3 Endpoints (verified live 2026-10-06 against `https://auth.openai.com/.well-known/openid-configuration`), 2.4 Inference contract for subscription traffic (open-source flow docs) (+4 more)

### Community 99 - "v0.1.0-alpha (2026-10-07)"
Cohesion: 0.18
Nodes (10): Changelog, CLI, Context & sessions, External tools & the skills ecosystem, MCP, OAuth (subscription login), Providers & protocols, Tools & permissions (+2 more)

### Community 101 - "BuildSystemPrompt"
Cohesion: 0.17
Nodes (12): PromptInput, ServerLine, BuildSystemPrompt(), filterSkills(), oneLine(), TestBuildSystemPrompt(), TestFilterSkillsDropsMissingBuiltins(), TestPromptRTKIsConditional() (+4 more)

## Knowledge Gaps
- **79 isolated node(s):** `usageError`, `github.com/adeotek/moca`, `claudeServer`, `opencodeServer`, `request` (+74 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 212 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **28 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `MCP lazy proxy (§10.5)` connect `MCP lazy proxy (§10.5)` to `MCP call gating (readOnlyHint/destructiveHint, approve lists, allow-always persistence)`?**
  _High betweenness centrality (0.061) - this node is a cross-community bridge._
- **What connects `usageError`, `github.com/adeotek/moca`, `claudeServer` to the rest of the system?**
  _79 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `start.go` be split into smaller, more focused modules?**
  _Cohesion score 0.10717133342713757 - nodes in this community are weakly interconnected._
- **Why does `moca SPECS.md (current implemented state)` connect `MCP lazy proxy (§10.5)` to `external-tools.md`?**
  _High betweenness centrality (0.052) - this node is a cross-community bridge._
- **Should `newScript` be split into smaller, more focused modules?**
  _Cohesion score 0.08397271952259165 - nodes in this community are weakly interconnected._
- **Why does `model` connect `model` to `start.go`, `Sanitize`, `Item`, `charm.land/bubbletea/v2.Msg`, `session/entry.go`, `approvalPrompt`, `pagerModel`, `format.go`, `Input`, `Agent`, `Start`, `Run`?**
  _High betweenness centrality (0.049) - this node is a cross-community bridge._
- **Should `permissions/shell.go` be split into smaller, more focused modules?**
  _Cohesion score 0.0677555958862674 - nodes in this community are weakly interconnected._