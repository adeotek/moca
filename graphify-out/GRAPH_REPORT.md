# Graph Report - moca  (2026-10-10)

## Corpus Check
- 335 files · ~248,394 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 12 file(s) not represented in the graph (top: (none) 6, .jsonc 4, .v2 1)

## Summary
- 3397 nodes · 12866 edges · 176 communities (116 shown, 60 thin omitted)
- Extraction: 89% EXTRACTED · 11% INFERRED · 0% AMBIGUOUS · INFERRED: 1416 edges (avg confidence: 0.84)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `1e9c2077`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- go_pkg_strings
- newTestModel
- testing.T
- newScript
- run
- permissions/shell.go
- Input
- SPECS.md — current implemented state (living document)
- newFakeAS
- inline Bubble Tea TUI (moca with no -p)
- update/update.go
- Plan
- charm.land/bubbletea/v2.Cmd
- rev 21 (2026-10-09): eval corpus, overflow files, elision, cache-stable plan envelope, prompt_cache_key, verification nudge, exit-3 repeat stop, failing-test protocol re-armed, /do, project checks named in prompt
- NewManager
- internal/tui: Bubble Tea input, scrollback, status bar, spinner, pager package
- runTUI
- internal/provider — SSE reader, error taxonomy, catalog, 3 codecs, retry, registry
- internal/provider: protocol adapters, provider configs, model catalog, auth, retry package
- context.Context
- v0.1.0-alpha (2026-10-07): first release — minimal provider-agnostic coding agent, one Go binary, seven tools, no framework
- NewDefaultStore
- anthropic_test.go
- applyEdit
- internal/session — JSONL transcript, snapshots, undo, find/repair
- internal/provider/catalog.go Model struct (id, protocol, window, cost, thinking)
- vague/testdata/reference/main.go
- Context manager §6: token-denominated budgets (config-overridable)
- Phase 2 — seven frozen tools, path jail, shell analysis, approvals, sessions, agent loop
- Entry
- Auth mode oauth (moca login <provider> / /login)
- Summarize
- Message
- retry_test.go
- agent.Start wiring — jail → analyser → builtins → skills → instructions → prompt → session → build
- Sanitize
- Create
- internal/session: JSONL append-only transcripts, resume, snapshots/undo package
- webFetch
- shell tool: bash -c / pwsh -NoProfile, stateless at jail root, 30s default 300s max timeout
- Config
- Agent
- applog_test.go
- RenderStatus
- Discover
- sync.Mutex
- Request
- .startSession
- README.md - moca
- External tools - rtk, graphify, skills ecosystem
- internal/agent — system prompt, events, the loop, compaction, resume, transforms
- model
- BASELINE.md
- v0.1.1 (2026-10-08): polish, credential store in config dir, / autocomplete, bigger catalog
- Phase 7: OAuth providers + upstream graphify PR + v0.1 tag when §14 passes
- run
- internal/update — moca update (leaf, no internal imports)
- Slash commands from ~/.config/moca/prompts/<name>.md (+ workdir .moca/prompts when trusted); built-ins win collisions
- AGENTS.md — moca
- tui/trust.go
- NewRegistry
- Release - build packages, publish GitHub Release
- .Run
- hints.go
- TestWelcomeLines
- system-prompt.md
- cut_test.go
- SetString
- Start
- AGENTS.md
- .Compact
- lifecycle Manager — lazy ensure, idle timeout, dead-transport replay rules
- Manager
- SOURCES.md
- Claude Code skill-creator skill
- picker.go
- renderMarkdown
- time.Duration
- shipgate/fixture/main.go
- 2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)
- .Run
- Render
- newModel
- openai-responses adapter — /responses with stateless reasoning replay
- input_test.go
- BuildSystemPrompt
- v0.1.0-alpha (2026-10-07)
- LoadPrompts + ExpandPrompt ($ARGUMENTS, $1..$9) — slash-command templates
- runOneShot
- newAgentModel
- web tool (rev 19): fetch + search ops, no approval prompt
- fixture/order.go
- initialize
- quick_validate.py
- ollamaStart
- NewUnjailed
- System prompt ~40 lines: role, workdir, platform, date, git state, tool contract, token discipline, version stamp
- .RunPlan
- encoding/json.RawMessage
- exit-code contract 0/1/2/3/130 with per-class semantics
- create-command.md — starter prompt template
- install.sh script
- fixture/slug.go
- CI Workflow (gofmt · vet · build · test)
- Agent
- pagerModel
- Lookup
- Claude Code frontend-design skill
- pi mcp-scripting skill
- evals/run.sh
- allowAll
- reference/codes.go
- shipgate/run.sh
- ToolSpec
- .Run
- moca/log_test.go
- .DiscoverOllama
- ParseInput
- max_tokens = catalog max output capped at reserveTokens
- Item
- Built-in rtk skill (SKILL.md)
- Arrange-Act-Assert with xUnit/NSubstitute
- bugfix/check.sh
- feature/check.sh
- largefile/check.sh
- plan/check.sh
- refactor/check.sh
- vague/check.sh
- web/check.sh
- reference/order.go
- /copy keeps last answer on no-text turn; failed tools get red x; thinking 1 line; pager title clamped
- adaptive models take no thinking budget (sending one is a 400)
- Model catalog user-extensible in config
- /clear starts a new session file; the old one stays resumable
- cmd/moca entrypoint (CLI surface, mode select TUI | -p one-shot)
- Loop guard: if compaction leaves context above trigger, stop with error naming oversized entries
- House dotfiles merge machinery: merge-opencode-config.py / merge-pi-config.py semantics (recursive merge, .bak, byte-identical)
- moca — MO Coding Agent (binary name = repo name = module path)
- Module path github.com/adeotek/moca
- Tagline: the coding agent you can read in an afternoon
- rev 1 (2026-10-03): initial draft
- rev 10 (2026-10-04): yolo mode spec
- rev 17 (2026-10-09): /sessions manager — picker over every stored session, ctrl+d delete with single-writer-locked session.Delete
- rev 2 (2026-10-03): bash->shell, model/modelHard, three day-1 providers, rtk/graphify from day 1, phase restructure
- rev 3 (2026-10-03): MCP lazy by design v1; tool count six -> seven; phases six -> seven
- rev 4 (2026-10-03): status bar field set from day 1, effort first-class, /hard modelHard
- rev 5 (2026-10-03): status bar below input, multi-line paste, shift+enter newline
- rev 6 (2026-10-03): TOML -> JSONC config format adopted
- rev 7 (2026-10-03): compaction rewrite, entry ids, thinking persisted, steering, project trust, maxSteps, snapshots
- shell runs in its own process group; timeout or esc kills the whole group
- Three skill sources in precedence order: project .moca/skills, global ~/.config/moca/skills, built-in embedded
- Bubble Tea + lipgloss; compact/dense house style, no banner boxes
- Windows: normalized slash + case-insensitive comparison
- --help / -h usage (handled before the config check)
- module github.com/adeotek/moca
- thinking mode none — clamps every effort to off
- example.com/codes
- example.com/fetcher
- example.com/inventory
- example.com/shop
- example.com/slug
- example.com/temp
- example.com/totals
- github.com/adeotek/moca
- Model
- format.go
- Writer
- redact.go
- drained
- tui/log_test.go

## God Nodes (most connected - your core abstractions)
1. `model` - 75 edges
2. `newScript()` - 64 edges
3. `newTestModel()` - 63 edges
4. `run()` - 62 edges
5. `key()` - 44 edges
6. `inline Bubble Tea TUI (moca with no -p)` - 44 edges
7. `newAgentModel()` - 43 edges
8. `SPECS.md — current implemented state (living document)` - 42 edges
9. `textTurn()` - 41 edges
10. `Config` - 39 edges

