#!/usr/bin/env bash
# Release a published moca version to one, several or all distribution channels.
#
#   scripts/release/release-channels.sh --version v0.8.4
#   scripts/release/release-channels.sh --version v0.8.4 --channel homebrew
#   scripts/release/release-channels.sh --version v0.8.4 --channel acp --channel winget
#   scripts/release/release-channels.sh --version v0.8.4 --dry-run
#
# Channels:
#   acp       agentclientprotocol/registry   -> moca/agent.json + moca/icon.svg, PR from your fork
#   homebrew  adeotek/homebrew-tap           -> Formula/moca.rb, committed to main directly
#   winget    microsoft/winget-pkgs          -> three manifests, PR from your fork (API, no clone)
#
# The release itself must already be published (the GitHub Release with all six
# packages + checksums.txt); this script never creates it. It renders every
# selected channel with the renderers beside it, then checks the release:
# checksums downloaded and verified against the archives, every channel's own
# current version read from its repository, and only then mutates anything.
# A failure before the first mutation leaves every channel untouched.
#
# Running it again is safe: a channel already carrying the version is verified
# and skipped, an open PR for the version is reported instead of duplicated.
set -euo pipefail

MOCA_REPO="${MOCA_REPO:-adeotek/moca}"

usage() {
	cat <<'EOF'
usage: release-channels.sh --version vX.Y.Z [options]

  --version, --tag <vX.Y.Z>  published release to distribute (required)
  --channel <name>           acp, homebrew, winget, or all (repeatable,
                             comma-separated too; default: all)
  --dry-run                  run every check and render, change nothing
  --allow-downgrade          allow the tap to move to an older version
  --tap-repo <owner/repo>    Homebrew tap (default: adeotek/homebrew-tap)
  --registry-repo <o/r>      ACP registry (default: agentclientprotocol/registry)
  --winget-repo <owner/repo> winget package repo (default: microsoft/winget-pkgs)
  --workdir <dir>            working directory (default: a fresh temp dir;
                             kept on failure so the rendered files survive)
  -h, --help                 this text

Requires: gh (authenticated), git, python3, sha256sum or shasum; uv for the
ACP registry's own validator. Forks are created on demand for the two
upstream repositories; the tap must already exist.
EOF
}

die() { printf 'error: %s\n' "$*" >&2; exit 1; }
usage_err() {
	printf 'error: %s\n' "$*" >&2
	usage >&2
	exit 2
}
need() { command -v "$1" >/dev/null 2>&1 || die "missing required command: $1"; }

# newer a b: a sorts after b (release ordering, not semver prerelease rules —
# v0.8.4 > v0.8.3 and v0.8.4-beta < v0.8.4 is all the guard needs).
newer() {
	[ "$1" != "$2" ] || return 1
	[ "$(printf '%s\n%s\n' "$1" "$2" | sort -V | tail -n 1)" = "$1" ]
}

tag=""
channels=""
dry_run=""
allow_downgrade=""
tap_repo="adeotek/homebrew-tap"
registry_repo="agentclientprotocol/registry"
winget_repo="microsoft/winget-pkgs"
workdir=""
workdir_owned=""

while [ $# -gt 0 ]; do
	case "$1" in
	--version | --tag)
		[ $# -ge 2 ] || usage_err "$1 needs a value"
		tag="$2"
		shift 2
		;;
	--channel)
		[ $# -ge 2 ] || usage_err "--channel needs a value"
		channels="$channels,$2"
		shift 2
		;;
	--dry-run)
		dry_run=1
		shift
		;;
	--allow-downgrade)
		allow_downgrade=1
		shift
		;;
	--tap-repo)
		[ $# -ge 2 ] || usage_err "--tap-repo needs a value"
		tap_repo="$2"
		shift 2
		;;
	--registry-repo)
		[ $# -ge 2 ] || usage_err "--registry-repo needs a value"
		registry_repo="$2"
		shift 2
		;;
	--winget-repo)
		[ $# -ge 2 ] || usage_err "--winget-repo needs a value"
		winget_repo="$2"
		shift 2
		;;
	--workdir)
		[ $# -ge 2 ] || usage_err "--workdir needs a value"
		workdir="$2"
		shift 2
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		usage_err "unknown argument: $1"
		;;
	esac
done

[ -n "$tag" ] || usage_err "--version is required"
[[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([._-][0-9A-Za-z.-]+)?$ ]] ||
	usage_err "version must look like vX.Y.Z or vX.Y.Z-suffix, got '$tag'"
version="${tag#v}"

want_acp=""
want_homebrew=""
want_winget=""
add_channel() {
	case "$1" in
	all)
		want_acp=1 want_homebrew=1 want_winget=1
		;;
	acp) want_acp=1 ;;
	homebrew | tap) want_homebrew=1 ;;
	winget) want_winget=1 ;;
	"") ;;
	*) usage_err "unknown channel '$1' (acp, homebrew, winget, all)" ;;
	esac
}
# `--chat a,b --channel c` -> the selected channels; commas and spaces both
# split. No --channel at all means all three.
[ -n "$channels" ] || channels="all"
for c in $(printf '%s' "$channels" | tr ',' ' '); do add_channel "$c"; done
[ -n "$want_acp$want_homebrew$want_winget" ] || usage_err "no channel selected"

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
if [ -n "$workdir" ]; then
	mkdir -p "$workdir"
	[ -z "$(ls -A "$workdir")" ] || die "--workdir must be empty: $workdir"
