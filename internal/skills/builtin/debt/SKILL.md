---
name: debt
description: Harvest `shortcut:` and TODO/FIXME deferrals into a debt ledger, so a deferred limit cannot rot into "later means never".
argument-hint: "[marker|word]  the deferral word to look for (default: shortcut, then TODO/FIXME/HACK)"
---
<!-- built into moca: this copy follows the binary and an edit here does not survive an update. `moca skills fork debt` copies it into your own skills dir, where the copy shadows it. -->

Report only. Change no code, write no file, run nothing that mutates.

1. Harvest. Search the whole tree for `shortcut:` **and** the deferral words every codebase already carries — `TODO`, `FIXME`, `HACK`, `XXX`, `for now`, `temporarily`, `later`. The bare marker is a low-frequency signal (measured: 1 of 42 runs emitted one, and that one named no trigger), so a ledger that only reads it would mostly report an empty repo while real deferrals sat three lines away. Use the search tool, not a shell grep, so the hits are windowed and tracked. Skip hits that are not deferrals — a keyboard-shortcut note, a doc quoting the syntax, a TODO in vendored code. The user may name one word; then harvest only that.
2. Judge each hit. Read the lines around it. A marker names a limit and, ideally, when to revisit it. A marker with no trigger is the one that rots: tag it `no-trigger`. Say which word matched, because `TODO fix the typo` and `shortcut: no retry on 5xx` are not the same debt — the first is a note, the second is a decision. A `TODO` that names no limit is a note: list it under a final `notes` group, not in the ledger proper.
3. Ledger, one row per marker, grouped by file:

       <file>:<line> — <what was simplified>
         ceiling: <the limit the comment names>
         upgrade: <the trigger to revisit it>   |   no-trigger

4. Close with the counts: `N markers, M without a trigger.` When there are none, print exactly `No shortcut debt. Clean ledger.` and stop. When there is a `notes` group, `N deferrals (+K notes).`

Never invent a trigger for a marker that names none, and never edit a comment to add one. If a marker's limit has clearly already been exceeded (the code around it contradicts it), say so in one line under that row — that is a finding, not a fix.
