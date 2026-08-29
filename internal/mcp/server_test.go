package mcp

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

type fakeReg struct{}

func (f fakeReg) Tools(tier string) []Tool {
	return []Tool{{Name: "coprctl_project", Description: "manage projects", Tier: "read"}}
}

type errWriter struct{}

func (errWriter) Write(p []byte) (int, error) { return 0, errors.New("broken pipe") }

func TestInitializeAndToolsList(t *testing.T) {
	var in bytes.Buffer
	var out bytes.Buffer
	in.WriteString(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n")
	in.WriteString(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n")
	srv := &Server{In: &in, Out: &out, Reg: fakeReg{}, Tier: "read"}
	if err := srv.Serve(); err != nil {
		t.Fatalf("serve: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"protocolVersion"`) {
		t.Errorf("initialize response missing protocolVersion: %s", got)
	}
	if !strings.Contains(got, `"coprctl_project"`) {
		t.Errorf("tools/list response missing tool: %s", got)
	}
}

func TestUnknownMethod(t *testing.T) {
	var in bytes.Buffer
	var out bytes.Buffer
	in.WriteString(`{"jsonrpc":"2.0","id":3,"method":"bogus","params":{}}` + "\n")
	srv := &Server{In: &in, Out: &out, Tier: "read"}
	_ = srv.Serve()
	if !strings.Contains(out.String(), "method not found") {
		t.Errorf("expected method-not-found error, got: %s", out.String())
	}
}

func TestToolsCallExecutes(t *testing.T) {
	var in bytes.Buffer
	var out bytes.Buffer
	in.WriteString(`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"echo","arguments":{}}}` + "\n")
	srv := &Server{
		In:   &in,
		Out:  &out,
		Tier: "read",
		Call: func(name string, args []string) (string, error) {
			if name == "echo" {
				return "hello world\n", nil
			}
			return "", fmt.Errorf("unknown")
		},
	}
	_ = srv.Serve()
	if !strings.Contains(out.String(), "hello world") {
		t.Errorf("expected tool result, got: %s", out.String())
	}
}

func TestCallError(t *testing.T) {
	var in bytes.Buffer
	var out bytes.Buffer
	in.WriteString(`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"boom","arguments":{}}}` + "\n")
	srv := &Server{
		In:   &in,
		Out:  &out,
		Tier: "read",
		Call: func(name string, args []string) (string, error) {
			return "", fmt.Errorf("failed to run")
		},
	}
	_ = srv.Serve()
	if !strings.Contains(out.String(), "failed to run") {
		t.Errorf("expected call error, got: %s", out.String())
	}
}

func TestServePropagatesWriteError(t *testing.T) {
	var in bytes.Buffer
	in.WriteString(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n")
	srv := &Server{In: &in, Out: errWriter{}, Reg: fakeReg{}, Tier: "read"}
	err := srv.Serve()
	if err == nil {
		t.Fatal("expected Serve to propagate the write error")
	}
	if !strings.Contains(err.Error(), "broken pipe") {
		t.Errorf("error = %v, want broken pipe", err)
	}
}
