# PowerShell AST payloads — provenance

Fixtures for `internal/permissions/psparse.go` (Layer D). Each `ast-<case>.json` is the
exact stdout of `wrapper.ps1` for one script, and nothing here is hand-written: the
point of Layer D is to stop guessing.

## Engines

Captured twice during the Task 13 spike (plan: `docs/plans/2026-11-21-windows-permissions-parser.md`):

- **`powershell.exe` 5.1.26100** — Windows 11 Pro, build 26300, Defender realtime on.
  Seven cases recorded: `one_line`, `forty_lines`, `two_hundred`, `params`, `provider`,
  `destructive`, `broken`.
- **`pwsh` 7** — same host, and Linux `pwsh` 7 for the rest.

The seven shared cases are **byte-identical** between the two engines; the only wire
difference is 5.1 escaping `'` as `\u0027`, which decodes to the same string. So the
committed payloads are engine-neutral, and the decoder needs no per-engine branch.
The remaining cases were recorded on `pwsh` 7 only; a `/u0027`-only difference is the
expected failure mode if one is ever diffed against 5.1.

## Contract

See the `OUTCOME` block in the plan. In one line: `e` errors, `dyn` any scriptblock/
subexpression/array/variable in the script, `w` file-redirect targets, `c` commands
with a per-command `k` ("every element is a plain parameter or a plain constant") and
`a` elements of `{p,v}`, `{v,q,n}` or `{d}`.

## Regenerating

`wrapper.ps1` is the source of truth — it is `//go:embed`ed into the binary, so a change
here must be mirrored there. On any host with PowerShell:

```powershell
$b64 = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes('Get-ChildItem > out.txt'))
$enc = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes((Get-Content -Raw .\wrapper.ps1)))
$b64 | pwsh -NoProfile -NonInteractive -EncodedCommand $enc
```

## Cases worth reading

| fixture | what it pins |
|---|---|
| `params` | a parameter followed by its own value, and the `-Path`/`-Destination` role split |
| `redirect_out`, `redirect_append`, `redirect_two` | `w`, and `a` for `>>` — the bug that made every redirect write look like a read |
| `redirect_merge` | `2>&1` is not a file write |
| `redirect_null` | `> $null` yields `dyn:true` with a `$null` target (the caller asks) |
| `variable`, `expandable`, `subexpression`, `array_expr` | `{d}` elements and `k:false` |
| `array_inline` | `a.txt,b.txt` expands to two exact values |
| `colon_value` | `-Path:C:\Windows\win.ini` carries its value on the parameter |
| `dynamic_name` | `& $cmd` arrives as `n:"$cmd", k:false` — never a command named `$cmd` |
| `call_operator`, `dot_source` | a script path as the command name: the table cannot know it |
| `broken` | non-empty `e` with partially-parsed commands — the caller must fall back |
| `empty`, `comment_only` | zero commands: nothing to judge, nothing to ask |
