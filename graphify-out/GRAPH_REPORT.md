# Graph Report - moca  (2026-10-10)

## Corpus Check
- 322 files · ~247,181 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 12 file(s) not represented in the graph (top: (none) 6, .jsonc 4, .v2 1)

## Summary
- 3103 nodes · 12003 edges · 165 communities (114 shown, 51 thin omitted)
- Extraction: 89% EXTRACTED · 11% INFERRED · 0% AMBIGUOUS · INFERRED: 1374 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `6f262764`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- start.go
- SPECS.md — moca implemented-state normative spec
- ollamaStart
- LineReader
- testing.T
- run
- permissions/shell.go
- newTestModel
- charm.land/bubbletea/v2.Cmd
- retry_test.go
- .Run
- update/update.go
- newAgentModel
- Parse
- newScript
- NewManager
- moca -p "<prompt>" — one-shot agent run (stdin -p form)
- Input
- anthropic_test.go
- Phase 2: seven tools + agent loop + permissions + skills loader + session schema
- TUI: Bubble Tea + lipgloss, compact/dense, welcome lines
- NewDefaultStore
- P2 — readability & feedback polish
- v0.1.0-alpha (2026-10-07): first release
- /-dropdown autocomplete (built-ins + prompt templates, ≤8 rows, tab/enter/esc)
- newFakeAS
- Model
- time.Duration
- Registry
- opencode-go provider (zen gateway)
- format.go
- v0.2.0-beta (2026-10-09): Ollama, saved / commands, self-update
- Agent
- cut_test.go
- Sanitize
- tools registry — Tool interface, seven frozen tools, sequential execution in emitted order
- pagerModel
- stdioTransport
- styleBlock
- Request
- Context manager: token-denominated budgets, compaction, recovery
- internal/agent — system prompt, events, loop, compaction + resume, cross-provider transform, Start wiring
- Manager
- model
- Snapshots
- Summarize
- .openSessions
- Seven frozen tools for v1 (schema churn = v2 discussion)
- credential store ~/.config/moca/auth.json (0600, atomic writes, cross-process lock)
- DESIGN §12 example config fixture (phase-1 decode gate)
- run
- post
- picker.go
- MCP lazy by design: 0 servers running at session start, ~200 prompt tokens
- moca update [--check] (newest published GitHub release, in-place swap)
- MCP gating — readOnlyHint && !destructiveHint || approve list || session allow; no name heuristics
- Create
- Summarizer injection — provider-free compaction over a neutral Entry view
- NewRegistry
- Config
- find.go
- Dynamic client registration: dynamic_agent_client → issued oaiapp_ id; no client secret
- go_pkg_fmt
- SOURCES.md
- Claude Code skill-creator skill
- renderMarkdown
- Architecture: package layout with strict import direction
- Plan: make moca work like a senior engineer — reliable, consistent, token-efficient
- applyEdit
- drained
- shipgate/fixture/main.go
- newModel
- system prompt — built once per run, stored in the session, never rebuilt
- prepare
- MCP Manager — zero start, lazy ensure, busy-guarded idle stop, replay-once rules
- moca login <provider> / TUI /login: credential wizard (OAuth or masked API key)
- 2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)
- runOneShot
- ToolSpec
- context.Context
- input_test.go
- v0.1.0-alpha (2026-10-07)
- Agent
- External tools: rtk, graphify, skills ecosystem
- .startSession
- overflowDir
- CI Workflow (gofmt · vet · build · test)
- Start
- parseIgnore
- Render
- web.go
- quick_validate.py
- NewUnjailed
- AGENTS.md — moca
- ResolveEnv
- create-command.md — starter prompt template
- 1.6 Resume replay + /resume picker
- runReg
- install.sh script
- 1.1 Markdown-lite rendering of responses
- fixture/user.go
- Claude Code frontend-design skill
- pi mcp-scripting skill
- history.go
- allowAll
- StartError
- 1.2 Inline diff preview for edit/write items
- fixture/slug.go
- AllowAll
- askRootChecker
- boundedWriter
- fakeCmds
- Never create a commit without the user's explicit approval
- Lookup
- evals/run.sh
- reference/codes.go
- Item
- bugfix/check.sh
- feature/check.sh
- largefile/check.sh
- plan/check.sh
- 2.3 Richer activity row
- compact.Budget.Trigger() as the red threshold
- history.go / history.jsonl store
- 2.2 One-row item lines
- OpenCode — comparison reference
- Arrange-Act-Assert with xUnit/NSubstitute
- OAuth login flow — dynamic registration, PKCE S256, loopback/headless, 30s per call
- response-cache.md
- refactor/check.sh
- Cache breakpoints: anthropic cache_control positions, ≤4 rolled forward each turn
- DESIGN revision 1: initial draft
- Parallel tool calls execute sequentially in emitted order (no cancellation of rest)
- Serialization for summarization: tool results truncated at 2K chars (COMPACTION NOTE lossy)
- 2.8 Echo !cmd like other input
- 2.5 Notify when unfocused (tui.notify: osc9/bell/off)
- Pi — comparison reference
- 2.7 Quieter first run (hints.json marker)
- 3.1 shift+tab cycles effort
- example.com/totals
- github.com/adeotek/moca
- .Run
- vague/check.sh
- web/check.sh
- README.md
- ToolCall
- example.com/codes
- LoadPrompts
- example.com/fetcher
- example.com/inventory
- TestWelcomeLines
- example.com/shop
- example.com/slug
- example.com/temp
- agent events — TextDelta … TurnEnd/SteeringApplied/Compacted/Resumed
- ReadTracker
- tui/trust.go
- ParseInput
- Eval baseline — DESIGN rev 21 harness pass
- system-prompt.md

## God Nodes (most connected - your core abstractions)
1. `SPECS.md — moca implemented-state normative spec` - 378 edges
2. `model` - 75 edges
3. `newTestModel()` - 63 edges
4. `newScript()` - 61 edges
5. `run()` - 61 edges
6. `key()` - 44 edges
7. `newAgentModel()` - 43 edges
8. `textTurn()` - 39 edges
9. `Config` - 37 edges
10. `Parse()` - 37 edges

## Surprising Connections (you probably didn't know these)
- `Entry point cmd/moca/main.go; run() is the testable core; sibling subcommand files` --references--> `run()`  [INFERRED]
  AGENTS.md → cmd/moca/main.go
- `Strict import direction + structural interfaces; seven frozen tools with golden schemas` --semantically_similar_to--> `Architecture: package layout with strict import direction`  [INFERRED] [semantically similar]
  AGENTS.md → docs/specs/DESIGN.md
