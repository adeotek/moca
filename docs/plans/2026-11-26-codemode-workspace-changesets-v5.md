# Codemode redesign: workspace changesets — implementation plan (revision 5)

**Status: amends revision 4, which amends revision 3; proposal, not approval.** Implementation baseline
`cd3302055aec27b9fe79350a62052dc5c33f3d96`. Revisions 1–4 remain the record. Except where a passage is
explicitly replaced below, revision 4's amendments and the inherited revision 3 text stand. No
implementation, release, default change or campaign spend is authorized here.

**Why a delta.** This is a delta of the immediately previous revision: eight targeted corrections do not
justify retyping the combined plan. Every replaced passage appears below in operative form, and §10 lists
what stands unchanged. The raw `v2pilot-*` rows, the production spec golden and the live test harnesses
were rechecked for this revision.

## 1. Revision 4 review: dispositions

| ID | Finding | Disposition |
|---|---|---|
| R5-1 · blocking | V4 selects the existing `-p --mode yolo` campaign but still adopts rev 3 §6's “real `moca acp` … in `ask` mode” question and its consent-dialogue arm description. | §2 makes `-p --mode yolo` the only campaign protocol and explicitly replaces the stale rev 3 passages. ACP tests consent separately. |
| R5-2 · high | The four sign-test events share each triple's C run, so their probabilities do not “multiply.” A 0.75 pairwise win rate is also not a 25% request-ratio improvement. | §3 removes the independence claim, states a dependence-safe bound, separates the directional bar from reported two-sided p-values, and scopes the power numbers to the sign component only. |
| R5-3 · high | The production read-only golden is 542 B including its newline against a 576 B cap; v4's proposed clause alone is 161 B. It cannot fit “within the existing description budget.” Putting it in every read-only spec would also advertise a capability those sessions do not have. | §4 creates an edits-only spec and golden, gives the exact description, and pins 712 B including the newline against a new 768 B cap for that variant only. |
| R5-4 · high | `test/codemodegate` currently drives only `moca -p`; its harness cannot observe an interactive approval. V4 names an interactive test without naming the process or driver. | §5 keeps one new top-level test but splits its subtests between a real ACP stdio client and the existing headless harness. The count remains 8 → 9. |
| R5-5 · medium | “Aligned with `edit`/`write`” is false for interactive ask/auto: an ordinary in-workspace write does not ask merely because it is a write, and auto's classifier judges shell commands, not changesets. | §6 retains the stricter consent matrix as an explicit design choice and makes it deterministic; no classifier request is added. |
| R5-6 · medium | V4 correctly gives transport carry as 32,863, but `test/evals/BASELINE.md` still says 32,679. The first correction pass fixed only two of three stale sums. | §9 records all four raw-row sums; the baseline source row and footnote are corrected with this revision. The pilot verdict is unchanged. |
| R5-7 · medium | Allowing `.env.example` only in the credential predicate cannot work while rev 3 independently rejects every dot-prefixed component. “Either accept it or allow it” also leaves the implementer a fork. | §8 ships the suffix allow-list through one helper called by both predicates, with exact positive and negative cases. |
| R5-8 · low | `not_granted` is a new refusal class with no existing taxonomy or consumer, introduced for a P0 fix whose goal is to delete file grants. | §7 withdraws it. Skipped file grants and ungranted siblings use the existing `outside` refusal; the release note carries the behavior change. |

## 2. R5-1 — One campaign protocol, everywhere

**Replaces rev 3 §6's “Question” paragraph and “Arms” bullet, and the `-p incl. yolo` expectation in rev
3 §5 phase 5. V4 §3's consent table and its replacement of the bespoke campaign driver remain.**

**Question (§5 phase 7):** is opt-in changeset application worth shipping for these workloads, compared
with (A) moca as it ships and (B) the new reads without edits, measured through the existing two-prompt,
two-process `moca -p` trial path under `--mode yolo`? Approval is the campaign's separate decision, before
any billed trial.

- **Arms:** A = codemode off; B = `workspace`, reads only; C = B + `workspaceEdits`. Every arm runs the
  same front end, mode, trust policy, direct tools and shell. C applies a valid changeset without a human
  question because that is v4's explicit `-p --mode yolo` rule; path, stamp, journal, investigation and
  every other non-consent guard still apply. No arm has a changeset consent dialogue in this campaign.
