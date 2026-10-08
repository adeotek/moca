# Graph Report - moca  (2026-10-08)

## Corpus Check
- 214 files · ~152,898 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 10 file(s) not represented in the graph (top: (none) 5, .jsonc 3, .v2 1)

## Summary
- 1682 nodes · 6805 edges · 93 communities (76 shown, 17 thin omitted)
- Extraction: 89% EXTRACTED · 11% INFERRED · 0% AMBIGUOUS · INFERRED: 777 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `b79a9751`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- start.go
- newScript
- permissions/shell.go
- moca DESIGN.md — v1 contract (rev 11)
- startStdio
- anthropic_test.go
- testing.T
- context.Context
- SOURCES.md
- NewManager
- run
- go_pkg_testing
- CI Workflow (gofmt · vet · build · test)
- Parse
- sync.Mutex
- model
- cli_test.go
- post
- cut_test.go
- Create
- newAgentModel
- snapshot.go
- Agent
- MCP call gating (readOnlyHint/destructiveHint, approve lists, allow-always persistence)
- importer.go
- Manager
- Registry
- Start
- Writer
- Model
- session/entry.go
- estimate.go
- .Run
- Message
- Sanitize
- transform_test.go
- Run
- Claude Code skill-creator skill
- Index
- Config
- testRunCommand
- MCP lazy proxy (§10.5)
- applyEdit
- oauth_test.go
- ToolSpec
- httpTransport
- TestSearchReportsAndRemembersFailedServer
- commands.go
- Item
- DataDir
- input_test.go
- retry_test.go
- builtin.go
- Request
- AGENTS.md — moca
- CostConfig
- .applyYolo
- quick_validate.py
- tui.go
- parseArgs
- pasteReader
- Retry/error taxonomy (DefaultRetryPolicy, mid-stream reset retry, stall timeout)
- Agent
- Agent loop (§14: turn contract, steering, maxSteps wrap-up)
- .Run
- ReadTracker
- external-tools.md
- Claude Code frontend-design skill
- pi mcp-scripting skill
- allowAll
- StartError
- boundedWriter
- fakeCmds
- format.go
- serverEnv
- .Compact
- Input
- auth_test.go
- Arrange-Act-Assert with xUnit/NSubstitute
- TestWelcomeLines
- fixture/main.go
- config.AppendString
- Agent.Close — stops MCP servers, closes session writer at shutdown
- github.com/adeotek/moca
- 2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)
- Usage
- v0.1.0-alpha (2026-10-07)
- runOneShot
- prompt.go
- runLogin
- tui/trust.go
- run.sh
- example.com/totals

## God Nodes (most connected - your core abstractions)
1. `newScript()` - 46 edges
2. `model` - 45 edges
3. `run()` - 44 edges
4. `startTestWith()` - 35 edges
5. `sseServer()` - 34 edges
6. `ReadFile()` - 31 edges
7. `Agent` - 30 edges
8. `Config` - 30 edges
9. `Parse()` - 28 edges
10. `keyCred()` - 27 edges

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

## Communities (93 total, 17 thin omitted)

### Community 0 - "start.go"
Cohesion: 0.10
Nodes (21): Compacted, Resumed, SteeringApplied, StreamReset, TextDelta, ThinkingDelta, Warning, YoloChanged (+13 more)

### Community 1 - "newScript"
Cohesion: 0.09
Nodes (63): scriptServer, newScript(), startTest(), startTestWith(), startWith(), TestAbortWritesSyntheticResults(), TestFailedCallDoesNotCancelRest(), TestLengthStopTruncatedToolCall() (+55 more)

### Community 2 - "permissions/shell.go"
Cohesion: 0.08
Nodes (36): Jail, NewJail(), resolveDeep(), setup(), TestJail(), TestJailDanglingSymlinks(), TestJailKeepsNotYetExistingReadOnlyRoot(), TestJailResolvedPathIsCanonical() (+28 more)

### Community 3 - "moca DESIGN.md — v1 contract (rev 11)"
Cohesion: 0.18
Nodes (20): moca DESIGN.md — v1 contract (rev 11), Architecture and dependency direction (§2), CLI surface (§12.5), Config: JSONC, typed, fail-fast (§12), Context manager: budgets, compaction, overflow (§6), Edit-tool contract (§5), Explicit non-goals (§15), External tools & extensibility (§10) (+12 more)

