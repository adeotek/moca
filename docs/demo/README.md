# Demo recording

`moca-demo.gif` is recorded from `demo.tape` with [VHS](https://github.com/charmbracelet/vhs).

VHS needs three things at runtime: `ttyd`, `ffmpeg` and a Chrome/Chromium for its renderer. Two
ways to get them.

## Recipe A — the Docker image (no host changes, recommended)

The image carries all three; it ships **no git**, which is why the tape clones nothing and instead
`cd`s into a prepared `/work/repo`.

```bash
# 0. a small repo with ONE failing test, plus a scratch moca config for it
git clone <sample-repo> /tmp/moca-demo/repo
mkdir -p /tmp/moca-demo/config/moca
cat > /tmp/moca-demo/config/moca/config.jsonc <<'EOF'
{ "model": "opencode-go/glm-5.3-flash",
  "providers": { "opencode-go": { "auth": "api_key", "apiKey": "env:OPENCODE_API_KEY" } } }
EOF

# 1. record — from the repo root; the tape writes docs/demo/moca-demo.gif into /vhs
docker run --rm \
  -v "$PWD":/vhs -w /vhs \
  -v /tmp/moca-demo:/work \
  -v "$PWD/bin/moca":/usr/local/bin/moca:ro \
  -v /tmp/moca-demo/config/moca:/root/.config/moca \
  -e OPENCODE_API_KEY \
  ghcr.io/charmbracelet/vhs docs/demo/demo.tape

sudo chown "$USER" docs/demo/moca-demo.gif   # the container runs as root
```

## Recipe B — native

```bash
mise use -g ubi:charmbracelet/vhs    # the VHS binary from its GitHub release (no Go toolchain)
sudo dnf install ttyd ffmpeg chromium   # Fedora: only ttyd is in a default repo
vhs docs/demo/demo.tape                 # with moca on PATH and the fixture at /work/repo
```

`gifsicle -O3 --lossy=60 moca-demo.gif -o moca-demo.gif` keeps it under 3 MB.

## Rules for the recording

- **A throwaway API key, in a scratch config dir**, never the real `~/.config/moca/auth.json`.
  Before committing, scrub every frame: no key, no token, no `$HOME` path containing your username.
- 15–20 s, no narration: prompt → the approval picker (`o` allow once) → the diff → `shift+tab`
  for the mode badge. The picker is the shot that explains moca.
- Check it reads at 800 px wide and that the first frame is not blank.

## The committed recording

`moca-demo.gif` (1100×620, ~690 KB, 22 s) was taken with **Recipe A** against moca **v0.7.7**
running `--mode ask` on a two-file bash fixture: `sum.sh` (adds numbers, but skips negatives) and
`test.sh` (checks both cases, one fails). The tape answers the picker with `s` — *allow for this
session* — so the agent's second `./test.sh` run needs no second prompt.

Gotchas learned while recording, all encoded in `demo.tape`:

- **Paths in a tape are relative.** VHS's parser rejects `/work/out/x.png` outright
  (`Expected file path…`, then one `Invalid command` per path segment), so mount the workdir and
  use paths under it.
- **`Wait+Screen /regex/` beats fixed sleeps** for anything model-paced; a `Sleep` that guesses
  wrong either cuts a beat off or shows a stalled screen. The one place a sleep is deliberate is
  **the 1.6 s hold on the approval picker** — answered instantly, it flashes by in two frames.
- **A bare `/mode` + `Enter` opens the model picker**: `<enter>` completes a partial word and
  `/mode` is a prefix of `/model`. The tape types `/mode auto`, which also keeps the `/`-dropdown
  closed.
- **The one-time newline hint is persisted per machine** (`hints.json` in the data dir), so the
  tape opens with a hidden `moca` launch to consume it — otherwise every frame carries
  `hint: this terminal can't report shift+enter …`.
- The Docker image runs as root, so `chown "$USER" docs/demo/moca-demo.gif` afterwards.