- `Strict config decode: unknown keys rejected; new key = defaults + validation + SPECS update` --references--> `SPECS.md — moca implemented-state normative spec`  [INFERRED]
  AGENTS.md → docs/specs/SPECS.md
- `README non-goals (v1, probably forever)` --semantically_similar_to--> `v1 explicit non-goals: subagents, hooks, plan mode, LSP, binary plugins…`  [INFERRED] [semantically similar]
  README.md → docs/specs/DESIGN.md
- `DESIGN revision 17: /sessions picker + lock-guarded session.Delete` --semantically_similar_to--> `/sessions manager: list/switch/ctrl+d delete; single-writer delete refusal`  [INFERRED] [semantically similar]
  docs/specs/DESIGN.md → CHANGELOG.md

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

## Communities (165 total, 51 thin omitted)

### Community 0 - "start.go"
Cohesion: 0.09
Nodes (45): PlanChanged, StreamReset, TextDelta, ThinkingDelta, Warning, YoloChanged, DetectVerify(), fileContains() (+37 more)

### Community 1 - "SPECS.md — moca implemented-state normative spec"
Cohesion: 0.03
Nodes (95): cmd/moca/main.go (entry, signals, testable run()), cmd/moca/tui.go (TUI path + trust), SPECS.md — moca implemented-state normative spec, charm.land/bubbletea/v2 (TUI framework, with bubbles + lipgloss), /copy — OSC 52 copy of last assistant text, /cost — in/out/cache breakdown + $cost, /help — command listing, /undo — undo newest snapshot (+87 more)

### Community 2 - "ollamaStart"
Cohesion: 0.25
Nodes (8): ollamaServer(), ollamaStart(), TestOllamaDeclaredWindowIsQuiet(), TestOllamaReportedWindowIsQuiet(), TestOllamaStartErrors(), TestOllamaTinyWindowWarns(), TestStartWithOllamaModel(), abortingHTTPServer()

### Community 3 - "LineReader"
Cohesion: 0.29
Nodes (5): LineReader, NewLineReader(), TestLineReaderAbandonedReadKeepsLine(), readLine(), lineResult

### Community 4 - "testing.T"
Cohesion: 0.04
Nodes (74): TestModelCanReadGlobalSkillSupportFiles(), TestCarryModel(), TestToggleHard(), NormalizeToolID(), TestNormalizeToolID(), TestTransformDropsEmptiedAssistantAndMerges(), TestTransformDropsRedactedForeignThinking(), TestTransformHistory() (+66 more)

### Community 5 - "run"
Cohesion: 0.07
Nodes (51): fakeCompletions(), isolate(), TestExitCodes(), TestHelpListsSurface(), TestInterruptedExit130(), TestLoginLogoutCLI(), TestMCPImportCLI(), TestNoApproveSkipsProjectInstructions() (+43 more)

### Community 6 - "permissions/shell.go"
Cohesion: 0.06
Nodes (42): setup, canonical(), Jail, NewJail(), resolveDeep(), setup(), TestJail(), TestJailApprovable() (+34 more)

### Community 7 - "newTestModel"
Cohesion: 0.06
Nodes (59): TestActivityLine(), TestCoalescedShrinkRemembersTheLastFlushedRow(), TestInputBoxGrowsAndShrinks(), TestPagerCloseDefersTheRelease(), TestRefuseRunningWhileOutputInFlight(), TestShrinkIsPaddedUntilTheGuardWindowPasses(), TestShrinkPaddingFitsTheScreen(), TestSpinnerTicksOnlyForTheLiveRun() (+51 more)

### Community 8 - "charm.land/bubbletea/v2.Cmd"
Cohesion: 0.12
Nodes (7): ShellOutput, model, AbbrevEffort(), hintSeen(), markHint(), compactDoneMsg, shellDoneMsg

### Community 9 - "retry_test.go"
Cohesion: 0.44
Nodes (12): fail(), TestMidStreamFailureResetsAndRetriesOnce(), TestMidStreamRetryBacksOffAndNotifies(), TestNoRetryOn400(), testPolicy(), TestRetryAfterCapped(), TestRetryBackoffThenSuccess(), TestRetryGivesUpAfterFive() (+4 more)

### Community 10 - ".Run"
Cohesion: 0.08
Nodes (14): planAllows(), Env, isDocPath(), noteChange(), TestRepeatedFailureHintAndReset(), trackFailure(), askOutsideWrite(), CommandChecker (+6 more)

### Community 11 - "update/update.go"
Cohesion: 0.08
Nodes (35): envOr(), githubToken(), runUpdate(), Compare(), comparePre(), ExtractBinary(), FindAsset(), FindChecksums() (+27 more)

### Community 12 - "newAgentModel"
Cohesion: 0.06
Nodes (57): keyMsg(), TestExitCommandQuits(), TestCompactEscapeCancels(), TestCompactingBlocksRun(), TestCompactNothing(), TestEditItemShowsDiff(), TestCompactedAndResumedLines(), TestAllowAlwaysRequiresCtrl() (+49 more)

### Community 13 - "Parse"
Cohesion: 0.06
Nodes (45): scanner, span, Parse(), TestAPIKeyOptional(), TestDefaultsApplied(), TestEnvRefs(), TestOAuthAllowedForOpenAI(), TestOAuthRejectedForNonOAuthProvider() (+37 more)

### Community 14 - "newScript"
Cohesion: 0.06
Nodes (86): scriptServer, newScript(), startTest(), startTestWith(), startWith(), TestAbortWritesSyntheticResults(), TestFailedCallDoesNotCancelRest(), TestLengthStopTruncatedToolCall() (+78 more)

### Community 15 - "NewManager"
Cohesion: 0.11
Nodes (44): MCPConfig, build(), MCPServer, crashingServer(), shServer(), skipWindows(), TestCallTimeoutStopsWedgedServer(), TestHandshakeTimeout() (+36 more)

### Community 16 - "moca -p "<prompt>" — one-shot agent run (stdin -p form)"
Cohesion: 0.07
Nodes (36): cmd/moca/cli.go (flags → Options), cmd/moca/oneshot.go (-p agent path), Agent.Close — stops live MCP servers and closes the session writer, agent.Resume — validate first, repair, restore model/effort, jail at header workdir, /clear — new session file, old stays resumable, resets yolo to startup flag, /exit //q ///quit — quit (cancels in-flight runs), /resume [id8] — picker over this directory’s sessions, /sessions — session manager (delete via lock, open refuses deletion) (+28 more)

