package cli

import "testing"

func TestMigrateArgs(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"list", "quadzero"}, "project list quadzero"},
		{[]string{"create", "quadzero/aetherpak", "--chroot", "fedora-42-x86_64"}, "project create quadzero/aetherpak --chroot fedora-42-x86_64"},
		{[]string{"buildscm", "quadzero/aetherpak", "--clone-url", "https://example.com"}, "build submit --source scm quadzero/aetherpak --clone-url https://example.com"},
		{[]string{"list-packages", "quadzero/aetherpak"}, "package list quadzero/aetherpak"},
		{[]string{"whoami"}, "auth whoami"},
		{[]string{"new-webhook-secret", "quadzero/aetherpak"}, "integration rotate-secret quadzero/aetherpak"},
		{[]string{"nonsense-verb"}, ""},
	}
	for _, tt := range tests {
		got, err := migrateArgs(tt.args)
		if tt.want == "" {
			if err == nil {
				t.Errorf("migrateArgs(%v) expected error", tt.args)
			}
			continue
		}
		if err != nil {
			t.Errorf("migrateArgs(%v) error: %v", tt.args, err)
			continue
		}
		if got != tt.want {
			t.Errorf("migrateArgs(%v) = %q, want %q", tt.args, got, tt.want)
		}
	}
}
