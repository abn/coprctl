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
		{[]string{"whoami"}, "auth status"},
		{[]string{"list-permissions", "quadzero/aetherpak"}, "project permission list quadzero/aetherpak"},
		{[]string{"download-build", "123"}, "build download 123"},
		{[]string{"mock-config", "quadzero/aetherpak", "fedora-42-x86_64"}, "project chroot get quadzero/aetherpak fedora-42-x86_64"},
		{[]string{"new-webhook-secret", "quadzero/aetherpak"}, "integration rotate-secret quadzero/aetherpak"},
		{[]string{"build", "quadzero/aetherpak", "aetherpak-0.4.1-1.src.rpm"}, "build submit quadzero/aetherpak --source upload --upload aetherpak-0.4.1-1.src.rpm"},
		{[]string{"build", "quadzero/aetherpak", "--nowait", "pkg-1.0.src.rpm"}, "build submit quadzero/aetherpak --source upload --upload pkg-1.0.src.rpm --nowait"},
		{[]string{"build", "quadzero/aetherpak", "--chroot", "fedora-42-x86_64"}, "build submit quadzero/aetherpak --chroot fedora-42-x86_64"},
		{[]string{"uploadrpm", "quadzero/aetherpak", "pkg-1.0.src.rpm"}, "build submit quadzero/aetherpak --source upload --upload pkg-1.0.src.rpm"},
		{[]string{"build", "quadzero/aetherpak", "a.src.rpm", "b.src.rpm"}, ""},
		{[]string{"build", "quadzero/aetherpak", "pkg-1.0.x86_64.rpm"}, ""},
		{[]string{"build", "--nowait", "quadzero/aetherpak", "pkg-1.0.src.rpm"}, ""},
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
