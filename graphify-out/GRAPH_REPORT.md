# Graph Report - moca  (2026-10-10)

## Corpus Check
- 346 files · ~250,412 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 12 file(s) not represented in the graph (top: (none) 6, .jsonc 4, .v2 1)

## Summary
- 3462 nodes · 12806 edges · 204 communities (118 shown, 86 thin omitted)
- Extraction: 89% EXTRACTED · 11% INFERRED · 0% AMBIGUOUS · INFERRED: 1458 edges (avg confidence: 0.85)
- Token cost: 52,000 input · 21,000 output

## Graph Freshness
- Built from commit: `261ab308`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- quick_validate.py
- v0.1.0-alpha (2026-10-07)
- go_pkg_strings
- testing.T
- NewUnjailed
- reference/order.go
- Items
- Snapshots
- history.go
- .RunPlan
- install.sh script
- Parse
- Serialize
- serverEnv
- TestModelPickerRefreshesOllama
- fixture/slug.go
- Agent
- captureLog
- Lookup
- .logger
- Eval baseline — DESIGN rev 21 harness pass
- evals/run.sh
- StartError
- hints.go
- AllowAll
- reference/codes.go
- shipgate/run.sh
- askRootChecker
- boundedWriter
- fakeCmds
- Item
- NewManager
- ShellOutput
- TestWelcomeLines
- bugfix/check.sh
- feature/check.sh
- largefile/check.sh
- plan/check.sh
- refactor/check.sh
- vague/check.sh
- web/check.sh
- sync.Mutex
- .Run
- Config
- anthropic_test.go
- example.com/codes
- example.com/fetcher
- example.com/inventory
- example.com/shop
- example.com/slug
- example.com/temp
- newScript
- example.com/totals
- github.com/adeotek/moca
- encoding/json.RawMessage
- Start
- Model
- webFetch
- applog_test.go
- Manager
- run
- errorf
- Agent
- NewDefaultStore
- Sanitize
- LoadPrompts
- Message
- run
- update/update.go
- newFakeAS
- model
- Input
- BASELINE.md
- context.Context
- Request
- drained
- charm.land/bubbletea/v2.Msg
- format.go
- moca/update_test.go
- NewRegistry
- newAgentModel
- system-prompt.md
- cut_test.go
- NewRegistry
- Summarize
- .startSession
- .Compact
- newTestModel
- update/update_test.go
- .DiscoverOllama
- MCPServer
- readSSE
- Create
- .openSessions
- Find
- moca/log_test.go
- permissions/shell.go
- renderMarkdown
- retry_test.go
- shipgate/fixture/main.go
- 2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)
- .Run
- time.Duration
- charm.land/bubbletea/v2.Cmd
- .View
- post
- .Run
- input_test.go
- Render
- compact/entry.go
- runOneShot
- fetch_test.go
- fixture/order.go
- SOURCES.md
- create-command.md — starter prompt template
- CI Workflow (gofmt · vet · build · test)
- Claude Code frontend-design skill
- pi mcp-scripting skill
- CI gate (.github/workflows/ci.yml)
- DESIGN.md - v1 contract/vision
- Git flow: main is PR-only (local .git/hooks/pre-push blocks pushes)
- Built-in rtk skill (SKILL.md)
- Arrange-Act-Assert with xUnit/NSubstitute
- Strict internal import direction
- Cross-provider thinking-block transforms in the agent
- Release - build packages, publish GitHub Release
- README.md - moca
- External tools - rtk, graphify, skills ecosystem
- Context budgets are token-denominated because a percentage conflates a 200K and a 1M window; the catalog already knows the window; budgets config-overridable
- Verified live endpoints: authorize/token/revoke URLs, JWKS RS256, resource https://api.openai.com/v1
- Plan mode (rev 20): a run analyzes the request and the code then writes an implementation plan to docs/plans and stops; a scope, not a permission
- Decision: api_key only. Enforced in config validation with recorded reason (DESIGN rev 12)
- Claude Code skill-creator skill
- internal/provider/auth.go credential store — ~/.config/moca/auth.json (file 0600, dir 0700; the write forces 0600 even over a stale tmp file and the rename deliberately replaces a symlink rather than following it): providers.<name> holds the OAuth token (access/refresh, expiry, account/client id, id token, email, scopes) and apiKeys.<name> holds API keys from /login or moca login (validated: empty values and control bytes refused, so a credential can never smuggle a newline into a header); atomic writes (temp + rename)
- Bet: a tight agent loop plus disciplined context handling beats a feature-rich agent on cost-per-task for everyday coding (read, edit, run tests, commit)
- Phase-3 gate (2026-10-05): a mock-driven TUI session in a real tmux terminal against a scripted openai-completions server — status bar from the first frame, branch switch, streamed commits, items, slash commands (/help /cost /model /effort /hard /copy /show /compact), missing-key switch refused, newline hint, 120-line paste chip expand send, steering verified in the CAPTURED REQUEST PAYLOAD, approvals (a ran; A appended to shell.allow byte-preserving + .bak; rm prompted without [A]), /yolo on/off incl. a 60-column bar keeping YOLO, ! local, /undo, esc interrupting a tool, /clear; it found and fixed two real bugs (event-pipe deadlock on /yolo; styled prints mangled by Sanitize)
- Commands: mise install provisions Go 1.27.1 pinned in go.mod with PATH export because go is not on the global PATH on every host; make build produces bin/moca version-stamped from git describe; make test runs go test with race and count; make vet runs go vet plus gofmt with both clean; make fmt; make release builds five cross-compiled binaries in dist; bin moca update with check compares with the latest GitHub release; mise run tasks mirror the make targets
- Execution: calls of one turn run SEQUENTIALLY in emitted order; a failed call does not cancel the rest; every call gets exactly one tool_result; unknown tool → error listing available tools; invalid JSON arguments → error telling the model to split the work; a panic inside a tool is recovered by Registry.Run into an error result ('internal error in <tool>: … — this is a moca bug, not your input; the call may have partly applied …') instead of crashing the session (rev 21 — eval feature#3 crashed on the edit-diff bug)
- AGENTS.md headline: one Go binary, a minimal TUI coding agent at github.com/adeotek/moca with binary name moca; 220 Go files across 12 internal packages
- Single package or test via go test with the package path run flag race count
- Richer activity row with tool operand, streamed-token estimate and queued steering count; pager search; an approval pending marker; shift-tab cycles effort; a desktop notification or bell for a finished long run or a waiting approval while the terminal is unfocused via tui.notify defaulting to osc9
- At path completes workdir files gitignore-aware; the path is sent and the model reads the file itself
- slash copy keeps the last answer when the final turn had no text; failed tool items get a red cross; thinking line count singular; the pager title clamped to one row showing the scroll position
- ctrl c on an empty draft says press ctrl c again to quit; inside the pager it now also clears the textarea so the draft does not come back on the next key
- Exit keeps the final status bar on screen and prints the session id8 with the resume hint
- Inline edit diffs: a successful edit prints its changed lines in red and green up to 8 under its item line
- Calmer layout: a blank row between tool items and the response, messages hanging their text, one-row item lines, bang command echoes, a per-machine shift-enter hint; scrollback colors follow the terminal profile with NO_COLOR honored and 256-color terminals getting approximated band colors instead of raw truecolor
- Markdown-lite responses: bold, code, headings, bullets, quotes and fenced code blocks in a darker band never word-wrapped instead of raw markdown; slash copy still copies the raw text
- Persistent prompt history in a per-workdir history jsonl file plus ctrl-r history search with type to filter and enter using the match without sending
- Changelog frontmatter: the version history document
- Eval scenario `bugfix`: two bugs behind failing tests, tests must stay untouched
- Eval scenario `feature`: a --json flag across files + tests, the table output byte-identical
- Eval scenario `largefile`: a bug at line ~1410 of a 1423-line generated file, the table untouched
- Unit/integration: go test ./... -race — 685 top-level tests across 16 Go packages (count established via `go test ./... -count=1 -v | grep -c '^=== RUN'`); phases 1-5 keep passing; growth recorded: 585 before the web-tool/plan-mode suites → 613 rev-21 harness pass → 640 system-prompt template extraction → 641 overflow-per-session → 643 branch-review pins → 646 diagnostic-log suites → 679 web private-address + branch-review pins → 685
- Eval scenario `plan`: a --plan run must produce ≥3 checkbox steps naming fetch.go and change nothing else
- Eval scenario `refactor`: shared email validation in validate.go, the duplicate gone
- Eval scenario `vague`: "add input validation" — judged by CLI behaviour incl. below-absolute-zero
- Eval scenario `web`: the current Go release, checked live against go.dev with the web tool used
- --approve/--no-approve — explicit project trust for this run (default: no; --yolo implies yes unless --no-approve)
- --config <path> — dev/test config file override (hidden from help text)
- provider.NewRegistry(cfg, httpClient, notify) — builds the catalog (built-ins + config overrides/additions) and validates that model/modelHard resolve
- Known limitation: terminals that reflow on shrink (tmux, kitty, iTerm2, WezTerm, VTE) re-wrap old full-width frame rows before the renderer erases them, which can leave one duplicate rule row above the input box; the next frame is correct; compensating needs terminal-specific knowledge (a wrong guess erases scrollback)
- Append-only JSONL transcript at ~/.local/share/moca/sessions/<date>-<slug>-<id8>.jsonl (file 0600, dir 0700); every entry has id, parentId (chain), type, ts; types: session (header: workdir, provider, model, EFFECTIVE effort, startedAt, mocaVersion, the FULL system prompt, yolo), message, tool_use, tool_result, compaction (summary + firstKeptEntryId + tokensBefore + usage + cumulative readFiles/modifiedFiles — its usage/cost count toward session totals and are restored on resume), snapshot, error, permission_mode, model_change (written by /model, /effort, /hard), elision (rev 21: {ids, tokens} — tool_result entry ids superseded later)
- Install via go install github.com/adeotek/moca/cmd/moca at latest
- Optional TUI knob: tui notify osc9, bell or off — a desktop notification or bell when a run longer than 30 seconds ends or an approval is waiting, only while the terminal is unfocused
- Skills: the Agent Skills standard, same shape as Pi Claude Code and OpenCode, three sources in precedence order where the earlier wins on name collision
- Plan mode (rev 20) in the tools: with Env.Plan set, write/edit reach only <workdir>/docs/plans/*.md (everything else → `refused: plan mode only writes docs/plans/*.md …`; the ask-write roots stay closed — nothing is asked) and a successful plan-file write/edit sets Env.PlanWrote; read/search/ls/web and shell/MCP stay available under the normal ladder (2026-10-09 follow-up: planning needs git log, builds, docs lookups); the failing-test banner/refusal is suppressed in plan mode (plan runs inspect, they don't fix); the confinement is NOT a permission — --yolo does not lift it; edit applies the plan-zone check BEFORE the ask-write approval (plan mode never asks)
- Permissions threat model stated plainly: guard against model mistakes and casual prompt injection; NOT a sandbox because an allowlisted interpreter can do anything the user can; OS-level sandboxing is a v2 discussion
- Phase 7 — subscription OAuth behind the DESIGN §3 policy gate: anthropic ships api_key only, openai ships SIWC token sharing on the openai-responses route with dynamic client registration, PKCE S256, nonce/RS256/JWKS validation, a 0600 token store with cross-process locked rotating refresh, moca login|logout; plus the automated ship gate and version stamping
- TUI built on Bubble Tea plus lipgloss, compact and dense per house style: tight padding, no banner boxes, a session opens with two compact welcome lines at the top of the scrollback (rev 13); without a configured model the welcome carries the red setup notice (rev 18)
- Eight tools frozen for v1: a new tool or parameter is a v2 discussion because each session re-reads schemas and tool-list churn kills prompt-cache reuse
- MCP support ships in v1 lazy by design (rev 3) because Ben uses MCP servers daily; learned from pi-mcp-adapter's measured results: 100 installed servers but 0 running at session start and about 200 tokens of prompt cost versus eager approaches costing 7 GB RAM and over 10k tokens
- Revision log: numbered revisions each a dated contract change; changes require a new revision, not silent drift; rev 1 through rev 23 recorded
- SPECS.md — moca's living normative spec (16 numbered sections, updated with every behavioral PR)
- Ship gate for the v0.1 demo, one session on the opencode-go provider proving multi-provider from day 1: read a failing test, locate the bug with search, edit the fix, run go test via shell and see green, commit on a feature branch, and the built-in rtk skill was discoverable and followed
- Repository layout: cmd/moca plus twelve internal packages
- Session entry types: session first entry, message, tool_use and tool_result, compaction, model_change, permission_mode, snapshot, error, elision (rev 21)
- skills.LoadPrompts reads <dir>/<name>.md (optional frontmatter description/argument-hint; first dir wins a collision; sorted); ExpandPrompt substitutes $ARGUMENTS/$@ (all args), $1..$9 (whitespace-split) and $$ (literal $); Claude Code command files (allowed-tools/model keys ignored) and pi templates load unchanged (testdata/prompts/, TestEcosystemPromptsLoad); skills.SeedUserPrompts(config.PromptsDir()) materializes the embedded starter template on the first TUI run (best-effort, stderr warning on failure) — never overwriting an existing file (O_EXCL); the global prompts dir is a jail read root and ask-write root
- Cross-provider history transform applied when building every request so /model and /hard can switch mid-session: thinking blocks replay unchanged only to the exact model that produced them; thinking from any other model becomes a plain text block labelled prior reasoning; the transcript keeps the originals
- `moca update [--check]` — replaces the running binary with the newest GitHub release package via internal/update
- test/evals — the rev 21 eval corpus: seven scenarios, each a fixture repo (committed base), prompt.txt, check.sh judged against the base commit (SESSION names the transcript) and optional flags; not in CI, same .env key; harness changes show their delta against BASELINE.md

## God Nodes (most connected - your core abstractions)
1. `SPECS.md — moca's living normative spec (16 numbered sections, updated with every behavioral PR)` - 151 edges
2. `model` - 75 edges
3. `newScript()` - 64 edges
4. `newTestModel()` - 63 edges
5. `run()` - 62 edges
6. `key()` - 44 edges
7. `newAgentModel()` - 43 edges
8. `textTurn()` - 41 edges
9. `Unit/integration: go test ./... -race — 685 top-level tests across 16 Go packages (count established via `go test ./... -count=1 -v | grep -c '^=== RUN'`); phases 1-5 keep passing; growth recorded: 585 before the web-tool/plan-mode suites → 613 rev-21 harness pass → 640 system-prompt template extraction → 641 overflow-per-session → 643 branch-review pins → 646 diagnostic-log suites → 679 web private-address + branch-review pins → 685` - 41 edges
10. `Config` - 39 edges

## Surprising Connections (you probably didn't know these)
- `Scenario: vague (6 runs - highest swing, stays hard)` --semantically_similar_to--> `Locate the cause with search before editing; fix root cause`  [INFERRED] [semantically similar]
  test/evals/BASELINE.md → internal/agent/system-prompt.md
- `Permissions threat model stated plainly: guard against model mistakes and casual prompt injection; NOT a sandbox because an allowlisted interpreter can do anything the user can; OS-level sandboxing is a v2 discussion` --semantically_similar_to--> `Debugging and logs: moca keeps a small diagnostic log per run under the state moca logs directory recording what moca did like retries, MCP servers starting and stopping, and how a run ended, but never prompts, code or keys; attach the newest file to a bug report; MOCA_LOG debug adds per-run detail; log level off turns it off; old logs removed after snapshot retention days at 30 by default`  [INFERRED] [semantically similar]
  docs/specs/DESIGN.md → README.md
- `OAuth policy gate (oauth-verification.md)` --references--> `External tools - rtk, graphify, skills ecosystem`  [INFERRED]
  AGENTS.md → docs/specs/external-tools.md
- `System prompt built once per run, never rebuilt` --semantically_similar_to--> `Skills mechanism (SKILL.md directories)`  [INFERRED] [semantically similar]
  AGENTS.md → docs/specs/external-tools.md
- `Bet: a tight agent loop plus disciplined context handling beats a feature-rich agent on cost-per-task for everyday coding (read, edit, run tests, commit)` --semantically_similar_to--> `Competes on cost per task and transparency: prompt caching, windowed reads, diff-shaped results and token-denominated compaction keep a session cheap; MCP support that never floods the prompt via one 200-token lazy proxy; loads Agent-Skills SKILL.md files written for other tools unchanged; searches the web and fetches pages without another tool with keyless default; prefers external token-savers rtk and graphify when installed`  [INFERRED] [semantically similar]
  docs/specs/DESIGN.md → README.md

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Agent loop input pipeline** — docs_specs_specs_turn_contract, docs_specs_specs_cross_provider_transform, docs_specs_specs_system_prompt_node, docs_specs_specs_frozen_tools [EXTRACTED 1.00]
- **Credential resolution pipeline** — docs_specs_specs_env_indirection, docs_specs_specs_store_first_credential, docs_specs_specs_credential_store, docs_specs_specs_login_wizard [EXTRACTED 1.00]
- **Tracked read/edit staleness gate** — docs_specs_specs_read_tool, docs_specs_specs_write_tool, docs_specs_specs_edit_tool, docs_specs_specs_swe_agent_read_guard [EXTRACTED 1.00]
- **Ecosystem SKILL.md compatibility corpus (graphify, Claude Code, pi, OpenCode)** — internal_skills_testdata_ecosystem_sources, internal_skills_testdata_ecosystem, internal_skills_testdata_ecosystem_graphify_graphify_vendored, internal_skills_testdata_ecosystem_claude_code_skill_creator_vendored, internal_skills_testdata_ecosystem_claude_code_frontend_design_vendored, internal_skills_testdata_ecosystem_pi_pi_subagents_vendored, internal_skills_testdata_ecosystem_pi_mcp_scripting_vendored, internal_skills_testdata_ecosystem_opencode_dotnet_unit_testing_vendored [EXTRACTED 1.00]
- **The permission surface: jail, shell analysis, allowlist, trust, yolo, plan gating** — docs_specs_design_path_jail, docs_specs_design_shell_command_analysis, docs_specs_design_project_trust, docs_specs_design_yolo_mode, docs_specs_design_plan_write_confinement, docs_specs_design_ask_write_root [EXTRACTED 1.00]
- **Frozen tool set and prompt-cache discipline across DESIGN, CHANGELOG and AGENTS** — docs_specs_design_eight_frozen_tools, docs_specs_design_schema_churn_cost_rule, changelog_seven_frozen_tools, agents_eight_frozen_tools_rule, docs_specs_design_system_prompt_built_once [EXTRACTED 1.00]
- **The self-update and release pipeline chain** — changelog_install_scripts, changelog_moca_update, changelog_release_workflow, readme_self_update, agents_release_workflow, docs_specs_design_rev16 [EXTRACTED 1.00]
- **Synthetic frontmatter edge-case fixtures (CRLF, BOM, folding, quoting, rich keys, long description)** — internal_skills_testdata_ecosystem_synthetic_bom_skill, internal_skills_testdata_ecosystem_synthetic_colon_unquoted_skill, internal_skills_testdata_ecosystem_synthetic_crlf_skill, internal_skills_testdata_ecosystem_synthetic_folded_skill, internal_skills_testdata_ecosystem_synthetic_literal_skill, internal_skills_testdata_ecosystem_synthetic_long_description_skill, internal_skills_testdata_ecosystem_synthetic_rich_keys_skill [EXTRACTED 1.00]
- **OAuth policy gate path: verify terms -> per-provider decision -> config enforcement** — docs_specs_oauth_verification_oauth_verification_record, docs_specs_oauth_verification_anthropic_api_key_only_decision, docs_specs_oauth_verification_ship_oauth_openai_decision [INFERRED 0.75]
- **Human-in-the-loop validation before declaring done** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_claude_code_frontend_design_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_review_validation [INFERRED 0.75]
- **Parallel subagent fanout with parent synthesis** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_graphify_graphify_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_skill [INFERRED 0.85]
- **Skills referencing bundled reference files for progressive disclosure** — internal_skills_testdata_ecosystem_claude_code_skill_creator_skill, internal_skills_testdata_ecosystem_graphify_graphify_skill, internal_skills_testdata_ecosystem_pi_mcp_scripting_skill, internal_skills_testdata_ecosystem_pi_pi_subagents_skill [INFERRED 0.85]
- **Seven-scenario eval corpus** — test_evals_baseline, agents_eval_corpus, test_evals_baseline_evalstats, test_evals_baseline_scenario_bugfix, test_evals_baseline_scenario_feature, test_evals_baseline_scenario_largefile, test_evals_baseline_scenario_plan, test_evals_baseline_scenario_refactor, test_evals_baseline_scenario_vague, test_evals_baseline_scenario_web [INFERRED 0.95]
- **rtk allowlist unwrapping and its subcommand classes** — docs_specs_external_tools_rtk, docs_specs_external_tools_rtk_unwrapping, docs_specs_external_tools_rtk_read_only_class, docs_specs_external_tools_rtk_write_config_class, docs_specs_external_tools_rtk_native_proxy_class, docs_specs_external_tools_rtk_shell_string_refusal, internal_permissions_rtk [INFERRED 0.95]
- **Token-efficiency mechanisms implementing the minimal-agent philosophy** — readme_minimal_tui_coding_agent, readme_prompt_caching, internal_agent_system_prompt_windowed_read, readme_diff_shaped_results, readme_token_denominated_compaction, internal_agent_system_prompt_token_discipline, readme_rtk_token_saver, readme_graphify_token_saver [INFERRED 0.95]

## Communities (204 total, 86 thin omitted)

### Community 25 - "v0.1.0-alpha (2026-10-07)"
Cohesion: 0.08
Nodes (28): CHANGELOG, CLI, Context & sessions, External tools & the skills ecosystem, MCP, OAuth (subscription login), Providers & protocols, Tools & permissions (+20 more)

### Community 0 - "go_pkg_strings"
Cohesion: 0.06
Nodes (80): allowAll, Compacted, PlanChanged, PromptInput, Resumed, ServerLine, SteeringApplied, StreamReset (+72 more)

### Community 1 - "testing.T"
Cohesion: 0.03
Nodes (92): NormalizeToolID(), TestNormalizeToolID(), TestTransformDropsEmptiedAssistantAndMerges(), TestTransformDropsRedactedForeignThinking(), TestTransformHistory(), TestTransformSameBareIDDifferentProvider(), TransformHistory(), CapChars() (+84 more)

### Community 101 - "NewUnjailed"
Cohesion: 0.29
Nodes (4): Agent, NewUnjailed(), TestUnjailedAndAllowAll(), Unjailed

### Community 102 - "reference/order.go"
Cohesion: 0.32
Nodes (5): Order, User, NewOrder(), NewUser(), normalizeEmail()

### Community 105 - "Snapshots"
Cohesion: 0.29
Nodes (4): gitClean(), Snapshots, NewSnapshots(), SnapshotRec

### Community 107 - "history.go"
Cohesion: 0.43
Nodes (6): appendHistory(), loadHistory(), rewriteHistory(), TestHistoryPersistence(), hasControl(), historyRec

### Community 108 - ".RunPlan"
Cohesion: 0.40
Nodes (4): doNudge(), doPrompt(), Agent, uncheckedSteps()

### Community 109 - "install.sh script"
Cohesion: 0.67
Nodes (5): die(), fetch(), note(), install.sh script, usage()

### Community 11 - "Parse"
Cohesion: 0.06
Nodes (46): scanner, span, Parse(), TestAPIKeyOptional(), TestDefaultsApplied(), TestEnvRefs(), TestOAuthAllowedForOpenAI(), TestOAuthRejectedForNonOAuthProvider() (+38 more)

### Community 110 - "Serialize"
Cohesion: 0.40
Nodes (5): callLine(), safeTail(), Serialize(), TestSerialize(), TestSerializeRuneSafeAndEmptyText()

### Community 111 - "serverEnv"
Cohesion: 0.40
Nodes (6): TestKeepNameWindows(), FilterEnv(), keepName(), serverEnv(), TestFilterEnv(), TestServerEnvExplicitEntryReplacesInherited()

### Community 112 - "TestModelPickerRefreshesOllama"
Cohesion: 0.33
Nodes (6): contains(), count(), dropNames(), TestDropdownListsCommandsAndPrompts(), TestModelPickerRefreshesOllama(), TestShowCompletionNewestFirst()

### Community 113 - "fixture/slug.go"
Cohesion: 0.40
Nodes (4): Limit(), Make(), TestLimit(), TestMake()

### Community 116 - "captureLog"
Cohesion: 0.50
Nodes (4): captureLog(), logPolicy(), TestProviderLogsFinalFailureAtInfo(), TestProviderLogsRetryAndResponse()

### Community 117 - "Lookup"
Cohesion: 0.50
Nodes (3): Lookup(), TestLookupBounds(), Valid()

### Community 121 - "Eval baseline — DESIGN rev 21 harness pass"
Cohesion: 0.50
Nodes (4): Eval baseline — DESIGN rev 21 harness pass, Found by the evals (fixed in rev 21), Reading it, Results

### Community 122 - "evals/run.sh"
Cohesion: 1.00
Nodes (3): fresh_repo(), one(), run.sh script

### Community 14 - "NewManager"
Cohesion: 0.11
Nodes (40): crashingServer(), shServer(), skipWindows(), TestCallTimeoutStopsWedgedServer(), TestHandshakeTimeout(), TestHTTPAbortedStreamReplayRule(), TestIndexSaveMergesConcurrentWriters(), TestListToolsRepeatedCursor() (+32 more)

### Community 15 - "sync.Mutex"
Cohesion: 0.06
Nodes (17): logSink, cappedWriter, wc, cutRecord(), callResult, stampOf(), logSink, response (+9 more)

### Community 16 - ".Run"
Cohesion: 0.06
Nodes (28): applyEdit(), formatDiff(), indentOf(), joinInts(), lineOf(), matchLines(), replaceExact(), replaceFuzzy() (+20 more)

### Community 18 - "Config"
Cohesion: 0.08
Nodes (23): ContextConfig, LogConfig, MCPConfig, ShellConfig, SnapshotConfig, TUIConfig, WebConfig, WebSearchConfig (+15 more)

### Community 19 - "anthropic_test.go"
Cohesion: 0.16
Nodes (37): Adapter, newAnthropic(), keyCred(), mustJSON(), sseServer(), TestAnthropicBodyHygiene(), TestAnthropicBudgetAndNoCache(), TestAnthropicBudgetFollowsEffort() (+29 more)

### Community 2 - "newScript"
Cohesion: 0.07
Nodes (83): scriptServer, newScript(), startTest(), startTestWith(), startWith(), TestAbortWritesSyntheticResults(), TestFailedCallDoesNotCancelRest(), TestLoopReadEditVerify() (+75 more)

### Community 21 - "encoding/json.RawMessage"
Cohesion: 0.07
Nodes (15): ToolSpec, CallResult, responseID(), TestResponseID(), TestProxySpecFrozen(), MCPSpec(), TestRegistryDispatch(), Content (+7 more)

### Community 22 - "Start"
Cohesion: 0.09
Nodes (33): setup, main(), decideTrust(), hasProjectResources(), runTUI(), TestPruneOverflow(), TestSpillDirsAreRandom(), planMode() (+25 more)

### Community 24 - "Model"
Cohesion: 0.08
Nodes (8): savedModel, Status, Agent, Effort, ParseEffort(), TestParseEffort(), Model, supportedEfforts()

### Community 27 - "webFetch"
Cohesion: 0.09
Nodes (26): Spill(), cutWeb(), isAlnumByte(), isSpaceByte(), TestWebDataAttrsAreNotLinkAttrs(), TestWebHTMLConverters(), TestWebHTMLStrayClosingPre(), TestWebPrivateHost() (+18 more)

### Community 28 - "applog_test.go"
Cohesion: 0.09
Nodes (23): Level, nopCloser, startLog(), Discard(), Open(), ParseLevel(), prune(), keepDefault() (+15 more)

### Community 29 - "Manager"
Cohesion: 0.11
Nodes (14): ConfigHash(), Rank(), TestRank(), words(), Manager, startFailClass(), Annotations, client (+6 more)

### Community 3 - "run"
Cohesion: 0.05
Nodes (69): fakeCompletions(), isolate(), TestExitCodes(), TestHelpListsSurface(), TestInterruptedExit130(), TestLoginLogoutCLI(), TestMCPImportCLI(), TestNoApproveSkipsProjectInstructions() (+61 more)

### Community 30 - "errorf"
Cohesion: 0.10
Nodes (21): TestProxyPlanModeCallFollowsNormalGating(), cutRunes(), errResult(), isTrue(), NewTool(), newProxy(), runP(), TestCutRunesSafe() (+13 more)

### Community 31 - "Agent"
Cohesion: 0.13
Nodes (14): Options, ToolEnd, ToolStart, Agent, Outcome, New(), userText(), verifyNudge() (+6 more)

### Community 32 - "NewDefaultStore"
Cohesion: 0.11
Nodes (12): NewDefaultStore(), OAuthProvider(), model, providerNames(), providerRow(), TestLoginRowForOllama(), loginChoice, loginDoneMsg (+4 more)

### Community 33 - "Sanitize"
Cohesion: 0.09
Nodes (20): AutoAllow(), Answer, Question, approvalPanel(), approvalPrompt(), TestApprovalPromptSanitized(), diffPreview(), TestDiffPreview() (+12 more)

### Community 34 - "LoadPrompts"
Cohesion: 0.08
Nodes (30): builtinHash(), ExtractBuiltins(), extractBuiltins(), TestEcosystemSkillsLoadUnchanged(), ParseFrontmatter(), TestFrontmatter(), TestFrontmatterBlockAndQuoteEdges(), TestFrontmatterBlockIndicatorsAndComments() (+22 more)

### Community 35 - "Message"
Cohesion: 0.11
Nodes (23): TurnEnd, Result, withPlanEnvelope(), MessageChars(), RequestChars(), TestEstimate(), UsageTokens(), Message (+15 more)

### Community 36 - "run"
Cohesion: 0.12
Nodes (24): parseArgs(), printUsage(), TestParseArgs(), loginAPIKey(), runLogin(), runLogout(), yesNo(), run() (+16 more)

### Community 37 - "update/update.go"
Cohesion: 0.12
Nodes (20): envOr(), githubToken(), runUpdate(), Compare(), comparePre(), ExtractBinary(), FindAsset(), FindChecksums() (+12 more)

### Community 4 - "newFakeAS"
Cohesion: 0.06
Nodes (42): TestDefaultHTTPClientBoundsHeaderWait(), defaultHTTPClient(), Store, newHostID(), validateAPIKey(), audContains(), b64d(), OAuthConfig (+34 more)

### Community 40 - "model"
Cohesion: 0.11
Nodes (17): firstLineOf(), clampRunes(), model, matchCommands(), parseSlash(), slashQuery(), TestClampRunes(), TestSlashQuery() (+9 more)

### Community 45 - "Input"
Cohesion: 0.13
Nodes (4): lineCount(), Input, padGuard, paste

### Community 51 - "BASELINE.md"
Cohesion: 0.13
Nodes (22): release notes, Seven-scenario eval corpus (test/evals/), baseline label (pre-rev-21 harness), edit diff panic on trailing blank line, Empty-reply retry, final label (DESIGN rev 21), Investigation-protocol scope fix, 400-line read window (+14 more)

### Community 52 - "context.Context"
Cohesion: 0.13
Nodes (13): fakeSum, initialize(), TestHTTPCloseBounded(), startHTTP(), TestHTTPNotifyStatusChecked(), TestHTTPTransport(), readEvents(), lockFile() (+5 more)

### Community 53 - "Request"
Cohesion: 0.16
Nodes (14): Event, Request, Response, causeClass(), TestCauseClass(), isDialFailure(), ollamaUnreachable(), openaiOverflow() (+6 more)

### Community 54 - "drained"
Cohesion: 0.19
Nodes (19): drained(), newLoginModel(), TestLoginMarkedBackAndCancel(), TestLoginOAuthDeclinedFlipAndCancel(), TestLoginOAuthFlowAndAuthFlip(), TestLoginRefusedWhileRunning(), TestLoginStoresAPIKey(), TestLogoutClearsStoredCredential() (+11 more)

### Community 55 - "charm.land/bubbletea/v2.Msg"
Cohesion: 0.11
Nodes (10): Event, Asker, newAsker(), flatten(), TestMatchRanges(), matchRanges(), newPager(), agentEventMsg (+2 more)

### Community 56 - "format.go"
Cohesion: 0.13
Nodes (17): TestStatusWideRunesNeverExceedWidth(), AbbrevHome(), FmtTokens(), FmtWindow(), TestFormatters(), trimZero(), FmtPercent(), renderLine() (+9 more)

### Community 57 - "moca/update_test.go"
Cohesion: 0.16
Nodes (17): fakeReleases(), setVersion(), TestRunUpdateCheck(), TestRunUpdateFetchFails(), TestRunUpdateLocalNewer(), TestRunUpdateMissingPlatformPackage(), TestRunUpdateNoVersionStamp(), TestRunUpdateUpToDate() (+9 more)

### Community 58 - "NewRegistry"
Cohesion: 0.12
Nodes (14): Builtins(), captureLog(), TestSpillLogged(), TestToolCallLoggedWithoutContent(), TestToolPanicLogsStack(), TestRegistryRecoversToolPanic(), TestRepeatedFailureIgnoresSpillPath(), runReg() (+6 more)

### Community 6 - "newAgentModel"
Cohesion: 0.06
Nodes (60): keyMsg(), TestExitCommandQuits(), TestCompactEscapeCancels(), TestCompactingBlocksRun(), TestCompactNothing(), TestEditItemShowsDiff(), TestThinkingLinePrintsBeforeTheResponse(), TestAllowAlwaysRequiresCtrl() (+52 more)

### Community 61 - "system-prompt.md"
Cohesion: 0.14
Nodes (16): Environment, Token discipline, Tools, Working style, edit prefers small exact replacements; read before edit, Don't echo file contents or tool output; summarize, Do not retry refused variants, Orient first; reuse the existing pattern (+8 more)

### Community 62 - "cut_test.go"
Cohesion: 0.29
Nodes (15): FindCut(), a(), call(), res(), TestFindCutKeepsRecentAtValidPoint(), TestFindCutNeverBetweenCallAndResult(), TestFindCutNothingToDo(), TestFindCutSplitTurn() (+7 more)

### Community 63 - "NewRegistry"
Cohesion: 0.19
Nodes (20): TestNoOllamaNoProbe(), TestOllamaVerify(), NewRegistry(), mustCfg(), TestAPIKeyMissingMentionsLogin(), TestAPIKeyStoreWinsOverEnv(), TestBaseURLPrecedence(), TestBuiltinModelNeedsProtocol() (+12 more)

### Community 66 - "Summarize"
Cohesion: 0.15
Nodes (15): Metrics, Collect(), fatal(), main(), meanSD(), readResults(), Summarize(), TestCollect() (+7 more)

### Community 68 - ".startSession"
Cohesion: 0.16
Nodes (8): modelPickItems(), TestWindowRange(), model, windowRange(), model, noProviderNotice(), pickItem, pickState

### Community 69 - ".Compact"
Cohesion: 0.21
Nodes (7): Agent, summarizerUnavailable(), Budget, NewBudget(), TestBudget(), Prev, Summarizer

### Community 7 - "newTestModel"
Cohesion: 0.06
Nodes (59): TestActivityLine(), TestCoalescedShrinkRemembersTheLastFlushedRow(), TestInputBoxGrowsAndShrinks(), TestPagerCloseDefersTheRelease(), TestRefuseRunningWhileOutputInFlight(), TestShrinkIsPaddedUntilTheGuardWindowPasses(), TestShrinkPaddingFitsTheScreen(), TestSpinnerTicksOnlyForTheLiveRun() (+51 more)

### Community 71 - "update/update_test.go"
Cohesion: 0.19
Nodes (15): IsTemporary(), sameHost(), installServer(), tarGz(), TestExtractBinary(), TestFindAsset(), TestInstallChecksumEntryMissing(), TestInstallChecksumMismatch() (+7 more)

### Community 72 - ".DiscoverOllama"
Cohesion: 0.13
Nodes (11): Registry, modelFromShow(), normalizeOllamaURL(), ollamaBaseURL(), ollamaRoot(), TestNormalizeOllamaURL(), TestOllamaBaseURLPrecedence(), countingTransport (+3 more)

### Community 73 - "MCPServer"
Cohesion: 0.16
Nodes (17): MCPServer, TestImportRewritesEmbeddedSecretsInValues(), TestRewriteSecretsValueRuleAlone(), DiscoverSources(), embeddedSecret(), envName(), literalSecretFields(), ParseSource() (+9 more)

### Community 74 - "readSSE"
Cohesion: 0.15
Nodes (14): ContentBlock, StopReason, ToolResult, anthropicOverflow(), anthropicStop(), priorReasoningText(), replaysVerbatim(), toolInput() (+6 more)

### Community 75 - "Create"
Cohesion: 0.19
Nodes (9): lockFile(), lockFile(), Create(), Writer, lastNewline(), newID(), Open(), TestOpenRefusesWhileAnotherWriterHoldsTheFile() (+1 more)

### Community 77 - ".openSessions"
Cohesion: 0.19
Nodes (11): Delete(), TestDelete(), TestListAll(), Info, List(), ListAll(), TestListPreviewsAndFilters(), fmtAge() (+3 more)

### Community 78 - "Find"
Cohesion: 0.17
Nodes (16): canonical(), Find(), FindForWorkdir(), header(), headerAndPreview(), idOf(), InWorkdir(), list() (+8 more)

### Community 79 - "moca/log_test.go"
Cohesion: 0.27
Nodes (14): logRun(), logRunCfg(), logRunCfgIn(), logRunIn(), readLog(), sessionID8(), TestLogDebugViaEnv(), TestLogFileForOneShot() (+6 more)

### Community 8 - "permissions/shell.go"
Cohesion: 0.07
Nodes (41): canonical(), Jail, NewJail(), resolveDeep(), setup(), TestJail(), TestJailApprovable(), TestJailDanglingSymlinks() (+33 more)

### Community 82 - "renderMarkdown"
Cohesion: 0.23
Nodes (12): codeBandStyle(), fenceLang(), renderInline(), renderMarkdown(), plainMD(), TestMarkdownBlocks(), TestMarkdownFence(), TestMarkdownSanitizes() (+4 more)

### Community 85 - "retry_test.go"
Cohesion: 0.44
Nodes (12): fail(), TestMidStreamFailureResetsAndRetriesOnce(), TestMidStreamRetryBacksOffAndNotifies(), TestNoRetryOn400(), testPolicy(), TestRetryAfterCapped(), TestRetryBackoffThenSuccess(), TestRetryGivesUpAfterFive() (+4 more)

### Community 86 - "shipgate/fixture/main.go"
Cohesion: 0.21
Nodes (7): Sum(), TestTotals(), ParseList(), Average(), Max(), TestMaxAndAverage(), main()

### Community 87 - "2. openai — decision: `ship oauth` (Sign in with ChatGPT — open-source token sharing)"
Cohesion: 0.15
Nodes (12): 1.1 Terms, 1.2 Client registration, 1.3 Decision, 1. anthropic — decision: `api_key only` (subscription OAuth not permitted), 2.1 Terms, 2.2 Client registration (per the docs), 2.3 Endpoints (verified live 2026-10-06 against `https://auth.openai.com/.well-known/openid-configuration`), 2.4 Inference contract for subscription traffic (open-source flow docs) (+4 more)

### Community 88 - ".Run"
Cohesion: 0.19
Nodes (8): globToRegexp(), ignored(), parseIgnore(), TestIgnore(), ListFiles(), TestListFiles(), ignoreRule, searchTool

### Community 89 - "time.Duration"
Cohesion: 0.21
Nodes (9): Retry, DefaultRetryPolicy(), RetryNotice, TestFmtElapsed(), fmtElapsed(), fmtThinkDuration(), Options, HTTPError (+1 more)

### Community 9 - "charm.land/bubbletea/v2.Cmd"
Cohesion: 0.11
Nodes (8): model, newModel(), Run(), AbbrevEffort(), AppOptions, compactDoneMsg, InputKind, Parsed

### Community 91 - "post"
Cohesion: 0.24
Nodes (7): fakeHTTPServer(), post(), newHTTPError(), retryAfter(), TestRetryAfterFormats(), TestRetryAfterParsed(), headerLog

### Community 92 - ".Run"
Cohesion: 0.20
Nodes (8): TestVerifies(), verifies(), approve(), TestTestRunCommand(), TestTestRunFailed(), testRunCommand(), testRunFailed(), shellTool

### Community 93 - "input_test.go"
Cohesion: 0.30
Nodes (11): NewInput(), TestCollapseUsesExactOffset(), TestCtrlC(), TestEditedMarkerSentLiterally(), TestHistory(), TestHistoryRecallChipsUnsafeContent(), TestMarkerNeverCollidesWithTypedText(), TestPasteChipKeepsContent() (+3 more)

### Community 94 - "Render"
Cohesion: 0.17
Nodes (7): jsonItem, main(), Render(), TestRenderTotal(), main(), RenderJSON(), TestRenderJSON()

### Community 95 - "compact/entry.go"
Cohesion: 0.25
Nodes (9): Kind, callKey(), Agent, staleResults(), TestStaleResults(), EntryTokens(), Entry, Tokens() (+1 more)

### Community 96 - "runOneShot"
Cohesion: 0.24
Nodes (10): TestExitForOAuthSentinels(), TestParseArgsDo(), exitFor(), firstLine(), oneShotSlug(), resolveResume(), runOneShot(), Slug() (+2 more)

### Community 97 - "fetch_test.go"
Cohesion: 0.24
Nodes (8): Client, New(), TestGet(), Plan: on-disk response cache, Cache type (dir, TTL) in cache.go, Client.Get in fetch.go uses the cache, TTL option on New, Eval prompt: plan an on-disk response cache

### Community 99 - "fixture/order.go"
Cohesion: 0.28
Nodes (6): Order, User, NewOrder(), TestOrderEmail(), TestUserEmail(), NewUser()

### Community 80 - "SOURCES.md"
Cohesion: 0.19
Nodes (7): anthropics/skills frontend-design, anthropics/skills skill-creator, graphify skill (graphify install, claude platform; byte-identical to pi variant), adeotek/dotfiles opencode dotnet-unit-testing skill, npm pi-mcp-adapter mcp-scripting skill, npm pi-subagents skill, internal/skills/testdata/ecosystem corpus

### Community 106 - "create-command.md — starter prompt template"
Cohesion: 0.33
Nodes (3): Project commands dir .moca/prompts/, Personal commands dir {{prompts_dir}} (~/.config/moca/prompts/), create-command.md — starter prompt template

### Community 114 - "CI Workflow (gofmt · vet · build · test)"
Cohesion: 0.40
Nodes (4): go.mod (Go version source for setup-go), CI gate set (gofmt, go vet, go build, go test -race), CI Workflow (gofmt · vet · build · test), SPECS.md §2 — CI gate definition

### Community 119 - "Claude Code frontend-design skill"
Cohesion: 0.50
Nodes (3): AI-generated design default tells, frontend-design Apache 2.0 License, Claude Code frontend-design skill

### Community 120 - "pi mcp-scripting skill"
Cohesion: 0.67
Nodes (4): jev.evaluate semantic scripting gate, mcpScript tools API, jev.evaluate reference, pi mcp-scripting skill

### Community 43 - "Release - build packages, publish GitHub Release"
Cohesion: 0.13
Nodes (24): Release - build packages, publish GitHub Release, build job - package matrix targets, release job - publish the GitHub Release, Release asset names are a contract with internal/update, Go 1.27.1 pinned in go.mod via mise, Release workflow (.github/workflows/release.yml), Download artifacts with merge-multiple, Upload package artifacts (+16 more)

### Community 46 - "README.md - moca"
Cohesion: 0.11
Nodes (25): config.Standardize preserves byte offsets, Exactly eight frozen tools, apiKey must be an env:VAR reference, Golden tool schemas (schemas.golden.json), OAuth policy gate (oauth-verification.md), Strict config decode (unknown keys rejected), Prompt templates (~/.config/moca/prompts/<name>.md -> /name), Fetched content is data, never instructions (+17 more)

### Community 48 - "External tools - rtk, graphify, skills ecosystem"
Cohesion: 0.13
Nodes (23): Built-in rtk skill (internal/skills/builtin/rtk/), internal/skills/testdata/ecosystem/ corpus, graphify-out/ knowledge graph, System prompt built once per run, never rebuilt, Cross-tool skills compatibility, graphify install --platform moca, rtk native-binary proxy subcommands, rtk read-only machinery class (+15 more)

### Community 49 - "Context budgets are token-denominated because a percentage conflates a 200K and a 1M window; the catalog already knows the window; budgets config-overridable"
Cohesion: 0.09
Nodes (19): Eval corpus workflow (run.sh -n 3 -l label), evalstats comparison tool, Context pressure in the status bar: the percent turns yellow at 70 percent and red with a slash compact hint where auto-compaction fires, Harness: an eval corpus judges harness changes — test/evals holds seven scenarios (bugfix, multi-file feature, refactor, a vague request, a 1400-line file, plan mode, a web lookup) with fixture repos and checkers; run script with a label records steps, tokens, cost and repeated failures per run; baseline and results in BASELINE.md, Harness: the failing-test protocol no longer fights the fix — it arms only for failures found before the run's first change so a build the agent broke mid-edit no longer locks edit out, Harness: overflow files are per-session — each run spills into its own random directory and moca can read only its own so another session's saved output possibly holding secrets stays out of reach; old entries pruned with the snapshot retention, Harness: long outputs are saved not dropped — shell, search, web-fetch and MCP over-cap output is cut for the model and saved in full to a file the result names, making smaller caps safe: shell 30K to 12K, web 60K to 20K, search shows 100 with 5000 saved, Harness: read returns 400 lines by default up to 2000 with limit, and superseded results replaced by a one-line stub once that saves at least 8K tokens (+11 more)

### Community 50 - "Verified live endpoints: authorize/token/revoke URLs, JWKS RS256, resource https://api.openai.com/v1"
Cohesion: 0.09
Nodes (22): Credential storage per docs: email, issuer, subject, issued client_id, host id, id_token, tokens, scopes; owner-only permissions, atomic writes, never logged or committed, First registration: client_id=dynamic_agent_client, agent_name_hint=moca, per-host ext_agent_host_id; issued client id (oaiapp_...) persisted and reused; never stored placeholder for exchange, ext_agent_host_id: stable opaque per-host id (urn:uuid: or JWK thumbprint); not a credential; multi-host per client id, Logout: POST revoke with refresh_token, token_type_hint=refresh_token, issued client_id; empty 200 = success, Token-endpoint auth method none (no secret) — confirmed in live discovery document, PKCE S256 only, response_type=code, nonce required; redirect URI loopback on 127.0.0.1 path /auth/callback (localhost rejected), Reauthorization: reuse issued client id (optional id_token_hint/login_hint); omit agent_name_hint; different callback id must be rejected, Refresh: grant_type=refresh_token, issued client_id, saved refresh_token, resource; omit scope to retain grant (+14 more)

### Community 60 - "Plan mode (rev 20): a run analyzes the request and the code then writes an implementation plan to docs/plans and stops; a scope, not a permission"
Cohesion: 0.12
Nodes (18): Implementation delta 8: single openai registration per provider (single-account simplification); switching accounts = logout then fresh dynamic registration, Plan mode (rev 20): a run analyzes the request and the code then writes an implementation plan to docs/plans and stops; a scope, not a permission, CLI landed in v0.1.0-alpha: moca for the TUI, moca p for one-shot with stdout the final text and exit codes 0 1 2 3 130, model effort approve no-approve yolo no-yolo resume continue version and help flags, moca login and logout, moca mcp import and index; the Makefile does version-stamped builds with git describe into ldflags, make test with race detection, and make release producing five cross-compiled binaries, Harness: slash do plan or do flag executes a plan written by plan mode step by step ticking each unchecked box as it is verified; long plan-mode runs are told to write their file before the step limit, Plan mode: plan flag or slash plan means the agent writes the plan not the code — a plan-mode run analyzes the request and codebase then writes an implementation plan to docs plans slug markdown with goal, current state, unchecked steps, verification, out of scope and risks; writes confined to docs plans markdown so nothing else is modified; shell, web and MCP stay available for investigation under their normal permission rules; slash plan toggles in the TUI with a blue PLAN status field and slash plan with a request plans immediately; a run that would finish without the file is nudged once then warned; the write confinement is not a permission so yolo does not lift it, Harness: prompt-cache friendlier — the plan-mode instruction no longer moves between messages and OpenAI requests carry a per-session prompt_cache_key, CLI surface: moca alone for the TUI; moca p prompt for one-shot accepting stdin; model effort approve no-approve yolo no-yolo plan resume continue flags; moca login and logout for OAuth; moca mcp import and mcp index; version flag, One-shot exit codes: 0 completed; 1 provider or runtime error; 2 config or usage error; 3 ended incomplete from maxSteps or stopped after the same failing call repeated 5 times with wrap-up text still printed; 130 interrupted (+10 more)

### Community 76 - "Decision: api_key only. Enforced in config validation with recorded reason (DESIGN rev 12)"
Cohesion: 0.15
Nodes (11): No registration path for third-party consumer-plan OAuth; using Anthropic's own client ids would be spoofing, Timeline: 2026-01 server-side OAuth checks, bans later partially reversed; 2026-02 clause formalized; 2026-04 subscription coverage ends for third-party tools; 2026-09 ban reaffirmed, Anthropic Claude Code legal docs quote: OAuth exclusively for subscription-plan purchasers of native apps; third-party developers must use API keys, Primary source cited: code.claude.com/docs/en/legal-and-compliance, Implementation delta 1: anthropic — no OAuth code; config validation + docs only, OAuth verification record (phase 7 §3 policy gate, retrieved 2026-10-06), Source cited: help.openai.com article 'Using your ChatGPT plan in other apps and sites', Partner directory lists OpenClaw, OpenCode, Pi, T3 under Open-source integrations (+3 more)

### Community 81 - "Claude Code skill-creator skill"
Cohesion: 0.16
Nodes (13): Skill description (trigger) optimization, Skill eval/iterate loop (with-skill vs baseline), SKILL.md anatomy + bundled resources layout, Constrained query expansion against graph vocab, Progressive Disclosure (three-level skill loading), Self-improving save-result/reflect loop, Fresh-context reviewer disposition, skill-creator Apache 2.0 License (Anthropic) (+5 more)

### Community 10 - "internal/provider/auth.go credential store — ~/.config/moca/auth.json (file 0600, dir 0700; the write forces 0600 even over a stale tmp file and the rename deliberately replaces a symlink rather than following it): providers.<name> holds the OAuth token (access/refresh, expiry, account/client id, id token, email, scopes) and apiKeys.<name> holds API keys from /login or moca login (validated: empty values and control bytes refused, so a credential can never smuggle a newline into a header); atomic writes (temp + rename)"
Cohesion: 0.06
Nodes (10): internal/provider/catalog.go — Model{Provider, ID, Protocol, ContextWindow, MaxOutput, Cost(USD per M in/out/cacheRead/cacheWrite), ThinkingMode, ThinkingLevelMap}, internal/config — version, paths, JSONC pre-pass, typed config + validation, env indirection, AppendString/SetObjectEntry/SetString, internal/llm — leaf types: Message, ContentBlock, ToolCall, ToolResult, Request, Usage, Event…, internal/provider — SSE reader, error taxonomy, catalog, 3 protocol codecs, replay helpers, retry wrapper, registry (incl. CheckCredential), subscription OAuth store + flow, provider.NewRegistry (catalog build + model validation), Built-in vendor catalog rows: anthropic claude-opus-5-5 / claude-sonnet-5-5 (adaptive thinking, 1M ctx) / claude-haiku-4-5 (budget, 200K); openai gpt-6-astra / gpt-6-luna (openai-responses, openaiReasoning), *config.EnvError (unset env: var at request build), mvdan.cc/sh/v3 — the only non-UI dependency; shell parsing for the shell analyser (+2 more)

### Community 104 - "Bet: a tight agent loop plus disciplined context handling beats a feature-rich agent on cost-per-task for everyday coding (read, edit, run tests, commit)"
Cohesion: 0.29
Nodes (3): Tagline: the coding agent you can read in an afternoon, moca: MO Coding Agent; binary name equals repo name equals module path github.com/adeotek/moca, moca — MO Coding Agent: a minimal token-efficient provider-agnostic coding agent, one Go binary, a Bubble Tea TUI, eight tools, no framework; inspired by Claude Code, OpenCode and Pi at deliberately about 10 percent of their surface area; competes on cost per task and read-the-whole-codebase-in-an-hour transparency rather than features

### Community 12 - "Phase-3 gate (2026-10-05): a mock-driven TUI session in a real tmux terminal against a scripted openai-completions server — status bar from the first frame, branch switch, streamed commits, items, slash commands (/help /cost /model /effort /hard /copy /show /compact), missing-key switch refused, newline hint, 120-line paste chip expand send, steering verified in the CAPTURED REQUEST PAYLOAD, approvals (a ran; A appended to shell.allow byte-preserving + .bak; rm prompted without [A]), /yolo on/off incl. a 60-column bar keeping YOLO, ! local, /undo, esc interrupting a tool, /clear; it found and fixed two real bugs (event-pipe deadlock on /yolo; styled prints mangled by Sanitize)"
Cohesion: 0.05
Nodes (5): internal/tui — Bubble Tea TUI: pure state (input/items/commands/status/format/sanitize/bridge/markdown) + glue (app/pager/trust), `moca` with no -p opens the interactive TUI in the current directory (§3.5), --effort <level> — off|minimal|low|medium|high|xhigh|max, validated and clamped to the model's supported levels, Phase-3 TUI reviews (2026-10-05) — four passes: pass 1 Approve with fixes 1M/3L; pass 2 1H/2M/6L (all fixed in 640be8e: startup panic from pre-program events, AppendString bounds, /clear ordering, chip-marker collisions, live flush, mid-run ! refusal, approval sanitizing, GitBranch timeouts); pass 3 0H/9M/7L fixed in efcdaf1; pass 4 (re-review) fixed in a2b091c; three items recorded as intentionally unchanged, Phase-7 OAuth/release reviews (2026-10-06): pass 1 0H/2M/4L, adversarial pass 2 0H/4M/6L, pass 3 0H/3M/6L — all confirmed findings fixed with regression tests; guard tests that cannot fail against the pre-fix tree were MUTATION-PROVEN load-bearing (randText error path, constant-randText, JWKS kid filter, exponent guard, exitFor sentinel, OAuth-list sync); contested items adjudicated with own probes (duplicate-keys fix is last-wins alignment, not refusal; the callback client-id contract kept; the two-channel paste race fixed structurally); pass-3: the checker runs the suite in the final repo, CheckCredential never refreshes, the shared stdin line reader, ctx-aware lock wait + bounded refresh, the callback page's 400

### Community 124 - "Commands: mise install provisions Go 1.27.1 pinned in go.mod with PATH export because go is not on the global PATH on every host; make build produces bin/moca version-stamped from git describe; make test runs go test with race and count; make vet runs go vet plus gofmt with both clean; make fmt; make release builds five cross-compiled binaries in dist; bin moca update with check compares with the latest GitHub release; mise run tasks mirror the make targets"
Cohesion: 0.67
Nodes (3): Commands: mise install provisions Go 1.27.1 pinned in go.mod with PATH export because go is not on the global PATH on every host; make build produces bin/moca version-stamped from git describe; make test runs go test with race and count; make vet runs go vet plus gofmt with both clean; make fmt; make release builds five cross-compiled binaries in dist; bin moca update with check compares with the latest GitHub release; mise run tasks mirror the make targets, make falls back to mise x go when go is missing from PATH; the make targets are mirrored as mise tasks invoked as mise run task; bare mise fmt is mise's own config formatter, Development setup: mise install provisions Go 1.27.1 pinned once in go.mod; make build produces bin/moca version-stamped from git describe; make test runs the full test suite with race detection; make release produces five cross-compiled binaries in dist; make falls back to launching go through mise when go is not on the PATH and the same targets run as mise run tasks

### Community 13 - "Execution: calls of one turn run SEQUENTIALLY in emitted order; a failed call does not cancel the rest; every call gets exactly one tool_result; unknown tool → error listing available tools; invalid JSON arguments → error telling the model to split the work; a panic inside a tool is recovered by Registry.Run into an error result ('internal error in <tool>: … — this is a moca bug, not your input; the call may have partly applied …') instead of crashing the session (rev 21 — eval feature#3 crashed on the edit-diff bug)"
Cohesion: 0.07
Nodes (4): Tool result availability boundaries (session/snapshot scope), --yolo/--no-yolo — override config yolo; yolo turns every permission check off, web.search config keys (provider tavily|exa, apiKey env reference), Independent review 2026-10-05 (docs/reviews/2026-10-05-phase-2-tools-agent-loop.md, verdict Block): all findings fixed with regression tests

### Community 17 - "Unit/integration: go test ./... -race — 685 top-level tests across 16 Go packages (count established via `go test ./... -count=1 -v | grep -c '^=== RUN'`); phases 1-5 keep passing; growth recorded: 585 before the web-tool/plan-mode suites → 613 rev-21 harness pass → 640 system-prompt template extraction → 641 overflow-per-session → 643 branch-review pins → 646 diagnostic-log suites → 679 web private-address + branch-review pins → 685"
Cohesion: 0.09
Nodes (3): MCP server config keys (command|url, args, env, headers, description, approve), MCP frozen stub tool (registered in place of the proxy when mcp.servers is empty), Phase-5 MCP reviews (2026-10-06): passes 1-4 — all findings fixed with regression tests (sentinel-based retry instead of text matching; write-failure preferring the recorded exit error; bounded default HTTP client; import covering literal secrets outside env/headers; export-name collision skip; corrupt-index reset; rune-safe description cut; bounded Close; notification status check; no respawn after Close; quoted-id echo; import summary counting) — every behavior-pinning test verified to fail against pre-fix sources

### Community 20 - "Append-only JSONL transcript at ~/.local/share/moca/sessions/<date>-<slug>-<id8>.jsonl (file 0600, dir 0700); every entry has id, parentId (chain), type, ts; types: session (header: workdir, provider, model, EFFECTIVE effort, startedAt, mocaVersion, the FULL system prompt, yolo), message, tool_use, tool_result, compaction (summary + firstKeptEntryId + tokensBefore + usage + cumulative readFiles/modifiedFiles — its usage/cost count toward session totals and are restored on resume), snapshot, error, permission_mode, model_change (written by /model, /effort, /hard), elision (rev 21: {ids, tokens} — tool_result entry ids superseded later)"
Cohesion: 0.10
Nodes (4): internal/compact — token estimate helpers + compaction: budgets, cut points, lossy serialization, structured summaries (no provider import), --continue — resume the newest session started in this workdir (canonical-path match), --resume <id8|last> — resume a stored session (8-hex suffix; `last` = newest by mtime), Phase-4 compaction/resume reviews (2026-10-05) — passes 1 and 2, all confirmed findings fixed with regression tests (skip-and-warn summarizer, ! busy guard, resume workdir validation, rune-safe truncation, tracked-path normalization, kept-id fallback in liveEntries, fixed-overhead guard clause, cancellable /compact, llm.TextOf dedupe, --model-on-resume test, stable Find order, direct exactly-once request count); re-verified pass 1, retracted one item, added the !-during-compact race and the missing-workdir resume diagnostic

### Community 23 - "Skills: the Agent Skills standard, same shape as Pi Claude Code and OpenCode, three sources in precedence order where the earlier wins on name collision"
Cohesion: 0.07
Nodes (31): Extension model: skills plus slash commands plus shell allowlist plus the lazy MCP proxy, Status header: REV 11 — plan review fixes, awaiting final lock; new revisions required for changes once locked, graphify workflow in this repo: a knowledge graph at graphify-out with god nodes and community structure and cross-file relationships; for codebase questions first run graphify query when the graph json exists, graphify path for relationships, graphify explain for focused concepts; dirty graphify-out files are expected after hooks or incremental updates and are not a reason to skip; use the wiki index for broad navigation when it exists; read the graph report only for broad architecture review; run graphify update after modifying code as an AST-only pass with no API cost; the installed graphify skill or instructions come first whenever the user types the graphify slash command, Ground truth rules: docs/specs/SPECS.md is the living normative description of what is implemented today covering the CLI contract, config keys, wire behavior, tools and agent loop; update it in the same PR as any behavior change per its section 16; when it and DESIGN.md disagree SPECS wins for the current code; DESIGN.md is the v1 contract and README.md is user-facing, The system prompt is built once per run and stored in the session, never rebuilt; its fixed text is the embedded internal agent system-prompt.md file with token slots rendered at start, slash create-command — a starter command template written on the first TUI run into the global prompts dir (an existing file is never overwritten); running it has the agent create or update a saved slash command, personal ones in that directory asking to approve the write since it is outside the workdir and once per write never persisted, or project ones in the workdir moca prompts; new commands are usable as soon as the run finishes with no restart, External tools and the skills ecosystem landed in v0.1.0-alpha: rtk and graphify in the default allowlist with built-in guidance skills and the shell analyser unwrapping rtk so it cannot launder a non-allowlisted command; Agent-Skills compatibility with SKILL.md files written for pi, Claude Code and OpenCode including graphify's proven by a vendored corpus with provenance and SHA-256 enforcement; prompt templates from Claude Code command files and pi templates drop in; graphify install platform moca contributed upstream in Graphify Labs graphify pull request 4174, Harness: the system prompt names the project's checks (detected from the Makefile, package.json or the ecosystem) and its working style was rewritten: state the assumption on a vague request, think through edge cases, fix root causes, never weaken a test, review the diff, never edit files through the shell (+23 more)

### Community 26 - "Plan mode (rev 20) in the tools: with Env.Plan set, write/edit reach only <workdir>/docs/plans/*.md (everything else → `refused: plan mode only writes docs/plans/*.md …`; the ask-write roots stay closed — nothing is asked) and a successful plan-file write/edit sets Env.PlanWrote; read/search/ls/web and shell/MCP stay available under the normal ladder (2026-10-09 follow-up: planning needs git log, builds, docs lookups); the failing-test banner/refusal is suppressed in plan mode (plan runs inspect, they don't fix); the confinement is NOT a permission — --yolo does not lift it; edit applies the plan-zone check BEFORE the ask-write approval (plan mode never asks)"
Cohesion: 0.08
Nodes (9): Agent run plan, agent.StartError — a failed run setup (data dir, session files, skills) exits 1, `moca -p "<prompt>"` — one-shot agent run; `-p -` reads the prompt from stdin (trailing newlines stripped), Credential env error, Env.Plan flag (set by --plan or the TUI /plan toggle), --do <plan.md> — execute an implementation plan (rev 21): implies a one-shot run; -p adds instructions; exclusive with --plan, --plan — plan mode (rev 20): analyze the request and write an implementation plan to docs/plans/, change nothing else, Plan mode (rev 20 read-only runs with a docs/plans deliverable) (+1 more)

### Community 38 - "Permissions threat model stated plainly: guard against model mistakes and casual prompt injection; NOT a sandbox because an allowlisted interpreter can do anything the user can; OS-level sandboxing is a v2 discussion"
Cohesion: 0.09
Nodes (26): Yolo mode: an explicit opt-in turning every permission check off, for sandboxed or throwaway environments and users who accept the model can do anything they can, Logging: packages call standard log/slog directly; internal applog with the file handler, redaction and caps is imported only by cmd/moca; log metadata only, never prompts, file contents, command text or secrets per SPECS section 13.5, Approvals show the whole command: up to 6 lines under the question with overflow counted and ctrl-o paging all of it; the prompt used to show only the first line so approving python3 hid the heredoc it would run, Diagnostic log levels: MOCA_LOG=debug adds per-request and per-tool-call detail for one run; log.level info debug off sets the default; files capped at 10 MB and pruned with the snapshot retention, MCP landed in v0.1.0-alpha: the lazy MCP proxy with stdio and streamable HTTP transports, a persisted discovery index, search describe and call in one roughly 200-token tool with server tool lists never entering the prompt, servers starting on first call and stopping after idle; moca mcp import imports Claude Code OpenCode and Pi server configs with secrets rewritten to env references, Permissions landed in v0.1.0-alpha: path jail anchored at the session workdir with symlink resolution and read-only skill roots; shell command analysis via mvdan sh with hard-deny and ask ladders, compound-command checking and rtk unwrapping; project trust with approve and no-approve and a trust json store; yolo mode via flag and slash yolo turning every check off between runs, Unreleased: a diagnostic log safe to attach to bug reports — every TUI and one-shot run writes a small logfmt file to state moca logs with retries, MCP lifecycle, run endings and tool panics with stacks, never prompts, code, command text or keys; a redaction net blanks secret-named fields and URL credentials, Default allowlist includes plain commands plus the external tools rtk and graphify (+18 more)

### Community 39 - "Phase 7 — subscription OAuth behind the DESIGN §3 policy gate: anthropic ships api_key only, openai ships SIWC token sharing on the openai-responses route with dynamic client registration, PKCE S256, nonce/RS256/JWKS validation, a 0600 token store with cross-process locked rotating refresh, moca login|logout; plus the automated ship gate and version stamping"
Cohesion: 0.10
Nodes (8): anthropic-messages adapter — POST {base}/v1/messages; headers anthropic-version: 2023-06-01 and x-api-key (Authorization: Bearer for OAuth tokens), openai-completions adapter — POST {base}/chat/completions; Authorization Bearer; system first, per message text joined, tool calls with arguments as a JSON STRING, tool results as role:tool; stream:true with stream_options.include_usage; reasoning_effort per §5, ErrContextOverflow (wrapped by adapters; drives overflow recovery), --model <provider/model> — override the config model for this run (recorded as model_change when replacing a stored pair), `moca login <provider>` (--api-key prompt instead of OAuth; --no-browser headless OAuth) stores a credential in auth.json; openai runs the SIWC flow; every other provider prompts for an API key (anthropic prints the recorded policy reason); unknown provider exits 2, `moca logout <provider>` — best-effort revoke of a subscription session (bounded 15 s), then clears the stored token AND API key, openai-responses adapter — POST {base}/responses; Authorization Bearer; input items (message text in/out, reasoning items with encrypted_content, function_call/function_call_output), store:false, include reasoning.encrypted_content (stateless reasoning replay), instructions = system, tools as {type:function,…}, reasoning per §5, docs/specs/oauth-verification.md — the recorded, live-fetched policy verification (anthropic api_key-only; openai permitted via SIWC token sharing)

### Community 41 - "TUI built on Bubble Tea plus lipgloss, compact and dense per house style: tight padding, no banner boxes, a session opens with two compact welcome lines at the top of the scrollback (rev 13); without a configured model the welcome carries the red setup notice (rev 18)"
Cohesion: 0.07
Nodes (23): Effort is a first-class request parameter (off, minimal, low, medium, high, xhigh, max), mapped per model via thinkingMode and thinkingLevelMap; default medium; set via /effort, shown in the status bar, config.Standardize for JSONC preserves byte offsets so decode errors keep the original line col; the internal config testdata example jsonc file is the byte-exact DESIGN section 12 fixture, Compaction entry holds summary, firstKeptEntryId, tokensBefore and usage; the summary's own cost counts toward session totals; repeated compactions summarize from the previous compaction's kept boundary so kept messages are never silently dropped from later summaries, Config example surface: model and modelHard keys, providers block with per-provider auth and apiKey env references, shell allow list, mcp block with idleTimeout and servers, web search provider and apiKey, snapshot retentionDays, log level, yolo flag, context budgets with reserveTokens keepRecentTokens maxSteps, Single config file at config moca config.jsonc, a typed struct with fail-fast validation; JSONC over TOML (rev 6) because it is stdlib-parseable with a comment-strip pre-parse and matches the house dotfiles sync machinery and the OpenCode ecosystem configs mcp import reads; one parser everywhere, Model routing: slash effort sets the effort level validated against the catalog and clamped; slash hard toggles to modelHard plus effort high and back; compaction summaries always run on the cheap model at minimal effort; no auto-escalation heuristics in v1 because explicit is cheaper to debug than clever, Slash hard is a toggle: first use saves the current model and effort pair and switches to modelHard plus effort high clamped; second use restores the saved pair; both directions write a model_change entry, The JSONC pre-pass is string-literal-aware and handles comments and trailing commas: slash slash comments only outside quoted strings with escape sequences honoured, trailing commas removed, comments replaced with spaces or newlines so encoding json error offsets map to original line col; the Phase-1 parser gate requires the §12 example to decode intact and the pre-pass to be a no-op on plain JSON (+15 more)

### Community 42 - "Eight tools frozen for v1: a new tool or parameter is a v2 discussion because each session re-reads schemas and tool-list churn kills prompt-cache reuse"
Cohesion: 0.08
Nodes (24): Fixed: an edit could crash moca — an old_string ending in a blank line the file does not have matched, was written, then crashed the diff rendering taking the session down; the diff is fixed and any panic inside a tool now becomes an error result for that call instead of a crash, Seven frozen tools in v0.1.0-alpha: read, write, edit with exact-match-first and one whitespace-tolerant fallback with re-indentation and CRLF/BOM preservation, shell stateless with bash c and pwsh NoProfile and process-group kill, search pure Go ripgrep semantics gitignore-aware, ls, and the lazy mcp proxy, Edit fallback: a whitespace tolerant match occurs when the exact match fails, re-indentation applies the indentation delta to every new_string line, and at most one hit is accepted; replace_all never uses the fallback, Edit line endings and encoding: matching on an LF-normalized view; the write restores the dominant line ending so CRLF stays CRLF; a UTF-8 BOM is preserved and never matchable; non-UTF-8 files refused, Edit returns no-op rejection when new_string equals old_string, Edit replace_all replaces every exact match and reports the count; ambiguous without replace_all is an error listing hit line numbers with no write, Everything else stays available in plan mode: read search ls web as always and shell plus mcp calls run under their normal permission rules; planning needs git log, builds and docs lookups; the failing-test investigation banner is suppressed while planning because plan runs inspect, they do not fix, Plan enforcement: with Env.Plan set, write and edit reach only workdir docs plans markdown files; the prompts ask-write root stays closed; anything else is a model-facing refusal naming the plan file; the one hard rule (+16 more)

### Community 44 - "MCP support ships in v1 lazy by design (rev 3) because Ben uses MCP servers daily; learned from pi-mcp-adapter's measured results: 100 installed servers but 0 running at session start and about 200 tokens of prompt cost versus eager approaches costing 7 GB RAM and over 10k tokens"
Cohesion: 0.08
Nodes (22): Auth mode api_key: key stored with /login (rev 14) or the apiKey env:VAR indirection; stored key wins, env value is the fallback, The env:VAR indirection is the single syntax used everywhere in config for secrets (apiKey, headers, env), apiKey is optional in the config: a key stored with slash login or the CLI wins over the env VAR reference which stays the fallback so an env var set by an external process still works and slash logout restores it; the stored key applies from the running session's next request, Edit-ladder tests (phase 2, non-negotiable): exact unique, exact ambiguous, replace_all with hits, whitespace fallback hits with re-indentation deeper and shallower, fallback ambiguous, no match, no-op new equals old, empty old_string, CRLF file stays CRLF, BOM file, unicode file, 50-line span, trailing-newline-missing file, write to new file without read, and write or edit after an on-disk change refused, MCP config in the same file with one block per server under mcp servers and scalars like idleTimeout sitting beside servers not inside it; pi-mcp-adapter and Claude-Code-compatible fields in JSONC shape, Discovery index persisted at a state mcp-index json path holding server, configHash and tools with name description annotations; refreshed every time a server starts and invalidated when that server's config hash changes; a search over servers with no valid index entry starts only those servers to index them; moca mcp index prebuilds the whole index from the CLI, HTTP server auth via optional per-server headers whose values are literal or env-indirection strings; MCP OAuth for remote servers is a v2 discussion, moca mcp import reads existing Claude Code OpenCode and Pi MCP configs and writes them into moca's config as a mechanical translation previewed before writing; literal secrets are never copied: matching keys are rewritten to env references and the preview lists the variables to export (+14 more)

### Community 47 - "Revision log: numbered revisions each a dated contract change; changes require a new revision, not silent drift; rev 1 through rev 23 recorded"
Cohesion: 0.09
Nodes (22): Exactly eight frozen tools with the web tool joining in DESIGN rev 19; schemas are golden at internal/tools/testdata/schemas.golden.json; changing a schema is a v2 discussion and description-only edits from rev 21 are batched into one golden update because each costs every user a prompt-cache reset, slash model picker marking providers without a key plus argument completion in the slash dropdown for model effort login logout show and resume, TUI: a fresh install opens the TUI instead of exiting — with no model configured moca opens in setup mode with a red notice riding the welcome print, the status bar carrying no provider configured slash login, and sending a message repeating the notice; slash login stores a credential then slash model opens the first-run picker checking the credential, starting the session immediately and saving the choice as config model; slash help exit resume sessions and bang prefixes keep working; one-shot p without a model keeps the old error and resume or continue need no configured model, Web: a web tool with fetch and search needing no API key — fetch GETs a URL returning readable markdown with the page title as headline or plain text or raw html; search returns ranked results with snippets; search works out of the box via Tavily's keyless mode with web.search.apiKey as an env reference to lift the rate limit or web.search.provider exa with a key; fetching is bounded like every other tool (5 redirects, 2 MiB, 20K chars, 30s default); like curl it needs no approval for public sites and fetching a private or local address asks every time per host so a redirect to a different private address asks again; fetched pages are untrusted data, never instructions, DESIGN rev 1 (2026-10-03): initial draft, DESIGN rev 11 (2026-10-04): Ben approved full plan review with fixes applied across the seven phase plans line by line with every machine-checkable claim verified programmatically: allowlist test count, dead-code trailing comma branch, missing snapshotDir parameter, an mcp idle-timer race fixed into one critical section with a race regression test, statelessness test using a physical path, case-insensitive assertion, skill-list line shape pinned, slash yolo between-runs clarification, plan headers recited, de-prefixed sub-skill references, and one reported blocker retracted after verification and recorded so plan trustworthiness stays auditable, DESIGN rev 15 (2026-10-09): slash-command management: the global prompt templates dir becomes a first-class agent-aware surface; a starter create-command template is seeded on the first TUI run embedded and never overwriting an existing file; the jail gains an ask-write root for the config prompts dir with a plain once or deny approval never persisted and no asker meaning denial and yolo never refusing; the config dir stays a user-owned zone with the user as the gate against prompt-injected installs; the TUI re-reads prompt dirs when a run or bang command finishes so a mid-run command is usable without restart, DESIGN rev 18 (2026-10-09): first-run setup mode: moca with no model configured opens the TUI instead of exiting; the one-shot p path keeps the error since it has no scrollback to explain itself in; the welcome carries a red no-provider notice with the status line and message answers matching; slash model becomes the first-run picker marking providers without a credential, cursoring the first credentialed model, discovering a configured ollama server first, checking the credential and starting the session in place saving the choice as config model so the next launch starts straight in; resume and continue need no configured model (+14 more)

### Community 5 - "SPECS.md — moca's living normative spec (16 numbered sections, updated with every behavioral PR)"
Cohesion: 0.06
Nodes (21): cmd/moca — main.go (entry, signals), cli.go (flags → Options), oneshot.go (-p agent path), tui.go (TUI path + trust), update.go, login, mcp subcommands, internal/agent — system prompt, events, the loop, compaction + resume, cross-provider transform + control surface, Start wiring, internal/mcp — lazy MCP proxy: JSON-RPC core, stdio + streamable HTTP transports, SSE reader, persisted discovery index, lifecycle manager, the mcp proxy tool, the importer, internal/permissions — path jail, parsed shell analysis (+Windows best-effort), trust store, yolo checkers, internal/session — JSONL transcript (entry/writer/rebuild/repair/find), snapshot store + undo, internal/skills — SKILL.md frontmatter + discovery, embedded built-ins, AGENTS.md/CLAUDE.md loader, prompt templates, internal/tools — Tool interface + registry, the 8 frozen tools, ReadTracker, shell runner, ignore matcher, spill files, run-state env, internal/update — moca update: GitHub release fetch, semver compare over the describe stamp, checksum verify, atomic binary replace (leaf — no internal imports) (+13 more)

### Community 59 - "Ship gate for the v0.1 demo, one session on the opencode-go provider proving multi-provider from day 1: read a failing test, locate the bug with search, edit the fix, run go test via shell and see green, commit on a feature branch, and the built-in rtk skill was discoverable and followed"
Cohesion: 0.10
Nodes (20): Ship gate not in CI needing the repo env file with the OpenCode Go key and a build first: make build then bash test/shipgate/run.sh; the checker carries a build tag so plain go test skips it; several internal tools tests carry a not-windows build tag, Failing-test investigation protocol: while a failing test run is unresolved, every tool result states the next step — read the failing test with the read tool and locate the cause with search — and edit refuses until both happened; the ship-gate loop's investigation steps enforced rather than advisory, The status bar shows the branch of a fresh git init with no commit yet instead of a dash, slash login in the TUI: pick a provider then paste an API key masked never echoed never written to the transcript or sign in to a subscription in the browser; while a sign-in is in flight the pasted redirect URL or code from the input box feeds the flow and esc cancels; after a successful ChatGPT sign-in the wizard offers to flip providers openai auth to oauth; slash logout revokes a subscription session best-effort and clears the stored token and key, TUI landed in v0.1.0-alpha: Bubble Tea with a bottom status bar from the first frame showing cwd, git branch, provider model, effort, context size and usage percent, session tokens, cost with the subscription marker, transient retry states and the YOLO state; multi-line input with bracketed paste and collapsed chips; shift enter newline via enhanced keyboard reporting with fallbacks; steering during runs; bang prefixes; approvals allow once, allow always, deny; pager; slash copy via OSC 52; slash commands for model, effort, hard, yolo, clear, compact, cost, undo, copy, show, help plus prompt templates, v0.1.0-alpha (2026-10-07): first release — a minimal token-efficient provider-agnostic coding agent: one Go binary, a Bubble Tea TUI, seven tools, no framework, v0.1.1 (2026-10-08): polish, a credential store in the config dir, slash autocomplete, and a bigger catalog — still one binary, still no framework, TUI polish in v0.1.1: slash autocomplete dropdown listing built-in commands and loaded templates with one-line hints filtered as you type with up down pick tab complete enter run esc dismiss and built-ins winning collision; commands echo as messages in the same band as user messages before their output including refusals; welcome line product name renders orange; thinking blocks print their item line when the block ends with the streamed duration; two-line status bar from the first frame; full-width rules around the input and compact welcome lines; full-row backgrounds for submitted messages and assistant responses; muted tool and thinking lines; exit aliases; the inline-renderer shrink guard became burst-aware so shrinking frames cannot strand stale rows when keystrokes coalesce (+12 more)

### Community 64 - "Repository layout: cmd/moca plus twelve internal packages"
Cohesion: 0.15
Nodes (11): Repository layout: cmd/moca plus twelve internal packages, Layout: entrypoint cmd/moca/main.go with run as the testable core; sibling files handle the p flag, the TUI, login, mcp and update subcommands, cmd/moca: CLI surface, mode select (TUI or one-shot -p run), internal/agent: the loop user message, model, tool calls, results, repeat, internal/compact: token accounting, truncation, summarization, cache markers, internal/config: single JSONC config, typed and validated, internal/llm leaf package: Message, ContentBlock, ToolCall, Usage, Request types; imports nothing internal, internal/mcp: lazy MCP proxy, stdio and streamable HTTP transports, discovery index, server lifecycle; registers the mcp tool (+3 more)

### Community 65 - "Session entry types: session first entry, message, tool_use and tool_result, compaction, model_change, permission_mode, snapshot, error, elision (rev 21)"
Cohesion: 0.12
Nodes (19): Context and sessions landed in v0.1.0-alpha: a token-denominated context manager with reserve and keepRecent budgets, usage-anchored estimates, cut-point rules that never orphan a tool call, structured summaries with cumulative file tracking, overflow compact-and-retry, no-cache-write summary requests; JSONL session transcripts with a single-writer lock and 0600 mode carrying entry ids, thinking blocks, model changes and compaction entries; resume with id8 or last and the continue flag, slash cost uses the bar's compact token format and breaks the input down into fresh, cache read and cache write with subscription for OAuth, esc kills a running bang command which now gets the activity row like runs and slash compact, Resume picks up where you left off: resume and continue flags replay the last 3 turns into the scrollback; a new slash resume picker lists this directory's other sessions and an id from another directory is refused, slash sessions — manage this directory's stored sessions in one place: every session listed with the open one marked current, enter switches to it, ctrl-d deletes it after a y confirm; the open session and sessions held open by another moca process refuse and a file under a live writer is never unlinked; the list stays open and the newest 100 are listed with the title giving the real total, internal/session: JSONL append-only transcripts, resume, snapshots and undo, DESIGN rev 17 (2026-10-09): session manager: the TUI gains slash sessions, a picker over every stored session of the current directory newest first marking the open one current and message-less ones; enter switches with exact resume semantics refusing another directory's session and refusing while a run or compaction is in flight; ctrl d arms a delete confirm and on y removes the file through a new single-writer-locked session Delete refusing a file held open by another process so a file under a live writer can never be unlinked; the panel stays open one row shorter and the current session refuses deletion, Every session entry has an id plus parentId forming a linear chain in v1; ids make a session tree a v2 extension, not a schema change (Pi's proven shape) (+11 more)

### Community 67 - "skills.LoadPrompts reads <dir>/<name>.md (optional frontmatter description/argument-hint; first dir wins a collision; sorted); ExpandPrompt substitutes $ARGUMENTS/$@ (all args), $1..$9 (whitespace-split) and $$ (literal $); Claude Code command files (allowed-tools/model keys ignored) and pi templates load unchanged (testdata/prompts/, TestEcosystemPromptsLoad); skills.SeedUserPrompts(config.PromptsDir()) materializes the embedded starter template on the first TUI run (best-effort, stderr warning on failure) — never overwriting an existing file (O_EXCL); the global prompts dir is a jail read root and ask-write root"
Cohesion: 0.16
Nodes (5): Shrink-reflow limitation (tmux/kitty/iTerm2 rewrap leaves one stale row), TUI slash commands (/model /resume /sessions /effort /hard /yolo /plan /do /clear /compact /cost /undo /copy /show /login /logout /help /exit), Vendored ecosystem skill corpus (pi/Claude Code/OpenCode/graphify fixtures), Starter command template seeded on the first TUI run, Starter create-command prompt template (create/update saved / commands)

### Community 70 - "Cross-provider history transform applied when building every request so /model and /hard can switch mid-session: thinking blocks replay unchanged only to the exact model that produced them; thinking from any other model becomes a plain text block labelled prior reasoning; the transcript keeps the originals"
Cohesion: 0.12
Nodes (13): Cache breakpoints: explicit positions in Request: anthropic cache_control at the end of the tools plus system prefix and on the last message, rolled forward each turn with a maximum of 4 breakpoints; openai protocols use prefix-stability discipline, Model ids are provider-qualified (opencode-go/glm-5.3-flash); hence no defaultProvider key: the model id names the provider, SSE stall timeout: no bytes for 90s aborts that attempt and retries, thinkingMode values none, budget, adaptive, openai plus thinkingLevelMap: anthropic adaptive models take thinking adaptive plus output_config effort; budget models take budget_tokens; openai protocols take reasoning_effort, Thinking blocks and signatures are model-bound; cross-provider transforms happen in the agent before each request per SPECS section 14, Catalog and tooling in v0.1.1: ten more opencode-go models all live-verified including deepseek and kimi-k2.7-code and mimo and qwen variants; the Go version pinned once in go.mod; make targets mirrored as mise run tasks; make vet now includes the gofmt check; documentation trimmed to the two living specs, Retry/backoff with jitter honoring retry-after, SSE stall timeout, mid-stream failure rule; effort as a first-class request parameter with per-model clamping; cross-provider history transform with thinking replay bound to the producing model and tool-id normalization; upstream failures the gateway labels server_error are retried even when framed as HTTP 400 like the opencode-go thinking-config flake; the default HTTP client bounds the response-header wait at 120s so a silent provider cannot hang an unattended run, Cross-provider history transform applied when building every request so /model and /hard can switch mid-session: thinking blocks replay unchanged only to the exact model that produced them; thinking from any other model becomes a plain text block labelled prior reasoning; the transcript keeps the originals (+5 more)

### Community 84 - "`moca update [--check]` — replaces the running binary with the newest GitHub release package via internal/update"
Cohesion: 0.21
Nodes (3): Built-in provider env keys (ANTHROPIC_API_KEY / OPENCODE_API_KEY / OPENAI_API_KEY), Env overrides: MOCA_API_BASE (install API base), MOCA_UPDATE_API (update API base), MOCA_LOG (one-run log level) — for tests and mirrors, `moca update [--check]` — replaces the running binary with the newest GitHub release package via internal/update

### Community 98 - "test/evals — the rev 21 eval corpus: seven scenarios, each a fixture repo (committed base), prompt.txt, check.sh judged against the base commit (SESSION names the transcript) and optional flags; not in CI, same .env key; harness changes show their delta against BASELINE.md"
Cohesion: 0.22
Nodes (4): Eval corpus scenario definitions, Eval scenarios (bugfix/feature/refactor/vague/largefile/plan/web), test/evals/BASELINE.md — records the measured eval runs; the comparison point for harness or prompt changes, test/evals/config.jsonc (eval harness config)

## Knowledge Gaps
- **220 isolated node(s):** `example.com/codes`, `example.com/fetcher`, `example.com/inventory`, `example.com/shop`, `example.com/slug` (+215 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 581 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **86 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `External tools - rtk, graphify, skills ecosystem` connect `External tools - rtk, graphify, skills ecosystem` to `permissions/shell.go`, `README.md - moca`?**
  _High betweenness centrality (0.162) - this node is a cross-community bridge._
- **What connects `example.com/codes`, `example.com/fetcher`, `example.com/inventory` to the rest of the system?**
  _220 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `v0.1.0-alpha (2026-10-07)` be split into smaller, more focused modules?**
  _Cohesion score 0.0766488413547237 - nodes in this community are weakly interconnected._
- **Why does `README.md - moca` connect `README.md - moca` to `External tools - rtk, graphify, skills ecosystem`, `BASELINE.md`, `Release - build packages, publish GitHub Release`, `system-prompt.md`?**
  _High betweenness centrality (0.150) - this node is a cross-community bridge._
- **Should `go_pkg_strings` be split into smaller, more focused modules?**
  _Cohesion score 0.06228760878574389 - nodes in this community are weakly interconnected._
- **Why does `Revision log: numbered revisions each a dated contract change; changes require a new revision, not silent drift; rev 1 through rev 23 recorded` connect `Revision log: numbered revisions each a dated contract change; changes require a new revision, not silent drift; rev 1 through rev 23 recorded` to `Session entry types: session first entry, message, tool_use and tool_result, compaction, model_change, permission_mode, snapshot, error, elision (rev 21)`, `Permissions threat model stated plainly: guard against model mistakes and casual prompt injection; NOT a sandbox because an allowlisted interpreter can do anything the user can; OS-level sandboxing is a v2 discussion`, `TUI built on Bubble Tea plus lipgloss, compact and dense per house style: tight padding, no banner boxes, a session opens with two compact welcome lines at the top of the scrollback (rev 13); without a configured model the welcome carries the red setup notice (rev 18)`, `Release - build packages, publish GitHub Release`, `MCP support ships in v1 lazy by design (rev 3) because Ben uses MCP servers daily; learned from pi-mcp-adapter's measured results: 100 installed servers but 0 running at session start and about 200 tokens of prompt cost versus eager approaches costing 7 GB RAM and over 10k tokens`, `Context budgets are token-denominated because a percentage conflates a 200K and a 1M window; the catalog already knows the window; budgets config-overridable`, `v0.1.0-alpha (2026-10-07)`, `Ship gate for the v0.1 demo, one session on the opencode-go provider proving multi-provider from day 1: read a failing test, locate the bug with search, edit the fix, run go test via shell and see green, commit on a feature branch, and the built-in rtk skill was discoverable and followed`, `Plan mode (rev 20): a run analyzes the request and the code then writes an implementation plan to docs/plans and stops; a scope, not a permission`?**
  _High betweenness centrality (0.138) - this node is a cross-community bridge._
- **Should `testing.T` be split into smaller, more focused modules?**
  _Cohesion score 0.032944406314344546 - nodes in this community are weakly interconnected._