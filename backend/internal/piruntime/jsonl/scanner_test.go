package jsonl_test

import (
	"reflect"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/mishankov/todai/backend/internal/piruntime/jsonl"
)

func TestScanAcceptsFragmentedLines(t *testing.T) {
	reader := iotest.OneByteReader(strings.NewReader("first record\nsecond record\n"))
	lines := make([]string, 0, 2)

	err := jsonl.Scan(reader, 1024, func(line []byte) error {
		lines = append(lines, string(line))
		return nil
	})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	want := []string{"first record", "second record"}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("lines = %#v, want %#v", lines, want)
	}
}