### Community 17 - "Input"
Cohesion: 0.12
Nodes (5): lineCount(), hasControl(), Input, padGuard, paste

### Community 18 - "anthropic_test.go"
Cohesion: 0.16
Nodes (37): Adapter, newAnthropic(), keyCred(), mustJSON(), sseServer(), TestAnthropicBodyHygiene(), TestAnthropicBudgetAndNoCache(), TestAnthropicBudgetFollowsEffort() (+29 more)

### Community 19 - "Phase 2: seven tools + agent loop + permissions + skills loader + session schema"
Cohesion: 0.08
Nodes (34): graphify workflow rules: query/path/explain first, update . after code changes, Failing-test investigation protocol: edit refuses until read+search happened, ! / !! shell prefixes: user-initiated, bypass model-facing analysis, Per-provider baseUrl override + custom providers (vLLM/LM Studio/Ollama day 1), Built-in rtk skill (embed.FS, extracted to ~/.local/share/moca/builtin-skills/<version>), Edit-ladder tests (phase 2, non-negotiable: exact/ambiguous/replace_all/CRLF/BOM…), graphify: codebase-graph CLI (default allowlist; SKILL.md drop-in), MCP server roster: one line per configured server in the system prompt (+26 more)

### Community 20 - "TUI: Bubble Tea + lipgloss, compact/dense, welcome lines"
Cohesion: 0.15
Nodes (21): Approvals show the whole command (≤6 lines) + ctrl+o pages all of it, Persistent prompt history (history.jsonl per workdir) + ctrl+r search, Markdown-lite response rendering (bold/code/headings/bullets/fenced blocks), Context pressure: % yellow at 70%, red where auto-compaction fires, Bracketed paste: verbatim, never auto-sent, [paste N lines] chip (alt+p), /copy: last assistant message to clipboard via OSC 52 (SSH-safe), Pager: ctrl+o (latest item) / /show <n>; alt+t for thinking, permission_mode audit entry {yolo}; yolo never restored on resume (+13 more)

### Community 21 - "NewDefaultStore"
Cohesion: 0.12
Nodes (12): NewDefaultStore(), OAuthProvider(), model, providerNames(), providerRow(), TestLoginRowForOllama(), loginChoice, loginDoneMsg (+4 more)

### Community 22 - "P2 — readability & feedback polish"
Cohesion: 0.06
Nodes (29): 0. Fixed in this pass (already on `fix/polish`), 1.1 Markdown-lite rendering of responses — **M**, 1.2 Inline diff preview for `edit`/`write` items — **S/M**, 1.3 Context-pressure color in the status bar — **S**, 1.4 Argument completion in the `/` dropdown + interactive `/model` — **M**, 1.5 `@path` file mentions — **M**, 1.6 Resume shows where you left off; `/resume` picker — **M**, 1.7 Persistent input history + `ctrl+r` search — **S/M** (+21 more)

### Community 23 - "v0.1.0-alpha (2026-10-07): first release"
Cohesion: 0.12
Nodes (26): Default HTTP client bounds response-header wait at 120s, opencode-go [1210] server_error flake retried even on HTTP 400, v0.1.0-alpha (2026-10-07): first release, anthropic-messages protocol adapter (~150 LOC, hand-rolled HTTP+SSE), anthropic provider (api_key only; subscription OAuth not permitted), Mid-stream failure: no silent retry, discard partial, one retry from same request, openai-completions protocol adapter, openai provider (API key or ChatGPT subscription OAuth) (+18 more)

### Community 24 - "/-dropdown autocomplete (built-ins + prompt templates, ≤8 rows, tab/enter/esc)"
Cohesion: 0.09
Nodes (23): activity row ⠋ working… (spinner, elapsed, esc to interrupt), Agent.Status() — model, effort, anchored tokens, window, usage, cost, Sub/Hard/Yolo, @path completion (≤5000 indexed files, ranked, text sent as written), /effort [level] (also shift+tab wrap; refused while running), /hard — modelHard + high ↔ saved pair, /show <n> — open any item in the pager, usage-anchored context estimate — anchor + delta after each turn, agent control surface — SetModel/SetEffort/ToggleHard/Models/Status/Steer/AddNote/Undo (mutex-guarded) (+15 more)

### Community 25 - "newFakeAS"
Cohesion: 0.06
Nodes (42): fakeSIWC(), fakeSIWCOpts(), Store, newHostID(), validateAPIKey(), audContains(), b64d(), OAuthConfig (+34 more)

### Community 26 - "Model"
Cohesion: 0.08
Nodes (8): savedModel, Status, Agent, Effort, ParseEffort(), TestParseEffort(), Model, supportedEfforts()

### Community 27 - "time.Duration"
Cohesion: 0.15
Nodes (12): Retry, DefaultRetryPolicy(), RetryNotice, TestFmtElapsed(), fmtElapsed(), fmtThinkDuration(), Options, HTTPError (+4 more)

### Community 28 - "Registry"
Cohesion: 0.26
Nodes (5): discoverLocal(), SplitModel(), TestSplitModel(), Registry, CredentialFunc

### Community 29 - "opencode-go provider (zen gateway)"
Cohesion: 0.05
Nodes (54): DESIGN.md — v1 vision/contract (doc/specs), opencode.ai Go-tier docs (MissingSessionID, /zen/v1 exemption), level map anthropicAdaptive {low,medium,high,max} (off/minimal clamp up), anthropic stream flow (message_start/content_block_*/message_delta/stop), level map basicThinking {low,medium,high} (gateway rejects off/minimal: kimi-k2.7-code, mimo-v2.6-*), anthropic cache breakpoints — system end + last tool + last eligible block, openai-completions thinking — reasoning_content; history thinking never sent, cross-provider transform — thinking replay bound by Model == request model (+46 more)

### Community 30 - "format.go"
Cohesion: 0.14
Nodes (19): TestStatusWideRunesNeverExceedWidth(), AbbrevHome(), FmtTokens(), FmtWindow(), TestFormatters(), trimZero(), TestStatusLineCarriesTheSetupState(), FmtPercent() (+11 more)

### Community 31 - "v0.2.0-beta (2026-10-09): Ollama, saved / commands, self-update"
Cohesion: 0.14
Nodes (25): Release workflow asset contract with internal/update; install.sh/install.ps1 consumers, Ollama fail-fast: server down says how to start; unpulled model says ollama pull, --resume/--continue replay last 3 turns; new /resume picker, v0.2.0-beta (2026-10-09): Ollama, saved / commands, self-update, Built-in & user-extensible model catalog (window, max out, cost, thinkingMode), /clear starts a new session file (old one stays resumable), Provider-qualified model ids (opencode-go/glm-5.3-flash), Resume: moca --resume <id8|last> / --continue (workdir→session mapping) (+17 more)

