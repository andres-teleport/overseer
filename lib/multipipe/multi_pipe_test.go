package multipipe

import (
	"bytes"
	"io"
	"testing"
)

func TestReader(t *testing.T) {
	testPhrase := []byte("hello multipipe")

	mp := NewMultiPipe()

	go func() {
		mp.Write(testPhrase)
		mp.Close()
	}()

	rd := mp.NewReader()

	out, err := io.ReadAll(rd)
	if err != nil {
		t.Fatal(err)
	} else if !bytes.Equal(out, testPhrase) {
		t.Errorf("expected '%s', got '%s'", string(testPhrase), string(out))
	}
}
