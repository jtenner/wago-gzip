package gzip

import (
	"bytes"
	stdflate "compress/flate"
	stdgzip "compress/gzip"
	"encoding/binary"
	"errors"
	"io"
	"time"
)

const (
	gzipFlagHeaderCRC = 1 << 1
	gzipFlagExtra     = 1 << 2
	gzipFlagName      = 1 << 3
	gzipFlagComment   = 1 << 4
	gzipFlagReserved  = 0xe0
	// compress/gzip.Reader uses a fixed 512-byte buffer for each optional
	// NUL-terminated name and comment. Enforce that maintained-code limit in
	// the pre-scan so oversized fields have a stable policy status.
	gzipHeaderStringLimit = 512
)

type codec struct {
	config Config
}

func (c codec) compress(source []byte, destinationCapacity uint64, level int32) (Status, *stagedOutput) {
	if uint64(len(source)) > c.config.MaxInputBytes {
		return StatusInputTooLarge, nil
	}
	if destinationCapacity > c.config.MaxOutputBytes {
		return StatusOutputTooLarge, nil
	}
	if level < stdflate.HuffmanOnly || level > stdflate.BestCompression {
		return StatusInvalidArgument, nil
	}

	output := newStagedOutput(destinationCapacity)
	writer, err := stdgzip.NewWriterLevel(output, int(level))
	if err != nil {
		return StatusInvalidArgument, nil
	}
	// An explicit empty header makes the wire representation reproducible:
	// MTIME=0, no optional metadata, and the portable unknown-OS marker.
	writer.Header = stdgzip.Header{ModTime: time.Time{}, OS: 255}
	_, writeErr := writer.Write(source)
	closeErr := writer.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		if errors.Is(err, errStageLimit) {
			return c.outputLimitStatus(destinationCapacity), nil
		}
		return StatusInternalError, nil
	}
	return StatusOK, output
}

func (c codec) decompress(source []byte, destinationCapacity uint64) (Status, *stagedOutput) {
	if uint64(len(source)) > c.config.MaxInputBytes {
		return StatusInputTooLarge, nil
	}
	if destinationCapacity > c.config.MaxOutputBytes {
		return StatusOutputTooLarge, nil
	}
	if len(source) == 0 {
		return StatusTruncated, nil
	}

	output := newStagedOutput(destinationCapacity)
	position := 0
	var metadata uint64
	var members uint32
	for position < len(source) {
		headerBytes, headerMetadata, status := inspectHeader(source[position:], members != 0)
		if status != StatusOK {
			return status, nil
		}
		// Classify a non-member suffix or incomplete next header before enforcing
		// the member limit so the documented trailing-data policy is independent
		// of MaxMembers. Once a complete header establishes an extra member, the
		// limit takes precedence without decoding that member's payload.
		if members == c.config.MaxMembers {
			return StatusInputTooLarge, nil
		}
		_ = headerBytes // The standard library validates the complete header.
		if headerMetadata > c.config.MaxMetadataBytes-metadata {
			return StatusInputTooLarge, nil
		}
		metadata += headerMetadata

		input := bytes.NewReader(source[position:])
		reader, err := stdgzip.NewReader(input)
		if err != nil {
			return classifyGzipError(err), nil
		}
		reader.Multistream(false)
		status = copyGzipMember(output, reader, destinationCapacity, c.config.MaxOutputBytes)
		closeErr := reader.Close()
		if status != StatusOK {
			return status, nil
		}
		if closeErr != nil {
			return classifyGzipError(closeErr), nil
		}
		consumed := len(source[position:]) - input.Len()
		if consumed <= 0 {
			return StatusInternalError, nil
		}
		position += consumed
		members++
	}
	return StatusOK, output
}

func (c codec) outputLimitStatus(destinationCapacity uint64) Status {
	if destinationCapacity == c.config.MaxOutputBytes {
		return StatusOutputTooLarge
	}
	return StatusOutputTooSmall
}

func copyGzipMember(output *stagedOutput, reader *stdgzip.Reader, destinationCapacity, maximumOutput uint64) Status {
	var scratch [32 << 10]byte
	for {
		count, readErr := reader.Read(scratch[:])
		if count != 0 {
			if _, writeErr := output.Write(scratch[:count]); writeErr != nil {
				if errors.Is(writeErr, errStageLimit) {
					if destinationCapacity == maximumOutput {
						return StatusOutputTooLarge
					}
					return StatusOutputTooSmall
				}
				return StatusInternalError
			}
		}
		if readErr == nil {
			if count == 0 {
				return StatusInternalError
			}
			continue
		}
		if errors.Is(readErr, io.EOF) {
			return StatusOK
		}
		return classifyGzipError(readErr)
	}
}

func classifyGzipError(err error) Status {
	if errors.Is(err, stdgzip.ErrChecksum) {
		return StatusChecksumMismatch
	}
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return StatusTruncated
	}
	if errors.Is(err, stdgzip.ErrHeader) {
		return StatusInvalidData
	}
	var corrupt stdflate.CorruptInputError
	if errors.As(err, &corrupt) {
		return StatusInvalidData
	}
	// gzip/flate read errors are input-derived; reserve INTERNAL_ERROR for
	// plugin invariants and host integration failures.
	return StatusInvalidData
}

// inspectHeader bounds optional metadata before compress/gzip is allowed to
// materialize Name, Comment, or Extra. It deliberately does not duplicate the
// standard library's CRC validation.
func inspectHeader(input []byte, afterMember bool) (headerBytes int, metadataBytes uint64, status Status) {
	if len(input) < 2 {
		if afterMember && (len(input) == 0 || input[0] != 0x1f) {
			return 0, 0, StatusTrailingData
		}
		return 0, 0, StatusTruncated
	}
	if input[0] != 0x1f || input[1] != 0x8b {
		if afterMember {
			return 0, 0, StatusTrailingData
		}
		return 0, 0, StatusInvalidData
	}
	if len(input) < 10 {
		return 0, 0, StatusTruncated
	}
	if input[2] != 8 || input[3]&gzipFlagReserved != 0 {
		return 0, 0, StatusInvalidData
	}
	flags := input[3]
	position := 10
	if flags&gzipFlagExtra != 0 {
		if len(input)-position < 2 {
			return 0, 0, StatusTruncated
		}
		extraLength := int(binary.LittleEndian.Uint16(input[position : position+2]))
		position += 2
		if len(input)-position < extraLength {
			return 0, 0, StatusTruncated
		}
		metadataBytes += uint64(extraLength)
		position += extraLength
	}
	if flags&gzipFlagName != 0 {
		length := bytes.IndexByte(input[position:], 0)
		if length < 0 {
			return 0, 0, StatusTruncated
		}
		length++
		if length > gzipHeaderStringLimit {
			return 0, 0, StatusInputTooLarge
		}
		metadataBytes += uint64(length)
		position += length
	}
	if flags&gzipFlagComment != 0 {
		length := bytes.IndexByte(input[position:], 0)
		if length < 0 {
			return 0, 0, StatusTruncated
		}
		length++
		if length > gzipHeaderStringLimit {
			return 0, 0, StatusInputTooLarge
		}
		metadataBytes += uint64(length)
		position += length
	}
	if flags&gzipFlagHeaderCRC != 0 {
		if len(input)-position < 2 {
			return 0, 0, StatusTruncated
		}
		position += 2
	}
	return position, metadataBytes, StatusOK
}
