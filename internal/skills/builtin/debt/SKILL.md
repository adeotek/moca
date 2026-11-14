---
name: debt
description: Harvest every `shortcut:` comment into a debt ledger, so a deferred limit cannot rot into "later means never".
argument-hint: "[marker|word]  the deferral word to look for (default: shortcut)"
---
<!-- built into moca: this copy follows the binary and an edit here does not survive an update. `moca skills fork debt` copies it into your own skills dir, where the copy shadows it. -->

Report only. Change no code, write no file, run nothing that mutates.

1. Harvest. Search the whole tree for `shortcut:` (the user may name another word — any deferral marker of the same shape counts). Use the search tool, not a shell grep, so the hits are windowed and tracked. Skip hits that are not deferrals — a keyboard-shortcut note, a doc quoting the syntax.
2. Judge each hit. Read the lines around it. A marker names a limit and, ideally, when to revisit it. A marker with no trigger is the one that rots: tag it `no-trigger`.
3. Ledger, one row per marker, grouped by file:

       <file>:<line> — <what was simplified>
         ceiling: <the limit the comment names>
         upgrade: <the trigger to revisit it>   |   no-trigger

4. Close with the counts: `N markers, M without a trigger.` When there are none, print exactly `No shortcut debt. Clean ledger.` and stop.

Never invent a trigger for a marker that names none, and never edit a comment to add one. If a marker's limit has clearly already been exceeded (the code around it contradicts it), say so in one line under that row — that is a finding, not a fix.
