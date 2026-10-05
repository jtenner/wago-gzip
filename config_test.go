package gzip

import (
	"encoding/json"
	"testing"
)

func TestParseConfigDefaultsAndOverrides(t *testing.T) {
	config, err := parseConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if config != defaultConfig() {
		t.Fatalf("defaults = %+v", config)
	}
	config, err = parseConfig(json.RawMessage(`{"max_input_bytes":123,"max_members":7}`))
	if err != nil {
		t.Fatal(err)
	}
	if config.MaxInputBytes != 123 || config.MaxMembers != 7 || config.MaxOutputBytes != DefaultMaxOutputBytes {
		t.Fatalf("overrides = %+v", config)
	}
}

func TestParseConfigRejectsInvalidJSONAndLimits(t *testing.T) {
	for _, raw := range []string{
		`{"unknown":1}`,
		`{"max_input_bytes":0}`,
		`{"max_output_bytes":67108865}`,
		`{"max_concurrent_operations":9}`,
		`{"max_metadata_bytes":1048577}`,
		`{"max_members":65537}`,
		`{} {}`,
	} {
		if _, err := parseConfig(json.RawMessage(raw)); err == nil {
			t.Fatalf("accepted invalid config %s", raw)
		}
	}
}

func TestNormalizeProgrammaticZeroes(t *testing.T) {
	config, err := normalizeConfig(Config{MaxInputBytes: 123})
	if err != nil {
		t.Fatal(err)
	}
	if config.MaxInputBytes != 123 || config.MaxOutputBytes != DefaultMaxOutputBytes || config.MaxMembers != DefaultMaxMembers {
		t.Fatalf("normalized = %+v", config)
	}
}
