package gzip

import (
	"bytes"
	stdgzip "compress/gzip"
	"encoding/binary"
	"io"
	"strings"
	"testing"
)

func testCodec(t *testing.T, edit func(*Config)) codec {
	t.Helper()
	config := defaultConfig()
	if edit != nil {
		edit(&config)
	}
	if err := validateConfig(config); err != nil {
		t.Fatal(err)
	}
	return codec{config: config}
}

func standardMember(t testing.TB, payload []byte, edit func(*stdgzip.Writer)) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := stdgzip.NewWriter(&output)
	if edit != nil {
		edit(writer)
	}
	if _, err := writer.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func requireStatus(t testing.TB, got, want Status) {
	t.Helper()
	if got != want {
		t.Fatalf("status = %d (%s), want %d (%s)", got, got, want, want)
	}
}

func TestRoundTripAndDeterministicHeader(t *testing.T) {
	engine := testCodec(t, nil)
	payload := bytes.Repeat([]byte("bounded gzip\n"), 100)
	status, first := engine.compress(payload, 4096, -1)
	requireStatus(t, status, StatusOK)
	status, second := engine.compress(payload, 4096, -1)
	requireStatus(t, status, StatusOK)
	compressed := first.bytes()
	if !bytes.Equal(compressed, second.bytes()) {
		t.Fatal("same input and level produced different gzip members")
	}
	if len(compressed) < 10 || !bytes.Equal(compressed[:2], []byte{0x1f, 0x8b}) {
		t.Fatalf("invalid gzip header: %x", compressed)
	}
	if compressed[3] != 0 || binary.LittleEndian.Uint32(compressed[4:8]) != 0 || compressed[9] != 255 {
		t.Fatalf("nondeterministic header fields: %x", compressed[:10])
	}
	status, plain := engine.decompress(compressed, uint64(len(payload)))
	requireStatus(t, status, StatusOK)
	if !bytes.Equal(plain.bytes(), payload) {
		t.Fatal("round-trip payload mismatch")
	}
}

func TestEmptyPolicy(t *testing.T) {
	engine := testCodec(t, nil)
	status, compressed := engine.compress(nil, 64, -1)
	requireStatus(t, status, StatusOK)
	if compressed.Len() == 0 {
		t.Fatal("empty plaintext did not produce a gzip member")
	}
	status, plain := engine.decompress(compressed.bytes(), 0)
	requireStatus(t, status, StatusOK)
	if plain.Len() != 0 {
		t.Fatalf("empty member decoded to %d bytes", plain.Len())
	}
	status, _ = engine.decompress(nil, 0)
	requireStatus(t, status, StatusTruncated)
}

func TestConcatenatedMembersAndTrailingPolicy(t *testing.T) {
	engine := testCodec(t, nil)
	first := standardMember(t, []byte("first"), nil)
	second := standardMember(t, []byte("second"), nil)
	concatenated := append(append([]byte(nil), first...), second...)
	status, output := engine.decompress(concatenated, 32)
	requireStatus(t, status, StatusOK)
	if got := string(output.bytes()); got != "firstsecond" {
		t.Fatalf("concatenated output = %q", got)
	}

	status, _ = engine.decompress(append(concatenated, 'x'), 32)
	requireStatus(t, status, StatusTrailingData)
	status, _ = engine.decompress(append(concatenated, 0x1f), 32)
	requireStatus(t, status, StatusTruncated)
	status, _ = engine.decompress(append(concatenated, 0x1f, 0x8b, 0x08), 32)
	requireStatus(t, status, StatusTruncated)

	invalidNext := append(append([]byte(nil), first...), 0x1f, 0x8b, 0x07)
	invalidNext = append(invalidNext, make([]byte, 7)...)
	status, _ = engine.decompress(invalidNext, 32)
	requireStatus(t, status, StatusInvalidData)
}

func TestMalformedTruncatedAndChecksumErrors(t *testing.T) {
	engine := testCodec(t, nil)
	member := standardMember(t, []byte("checksum payload"), nil)

	status, _ := engine.decompress([]byte("not gzip"), 64)
	requireStatus(t, status, StatusInvalidData)
	reserved := append([]byte(nil), member...)
	reserved[3] |= 0x20
	status, _ = engine.decompress(reserved, 64)
	requireStatus(t, status, StatusInvalidData)
	status, _ = engine.decompress(member[:len(member)-1], 64)
	requireStatus(t, status, StatusTruncated)

	badCRC := append([]byte(nil), member...)
	badCRC[len(badCRC)-8] ^= 1
	status, _ = engine.decompress(badCRC, 64)
	requireStatus(t, status, StatusChecksumMismatch)
	badSize := append([]byte(nil), member...)
	badSize[len(badSize)-4] ^= 1
	status, _ = engine.decompress(badSize, 64)
	requireStatus(t, status, StatusChecksumMismatch)
}

