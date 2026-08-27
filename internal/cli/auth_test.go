package cli

import (
	"testing"
	"time"
)

func TestParseExpiry(t *testing.T) {
	// Date-only.
	got, err := parseExpiry("2027-02-23")
	if err != nil {
		t.Fatalf("date-only parse: %v", err)
	}
	if got.Year() != 2027 || got.Month() != 2 || got.Day() != 23 {
		t.Errorf("date-only = %v", got)
	}
	// Full RFC3339.
	got2, err := parseExpiry("2027-02-23T00:00:00Z")
	if err != nil {
		t.Fatalf("rfc3339 parse: %v", err)
	}
	if got2.Year() != 2027 {
		t.Errorf("rfc3339 = %v", got2)
	}
	// Invalid.
	if _, err := parseExpiry("garbage"); err == nil {
		t.Fatal("expected error for garbage expiry")
	}
}

func TestRoundDuration(t *testing.T) {
	if got := roundDuration(24*time.Hour + 2*time.Hour); got != "1d 2h" {
		t.Errorf("roundDuration = %q", got)
	}
	if got := roundDuration(-time.Hour); got != "expired" {
		t.Errorf("negative = %q", got)
	}
}