- **Driver:** `test/evals/_lib/trial.sh:106–124` already supplies `--mode "${MODE:-ask}"` to both the
  initial process and its `--continue` process. The campaign sets `MODE=yolo` and reuses the existing tree
  setup, two prompts, transcript checks, telemetry and one `-cap-scope` ledger. No ACP campaign driver and
  no second request ledger.
- **What the campaign does not measure:** interactive consent. Its `approvals` field must remain an
  observed value, never inferred from mode; the changeset path itself asks zero questions in this campaign.
  Interactive ask/auto/yolo/plan behavior is proved by §5's hermetic ACP and unit tests, not by billed
  trials.
- **Feasibility and phase 5:** rev 3 §5 phase 6's “same permission policy” now means this same `-p --mode
  yolo` policy. Phase 5 tests headless ask/auto/plan refusal and headless yolo success; it no longer expects
  headless yolo refusal. ACP approval/denial remains a real gate leg.
- **Scope of the result:** a win supports unattended yolo changesets on the measured fixtures. It does not
  establish that interactive ask/auto consent is usable, desirable or worth promoting; that remains a
  separate product judgment backed by the non-billed client tests.

Rev 3 §6's assignment, endpoints except as amended by v4 and §3 below, preconditions, vetoes and verdict
remain. If the owner instead chooses ACP `ask` sessions for the campaign, v4 A2 and this section are
withdrawn together: the bespoke ACP driver, interactive request measurement and consent-dialogue arms must
return as one protocol change. Do not mix the two protocols in one campaign.

## 3. R5-2 — Statistics: dependence, ties and power

**Replaces v4 §4 items 1–2. Item 3 (carry is diagnostic, never a bound) and the instrumentation paragraph
remain.**

1. **The AND is conservative, but the events are not independent.** Under the zero-tie sign null, one
   directional ≥9/10 event has marginal tail `11/1024 ≈ 0.010742`. C vs A and C vs B reuse the same C run
   in a triple, so no product of four marginal tails is valid without a dependence-aware model. The joint
   event is nevertheless a subset of any one marginal event: under a global null its probability is at
   most `11/1024`, before the additional ratio and precondition filters. State that bound; do not publish an
   invented exact joint probability or an independence assumption.
2. **The directional score and the reported p-value have distinct jobs.** For each of the ten assigned
   pairs, a win means strictly fewer valid logical work requests for C; a numeric tie is not a win, and
   missing/invalid data follows the campaign invalidity rules. Report exact two-sided binomial sign-test
   p-values conditional on the untied valid pairs, together with the number of ties. The ship bar still
   requires ≥9 wins out of all ten assigned pairs, so a tie cannot be silently dropped to reach 9/9. The
   `11/1024` figure is the zero-tie marginal tail and a conservative bound when ties count as non-wins,
   not a substitute for the reported conditional p-value.
3. **The power calculation covers only the directional component.** With independent trials at a true
   per-pair C-win probability of 0.75, `P(≥9/10) = 0.244025`; at 0.90 it is `0.736099`. These are binomial
   calculations over pair outcomes, not power estimates for a median paired request ratio of 0.75. A 25%
   median reduction has no stated power without a distribution for the paired ratios. The full campaign's
   pass probability is lower still because ratio, correctness, lost-pass, elapsed-time and cost conditions
   also apply. The predeclared outcome remains “underpowered, no promotion” when the evidence is
   insufficient; no post-hoc relabeling as a directional win.

The campaign keeps v4's ≥9/10 and median-ratio bars, exact two-sided p-value reporting, ties-not-wins rule,
and “record underpowered, not directional.”

## 4. R5-3 — The edits description gets its own measured variant

**Replaces v4 §5 item 2.**

The current `internal/tools/testdata/codemode-schema.golden.json` is **542 B including its trailing
newline** (541 B marshaled), leaving 34 B under the 576 B test cap. V4's sketched clause is 161 B and cannot
fit. More importantly, additive, transport and read-only workspace sessions must not advertise `stageEdit`.

Ship a third spec description used **only** by `workspace + workspaceEdits`, exactly as follows:

```text
Workspace JS. Await tools.read({path}|{next})→{lines:[{n,text}]}, tools.search({pattern})→{matches:[{path,line,text}]}, tools.ls({path})→{entries:[{name,kind}]}, tools.glob({pattern})→{paths}. Await tools.stageEdit({path,edits:[{old,new}]})→{staged,path,edits} to stage exact edits the host applies after this program returns; read the file first; staged is not written. Return the answer; a bare expression is null.
```

Using the existing schema and the same JSON shape as the production golden, this is **712 B including the
trailing newline** (711 B marshaled). The edits-variant cap is **768 B**; the existing read-only 576 B and
network 640 B caps do not change. The byte measurement was checked by round-tripping the current golden
byte-for-byte with the serializer shape, not by counting characters.

Implementation and pins:

- `internal/tools/codemode.go`: select workspace, network and workspace-edits descriptions from the same
  capability inventory that feeds the prelude and dispatch. `stageEdit` occurs only in the edits variant's
  description, prelude and table. Resume keeps rev 3's original edit-declaration bit: if current edit
  authority is revoked, the edits-shaped spec remains but the method returns its typed refusal.
- `internal/tools/codemode_test.go`: add `TestCodemodeWorkspaceEditsSpec`; pin the exact golden, the 768 B
  cap, and table/spec agreement. Existing `TestCodemodeSpec` and refusal-spec tests continue to prove that
  read-only sessions do not acquire the edit clause. Add a resume test for edits-declared → edits-revoked.
- Create `internal/tools/testdata/codemode-workspace-edits-schema.golden.json`. Do not regenerate the two
  read-only goldens for this item; rev 3 §3.2 independently owns any network catalogue correction.
- Update the measured byte/budget record in `docs/specs/SPECS.md` and its byte mirror in the same behavior
  commit. A description change is batched with §3.2 only when both touch the same variant; there is no
  claim that every codemode user pays a cache reset for this new opt-in variant.

## 5. R5-4 — Consent tests run in processes that can express them

**Replaces v4 §7.**

Add one top-level `TestChangesetConsent` in `test/codemodegate/codemodegate_test.go`, with two explicit
test paths:

1. **Interactive ACP subtests.** Add `test/codemodegate/acp_client_test.go` under the same
   `//go:build codemodegate` tag, adapting the stdio child-process/request loop in
   `test/acpgate/acpgate_test.go:63–150`. Drive the built `moca acp`, not an in-process fake asker. Observe
   `session/request_permission`, its once/deny options, the parent tool call and the bounded diff preview.
   Prove ask and auto each ask once per nonempty batch; two successive batches ask twice; denial applies
   nothing and cannot reuse an earlier approval; yolo asks zero; plan refuses; an empty set asks zero.