### Community 4 - "startStdio"
Cohesion: 0.17
Nodes (18): initialize(), shServer(), skipWindows(), TestHTTPCloseBounded(), TestListToolsRepeatedCursor(), TestStdioGrandchildHoldingStdoutDoesNotHangExit(), TestStdioReplyThenExitKeepsReply(), TestStdioStdoutFatalAlwaysSurfaces() (+10 more)

### Community 5 - "anthropic_test.go"
Cohesion: 0.13
Nodes (39): SplitModel(), TestSplitModel(), Adapter, newAnthropic(), keyCred(), mustJSON(), sseServer(), TestAnthropicBodyHygiene() (+31 more)

### Community 6 - "testing.T"
Cohesion: 0.07
Nodes (49): CapChars(), TestCapChars(), TestCapCharsRuneSafe(), TestEffortRankOrdered(), TestUsageAdd(), TestCallToolNullArgsSentAsEmptyObject(), TestHelperProcess(), TestBuiltinCatalogSane() (+41 more)

### Community 7 - "context.Context"
Cohesion: 0.09
Nodes (22): CallResult, errResult(), lockFile(), lockFile(), approve(), findTestFile(), isPathByte(), TestFindTestFile() (+14 more)

### Community 8 - "SOURCES.md"
Cohesion: 0.19
Nodes (7): internal/skills/testdata/ecosystem corpus, anthropics/skills frontend-design, anthropics/skills skill-creator, graphify skill (graphify install, claude platform; byte-identical to pi variant), adeotek/dotfiles opencode dotnet-unit-testing skill, npm pi-mcp-adapter mcp-scripting skill, npm pi-subagents skill

### Community 9 - "NewManager"
Cohesion: 0.19
Nodes (21): crashingServer(), TestCallTimeoutStopsWedgedServer(), TestHandshakeTimeout(), TestHTTPAbortedStreamReplayRule(), TestIndexSaveMergesConcurrentWriters(), TestSearchReportsUnsavedIndex(), TestStdioCrashMidCallReplayRule(), TestStdioDeadBetweenCallsRestarts() (+13 more)

### Community 10 - "run"
Cohesion: 0.36
Nodes (8): printUsage(), main(), run(), confirm(), runMCP(), runMCPImport(), runMCPIndex(), LoginIO

### Community 11 - "go_pkg_testing"
Cohesion: 0.29
Nodes (4): lockFile(), lockFile(), lastNewline(), use

### Community 12 - "CI Workflow (gofmt · vet · build · test)"
Cohesion: 0.40
Nodes (4): CI Workflow (gofmt · vet · build · test), CI gate set (gofmt, go vet, go build, go test -race), go.mod (Go version source for setup-go), SPECS.md §2 — CI gate definition

### Community 13 - "Parse"
Cohesion: 0.07
Nodes (37): scanner, span, Parse(), TestDefaultsApplied(), TestOAuthAllowedForOpenAI(), TestOAuthRejectedForNonOAuthProvider(), TestRetentionZeroMeansForever(), TestSpecExampleDecodesIntact() (+29 more)

### Community 14 - "sync.Mutex"
Cohesion: 0.12
Nodes (9): callResult, KillProcessGroup(), SetProcessGroup(), KillProcessGroup(), SetProcessGroup(), response, ring, rpcError (+1 more)

### Community 15 - "model"
Cohesion: 0.18
Nodes (4): model, println(), printlnContent(), compactDoneMsg

### Community 16 - "cli_test.go"
Cohesion: 0.09
Nodes (49): fakeCompletions(), isolate(), TestExitCodes(), TestHelpListsSurface(), TestInterruptedExit130(), TestLoginLogoutCLI(), TestMCPImportCLI(), TestNoApproveSkipsProjectInstructions() (+41 more)

### Community 17 - "post"
Cohesion: 0.14
Nodes (10): fakeHTTPServer(), TestHTTPTransport(), post(), newHTTPError(), retryAfter(), TestRetryAfterFormats(), TestRetryAfterParsed(), headerLog (+2 more)