else
	workdir="$(mktemp -d "${TMPDIR:-/tmp}/moca-release-channels.XXXXXX")"
	workdir_owned=1
fi
workdir="$(cd "$workdir" && pwd)"

cleanup() {
	local rc=$?
	if [ $rc -ne 0 ]; then
		printf '\nworking directory kept: %s\n' "$workdir" >&2
	elif [ -n "$workdir_owned" ]; then
		rm -rf "$workdir"
	fi
}
trap cleanup EXIT

# --- prerequisites -----------------------------------------------------------

need gh
need git
need python3
gh auth status >/dev/null 2>&1 || die "gh is not authenticated — run: gh auth login"
fork_owner="$(gh api user --jq .login)" || die "cannot resolve the authenticated GitHub user"
gh api "repos/$MOCA_REPO" >/dev/null 2>&1 || die "cannot read repository $MOCA_REPO"
git -C "$root" remote get-url origin >/dev/null 2>&1 || die "$root is not a git checkout of moca"
git config --get user.email >/dev/null 2>&1 || die "git user.email is not configured — set it before releasing"
[ -z "$want_acp" ] || need uv

# --- release preflight -------------------------------------------------------

printf 'release: %s (%s)\n' "$tag" "$MOCA_REPO"
release_json="$(gh release view "$tag" --repo "$MOCA_REPO" --json tagName,isDraft,assets 2>/dev/null)" ||
	die "release '$tag' is not published in $MOCA_REPO — publish it before releasing its channels"
release_ok="$(printf '%s' "$release_json" | python3 -c '
import json, sys
d = json.load(sys.stdin)
print("draft" if d.get("isDraft") else "published")
' 2>/dev/null)" || die "cannot parse the release metadata for $tag"
[ "$release_ok" = "published" ] || die "release '$tag' is still a draft — publish it first"

expected_assets="checksums.txt
moca-$tag-linux-amd64.tar.gz
moca-$tag-linux-arm64.tar.gz
moca-$tag-darwin-amd64.tar.gz
moca-$tag-darwin-arm64.tar.gz
moca-$tag-windows-amd64.zip
moca-$tag-windows-arm64.zip"
have_assets="$(printf '%s' "$release_json" | python3 -c '
import json, sys
for a in json.load(sys.stdin).get("assets", []):
    print(a["name"])
')" || die "cannot list the release assets"
for asset in $expected_assets; do
	printf '%s\n' "$have_assets" | grep -qxF "$asset" ||
		die "release $tag is missing asset $asset — publish the full release first"
done
printf '  assets     all seven present\n'

mkdir -p "$workdir/download"
gh release download "$tag" --repo "$MOCA_REPO" --dir "$workdir/download" --clobber \
	--pattern 'checksums.txt' --pattern 'moca-*' >/dev/null ||
	die "cannot download the release assets for $tag"
if command -v sha256sum >/dev/null 2>&1; then
	(cd "$workdir/download" && sha256sum -c --quiet checksums.txt) ||
		die "checksums.txt does not match the downloaded archives"
elif command -v shasum >/dev/null 2>&1; then
	(cd "$workdir/download" && shasum -a 256 -c checksums.txt >/dev/null) ||
		die "checksums.txt does not match the downloaded archives"
