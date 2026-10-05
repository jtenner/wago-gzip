//go:build tinygo_qualification

package gzip

import (
	"os"
	"testing"
)

// TestTinyGoProducedWasm32VersionGuest executes a guest compiled by the pinned
// TinyGo toolchain inside a TinyGo-compiled Wago host. The fixture deliberately
// uses the single-result ABI version import: TinyGo 0.42.0 rejects the codec's
// two-result compress/decompress imports, which the guest verification script
// checks as an explicit unsupported case.
func TestTinyGoProducedWasm32VersionGuest(t *testing.T) {
	guest, err := os.ReadFile("testdata/tinygo-wasm32-version.generated.wasm")
	if err != nil {
		t.Fatal(err)
	}
	instance := integrationGuest(t, guest)
	result := invokeIntegration(t, instance, "qualify")
	if len(result) != 1 || result[0] != uint64(ABIVersion) {
		t.Fatalf("TinyGo-produced guest ABI version = %v", result)
	}
}
