// Package main is a positive TinyGo-produced Wasm32 guest fixture. The
// generated module is intentionally not checked in.
package main

//go:wasmimport gzip.wasm32 abi_version
func abiVersion() int32

//go:export qualify
func qualify() int32 {
	return abiVersion()
}

func main() {}