### Community 32 - "Agent"
Cohesion: 0.07
Nodes (28): Options, TurnEnd, Result, Agent, Outcome, New(), userText(), verifyNudge() (+20 more)

### Community 33 - "cut_test.go"
Cohesion: 0.07
Nodes (43): Kind, Agent, summarizerUnavailable(), callKey(), Agent, staleResults(), TestStaleResults(), Budget (+35 more)

### Community 34 - "Sanitize"
Cohesion: 0.08
Nodes (20): AutoAllow(), Answer, Question, approvalPanel(), approvalPrompt(), firstLineOf(), TestApprovalPromptSanitized(), diffPreview() (+12 more)

### Community 35 - "tools registry — Tool interface, seven frozen tools, sequential execution in emitted order"
Cohesion: 0.11
Nodes (21): Jail.Approvable — resolves refused writes to the ask-write root, ask-write root ~/.config/moca/prompts (write/edit ask, never persistent), !cmd / !!cmd user-initiated shell (30 s, ShellEnv, 30K truncation; esc kills), config.EnvRefs — every referenced variable (shell hygiene input), edit ladder — exact match → replace_all → whitespace-insensitive re-indent → error, frozen tool schemas — golden schemas.golden.json (v2 discussion to change), inline edit diffs — changed lines indented, ≤8 rows then N more, mcp proxy tool — byte-identical MCPSpec, search/describe/call rendering, 30K truncate (+13 more)

### Community 36 - "pagerModel"
Cohesion: 0.18
Nodes (4): TestMatchRanges(), matchRanges(), newPager(), pagerModel

### Community 37 - "stdioTransport"
Cohesion: 0.13
Nodes (9): callResult, KillProcessGroup(), SetProcessGroup(), KillProcessGroup(), SetProcessGroup(), response, ring, rpcError (+1 more)

### Community 38 - "styleBlock"
Cohesion: 0.31
Nodes (10): respMsgStyle(), styleBlock(), styleBlockLead(), TestCommandEchoesAsUserMessage(), TestMessageBackgroundsCoverTheFullRow(), TestResponseCommitUsesBackground(), TestScrollbackMessageBackgrounds(), TestStyleBlockExpandsTabs() (+2 more)

### Community 39 - "Request"
Cohesion: 0.11
Nodes (21): ContentBlock, Event, Request, ToolResult, anthropicOverflow(), isDialFailure(), ollamaUnreachable(), openaiOverflow() (+13 more)

### Community 40 - "Context manager: token-denominated budgets, compaction, recovery"
Cohesion: 0.14
Nodes (23): Thinking blocks/signatures model-bound; cross-provider transforms in agent (SPECS §14), /sessions manager: list/switch/ctrl+d delete; single-writer delete refusal, JSONL sessions: single-writer flock, 0600, compaction entry (summary, firstKeptEntryId, tokensBefore, usage), Compaction loop guard: stop with error if still above trigger, Context manager: token-denominated budgets, compaction, recovery, Cross-provider history transform (thinking bound to producing model, tool-id normalization), -p exit codes: 0 completed · 1 runtime · 2 config/usage · 3 maxSteps · 130 interrupted (+15 more)

### Community 41 - "internal/agent — system prompt, events, loop, compaction + resume, cross-provider transform, Start wiring"
Cohesion: 0.15
Nodes (19): agent build() — registry, jail, checkers, env, snapshots, MCP manager, agent (shared with Resume), /compact — manual compaction on cancellable context (compacting… transient), cmd/moca entrypoint package, compact.Budget — window/reserve/keepRecent, small windows scale to window/4, context pressure coloring — yellow 70%, red at trigger from compact.Budget.Trigger(), internal/agent — system prompt, events, loop, compaction + resume, cross-provider transform, Start wiring, internal/compact — budgets, cut points, lossy serialization, structured summaries (no provider import), internal/config — paths, JSONC pre-pass, typed config + validation, env indirection (+11 more)

### Community 42 - "Manager"
Cohesion: 0.13
Nodes (9): Rank(), TestRank(), words(), Manager, Hit, Index, IndexEntry, state (+1 more)

### Community 43 - "model"
Cohesion: 0.18
Nodes (11): model, matchCommands(), parseSlash(), slashQuery(), TestSlashQuery(), mentionMatches(), rankFile(), TestMentionRanking() (+3 more)

### Community 44 - "Snapshots"
Cohesion: 0.29
Nodes (4): gitClean(), Snapshots, NewSnapshots(), SnapshotRec

### Community 45 - "Summarize"
Cohesion: 0.15
Nodes (15): Metrics, Collect(), fatal(), main(), meanSD(), readResults(), Summarize(), TestCollect() (+7 more)

### Community 46 - ".openSessions"
Cohesion: 0.19
Nodes (11): Delete(), TestDelete(), TestListAll(), Info, List(), ListAll(), TestListPreviewsAndFilters(), fmtAge() (+3 more)

### Community 47 - "Seven frozen tools for v1 (schema churn = v2 discussion)"
Cohesion: 0.11
Nodes (18): Seven frozen tools; schemas golden in internal/tools/testdata/schemas.golden.json, Inline edit diffs: changed lines (red/green, ≤8) under the item line, edit tool: path + old_string/new_string + replace_all, ls tool: one level, dirs suffixed /, hidden opt-in, mvdan.cc/sh/v3/syntax — the single non-UI third-party dependency, Read-before-write + unchanged-on-disk guard (mtime + size + hash), Output redirection targets must pass the path jail (literal targets only), DESIGN revision 9: trailing-comma fix, read-only roots, mcp describe, shell stateless (+10 more)

### Community 48 - "credential store ~/.config/moca/auth.json (0600, atomic writes, cross-process lock)"
Cohesion: 0.12
Nodes (18): credential store ~/.config/moca/auth.json (0600, atomic writes, cross-process lock), Registry.CheckCredential — network-free login check (/model, resume), /model [id] — cross-provider switch (refused while running, forfeits prompt cache), config.SetObjectEntry — insert key: rawJSON (mcp import), config.SetString — replace a string in place, last-duplicate wins, non-string refused, mcp.DiscoverSources — claude/opencode/pi config discovery paths, moca login --api-key (prompt for a key instead of OAuth), moca login --no-browser (headless OAuth: URL + pasted code) (+10 more)

