package logdetective

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExplainStructured(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/123/fedora-44-x86_64" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"summary": "missing golang build dependency", "suggestion": "add golang to BuildRequires"}`))
	}))
	defer srv.Close()
	c := New()
	c.HTTP = srv.Client()
	c.baseURL = srv.URL
	expl, err := c.Explain(context.Background(), ExplainRequest{BuildID: 123, Chroot: "fedora-44-x86_64"})
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	if expl.Summary != "missing golang build dependency" {
		t.Errorf("summary = %q", expl.Summary)
	}
}

func TestExplainPlainText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("plain explanation"))
	}))
	defer srv.Close()
	c := New()
	c.HTTP = srv.Client()
	c.baseURL = srv.URL
	expl, err := c.Explain(context.Background(), ExplainRequest{BuildID: 1, Chroot: "c"})
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	if expl.Summary != "plain explanation" {
		t.Errorf("summary = %q", expl.Summary)
	}
}

func TestExplainUnreachable(t *testing.T) {
	c := New()
	_, err := c.Explain(context.Background(), ExplainRequest{BuildID: 1, Chroot: "c"})
	if err == nil {
		t.Fatal("expected error for unreachable service")
	}
}
