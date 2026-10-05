package gzip

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	DefaultMaxInputBytes           uint64 = 8 << 20
	DefaultMaxOutputBytes          uint64 = 32 << 20
	DefaultMaxConcurrentOperations uint32 = 2
	DefaultMaxMetadataBytes        uint64 = 64 << 10
	DefaultMaxMembers              uint32 = 1024

	HardMaxInputBytes           uint64 = 64 << 20
	HardMaxOutputBytes          uint64 = 64 << 20
	HardMaxConcurrentOperations uint32 = 8
	HardMaxMetadataBytes        uint64 = 1 << 20
	HardMaxMembers              uint32 = 65536
)

// Config bounds all input-controlled work. Zero Go values select defaults when
// Config is supplied programmatically. Explicit JSON zeroes are rejected.
type Config struct {
	MaxInputBytes           uint64 `json:"max_input_bytes,omitempty"`
	MaxOutputBytes          uint64 `json:"max_output_bytes,omitempty"`
	MaxConcurrentOperations uint32 `json:"max_concurrent_operations,omitempty"`
	MaxMetadataBytes        uint64 `json:"max_metadata_bytes,omitempty"`
	MaxMembers              uint32 `json:"max_members,omitempty"`
}

func defaultConfig() Config {
	return Config{
		MaxInputBytes:           DefaultMaxInputBytes,
		MaxOutputBytes:          DefaultMaxOutputBytes,
		MaxConcurrentOperations: DefaultMaxConcurrentOperations,
		MaxMetadataBytes:        DefaultMaxMetadataBytes,
		MaxMembers:              DefaultMaxMembers,
	}
}

func normalizeConfig(config Config) (Config, error) {
	if config.MaxInputBytes == 0 {
		config.MaxInputBytes = DefaultMaxInputBytes
	}
	if config.MaxOutputBytes == 0 {
		config.MaxOutputBytes = DefaultMaxOutputBytes
	}
	if config.MaxConcurrentOperations == 0 {
		config.MaxConcurrentOperations = DefaultMaxConcurrentOperations
	}
	if config.MaxMetadataBytes == 0 {
		config.MaxMetadataBytes = DefaultMaxMetadataBytes
	}
	if config.MaxMembers == 0 {
		config.MaxMembers = DefaultMaxMembers
	}
	if err := validateConfig(config); err != nil {
		return Config{}, err
	}
	return config, nil
}

func validateConfig(config Config) error {
	switch {
	case config.MaxInputBytes == 0 || config.MaxInputBytes > HardMaxInputBytes:
		return fmt.Errorf("max_input_bytes must be between 1 and %d", HardMaxInputBytes)
	case config.MaxOutputBytes == 0 || config.MaxOutputBytes > HardMaxOutputBytes:
		return fmt.Errorf("max_output_bytes must be between 1 and %d", HardMaxOutputBytes)
	case config.MaxConcurrentOperations == 0 || config.MaxConcurrentOperations > HardMaxConcurrentOperations:
		return fmt.Errorf("max_concurrent_operations must be between 1 and %d", HardMaxConcurrentOperations)
	case config.MaxMetadataBytes == 0 || config.MaxMetadataBytes > HardMaxMetadataBytes:
		return fmt.Errorf("max_metadata_bytes must be between 1 and %d", HardMaxMetadataBytes)
	case config.MaxMembers == 0 || config.MaxMembers > HardMaxMembers:
		return fmt.Errorf("max_members must be between 1 and %d", HardMaxMembers)
	default:
		return nil
	}
}

func parseConfig(raw json.RawMessage) (Config, error) {
	config := defaultConfig()
	if len(raw) == 0 {
		return config, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return Config{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Config{}, errors.New("trailing JSON value")
	}
	if err := validateConfig(config); err != nil {
		return Config{}, err
	}
	return config, nil
}
