package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/knowgyu/dev-control-room/internal/app"
)

func TestServeExposesOnlyTypedToolsAndStableResults(t *testing.T) {
	service, err := app.New(t.TempDir(), "127.0.0.1:38471")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	input := strings.NewReader("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\"}\n" +
		"{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/list\"}\n" +
		"{\"jsonrpc\":\"2.0\",\"id\":3,\"method\":\"tools/call\",\"params\":{\"name\":\"project.list\",\"arguments\":{}}}\n")
	var output bytes.Buffer
	if err := Serve(context.Background(), input, &output, service); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 3 || !strings.Contains(output.String(), `"dev-control-room"`) || strings.Contains(output.String(), "shell") || strings.Contains(output.String(), "file.read") {
		t.Fatalf("unexpected MCP output: %s", output.String())
	}
	var call map[string]any
	if err := json.Unmarshal([]byte(lines[2]), &call); err != nil || call["result"] == nil {
		t.Fatalf("invalid MCP tool result: %s (%v)", lines[2], err)
	}
}
