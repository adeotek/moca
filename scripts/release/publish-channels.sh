#!/usr/bin/env bash
# Render every distribution channel for a release, and print what to do with them.
#
#   scripts/release/publish-channels.sh --tag v0.7.9
#   scripts/release/publish-channels.sh --tag v0.7.9 --out /tmp/channels --verify
#   scripts/release/publish-channels.sh --tag v0.7.9 --registry-repo ../registry --tap-repo ../homebrew-tap
#
# Three channels, three renderers, one command:
#
#   acp-registry/   moca/agent.json + moca/icon.svg      -> agentclientprotocol/registry
#   homebrew-tap/   Formula/moca.rb                      -> adeotek/homebrew-tap
#   winget/         three manifests under one directory  -> microsoft/winget-pkgs
#
# It never writes to another repository's history: with --registry-repo/--tap-repo it
# only COPIES the rendered files into an existing clone and prints the commit, push and
# PR commands. Pushing is a human decision, and for winget the PR must go through the
# contents API anyway (that repository is far too large to clone).
#
# The renderers read the release's own checksums.txt, so every URL and hash matches the
# published archives; a tag whose packages are not all published fails loudly and names
# what is missing. Publish the release BEFORE running this.
set -euo pipefail

usage() {
	cat <<'EOF'
usage: publish-channels.sh --tag vX.Y.Z [options]

  --tag <vX.Y.Z>       release tag to render (required)
  --out <dir>          output directory (default: a fresh temp dir)
  --verify             HEAD every archive URL the channels point at
  --date <YYYY-MM-DD>  winget ReleaseDate (default: today, UTC)
  --registry-repo <d>  copy the ACP entry into this clone of the registry fork
  --tap-repo <d>       copy Formula/moca.rb into this clone of the tap
  -h, --help           this text

The channel renderers themselves are scripts/release/render-{acp-registry,homebrew-formula,winget-manifests}.py.
EOF
}

tag=""
out=""
verify=""
date=""
registry_repo=""
tap_repo=""

while [ $# -gt 0 ]; do
	case "$1" in
	--tag)
		[ $# -ge 2 ] || { echo "--tag needs a value" >&2; exit 2; }
		tag="$2"
		shift 2
		;;
	--out)
		[ $# -ge 2 ] || { echo "--out needs a value" >&2; exit 2; }
		out="$2"
		shift 2
		;;
	--date)
		[ $# -ge 2 ] || { echo "--date needs a value" >&2; exit 2; }
		date="$2"
		shift 2
		;;
	--registry-repo)
		[ $# -ge 2 ] || { echo "--registry-repo needs a value" >&2; exit 2; }
		registry_repo="$2"
		shift 2
		;;
	--tap-repo)
		[ $# -ge 2 ] || { echo "--tap-repo needs a value" >&2; exit 2; }
		tap_repo="$2"
		shift 2
		;;
	--verify)
		verify="--verify"
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo "unknown argument: $1" >&2
		usage >&2
		exit 2
		;;
	esac
done

if [ -z "$tag" ]; then
	echo "--tag is required" >&2
	usage >&2
	exit 2
fi
case "$tag" in
v[0-9]*.[0-9]*.[0-9]*) ;;
*)
	echo "tag must look like vX.Y.Z, got '$tag'" >&2
	exit 2
	;;
esac

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
[ -n "$out" ] || out="$(mktemp -d "${TMPDIR:-/tmp}/moca-channels.XXXXXX")"
mkdir -p "$out"
version="${tag#v}"

failed=""
render() { # render <label> <script> [args...]
	local label="$1" script="$2"
	shift 2
	if python3 "$root/scripts/release/$script" "$@"; then
		printf '  %-12s %s\n' "$label" "rendered"
	else
		printf '  %-12s %s\n' "$label" "FAILED — see the message above" >&2
		failed="$failed $label"
	fi
}

echo "rendering the channels for $tag into $out"
extra=()
[ -n "$verify" ] && extra+=("$verify")

render "acp-registry" render-acp-registry.py --tag "$tag" --out "$out/acp-registry" "${extra[@]}"
render "homebrew" render-homebrew-formula.py --tag "$tag" --out "$out/homebrew-tap"

winget_args=(--tag "$tag" --out "$out/winget" "${extra[@]}")
[ -n "$date" ] && winget_args+=(--date "$date")
render "winget" render-winget-manifests.py "${winget_args[@]}"

if [ -n "$registry_repo" ]; then
	[ -d "$registry_repo" ] || { echo "--registry-repo: $registry_repo is not a directory" >&2; exit 1; }
	mkdir -p "$registry_repo/moca"
	cp "$out/acp-registry/moca/agent.json" "$out/acp-registry/moca/icon.svg" "$registry_repo/moca/"
	echo "copied moca/ into $registry_repo"
fi
if [ -n "$tap_repo" ]; then
	[ -d "$tap_repo" ] || { echo "--tap-repo: $tap_repo is not a directory" >&2; exit 1; }
	mkdir -p "$tap_repo/Formula"
	cp "$out/homebrew-tap/Formula/moca.rb" "$tap_repo/Formula/moca.rb"
	echo "copied Formula/moca.rb into $tap_repo"
fi

cat <<EOF

Next steps — nothing has been pushed.

ACP registry (agentclientprotocol/registry)
  git -C <a clone of your fork> checkout -b add-moca
  cp -r "$out/acp-registry/moca" <fork>/moca     # or --registry-repo <fork>
  git -C <fork> add moca && git -C <fork> commit -m "Add moca as a binary agent" && git -C <fork> push
  gh pr create --repo agentclientprotocol/registry --head ${GITHUB_USER:-adeotek}:add-moca \\
      --title "Add moca as a binary agent" --body-file <body>
  Then re-run the entry through their own tooling before pushing:
      cd <fork> && uv run --with jsonschema .github/workflows/build_registry.py
      cd .github/workflows && uv run python verify_agents.py --auth-check --agent moca

Homebrew tap (adeotek/homebrew-tap)
  git -C <tap clone> checkout -b moca-$version
  cp "$out/homebrew-tap/Formula/moca.rb" <tap>/Formula/moca.rb   # or --tap-repo <tap>
  git -C <tap> commit -am "moca $version" && git -C <tap> push
  # the tap is a personal repository: push straight to main, no PR needed
  Then, in the homebrew/brew container:
      brew tap adeotek/tap && brew style adeotek/tap/moca && brew audit --strict --online adeotek/tap/moca \\
        && brew install adeotek/tap/moca && brew test adeotek/tap/moca

winget (microsoft/winget-pkgs)
  Never clone that repository — fork it and put the files up through the API:
      gh repo fork microsoft/winget-pkgs --clone=false
      # PUT each file under manifests/a/adeotek/moca/$version/ on a branch of the fork
      gh pr create --repo microsoft/winget-pkgs --head ${GITHUB_USER:-adeotek}:adeotek.moca-$version \\
          --title "New package: adeotek.moca version $version" --body-file <body>
  A version already in the registry needs a NEW directory ($version); the old one stays.
  Only the maintainer can clear the Microsoft CLA (@microsoft-github-policy-service agree).

Files:
$(find "$out" -type f | sed 's/^/  /' | sort)
EOF

if [ -n "$failed" ]; then
	echo "incomplete:$failed" >&2
	exit 1
fi