else
	die "sha256sum (Linux) or shasum (macOS) is required to verify the release"
fi
printf '  checksums  verified against the archives\n'

# --- render every selected channel before touching any repository ------------

render_root="$workdir/render"
mkdir -p "$render_root"

if [ -n "$want_acp" ]; then
	printf 'render: acp…\n'
	python3 "$root/scripts/release/render-acp-registry.py" --tag "$tag" --out "$render_root/acp" --verify >/dev/null ||
		die "the ACP registry entry failed to render for $tag"
fi
if [ -n "$want_homebrew" ]; then
	printf 'render: homebrew…\n'
	python3 "$root/scripts/release/render-homebrew-formula.py" --tag "$tag" --out "$render_root/homebrew" >/dev/null ||
		die "the Homebrew formula failed to render for $tag"
fi
if [ -n "$want_winget" ]; then
	printf 'render: winget…\n'
	python3 "$root/scripts/release/render-winget-manifests.py" --tag "$tag" --out "$render_root/winget" --verify >/dev/null ||
		die "the winget manifests failed to render for $tag"
fi

# --- current state -----------------------------------------------------------

pr_view() { # repo, head -> "state<TAB>url" or empty
	gh pr list --repo "$1" --head "$fork_owner:$2" --state all --json state,url \
		--jq 'if length == 0 then empty else (.[0] | [.state, .url] | @tsv) end' 2>/dev/null || true
}

hb_current=""
if [ -n "$want_homebrew" ]; then
	hb_b64=""
	if hb_json="$(gh api "repos/$tap_repo/contents/Formula/moca.rb" 2>/dev/null)"; then
		hb_b64="$(printf '%s' "$hb_json" | python3 -c 'import json, sys; print(json.load(sys.stdin).get("content", ""))')" ||
			die "cannot parse $tap_repo/Formula/moca.rb"
	fi
	[ -n "$hb_b64" ] || die "cannot read Formula/moca.rb from $tap_repo"
	hb_formula="$(printf '%s' "$hb_b64" | base64 -d 2>/dev/null)" || die "cannot decode $tap_repo/Formula/moca.rb"
	if [[ "$hb_formula" =~ releases/download/(v[^/]+)/ ]]; then
		hb_current="${BASH_REMATCH[1]}"
	else
		die "cannot find a release tag in $tap_repo/Formula/moca.rb"
	fi
	if [ "$hb_current" != "$tag" ] && newer "$hb_current" "$tag" && [ -z "$allow_downgrade" ]; then
		die "$tap_repo already carries $hb_current, newer than $tag (pass --allow-downgrade to move it back)"
	fi
fi

acp_state=""
acp_current=""
acp_branch="moca-$version"
acp_b64=""
if [ -n "$want_acp" ]; then
	if acp_json="$(gh api "repos/$registry_repo/contents/moca/agent.json" 2>/dev/null)"; then
		acp_b64="$(printf '%s' "$acp_json" | python3 -c 'import json, sys; print(json.load(sys.stdin).get("content", ""))')" ||
			die "cannot parse $registry_repo/moca/agent.json"
	fi
	if [ -n "$acp_b64" ]; then
		acp_current="$(printf '%s' "$acp_b64" | base64 -d 2>/dev/null |
			python3 -c 'import json, sys; print(json.load(sys.stdin).get("version", ""))' 2>/dev/null)" ||
			die "cannot parse $registry_repo/moca/agent.json"
		if [ "$acp_current" = "$version" ]; then
			acp_state="skip"
		elif [ -n "$acp_current" ] && newer "v$acp_current" "$tag"; then
			die "$registry_repo already carries moca $acp_current, newer than $tag"
		fi
	fi
	acp_pr="$(pr_view "$registry_repo" "$acp_branch")"
	if [ -n "$acp_pr" ]; then
		acp_pr_state="${acp_pr%%$'\t'*}"
		acp_pr_url="${acp_pr#*$'\t'}"
		[ "$acp_pr_state" = "OPEN" ] || die "a $acp_pr_state PR already exists for $acp_branch: $acp_pr_url"
		acp_state="pr"
	fi
fi

