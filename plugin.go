// Package gzip provides bounded RFC 1952 compression and decompression imports
// for Wago guests. It delegates the codec implementation to Go's maintained
// compress/gzip package.
package gzip

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	wago "github.com/wago-org/wago"
)

const (
	ModuleGC     = "gzip.gc"
	ModuleWasm32 = "gzip.wasm32"
	ModuleWasm64 = "gzip.wasm64"
	PluginID     = "github.com/jtenner/wago-gzip"
	ABIVersion   = int32(1)
)

//go:embed config.schema.json
var configSchema []byte

var errWrongMemoryWidth = errors.New("gzip namespace does not match guest memory width")

type transport uint8

const (
	transportGC transport = iota
	transportWasm32
	transportWasm64
)

// Plugin owns one per-Runtime concurrency gate. It retains no guest storage or
// codec data between calls.
type Plugin struct {
	mu          sync.Mutex
	active      sync.WaitGroup
	config      Config
	slots       chan struct{}
	initialized bool
	closed      bool
}

// Provider returns the immutable provider definition. Configuration belongs to
// the reviewed PluginSelection and is decoded during Register.
func Provider() wago.PluginProvider {
	definition := wago.PluginDefinition{
		ID:          PluginID,
		Name:        "Gzip",
		Version:     "0.0.1",
		Description: "Bounded RFC 1952 gzip compression for GC, Wasm32, and Wasm64 guests",
		Stability:   wago.Experimental,
		Compatibility: wago.Compatibility{
			Engines: map[string]string{"wago": ">=0.1.0-beta.11"},
		},
		Provenance: wago.PluginProvenance{
			Repository: "https://github.com/jtenner/wago-gzip",
			License:    "Apache-2.0",
			Authors:    []string{"jtenner"},
		},
		ConfigSchema: append(json.RawMessage(nil), configSchema...),
		Authorities: []wago.AuthorityRequest{{
			Name:   wago.AuthorityHostImportDefine,
			Mode:   wago.AuthorityRequired,
			Reason: "Define bounded gzip codec imports",
			Scope:  wago.AuthorityScope{Modules: []string{ModuleGC, ModuleWasm32, ModuleWasm64}},
		}},
	}
	return wago.PluginProvider{
		Definition: definition,
		New:        func() wago.Plugin { return &Plugin{} },
		ValidateConfig: func(raw json.RawMessage) error {
			_, err := parseConfig(raw)
			return err
		},
	}
}

// PluginSet selects the provider and grants only its declared import authority.
// At most one Config may be supplied; invalid programmatic configuration
// panics so it cannot silently weaken a reviewed limit.
func PluginSet(configs ...Config) wago.PluginSet {
	if len(configs) > 1 {
		panic("gzip.PluginSet accepts at most one Config")
	}
	provider := Provider()
	selection := wago.PluginSelection{
		ID:           provider.Definition.ID,
		Direct:       true,
		Dependencies: map[string]string{},
	}
	digest, err := wago.DefinitionDigest(provider.Definition)
	if err != nil {
		panic(err)
	}
	selection.DefinitionDigest = digest
	request := provider.Definition.Authorities[0]
	selection.Grants = []wago.AuthorityGrant{{Name: request.Name, Scope: request.Scope}}
	if len(configs) == 1 {
		config, err := normalizeConfig(configs[0])
		if err != nil {
			panic(err)
		}
		selection.Config, err = json.Marshal(config)
		if err != nil {
			panic(err)
		}
	}
	return wago.PluginSet{
		Providers:  []wago.PluginProvider{provider},
		Selections: []wago.PluginSelection{selection},
	}
}

func (p *Plugin) Register(registrar *wago.Registrar) error {
	config := defaultConfig()
	if err := registrar.Config(&config); err != nil {
		return err
	}
	if err := validateConfig(config); err != nil {
		return fmt.Errorf("gzip: invalid configuration: %w", err)
	}
	if err := p.initialize(config); err != nil {
		return err
	}

	imports, err := registrar.HostImports()
	if err != nil {
		return err
	}
	for _, binding := range []struct {
		module string
		mode   transport
	}{
		{ModuleGC, transportGC},
		{ModuleWasm32, transportWasm32},
		{ModuleWasm64, transportWasm64},
	} {
		mode := binding.mode
		imports.HostFunc(binding.module, "abi_version", func(call wago.HostCall) {
			call.SetI32(0, ABIVersion)
		}).Results(wago.ValI32).Docs("return the gzip ABI version")

		compressParams, decompressParams := signatures(mode)
		imports.HostFunc(binding.module, "compress", func(caller wago.Caller, call wago.HostCall) {
			p.invoke(caller, call, mode, true)
		}).Params(compressParams...).Results(wago.ValI32, wago.ValI32).
			Docs("compress one deterministic RFC 1952 member")
		imports.HostFunc(binding.module, "decompress", func(caller wago.Caller, call wago.HostCall) {
			p.invoke(caller, call, mode, false)
		}).Params(decompressParams...).Results(wago.ValI32, wago.ValI32).
			Docs("decompress and verify one or more RFC 1952 members")
		imports.HostFunc(binding.module, "compress_packed", func(caller wago.Caller, call wago.HostCall) {
			p.invokePacked(caller, call, mode, true)
		}).Params(compressParams...).Results(wago.ValI64).
			Docs("compress and pack status in low 32 bits and written in high 32 bits")
		imports.HostFunc(binding.module, "decompress_packed", func(caller wago.Caller, call wago.HostCall) {
			p.invokePacked(caller, call, mode, false)
		}).Params(decompressParams...).Results(wago.ValI64).
			Docs("decompress and pack status in low 32 bits and written in high 32 bits")
	}
	return registrar.Lifecycle(wago.PluginLifecycle{Stop: func(context.Context) error {
		p.Close()
		return nil
	}})
}

