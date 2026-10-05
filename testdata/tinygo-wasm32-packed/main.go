package main

import "unsafe"

//go:wasmimport gzip.wasm32 abi_version
func abiVersion() int32

//go:wasmimport gzip.wasm32 compress_packed
func gzipCompressPacked(sourceOffset, sourceLength, destinationOffset, destinationCapacity uint32, level int32) uint64

//go:wasmimport gzip.wasm32 decompress_packed
func gzipDecompressPacked(sourceOffset, sourceLength, destinationOffset, destinationCapacity uint32) uint64

var (
	source     [64]byte
	compressed [2048]byte
	output     [128]byte
	overlap    [2048]byte
)

var payload = [...]byte{'t', 'i', 'n', 'y', 'g', 'o', '-', 'p', 'a', 'c', 'k', 'e', 'd'}

func offset(data *byte) uint32 {
	return uint32(uintptr(unsafe.Pointer(data)))
}

func result(raw uint64) (status, written uint32) {
	return uint32(raw), uint32(raw >> 32)
}

func fill(data []byte, value byte) {
	for index := range data {
		data[index] = value
	}
}

func putPayload(data []byte) {
	fill(data, 0)
	copy(data, payload[:])
}

func setupCompressed() (uint32, bool) {
	putPayload(source[:])
	fill(compressed[:], 0)
	status, written := result(gzipCompressPacked(
		offset(&source[0]), uint32(len(payload)),
		offset(&compressed[0]), uint32(len(compressed)), -1,
	))
	return written, status == 0 && written != 0
}

func unchanged(data []byte, value byte) bool {
	for _, got := range data {
		if got != value {
			return false
		}
	}
	return true
}

//go:wasmexport roundtrip
func roundtrip() uint32 {
	if abiVersion() != 1 {
		return 0
	}
	written, ok := setupCompressed()
	if !ok {
		return 0
	}
	fill(output[:], 0)
	status, plain := result(gzipDecompressPacked(
		offset(&compressed[0]), written,
		offset(&output[0]), uint32(len(output)),
	))
	if status != 0 || plain != uint32(len(payload)) {
		return 0
	}
	for index, want := range payload {
		if output[index] != want {
			return 0
		}
	}
	return 1
}

//go:wasmexport empty
func empty() uint32 {
	fill(compressed[:], 0)
	status, written := result(gzipCompressPacked(0, 0, offset(&compressed[0]), uint32(len(compressed)), -1))
	if status != 0 || written == 0 {
		return 0
	}
	fill(output[:], 0xaa)
	status, plain := result(gzipDecompressPacked(offset(&compressed[0]), written, offset(&output[0]), uint32(len(output))))
	if status != 0 || plain != 0 || output[0] != 0xaa {
		return 0
	}
	return 1
}

//go:wasmexport overlap_case
func overlapCase() uint32 {
	putPayload(overlap[:])
	status, written := result(gzipCompressPacked(
		offset(&overlap[0]), uint32(len(payload)),
		offset(&overlap[1]), uint32(len(overlap)-1), -1,
	))
	if status != 0 || written == 0 {
		return 0
	}
	status, plain := result(gzipDecompressPacked(
		offset(&overlap[1]), written,
		offset(&overlap[0]), uint32(len(overlap)),
	))
	if status != 0 || plain != uint32(len(payload)) {
		return 0
	}
	for index, want := range payload {
		if overlap[index] != want {
			return 0
		}
	}
	return 1
}

//go:wasmexport output_small
func outputSmall() uint32 {
	written, ok := setupCompressed()
	if !ok {
		return 0
	}
	fill(output[:], 0xaa)
	status, plain := result(gzipDecompressPacked(offset(&compressed[0]), written, offset(&output[0]), 1))
	if status != 4 || plain != 0 || !unchanged(output[:16], 0xaa) {
		return 0
	}
	return 1
}

//go:wasmexport bounds
func bounds() uint32 {
	fill(output[:], 0xab)
	status, written := result(gzipCompressPacked(^uint32(0)-1, 4, offset(&output[0]), 64, -1))
	if status != 1 || written != 0 || !unchanged(output[:16], 0xab) {
		return 0
	}
	return 1
}

//go:wasmexport checksum
func checksum() uint32 {
	written, ok := setupCompressed()
	if !ok || written < 8 {
		return 0
	}
	compressed[written-8] ^= 1
	fill(output[:], 0xac)
	status, plain := result(gzipDecompressPacked(offset(&compressed[0]), written, offset(&output[0]), uint32(len(output))))
	if status != 7 || plain != 0 || !unchanged(output[:16], 0xac) {
		return 0
	}
	return 1
}

//go:wasmexport truncation
func truncation() uint32 {
	written, ok := setupCompressed()
	if !ok || written == 0 {
		return 0
	}
	fill(output[:], 0xad)
	status, plain := result(gzipDecompressPacked(offset(&compressed[0]), written-1, offset(&output[0]), uint32(len(output))))
	if status != 6 || plain != 0 || !unchanged(output[:16], 0xad) {
		return 0
	}
	return 1
}

//go:wasmexport invalid_level
func invalidLevel() uint32 {
	putPayload(source[:])
	fill(compressed[:], 0xae)
	status, written := result(gzipCompressPacked(
		offset(&source[0]), uint32(len(payload)),
		offset(&compressed[0]), uint32(len(compressed)), 10,
	))
	if status != 1 || written != 0 || !unchanged(compressed[:16], 0xae) {
		return 0
	}
	return 1
}

func main() {}