wg_state=""
wg_prior=""
wg_branch="adeotek.moca-$version"
wg_dir="manifests/a/adeotek/moca/$version"
if [ -n "$want_winget" ]; then
	wg_prior=""
	if wg_json="$(gh api "repos/$winget_repo/contents/manifests/a/adeotek/moca" 2>/dev/null)"; then
		wg_prior="$(printf '%s' "$wg_json" | python3 -c 'import json, sys
d = json.load(sys.stdin)
print("\n".join(x["name"] for x in d) if isinstance(d, list) else "")')" ||
			die "cannot list manifests/a/adeotek/moca in $winget_repo"
	fi
	if gh api "repos/$winget_repo/contents/$wg_dir" >/dev/null 2>&1; then
		wg_state="skip"
	fi
	wg_pr="$(pr_view "$winget_repo" "$wg_branch")"
	if [ -n "$wg_pr" ]; then
		wg_pr_state="${wg_pr%%$'\t'*}"
		wg_pr_url="${wg_pr#*$'\t'}"
		[ "$wg_pr_state" = "OPEN" ] || die "a $wg_pr_state PR already exists for $wg_branch: $wg_pr_url"
		wg_state="pr"
	fi
fi

# --- plan --------------------------------------------------------------------

print_plan() { # channel, action, detail
	printf '  %-9s %-18s %s\n' "$1" "$2" "$3"
}

printf '\nplan:\n'
if [ -n "$want_homebrew" ]; then
	if [ "$hb_current" = "$tag" ]; then
		print_plan homebrew "already released" "$tap_repo carries $tag; nothing to push"
	else
		print_plan homebrew "commit and push" "$tap_repo $hb_current -> $tag (main)"
	fi
fi
if [ -n "$want_acp" ]; then
	case "$acp_state" in
	skip) print_plan acp "already released" "$registry_repo carries moca $version" ;;
	pr) print_plan acp "PR already open" "$acp_pr_url" ;;
	*) print_plan acp "open PR" "$registry_repo moca/ (from $fork_owner:$acp_branch)" ;;
	esac
fi
if [ -n "$want_winget" ]; then
	case "$wg_state" in
	skip) print_plan winget "already released" "$winget_repo has $wg_dir" ;;
	pr) print_plan winget "PR already open" "$wg_pr_url" ;;
	*) print_plan winget "open PR" "$winget_repo $wg_dir (from $fork_owner:$wg_branch)" ;;
	esac
fi

# validate_registry_entry runs the registry's own tools over a checkout that
# carries the rendered moca/ directory: build_registry.py checks the schema and
# the URLs, verify_agents.py downloads the linux archive and launches `moca acp`.
validate_registry_entry() { # dir
	printf '  validating with the registry'"'"'s own build and launch checks…\n'
	(cd "$1" && uv run --quiet --with jsonschema .github/workflows/build_registry.py --agents moca >/dev/null) ||
		die "the registry build rejected the rendered entry"
	(cd "$1/.github/workflows" &&
		uv run --quiet python verify_agents.py --agent moca --auth-check --timeout 120 --sandbox-dir "$1/.sandbox" >/dev/null) ||
		die "the registry auth check failed for moca $version"
}

if [ -n "$dry_run" ]; then
	# The registry has its own schema + launch validators; run them against the
	# rendered entry now, so a dry run proves the entry as well as the release.
	if [ -n "$want_acp" ] && [ "$acp_state" != "skip" ] && [ "$acp_state" != "pr" ]; then
		printf '\ndry run: validating the entry with the registry'"'"'s own checks…\n'
		check_dir="$workdir/registry-check"
		gh repo clone "$registry_repo" "$check_dir" -- --depth 1 --quiet >/dev/null 2>&1 ||
			die "cannot clone $registry_repo for the dry-run check"
		mkdir -p "$check_dir/moca"
		cp "$render_root/acp/moca/agent.json" "$render_root/acp/moca/icon.svg" "$check_dir/moca/"
		validate_registry_entry "$check_dir"
	fi
	printf '\ndry run: checks and renders passed; nothing was changed.\n'
	exit 0
fi

# --- mutations ---------------------------------------------------------------

assert_clean_repo() { # dir
	[ -z "$(git -C "$1" status --porcelain)" ] || die "$1 has uncommitted changes; refusing to build a release commit on top"
}

