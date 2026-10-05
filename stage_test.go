package gzip

import (
	"bytes"
	"errors"
	"testing"
)

func TestStagedOutputChunksAndLimit(t *testing.T) {
	stage := newStagedOutput(stageChunkSize + 3)
	payload := bytes.Repeat([]byte{0xa5}, stageChunkSize+3)
	if n, err := stage.Write(payload); err != nil || n != len(payload) {
		t.Fatalf("write = %d, %v", n, err)
	}
	if len(stage.chunks) != 2 || !bytes.Equal(stage.bytes(), payload) {
		t.Fatalf("chunks=%d size=%d", len(stage.chunks), stage.Len())
	}
	before := stage.bytes()
	if n, err := stage.Write([]byte{1}); n != 0 || !errors.Is(err, errStageLimit) {
		t.Fatalf("overflow write = %d, %v", n, err)
	}
	if !bytes.Equal(stage.bytes(), before) {
		t.Fatal("failed write mutated staged output")
	}
}
