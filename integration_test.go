package gzip

import (
	"context"
	"embed"
	"testing"

	wago "github.com/wago-org/wago"
)

// Precompiled fixtures keep normal tests independent of wasm-tools.
//
//go:embed testdata/*.wasm
var integrationGuests embed.FS

func integrationFixture(t *testing.T, mode string) *wago.Instance {
	t.Helper()
	guest, err := integrationGuests.ReadFile("testdata/" + mode + ".wasm")
	if err != nil {
		t.Fatal(err)
	}
	runtime := wago.NewRuntime(wago.WithRuntimeConfig(wago.NewRuntimeConfig().WithCoreFeatures(wago.CoreFeaturesV3)))
	t.Cleanup(func() {
		if err := runtime.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := runtime.LoadPlugins(context.Background(), PluginSet()); err != nil {
		t.Fatal(err)
	}
	module, err := runtime.Compile(guest)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { module.Close() })
	instance, err := runtime.Instantiate(context.Background(), module)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { instance.Close() })
	return instance
}

func invokeIntegration(t *testing.T, instance *wago.Instance, export string) []uint64 {
	t.Helper()
	result, err := instance.Invoke(export)
	if err != nil {
		t.Fatalf("%s: %v", export, err)
	}
	return result
}

func TestWagoABIsRoundTrip(t *testing.T) {
	for _, mode := range []string{"gc", "wasm32", "wasm64"} {
		t.Run(mode, func(t *testing.T) {
			instance := integrationFixture(t, mode)
			if result := invokeIntegration(t, instance, "version"); len(result) != 1 || int32(result[0]) != ABIVersion {
				t.Fatalf("version = %v", result)
			}
			result := invokeIntegration(t, instance, "roundtrip")
			if len(result) != 6 || result[0] != uint64(StatusOK) || result[1] == 0 ||
				result[2] != uint64(StatusOK) || result[3] != 8 || result[4] != 'g' || result[5] != 'g' {
				t.Fatalf("roundtrip = %v", result)
			}
		})
	}

	wasm64 := integrationFixture(t, "wasm64")
	if result := invokeIntegration(t, wasm64, "wide_bounds_failure"); len(result) != 2 ||
		result[0] != uint64(StatusInvalidArgument) || result[1] != 0 {
		t.Fatalf("wide bounds failure = %v", result)
	}
	gc := integrationFixture(t, "gc")
	if result := invokeIntegration(t, gc, "type_failure"); len(result) != 2 ||
		result[0] != uint64(StatusInvalidArgument) || result[1] != 0 {
		t.Fatalf("GC type failure = %v", result)
	}
}

func TestWagoABIsOverlapBoundsAndFailureAtomicity(t *testing.T) {
	for _, mode := range []string{"gc", "wasm32", "wasm64"} {
		t.Run(mode, func(t *testing.T) {
			instance := integrationFixture(t, mode)
			result := invokeIntegration(t, instance, "overlap")
			if len(result) != 6 || result[0] != uint64(StatusOK) || result[1] == 0 ||
				result[2] != uint64(StatusOK) || result[3] != 8 || result[4] != 'g' || result[5] != 'g' {
				t.Fatalf("overlap = %v", result)
			}
			result = invokeIntegration(t, instance, "atomic_failure")
			if len(result) != 3 || result[0] != uint64(StatusOutputTooSmall) || result[1] != 0 || result[2] != 170 {
				t.Fatalf("atomic failure = %v", result)
			}
			result = invokeIntegration(t, instance, "bounds_failure")
			if len(result) != 2 || result[0] != uint64(StatusInvalidArgument) || result[1] != 0 {
				t.Fatalf("bounds failure = %v", result)
			}
		})
	}
}
