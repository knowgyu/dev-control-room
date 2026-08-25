package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/knowgyu/dev-control-room/internal/app"
	"github.com/knowgyu/dev-control-room/internal/contract"
)

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

var tools = []tool{
	{Name: "project.list", Description: "List registered projects.", InputSchema: objectSchema(nil)},
	{Name: "finding.list", Description: "List masked findings within an optional project and repository scope.", InputSchema: objectSchema(map[string]any{"projectId": stringProperty(), "repositoryId": stringProperty()})},
	{Name: "cleanup.list", Description: "Inspect read-only blocked cleanup candidates.", InputSchema: objectSchema(map[string]any{"projectId": stringProperty()})},
	{Name: "guidance.check", Description: "Run bounded guidance checks for one observed Worktree.", InputSchema: objectSchema(map[string]any{"projectId": stringProperty(), "repositoryId": stringProperty(), "worktreeId": stringProperty()})},
	{Name: "handoff.preview", Description: "Prepare a masked Agent Handoff preview without transcript collection or launch.", InputSchema: objectSchema(map[string]any{"profileId": stringProperty(), "projectId": stringProperty(), "repositoryId": stringProperty(), "worktreeId": stringProperty(), "model": stringProperty()})},
}

func Serve(ctx context.Context, input io.Reader, output io.Writer, service app.ApplicationService) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 1024), 256<<10)
	encoder := json.NewEncoder(output)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var item request
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			if err := encoder.Encode(response{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "invalid JSON-RPC request"}}); err != nil {
				return err
			}
			continue
		}
		if len(item.ID) == 0 && item.Method == "notifications/initialized" {
			continue
		}
		result, rpcErr := dispatch(ctx, service, item.Method, item.Params)
		if len(item.ID) == 0 {
			continue
		}
		if err := encoder.Encode(response{JSONRPC: "2.0", ID: item.ID, Result: result, Error: rpcErr}); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func dispatch(ctx context.Context, service app.ApplicationService, method string, params json.RawMessage) (any, *rpcError) {
	switch method {
	case "initialize":
		return map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]string{"name": "dev-control-room", "version": "0.5.0"}}, nil
	case "tools/list":
		return map[string]any{"tools": tools}, nil
	case "tools/call":
		var call struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(params, &call); err != nil || call.Name == "" {
			return nil, &rpcError{Code: -32602, Message: "tool name and arguments are required"}
		}
		value, err := callTool(ctx, service, call.Name, call.Arguments)
		if err != nil {
			classified := contract.Classify(err)
			data, _ := json.Marshal(contract.Failure[map[string]any](classified.Code, classified.Message, classified.Details))
			return map[string]any{"content": []map[string]string{{"type": "text", "text": string(data)}}, "isError": true}, nil
		}
		data, marshalErr := json.Marshal(contract.Success(value))
		if marshalErr != nil {
			return nil, &rpcError{Code: -32603, Message: "result could not be encoded"}
		}
		return map[string]any{"content": []map[string]string{{"type": "text", "text": string(data)}}, "isError": false}, nil
	default:
		return nil, &rpcError{Code: -32601, Message: "method not found"}
	}
}

func callTool(ctx context.Context, service app.ApplicationService, name string, arguments map[string]any) (any, error) {
	value := func(key string) string {
		if item, ok := arguments[key].(string); ok {
			return item
		}
		return ""
	}
	switch name {
	case "project.list":
		return service.Projects(ctx)
	case "finding.list":
		return service.Findings(ctx, value("projectId"), value("repositoryId"))
	case "cleanup.list":
		return service.CleanupCandidates(ctx, value("projectId"))
	case "guidance.check":
		return service.Guidance(ctx, value("projectId"), value("repositoryId"), value("worktreeId"))
	case "handoff.preview":
		return service.PrepareHandoff(ctx, app.HandoffInput{ProfileID: value("profileId"), ProjectID: value("projectId"), RepositoryID: value("repositoryId"), WorktreeID: value("worktreeId"), Model: value("model")})
	default:
		return nil, fmt.Errorf("unknown MCP tool %q", name)
	}
}

func objectSchema(properties map[string]any) map[string]any {
	if properties == nil {
		properties = map[string]any{}
	}
	return map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
}

func stringProperty() map[string]string { return map[string]string{"type": "string"} }