### Community 18 - "cut_test.go"
Cohesion: 0.17
Nodes (23): FindCut(), a(), call(), res(), TestFindCutKeepsRecentAtValidPoint(), TestFindCutNeverBetweenCallAndResult(), TestFindCutNothingToDo(), TestFindCutSplitTurn() (+15 more)

### Community 19 - "Create"
Cohesion: 0.13
Nodes (19): Find(), FindForWorkdir(), idOf(), listNewestFirst(), TestFind(), TestFindForWorkdirViaSymlink(), TestFindMtimeTieDeterministic(), TestFindNoSessions() (+11 more)

### Community 20 - "newAgentModel"
Cohesion: 0.07
Nodes (41): key(), keyMsg(), newTestModel(), TestApprovalPreservesDraft(), TestExitCommandQuits(), TestInputRulesAndNoPrefix(), TestPasteDoesNotSend(), TestRunDoneClearsTransient() (+33 more)

### Community 21 - "snapshot.go"
Cohesion: 0.21
Nodes (5): Prune(), TestPrune(), hashFile(), TestEcosystemSkillsLoadUnchanged(), TestEcosystemSourcesHashes()

### Community 22 - "Agent"
Cohesion: 0.12
Nodes (12): Options, ToolEnd, ToolStart, Agent, Outcome, New(), Response, ToolCall (+4 more)

### Community 23 - "MCP call gating (readOnlyHint/destructiveHint, approve lists, allow-always persistence)"
Cohesion: 0.20
Nodes (9): config.AppendString — comment-preserving JSONC string-array append, JSONC config pre-pass + comment-preserving edits (AppendString/SetObjectEntry), config.SetObjectEntry — JSONC object-key insert (phase 5), MCP call gating (readOnlyHint/destructiveHint, approve lists, allow-always persistence), `moca mcp import` importer (DiscoverSources, secret rewriting, SetObjectEntry writes), Path jail (canonical root + read-only roots, symlink resolution), Parsed shell analysis incl. rtk unwrap (phase 6), Skills ecosystem compatibility (vendored corpus, SKILL.md parsers, prompt templates) (+1 more)

### Community 24 - "importer.go"
Cohesion: 0.16
Nodes (19): MCPServer, TestImportRewritesEmbeddedSecretsInValues(), TestRewriteSecretsValueRuleAlone(), DiscoverSources(), embeddedSecret(), envName(), literalSecretFields(), ParseSource() (+11 more)

### Community 26 - "Registry"
Cohesion: 0.25
Nodes (6): runReg(), TestRegistryInvestigationBanner(), Registry, NewRegistry(), TestRegistryDispatch(), Tool

### Community 27 - "Start"
Cohesion: 0.16
Nodes (13): setup, GitState(), Platform(), build(), StartOptions, prepare(), Start(), TestEnvRefs() (+5 more)

### Community 28 - "Writer"
Cohesion: 0.22
Nodes (4): gitClean(), Snapshots, NewSnapshots(), Writer

### Community 29 - "Model"
Cohesion: 0.14
Nodes (7): savedModel, Status, Effort, ParseEffort(), TestParseEffort(), Model, supportedEfforts()

### Community 30 - "session/entry.go"
Cohesion: 0.33
Nodes (9): Entry, ModelChange, header(), Compaction, ErrorInfo, Header, PermissionMode, SnapshotRec (+1 more)

### Community 31 - "estimate.go"
Cohesion: 0.50
Nodes (6): EntryTokens(), MessageChars(), RequestChars(), TestEstimate(), Tokens(), UsageTokens()

### Community 32 - ".Run"
Cohesion: 0.14
Nodes (12): cutRunes(), isTrue(), TestCutRunesSafe(), runePrefix(), runeSuffix(), TestTruncate(), TestTruncateLongLines(), Truncate() (+4 more)

### Community 33 - "Message"
Cohesion: 0.22
Nodes (13): Kind, Entry, ContentBlock, Message, Role, ToolResult, appendUser(), LatestCompaction() (+5 more)