### Community 49 - "DESIGN §12 example config fixture (phase-1 decode gate)"
Cohesion: 0.16
Nodes (20): config.Standardize preserves byte offsets; example.jsonc is byte-exact DESIGN §12 fixture, Secrets: apiKey must be env:VAR; .env and *.key gitignored, never committed, Strict config decode: unknown keys rejected; new key = defaults + validation + SPECS update, Argument completion for /model, /effort, /login, /logout, /show, /resume, api_key auth mode (stored credential or env: reference), DESIGN §12 example config fixture (phase-1 decode gate), Config: ~/.config/moca/config.jsonc, typed struct, fail-fast validation, Effort clamping to nearest supported level at/below requested (+12 more)

### Community 50 - "run"
Cohesion: 0.24
Nodes (13): Entry point cmd/moca/main.go; run() is the testable core; sibling subcommand files, printUsage(), loginAPIKey(), runLogin(), runLogout(), yesNo(), run(), confirm() (+5 more)

### Community 51 - "post"
Cohesion: 0.10
Nodes (17): fakeHTTPServer(), TestHTTPTransport(), post(), newHTTPError(), retryAfter(), Registry, modelFromShow(), ollamaBaseURL() (+9 more)

### Community 52 - "picker.go"
Cohesion: 0.18
Nodes (8): modelPickItems(), choiceRow(), masked(), TestWindowRange(), model, windowRange(), pickItem, pickState

### Community 53 - "MCP lazy by design: 0 servers running at session start, ~200 prompt tokens"
Cohesion: 0.35
Nodes (12): MCP call gating: default-deny via readOnlyHint annotations / per-server approve lists, mcp config block: idleTimeout scalar beside mcp.servers map + approve lists, Persisted MCP discovery index (~/.local/share/moca/mcp-index.json, configHash-invalidated), MCP env hygiene: filtered subprocess env (PATH/HOME/USER/LANG/TERM/TMPDIR/XDG_*), moca mcp import (Claude Code/OpenCode/Pi configs; secrets rewritten to env:) + mcp index, MCP lazy lifecycle: start on first describe/call, stop after mcp.idleTimeout (600s), Path jail does NOT apply to MCP tools (server tool defines its own scope), mcp tool: lazy MCP proxy (search|describe|call, ~200-token schema) (+4 more)

### Community 54 - "moca update [--check] (newest published GitHub release, in-place swap)"
Cohesion: 0.15
Nodes (17): cmd/moca/update.go (moca update subcommand), checksums.txt sha256 verification (missing warns; mismatch refuses), moca update --check flag (report only, never writes), git describe version stamp (-ldflags -X config.Version), install-script gate — fresh/update/mismatch refusals for install.sh + functional pwsh run, internal/update — GitHub release fetch, semver compare, checksum verify, atomic replace, make build (bin/moca, version-stamped from git describe), make release (dist/ cross-compiled, darwin included) (+9 more)

### Community 55 - "MCP gating — readOnlyHint && !destructiveHint || approve list || session allow; no name heuristics"
Cohesion: 0.12
Nodes (17): TUI approval prompt (allow <cmd>? a once / ctrl+a always / d deny; 700 ms typing pause; esc denies), config.AppendString — comment-preserving array append (allow lists), 32-entry shell.allow default list (DESIGN §12), hard-deny class — sudo/doas/dd/shred/chown/mkfs* (+ su), never overridable, config key: mcp.servers.<name>.approve (allow list, "*" trusts the server), MCP gating — readOnlyHint && !destructiveHint || approve list || session allow; no name heuristics, mvdan.cc/sh/v3 (only non-UI dependency, shell parsing), phase-5 live legs — real model drives mcp search/describe/call + gating (+9 more)

### Community 56 - "Create"
Cohesion: 0.15
Nodes (14): TestFindForWorkdirViaSymlink(), TestInWorkdir(), lockFile(), lockFile(), Create(), Writer, lastNewline(), newID() (+6 more)

### Community 57 - "Summarizer injection — provider-free compaction over a neutral Entry view"
Cohesion: 0.33
Nodes (7): compaction entry — summary, firstKeptEntryId, tokensBefore, usage, cumulative read/modified lists, llm.Message, NoCacheWrite — compaction strips every cache_control, session.Messages rebuild — assistant batches by message id, merged user messages, skip non-conversation, Summarizer injection — provider-free compaction over a neutral Entry view, structured summary — Goal/Constraints/Progress/Key Decisions/Next Steps/Critical Context + cumulative file lists, summary request shape — cheap model, effort minimal, NoCacheWrite, max_tokens ≤4096, payload capped 3 chars/token

### Community 58 - "NewRegistry"
Cohesion: 0.13
Nodes (28): newFakeOllama(), ollamaRegistry(), TestNoOllamaNoProbe(), TestOllamaChatWithoutKey(), TestOllamaDiscovery(), TestOllamaDiscoveryVerifyConcurrent(), TestOllamaKeylessAndOptionalKey(), TestOllamaServerDown() (+20 more)

### Community 59 - "Config"
Cohesion: 0.17
Nodes (10): ContextConfig, ShellConfig, SnapshotConfig, TUIConfig, Default(), Config, ProviderConfig, IsLocalProvider() (+2 more)

### Community 60 - "find.go"
Cohesion: 0.28
Nodes (14): canonical(), Find(), FindForWorkdir(), header(), headerAndPreview(), idOf(), InWorkdir(), list() (+6 more)

### Community 61 - "Dynamic client registration: dynamic_agent_client → issued oaiapp_ id; no client secret"
Cohesion: 0.33
Nodes (6): Dynamic client registration: dynamic_agent_client → issued oaiapp_ id; no client secret, ext_agent_host_id: stable opaque per-host identity (not a credential), Live probes 2026-10-06: discovery/JWKS/token endpoint results; interactive flow via phase-7 live gate, Verified OpenAI OAuth endpoints, PKCE S256, loopback 127.0.0.1 callback, JWKS, Sign in with ChatGPT open-source token sharing: permitted for open-source tools, Rotating 30-day refresh tokens; refreshes serialized across processes

### Community 62 - "go_pkg_fmt"
Cohesion: 0.07
Nodes (60): Compacted, Resumed, SteeringApplied, EnvError, ModelOverride, WebConfig, WebSearchConfig, Client (+52 more)