## Surprising Connections (you probably didn't know these)
- `Persistent prompt history (history.jsonl per workdir) and ctrl+r history search` --semantically_similar_to--> `Session files ~/.local/share/moca/sessions/<date>-<slug>-<id8>.jsonl append-only, mode 0600, dir 0700`  [INFERRED] [semantically similar]
  CHANGELOG.md → docs/specs/DESIGN.md
- `Scenario: vague (6 runs - highest swing, stays hard)` --semantically_similar_to--> `Locate the cause with search before editing; fix root cause`  [INFERRED] [semantically similar]
  test/evals/BASELINE.md → internal/agent/system-prompt.md
- `Eval corpus test/evals/: seven scenarios with fixture repos and checkers; run.sh records steps/tokens/cost/repeated failures; BASELINE.md` --semantically_similar_to--> `Edit-ladder tests (phase 2, non-negotiable): exact unique/ambiguous, replace_all, fallback re-indentation, CRLF, BOM, unicode, 50-line span`  [INFERRED] [semantically similar]
  CHANGELOG.md → docs/specs/DESIGN.md
- `System prompt built once per run, never rebuilt` --semantically_similar_to--> `Skills mechanism (SKILL.md directories)`  [INFERRED] [semantically similar]
  AGENTS.md → docs/specs/external-tools.md
- `logRunCfgIn()` --calls--> `run()`  [INFERRED]
  cmd/moca/log_test.go → internal/tools/read_test.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Ecosystem SKILL.md compatibility corpus (graphify, Claude Code, pi, OpenCode)** — internal_skills_testdata_ecosystem_sources, internal_skills_testdata_ecosystem, internal_skills_testdata_ecosystem_graphify_graphify_vendored, internal_skills_testdata_ecosystem_claude_code_skill_creator_vendored, internal_skills_testdata_ecosystem_claude_code_frontend_design_vendored, internal_skills_testdata_ecosystem_pi_pi_subagents_vendored, internal_skills_testdata_ecosystem_pi_mcp_scripting_vendored, internal_skills_testdata_ecosystem_opencode_dotnet_unit_testing_vendored [EXTRACTED 1.00]
- **Synthetic frontmatter edge-case fixtures (CRLF, BOM, folding, quoting, rich keys, long description)** — internal_skills_testdata_ecosystem_synthetic_bom_skill, internal_skills_testdata_ecosystem_synthetic_colon_unquoted_skill, internal_skills_testdata_ecosystem_synthetic_crlf_skill, internal_skills_testdata_ecosystem_synthetic_folded_skill, internal_skills_testdata_ecosystem_synthetic_literal_skill, internal_skills_testdata_ecosystem_synthetic_long_description_skill, internal_skills_testdata_ecosystem_synthetic_rich_keys_skill [EXTRACTED 1.00]
- **Release progression: v0.1.0-alpha -> v0.1.1 -> v0.2.0-beta -> v0.3.0-beta** — changelog_release_v0_1_0_alpha, changelog_release_v0_1_1, changelog_release_v0_2_0_beta, changelog_release_v0_3_0_beta [INFERRED 0.75]
- **Token efficiency pipeline: caps -> overflow files -> elision -> compaction** — docs_specs_design_overflow_files, docs_specs_design_elision, docs_specs_design_compaction_entry, docs_specs_design_token_estimate, docs_specs_design_keep_recent_tokens [INFERRED 0.75]
- **The eight frozen tools and their contract surfaces** — docs_specs_design_tool_read, docs_specs_design_tool_write, docs_specs_design_tool_edit, docs_specs_design_tool_shell, docs_specs_design_tool_search, docs_specs_design_tool_ls, docs_specs_design_tool_web, docs_specs_design_tool_mcp [INFERRED 0.75]
- **OAuth policy gate path: verify terms -> per-provider decision -> config enforcement** — docs_specs_design_subscription_oauth_policy, docs_specs_oauth_verification_oauth_verification_record, docs_specs_oauth_verification_anthropic_api_key_only_decision, docs_specs_oauth_verification_ship_oauth_openai_decision, docs_specs_design_config_example [INFERRED 0.75]
- **Human-in-the-loop validation before declaring done** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_claude_code_frontend_design_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_review_validation [INFERRED 0.75]
- **The -p/TUI agent run pipeline** — docs_specs_specs_system_prompt, docs_specs_specs_session_rebuild, docs_specs_specs_turn_contract, docs_specs_specs_tools_registry, docs_specs_specs_compaction, docs_specs_specs_session_transcript, docs_specs_specs_agent_events [INFERRED 0.85]
- **Parallel subagent fanout with parent synthesis** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_graphify_graphify_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_skill [INFERRED 0.85]
- **Skills referencing bundled reference files for progressive disclosure** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_graphify_graphify_skill, internal_skills_testdata_ecosystem_pi_mcp_scripting_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_skill [INFERRED 0.85]
- **Permission & safety ladder** — docs_specs_specs_path_jail, docs_specs_specs_shell_analysis, docs_specs_specs_trust_store, docs_specs_specs_yolo, docs_specs_specs_tools_approvals, docs_specs_specs_ask_write_root [INFERRED 0.90]
- **MCP lazy proxy stack** — docs_specs_specs_mcp_jsonrpc, docs_specs_specs_mcp_stdio_transport, docs_specs_specs_mcp_http_transport, docs_specs_specs_mcp_index, docs_specs_specs_mcp_manager, docs_specs_specs_mcp_gating, docs_specs_specs_tool_mcp [INFERRED 0.95]
- **Seven-scenario eval corpus** — test_evals_baseline, agents_eval_corpus, test_evals_baseline_evalstats, test_evals_baseline_scenario_bugfix, test_evals_baseline_scenario_feature, test_evals_baseline_scenario_largefile, test_evals_baseline_scenario_plan, test_evals_baseline_scenario_refactor, test_evals_baseline_scenario_vague, test_evals_baseline_scenario_web [INFERRED 0.95]
- **rtk allowlist unwrapping and its subcommand classes** — docs_specs_external_tools_rtk, docs_specs_external_tools_rtk_unwrapping, docs_specs_external_tools_rtk_read_only_class, docs_specs_external_tools_rtk_write_config_class, docs_specs_external_tools_rtk_native_proxy_class, docs_specs_external_tools_rtk_shell_string_refusal, internal_permissions_rtk [INFERRED 0.95]
- **Token-efficiency mechanisms implementing the minimal-agent philosophy** — readme_minimal_tui_coding_agent, readme_prompt_caching, internal_agent_system_prompt_windowed_read, readme_diff_shaped_results, readme_token_denominated_compaction, internal_agent_system_prompt_token_discipline, readme_rtk_token_saver, readme_graphify_token_saver [INFERRED 0.95]

## Communities (176 total, 60 thin omitted)

### Community 0 - "go_pkg_strings"
Cohesion: 0.06
Nodes (73): Compacted, PlanChanged, Resumed, StartError, SteeringApplied, StreamReset, TextDelta, ThinkingDelta (+65 more)

### Community 1 - "newTestModel"
Cohesion: 0.05
Nodes (70): TestActivityLine(), TestCoalescedShrinkRemembersTheLastFlushedRow(), TestInputBoxGrowsAndShrinks(), TestPagerCloseDefersTheRelease(), TestRefuseRunningWhileOutputInFlight(), TestShrinkIsPaddedUntilTheGuardWindowPasses(), TestShrinkPaddingFitsTheScreen(), TestSpinnerTicksOnlyForTheLiveRun() (+62 more)