func signatures(mode transport) (compress, decompress []wago.ValType) {
	switch mode {
	case transportGC:
		compress = []wago.ValType{wago.ValAnyRef, wago.ValI32, wago.ValI32, wago.ValAnyRef, wago.ValI32, wago.ValI32, wago.ValI32}
		decompress = compress[:6]
	case transportWasm32:
		compress = []wago.ValType{wago.ValI32, wago.ValI32, wago.ValI32, wago.ValI32, wago.ValI32}
		decompress = compress[:4]
	case transportWasm64:
		compress = []wago.ValType{wago.ValI64, wago.ValI64, wago.ValI64, wago.ValI64, wago.ValI32}
		decompress = compress[:4]
	default:
		panic("invalid gzip transport")
	}
	return compress, decompress
}

func (p *Plugin) initialize(config Config) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.initialized {
		return errors.New("gzip: plugin registered more than once")
	}
	if p.closed {
		return errors.New("gzip: plugin is closed")
	}
	p.config = config
	p.slots = make(chan struct{}, int(config.MaxConcurrentOperations))
	p.initialized = true
	return nil
}

func (p *Plugin) begin() Status {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.initialized || p.closed {
		return StatusUnsupported
	}
	select {
	case p.slots <- struct{}{}:
		p.active.Add(1)
		return StatusOK
	default:
		return StatusBusy
	}
}

func (p *Plugin) end() {
	<-p.slots
	p.active.Done()
}

// Close prevents new work and waits for active operations to release all
// detached input and staged output. It is idempotent.
func (p *Plugin) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	p.mu.Unlock()
	p.active.Wait()
}

type request struct {
	mode                transport
	sourceToken         uint64
	sourceOffset        uint64
	sourceLength        uint64
	destinationToken    uint64
	destinationOffset   uint64
	destinationCapacity uint64
	level               int32
}

func parseRequest(call wago.HostCall, mode transport, compress bool) request {
	slots := call.ParamSlots()
	result := request{mode: mode, level: -1}
	switch mode {
	case transportGC:
		result.sourceToken = slots[0]
		result.sourceOffset = uint64(uint32(slots[1]))
		result.sourceLength = uint64(uint32(slots[2]))
		result.destinationToken = slots[3]
		result.destinationOffset = uint64(uint32(slots[4]))
		result.destinationCapacity = uint64(uint32(slots[5]))
		if compress {
			result.level = int32(uint32(slots[6]))
		}
	case transportWasm32:
		result.sourceOffset = uint64(uint32(slots[0]))
		result.sourceLength = uint64(uint32(slots[1]))
		result.destinationOffset = uint64(uint32(slots[2]))
		result.destinationCapacity = uint64(uint32(slots[3]))
		if compress {
			result.level = int32(uint32(slots[4]))
		}
	case transportWasm64:
		result.sourceOffset = slots[0]
		result.sourceLength = slots[1]
		result.destinationOffset = slots[2]
		result.destinationCapacity = slots[3]
		if compress {
			result.level = int32(uint32(slots[4]))
		}
	}
	return result
}

func (p *Plugin) invoke(caller wago.Caller, call wago.HostCall, mode transport, compress bool) {
	status := StatusInternalError
	var written int32
	defer func() {
		if recover() != nil {
			status = StatusInternalError
			written = 0
		}
		call.SetI32(0, int32(status))
		call.SetI32(1, written)
	}()

	operation := parseRequest(call, mode, compress)
	status, written = p.execute(caller, operation, compress)
}

func (p *Plugin) invokePacked(caller wago.Caller, call wago.HostCall, mode transport, compress bool) {
	status := StatusInternalError
	var written int32
	defer func() {
		if recover() != nil {
			status = StatusInternalError
			written = 0
		}
		call.SetI64(0, packResult(status, written))
	}()

	operation := parseRequest(call, mode, compress)
	status, written = p.execute(caller, operation, compress)
}

