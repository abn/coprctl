package secrets

import (
	"errors"
	"testing"
)

type fakeBackend struct {
	vals map[string]string
}

func (f *fakeBackend) Name() string     { return "fake" }
func (f *fakeBackend) Available() error { return nil }
func (f *fakeBackend) Get(k string) (string, error) {
	if f.vals == nil {
		f.vals = map[string]string{}
	}
	v, ok := f.vals[k]
	if !ok {
		return "", errors.New("not found")
	}
	return v, nil
}
func (f *fakeBackend) Set(k, v string) error {
	if f.vals == nil {
		f.vals = map[string]string{}
	}
	f.vals[k] = v
	return nil
}
func (f *fakeBackend) Delete(k string) error {
	delete(f.vals, k)
	return nil
}

func TestFakeBackendLifecycle(t *testing.T) {
	f := &fakeBackend{}
	if err := f.Set("copr/production/token", "sekrit"); err != nil {
		t.Fatal(err)
	}
	got, err := f.Get("copr/production/token")
	if err != nil || got != "sekrit" {
		t.Fatalf("get = %q, %v", got, err)
	}
	if err := f.Delete("copr/production/token"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Get("copr/production/token"); err == nil {
		t.Fatal("expected not found after delete")
	}
}

func TestDetectUnavailablePreference(t *testing.T) {
	// A preference for a backend that is not installed must return nil, not
	// silently fall back to another handler.
	if be := Detect("definitely-not-a-backend"); be != nil {
		t.Fatalf("expected nil for unknown preferred backend, got %s", be.Name())
	}
}
