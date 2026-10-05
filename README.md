# Wago Gzip

`wago-gzip` is a bounded RFC 1952 gzip plugin for the
[Wago](https://github.com/wago-org/wago) WebAssembly runtime. It uses Go's
maintained `compress/gzip` and `compress/flate` implementations; this project
does not implement or fork the compression algorithm.

The first release is a whole-buffer API. It intentionally does not accept zlib
streams or raw DEFLATE, expose streaming handles, or support preset
dictionaries.

## Use

```go
runtime := wago.NewRuntime(
    wago.WithRuntimeConfig(
        wago.NewRuntimeConfig().WithCoreFeatures(wago.CoreFeaturesV3),
    ),
)
defer runtime.Close()

if err := runtime.LoadPlugins(context.Background(), gzip.PluginSet()); err != nil {
    return err
}
```

`gzip.Provider()` exposes the provider for hosts that assemble their own
reviewed plugin set. `gzip.PluginSet(config)` is a convenience for direct use.
The provider requests only `host.import.define`, scoped to the three modules
below.

## ABI version 1

Every module exports:

```text
abi_version() -> i32
```

and returns `(status: i32, written: i32)` from each codec operation.

### `gzip.wasm32`

```text
compress(src: i32, src_len: i32, dst: i32, dst_cap: i32, level: i32)
  -> (status: i32, written: i32)
decompress(src: i32, src_len: i32, dst: i32, dst_cap: i32)
  -> (status: i32, written: i32)
```

Offsets and lengths are unsigned `i32` values in the guest's first 32-bit
linear memory.

### `gzip.wasm64`

```text
compress(src: i64, src_len: i64, dst: i64, dst_cap: i64, level: i32)
  -> (status: i32, written: i32)
decompress(src: i64, src_len: i64, dst: i64, dst_cap: i64)
  -> (status: i32, written: i32)
```

Offsets and lengths are unsigned `i64` values in the guest's first 64-bit
linear memory. `written` remains `i32` because version 1 caps output at 64 MiB.

### `gzip.gc`

```text
compress(src: anyref, src_off: i32, src_len: i32,
         dst: anyref, dst_off: i32, dst_cap: i32, level: i32)
  -> (status: i32, written: i32)
decompress(src: anyref, src_off: i32, src_len: i32,
           dst: anyref, dst_off: i32, dst_cap: i32)
  -> (status: i32, written: i32)
```

References must select packed `array i8` objects. The source may be mutable or
immutable; the destination must be mutable. Ranges are unsigned `i32` byte
ranges.

The compression `level` uses Go/DEFLATE values `-2` through `9`: Huffman-only
is `-2`, default is `-1`, no compression is `0`, and ordinary levels are
`1..9`.

### Status values

| Value | Name | Meaning |
| ---: | --- | --- |
| 0 | `OK` | Complete output was committed. |
| 1 | `INVALID_ARGUMENT` | Invalid level, reference, range, or destination. |
| 2 | `INPUT_TOO_LARGE` | Input, metadata, or member count exceeds configured policy. |
| 3 | `OUTPUT_TOO_LARGE` | Requested capacity exceeds policy, or output exceeds the configured maximum. |
| 4 | `OUTPUT_TOO_SMALL` | The guest destination is smaller than the complete output. |
| 5 | `INVALID_DATA` | The gzip header or DEFLATE payload is invalid. |
| 6 | `TRUNCATED` | A gzip member or required trailer is incomplete. |
| 7 | `CHECKSUM_MISMATCH` | CRC32 or ISIZE verification failed. |
| 8 | `DICTIONARY_REQUIRED` | Reserved for shared codec ABI compatibility; gzip never returns it. |
| 9 | `TRAILING_DATA` | Bytes after valid members do not begin another gzip member. |
| 10 | `BUSY` | All per-runtime operation slots are active; the call did not block. |
| 11 | `UNSUPPORTED` | The chosen namespace does not match guest storage, or the plugin is stopping. |
| 12 | `INTERNAL_ERROR` | A plugin invariant or codec writer failure occurred. |

For every nonzero status, `written` is zero and the destination range is
unchanged. Input is detached inside a callback-scoped Wago storage view, codec
output is staged in bounded 32 KiB chunks, and the destination is borrowed
again only for the final commit. This makes overlapping source and destination
ranges safe and prevents retention of guest pointers, slices, or GC references.

## Gzip policy

- Compression emits exactly one deterministic member: MTIME is zero, OS is
  255, and name, comment, and extra fields are absent.
- Empty plaintext produces a valid empty member. Empty compressed input is
  `TRUNCATED`.
- Decompression accepts concatenated RFC 1952 members and concatenates their
  plaintext.
- CRC32 and ISIZE are verified by reading every member through EOF. Output is
  tentative until that verification succeeds.
- A nonmagic suffix after valid members is `TRAILING_DATA`. A suffix beginning
  with gzip magic is treated as another member: malformed or incomplete
  headers report their actual status. Once a complete header establishes a
  member beyond `max_members`, `INPUT_TOO_LARGE` takes precedence without
  decoding that extra member's payload.
- Optional metadata is scanned without allocation before `compress/gzip` sees
  it. Both cumulative metadata bytes and member count are bounded. Names and
  comments are limited to the Go reader's 512-byte NUL-terminated field size
  (511 data bytes); a longer field is `INPUT_TOO_LARGE`.

## Resource limits

| Setting | Default | Hard maximum |
| --- | ---: | ---: |
| `max_input_bytes` | 8 MiB | 64 MiB |
| `max_output_bytes` | 32 MiB | 64 MiB |
| `max_concurrent_operations` | 2 | 8 |
| `max_metadata_bytes` | 64 KiB | 1 MiB |
| `max_members` | 1,024 | 65,536 |

The gate is per activated Runtime and returns `BUSY` immediately when full.
Runtime shutdown rejects new work and waits for active calls to release their
detached input and staged output.

For Wasm32 and Wasm64 guests, detached input plus staged output is bounded near
80 MiB across two maximally sized default operations. For WasmGC, Wago may copy
an immutable source array before the plugin makes its callback-safe detached
copy, so the worst case is near 96 MiB. Selecting every hard maximum permits
roughly 1 GiB for Wasm memories or 1.5 GiB for immutable WasmGC sources across
eight calls. Each estimate excludes the Go gzip/flate codec's bounded working
state, slice headers, and small read buffers. These are independent hard
ceilings, not an aggregate memory budget; hosts should choose a smaller
combination when their process budget requires it.

## Development

```sh
go test ./...
go test -race ./...
go vet ./...
```

The checked-in Wasm fixtures let ordinary tests run without an assembler. After
editing their `.wat` sources, regenerate them with:

```sh
wasm-tools parse testdata/wasm32.wat -o testdata/wasm32.wasm
wasm-tools parse testdata/wasm64.wat -o testdata/wasm64.wasm
wasm-tools parse testdata/gc.wat -o testdata/gc.wasm
```

### TinyGo qualification

The dedicated `TinyGo host integration / Wasm32, Wasm64, WasmGC` CI job pins
TinyGo 0.42.0 with Go 1.27.1 on Linux/amd64 and applies the exact upstream task
scheduler fix used by the pinned Wago dependency. It uses Wago's required
`-scheduler=tasks` host setting and executes the complete codec suite plus real
round-trip, overlap, output-small atomicity, bounds, checksum, Wasm64-width, and
WasmGC-type checks for all three plugin namespaces. This qualifies a
**TinyGo-compiled host**; the guest modules for those tests remain the checked-in
WAT fixtures.

TinyGo-produced guests have a narrower, separately tested scope. TinyGo 0.42.0
offers Wasm32 targets only. A generated Wasm32 guest successfully imports and
executes the single-result `abi_version` function, but TinyGo rejects the
two-result `compress` and `decompress` imports with `too many return values`.
It has no Wasm64 or WasmGC output target. CI asserts each limitation explicitly,
so guest compilation is neither mistaken for host qualification nor silently
skipped. A future guest-compatible ABI extension could add single-result packed
status/written entry points without changing the current common ABI.

## License

Project code is Apache-2.0. Go standard-library attribution is recorded in
[`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md); no third-party codec source
is vendored here.