# Homebrew: a personal tap, committed to main directly (no PR).
if [ -n "$want_homebrew" ] && [ "$hb_current" != "$tag" ]; then
	printf '\nhomebrew: %s -> %s\n' "$hb_current" "$tag"
	tap_dir="$workdir/tap"
	gh repo clone "$tap_repo" "$tap_dir" -- --depth 1 --quiet >/dev/null 2>&1 ||
		die "cannot clone $tap_repo"
	assert_clean_repo "$tap_dir"
	cp "$render_root/homebrew/Formula/moca.rb" "$tap_dir/Formula/moca.rb"
	if command -v ruby >/dev/null 2>&1; then
		ruby -c "$tap_dir/Formula/moca.rb" >/dev/null 2>&1 || die "the rendered formula is not valid Ruby"
	fi
	if [ -z "$(git -C "$tap_dir" diff --name-only)" ]; then
		die "$tap_repo already has the formula for $tag but names $hb_current — investigate"
	fi
	git -C "$tap_dir" add Formula/moca.rb
	git -C "$tap_dir" commit -q -m "moca $tag"
	git -C "$tap_dir" push origin HEAD:main >/dev/null 2>&1 ||
		die "push to $tap_repo failed (branch protection or a concurrent update?)"
	hb_after_b64=""
	if hb_after_json="$(gh api "repos/$tap_repo/contents/Formula/moca.rb" 2>/dev/null)"; then
		hb_after_b64="$(printf '%s' "$hb_after_json" | python3 -c 'import json, sys; print(json.load(sys.stdin).get("content", ""))')" ||
			die "cannot parse $tap_repo/Formula/moca.rb after the push"
	fi
	[ -n "$hb_after_b64" ] || die "cannot re-read $tap_repo/Formula/moca.rb after the push"
	hb_after="$(printf '%s' "$hb_after_b64" | base64 -d 2>/dev/null)"
	[[ "$hb_after" =~ releases/download/(v[^/]+)/ ]] || die "the tap formula lost its release URL"
	[ "${BASH_REMATCH[1]}" = "$tag" ] ||
		die "tap verification failed: $tap_repo still names ${BASH_REMATCH[1]}, wanted $tag"
	printf '  released  https://github.com/%s (main)\n' "$tap_repo"
fi

ensure_fork() { # upstream repo
	local upstream="$1" name="${1##*/}"
	if gh repo view "$fork_owner/$name" >/dev/null 2>&1; then
		return 0
	fi
	printf '  forking %s -> %s/%s…\n' "$upstream" "$fork_owner" "$name"
	gh repo fork "$upstream" --clone=false >/dev/null 2>&1 ||
		die "cannot fork $upstream"
	for _ in $(seq 1 30); do
		gh repo view "$fork_owner/$name" >/dev/null 2>&1 && return 0
		sleep 2
	done
	die "fork $fork_owner/$name did not appear"
}