func TestMetadataAndMemberLimits(t *testing.T) {
	engine := testCodec(t, func(config *Config) {
		config.MaxMetadataBytes = 8
		config.MaxMembers = 1
	})
	metadata := standardMember(t, []byte("x"), func(writer *stdgzip.Writer) {
		writer.Name = strings.Repeat("n", 9)
	})
	status, _ := engine.decompress(metadata, 16)
	requireStatus(t, status, StatusInputTooLarge)

	member := standardMember(t, nil, nil)
	status, _ = engine.decompress(append(append([]byte(nil), member...), member...), 0)
	requireStatus(t, status, StatusInputTooLarge)
	status, _ = engine.decompress(append(append([]byte(nil), member...), 'x'), 0)
	requireStatus(t, status, StatusTrailingData)
	status, _ = engine.decompress(append(append([]byte(nil), member...), 0x1f), 0)
	requireStatus(t, status, StatusTruncated)
	status, _ = engine.decompress(append(append([]byte(nil), member...), member[:10]...), 0)
	requireStatus(t, status, StatusInputTooLarge)
}

func TestOutputAndInputLimits(t *testing.T) {
	engine := testCodec(t, func(config *Config) {
		config.MaxInputBytes = 128
		config.MaxOutputBytes = 64
	})
	status, _ := engine.compress(make([]byte, 129), 64, -1)
	requireStatus(t, status, StatusInputTooLarge)
	status, _ = engine.compress(nil, 1, -1)
	requireStatus(t, status, StatusOutputTooSmall)
	status, _ = engine.compress(nil, 65, -1)
	requireStatus(t, status, StatusOutputTooLarge)
	status, _ = engine.compress(nil, 64, 10)
	requireStatus(t, status, StatusInvalidArgument)

	member := standardMember(t, bytes.Repeat([]byte{'a'}, 100), nil)
	status, _ = engine.decompress(member, 10)
	requireStatus(t, status, StatusOutputTooSmall)
	status, _ = engine.decompress(member, 64)
	requireStatus(t, status, StatusOutputTooLarge)
}

func TestOptionalMetadataHeaderForms(t *testing.T) {
	engine := testCodec(t, nil)
	member := standardMember(t, []byte("metadata"), func(writer *stdgzip.Writer) {
		writer.Name = "name"
		writer.Comment = "comment"
		writer.Extra = []byte{1, 2, 3}
	})
	status, output := engine.decompress(member, 32)
	requireStatus(t, status, StatusOK)
	if string(output.bytes()) != "metadata" {
		t.Fatal("metadata-bearing member payload mismatch")
	}

	maximumName := standardMember(t, []byte("x"), func(writer *stdgzip.Writer) {
		writer.Name = strings.Repeat("n", gzipHeaderStringLimit-1)
	})
	status, output = engine.decompress(maximumName, 1)
	requireStatus(t, status, StatusOK)
	if string(output.bytes()) != "x" {
		t.Fatal("maximum supported name did not decode")
	}

	oversizedComment := standardMember(t, []byte("x"), func(writer *stdgzip.Writer) {
		writer.Comment = strings.Repeat("c", gzipHeaderStringLimit)
	})
	status, _ = engine.decompress(oversizedComment, 1)
	requireStatus(t, status, StatusInputTooLarge)
}

func TestStandardReaderAcceptsCodecOutput(t *testing.T) {
	engine := testCodec(t, nil)
	status, output := engine.compress([]byte("portable"), 128, 9)
	requireStatus(t, status, StatusOK)
	reader, err := stdgzip.NewReader(bytes.NewReader(output.bytes()))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if string(plain) != "portable" {
		t.Fatalf("plain = %q", plain)
	}
}

func FuzzDecompressBounded(f *testing.F) {
	f.Add([]byte{})
	f.Add(standardMember(f, []byte("seed"), nil))
	engine := codec{config: Config{
		MaxInputBytes:           1 << 20,
		MaxOutputBytes:          1 << 20,
		MaxConcurrentOperations: 1,
		MaxMetadataBytes:        64 << 10,
		MaxMembers:              128,
	}}
	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > 1<<20 {
			t.Skip()
		}
		status, output := engine.decompress(input, 1<<20)
		if status == StatusOK && output.Len() > 1<<20 {
			t.Fatalf("bounded decode returned %d bytes", output.Len())
		}
	})
}

func BenchmarkCompress(b *testing.B) {
	engine := codec{config: defaultConfig()}
	payload := bytes.Repeat([]byte("wago gzip benchmark payload\n"), 4096)
	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	b.ResetTimer()
	for range b.N {
		status, _ := engine.compress(payload, 1<<20, -1)
		if status != StatusOK {
			b.Fatal(status)
		}
	}
}

func BenchmarkDecompress(b *testing.B) {
	engine := codec{config: defaultConfig()}
	payload := bytes.Repeat([]byte("wago gzip benchmark payload\n"), 4096)
	compressed := standardMember(b, payload, nil)
	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	b.ResetTimer()
	for range b.N {
		status, _ := engine.decompress(compressed, uint64(len(payload)))
		if status != StatusOK {
			b.Fatal(status)
		}
	}
}
