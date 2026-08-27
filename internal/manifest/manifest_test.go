package manifest

import (
	"context"
	"testing"

	"github.com/abn/coprctl/internal/copr"
)

func TestParseAndValidate(t *testing.T) {
	data := []byte(`apiVersion: coprctl/v1
kind: Project
metadata:
  owner: quadzero
  name: aetherpak
spec:
  description: test
  chroots:
    enabled:
      - fedora-42-x86_64
  packages:
    - name: aetherpak
      source:
        type: scm
        cloneUrl: https://github.com/quadzero/aetherpak.git
`)
	m, err := Parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if m.Metadata.Name != "aetherpak" || len(m.Spec.Packages) != 1 {
		t.Fatalf("parsed manifest wrong: %+v", m)
	}
	if issues := m.Validate(); len(issues) != 0 {
		t.Fatalf("unexpected validation issues: %+v", issues)
	}
}

func TestValidateCatchesErrors(t *testing.T) {
	m, err := Parse([]byte(`apiVersion: coprctl/v1
kind: Project
metadata: {owner: o, name: n}
spec:
  packages:
    - name: p
      source:
        type: scm
`))
	if err != nil {
		t.Fatal(err)
	}
	issues := m.Validate()
	found := false
	for _, i := range issues {
		if i.Path == "spec.packages[0].source.cloneUrl" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected cloneUrl validation error, got %+v", issues)
	}
}

func TestParseMissingRequired(t *testing.T) {
	if _, err := Parse([]byte(`foo: bar`)); err == nil {
		t.Fatal("expected error for missing apiVersion/kind")
	}
}

func TestExportFromLive(t *testing.T) {
	srv := newManifestServer(t)
	c := copr.New(srv.URL, nil)
	m, err := ExportFromLive(context.Background(), c, "quadzero", "aetherpak")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if m.Spec.Description != "live desc" {
		t.Errorf("description = %q", m.Spec.Description)
	}
	if len(m.Spec.Packages) != 1 || m.Spec.Packages[0].Name != "pkgo" {
		t.Errorf("packages = %+v", m.Spec.Packages)
	}
}
