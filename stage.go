package gzip

import (
	"errors"
	"io"
)

const stageChunkSize = 32 << 10

var errStageLimit = errors.New("gzip output limit reached")

// stagedOutput keeps output transactional without requiring one contiguous
// allocation or allowing bytes.Buffer growth to overshoot the configured cap.
type stagedOutput struct {
	chunks [][]byte
	limit  uint64
	size   uint64
}

func newStagedOutput(limit uint64) *stagedOutput {
	return &stagedOutput{limit: limit}
}

func (s *stagedOutput) Write(p []byte) (int, error) {
	if uint64(len(p)) > s.limit-s.size {
		return 0, errStageLimit
	}
	written := len(p)
	for len(p) != 0 {
		if len(s.chunks) == 0 || len(s.chunks[len(s.chunks)-1]) == cap(s.chunks[len(s.chunks)-1]) {
			remaining := s.limit - s.size
			capacity := uint64(stageChunkSize)
			if remaining < capacity {
				capacity = remaining
			}
			if capacity == 0 {
				return 0, errStageLimit
			}
			s.chunks = append(s.chunks, make([]byte, 0, int(capacity)))
		}
		last := len(s.chunks) - 1
		available := cap(s.chunks[last]) - len(s.chunks[last])
		count := len(p)
		if count > available {
			count = available
		}
		s.chunks[last] = append(s.chunks[last], p[:count]...)
		s.size += uint64(count)
		p = p[count:]
	}
	return written, nil
}

func (s *stagedOutput) Len() uint64 { return s.size }

func (s *stagedOutput) copyTo(destination []byte) int {
	offset := 0
	for _, chunk := range s.chunks {
		offset += copy(destination[offset:], chunk)
	}
	return offset
}

func (s *stagedOutput) bytes() []byte {
	result := make([]byte, int(s.size))
	s.copyTo(result)
	return result
}

var _ io.Writer = (*stagedOutput)(nil)
