# moca TUI demo — record with VHS: `vhs docs/demo/demo.tape`
#
# Before recording:
#   - put a small repo with ONE failing test at ~/repo (a fixture of your own is
#     better than someone else's). The tape clones nothing: the VHS Docker image
#     ships no git. Recording recipes (Docker and native): README.md here.
#   - use a throwaway API key in a scratch config dir, never your real
#     ~/.config/moca/auth.json — no frame may show a credential.
#   - `moca` must be on PATH (bin/moca, or an installed release).
# After recording: keep the GIF under 3 MB
#   (gifsicle -O3 --lossy=60 docs/demo/moca-demo.gif -o docs/demo/moca-demo.gif).
#
# The tape records `--mode ask` on purpose: auto mode runs a confined script
# without asking, and the approval picker is the shot that explains moca. No
# `Set Theme` either — the defaults are the palette a user actually gets.
#
# Beats: prompt → tool call → the picker (`s` = allow for this session) → the
# diff → a green test run → `/mode` flipping the badge.

Output docs/demo/moca-demo.gif

Require moca

Set Shell bash
Set FontSize 16
Set Width 1100
Set Height 620
Set Padding 12
Set Framerate 20
Set TypingSpeed 40ms
Set CursorBlink false

# One hidden launch marks the terminal's one-time newline hint as seen, so the
# recording opens on the greeting instead of a "this terminal can't report
# shift+enter" line.
Hide
Type "cd ~/repo && clear"
Enter
Sleep 500ms
Type "moca --mode ask"
Enter
Wait+Screen /How can I help/
Sleep 500ms
Ctrl+C
Ctrl+C
Sleep 1s
Type "clear"
Enter
Show

Type "moca --mode ask"
Enter
Wait+Screen /How can I help/
Sleep 600ms

Type "run ./test.sh and fix the failing check"
Enter

# The approval picker: hold it on screen for a beat before answering.
Wait+Screen /Allow once/
Sleep 1600ms
Type "s"

# The fix lands, the test goes green, the answer is read.
Wait+Screen /exit 0/
Sleep 1200ms
Sleep 2500ms

# The mode badge (ask → auto). `/mode auto` rather than a bare `/mode`: `<enter>`
# on a partial word COMPLETES it, and `/mode` is a prefix of `/model`, so a bare
# `/mode` + Enter opens the model picker. The argument also keeps the dropdown
# closed. (VHS cannot send a shifted Tab, so cycling is not an option here.)
Type "/mode auto"
Enter
Sleep 1600ms

Ctrl+C
Ctrl+C
Sleep 500ms