### Community 34 - "Sanitize"
Cohesion: 0.12
Nodes (13): ShellOutput, AutoAllow(), Answer, Question, approvalPrompt(), firstLineOf(), TestApprovalPromptSanitized(), hasControl() (+5 more)

### Community 35 - "transform_test.go"
Cohesion: 0.39
Nodes (7): NormalizeToolID(), TestNormalizeToolID(), TestTransformDropsEmptiedAssistantAndMerges(), TestTransformDropsRedactedForeignThinking(), TestTransformHistory(), TestTransformSameBareIDDifferentProvider(), TransformHistory()

### Community 36 - "Run"
Cohesion: 0.14
Nodes (13): Event, Asker, newModel(), Run(), runResult(), newAsker(), newEventPipe(), TestEventPipeBuffersUntilStarted() (+5 more)

### Community 37 - "Claude Code skill-creator skill"
Cohesion: 0.16
Nodes (13): Skill description (trigger) optimization, Skill eval/iterate loop (with-skill vs baseline), skill-creator Apache 2.0 License (Anthropic), Claude Code skill-creator skill, SKILL.md anatomy + bundled resources layout, Constrained query expansion against graph vocab, Progressive Disclosure (three-level skill loading), graphify query/path/explain reference (+5 more)

### Community 38 - "Index"
Cohesion: 0.22
Nodes (6): Rank(), TestRank(), words(), Hit, Index, IndexEntry

### Community 39 - "Config"
Cohesion: 0.21
Nodes (8): ContextConfig, MCPConfig, ShellConfig, SnapshotConfig, Default(), Config, Load(), TestLoadMissingFileIsDefault()

### Community 40 - "testRunCommand"
Cohesion: 0.50
Nodes (4): TestTestRunCommand(), TestTestRunFailed(), testRunCommand(), testRunFailed()

### Community 41 - "MCP lazy proxy (§10.5)"
Cohesion: 0.17
Nodes (11): moca SPECS.md (current implemented state), env:VAR lazy credential indirection, MCP persisted discovery index (mcp-index.json, ConfigHash, word-token Rank), `moca mcp index` subcommand (IndexAll over configured servers), MCP JSON-RPC 2.0 protocol handling (initialize, tools/list paging, tools/call), MCP lazy proxy (§10.5), MCP Manager lifecycle (lazy ensure, idle stop, busy counter, restart-once replay), The `mcp` proxy tool (search/describe/call rendering, 30K truncation, frozen spec) (+3 more)

### Community 42 - "applyEdit"
Cohesion: 0.23
Nodes (14): applyEdit(), formatDiff(), indentOf(), joinInts(), lineOf(), matchLines(), replaceExact(), replaceFuzzy() (+6 more)

### Community 43 - "oauth_test.go"
Cohesion: 0.06
Nodes (50): fakeSIWC(), fakeSIWCOpts(), TestDefaultHTTPClientBoundsHeaderWait(), defaultHTTPClient(), newHostID(), audContains(), b64d(), OAuthConfig (+42 more)

### Community 44 - "ToolSpec"
Cohesion: 0.13
Nodes (9): ToolSpec, TestProxySpecFrozen(), MCPSpec(), echoTool, editTool, lsTool, readTool, shellTool (+1 more)

### Community 45 - "httpTransport"
Cohesion: 0.25
Nodes (5): fakeSum, responseID(), TestResponseID(), readEvents(), httpTransport

### Community 46 - "TestSearchReportsAndRemembersFailedServer"
Cohesion: 0.31
Nodes (11): TestSearchReportsAndRemembersFailedServer(), ConfigHash(), TestIndexPersistAndHash(), NewTool(), newProxy(), runP(), TestProxyArgErrors(), TestProxyGating() (+3 more)

### Community 47 - "commands.go"
Cohesion: 0.16
Nodes (17): closingQuote(), ParseFrontmatter(), scalar(), TestFrontmatter(), TestFrontmatterBlockAndQuoteEdges(), TestFrontmatterBlockIndicatorsAndComments(), ExpandPrompt(), Prompt (+9 more)

### Community 48 - "Item"
Cohesion: 0.16
Nodes (5): newPager(), TestTeaAPI(), Item, Items, pagerModel

