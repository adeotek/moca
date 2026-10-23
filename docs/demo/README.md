# Demo recording

`moca-demo.gif` is recorded from `demo.tape` with [VHS](https://github.com/charmbracelet/vhs).

```bash
uv tool install vhs            # or: go install github.com/charmbracelet/vhs@latest
# edit DEMO_REPO in demo.tape (a small public repo with one failing test)
vhs demo.tape                  # writes moca-demo.gif here
gifsicle -O3 --lossy=60 moca-demo.gif -o moca-demo.gif   # keep it under 3 MB
```

Rules for the recording:

- **A throwaway API key, in a scratch config dir** (`XDG_CONFIG_HOME=$(mktemp -d)`), never the
  real `~/.config/moca/auth.json`. Before committing, scrub every frame: no key, no token, no
  `$HOME` path containing your username.
- 15–20 s, no narration: prompt → the approval picker (`o` allow once) → the diff → `shift+tab`
  for the mode badge. The picker is the shot that explains moca.
- Check it reads at 800 px wide and that the first frame is not blank.

The README links this file once it exists (`![moca demo](docs/demo/moca-demo.gif)`); do not add
the image line before the GIF is committed.