func packResult(status Status, written int32) int64 {
	if status != StatusOK {
		written = 0
	}
	if written < 0 {
		status = StatusInternalError
		written = 0
	}
	return int64(uint64(uint32(status)) | uint64(uint32(written))<<32)
}

func (p *Plugin) execute(caller wago.Caller, operation request, compress bool) (Status, int32) {
	if status := p.begin(); status != StatusOK {
		return status, 0
	}
	defer p.end()
	if operation.sourceLength > p.config.MaxInputBytes {
		return StatusInputTooLarge, 0
	}
	if operation.destinationCapacity > p.config.MaxOutputBytes {
		return StatusOutputTooLarge, 0
	}

	source, status := copyGuestInput(caller, operation)
	if status != StatusOK {
		return status, 0
	}
	engine := codec{config: p.config}
	var output *stagedOutput
	if compress {
		status, output = engine.compress(source, operation.destinationCapacity, operation.level)
	} else {
		status, output = engine.decompress(source, operation.destinationCapacity)
	}
	if status != StatusOK {
		return status, 0
	}
	if status = commitGuestOutput(caller, operation, output); status != StatusOK {
		return status, 0
	}
	return StatusOK, int32(output.Len())
}

func copyGuestInput(caller wago.Caller, operation request) ([]byte, Status) {
	var detached []byte
	err := caller.WithGuestStorage(func(storage wago.GuestStorage) error {
		source, destination, err := guestRanges(storage, operation)
		if err != nil {
			return err
		}
		_ = destination // Validate destination writability before doing codec work.
		detached = append(make([]byte, 0, len(source)), source...)
		return nil
	})
	if errors.Is(err, errWrongMemoryWidth) {
		return nil, StatusUnsupported
	}
	if err != nil {
		return nil, StatusInvalidArgument
	}
	return detached, StatusOK
}

func commitGuestOutput(caller wago.Caller, operation request, output *stagedOutput) Status {
	err := caller.WithGuestStorage(func(storage wago.GuestStorage) error {
		destination, err := guestDestination(storage, operation)
		if err != nil {
			return err
		}
		if uint64(len(destination)) < output.Len() {
			return errors.New("gzip: destination changed before commit")
		}
		output.copyTo(destination)
		return nil
	})
	if errors.Is(err, errWrongMemoryWidth) {
		return StatusUnsupported
	}
	if err != nil {
		return StatusInvalidArgument
	}
	return StatusOK
}

func guestRanges(storage wago.GuestStorage, operation request) (source, destination []byte, err error) {
	source, err = guestSource(storage, operation)
	if err != nil {
		return nil, nil, err
	}
	destination, err = guestDestination(storage, operation)
	if err != nil {
		return nil, nil, err
	}
	return source, destination, nil
}

func guestSource(storage wago.GuestStorage, operation request) ([]byte, error) {
	if operation.mode == transportGC {
		ref, err := storage.GCRef(operation.sourceToken)
		if err != nil {
			return nil, err
		}
		data, info, err := storage.GCArrayBytes(ref, wago.GuestStorageRead)
		if err != nil {
			return nil, err
		}
		if info.Storage != wago.GuestGCArrayI8 {
			return nil, errors.New("gzip: GC source must be an i8 array")
		}
		return checkedRange(data, operation.sourceOffset, operation.sourceLength)
	}
	if err := checkMemoryWidth(storage, operation.mode); err != nil {
		return nil, err
	}
	return storage.MemoryRange(0, operation.sourceOffset, operation.sourceLength, wago.GuestStorageRead)
}

func guestDestination(storage wago.GuestStorage, operation request) ([]byte, error) {
	if operation.mode == transportGC {
		ref, err := storage.GCRef(operation.destinationToken)
		if err != nil {
			return nil, err
		}
		data, info, err := storage.GCArrayBytes(ref, wago.GuestStorageWrite)
		if err != nil {
			return nil, err
		}
		if info.Storage != wago.GuestGCArrayI8 {
			return nil, errors.New("gzip: GC destination must be a mutable i8 array")
		}
		return checkedRange(data, operation.destinationOffset, operation.destinationCapacity)
	}
	if err := checkMemoryWidth(storage, operation.mode); err != nil {
		return nil, err
	}
	return storage.MemoryRange(0, operation.destinationOffset, operation.destinationCapacity, wago.GuestStorageWrite)
}

func checkMemoryWidth(storage wago.GuestStorage, mode transport) error {
	info, err := storage.MemoryInfo(0)
	if err != nil {
		return err
	}
	want := wago.GuestMemory32
	if mode == transportWasm64 {
		want = wago.GuestMemory64
	}
	if info.AddressType != want {
		return errWrongMemoryWidth
	}
	return nil
}

func checkedRange(data []byte, offset, length uint64) ([]byte, error) {
	if offset > uint64(len(data)) || length > uint64(len(data))-offset {
		return nil, errors.New("gzip: guest array range is out of bounds")
	}
	return data[int(offset):int(offset+length)], nil
}
