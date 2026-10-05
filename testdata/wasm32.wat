(module
  (import "gzip.wasm32" "abi_version" (func $abi_version (result i32)))
  (import "gzip.wasm32" "compress"
    (func $compress (param i32 i32 i32 i32 i32) (result i32 i32)))
  (import "gzip.wasm32" "decompress"
    (func $decompress (param i32 i32 i32 i32) (result i32 i32)))
  (import "gzip.wasm32" "compress_packed"
    (func $compress_packed (param i32 i32 i32 i32 i32) (result i64)))
  (import "gzip.wasm32" "decompress_packed"
    (func $decompress_packed (param i32 i32 i32 i32) (result i64)))
  (memory 1)
  (data (i32.const 0) "gzip-wag")

  (func (export "version") (result i32)
    call $abi_version)

  (func (export "roundtrip") (result i32 i32 i32 i32 i32 i32)
    (local $compress_status i32)
    (local $compressed i32)
    (local $decompress_status i32)
    (local $plain i32)
    i32.const 0 i32.const 8 i32.const 1024 i32.const 1024 i32.const -1
    call $compress
    local.set $compressed
    local.set $compress_status
    i32.const 1024 local.get $compressed i32.const 2048 i32.const 1024
    call $decompress
    local.set $plain
    local.set $decompress_status
    local.get $compress_status
    local.get $compressed
    local.get $decompress_status
    local.get $plain
    i32.const 2048 i32.load8_u
    i32.const 2055 i32.load8_u)

  ;; Both commits overwrite their own source range. Staging must make this safe.
  (func (export "overlap") (result i32 i32 i32 i32 i32 i32)
    (local $compress_status i32)
    (local $compressed i32)
    (local $decompress_status i32)
    (local $plain i32)
    i32.const 0 i32.const 8 i32.const 0 i32.const 1024 i32.const -1
    call $compress
    local.set $compressed
    local.set $compress_status
    i32.const 0 local.get $compressed i32.const 0 i32.const 1024
    call $decompress
    local.set $plain
    local.set $decompress_status
    local.get $compress_status
    local.get $compressed
    local.get $decompress_status
    local.get $plain
    i32.const 0 i32.load8_u
    i32.const 7 i32.load8_u)

  (func (export "atomic_failure") (result i32 i32 i32)
    (local $compressed i32)
    (local $status i32)
    (local $written i32)
    i32.const 0 i32.const 8 i32.const 1024 i32.const 1024 i32.const -1
    call $compress
    local.set $compressed
    drop
    i32.const 3000 i32.const 170 i32.store8
    i32.const 1024 local.get $compressed i32.const 3000 i32.const 1
    call $decompress
    local.set $written
    local.set $status
    local.get $status
    local.get $written
    i32.const 3000 i32.load8_u)

  (func (export "bounds_failure") (result i32 i32)
    i32.const 65535 i32.const 2 i32.const 1024 i32.const 64 i32.const -1
    call $compress)

  (func (export "checksum_failure") (result i32 i32 i32)
    (local $compressed i32)
    (local $checksum i32)
    (local $status i32)
    (local $written i32)
    i32.const 0 i32.const 8 i32.const 1024 i32.const 1024 i32.const -1
    call $compress
    local.set $compressed
    drop
    i32.const 1024 local.get $compressed i32.add i32.const 8 i32.sub
    local.set $checksum
    local.get $checksum
    local.get $checksum i32.load8_u i32.const 1 i32.xor
    i32.store8
    i32.const 3000 i32.const 170 i32.store8
    i32.const 1024 local.get $compressed i32.const 3000 i32.const 1024
    call $decompress
    local.set $written
    local.set $status
    local.get $status
    local.get $written
    i32.const 3000 i32.load8_u)

  (func (export "packed_parity") (result i32)
    (local $status i32)
    (local $written i32)
    (local $packed i64)
    (local $ok i32)
    i32.const 0 i32.const 8 i32.const 1024 i32.const 1024 i32.const -1
    call $compress
    local.set $written
    local.set $status
    i32.const 0 i32.const 8 i32.const 2048 i32.const 1024 i32.const -1
    call $compress_packed
    local.set $packed
    local.get $status local.get $packed i32.wrap_i64 i32.eq
    local.get $written local.get $packed i64.const 32 i64.shr_u i32.wrap_i64 i32.eq
    i32.and
    local.get $status i32.eqz i32.and
    local.get $written i32.const 0 i32.gt_u i32.and
    local.set $ok
    i32.const 1024 local.get $written i32.const 3072 i32.const 1024
    call $decompress
    local.set $written
    local.set $status
    i32.const 2048 local.get $packed i64.const 32 i64.shr_u i32.wrap_i64
    i32.const 4096 i32.const 1024
    call $decompress_packed
    local.set $packed
    local.get $ok
    local.get $status local.get $packed i32.wrap_i64 i32.eq
    i32.and
    local.get $written local.get $packed i64.const 32 i64.shr_u i32.wrap_i64 i32.eq
    i32.and
    local.get $status i32.eqz i32.and
    local.get $written i32.const 8 i32.eq i32.and
    i32.const 3072 i32.load8_u i32.const 4096 i32.load8_u i32.eq
    i32.and))
