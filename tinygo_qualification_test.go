//go:build tinygo_qualification

package gzip

import (
	"os"
	"testing"
)

// TestTinyGoProducedWasm32PackedGuest executes real codec calls from a guest
// built by the pinned TinyGo toolchain. CI runs this test in both Go-compiled
// and TinyGo-compiled Wago hosts.
func TestTinyGoProducedWasm32PackedGuest(t *testing.T) {
	guest, err := os.ReadFile("testdata/tinygo-wasm32-packed.generated.wasm")
	if err != nil {
		t.Fatal(err)
	}
	instance := integrationGuest(t, guest)
	if result := invokeIntegration(t, instance, "_initialize"); len(result) != 0 {
		t.Fatalf("TinyGo-produced guest initialization = %v", result)
	}
	for _, export := range []string{
		"roundtrip", "empty", "overlap_case", "output_small",
		"bounds", "checksum", "truncation", "invalid_level",
	} {
		result := invokeIntegration(t, instance, export)
		if len(result) != 1 || result[0] != 1 {
			t.Fatalf("TinyGo-produced guest %s = %v", export, result)
		}
	}
}
