(module
  (import "gzip.gc" "abi_version" (func $abi_version (result i32)))
  (import "gzip.gc" "compress"
    (func $compress (param anyref i32 i32 anyref i32 i32 i32) (result i32 i32)))
  (import "gzip.gc" "decompress"
    (func $decompress (param anyref i32 i32 anyref i32 i32) (result i32 i32)))
  (import "gzip.gc" "compress_packed"
    (func $compress_packed (param anyref i32 i32 anyref i32 i32 i32) (result i64)))
  (import "gzip.gc" "decompress_packed"
    (func $decompress_packed (param anyref i32 i32 anyref i32 i32) (result i64)))
  (type $bytes (array (mut i8)))
  (type $words (array (mut i32)))

  (func $new_source (result (ref $bytes))
    (local $source (ref $bytes))
    i32.const 1024 array.new_default $bytes
    local.set $source
    local.get $source i32.const 0 i32.const 103 array.set $bytes
    local.get $source i32.const 1 i32.const 122 array.set $bytes
    local.get $source i32.const 2 i32.const 105 array.set $bytes
    local.get $source i32.const 3 i32.const 112 array.set $bytes
    local.get $source i32.const 4 i32.const 45 array.set $bytes
    local.get $source i32.const 5 i32.const 119 array.set $bytes
    local.get $source i32.const 6 i32.const 97 array.set $bytes
    local.get $source i32.const 7 i32.const 103 array.set $bytes
    local.get $source)

  (func (export "version") (result i32)
    call $abi_version)

  (func (export "roundtrip") (result i32 i32 i32 i32 i32 i32)
    (local $source (ref $bytes))
    (local $compressed_array (ref $bytes))
    (local $plain_array (ref $bytes))
    (local $compress_status i32)
    (local $compressed i32)
    (local $decompress_status i32)
    (local $plain i32)
    call $new_source local.set $source
    i32.const 1024 array.new_default $bytes
    local.set $compressed_array
    i32.const 1024 array.new_default $bytes
    local.set $plain_array
    local.get $source i32.const 0 i32.const 8
    local.get $compressed_array i32.const 0 i32.const 1024 i32.const -1
    call $compress
    local.set $compressed
    local.set $compress_status
    local.get $compressed_array i32.const 0 local.get $compressed
    local.get $plain_array i32.const 0 i32.const 1024
    call $decompress
    local.set $plain
    local.set $decompress_status
    local.get $compress_status
    local.get $compressed
    local.get $decompress_status
    local.get $plain
    local.get $plain_array i32.const 0 array.get_u $bytes
    local.get $plain_array i32.const 7 array.get_u $bytes)

  (func (export "overlap") (result i32 i32 i32 i32 i32 i32)
    (local $array (ref $bytes))
    (local $compress_status i32)
    (local $compressed i32)
    (local $decompress_status i32)
    (local $plain i32)
    call $new_source local.set $array
    local.get $array i32.const 0 i32.const 8
    local.get $array i32.const 0 i32.const 1024 i32.const -1
    call $compress
    local.set $compressed
    local.set $compress_status
    local.get $array i32.const 0 local.get $compressed
    local.get $array i32.const 0 i32.const 1024
    call $decompress
    local.set $plain
    local.set $decompress_status
    local.get $compress_status
    local.get $compressed
    local.get $decompress_status
    local.get $plain
    local.get $array i32.const 0 array.get_u $bytes
    local.get $array i32.const 7 array.get_u $bytes)

  (func (export "atomic_failure") (result i32 i32 i32)
    (local $source (ref $bytes))
    (local $compressed_array (ref $bytes))
    (local $plain_array (ref $bytes))
    (local $compressed i32)
    (local $status i32)
    (local $written i32)
    call $new_source local.set $source
    i32.const 1024 array.new_default $bytes local.set $compressed_array
    i32.const 8 array.new_default $bytes local.set $plain_array
    local.get $plain_array i32.const 0 i32.const 170 array.set $bytes
    local.get $source i32.const 0 i32.const 8
    local.get $compressed_array i32.const 0 i32.const 1024 i32.const -1
    call $compress
    local.set $compressed
    drop
    local.get $compressed_array i32.const 0 local.get $compressed
    local.get $plain_array i32.const 0 i32.const 1
    call $decompress
    local.set $written
    local.set $status
    local.get $status
    local.get $written
    local.get $plain_array i32.const 0 array.get_u $bytes)

  (func (export "bounds_failure") (result i32 i32)
    (local $source (ref $bytes))
    (local $destination (ref $bytes))
    call $new_source local.set $source
    i32.const 64 array.new_default $bytes local.set $destination
    local.get $source i32.const 1023 i32.const 2
    local.get $destination i32.const 0 i32.const 64 i32.const -1
    call $compress)

  (func (export "type_failure") (result i32 i32)
    (local $source (ref $words))
    (local $destination (ref $bytes))
    i32.const 8 array.new_default $words local.set $source
    i32.const 64 array.new_default $bytes local.set $destination
    local.get $source i32.const 0 i32.const 8
    local.get $destination i32.const 0 i32.const 64 i32.const -1
    call $compress)

  (func (export "checksum_failure") (result i32 i32 i32)
    (local $source (ref $bytes))
    (local $compressed_array (ref $bytes))
    (local $plain_array (ref $bytes))
    (local $compressed i32)
    (local $checksum i32)
    (local $status i32)
    (local $written i32)
    call $new_source local.set $source
    i32.const 1024 array.new_default $bytes local.set $compressed_array
    i32.const 1024 array.new_default $bytes local.set $plain_array
    local.get $plain_array i32.const 0 i32.const 170 array.set $bytes
    local.get $source i32.const 0 i32.const 8
    local.get $compressed_array i32.const 0 i32.const 1024 i32.const -1
    call $compress
    local.set $compressed
    drop
    local.get $compressed i32.const 8 i32.sub local.set $checksum
    local.get $compressed_array
    local.get $checksum
    local.get $compressed_array local.get $checksum array.get_u $bytes
    i32.const 1 i32.xor
    array.set $bytes
    local.get $compressed_array i32.const 0 local.get $compressed
    local.get $plain_array i32.const 0 i32.const 1024
    call $decompress
    local.set $written
    local.set $status
    local.get $status
    local.get $written
    local.get $plain_array i32.const 0 array.get_u $bytes)

  (func (export "packed_parity") (result i32)
    (local $source (ref $bytes))
    (local $legacy_compressed (ref $bytes))
    (local $packed_compressed (ref $bytes))
    (local $legacy_plain (ref $bytes))
    (local $packed_plain (ref $bytes))
    (local $status i32)
    (local $written i32)
    (local $packed i64)
    (local $ok i32)
    call $new_source local.set $source
    i32.const 1024 array.new_default $bytes local.set $legacy_compressed
    i32.const 1024 array.new_default $bytes local.set $packed_compressed
    i32.const 1024 array.new_default $bytes local.set $legacy_plain
    i32.const 1024 array.new_default $bytes local.set $packed_plain
    local.get $source i32.const 0 i32.const 8
    local.get $legacy_compressed i32.const 0 i32.const 1024 i32.const -1
    call $compress
    local.set $written
    local.set $status
    local.get $source i32.const 0 i32.const 8
    local.get $packed_compressed i32.const 0 i32.const 1024 i32.const -1
    call $compress_packed
    local.set $packed
    local.get $status local.get $packed i32.wrap_i64 i32.eq
    local.get $written local.get $packed i64.const 32 i64.shr_u i32.wrap_i64 i32.eq
    i32.and
    local.get $status i32.eqz i32.and
    local.get $written i32.const 0 i32.gt_u i32.and
    local.set $ok
    local.get $legacy_compressed i32.const 0 local.get $written
    local.get $legacy_plain i32.const 0 i32.const 1024
    call $decompress
    local.set $written
    local.set $status
    local.get $packed_compressed i32.const 0
    local.get $packed i64.const 32 i64.shr_u i32.wrap_i64
    local.get $packed_plain i32.const 0 i32.const 1024
    call $decompress_packed
    local.set $packed
    local.get $ok
    local.get $status local.get $packed i32.wrap_i64 i32.eq
    i32.and
    local.get $written local.get $packed i64.const 32 i64.shr_u i32.wrap_i64 i32.eq
    i32.and
    local.get $status i32.eqz i32.and
    local.get $written i32.const 8 i32.eq i32.and
    local.get $legacy_plain i32.const 0 array.get_u $bytes
    local.get $packed_plain i32.const 0 array.get_u $bytes i32.eq
    i32.and))
