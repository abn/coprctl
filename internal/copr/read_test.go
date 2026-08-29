package copr

import (
	"encoding/json"
	"testing"
)

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