### Community 63 - "SOURCES.md"
Cohesion: 0.19
Nodes (7): internal/skills/testdata/ecosystem corpus, anthropics/skills frontend-design, anthropics/skills skill-creator, graphify skill (graphify install, claude platform; byte-identical to pi variant), adeotek/dotfiles opencode dotnet-unit-testing skill, npm pi-mcp-adapter mcp-scripting skill, npm pi-subagents skill

### Community 64 - "Claude Code skill-creator skill"
Cohesion: 0.16
Nodes (13): Skill description (trigger) optimization, Skill eval/iterate loop (with-skill vs baseline), skill-creator Apache 2.0 License (Anthropic), Claude Code skill-creator skill, SKILL.md anatomy + bundled resources layout, Constrained query expansion against graph vocab, Progressive Disclosure (three-level skill loading), graphify query/path/explain reference (+5 more)

### Community 65 - "renderMarkdown"
Cohesion: 0.23
Nodes (12): codeBandStyle(), fenceLang(), renderInline(), renderMarkdown(), plainMD(), TestMarkdownBlocks(), TestMarkdownFence(), TestMarkdownSanitizes() (+4 more)

### Community 66 - "Architecture: package layout with strict import direction"
Cohesion: 0.21
Nodes (12): Strict import direction + structural interfaces; seven frozen tools with golden schemas, internal/agent: the agent loop (user msg → model → tools → results), internal/compact: token accounting, truncation, summarization, cache markers, internal/config: single JSONC config, typed, validated, internal/llm: leaf Message/ContentBlock/ToolCall/Usage/Request types, internal/mcp: lazy MCP proxy (stdio + streamable HTTP, discovery index, lifecycle), internal/permissions: path jail, shell command analysis + allowlist, project trust, internal/provider: protocol adapters, provider configs, model catalog, auth, retry (+4 more)

### Community 67 - "Plan: make moca work like a senior engineer — reliable, consistent, token-efficient"
Cohesion: 0.15
Nodes (12): Current state (audited 2026-10-09, branch `chore/polish`), Goal, Out of scope, Outcome (2026-10-09), Phase A — measurement (do this first; everything else is judged by it), Phase B — token efficiency (cheapest wins first), Phase C — reliability (move "please do X" from the prompt into the harness), Phase D — senior-engineer working style (prompt, judged by evals) (+4 more)

### Community 68 - "applyEdit"
Cohesion: 0.23
Nodes (14): applyEdit(), formatDiff(), indentOf(), joinInts(), lineOf(), matchLines(), replaceExact(), replaceFuzzy() (+6 more)

### Community 69 - "drained"
Cohesion: 0.35
Nodes (11): TestThinkingLinePrintsBeforeTheResponse(), drained(), newLoginModel(), TestLoginMarkedBackAndCancel(), TestLoginOAuthDeclinedFlipAndCancel(), TestLoginOAuthFlowAndAuthFlip(), TestLoginRefusedWhileRunning(), TestLoginStoresAPIKey() (+3 more)

### Community 70 - "shipgate/fixture/main.go"
Cohesion: 0.22
Nodes (7): Sum(), TestTotals(), ParseList(), Average(), Max(), TestMaxAndAverage(), main()

### Community 71 - "newModel"
Cohesion: 0.11
Nodes (16): Event, newModel(), Run(), newAsker(), newEventPipe(), TestEventPipeBuffersUntilStarted(), TestEventPipeNonBlockingAndFIFO(), TestPipeSendKeepsOrderWithEvents() (+8 more)

### Community 72 - "system prompt — built once per run, stored in the session, never rebuilt"
Cohesion: 0.17
Nodes (13): AGENTS.md — project instructions file, builtin skills — embed, ExtractBuiltins into content-hash dirs (currently rtk), CLAUDE.md fallback instructions file, vendored ecosystem corpus internal/skills/testdata/ecosystem (pi/claude/opencode/graphify skills), ExtractBuiltins(dataDir) — temp dir + rename, race-safe, agent.filterSkills — omit builtins whose CLI is missing from PATH, LoadInstructions — global AGENTS.md, workdir AGENTS.md (trusted) else CLAUDE.md, 32K cap, phase-6 live legs — sharpened rtk description drives rtk test, graphify answer (+5 more)

### Community 73 - "prepare"
Cohesion: 0.18
Nodes (16): TestExitForOAuthSentinels(), main(), exitFor(), decideTrust(), hasProjectResources(), runTUI(), TestDefaultHTTPClientBoundsHeaderWait(), defaultHTTPClient() (+8 more)

### Community 74 - "MCP Manager — zero start, lazy ensure, busy-guarded idle stop, replay-once rules"
Cohesion: 0.15
Nodes (12): config key: mcp.servers.<name> (command|url, args, env, headers), errNotSent — request never left: replay-safe, errTransportDead sentinel (errors.Is-visible), MCP streamable HTTP transport — Mcp-Session-Id echo, SSE responses, 15m client, persisted MCP index mcp-index.json (configHash, no schemas, merged saves), JSON-RPC 2.0 core — id routing, ping, paged tools/list, transports interface, MCP Manager — zero start, lazy ensure, busy-guarded idle stop, replay-once rules, MCP stdio transport — line-wise, 4KiB ring, exit drain ≤500ms, dead-sentinel errors (+4 more)

### Community 75 - "moca login <provider> / TUI /login: credential wizard (OAuth or masked API key)"
Cohesion: 0.25
Nodes (15): auth.json moved to config dir; v0.1 store auto-migrated, +10 opencode-go catalog models (deepseek v4.1, kimi-k2.7-code, mimo-v2.6, qwen3.x…), v0.1.1 (2026-10-08): polish + credential store + autocomplete, moca CLI surface: TUI, -p, login/logout, mcp import/index, --version (§12.5), moca login <provider> / TUI /login: credential wizard (OAuth or masked API key), moca logout <provider> / TUI /logout: clear a stored credential, oauth auth mode (moca login; auth.json 0600, auto-refresh), moca -p one-shot mode (prompt also via stdin, -p -) (+7 more)

### Community 76 - "2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)"
Cohesion: 0.17
Nodes (12): 1.1 Terms, 1.2 Client registration, 1.3 Decision, 1. anthropic — decision: `api_key only` (subscription OAuth not permitted), 2.1 Terms, 2.2 Client registration (per the docs), 2.3 Endpoints (verified live 2026-10-06 against `https://auth.openai.com/.well-known/openid-configuration`), 2.4 Inference contract for subscription traffic (open-source flow docs) (+4 more)