# ACP registry: render, validate with the registry's own scripts, push a fork
# branch and open the PR.
if [ -n "$want_acp" ] && [ "$acp_state" != "skip" ] && [ "$acp_state" != "pr" ]; then
	printf '\nacp: moca %s -> %s\n' "${acp_current:-not listed}" "$version"
	ensure_fork "$registry_repo"
	registry_dir="$workdir/registry"
	gh repo clone "$fork_owner/registry" "$registry_dir" -- --depth 1 --quiet >/dev/null 2>&1 ||
		die "cannot clone the registry fork $fork_owner/registry"
	registry_base="$(gh api "repos/$registry_repo" --jq .default_branch)" || die "cannot read $registry_repo"
	git -C "$registry_dir" remote add upstream "https://github.com/$registry_repo.git" 2>/dev/null || true
	git -C "$registry_dir" fetch --depth 1 -q upstream "$registry_base:refs/remotes/upstream/$registry_base" ||
		die "cannot fetch $registry_repo"
	git -C "$registry_dir" checkout -q -B "$acp_branch" "upstream/$registry_base"
	mkdir -p "$registry_dir/moca"
	cp "$render_root/acp/moca/agent.json" "$render_root/acp/moca/icon.svg" "$registry_dir/moca/"
	validate_registry_entry "$registry_dir"
	git -C "$registry_dir" add moca
	git -C "$registry_dir" commit -q -m "Add moca as a binary agent" -m "moca $tag"
	git -C "$registry_dir" push -q -u origin "$acp_branch" ||
		die "push to the registry fork failed"
	acp_title="Add moca as a binary agent"
	[ -z "$acp_current" ] || acp_title="Update moca to $tag"
	acp_url="$(gh pr create --repo "$registry_repo" --base "$registry_base" \
		--head "$fork_owner:$acp_branch" --title "$acp_title" \
		--body "Regenerated the moca entry for $tag from the published release (six binary targets, sha256 from the release's \`checksums.txt\`). Validated with \`.github/workflows/build_registry.py --agents moca\` and \`verify_agents.py --agent moca --auth-check\` before pushing.")" ||
		die "cannot open the ACP registry PR"
	printf '  PR opened %s\n' "$acp_url"
fi

# winget: the repository is too large to clone, so the manifests go up through
# the contents API on a fork branch.
if [ -n "$want_winget" ] && [ "$wg_state" != "skip" ] && [ "$wg_state" != "pr" ]; then
	printf '\nwinget: adeotek.moca %s\n' "$version"
	ensure_fork "$winget_repo"
	# Best effort: fast-forward the fork so the branch starts near upstream.
	gh api -X POST "repos/$fork_owner/winget-pkgs/merge-upstream" -f branch=master >/dev/null 2>&1 || true
	wg_base_sha="$(gh api "repos/$fork_owner/winget-pkgs/git/ref/heads/master" --jq .object.sha 2>/dev/null)" ||
		die "cannot read $fork_owner/winget-pkgs master (is the fork empty?)"
	if gh api "repos/$fork_owner/winget-pkgs/git/ref/heads/$wg_branch" >/dev/null 2>&1; then
		die "branch $fork_owner/winget-pkgs:$wg_branch already exists without a PR — delete it and re-run"
	fi
	gh api -X POST "repos/$fork_owner/winget-pkgs/git/refs" \
		-f ref="refs/heads/$wg_branch" -f sha="$wg_base_sha" >/dev/null ||
		die "cannot create branch $wg_branch on the winget fork"
	wg_title="New package: adeotek.moca version $version"
	wg_message="$wg_title"
	[ -z "$wg_prior" ] || {
		wg_title="Update: adeotek.moca version $version"
		wg_message="$wg_title"
	}
	for manifest in adeotek.moca.yaml adeotek.moca.installer.yaml adeotek.moca.locale.en-US.yaml; do
		manifest_b64="$(base64 <"$render_root/winget/$manifest" | tr -d '\n')"
		gh api -X PUT "repos/$fork_owner/winget-pkgs/contents/$wg_dir/$manifest" \
			-f message="$wg_message" -f content="$manifest_b64" -f branch="$wg_branch" >/dev/null ||
			die "cannot upload $manifest to the winget fork"
	done
	wg_title="New package: adeotek.moca version $version"
	[ -z "$wg_prior" ] || wg_title="Update: adeotek.moca version $version"
	wg_url="$(gh pr create --repo "$winget_repo" --base master \
		--head "$fork_owner:$wg_branch" --title "$wg_title" \
		--body "Release: https://github.com/$MOCA_REPO/releases/tag/$tag

The manifests are generated from the release's own \`checksums.txt\`; both Windows
archives were HEAD-checked before upload.")" ||
		die "cannot open the winget PR"
	printf '  PR opened %s\n' "$wg_url"
fi

# --- summary -----------------------------------------------------------------

printf '\nsummary:\n'
if [ -n "$want_homebrew" ]; then
	if [ "$hb_current" = "$tag" ]; then
		print_plan homebrew "already released" "$tap_repo carries $tag"
	else
		print_plan homebrew "released" "https://github.com/$tap_repo (main)"
	fi
fi
if [ -n "$want_acp" ]; then
	case "$acp_state" in
	skip) print_plan acp "already released" "$registry_repo carries moca $version" ;;
	pr) print_plan acp "PR already open" "$acp_pr_url" ;;
	*) print_plan acp "PR opened" "$acp_url (waiting on registry review)" ;;
	esac
fi
if [ -n "$want_winget" ]; then
	case "$wg_state" in
	skip) print_plan winget "already released" "$winget_repo has $wg_dir" ;;
	pr) print_plan winget "PR already open" "$wg_pr_url" ;;
	*) print_plan winget "PR opened" "$wg_url (waiting on winget review)" ;;
	esac
fi
printf '\n'