2. **Headless `-p` subtests.** Reuse the existing codemodegate harness. Ask, auto and plan refuse with zero
   writes and a paired tool result; `--mode yolo` applies the expected bytes and returns the host receipt.
   Assert the provider-visible result and the filesystem, not unexported agent state.

The ACP helper is test-only and introduces no production kill switch, environment override or build-tagged
product hook. `TestZeroApprovalsInEveryMode` remains read-only and unchanged. The top-level gate count goes
from eight to nine; update the package comment and every pinned gate-count record that names top-level
tests. Run `make build && bash test/codemodegate/run.sh` and `bash test/acpgate/run.sh`; TUI rendering and
interaction remain the separate phase-5 TUI check.

## 6. R5-5 — Consent is deliberately stricter, not “the same” as direct writes

**Replaces v4 §3's alignment rationale; its mode table and question mechanics stand.**

`internal/permissions/jail.go:210–227` admits an in-workspace write without an approval, and
`internal/tools/write.go:37–90` has no general write question. Auto's optional classifier in
`internal/agent/mode.go:112–125` judges shell command strings; there is no changeset “unsure” classifier.
Therefore the interactive ask/auto changeset question is a **deliberate, changeset-specific guard** for one
aggregated multi-file mutation, not a restatement of existing direct-write behavior.

The policy is deterministic: interactive ask/auto invoke the host question exactly as v4's table says;
interactive yolo skips it; headless ask/auto/plan refuse; headless yolo applies; plan always refuses. No
provider classifier call, extra model request or `env.Ask != nil` front-end inference is added. The stricter
interactive policy is part of the scope requiring owner approval; the campaign does not silently relabel it
as measured consent parity.

