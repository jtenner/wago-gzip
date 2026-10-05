# Changelog

## v0.0.1 — 2026-10-05

- Add single-result `compress_packed` and `decompress_packed` ABI v1 adapters
  for TinyGo-produced Wasm32 guests while retaining the legacy multi-result
  functions.
- Qualify a real TinyGo 0.42.0 guest in Go and TinyGo hosts.
- Add dedicated pinned TinyGo host CI that executes the codec and Wasm32,
  Wasm64, and WasmGC integrations, including checksum and bounds failures.
- Record the remaining TinyGo 0.42.0 guest-generation limits explicitly:
  Wasm32 only, with legacy multi-result imports still rejected.

## v0.0.0

Experimental publication-test release.

- Add bounded RFC 1952 compression and decompression using Go's standard
  library.
- Add equivalent WasmGC byte-array, Wasm32, and Wasm64 ABIs.
- Add transactional overlap-safe output, checksum/ISIZE verification,
  concatenated-member handling, explicit trailing-data policy, metadata/member
  limits, and a nonblocking per-runtime concurrency gate.
- Add native codec tests and real Wago integration fixtures for success,
  overlap, atomic failure, bounds, GC type validation, and Wasm64 range width.

This is not a production-stability declaration.