### Community 2 - "testing.T"
Cohesion: 0.03
Nodes (109): TestResumeNotASession(), CapChars(), TestCapChars(), TestCapCharsRuneSafe(), Parse(), TestAPIKeyOptional(), TestDefaultsApplied(), TestEnvRefs() (+101 more)

### Community 3 - "newScript"
Cohesion: 0.07
Nodes (82): scriptServer, newScript(), startTest(), startTestWith(), startWith(), TestAbortWritesSyntheticResults(), TestFailedCallDoesNotCancelRest(), TestLengthStopTruncatedToolCall() (+74 more)

### Community 4 - "run"
Cohesion: 0.05
Nodes (76): fakeCompletions(), isolate(), TestExitCodes(), TestHelpListsSurface(), TestInterruptedExit130(), TestLoginLogoutCLI(), TestMCPImportCLI(), TestNoApproveSkipsProjectInstructions() (+68 more)

### Community 5 - "permissions/shell.go"
Cohesion: 0.07
Nodes (38): setup, Jail, NewJail(), resolveDeep(), setup(), TestJail(), TestJailApprovable(), TestJailDanglingSymlinks() (+30 more)

### Community 6 - "Input"
Cohesion: 0.05
Nodes (34): TestListAll(), canonical(), Find(), FindForWorkdir(), Info, header(), headerAndPreview(), idOf() (+26 more)

### Community 7 - "SPECS.md — current implemented state (living document)"
Cohesion: 0.05
Nodes (45): .github/workflows/ci.yml — gofmt/vet/build/test gate, config validation rules (model ids, protocols, context windows, OAuth policy gate), DESIGN.md — v1 vision/contract (rev 20), test/evals/BASELINE.md — measured runs, test/evals/config.jsonc — eval model config, test/evals harness corpus (DESIGN rev 21, seven scenarios), test/evals/results/ — gitignored per-run metrics lines, test/evals/run.sh — N parallel repetitions in fresh dirs (+37 more)

### Community 8 - "newFakeAS"
Cohesion: 0.05
Nodes (50): fakeSIWC(), fakeSIWCOpts(), Client, TestDefaultHTTPClientBoundsHeaderWait(), defaultHTTPClient(), Store, newHostID(), validateAPIKey() (+42 more)

### Community 9 - "inline Bubble Tea TUI (moca with no -p)"
Cohesion: 0.05
Nodes (46): agent events — TextDelta…Resumed flow to the TUI, bin/moca — opens the interactive TUI with no arguments, agent control surface — SetModel, SetEffort, ToggleHard, Status, Steer, AddNote, Undo, Phase-3 gate — mock-driven TUI session in tmux, Phase 3 — interactive TUI, internal/tui — Bubble Tea TUI state + glue, Resolve(provider/model) → (Model, retry-wrapped Adapter); Models() for the picker, mid-stream failure — EventReset, one outside-budget retry (+38 more)

### Community 10 - "update/update.go"
Cohesion: 0.09
Nodes (35): envOr(), githubToken(), runUpdate(), Compare(), comparePre(), ExtractBinary(), FindAsset(), FindChecksums() (+27 more)

### Community 11 - "Plan"
Cohesion: 0.14
Nodes (17): TestImportRewritesEmbeddedSecretsInValues(), TestRewriteSecretsValueRuleAlone(), DiscoverSources(), embeddedSecret(), envName(), literalSecretFields(), ParseSource(), Plan() (+9 more)

### Community 12 - "charm.land/bubbletea/v2.Cmd"
Cohesion: 0.13
Nodes (5): model, AbbrevEffort(), compactDoneMsg, InputKind, Parsed

### Community 13 - "rev 21 (2026-10-09): eval corpus, overflow files, elision, cache-stable plan envelope, prompt_cache_key, verification nudge, exit-3 repeat stop, failing-test protocol re-armed, /do, project checks named in prompt"
Cohesion: 0.07
Nodes (42): Failing-test investigation protocol: while unresolved, tool results state next step (read failing test, locate cause) and edit refuses until both happened — §14 loop enforced, Inline edit diffs: successful edit prints changed lines (red/green, <=8) under its item line, Makefile: version-stamped builds (git describe -> -ldflags), make test -race, make release five cross-compiled binaries, v0.3.0-beta (2026-10-09): first-run setup mode, web tool, plan mode, measured harness pass, /do <plan> / --do <plan> executes a written plan step by step, ticking - [ ] boxes as verified, Fixed: edit with old_string ending in blank line crashed diff rendering; any tool panic now becomes an error result, Eval corpus test/evals/: seven scenarios with fixture repos and checkers; run.sh records steps/tokens/cost/repeated failures; BASELINE.md, Failing-test protocol arms only for failures found before the run's first change — no edit lockout for self-inflicted build breaks (+34 more)

### Community 14 - "NewManager"
Cohesion: 0.09
Nodes (51): runMCPIndex(), MCPConfig, MCPServer, crashingServer(), shServer(), skipWindows(), TestCallTimeoutStopsWedgedServer(), TestHandshakeTimeout() (+43 more)

### Community 15 - "internal/tui: Bubble Tea input, scrollback, status bar, spinner, pager package"
Cohesion: 0.08
Nodes (40): @path completes workdir files (gitignore-aware); path is sent, model reads the file itself, ctrl+c on empty draft says 'press ctrl+c again to quit'; in pager it clears the textarea, esc kills a running !cmd, which now gets its own activity row, Status bar shows the branch of a fresh git init instead of '-', install.sh / install.ps1: download release package, verify checksums.txt, install to ~/.local/bin or LOCALAPPDATA, re-run updates in place, moca update: replaces installed binary with latest GitHub release package; drafts skipped; atomic swap; runs before config load, /model picker marks keyless providers; argument completion in the / dropdown for several commands, Desktop notification/bell (tui.notify, default osc9) for finished run or waiting approval while terminal unfocused (+32 more)

### Community 16 - "runTUI"
Cohesion: 0.16
Nodes (15): TestExitForOAuthSentinels(), exitFor(), decideTrust(), hasProjectResources(), runTUI(), ExpandPrompt(), LoadPrompts(), TestEcosystemPromptsLoad() (+7 more)

### Community 17 - "internal/provider — SSE reader, error taxonomy, catalog, 3 codecs, retry, registry"
Cohesion: 0.08
Nodes (34): default base URLs and precedence baseUrls[protocol] > baseUrl > built-in, Registry.CheckCredential — credential existence check without the network, moca login <provider> — store a credential (OAuth or API key), moca mcp import — import MCP servers from host configs, config.AppendString — comment-preserving string-array edit, ~/.config/moca/config.jsonc ($XDG_CONFIG_HOME) and ~/.local/share/moca data dir, config.SetObjectEntry — insert raw-JSON key with the same guarantees, config.SetString — in-place string replacement (last-wins duplicate rule) (+26 more)

### Community 18 - "internal/provider: protocol adapters, provider configs, model catalog, auth, retry package"
Cohesion: 0.08
Nodes (32): +10 opencode-go models (deepseek v4.1/v4 flash & pro, kimi-k2.7-code, mimo-v2.6, qwen3.8, qwen3.7), all live-verified, A down Ollama server fails at once with how to start it; unpulled model says ollama pull; key optional for authenticating proxy; nothing probed unless opted in, Provider ollama: local or LAN server, no API key; baseUrl/$OLLAMA_HOST; models discovered from server (thinking models get effort levels), anthropic-messages protocol adapter, openai-completions protocol adapter, openai-responses protocol adapter, Auth mode api_key (stored key or env:VAR fallback), Optional per-provider baseUrl override and custom providers (vLLM, LM Studio, Ollama, gateways) (+24 more)

