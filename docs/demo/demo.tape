# moca TUI demo — record with VHS: `vhs docs/demo/demo.tape`
#
# Before recording:
#   - fill in DEMO_REPO below with a small public repo that has ONE failing test
#     (a fixture repo of your own is better than someone else's);
#   - use a throwaway API key in a scratch config dir, never your real
#     ~/.config/moca/auth.json — the tape must not show a credential;
#   - `moca` must be on PATH (bin/moca, or an installed release).
# After recording: keep the GIF under 3 MB
#   (gifsicle -O3 --lossy=60 docs/demo/moca-demo.gif -o docs/demo/moca-demo.gif).
#
# The money shot is the approval picker at ~00:08: `o` = allow once,
# `s` = allow for this session, `a` = allow always (the only answer that
# saves), `d` = deny. Then the diff, then shift+tab for the mode badge.

Output docs/demo/moca-demo.gif

Set Shell "bash"
Set FontSize 16
Set Width 1100
Set Height 620
Set Padding 12
Set Framerate 30
Set TypingSpeed 55ms
Set Theme { "background": "#101418", "foreground": "#d6deeb", "cursor": "#7fdbca", "selection": "#2c3a41" }

Hide
Type "cd $(mktemp -d) && git clone -q DEMO_REPO repo && cd repo"
Enter
Show
Sleep 2s

Type "moca"
Enter
Sleep 2s

Type "fix the failing test in internal/calc"
Enter
Sleep 3s
# the run proposes a command; the approval picker opens
Sleep 4s
Type "o"
Sleep 3s
# the edit lands and renders as a diff
Sleep 4s
Shift+Tab
Sleep 2s
Ctrl+C
Ctrl+C
