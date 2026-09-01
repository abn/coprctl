package copr

import (
	"encoding/json"
	"os"
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