### Community 19 - "context.Context"
Cohesion: 0.18
Nodes (7): fakeSum, readEvents(), lockFile(), lockFile(), runResult(), TestRunResultMapping(), httpTransport

### Community 20 - "v0.1.0-alpha (2026-10-07): first release — minimal provider-agnostic coding agent, one Go binary, seven tools, no framework"
Cohesion: 0.08
Nodes (30): v0.1.0-alpha (2026-10-07): first release — minimal provider-agnostic coding agent, one Go binary, seven tools, no framework, Anthropic ships API key only per phase-7 policy gate (docs/specs/oauth-verification.md); config rejects auth oauth with recorded reason, CLI: moca TUI, -p one-shot with exit codes 0/1/2/3/130, --model, --effort, --approve/--no-approve, --yolo, --resume/--continue, login/logout, mcp import/index, Lazy MCP proxy: stdio + streamable HTTP, persisted index, ~200-token tool, moca mcp import with env: rewriting, Three protocol adapters hand-rolled HTTP+SSE, no SDKs (anthropic-messages, openai-completions, openai-responses), Seven frozen tools: read/write/edit (whitespace fallback, re-indentation, CRLF/BOM), shell (stateless, process-group kill), search (pure-Go, ripgrep semantics), ls, lazy mcp proxy, Call gating default-deny: readOnlyHint true (not destructiveHint) or per-server approve allowlist; otherwise TUI approval; -p refuses, mcp call op: executes; result truncated at 30K chars; non-text content replaced with [<type> omitted] (+22 more)

### Community 21 - "NewDefaultStore"
Cohesion: 0.12
Nodes (12): NewDefaultStore(), OAuthProvider(), model, providerNames(), providerRow(), TestLoginRowForOllama(), loginChoice, loginDoneMsg (+4 more)

### Community 22 - "anthropic_test.go"
Cohesion: 0.12
Nodes (40): SplitModel(), TestSplitModel(), newAnthropic(), keyCred(), mustJSON(), sseServer(), TestAnthropicBodyHygiene(), TestAnthropicBudgetAndNoCache() (+32 more)

### Community 23 - "applyEdit"
Cohesion: 0.24
Nodes (13): applyEdit(), formatDiff(), indentOf(), joinInts(), lineOf(), matchLines(), replaceExact(), replaceFuzzy() (+5 more)

### Community 24 - "internal/session — JSONL transcript, snapshots, undo, find/repair"
Cohesion: 0.10
Nodes (34): agent.Resume — pre-validate, repair orphans, stored prompt reuse, totals restore, --resume / --continue validation & failure handling, compaction — structured summary, lossy serialization, cumulative file lists, NewBudget / FindCut — trigger, keepRecent scaling, valid cut points, maybeCompact checkpoints + loop guard naming largest entries, summary request — cheap model, effort minimal, NoCacheWrite, usage-anchored context estimate — anchor + entries delta, agent elision — superseded tool results replaced by stubs at compaction checkpoints (+26 more)

### Community 25 - "internal/provider/catalog.go Model struct (id, protocol, window, cost, thinking)"
Cohesion: 0.12
Nodes (36): Model.BudgetTokens — effort-share of maxTokens, 1024 floor/cap, internal/provider/catalog.go Model struct (id, protocol, window, cost, thinking), Model.CostOf(usage) — per-token rate sum, Model.ClampEffort — nearest supported level at or below, level map anthropicAdaptive {low, medium, high, max}, level map basicThinking {low, medium, high} — no off/minimal, level map deepseekThinking {low, high, max}, level map deepseekProThinking {high, max} (+28 more)

### Community 26 - "vague/testdata/reference/main.go"
Cohesion: 0.29
Nodes (7): TestCollect(), CToF(), FToC(), TestRoundTrip(), main(), fail(), main()

### Community 27 - "Context manager §6: token-denominated budgets (config-overridable)"
Cohesion: 0.08
Nodes (26): Context pressure: percent yellow at 70%, red (with /compact) where auto-compaction fires, /cost uses the bar's compact token format and breaks down input (fresh/cache read/cache write); 'subscription' for OAuth, Retry/backoff with jitter honoring retry-after, SSE stall timeout, mid-stream failure rule; opencode-go [1210] 400-framed server_error retried; 120s header wait bound, Long outputs saved, not dropped: per-session overflow directories, another session's output stays out of reach, Trigger: estimatedTokens > contextWindow - reserve; checked after tool batches and before prompts, Context manager §6: token-denominated budgets (config-overridable), Custom-provider models declare contextWindow in config (validated against minimum), Effort default medium where the model supports it (+18 more)

### Community 28 - "Phase 2 — seven frozen tools, path jail, shell analysis, approvals, sessions, agent loop"
Cohesion: 0.15
Nodes (29): the agent loop — tool batches, steering drain, wrap-up, outcomes, tools.ApprovablePaths — read guard before prompting, single approval, built-in defaults (providers, allowlist, timeouts, budgets), config.EnvRefs — every referenced variable listed for shell-env hygiene, config keys (model, modelHard, providers, shell.allow, mcp, snapshot, context, web.search, tui.notify, yolo), web.search provider/key validation (tavily|exa; exa needs a key), empty reply handling — one nudge, then a warning, env:VAR indirection resolved lazily at request time (+21 more)

### Community 29 - "Entry"
Cohesion: 0.10
Nodes (15): ToolEnd, ToolStart, TurnEnd, Result, StopReason, ToolCall, Usage, anthropicStop() (+7 more)

### Community 30 - "Auth mode oauth (moca login <provider> / /login)"
Cohesion: 0.09
Nodes (27): moca login/logout openai: SIWC dynamic registration, PKCE S256, loopback 127.0.0.1, nonce + RS256 validation, rotating serialized refresh tokens, SSH fallback, best-effort revoke, Auth mode oauth (moca login <provider> / /login), OAuth credentials at ~/.config/moca/auth.json, mode 0600, auto-refresh, SSH/headless login fallback: print authorize URL + accept pasted code, Credential storage per docs: email, issuer, subject, issued client_id, host id, id_token, tokens, scopes; owner-only permissions, atomic writes, never logged or committed, Implementation delta 7: models on subscription route are account-visible slugs; runtime never calls /v1/models; user declares models in config, Implementation delta 2: openai uses dynamic client registration (plan assumed fixed client id), Implementation delta 3: ext_agent_host_id persisted alongside the store, sent on every authorize attempt (+19 more)

### Community 31 - "Summarize"
Cohesion: 0.31
Nodes (8): Metrics, Collect(), fatal(), main(), meanSD(), readResults(), Summarize(), TestSummarize()

### Community 32 - "Message"
Cohesion: 0.11
Nodes (22): NormalizeToolID(), TestNormalizeToolID(), TestTransformDropsEmptiedAssistantAndMerges(), TestTransformDropsRedactedForeignThinking(), TestTransformHistory(), TestTransformSameBareIDDifferentProvider(), TransformHistory(), MessageChars() (+14 more)

### Community 33 - "retry_test.go"
Cohesion: 0.14
Nodes (25): Retry, Event, Adapter, causeClass(), TestCauseClass(), isDialFailure(), DefaultRetryPolicy(), RetryNotice (+17 more)

