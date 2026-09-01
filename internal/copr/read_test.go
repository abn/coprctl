package copr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return raw
}

func TestProjectDecodeModernFields(t *testing.T) {
	var p Project
	raw := []byte(`{
		"id": 1, "name": "aetherpak", "ownername": "quadzero",
		"persistent": true, "auto_prune": false, "bootstrap": "on",
		"isolation": "nspawn", "module_hotfixes": true, "appstream": true,
		"packit_forge_projects_allowed": ["github.com/quadzero/aetherpak"],
		"follow_fedora_branching": false, "repo_priority": 42,
		"storage": "pulp", "unlisted_on_hp": true
	}`)
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p.Name != "aetherpak" || p.Ownername != "quadzero" {
		t.Errorf("identity = %+v", p)
	}
	if !p.Persistent || p.AutoPrune {
		t.Errorf("persistent/auto_prune = %v/%v", p.Persistent, p.AutoPrune)
	}
	if p.Bootstrap != "on" || p.Isolation != "nspawn" {
		t.Errorf("bootstrap/isolation = %q/%q", p.Bootstrap, p.Isolation)
	}
	if !p.ModuleHotfixes || !p.Appstream {
		t.Errorf("module_hotfixes/appstream = %v/%v", p.ModuleHotfixes, p.Appstream)
	}
	if len(p.PackitForgeProjectsAllowed) != 1 || p.PackitForgeProjectsAllowed[0] != "github.com/quadzero/aetherpak" {
		t.Errorf("packit_forge_projects_allowed = %v", p.PackitForgeProjectsAllowed)
	}
	if p.FollowFedoraBranching {
		t.Errorf("follow_fedora_branching = %v", p.FollowFedoraBranching)
	}
	if p.RepoPriority != 42 || p.Storage != "pulp" || !p.UnlistedOnHomepage {
		t.Errorf("repo_priority/storage/unlisted_on_hp = %v/%q/%v", p.RepoPriority, p.Storage, p.UnlistedOnHomepage)
	}
}

func TestBuildDecodeWireShape(t *testing.T) {
	var b Build
	if err := json.Unmarshal(readFixture(t, "testdata/build-2926020.json"), &b); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if b.SourcePackage.Name != "" || b.SourcePackage.Version != "" || b.SourcePackage.URL != "" {
		t.Errorf("source_package = %+v, want the always-present value struct with null members", b.SourcePackage)
	}
	if b.State != "starting" {
		t.Errorf("state = %q, want starting", b.State)
	}
	if len(b.Chroots) != 0 {
		t.Errorf("chroots = %v, want empty before the build starts", b.Chroots)
	}
	if b.Builds != nil {
		t.Errorf("builds = %v, want nil on a bare build", b.Builds)
	}
	if got := b.PackageName(); got != "" {
		t.Errorf("PackageName() = %q, want empty", got)
	}
	if got := b.ChrootStates(); len(got) != 0 {
		t.Errorf("ChrootStates() = %v, want empty", got)
	}
	if got := b.RollupState(); got != "starting" {
		t.Errorf("RollupState() = %q, want starting", got)
	}
}

func TestBuildDecodeSourcePackage(t *testing.T) {
	var b Build
	if err := json.Unmarshal(readFixture(t, "testdata/build-source-package.json"), &b); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := b.PackageName(); got != "hello" {
		t.Errorf("PackageName() = %q, want hello", got)
	}
}

func TestBuildDecodeRunningChrootsFallback(t *testing.T) {
	var b Build
	if err := json.Unmarshal(readFixture(t, "testdata/build-running-chroots.json"), &b); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if b.State != "running" {
		t.Errorf("state = %q, want running", b.State)
	}
	m := b.ChrootStates()
	for _, name := range b.Chroots {
		if m[name] != "running" {
			t.Errorf("ChrootStates()[%q] = %q, want fallback to the build state running", name, m[name])
		}
	}
}

