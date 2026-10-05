// Package main is a negative TinyGo-produced Wasm32 qualification fixture. It
// records the natural Go signature of the codec's multi-result imports. The
// dedicated CI job requires TinyGo 0.42.0 to reject these declarations with its
// explicit "too many return values" diagnostic instead of silently treating
// this fixture as executable guest coverage.
package main

import "unsafe"

var (
	source     = [8]byte{'g', 'z', 'i', 'p', '-', 'w', 'a', 'g'}
	compressed [256]byte
	plain      [256]byte
)

//go:wasmimport gzip.wasm32 abi_version
func abiVersion() int32

//go:wasmimport gzip.wasm32 compress
func gzipCompress(sourceOffset, sourceLength, destinationOffset, destinationCapacity uint32, level int32) (status, written uint32)

//go:wasmimport gzip.wasm32 decompress
func gzipDecompress(sourceOffset, sourceLength, destinationOffset, destinationCapacity uint32) (status, written uint32)

func offset(data *byte) uint32 {
	return uint32(uintptr(unsafe.Pointer(data)))
}

//go:export qualify
func qualify() uint32 {
	if abiVersion() != 1 {
		return 1
	}
	status, compressedLength := gzipCompress(
		offset(&source[0]), uint32(len(source)),
		offset(&compressed[0]), uint32(len(compressed)), -1,
	)
	if status != 0 || compressedLength == 0 {
		return 2
	}
	status, plainLength := gzipDecompress(
		offset(&compressed[0]), compressedLength,
		offset(&plain[0]), uint32(len(plain)),
	)
	if status != 0 || plainLength != uint32(len(source)) {
		return 3
	}
	for index := range source {
		if plain[index] != source[index] {
			return 4
		}
	}
	return 0
}

func main() {}