## 7. R5-8 — P0 uses the existing `outside` refusal

**Replaces the `not_granted` portion of v4 §2 item 1 and its corresponding test expectation.**

`openWorkspace` accepts only grants that are directories at open time. A file entry is skipped and never
widened; a missing entry grants nothing. No rejected-file map, new refusal class or new result channel is
added for P0. A scripted read of the former exact file grant, a sibling beside it, or any other path not
covered by the workspace or a directory grant returns the existing typed **`outside`** error and no file
content.

Update the `ReadRoots`/holder comments that currently claim exact direct/script parity: directory grants
retain parity for directory content, while the direct tool alone retains its legitimate exact-file read
roots, including the config file. The standalone P0 release note explicitly says that script access to the
config file is removed. All other directory-grant tests and the synthetic `auth.json` fixture in v4 §2
stand; assert `outside` for both the former file and its sibling.

## 8. R5-7 — `.env.example` is an exception to both path predicates

**Replaces v4 §5 item 4.**

Ship the suffix allow-list. Define one case-insensitive helper over the final path component, conceptually
`envExample(base)`: it returns true only when `base` begins with `.env` and ends in `.example`, `.sample` or
`.template`. Use the **same helper** in both places:

1. the dot-prefixed-component rejection, which may exempt only that final component; and
2. the `.env*` credential-name rejection, which must not block an admitted example.

Every other dot-prefixed component remains refused, and all identity, link, read-stamp, UTF-8/NUL, size and
content rules still apply. This is a filename convention, not secret detection; the plan does not claim that
an example-suffixed file is safe in general.

Pin at least: `services/api/.env.example` and `.ENV.SAMPLE` can be staged; `.env`, `.env.local`,
`.env.example.local`, `services/.config/.env.template` and ordinary dotfiles are refused. Test the helper's
two call sites independently so fixing one predicate cannot leave the other denying the same path.

## 9. R5-6 — Corrected measurement record

The ten raw rows in each gitignored `test/evals/results/v2pilot-*.jsonl` file were parsed and summed again:

| Arm | Rows | Carry sum | Cache-read sum | Passes |
|---|---:|---:|---:|---:|
| off | 10 | 42,870 | 1,406,720 | 4 |
| additive (50) | 10 | 70,679 | 2,504,704 | 6 |
| additive-400 | 10 | 30,146 | 1,152,256 | 8 |
| transport | 10 | 32,863 | 1,387,520 | 9 |

`test/evals/BASELINE.md` now records 42,870 / 70,679 / 30,146 / **32,863** and its correction footnote
covers all three stale sums. No median, paired ratio, pass count, cost or verdict changes. Carry remains a
diagnostic under §3 and v4 §4 item 3.

## 10. What stands, what is not authorized, and the first batch

Unchanged: v4 §2's directory-only grant design except §7 above; v4 §3's consent matrix, immutable
allow-once question and revalidation; v4 §4 item 3 and instrumentation split; v4 §5 items 1 and 3; v4 §6;
and every inherited rev 3 passage not named above. In particular the staging primitive, path policy except
the narrow example exception, budgets, apply/journal/recovery contract, result/resume matrices, campaign
assignment and vetoes, and rev 3 §7's rejected scope all stand.

**Do not implement** an ACP campaign driver for the yolo protocol, a four-way independence product, a
`stageEdit` clause in read-only specs, a fake in-process “interactive” gate, a new `not_granted` class, or a
credential-only `.env.example` exception that leaves the dot-component denial in place.

Three design choices need explicit owner sign-off before their respective phases: unattended `-p --mode
yolo` changesets and the scoped campaign claim; the edits-only 768 B spec cap; and public env-example
suffixes. The standalone directory-only P0 repair does not depend on approving changesets. Campaign spend
still requires its own amount, ceiling and operator abort approval.

**First implementable batch after approval:** the standalone directory-only P0 fix with `outside`
regressions and its release note, independently of the changeset branch. Next come the read-only surface,
grader/instrumentation and hermetic ACP/headless consent seams; no writable method ships before the apply,
consent, preview and journal guards land together.
