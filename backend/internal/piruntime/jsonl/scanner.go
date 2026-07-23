// Package jsonl reads newline-delimited records from streams.
package jsonl

import (
	"bufio"
	"io"
)

const initialBufferSize = 64 * 1024

// Scan visits each complete line from reader and stops on the first error.
func Scan(reader io.Reader, maximumLine int, visit func([]byte) error) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, initialBufferSize), maximumLine)
	for scanner.Scan() {
		if err := visit(scanner.Bytes()); err != nil {
			return err
		}
	}
	return scanner.Err()
}
