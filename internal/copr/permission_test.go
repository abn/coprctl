package copr

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestPermissionStateValid(t *testing.T) {
	for _, s := range []PermissionState{PermissionNothing, PermissionRequest, PermissionApproved} {
		if !s.Valid() {
			t.Errorf("%q should be a valid state", s)
		}
	}
	for _, s := range []PermissionState{"", "denied", "granted"} {
		if s.Valid() {
			t.Errorf("%q should be invalid", s)
		}
	}
}

func TestListPermissions(t *testing.T) {
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api_3/project/permissions/get/owner/proj" {
			t.Errorf("path = %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"permissions": map[string]any{
				"alice": map[string]any{"admin": "approved", "builder": "approved"},
				"bob":   map[string]any{"admin": "nothing", "builder": "request"},
			},
		})
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	perms, err := c.ListPermissions(context.Background(), "owner", "proj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(perms) != 2 {
		t.Fatalf("got %d perms, want 2", len(perms))
	}
	if perms["alice"].Admin != PermissionApproved || perms["alice"].Builder != PermissionApproved {
		t.Errorf("alice = %+v", perms["alice"])
	}
	if perms["bob"].Admin != PermissionNothing || perms["bob"].Builder != PermissionRequest {
		t.Errorf("bob = %+v", perms["bob"])
	}
}

func TestPermissionSetBodyOmitsEmptyStates(t *testing.T) {
	body := permissionSetBody(Permissions{
		"alice": {Admin: PermissionApproved},
		"bob":   {Admin: PermissionNothing, Builder: PermissionRequest},
		"carol": {},
	})
	if len(body) != 2 {
		t.Fatalf("body = %v, want 2 users", body)
	}
	if body["alice"]["admin"] != PermissionApproved {
		t.Errorf("alice = %v", body["alice"])
	}
	if _, ok := body["alice"]["builder"]; ok {
		t.Errorf("alice builder should be omitted: %v", body["alice"])
	}
	if body["bob"]["admin"] != PermissionNothing || body["bob"]["builder"] != PermissionRequest {
		t.Errorf("bob = %v", body["bob"])
	}
	if _, ok := body["carol"]; ok {
		t.Errorf("carol with no roles should be omitted: %v", body)
	}
}

func TestSetPermissions(t *testing.T) {
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if r.URL.Path != "/api_3/project/permissions/set/owner/proj" {
			t.Errorf("path = %s", r.URL.Path)
		}
		var body map[string]map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["alice"]["admin"] != "approved" {
			t.Errorf("body = %v", body)
		}
		if _, ok := body["alice"]["builder"]; ok {
			t.Errorf("builder should be omitted, body = %v", body)
		}
		json.NewEncoder(w).Encode(map[string]any{"updated": []string{"alice"}})
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	updated, err := c.SetPermissions(context.Background(), "owner", "proj", Permissions{
		"alice": {Admin: PermissionApproved},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updated) != 1 || updated[0] != "alice" {
		t.Errorf("updated = %v", updated)
	}
}

func TestRequestPermissions(t *testing.T) {
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if r.URL.Path != "/api_3/project/permissions/request/owner/proj" {
			t.Errorf("path = %s", r.URL.Path)
		}
		var body map[string]bool
		json.NewDecoder(r.Body).Decode(&body)
		if body["admin"] != true || body["builder"] != false {
			t.Errorf("body = %v", body)
		}
		json.NewEncoder(w).Encode(map[string]any{"updated": true})
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	updated, err := c.RequestPermissions(context.Background(), "owner", "proj", true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated {
		t.Errorf("updated = %v, want true", updated)
	}
}

func TestCanBuildIn(t *testing.T) {
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api_3/project/permissions/can_build_in/alice/owner/proj" {
			t.Errorf("path = %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{"can_build_in": true})
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	can, err := c.CanBuildIn(context.Background(), "alice", "owner", "proj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !can {
		t.Errorf("can_build_in = %v, want true", can)
	}
}
