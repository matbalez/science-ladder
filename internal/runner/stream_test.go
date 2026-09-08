package runner

import (
	"bytes"
	"github.com/matbalez/science-ladder/pkg/protocol"
	"os"
	"path/filepath"
	"testing"
)

func TestStreamedObjectRejectsTruncationAndTrailingBytes(t *testing.T) {
	data := bytes.Repeat([]byte("bounded immutable bytes"), 100000)
	ref := protocol.ObjectRef{Digest: protocol.DigestBytes(data), Size: int64(len(data))}
	for _, input := range [][]byte{data[:len(data)-1], append(append([]byte(nil), data...), 0), bytes.Repeat([]byte("x"), len(data))} {
		out := filepath.Join(t.TempDir(), "object")
		if writeVerifiedObject(bytes.NewReader(input), ref, out) == nil {
			t.Fatal("invalid stream admitted")
		}
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Fatal("partial file survived")
		}
	}
	out := filepath.Join(t.TempDir(), "object")
	if err := writeVerifiedObject(bytes.NewReader(data), ref, out); err != nil {
		t.Fatal(err)
	}
	if err := writeVerifiedObject(bytes.NewReader(data), ref, out); err == nil {
		t.Fatal("existing object overwritten")
	}
}
