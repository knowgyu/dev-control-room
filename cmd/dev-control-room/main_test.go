package main

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/knowgyu/dev-control-room/internal/contract"
)

func TestWriteCLIErrorDoesNotExposeInternalDetails(t *testing.T) {
	original := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = writer
	code := writeCLIError(errors.New("open C:\\private\\secret-canary: SQL details"))
	_ = writer.Close()
	os.Stderr = original
	data, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	if code != int(contract.ExitInternal) || strings.Contains(string(data), "secret-canary") || strings.Contains(string(data), "SQL details") || !strings.Contains(string(data), `"message":"internal error"`) {
		t.Fatalf("unexpected CLI error output: code=%d output=%s", code, data)
	}
}
