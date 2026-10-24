# Publishing a new version of moca

The operator-facing procedure for cutting a release. The design/contract details
live in `docs/specs/SPECS.md` §2 (release workflow, `moca update`) and §16
(governance); this page is the sequence of commands.

Release principle: **publishing is an explicit human decision.** A `v*` tag push
deliberately does *not* trigger the release workflow — it is a manual dispatch
only (`.github/workflows/release.yml`), runnable from Actions → Release → Run
workflow.

## 0. Prerequisites — the release PR

Before any tag exists, the release content is merged to `main`:

1. Work on a branch (`phase/N-<slug>` or `chore/<slug>`), commit and PR titles in
   the house format `[type:category] short description`,
   PR-only into `main` (the local `.git/hooks/pre-push` hook blocks direct
   pushes to `main`).
2. In the same PR, update every artifact the version touches:
   `CHANGELOG.md`, `docs/specs/SPECS.md` (header version line + the change
   narrative cited from §16), `README.md` if user-facing. One version bump per
   PR.
3. An agent may prepare the release, but never commit without your explicit
   approval.

## 1. Local gates

```bash
export PATH="$(mise where go)/bin:$PATH"   # go is not on the global PATH on every host
make vet                                   # go vet + gofmt -l must both be clean
make test                                  # go test ./... -race -count=1
```

Optional, as the change warrants (all need `make build` first, none in CI):

| Gate | Command | When |
|---|---|---|
| Skillgate (hermetic) | `bash test/skillgate/run.sh` | skills/skill-write-guard changes |
| ACP gate (hermetic) | `bash test/acpgate/run.sh` | `moca acp` or wire changes |
| Ship gate (needs repo `.env`) | `bash test/shipgate/run.sh` | agent-loop changes |
| Eval corpus (needs `.env`) | `bash test/evals/run.sh -n 3 -l <label>` | prompt/harness changes — A/B against `test/evals/BASELINE.md` |
| Release rehearsal | `bash test/evals/replay/run.sh` | harness/token-accounting changes |

CI (`.github/workflows/ci.yml`) runs the same four `make` gates on every PR to
`main`; green PR checks are the minimum bar for tagging.

## 2. Tag

`VERSION` everywhere (`make build`, the workflow, `moca update --check`,
installers, the Homebrew formula's `livecheck`) is stamped from `git describe`,
so the tag must be a `v*` semver tag on the merge commit:

```bash
git tag v0.7.11
git push origin v0.7.11     # tags push; main itself stays PR-only
```

If you forgot to tag before dispatching, the workflow fails loudly: the
`version` build/packaging step refuses a dispatcher-supplied `version` that
is not an existing tag (`git rev-parse --verify refs/tags/$version`).

## 3. Dispatch the Release workflow

GitHub → Actions → **Release** → Run workflow.

- **`version` input empty** — the dispatched ref must be `main` sitting exactly
  on the tag; the version comes from `git describe`. This is the normal case.
- **`version` input set to the tag** — use this when a fix to the release
  workflow itself lands *after* the tag: the workflow file is read from the
  dispatched ref, but the **source** is checked out from that tag's tree, so
  the tag never needs to be moved.

What it does: builds six packages (linux, darwin, windows × amd64, arm64,
`CGO_ENABLED=0`, `-trimpath`, the version ldflag), verifies each binary's
GOOS/GOARCH from the build metadata (`go version -m`) and the linux/amd64 one's
`--version` stamp, packs `moca-<version>-<os>-<arch>.tar.gz` (`.zip` on
Windows) + `LICENSE`, generates `checksums.txt` (`sha256sum moca-*`), and:

- tag exists → `gh release create <tag> dist/* --generate-notes` (or re-uploads
  with `--clobber` if the release already exists);
- tag missing → creates a **draft** release targeted at the dispatched SHA — a
  rehearsal. Drafts are invisible to `moca update` and the installers. Publish
  by hand only if it is meant to ship.

The asset names are a **contract** with `internal/update` (`moca update`
composes `moca-<version>-<os>-<arch>[.tar.gz|.zip]` from `runtime.GOOS`/
`GOARCH`) and with `install.sh`/`install.ps1`. Adding a platform is a matrix
row, not a code change.

Local cross-check (optional, produces the same six binaries, no publish):

```bash
make release    # dist/moca-<version>-<os>-<arch>
```

## 4. Verify the release

```bash
gh release view v0.7.11          # 6 packages + checksums.txt, not a draft

# and end to end, on a disposable copy (--version, or install.sh alone to
# resolve the latest release):
./install.sh --version v0.7.11 --dir /tmp/moca-upgrade-test
```

Check the generated release notes render sanely (GitHub builds them from the
merged pull requests since the previous tag).

## 5. Publish the distribution channels

Only after the release and its `checksums.txt` exist — the renderers read the
release's own checksums and fail loudly naming any missing package:

```bash
scripts/release/publish-channels.sh --tag v0.7.11 --verify
```

This renders, never pushes, and (with `--registry-repo`/`--tap-repo`) copies
into existing clones of the target repos before printing the commands:

| Rendered | Target repo | How it ships |
|---|---|---|
| `acp-registry/ moca/agent.json` + `moca/icon.svg` | `agentclientprotocol/registry` (its builder validates, verifier re-hashes + runs `moca acp`) | push/PR from your registry fork |
| `homebrew-tap/ Formula/moca.rb` | `adeotek/homebrew-tap` | push from your tap clone |
| `winget/` three manifests | `microsoft/winget-pkgs` | PR through the contents API (too large to clone) |

Pushing is a human decision; nothing in the pipeline publishes on your behalf.

## 6. Post-release

- Update this page's examples if a step moved.
- Run `graphify update .` if source changed (AST-only, no API cost).
- Any release-workflow fix discovered during dispatch lands as a normal PR with
  the workflow's `version` input pointing at the already-published tag — do not
  move the tag.