### Community 77 - "runOneShot"
Cohesion: 0.21
Nodes (10): parseArgs(), TestParseArgs(), TestParseArgsDo(), firstLine(), oneShotSlug(), resolveResume(), runOneShot(), Slug() (+2 more)

### Community 78 - "ToolSpec"
Cohesion: 0.07
Nodes (16): ToolSpec, Builtins(), MCPSpec(), TestRegistryRecoversToolPanic(), Registry, NewRegistry(), TestRegistryDispatch(), TestSchemasFrozen() (+8 more)

### Community 79 - "context.Context"
Cohesion: 0.08
Nodes (22): fakeSum, CallResult, initialize(), TestHTTPCloseBounded(), startHTTP(), TestHTTPNotifyStatusChecked(), responseID(), TestResponseID() (+14 more)

### Community 80 - "input_test.go"
Cohesion: 0.30
Nodes (11): NewInput(), TestCollapseUsesExactOffset(), TestCtrlC(), TestEditedMarkerSentLiterally(), TestHistory(), TestHistoryRecallChipsUnsafeContent(), TestMarkerNeverCollidesWithTypedText(), TestPasteChipKeepsContent() (+3 more)

### Community 81 - "v0.1.0-alpha (2026-10-07)"
Cohesion: 0.18
Nodes (10): CHANGELOG, CLI, Context & sessions, External tools & the skills ecosystem, MCP, OAuth (subscription login), Providers & protocols, Tools & permissions (+2 more)

### Community 83 - "External tools: rtk, graphify, skills ecosystem"
Cohesion: 0.14
Nodes (12): Docs map: external-tools.md, oauth-verification.md; deleted docs/plans & docs/reviews, docs/plans/phase-<n>-*.md task-level plans (deleted; cited as history), moca — MO Coding Agent (binary = repo = module path), External tools: rtk, graphify, skills ecosystem, graphify — codebase knowledge graph integration, internal/permissions/rtk.go — captured rtk subcommand classes, rtk — token-compressed CLI proxy, Cross-tool skills and prompt templates ecosystem (+4 more)

### Community 84 - ".startSession"
Cohesion: 0.29
Nodes (4): TestPathsHonourXDG(), ConfigFile(), model, noProviderNotice()

### Community 85 - "overflowDir"
Cohesion: 0.60
Nodes (5): TestPruneOverflow(), TestSpillDirsAreRandom(), newSpillDir(), overflowDir(), pruneOverflow()

