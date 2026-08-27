// Package mcp implements the optional stdio MCP server exposing the command
// surface as tiered tools. Read-only by default; write and destructive tiers
// are opt-in via flags.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// Tool is an MCP tool definition.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	Tier        string         `json:"tier,omitempty"`
}

// Registry provides the tool list for the server.
type Registry interface {
	Tools(tier string) []Tool
}

// Server is a minimal stdio JSON-RPC MCP server.
type Server struct {
	In   io.Reader
	Out  io.Writer
	Reg  Registry
	Tier string
	// Call executes a tool by name with string args, returning stdout.
	Call func(name string, args []string) (string, error)
}

// Serve runs the server until EOF on stdin.
func (s *Server) Serve() error {
	sc := bufio.NewScanner(s.In)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}
		s.handle(req)
	}
	return sc.Err()
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *Server) respond(id json.RawMessage, result any) {
	_ = json.NewEncoder(s.Out).Encode(response{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *Server) respondError(id json.RawMessage, code int, msg string) {
	_ = json.NewEncoder(s.Out).Encode(response{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}})
}

func (s *Server) handle(req request) {
	isNotification := len(req.ID) == 0 || string(req.ID) == "null"
	switch req.Method {
	case "initialize":
		s.respond(req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "coprctl", "version": "0.1.0"},
		})
	case "notifications/initialized":
		// No response expected; nothing to do.
	case "tools/list":
		if s.Reg == nil {
			s.respond(req.ID, map[string]any{"tools": []Tool{}})
			return
		}
		s.respond(req.ID, map[string]any{"tools": s.Reg.Tools(s.Tier)})
	case "tools/call":
		s.handleCall(req)
	default:
		if isNotification {
			return // never respond to a notification
		}
		s.respondError(req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method))
	}
}

func (s *Server) handleCall(req request) {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	_ = json.Unmarshal(req.Params, &params)
	if params.Name == "" {
		s.respondError(req.ID, -32602, "missing tool name")
		return
	}
	if s.Call == nil {
		s.respondError(req.ID, -32602, fmt.Sprintf("tool %q not executable in this mode", params.Name))
		return
	}
	// Convert arguments (a map) to positional string args in stable order.
	args := mapArgsToStrings(params.Arguments)
	out, err := s.Call(params.Name, args)
	if err != nil {
		s.respondError(req.ID, -32000, fmt.Sprintf("%s: %v", params.Name, err))
		return
	}
	s.respond(req.ID, map[string]any{
		"content": []map[string]string{{"type": "text", "text": out}},
	})
}

func mapArgsToStrings(args map[string]any) []string {
	// Simple deterministic conversion: key=value pairs.
	out := make([]string, 0, len(args))
	for k, v := range args {
		out = append(out, fmt.Sprintf("--%s=%v", k, v))
	}
	return out
}
