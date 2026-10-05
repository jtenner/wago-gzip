(module
  (import "gzip.wasm64" "abi_version" (func $abi_version (result i32)))
  (import "gzip.wasm64" "compress"
    (func $compress (param i64 i64 i64 i64 i32) (result i32 i32)))
  (import "gzip.wasm64" "decompress"
    (func $decompress (param i64 i64 i64 i64) (result i32 i32)))
  (memory i64 1)
  (data (i64.const 0) "gzip-wag")

  (func (export "version") (result i32)
    call $abi_version)

  (func (export "roundtrip") (result i32 i32 i32 i32 i32 i32)
    (local $compress_status i32)
    (local $compressed i32)
    (local $decompress_status i32)
    (local $plain i32)
    i64.const 0 i64.const 8 i64.const 1024 i64.const 1024 i32.const -1
    call $compress
    local.set $compressed
    local.set $compress_status
    i64.const 1024 local.get $compressed i64.extend_i32_u i64.const 2048 i64.const 1024
    call $decompress
    local.set $plain
    local.set $decompress_status
    local.get $compress_status
    local.get $compressed
    local.get $decompress_status
    local.get $plain
    i64.const 2048 i32.load8_u
    i64.const 2055 i32.load8_u)

  (func (export "overlap") (result i32 i32 i32 i32 i32 i32)
    (local $compress_status i32)
    (local $compressed i32)
    (local $decompress_status i32)
    (local $plain i32)
    i64.const 0 i64.const 8 i64.const 0 i64.const 1024 i32.const -1
    call $compress
    local.set $compressed
    local.set $compress_status
    i64.const 0 local.get $compressed i64.extend_i32_u i64.const 0 i64.const 1024
    call $decompress
    local.set $plain
    local.set $decompress_status
    local.get $compress_status
    local.get $compressed
    local.get $decompress_status
    local.get $plain
    i64.const 0 i32.load8_u
    i64.const 7 i32.load8_u)

  (func (export "atomic_failure") (result i32 i32 i32)
    (local $compressed i32)
    (local $status i32)
    (local $written i32)
    i64.const 0 i64.const 8 i64.const 1024 i64.const 1024 i32.const -1
    call $compress
    local.set $compressed
    drop
    i64.const 3000 i32.const 170 i32.store8
    i64.const 1024 local.get $compressed i64.extend_i32_u i64.const 3000 i64.const 1
    call $decompress
    local.set $written
    local.set $status
    local.get $status
    local.get $written
    i64.const 3000 i32.load8_u)

  (func (export "bounds_failure") (result i32 i32)
    i64.const 65535 i64.const 2 i64.const 1024 i64.const 64 i32.const -1
    call $compress)

  ;; This must not narrow the offset to 32 bits (which would alias address 0).
  (func (export "wide_bounds_failure") (result i32 i32)
    i64.const 4294967296 i64.const 8 i64.const 1024 i64.const 64 i32.const -1
    call $compress)

  (func (export "checksum_failure") (result i32 i32 i32)
    (local $compressed i32)
    (local $checksum i64)
    (local $status i32)
    (local $written i32)
    i64.const 0 i64.const 8 i64.const 1024 i64.const 1024 i32.const -1
    call $compress
    local.set $compressed
    drop
    i64.const 1024 local.get $compressed i64.extend_i32_u i64.add i64.const 8 i64.sub
    local.set $checksum
    local.get $checksum
    local.get $checksum i32.load8_u i32.const 1 i32.xor
    i32.store8
    i64.const 3000 i32.const 170 i32.store8
    i64.const 1024 local.get $compressed i64.extend_i32_u i64.const 3000 i64.const 1024
    call $decompress
    local.set $written
    local.set $status
    local.get $status
    local.get $written
    i64.const 3000 i32.load8_u))