### Community 86 - "CI Workflow (gofmt · vet · build · test)"
Cohesion: 0.22
Nodes (9): CI Workflow (gofmt · vet · build · test), CI gate set (gofmt, go vet, go build, go test -race), go.mod (Go version source for setup-go), SPECS.md §2 — CI gate definition, CI gate: gofmt, go vet, go build, go test -race (.github/workflows/ci.yml), make build/test/vet/fmt/release; single-package test invocation, mise provisions Go 1.27.1 (pinned in go.mod); make falls back to mise x go, Ship gate: make build && bash test/shipgate/run.sh (//go:build shipgate tag) (+1 more)

### Community 87 - "Start"
Cohesion: 0.11
Nodes (19): PromptInput, ServerLine, planMode(), BuildSystemPrompt(), GitState(), oneLine(), Platform(), renderPrompt() (+11 more)

### Community 88 - "parseIgnore"
Cohesion: 0.18
Nodes (10): globToRegexp(), ignored(), parseIgnore(), TestIgnore(), ListFiles(), TestListFiles(), model, mentionQuery() (+2 more)

### Community 89 - "Render"
Cohesion: 0.18
Nodes (7): jsonItem, main(), Render(), TestRenderTotal(), main(), RenderJSON(), TestRenderJSON()

### Community 90 - "web.go"
Cohesion: 0.10
Nodes (22): Spill(), decode(), errorf(), cutWeb(), isAlnumByte(), isSpaceByte(), TestWebHTMLConverters(), webClean() (+14 more)

### Community 92 - "NewUnjailed"
Cohesion: 0.29
Nodes (4): Agent, NewUnjailed(), TestUnjailedAndAllowAll(), Unjailed

### Community 93 - "AGENTS.md — moca"
Cohesion: 0.25
Nodes (7): AGENTS.md — moca, Commands, Conventions & gotchas, Docs, graphify, Ground truth (read before changing behavior), Layout & dependency rules

### Community 94 - "ResolveEnv"
Cohesion: 0.25
Nodes (9): webSearchKey(), TestResolveEnv(), ResolveEnv(), TestKeepNameWindows(), FilterEnv(), keepName(), serverEnv(), TestFilterEnv() (+1 more)

### Community 95 - "create-command.md — starter prompt template"
Cohesion: 0.33
Nodes (3): create-command.md — starter prompt template, Project commands dir .moca/prompts/, Personal commands dir {{prompts_dir}} (~/.config/moca/prompts/)

### Community 96 - "1.6 Resume replay + /resume picker"
Cohesion: 0.33
Nodes (6): agent package (Resume, History, SetEffort), 1.4 Argument completion in the / dropdown + interactive /model, /model picker marks (no key) via CheckCredential, picker type shared by login /model /resume, 1.6 Resume replay + /resume picker, session package (List, FindForWorkdir)

### Community 98 - "install.sh script"
Cohesion: 0.67
Nodes (5): die(), fetch(), note(), install.sh script, usage()

### Community 99 - "1.1 Markdown-lite rendering of responses"
Cohesion: 0.40
Nodes (4): ansi.Wrap (terminal ANSI wrapping), ansi.Wrap span re-open fix (targeted toggles), 1.1 Markdown-lite rendering of responses, styleBlock (pre-styled spans variant)

### Community 100 - "fixture/user.go"
Cohesion: 0.40
Nodes (4): User, TestOrderEmail(), TestUserEmail(), NewUser()

### Community 101 - "Claude Code frontend-design skill"
Cohesion: 0.50
Nodes (3): AI-generated design default tells, frontend-design Apache 2.0 License, Claude Code frontend-design skill

### Community 102 - "pi mcp-scripting skill"
Cohesion: 0.67
Nodes (4): jev.evaluate reference, jev.evaluate semantic scripting gate, mcpScript tools API, pi mcp-scripting skill

### Community 103 - "history.go"
Cohesion: 0.53
Nodes (5): appendHistory(), loadHistory(), rewriteHistory(), TestHistoryPersistence(), historyRec

### Community 106 - "1.2 Inline diff preview for edit/write items"
Cohesion: 0.67
Nodes (3): Claude Code — comparison reference, diffPreview(detail, width, maxLines), 1.2 Inline diff preview for edit/write items

### Community 107 - "fixture/slug.go"
Cohesion: 0.40
Nodes (4): Limit(), Make(), TestLimit(), TestMake()

### Community 113 - "Lookup"
Cohesion: 0.50
Nodes (3): Lookup(), TestLookupBounds(), Valid()

### Community 114 - "evals/run.sh"
Cohesion: 1.00
Nodes (3): fresh_repo(), one(), run.sh script

### Community 129 - "OAuth login flow — dynamic registration, PKCE S256, loopback/headless, 30s per call"
Cohesion: 0.15
Nodes (14): agent.StartError (run setup failure → exit 1), required OAuth scope chatgpt.tokens.use.direct, config.EnvError (unset env:VAR, first-request, exit 2), built-in defaults: anthropic/opencode-go/openai with env:ANTHROPIC_API_KEY/env:OPENCODE_API_KEY/env:OPENAI_API_KEY, env:VAR indirection — lazily resolved per request, provider.ErrInvalidClient (OAuth refusal → exit 2), provider.ErrInvalidGrant (OAuth refusal → exit 2 with login hint), moca exit-code contract 0/1/2/3/130 (SIGINT and SIGTERM both cancel) (+6 more)

### Community 146 - ".Run"
Cohesion: 0.11
Nodes (16): TestVerifies(), verifies(), approve(), findTestFile(), isPathByte(), TestFindTestFile(), TestTestRunCommand(), TestTestRunFailed() (+8 more)

### Community 152 - "ToolCall"
Cohesion: 0.10
Nodes (24): ToolEnd, ToolStart, ToolCall, TestProxyPlanModeCallFollowsNormalGating(), cutRunes(), errResult(), isTrue(), newProxy() (+16 more)

### Community 154 - "LoadPrompts"
Cohesion: 0.08
Nodes (29): filterSkills(), TestFilterSkillsDropsMissingBuiltins(), builtinHash(), ExtractBuiltins(), extractBuiltins(), TestEcosystemSkillsLoadUnchanged(), closingQuote(), ParseFrontmatter() (+21 more)

### Community 163 - "agent events — TextDelta … TurnEnd/SteeringApplied/Compacted/Resumed"
Cohesion: 0.22
Nodes (9): agent events — TextDelta … TurnEnd/SteeringApplied/Compacted/Resumed, agent loop — sequential calls, steering drains, length-turn handling, exit on no-tool answer, eventPipe — FIFO pipe bridging agent events into Bubble Tea, llm.Request, NormalizeToolID — deterministic ids on use and result, phase-3 gate — scripted tmux TUI session (status bar, steering, approvals, chips), resume replay — agent.Resumed banner + last 3 user turns replayed, runDoneMsg — completion rides the event pipe, cannot overtake a delta (+1 more)

### Community 167 - "ReadTracker"
Cohesion: 0.60
Nodes (3): stampOf(), ReadTracker, stamp

### Community 170 - "ParseInput"
Cohesion: 0.32
Nodes (8): Prompt, HelpText(), ParseInput(), TestParseInput(), TestPromptDirsReloadAtRunEnd(), TestLoginCommandParsingAndHelp(), InputKind, Parsed

### Community 173 - "Eval baseline — DESIGN rev 21 harness pass"
Cohesion: 0.40
Nodes (4): Eval baseline — DESIGN rev 21 harness pass, Found by the evals (fixed in rev 21), Reading it, Results

### Community 174 - "system-prompt.md"
Cohesion: 0.40
Nodes (4): Environment, Token discipline, Tools, Working style

## Knowledge Gaps
- **141 isolated node(s):** `github.com/adeotek/moca`, `Agent`, `Shell`, `example.com/slug`, `example.com/inventory` (+136 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 425 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **51 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `SPECS.md — moca implemented-state normative spec` connect `SPECS.md — moca implemented-state normative spec` to `start.go`, `OAuth login flow — dynamic registration, PKCE S256, loopback/headless, 30s per call`, `agent events — TextDelta … TurnEnd/SteeringApplied/Compacted/Resumed`, `tools registry — Tool interface, seven frozen tools, sequential execution in emitted order`, `permissions/shell.go`, `Context manager: token-denominated budgets, compaction, recovery`, `system prompt — built once per run, stored in the session, never rebuilt`, `internal/agent — system prompt, events, loop, compaction + resume, cross-provider transform, Start wiring`, `MCP Manager — zero start, lazy ensure, busy-guarded idle stop, replay-once rules`, `moca -p "<prompt>" — one-shot agent run (stdin -p form)`, `DESIGN §12 example config fixture (phase-1 decode gate)`, `credential store ~/.config/moca/auth.json (0600, atomic writes, cross-process lock)`, `External tools: rtk, graphify, skills ecosystem`, `moca update [--check] (newest published GitHub release, in-place swap)`, `MCP gating — readOnlyHint && !destructiveHint || approve list || session allow; no name heuristics`, `/-dropdown autocomplete (built-ins + prompt templates, ≤8 rows, tab/enter/esc)`, `Summarizer injection — provider-free compaction over a neutral Entry view`, `opencode-go provider (zen gateway)`?**
  _High betweenness centrality (0.297) - this node is a cross-community bridge._
- **What connects `github.com/adeotek/moca`, `Agent`, `Shell` to the rest of the system?**
  _141 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `start.go` be split into smaller, more focused modules?**
  _Cohesion score 0.09147609147609148 - nodes in this community are weakly interconnected._
- **Why does `model` connect `charm.land/bubbletea/v2.Cmd` to `start.go`, `Agent`, `Sanitize`, `renderMarkdown`, `pagerModel`, `newModel`, `model`, `Input`, `picker.go`, `NewDefaultStore`, `Start`, `time.Duration`, `Registry`, `format.go`?**
  _High betweenness centrality (0.042) - this node is a cross-community bridge._
- **Should `SPECS.md — moca implemented-state normative spec` be split into smaller, more focused modules?**
  _Cohesion score 0.030263157894736843 - nodes in this community are weakly interconnected._
- **Why does `Version stamp from git describe --tags --always --dirty` connect `v0.2.0-beta (2026-10-09): Ollama, saved / commands, self-update` to `go_pkg_fmt`?**
  _High betweenness centrality (0.041) - this node is a cross-community bridge._
- **Should `testing.T` be split into smaller, more focused modules?**
  _Cohesion score 0.03927927927927928 - nodes in this community are weakly interconnected._