### Community 34 - "agent.Start wiring — jail → analyser → builtins → skills → instructions → prompt → session → build"
Cohesion: 0.10
Nodes (26): custom providers (vLLM/LM Studio/Ollama…) via baseUrl + protocol, internal/skills/testdata/ecosystem/SOURCES.md — provenance, licenses, SHA-256, Phase-6 scripted gate — ecosystem skills, rtk unwrap, graphify query, Phase-6 live legs — rtk skill description sharpening, Upstream PR Graphify-Labs/graphify#4174 (graphify install --platform moca), LoadInstructions — global AGENTS.md, workdir AGENTS.md / CLAUDE.md, 32K cap, ollama base-URL normalization (OLLAMA_HOST > localhost:11434), ollama context window from Modelfile num_ctx, else assumed 16384 (+18 more)

### Community 35 - "Sanitize"
Cohesion: 0.08
Nodes (20): TextOf(), AutoAllow(), Answer, Question, approvalPanel(), approvalPrompt(), firstLineOf(), TestApprovalPromptSanitized() (+12 more)

### Community 36 - "Create"
Cohesion: 0.17
Nodes (14): Delete(), TestDelete(), TestFind(), TestFindForWorkdirViaSymlink(), TestFindMtimeTieDeterministic(), TestInWorkdir(), Create(), newID() (+6 more)

### Community 37 - "internal/session: JSONL append-only transcripts, resume, snapshots/undo package"
Cohesion: 0.11
Nodes (23): Exit keeps final status bar and prints 'session <id8> · resume with: moca --resume <id8>', Persistent prompt history (history.jsonl per workdir) and ctrl+r history search, --resume/--continue replay last 3 turns into scrollback; /resume picker; id from another directory refused, /sessions lists this directory's stored sessions; ctrl+d after y confirm; open and live-writer sessions refuse; newest 100 listed, Token-denominated context manager, JSONL transcripts with entry ids, wrestling single-writer flock (actually single-writer flock, 0600), resume/continue, Compaction entry: summary + firstKeptEntryId + tokensBefore + usage; repeated compactions chain, Elision (rev 21): superseded tool results stubbed via elision entries, >=8K savings, after compaction, internal/session: JSONL append-only transcripts, resume, snapshots/undo package (+15 more)

### Community 38 - "webFetch"
Cohesion: 0.14
Nodes (19): Spill(), cutWeb(), isSpaceByte(), TestWebHTMLConverters(), TestWebHTMLStrayClosingPre(), webClean(), webDisplayURL(), webDoJSON() (+11 more)

### Community 39 - "shell tool: bash -c / pwsh -NoProfile, stateless at jail root, 30s default 300s max timeout"
Cohesion: 0.13
Nodes (19): Approvals show the whole command up to 6 lines (overflow paged with ctrl+o) — a python3 approval used to hide the heredoc, Path jail, mvdan.cc/sh analysis with hard-deny/ask ladders and rtk unwrapping, project trust, yolo mode, pre-edit snapshots + /undo, Not allowlisted -> refusal naming the command; TUI allow-once/allow-always/deny, Every simple command anywhere in the tree checked; wrappers env/time/timeout/nice/nohup/command unwrapped, Path jail anchored at session-start workdir, symlink-resolved (EvalSymlinks), refuses symlink escapes, Phase 2: seven tools + agent loop + permissions + skills loader + session schema; mcp stub frozen day 1; edit-ladder + shell-analysis ladders, Yolo does not lift the plan-mode write confinement, Output redirections must pass the path jail; /dev/null allowed; non-literal targets refused (+11 more)

### Community 40 - "Config"
Cohesion: 0.10
Nodes (18): ContextConfig, LogConfig, ModelOverride, ShellConfig, SnapshotConfig, TUIConfig, WebConfig, WebSearchConfig (+10 more)

### Community 41 - "Agent"
Cohesion: 0.11
Nodes (14): savedModel, Agent, Outcome, New(), userText(), verifyNudge(), Agent, outcomeKind() (+6 more)

### Community 42 - "applog_test.go"
Cohesion: 0.13
Nodes (18): Level, wc, startLog(), Discard(), Open(), ParseLevel(), prune(), keepDefault() (+10 more)

### Community 43 - "RenderStatus"
Cohesion: 0.17
Nodes (14): TestStatusWideRunesNeverExceedWidth(), TestStatusLineCarriesTheSetupState(), FmtPercent(), renderLine(), RenderStatus(), TestContextPressure(), TestFmtPercent(), TestRenderStatusFull() (+6 more)

### Community 44 - "Discover"
Cohesion: 0.16
Nodes (13): filterSkills(), TestFilterSkillsDropsMissingBuiltins(), builtinHash(), ExtractBuiltins(), extractBuiltins(), TestEcosystemSkillsLoadUnchanged(), Discover(), Skill (+5 more)

### Community 45 - "sync.Mutex"
Cohesion: 0.07
Nodes (18): logSink, cappedWriter, cutRecord(), responseID(), TestResponseID(), callResult, canonical(), LoadTrust() (+10 more)

### Community 46 - "Request"
Cohesion: 0.11
Nodes (21): ContentBlock, Request, ToolResult, post(), anthropicOverflow(), newHTTPError(), retryAfter(), openaiOverflow() (+13 more)

### Community 48 - "README.md - moca"
Cohesion: 0.13
Nodes (22): Exactly eight frozen tools, Golden tool schemas (schemas.golden.json), Cross-tool skills compatibility, graphify - codebase knowledge graph tool, Prompt templates (~/.config/moca/prompts/<name>.md -> /name), internal/skills/testdata/ecosystem/ corpus, mcp tool (search, describe, call), README.md - moca (+14 more)

### Community 49 - "External tools - rtk, graphify, skills ecosystem"
Cohesion: 0.20
Nodes (16): graphify-out/ knowledge graph, External tools - rtk, graphify, skills ecosystem, graphify install --platform moca, Graphify-Labs/graphify PR 4174, rtk - token-compressed CLI proxy, Built-in rtk skill (internal/skills/builtin/rtk/), rtk native-binary proxy subcommands, rtk read-only machinery class (+8 more)

### Community 50 - "internal/agent — system prompt, events, the loop, compaction, resume, transforms"
Cohesion: 0.11
Nodes (20): CacheKey moca-<id8> — prompt_cache_key on responses (completions to api.openai.com), CLI surface (section 3), moca -p — one-shot agent run, agent.DetectVerify — project checks line from Makefile/package.json/go.mod &c, --approve / --no-approve project-trust flags, --config <path> dev/test override, --do <plan.md> plan-execution flag, --effort <level> flag (validated, clamped per model) (+12 more)

### Community 51 - "model"
Cohesion: 0.17
Nodes (10): model, matchCommands(), model, mentionMatches(), mentionQuery(), rankFile(), TestMentionQuery(), TestMentionRanking() (+2 more)

### Community 52 - "BASELINE.md"
Cohesion: 0.11
Nodes (25): baseline label (pre-rev-21 harness), edit diff panic on trailing blank line, Empty-reply retry, Eval baseline — DESIGN rev 21 harness pass, final label (DESIGN rev 21), Found by the evals (fixed in rev 21), Investigation-protocol scope fix, Baseline vs final metrics table (+17 more)

### Community 53 - "v0.1.1 (2026-10-08): polish, credential store in config dir, / autocomplete, bigger catalog"
Cohesion: 0.11
Nodes (15): apiKey optional: stored key wins over env:VAR fallback; applies from the next request of the running session; /logout restores, Credentials live in ~/.config/moca/auth.json (0600), v0.1 store migrated automatically; API keys and OAuth tokens share the file, / autocomplete dropdown with hints, filtered typing, built-ins win collisions; commands echo as messages, moca login <provider> stores API keys too (prompt only on terminal; echo -n | login stores silently); anthropic prints recorded policy reason, /login in TUI: provider picker, masked key entry, pasted redirect URL/code feeds the flow, esc cancels; /logout revokes and clears, Markdown-lite responses: bold, code, headings, bullets, quotes, fenced blocks on a darker band; /copy still copies raw text, v0.1.1 (2026-10-08): polish, credential store in config dir, / autocomplete, bigger catalog, Inline-renderer shrink guard remembers the last actually-flushed frame (no stale rows) (+7 more)

