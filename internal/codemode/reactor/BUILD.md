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
WASI_SDK_SHA256=<full sha256 of wasi-sdk-29.0-x86_64-linux.tar.gz> \
QJS_REF=<quickjs-ng commit> ./build.sh
```

Two inputs are required rather than defaulted, on purpose:

- **The SDK digest.** The plan pins `wasi-sdk-29.0-x86_64-linux.tar.gz`,
  119,441,678 bytes (macOS: 113,028,367). The plan records the digest only
  truncated (`87d1d1a2…a0171`), and a truncated digest cannot be verified, so
  `build.sh` refuses to unpack a toolchain it cannot check. Supply the full
  value; it belongs in this file once confirmed.
- **The QuickJS-NG revision.** The plan does not pin one yet — an open decision.
  Pin a commit, not a branch: the artifact is committed, so the build has to be
  reproducible from source.

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
