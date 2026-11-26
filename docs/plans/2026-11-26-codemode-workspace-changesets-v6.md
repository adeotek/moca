# Codemode redesign: workspace changesets — implementation plan (revision 6)

**Status: amends revision 5, which amends revision 4 and adopts revision 3. Proposal, not approval.**
Baseline `cd3302055aec27b9fe79350a62052dc5c33f3d96`. Revisions 1–5 remain the record. Rev 5 is
accepted in full; the five amendments below are the only changes, and none of them revisits a design
decision. No implementation, release, default change or spend is authorized here.

**Why a delta.** Rev 5 closed the campaign-protocol contradiction, the golden's byte budget and the
statistics; retyping the combined plan to add five passages would be churn. Every replaced passage
appears below in operative form; §7 lists what stands.

## 1. Review of revision 5

Verified rather than assumed, against the tree and the rows:

| Rev 5 claim | Check | Result |
|---|---|---|
| Golden is 542 B including the newline, 34 B under a 576 B cap | `wc -c`, `codemode_test.go:72` | **Correct.** |
| The proposed edits description is 711 B marshaled / 712 B with newline | Rebuilt with Go's `json.Marshal` shape from the golden, substituted verbatim | **Correct to the byte**; 56 B under a 768 B cap. |
| `jail.go` admits an in-workspace write without an approval | `Resolve(path, write=true)` returns inside-root paths unconditionally | **Correct** (`internal/permissions/jail.go:210–227`). |
| Auto's classifier judges commands, not changesets | `judgeFn` returns `func(ctx, command string)` for `Env.Judge` | **Correct.** |
| The gate cannot observe an interactive approval | `test/codemodegate` is `moca -p` only; `acpgate_test.go:63–150` is the stdio client | **Correct.** |
| Carry and cache-read sums per arm | Recomputed from the gitignored `v2pilot-*.jsonl` rows | **All four correct** (42 870 / 70 679 / 30 146 / 32 863; 1 406 720 / 2 504 704 / 1 152 256 / 1 387 520). |
| `BASELINE.md` now carries 32 863 | `git diff` shows all three sums and the footnote already applied | **Correct** — that record correction is complete; nothing left open. |
| Four sign-test events are dependent | C's run feeds both comparisons in a triple | **Correct**; my rev-4 "multiplies" was wrong, and the subset bound (≤ 11/1024) is the right way to say it. |
| `.env.example` collides with the dot-component rule | Dot components are refused by a separate predicate | **Correct**; one shared helper over the final component is the only workable shape. |

Rev 5's eight findings are accepted. Its §2 (one campaign protocol), §3 (dependence-safe statistics),
§4 (edits-only spec with its own golden and cap), §6 (consent is deliberately stricter than direct
writes), §7 (`outside`, no new class) and §8 (the env-example helper) are adopted whole.

## 2. A1 — A revoked grant needs a refusal that names the remedy

**Adds to rev 5 §4's resume paragraph.** Rev 5 says a session that declared edits keeps the edits-shaped
spec and "the method returns its typed refusal" if current authority is revoked. With the table built
only from the *live* variant, `bridge.dispatch` answers
`CallError{Class:"refused", Message:"no such tool in this session: stageEdit"}` — a refusal, but a
misleading one: the tool exists, the grant does not.

- The edits-declared-but-revoked session registers a `stageEdit` capability whose call returns
  `Class:"denied"` and a message naming both remedies — turn the grant back on for future calls, or use
  the direct `edit`, which is not affected. Reads keep working; nothing is staged.
- Test in `internal/tools` (unit) and the resume leg of rev 5 §5: a session with the bit set and
  authority revoked gets that message, `applied`/`staged` never appears, and a direct `edit` in the same
  session still lands.
- This is the same pattern as the existing refusal-only `codemode` tool: the prompt promised a shape, so
  the shape answers — but with the remedy, not "no such tool".

## 3. A2 — The consent surface is two strings; size and render them deliberately

**Replaces rev 3 §4's "Show counts, normalized paths and the exact diff … before consent" and adds to
rev 5 §5.** Checked: both front ends carry the question in `Subject` and `Detail`, and neither can show
an arbitrary diff.

