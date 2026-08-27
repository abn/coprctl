package cerr

import (
	"errors"
	"testing"
)

func TestExitCodesStable(t *testing.T) {
	want := map[string]int{
		"OK":           0,
		"Generic":      1,
		"Usage":        2,
		"NoConfig":     3,
		"BuildFailed":  4,
		"Transport":    5,
		"Config":       6,
		"Auth":         7,
		"NotFound":     8,
		"Permission":   9,
		"Conflict":     10,
		"Timeout":      11,
		"Drift":        12,
		"Precondition": 13,
		"Interrupted":  130,
	}
	for name, wantCode := range want {
		switch name {
		case "OK":
			if ExitOK != wantCode {
				t.Errorf("ExitOK = %d, want %d", ExitOK, wantCode)
			}
		case "Generic":
			if ExitGeneric != wantCode {
				t.Errorf("ExitGeneric = %d, want %d", ExitGeneric, wantCode)
			}
		case "Usage":
			if ExitUsage != wantCode {
				t.Errorf("ExitUsage = %d, want %d", ExitUsage, wantCode)
			}
		case "NoConfig":
			if ExitNoConfig != wantCode {
				t.Errorf("ExitNoConfig = %d, want %d", ExitNoConfig, wantCode)
			}
		case "BuildFailed":
			if ExitBuildFailed != wantCode {
				t.Errorf("ExitBuildFailed = %d, want %d", ExitBuildFailed, wantCode)
			}
		case "Transport":
			if ExitTransport != wantCode {
				t.Errorf("ExitTransport = %d, want %d", ExitTransport, wantCode)
			}
		case "Config":
			if ExitConfig != wantCode {
				t.Errorf("ExitConfig = %d, want %d", ExitConfig, wantCode)
			}
		case "Auth":
			if ExitAuth != wantCode {
				t.Errorf("ExitAuth = %d, want %d", ExitAuth, wantCode)
			}
		case "NotFound":
			if ExitNotFound != wantCode {
				t.Errorf("ExitNotFound = %d, want %d", ExitNotFound, wantCode)
			}
		case "Permission":
			if ExitPermission != wantCode {
				t.Errorf("ExitPermission = %d, want %d", ExitPermission, wantCode)
			}
		case "Conflict":
			if ExitConflict != wantCode {
				t.Errorf("ExitConflict = %d, want %d", ExitConflict, wantCode)
			}
		case "Timeout":
			if ExitTimeout != wantCode {
				t.Errorf("ExitTimeout = %d, want %d", ExitTimeout, wantCode)
			}
		case "Drift":
			if ExitDrift != wantCode {
				t.Errorf("ExitDrift = %d, want %d", ExitDrift, wantCode)
			}
		case "Precondition":
			if ExitPrecondition != wantCode {
				t.Errorf("ExitPrecondition = %d, want %d", ExitPrecondition, wantCode)
			}
		case "Interrupted":
			if ExitInterrupted != wantCode {
				t.Errorf("ExitInterrupted = %d, want %d", ExitInterrupted, wantCode)
			}
		}
	}
}

func TestErrorWrapAndUnwrap(t *testing.T) {
	underlying := errors.New("boom")
	e := New("x", ExitGeneric, "failed").Wrap(underlying)
	if !errors.Is(e, underlying) {
		t.Fatalf("expected errors.Is(e, underlying) to be true")
	}
	if got := e.Error(); got != "failed: boom" {
		t.Fatalf("Error() = %q, want %q", got, "failed: boom")
	}
}

func TestErrorRenderers(t *testing.T) {
	if e := Usage("bad args"); e.ExitCode != ExitUsage || e.Code != "usage_error" {
		t.Fatalf("Usage() wrong: %+v", e)
	}
	if e := NotFound("a/b"); e.ExitCode != ExitNotFound || e.Resource != "a/b" {
		t.Fatalf("NotFound() wrong: %+v", e)
	}
	if e := Auth("denied"); e.ExitCode != ExitAuth {
		t.Fatalf("Auth() wrong: %+v", e)
	}
}
