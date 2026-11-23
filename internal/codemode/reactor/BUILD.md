# The reactor: one QuickJS-NG WASM module, built once, committed

moca runs model-authored JavaScript in a **reactor**: QuickJS-NG plus a small C
host, compiled to `wasm32-wasi`, embedded in the Go binary and executed by
wazero. Nothing is downloaded at runtime, and no host interpreter, container or
shell is involved on any platform.

This directory is the artifact and its recipe. The ABI it must satisfy is data:
`../abi.json`, enforced by `codemode.VerifyModule` and by
`TestReactorIsEmbedded`.

## What "reactor" means here

A WASI **reactor** is initialized, never started: the module exports
`_initialize` and must not export `_start`. The engine calls `qjs_init` once per
guest instance and `qjs_run` per script; `qjs_destroy` frees the runtime.

| export | shape | meaning |
|---|---|---|
| `memory` | linear memory | the guest's only memory |
| `_initialize` | `() -> ()` | WASI reactor initialization |
| `qjs_init(heap, stack) -> i32` | bytes | `0` ok, `-1` fail |
| `qjs_run(src_ptr, src_len, out_ptr, out_cap, out_len_ptr) -> i32` | status | see below |
| `qjs_destroy() -> ()` | — | free the JS runtime |
| `moca_alloc(n) -> ptr` | bytes | guest-side allocation the host may write through |
| `moca_free(ptr) -> ()` | — | release it |

`qjs_run` statuses: `0` ok, `1` syntax, `2` throw, `3` out of memory, `4` job
limit, `5` output too large, `6` internal. `out_len` is always written.

The single import is `moca.invoke(req_ptr, req_len, res_ptr, res_cap) -> i32`,
returning the reply length, `-1` when it does not fit or `-2` on a host error.
`abi.json` lists the WASI imports that may appear; `path_*`, `fd_prestat_*`,
`sock_*` and `poll_oneoff` are **forbidden** — the guest has no filesystem and
no sockets, and the bridge is the only authority. A compiler that pulls one in
fails `VerifyModule` instead of shipping.

## Build

```sh
./build.sh          # uses the pins below
./build.sh --check  # rebuild and compare against the committed artifact
```

Both pins are now resolved and live in `build.sh`:

- **wasi-sdk 29.0**, asset `wasi-sdk-29.0-x86_64-linux.tar.gz` under the
  `wasi-sdk-29` release tag (the plan wrote `wasi-sdk-29.0`, which 404s — the
  tag has no minor). 119,441,678 bytes,
  sha256 `87d1d1a2879d139cdc624b968efad3d4a97b8078cdff95e63ac88ecafd1a0171` —
  both match the plan's truncated pin, verified 2026-11-23.
- **quickjs-ng v0.9.0**, commit `670492dd342dace0bb7bd6fbfbde8f0bc5651224`.

`build.sh` then:

1. downloads and verifies the SDK (size **and** sha256),
2. checks out `QJS_REF` and records the resolved commit,
3. compiles the reactor **twice** and refuses to continue unless the two builds
   are byte-identical (`cmp`),
4. with `--check`, compares a clean build against the committed artifact.

Outputs: `qjs-moca-reactor.wasm` and `LICENSE.quickjs-ng` (copied from the
checkout — the license text is never retyped by hand).

The build host is **x86_64 Linux only**, matching the pinned SDK. The artifact
it produces is platform-independent, so macOS and Windows users run the same
committed module.

## Build notes (learned from the first real build)

- **`--allow-undefined` must not be passed.** It makes wasm-ld satisfy every
  undefined symbol as an import, so libc is never pulled in and the whole import
  section lands under a synthetic module named after the output file. Drop it
  and let `-lc` resolve the WASI stubs.
- **`-mexec-model=reactor`** (not `--no-entry`) is what produces `_initialize`
  and no `_start`.
- **Do not link `quickjs-libc.c`.** It brings the `os`/`std` helpers, whose WASI
  file APIs would add exactly the imports `abi.json` forbids.
- **`--export=memory` fails** ("symbol exported via --export not found"):
  memory is defined by wasm-ld, not a symbol. Export it with
  `-Wl,--export-memory`-style options only if a later SDK needs it.

## The import-name question, resolved

The first artifact looked like it imported everything from a module named after
the output file. It did not: **wasm-ld was right and the gate was wrong.** The
ABI check called `CompiledModule.ImportedFunctions()`, and wazero reports the
NAME SECTION's module name for an imported function — which wasm-ld sets to the
output file's basename. Reading the import section directly shows the truth:

```
moca.invoke
wasi_snapshot_preview1.clock_time_get, fd_close, fd_fdstat_get, fd_seek, fd_write
```

`VerifyModule` now parses section 2 itself (`imports.go`), so the gate reads the
bytes and neither falsely rejects nor falsely accepts.

Two related traps, both fixed in `build.sh`:

- the artifact's bytes depend on the output **basename** (it goes into the name
  section), so two builds must use the same basename in different directories;
- even stripped, quickjs embeds its source paths through `__FILE__`, so builds
  in different directories differ by a few dozen bytes without
  `-ffile-prefix-map`/`-fmacro-prefix-map`.

With `-g0`, `-Wl,--strip-debug` and the prefix maps, two clean builds are
byte-identical: **990,613 bytes**, which `build.sh --check` verifies.

## Wiring the artifact in

`internal/codemode/reactorBytes` is nil until the artifact exists. To wire it:

```go
//go:embed reactor/qjs-moca-reactor.wasm
var reactorBytes []byte
```

`TestReactorIsEmbedded` skips while the file is absent and **fails** once it
exists but is not embedded, so the gap cannot be forgotten. The same test runs
`VerifyModule`, which is the export/import gate above.

## Guest limits

| limit | value | where |
|---|---|---|
| linear memory | 32 MiB (512 wazero pages) | `abi.json` + the runtime config |
| JS heap | 12 MiB | `qjs_init` |
| linear stack | 2 MiB | `build.sh` (`-Wl,-z,stack-size=2097152` when the SDK needs it) |
| JS stack | 1 MiB | `qjs_init` |
| request frame | 64 KiB | prelude + `hostInvoke` |
| response frame | 320 KiB | prelude + `hostInvoke` |
| scripts in flight | 1 per process | the engine slot (`busy`) |

The **measured-fit gate** is part of Task 2: after the first real build, run the
fixture scenarios at the limits above and confirm the JS heap and linear memory
actually fit the workloads the plan describes. A reactor that only works at
2× these numbers does not ship.
