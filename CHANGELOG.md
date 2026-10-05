# Changelog

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