### Community 49 - "DataDir"
Cohesion: 0.33
Nodes (6): runLogout(), TestPathsHonourXDG(), ConfigDir(), ConfigFile(), DataDir(), home()

### Community 50 - "input_test.go"
Cohesion: 0.30
Nodes (11): NewInput(), TestCollapseUsesExactOffset(), TestCtrlC(), TestEditedMarkerSentLiterally(), TestHistory(), TestHistoryRecallChipsUnsafeContent(), TestMarkerNeverCollidesWithTypedText(), TestPasteChipKeepsContent() (+3 more)

### Community 51 - "retry_test.go"
Cohesion: 0.21
Nodes (18): Retry, DefaultRetryPolicy(), RetryNotice, fail(), TestMidStreamFailureResetsAndRetriesOnce(), TestMidStreamRetryBacksOffAndNotifies(), TestNoRetryOn400(), testPolicy() (+10 more)

### Community 52 - "builtin.go"
Cohesion: 0.53
Nodes (3): builtinHash(), ExtractBuiltins(), extractBuiltins()

### Community 53 - "Request"
Cohesion: 0.16
Nodes (15): Event, Request, anthropicOverflow(), openaiOverflow(), priorReasoningText(), replaysVerbatim(), toolInput(), ok() (+7 more)

### Community 54 - "AGENTS.md — moca"
Cohesion: 0.25
Nodes (7): AGENTS.md — moca, Commands, Conventions & gotchas, Docs, graphify, Ground truth (read before changing behavior), Layout & dependency rules

### Community 55 - "CostConfig"
Cohesion: 0.40
Nodes (5): ModelOverride, ProviderConfig, CostConfig, costM(), TestCostOf()

### Community 58 - "tui.go"
Cohesion: 0.14
Nodes (8): canonical(), LoadTrust(), TestTrustStore(), NewUnjailed(), TestUnjailedAndAllowAll(), AllowAll, TrustStore, Unjailed

### Community 59 - "parseArgs"
Cohesion: 0.50
Nodes (3): parseArgs(), TestParseArgs(), Options

### Community 60 - "pasteReader"
Cohesion: 0.67
Nodes (4): parsePasted(), pasteReader(), TestParsePasted(), pastedResult

### Community 64 - "Agent loop (§14: turn contract, steering, maxSteps wrap-up)"
Cohesion: 0.40
Nodes (4): Agent loop (§14: turn contract, steering, maxSteps wrap-up), Context manager compaction (budgets, cut points, structured summary, overflow recovery), Cross-provider thinking replay transform + NormalizeToolID, Session resume (--resume/--continue, repair, single-writer flock)

### Community 65 - ".Run"
Cohesion: 0.24
Nodes (6): globToRegexp(), ignored(), parseIgnore(), TestIgnore(), ignoreRule, searchTool

### Community 66 - "ReadTracker"
Cohesion: 0.47
Nodes (4): NewReadTracker(), stampOf(), ReadTracker, stamp

### Community 68 - "external-tools.md"
Cohesion: 0.39
Nodes (3): SKILL.md (builtin rtk), graphify (codebase knowledge graph), rtk (token-compressed CLI proxy)

### Community 70 - "Claude Code frontend-design skill"
Cohesion: 0.50
Nodes (3): AI-generated design default tells, frontend-design Apache 2.0 License, Claude Code frontend-design skill

### Community 71 - "pi mcp-scripting skill"
Cohesion: 0.67
Nodes (4): jev.evaluate reference, jev.evaluate semantic scripting gate, mcpScript tools API, pi mcp-scripting skill

### Community 80 - "format.go"
Cohesion: 0.14
Nodes (19): TestGitStatusUsesNoOptionalLocks(), TestStatusWideRunesNeverExceedWidth(), AbbrevEffort(), AbbrevHome(), FmtTokens(), FmtWindow(), GitBranch(), gitStatusArgs() (+11 more)

### Community 81 - "serverEnv"
Cohesion: 0.29
Nodes (8): TestResolveEnv(), ResolveEnv(), TestKeepNameWindows(), FilterEnv(), keepName(), serverEnv(), TestFilterEnv(), TestServerEnvExplicitEntryReplacesInherited()