func TestMonitorDecodeEnriched(t *testing.T) {
	// monitor-enriched.json is the staging capture for build 2926024,
	// reformatted from the raw response for readability.
	var env MonitorEnvelope
	if err := json.Unmarshal(readFixture(t, "testdata/monitor-enriched.json"), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Output != "ok" || !strings.Contains(env.Message, "successful") {
		t.Errorf("envelope = %+v, want the ok envelope", env)
	}
	if len(env.Packages) != 1 {
		t.Fatalf("packages = %d, want 1", len(env.Packages))
	}
	info, ok := env.Packages[0].Chroots["fedora-rawhide-x86_64"]
	if !ok {
		t.Fatalf("missing chroot fedora-rawhide-x86_64 in %v", env.Packages[0].Chroots)
	}
	if info.Status != 1 {
		t.Errorf("status = %d, want 1", info.Status)
	}
	if info.State != "succeeded" || info.BuildID != 2926024 || info.PkgVersion != "2.10-1" {
		t.Errorf("info = %+v, want the succeeded state row", info)
	}
	wantLog := "https://download.copr-dev.fedorainfracloud.org/results/devnullcake/hello-rpm/fedora-rawhide-x86_64/02926024-hello/builder-live.log.gz"
	if info.URLBuildLog != wantLog {
		t.Errorf("url_build_log = %q, want %q", info.URLBuildLog, wantLog)
	}
	wantBackend := "https://download.copr-dev.fedorainfracloud.org/results/devnullcake/hello-rpm/fedora-rawhide-x86_64/02926024-hello/backend.log.gz"
	if info.URLBackendLog != wantBackend {
		t.Errorf("url_backend_log = %q, want %q", info.URLBackendLog, wantBackend)
	}
	// url_build is never requested or emitted upstream, so an empty string is
	// the forward-compatible decode.
	if info.URLBuild != "" {
		t.Errorf("url_build = %q, want empty", info.URLBuild)
	}
}

func TestMonitorDecodeAbsentURLFields(t *testing.T) {
	// A chroot without a result_dir emits null URL keys. status must survive a
	// zero value: 0 is failed, a real state.
	var env MonitorEnvelope
	raw := `{"packages":[{"name":"hello","chroots":{"fedora-rawhide-x86_64":{"state":"failed","status":0,"build_id":2926024,"url_build_log":null,"url_backend_log":null,"pkg_version":"2.10-1"}}}]}`
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	info := env.Packages[0].Chroots["fedora-rawhide-x86_64"]
	if info.Status != 0 {
		t.Errorf("status = %d, want 0 preserved", info.Status)
	}
	if info.State != "failed" {
		t.Errorf("state = %q, want failed", info.State)
	}
	if info.URLBuildLog != "" || info.URLBackendLog != "" || info.URLBuild != "" {
		t.Errorf("url fields = %q/%q/%q, want empty", info.URLBuildLog, info.URLBackendLog, info.URLBuild)
	}
}

func TestTimestampUnmarshalJSON(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int64
	}{
		{"rfc3339", `"2023-11-14T22:13:20Z"`, 1700000000},
		{"rfc3339 with offset", `"2023-11-15T00:13:20+02:00"`, 1700000000},
		{"fractional seconds", `"2023-11-14T22:13:20.123Z"`, 1700000000},
		{"fractional seconds with offset", `"2023-11-15T00:13:20.123+02:00"`, 1700000000},
		{"offset without colon", `"2023-11-15T00:13:20+0200"`, 1700000000},
		{"unix epoch", "1700000000", 1700000000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var ts Timestamp
			if err := json.Unmarshal([]byte(tc.in), &ts); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !ts.IsSet || ts.Unix != tc.want {
				t.Fatalf("got Unix=%d IsSet=%v, want %d", ts.Unix, ts.IsSet, tc.want)
			}
		})
	}
}

func TestTimestampUnmarshalJSONInvalid(t *testing.T) {
	cases := []string{
		`"not-a-timestamp"`,
		`"2023-13-45T99:00:00Z"`,
		`"2023-11-14"`,
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			var ts Timestamp
			if err := json.Unmarshal([]byte(in), &ts); err == nil {
				t.Fatalf("expected error for %s, got Unix=%d IsSet=%v", in, ts.Unix, ts.IsSet)
			}
		})
	}
}

func TestTimestampNull(t *testing.T) {
	var ts Timestamp
	if err := json.Unmarshal([]byte("null"), &ts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ts.IsSet {
		t.Fatal("null timestamp should not be set")
	}
	if !ts.Time().IsZero() {
		t.Fatalf("null timestamp should be zero time, got %v", ts.Time())
	}
}

func TestGetSourceBuildConfig(t *testing.T) {
	// Captured live response from the staging instance for a scm build; the
	// stored dict carries srpm_build_method (not source_build_method).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api_3/build/source-build-config/2926020" {
			t.Errorf("path = %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"source_type": "scm",
			"source_dict": map[string]any{
				"type":              "git",
				"clone_url":         "https://github.com/abn/hello-rpm",
				"committish":        "master",
				"subdirectory":      nil,
				"spec":              "hello.spec",
				"srpm_build_method": "rpkg",
			},
			"memory_limit":  2048,
			"timeout":       18000,
			"is_background": false,
		})
	}))
	defer srv.Close()
	c := New(srv.URL, nil)
	cfg, err := c.GetSourceBuildConfig(context.Background(), 2926020)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SourceType != "scm" {
		t.Errorf("source_type = %q", cfg.SourceType)
	}
	if cfg.SourceDict["srpm_build_method"] != "rpkg" {
		t.Errorf("srpm_build_method = %v (must use the stored key)", cfg.SourceDict["srpm_build_method"])
	}
	if _, ok := cfg.SourceDict["source_build_method"]; ok {
		t.Errorf("source_dict must not carry the submit-time key source_build_method")
	}
	if cfg.SourceDict["clone_url"] != "https://github.com/abn/hello-rpm" {
		t.Errorf("clone_url = %v", cfg.SourceDict["clone_url"])
	}
	if cfg.MemoryLimit == nil || *cfg.MemoryLimit != 2048 {
		t.Errorf("memory_limit = %v", cfg.MemoryLimit)
	}
	if cfg.Timeout == nil || *cfg.Timeout != 18000 {
		t.Errorf("timeout = %v", cfg.Timeout)
	}
	if cfg.IsBackground {
		t.Errorf("is_background = true, want false")
	}
}
