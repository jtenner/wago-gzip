package gzip

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPackResult(t *testing.T) {
	tests := []struct {
		status  Status
		written int32
		want    uint64
	}{
		{StatusOK, 17, uint64(17) << 32},
		{StatusOutputTooSmall, 17, uint64(StatusOutputTooSmall)},
		{StatusOK, -1, uint64(StatusInternalError)},
	}
	for _, test := range tests {
		if got := uint64(packResult(test.status, test.written)); got != test.want {
			t.Errorf("packResult(%v, %d) = %#x, want %#x", test.status, test.written, got, test.want)
		}
	}
	for status := StatusInvalidArgument; status <= StatusInternalError; status++ {
		if got := uint64(packResult(status, 17)); got != uint64(uint32(status)) {
			t.Errorf("packResult(%v, 17) = %#x; failure must have written=0", status, got)
		}
	}
}

func TestProviderAndPluginSet(t *testing.T) {
	provider := Provider()
	if provider.Definition.ID != PluginID || provider.Definition.Provenance.License != "Apache-2.0" {
		t.Fatalf("definition = %+v", provider.Definition)
	}
	if len(provider.Definition.Authorities) != 1 || len(provider.Definition.Authorities[0].Scope.Modules) != 3 {
		t.Fatalf("authorities = %+v", provider.Definition.Authorities)
	}
	if err := provider.ValidateConfig(json.RawMessage(`{"max_members":2}`)); err != nil {
		t.Fatal(err)
	}
	set := PluginSet(Config{MaxInputBytes: 1024})
	if len(set.Providers) != 1 || len(set.Selections) != 1 || len(set.Selections[0].Config) == 0 {
		t.Fatalf("plugin set = %+v", set)
	}
}

func TestConcurrencyGateIsNonblockingAndCloseDrains(t *testing.T) {
	config := defaultConfig()
	config.MaxConcurrentOperations = 2
	plugin := &Plugin{}
	if err := plugin.initialize(config); err != nil {
		t.Fatal(err)
	}
	requireStatus(t, plugin.begin(), StatusOK)
	requireStatus(t, plugin.begin(), StatusOK)
	started := time.Now()
	requireStatus(t, plugin.begin(), StatusBusy)
	if time.Since(started) > 50*time.Millisecond {
		t.Fatal("busy gate blocked")
	}

	closed := make(chan struct{})
	go func() {
		plugin.Close()
		close(closed)
	}()
	select {
	case <-closed:
		t.Fatal("Close returned with active operations")
	case <-time.After(10 * time.Millisecond):
	}
	plugin.end()
	plugin.end()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Close did not drain")
	}
	requireStatus(t, plugin.begin(), StatusUnsupported)
	plugin.Close()
}

func TestCheckedRange(t *testing.T) {
	data := []byte{0, 1, 2, 3}
	got, err := checkedRange(data, 1, 2)
	if err != nil || len(got) != 2 || got[0] != 1 {
		t.Fatalf("range = %v, %v", got, err)
	}
	for _, pair := range [][2]uint64{{5, 0}, {3, 2}, {^uint64(0), 2}} {
		if _, err := checkedRange(data, pair[0], pair[1]); err == nil {
			t.Fatalf("accepted range %v", pair)
		}
	}
}