### Community 54 - "Phase 7: OAuth providers + upstream graphify PR + v0.1 tag when §14 passes"
Cohesion: 0.12
Nodes (15): graphify install --platform moca contributed upstream (Graphify-Labs/graphify#4174), graphify install --platform moca needs a one-line upstream platform entry, Phase 7: OAuth providers + upstream graphify PR + v0.1 tag when §14 passes, rev 12 (2026-10-06): OAuth policy gate results — anthropic api_key only, openai ships SIWC OAuth (rev-12 spec change), No registration path for third-party consumer-plan OAuth; using Anthropic's own client ids would be spoofing, Timeline: 2026-01 server-side OAuth checks, bans later partially reversed; 2026-02 clause formalized; 2026-04 subscription coverage ends for third-party tools; 2026-09 ban reaffirmed, Anthropic Claude Code legal docs quote: OAuth exclusively for subscription-plan purchasers of native apps; third-party developers must use API keys, Primary source cited: code.claude.com/docs/en/legal-and-compliance (+7 more)

### Community 55 - "run"
Cohesion: 0.13
Nodes (23): parseArgs(), printUsage(), TestParseArgs(), loginAPIKey(), runLogin(), runLogout(), yesNo(), run() (+15 more)

### Community 56 - "internal/update — moca update (leaf, no internal imports)"
Cohesion: 0.12
Nodes (21): moca update — replace the running binary with the newest GitHub release, MOCA_API_BASE override, MOCA_UPDATE_API override, Install-script gate — fresh install, in-place update, mismatch refusal, moca update gate — version table, checksum refuse, real-binary run, Go 1.27.1 pinned in go.mod toolchain directive, install.ps1 — Windows install/update from release packages, install.sh — Linux install/update from release packages (+13 more)

### Community 57 - "Slash commands from ~/.config/moca/prompts/<name>.md (+ workdir .moca/prompts when trusted); built-ins win collisions"
Cohesion: 0.15
Nodes (17): rtk and graphify in default allowlist with built-in skills; Agent-Skills compatibility proven by vendored corpus with provenance and SHA-256 enforcement, /create-command starter template seeded on first TUI run (never overwrites existing); agent creates/updates saved slash commands; approvals outside workdir once per write, Agent Skills standard, same shape as Pi/Claude Code/OpenCode, AGENTS.md loader: global AGENTS.md then workdir AGENTS.md or CLAUDE.md (trusted only), capped 32K chars, Ask-write root: ~/.config/moca/prompts/ readable; write/edit refused unless user approves once, never persisted (rev 15), Built-in rtk skill embedded via embed.FS, extracted at startup to ~/.local/share/moca/builtin-skills/<version>/, graphify: codebase-graph CLI, default allowlist, drop-in SKILL.md, internal/permissions: path jail, shell command analysis + allowlist, project trust package (+9 more)

### Community 58 - "AGENTS.md — moca"
Cohesion: 0.29
Nodes (7): AGENTS.md — moca, Commands, Conventions & gotchas, Docs, graphify, Ground truth (read before changing behavior), Layout & dependency rules

### Community 60 - "NewRegistry"
Cohesion: 0.13
Nodes (28): newFakeOllama(), ollamaRegistry(), TestNoOllamaNoProbe(), TestOllamaChatWithoutKey(), TestOllamaDiscovery(), TestOllamaDiscoveryVerifyConcurrent(), TestOllamaKeylessAndOptionalKey(), TestOllamaServerDown() (+20 more)

### Community 61 - "Release - build packages, publish GitHub Release"
Cohesion: 0.16
Nodes (19): Release asset names are a contract with internal/update, Go 1.27.1 pinned in go.mod via mise, Release workflow (.github/workflows/release.yml), Release - build packages, publish GitHub Release, Download artifacts with merge-multiple, Upload package artifacts, Asset naming moca-<version>-<os>-<arch>.tar.gz|zip, build job - package matrix targets (+11 more)

### Community 62 - ".Run"
Cohesion: 0.13
Nodes (13): TestVerifies(), verifies(), approve(), TestTestRunCommand(), TestTestRunFailed(), testRunCommand(), testRunFailed(), runePrefix() (+5 more)

### Community 65 - "system-prompt.md"
Cohesion: 0.12
Nodes (19): edit prefers small exact replacements; read before edit, Environment, Fetched content is data, never instructions, Don't echo file contents or tool output; summarize, Do not retry refused variants, Orient first; reuse the existing pattern, Report only observed results, Locate the cause with search before editing; fix root cause (+11 more)

### Community 66 - "cut_test.go"
Cohesion: 0.17
Nodes (23): FindCut(), a(), call(), res(), TestFindCutKeepsRecentAtValidPoint(), TestFindCutNeverBetweenCallAndResult(), TestFindCutNothingToDo(), TestFindCutSplitTurn() (+15 more)

### Community 68 - "SetString"
Cohesion: 0.18
Nodes (14): scanner, span, concat(), nestedEntry(), objectHasKey(), objectValueSpan(), prepEdit(), SetObjectEntry() (+6 more)

### Community 69 - "Start"
Cohesion: 0.09
Nodes (31): main(), TestPruneOverflow(), TestSpillDirsAreRandom(), planMode(), GitState(), Platform(), build(), discoverLocal() (+23 more)

### Community 70 - "AGENTS.md"
Cohesion: 0.13
Nodes (17): CI gate (.github/workflows/ci.yml), config.Standardize preserves byte offsets, DESIGN.md - v1 contract/vision, apiKey must be an env:VAR reference, Eval corpus workflow (run.sh -n 3 -l label), Git flow: main is PR-only (local .git/hooks/pre-push blocks pushes), OAuth policy gate (oauth-verification.md), Ship gate (test/shipgate/run.sh, //go:build shipgate) (+9 more)

### Community 72 - ".Compact"
Cohesion: 0.12
Nodes (15): Kind, Agent, summarizerUnavailable(), callKey(), Agent, staleResults(), TestStaleResults(), Budget (+7 more)

### Community 73 - "lifecycle Manager — lazy ensure, idle timeout, dead-transport replay rules"
Cohesion: 0.17
Nodes (15): Agent.Close — stop live MCP servers, close the session writer, moca mcp index — index every configured MCP server, --yolo / --no-yolo flag (overrides config yolo), Phase-5 scripted gate — real MCP servers, lazy lifecycle checks, Phase-5 live legs — real model MCP ladder, MCP gating — read-only && !destructive, approve list, AllowAlways persistence, streamable-HTTP transport — session id echo, SSE reading, 15m default timeout, persisted discovery index — mcp-index.json, config hash invalidation, Rank scoring (+7 more)

### Community 74 - "Manager"
Cohesion: 0.15
Nodes (9): Rank(), TestRank(), words(), Manager, startFailClass(), Hit, Index, IndexEntry (+1 more)

### Community 76 - "SOURCES.md"
Cohesion: 0.19
Nodes (7): internal/skills/testdata/ecosystem corpus, anthropics/skills frontend-design, anthropics/skills skill-creator, graphify skill (graphify install, claude platform; byte-identical to pi variant), adeotek/dotfiles opencode dotnet-unit-testing skill, npm pi-mcp-adapter mcp-scripting skill, npm pi-subagents skill

### Community 77 - "Claude Code skill-creator skill"
Cohesion: 0.16
Nodes (13): Skill description (trigger) optimization, Skill eval/iterate loop (with-skill vs baseline), skill-creator Apache 2.0 License (Anthropic), Claude Code skill-creator skill, SKILL.md anatomy + bundled resources layout, Constrained query expansion against graph vocab, Progressive Disclosure (three-level skill loading), graphify query/path/explain reference (+5 more)

### Community 78 - "picker.go"
Cohesion: 0.18
Nodes (8): modelPickItems(), choiceRow(), masked(), TestWindowRange(), model, windowRange(), pickItem, pickState

### Community 79 - "renderMarkdown"
Cohesion: 0.23
Nodes (12): codeBandStyle(), fenceLang(), renderInline(), renderMarkdown(), plainMD(), TestMarkdownBlocks(), TestMarkdownFence(), TestMarkdownSanitizes() (+4 more)

### Community 81 - "time.Duration"
Cohesion: 0.24
Nodes (7): TestFmtElapsed(), fmtElapsed(), fmtThinkDuration(), Options, HTTPError, Item, Items

### Community 82 - "shipgate/fixture/main.go"
Cohesion: 0.21
Nodes (7): Sum(), TestTotals(), ParseList(), Average(), Max(), TestMaxAndAverage(), main()

### Community 83 - "2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)"
Cohesion: 0.15
Nodes (12): 1.1 Terms, 1.2 Client registration, 1.3 Decision, 1. anthropic — decision: `api_key only` (subscription OAuth not permitted), 2.1 Terms, 2.2 Client registration (per the docs), 2.3 Endpoints (verified live 2026-10-06 against `https://auth.openai.com/.well-known/openid-configuration`), 2.4 Inference contract for subscription traffic (open-source flow docs) (+4 more)

### Community 84 - ".Run"
Cohesion: 0.19
Nodes (8): globToRegexp(), ignored(), parseIgnore(), TestIgnore(), ListFiles(), TestListFiles(), ignoreRule, searchTool

### Community 85 - "Render"
Cohesion: 0.17
Nodes (7): jsonItem, main(), Render(), TestRenderTotal(), main(), RenderJSON(), TestRenderJSON()

### Community 86 - "newModel"
Cohesion: 0.14
Nodes (13): Event, Asker, newModel(), Run(), newAsker(), newEventPipe(), TestEventPipeBuffersUntilStarted(), TestEventPipeNonBlockingAndFIFO() (+5 more)

### Community 87 - "openai-responses adapter — /responses with stateless reasoning replay"
Cohesion: 0.21
Nodes (11): anthropic-messages adapter — /v1/messages, openai-completions adapter — /chat/completions, openai-responses adapter — /responses with stateless reasoning replay, shared post() — JSON request, SSE accept, UA moca/<version>, 2xx non-SSE is an error, anthropic cache breakpoints — system end, last tool, last eligible block, internal/config/testdata/example.jsonc — byte-exact DESIGN section 12 fixture, Phase-3 live gate — cross-provider model switching, reproducible local gates — forced-429 backoff, protocol e2e, golden schema (+3 more)

### Community 89 - "input_test.go"
Cohesion: 0.30
Nodes (11): NewInput(), TestCollapseUsesExactOffset(), TestCtrlC(), TestEditedMarkerSentLiterally(), TestHistory(), TestHistoryRecallChipsUnsafeContent(), TestMarkerNeverCollidesWithTypedText(), TestPasteChipKeepsContent() (+3 more)

### Community 91 - "BuildSystemPrompt"
Cohesion: 0.29
Nodes (8): PromptInput, ServerLine, BuildSystemPrompt(), oneLine(), renderPrompt(), TestBuildSystemPrompt(), TestPromptRTKIsConditional(), TestPromptTemplateSlots()

### Community 92 - "v0.1.0-alpha (2026-10-07)"
Cohesion: 0.18
Nodes (10): CHANGELOG, CLI, Context & sessions, External tools & the skills ecosystem, MCP, OAuth (subscription login), Providers & protocols, Tools & permissions (+2 more)

### Community 93 - "LoadPrompts + ExpandPrompt ($ARGUMENTS, $1..$9) — slash-command templates"
Cohesion: 0.22
Nodes (11): ask-write root ~/.config/moca/prompts — writes ask the user, never persistent, moca logout <provider> — revoke subscription session, clear stored credential, get-command? create-command.md starter template with {{prompts_dir}}, Slash-command gate — seeded template, ask-write approval, reload, best-effort revoke bounded at 15 s, then delete token and API key, LoadPrompts + ExpandPrompt ($ARGUMENTS, $1..$9) — slash-command templates, skills.SeedUserPrompts — seeds the create-command starter, never overwrites, @path file index — gitignore-aware, ranked, Insert as text (+3 more)

### Community 94 - "runOneShot"
Cohesion: 0.36
Nodes (7): TestParseArgsDo(), firstLine(), oneShotSlug(), resolveResume(), runOneShot(), Slug(), TestSlug()

### Community 95 - "newAgentModel"
Cohesion: 0.07
Nodes (54): keyMsg(), TestExitCommandQuits(), TestCompactEscapeCancels(), TestCompactingBlocksRun(), TestCompactNothing(), TestEditItemShowsDiff(), TestAllowAlwaysRequiresCtrl(), TestApprovalKeysAnswerAfterPause() (+46 more)

### Community 97 - "web tool (rev 19): fetch + search ops, no approval prompt"
Cohesion: 0.31
Nodes (5): Web tool in release: fetch (bounded 5 redirects, 2MiB, 20K chars) + Tavily keyless search; no approval; untrusted data, rev 19 (2026-10-09): web tool — fetch (OpenCode webfetch contract) + search (tavily keyless / exa), no approval, untrusted content; tool set to eight, Tavily keyless mode by default; web.search.apiKey env reference lifts the rate limit, web tool (rev 19): fetch + search ops, no approval prompt, web search op: ranked title/url/snippet results, tavily keyless by default or exa

### Community 98 - "fixture/order.go"
Cohesion: 0.28
Nodes (6): Order, User, NewOrder(), TestOrderEmail(), TestUserEmail(), NewUser()

### Community 99 - "initialize"
Cohesion: 0.13
Nodes (17): webSearchKey(), TestResolveEnv(), ResolveEnv(), initialize(), TestHTTPCloseBounded(), TestKeepNameWindows(), startHTTP(), fakeHTTPServer() (+9 more)

### Community 101 - "ollamaStart"
Cohesion: 0.25
Nodes (8): ollamaServer(), ollamaStart(), TestOllamaDeclaredWindowIsQuiet(), TestOllamaReportedWindowIsQuiet(), TestOllamaStartErrors(), TestOllamaTinyWindowWarns(), TestStartWithOllamaModel(), abortingHTTPServer()

### Community 102 - "NewUnjailed"
Cohesion: 0.29
Nodes (4): Agent, NewUnjailed(), TestUnjailedAndAllowAll(), Unjailed

### Community 103 - "System prompt ~40 lines: role, workdir, platform, date, git state, tool contract, token discipline, version stamp"
Cohesion: 0.29
Nodes (7): System prompt names detected project checks; working style rewritten (assumptions, edge cases, root causes, never weaken tests, review diffs, no shell edits), Explicit cache breakpoints in Request: anthropic cache_control on tools+system prefix and last message, rolled forward (<=4), rev 21: tool rules forbid editing files through the shell, rev 21: detected project check command (Makefile/package.json/ecosystem) named in the environment section, System prompt ~40 lines: role, workdir, platform, date, git state, tool contract, token discipline, version stamp, System prompt token-discipline section: prefer rtk-prefixed variants where they exist, rev 21: rewritten working style — orient, state assumptions, domain edge cases, root cause, never weaken tests, verify + review diff, evidence

### Community 104 - ".RunPlan"
Cohesion: 0.40
Nodes (4): doNudge(), doPrompt(), Agent, uncheckedSteps()

### Community 106 - "encoding/json.RawMessage"
Cohesion: 0.05
Nodes (24): CallResult, audContains(), splitLines(), planAllows(), Env, isDocPath(), noteChange(), trackFailure() (+16 more)

### Community 107 - "exit-code contract 0/1/2/3/130 with per-class semantics"
Cohesion: 0.29
Nodes (6): overflow recovery — errOverflow, one compact-and-retry, error taxonomy — HTTPError, ErrStall, ErrContextOverflow, EnvError, OAuth refusals, exit-code contract 0/1/2/3/130 with per-class semantics, context.maxSteps wrap-up — tool_choice none + Outcome MaxSteps (exit 3), Phase-4 compaction/resume review passes 1–2, stuck stop — repeated identical failing calls end the run with a warning

### Community 108 - "create-command.md — starter prompt template"
Cohesion: 0.33
Nodes (3): create-command.md — starter prompt template, Project commands dir .moca/prompts/, Personal commands dir {{prompts_dir}} (~/.config/moca/prompts/)

### Community 109 - "install.sh script"
Cohesion: 0.67
Nodes (5): die(), fetch(), note(), install.sh script, usage()

### Community 110 - "fixture/slug.go"
Cohesion: 0.40
Nodes (4): Limit(), Make(), TestLimit(), TestMake()

### Community 111 - "CI Workflow (gofmt · vet · build · test)"
Cohesion: 0.40
Nodes (4): CI Workflow (gofmt · vet · build · test), CI gate set (gofmt, go vet, go build, go test -race), go.mod (Go version source for setup-go), SPECS.md §2 — CI gate definition

### Community 114 - "pagerModel"
Cohesion: 0.18
Nodes (4): TestMatchRanges(), matchRanges(), newPager(), pagerModel

### Community 115 - "Lookup"
Cohesion: 0.50
Nodes (3): Lookup(), TestLookupBounds(), Valid()

### Community 116 - "Claude Code frontend-design skill"
Cohesion: 0.50
Nodes (3): AI-generated design default tells, frontend-design Apache 2.0 License, Claude Code frontend-design skill

### Community 117 - "pi mcp-scripting skill"
Cohesion: 0.67
Nodes (4): jev.evaluate reference, jev.evaluate semantic scripting gate, mcpScript tools API, pi mcp-scripting skill

### Community 119 - "evals/run.sh"
Cohesion: 1.00
Nodes (3): fresh_repo(), one(), run.sh script

### Community 129 - "ToolSpec"
Cohesion: 0.08
Nodes (18): ToolSpec, TestProxySpecFrozen(), MCPSpec(), TestRegistryRecoversToolPanic(), TestRepeatedFailureHintAndReset(), TestRepeatedFailureIgnoresSpillPath(), Registry, NewRegistry() (+10 more)

### Community 130 - ".Run"
Cohesion: 0.20
Nodes (9): cutRunes(), errResult(), isTrue(), TestCutRunesSafe(), Annotations, client, IndexTool, ProxyTool (+1 more)

### Community 133 - "moca/log_test.go"
Cohesion: 0.27
Nodes (14): logRun(), logRunCfg(), logRunCfgIn(), logRunIn(), readLog(), sessionID8(), TestLogDebugViaEnv(), TestLogFileForOneShot() (+6 more)

### Community 137 - ".DiscoverOllama"
Cohesion: 0.10
Nodes (12): Registry, modelFromShow(), normalizeOllamaURL(), ollamaBaseURL(), ollamaRoot(), ollamaUnreachable(), TestNormalizeOllamaURL(), TestOllamaBaseURLPrecedence() (+4 more)

### Community 138 - "ParseInput"
Cohesion: 0.47
Nodes (6): Prompt, HelpText(), ParseInput(), TestParseInput(), TestPromptDirsReloadAtRunEnd(), TestLoginCommandParsingAndHelp()

### Community 152 - "reference/order.go"
Cohesion: 0.32
Nodes (5): Order, User, NewOrder(), NewUser(), normalizeEmail()

### Community 192 - "Model"
Cohesion: 0.10
Nodes (7): Status, Agent, Effort, ParseEffort(), TestParseEffort(), Model, supportedEfforts()

### Community 193 - "format.go"
Cohesion: 0.26
Nodes (10): TestGitStatusUsesNoOptionalLocks(), AbbrevHome(), FmtTokens(), FmtWindow(), GitBranch(), gitStatusArgs(), TestFormatters(), TestGitBranch() (+2 more)

### Community 194 - "Writer"
Cohesion: 0.13
Nodes (9): Options, lockFile(), lockFile(), gitClean(), Snapshots, NewSnapshots(), Writer, lastNewline() (+1 more)

### Community 195 - "redact.go"
Cohesion: 0.33
Nodes (5): cutString(), isSecretKey(), replaceAttr(), safeURL(), segments()

### Community 197 - "drained"
Cohesion: 0.35
Nodes (11): TestThinkingLinePrintsBeforeTheResponse(), drained(), newLoginModel(), TestLoginMarkedBackAndCancel(), TestLoginOAuthDeclinedFlipAndCancel(), TestLoginOAuthFlowAndAuthFlip(), TestLoginRefusedWhileRunning(), TestLoginStoresAPIKey() (+3 more)

### Community 214 - "tui/log_test.go"
Cohesion: 0.24
Nodes (7): captureLog(), logPolicy(), TestProviderLogsFinalFailureAtInfo(), TestProviderLogsRetryAndResponse(), captureLog(), TestSetupFirstSessionLogged(), logSink

## Knowledge Gaps
- **182 isolated node(s):** `github.com/adeotek/moca`, `Agent`, `Shell`, `example.com/slug`, `example.com/inventory` (+177 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 474 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **60 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `External tools - rtk, graphify, skills ecosystem` connect `External tools - rtk, graphify, skills ecosystem` to `README.md - moca`, `permissions/shell.go`, `AGENTS.md`?**
  _High betweenness centrality (0.052) - this node is a cross-community bridge._
- **What connects `github.com/adeotek/moca`, `Agent`, `Shell` to the rest of the system?**
  _182 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `go_pkg_strings` be split into smaller, more focused modules?**
  _Cohesion score 0.06101136683134514 - nodes in this community are weakly interconnected._
- **Why does `model` connect `charm.land/bubbletea/v2.Cmd` to `go_pkg_strings`, `Sanitize`, `Start`, `Input`, `Agent`, `RenderStatus`, `picker.go`, `renderMarkdown`, `time.Duration`, `pagerModel`, `model`, `NewDefaultStore`, `anthropic_test.go`, `newModel`?**
  _High betweenness centrality (0.018) - this node is a cross-community bridge._
- **Should `newTestModel` be split into smaller, more focused modules?**
  _Cohesion score 0.04818865345181134 - nodes in this community are weakly interconnected._
- **Why does `README.md - moca` connect `README.md - moca` to `system-prompt.md`, `External tools - rtk, graphify, skills ecosystem`, `Release - build packages, publish GitHub Release`, `AGENTS.md`?**
  _High betweenness centrality (0.013) - this node is a cross-community bridge._
- **Should `testing.T` be split into smaller, more focused modules?**
  _Cohesion score 0.02902418682235196 - nodes in this community are weakly interconnected._