### Community 83 - ".Compact"
Cohesion: 0.19
Nodes (8): Agent, summarizerUnavailable(), Budget, NewBudget(), TestBudget(), Prev, Summarizer, TextOf()

### Community 84 - "Input"
Cohesion: 0.19
Nodes (3): lineCount(), Input, paste

### Community 85 - "auth_test.go"
Cohesion: 0.25
Nodes (14): NewStore(), TestConcurrentRefreshOnce(), TestCredentialForLockWaitHonoursContext(), TestDeleteClearsCorruptStore(), TestHostIDStable(), TestInvalidClient(), TestNotLoggedIn(), TestRefreshIsTimeBounded() (+6 more)

### Community 89 - "fixture/main.go"
Cohesion: 0.21
Nodes (7): Sum(), TestTotals(), ParseList(), Average(), Max(), TestMaxAndAverage(), main()

### Community 97 - "2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)"
Cohesion: 0.15
Nodes (12): 1.1 Terms, 1.2 Client registration, 1.3 Decision, 1. anthropic — decision: `api_key only` (subscription OAuth not permitted), 2.1 Terms, 2.2 Client registration (per the docs), 2.3 Endpoints (verified live 2026-10-06 against `https://auth.openai.com/.well-known/openid-configuration`), 2.4 Inference contract for subscription traffic (open-source flow docs) (+4 more)

### Community 98 - "Usage"
Cohesion: 0.22
Nodes (5): TurnEnd, Result, StopReason, Usage, anthropicStop()

### Community 99 - "v0.1.0-alpha (2026-10-07)"
Cohesion: 0.18
Nodes (10): Changelog, CLI, Context & sessions, External tools & the skills ecosystem, MCP, OAuth (subscription login), Providers & protocols, Tools & permissions (+2 more)

### Community 100 - "runOneShot"
Cohesion: 0.27
Nodes (8): TestExitForOAuthSentinels(), exitFor(), firstLine(), resolveResume(), runOneShot(), decideTrust(), hasProjectResources(), runTUI()

### Community 101 - "prompt.go"
Cohesion: 0.14
Nodes (19): PromptInput, ServerLine, BuildSystemPrompt(), filterSkills(), oneLine(), TestBuildSystemPrompt(), TestFilterSkillsDropsMissingBuiltins(), TestPromptRTKIsConditional() (+11 more)

### Community 102 - "runLogin"
Cohesion: 0.22
Nodes (8): runLogin(), yesNo(), LineReader, NewLineReader(), Headless(), TestHeadlessDetectionSSH(), readLine(), lineResult

## Knowledge Gaps
- **75 isolated node(s):** `usageError`, `github.com/adeotek/moca`, `claudeServer`, `opencodeServer`, `request` (+70 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 208 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **17 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `MCP lazy proxy (§10.5)` connect `MCP lazy proxy (§10.5)` to `MCP call gating (readOnlyHint/destructiveHint, approve lists, allow-always persistence)`?**
  _High betweenness centrality (0.064) - this node is a cross-community bridge._
- **What connects `usageError`, `github.com/adeotek/moca`, `claudeServer` to the rest of the system?**
  _75 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `start.go` be split into smaller, more focused modules?**
  _Cohesion score 0.10403916768665851 - nodes in this community are weakly interconnected._
- **Why does `model` connect `model` to `start.go`, `Sanitize`, `Run`, `Item`, `format.go`, `Input`, `Agent`, `Start`, `session/entry.go`?**
  _High betweenness centrality (0.052) - this node is a cross-community bridge._
- **Should `newScript` be split into smaller, more focused modules?**
  _Cohesion score 0.08604038630377524 - nodes in this community are weakly interconnected._
- **Why does `moca SPECS.md (current implemented state)` connect `MCP lazy proxy (§10.5)` to `external-tools.md`?**
  _High betweenness centrality (0.044) - this node is a cross-community bridge._
- **Should `permissions/shell.go` be split into smaller, more focused modules?**
  _Cohesion score 0.07918552036199095 - nodes in this community are weakly interconnected._