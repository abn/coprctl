package ref

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		opts    *Options
		want    Ref
		wantErr bool
	}{
		{"empty", "", nil, Ref{}, true},
		{"bare project", "aetherpak", nil, Ref{Kind: KindProject, Project: "aetherpak", Source: "aetherpak"}, false},
		{"bare name with dash", "my-proj", nil, Ref{Kind: KindProject, Project: "my-proj", Source: "my-proj"}, false},
		{"owner/project", "quadzero/aetherpak", nil, Ref{Kind: KindProject, Owner: "quadzero", Project: "aetherpak", Source: "quadzero/aetherpak"}, false},
		{"group owner", "@copr/copr-dev", nil, Ref{Kind: KindProject, Owner: "@copr", Project: "copr-dev", Source: "@copr/copr-dev"}, false},
		{"project with dir", "quadzero/aetherpak:testing", nil, Ref{Kind: KindProject, Owner: "quadzero", Project: "aetherpak", Dir: "testing", Source: "quadzero/aetherpak:testing"}, false},
		{"project with pr dir", "quadzero/aetherpak:pr:123", nil, Ref{Kind: KindProject, Owner: "quadzero", Project: "aetherpak", Dir: "pr:123", Source: "quadzero/aetherpak:pr:123"}, false},
		{"package", "quadzero/aetherpak/aetherpak-cli", nil, Ref{Kind: KindPackage, Owner: "quadzero", Project: "aetherpak", Segment: "aetherpak-cli", Source: "quadzero/aetherpak/aetherpak-cli"}, false},
		{"package force", "quadzero/aetherpak/epel-9-x86_64", &Options{ForcePackage: true}, Ref{Kind: KindPackage, Owner: "quadzero", Project: "aetherpak", Segment: "epel-9-x86_64", Source: "quadzero/aetherpak/epel-9-x86_64"}, false},
		{"project chroot by grammar", "quadzero/aetherpak/fedora-42-x86_64", nil, Ref{Kind: KindProjectChroot, Owner: "quadzero", Project: "aetherpak", Segment: "fedora-42-x86_64", Source: "quadzero/aetherpak/fedora-42-x86_64"}, false},
		{"project chroot force", "quadzero/aetherpak/mything", &Options{ForceChroot: true}, Ref{Kind: KindProjectChroot, Owner: "quadzero", Project: "aetherpak", Segment: "mything", Source: "quadzero/aetherpak/mything"}, false},
		{"project chroot via catalog", "quadzero/aetherpak/epel-9-x86_64", nil, Ref{Kind: KindProjectChroot, Owner: "quadzero", Project: "aetherpak", Segment: "epel-9-x86_64", Source: "quadzero/aetherpak/epel-9-x86_64"}, false},
		{"build id", "10653539", nil, Ref{Kind: KindBuild, BuildID: 10653539, Source: "10653539"}, false},
		{"build prefix", "build/10653539", nil, Ref{Kind: KindBuild, BuildID: 10653539, Source: "build/10653539"}, false},
		{"build chroot", "10653539/fedora-42-x86_64", nil, Ref{Kind: KindBuildChroot, BuildID: 10653539, BuildCht: "fedora-42-x86_64", Source: "10653539/fedora-42-x86_64"}, false},
		{"garbage", "!!!not a ref!!!", nil, Ref{}, true},
		{"just slash", "/", nil, Ref{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.in, tt.opts)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestChrootCatalogDisambiguation(t *testing.T) {
	// Install a catalog that knows epel-9-x86_64 but not a fake chroot.
	SetChrootCatalog(func(name string) bool {
		return name == "epel-9-x86_64"
	})
	defer SetChrootCatalog(nil)

	// A name that is in the catalog but does not match the grammar.
	// "epel-9-x86_64" matches the grammar too, so use a catalog-only name.
	SetChrootCatalog(func(name string) bool {
		return name == "special-chroot"
	})
	r, err := Parse("owner/proj/special-chroot", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Kind != KindProjectChroot {
		t.Fatalf("got kind %v, want KindProjectChroot", r.Kind)
	}
}