- **ACP** (`internal/acp/session.go:283–319`) sends `session/request_permission` with
  `Title: q.Kind + ": " + q.Subject` and `RawInput: {"detail": q.Detail}` — `Detail` is a **single
  string**. **TUI** (`internal/tui/approval.go:59–112`) renders `Detail` line by line, and `approvalPanel`
  caps it at `approvalDetailRows = 6` (`approval.go:51`), with the remainder promised to a pager.
- Therefore: `Subject` is `"<n> files, <m> edits"`; `Detail` is the ordered relative paths with each
  file's edit count, then the hunks, **bounded to 16 KiB** with an explicit elision marker and the count
  of omitted hunks. The full diff belongs to the receipt after apply, not to the question.
- The TUI needs a `Kind == "changeset"` branch in `approvalPanel`: today six rows of a batch is a list
  of paths with no room for hunks and no confirmation that the rest is reachable. Either scroll that
  panel or route the body to the pager and keep the prompt line. This is a real TUI change, not a
  rendering accident — 32 files approved from five visible lines is the failure mode the question
  exists to prevent.
- Tests assert the **string handed to `Ask`** in both clients (every path present, counts right,
  elision marker when truncated), plus the TUI view-model branch. The ACP leg asserts exactly two
  options — one allow-once and one deny — because `NoSession: true` is what removes the
  allow-for-session row (`approval.go:33–48`, `session.go:288–301`).

## 4. A3 — `-p --mode yolo` makes the control stronger, and the interpretation must say so

**Adds to rev 5 §2's "Scope of the result".** In yolo, `shell` is auto-approved in **every** arm, so the
A and B arms have the most unconstrained non-JS path moca can offer — the v2 pilot already showed a
shell-only arm solving the 120-service join. A C win under this protocol therefore beats a fully
unconstrained shell, not a hobbled control; that is the strongest form of the claim the campaign can
make, and it must be written into the interpretation rather than discovered by a reader. It also means
the campaign measures the primitive, never consent: yolo removes the dialog from all three arms.

## 5. A4 — Pin the variant *resolution*, not only the variant bytes

**Adds to rev 5 §4's pins.** Rev 5 pins the new golden and its cap inside `internal/tools`, where the
spec is built. The wiring that matters is one layer up, where a session decides which variant it gets:

- `internal/agent`: a resolved `workspace + workspaceEdits` session registers a `codemode` spec whose
  description contains `stageEdit`; `additive`, `transport` and read-only `workspace` sessions do not;
  `network: true` sessions get the network text and never `stageEdit`.
- `internal/tools`: one table test over all variants — every method in the capability table appears in
  that variant's description, and every name in a description exists in that variant's table. This is
  rev 3 §3.2's "one inventory" rule applied to all three descriptions, so a method added later cannot
  silently miss one.
- Keep rev 5's byte caps as the budget and its explicit "do not regenerate the read-only goldens here".

## 6. A5 — The P0 note must mention the additive behaviour change

**Adds to rev 5 §7's release note.** The fix removes file grants and stops applying grants outside
transport, so a session with `codemode.enabled: true` in additive loses absolute scripted reads it
should never have had: `read({path: "/…/config.jsonc"})` now returns `outside`. The note says both
halves — script access to the config file is removed, and additive sessions no longer receive external
grants at all (SPECS §10.6 always said transport-only) — and points at the direct `read` tool as the
supported way to read those roots. One sentence, in the release notes, not buried in a commit message.

## 7. What stands

Unchanged and in force: everything in rev 5 (§1 dispositions, §2 campaign protocol, §3 statistics,
§4 spec variants and caps, §5 consent tests, §6 stricter-than-direct consent, §7 `outside`, §8 env-example
helper, §9 measurement record, §10 scope), everything it inherits from rev 4 (§2 directory-only grants,
§3 consent matrix, §4 items 1 and 3, §5 items 1 and 3, §6 pins), and every rev 3 passage not replaced
along the way — the surface table, primitive, read eligibility, path policy, budgets, apply, crash
record, result, resume matrix, phases, §7's rejected scope.

Unchanged "would make this plan wrong" list (from rev 4): a directory-only grant set that cannot serve
transport; a judgment that `-p --mode yolo` is too permissive; or a feasibility check where an explicit
exact-edit list reaches the same request floor as a program.

Still needing the owner, unchanged: the unattended-yolo scope decision, the 768 B edits-variant cap, the
env-example suffixes, and the campaign's own amount, ceiling and abort authority. The standalone
directory-only P0 fix remains the first implementable item and depends on none of them.
