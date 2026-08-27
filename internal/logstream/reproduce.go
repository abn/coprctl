package logstream

import (
	"context"
	"fmt"
	"strings"
)

// Reproduction is the extracted local reproduction recipe from a build log.
type Reproduction struct {
	BuildID int    `json:"build_id"`
	Chroot  string `json:"chroot"`
	// Recipe is the copr-rpmbuild invocation Copr wrote into the log.
	Recipe string `json:"recipe,omitempty"`
	// TaskURL is the task-url argument, if present.
	TaskURL string `json:"task_url,omitempty"`
	// Mock is whether the recipe uses copr-rpmbuild (mock-level fidelity).
	Mock bool `json:"mock,omitempty"`
}

// ExtractReproduction finds the copr-rpmbuild reproduction recipe in a
// build-chroot log. The recipe is how a build can be reproduced at mock-level
// fidelity with the exact task Copr ran.
func (t *Tailer) ExtractReproduction(ctx context.Context, buildID int, chroot string) (*Reproduction, error) {
	bc, err := t.findBuildChroot(ctx, buildID, chroot)
	if err != nil {
		return nil, err
	}
	url := strings.TrimRight(bc.ResultURL, "/") + "/builder-live.log.gz"
	lines, err := fetchLines(ctx, t.Client.HTTP, url)
	if err != nil {
		return nil, err
	}
	rep := &Reproduction{BuildID: buildID, Chroot: chroot, Mock: true}
	for _, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		if strings.Contains(trimmed, "copr-rpmbuild") && strings.Contains(trimmed, "--task-url") {
			rep.Recipe = trimmed
			if i := strings.Index(trimmed, "--task-url "); i > 0 {
				rest := trimmed[i+len("--task-url "):]
				fields := strings.Fields(rest)
				if len(fields) > 0 {
					rep.TaskURL = fields[0]
				}
			}
			return rep, nil
		}
	}
	return rep, fmt.Errorf("no copr-rpmbuild reproduction recipe found in the log for %s", chroot)
}

func (t *Tailer) findBuildChroot(ctx context.Context, buildID int, chroot string) (*BuildChrootRef, error) {
	chroots, err := t.Client.ListBuildChroots(ctx, buildID)
	if err != nil {
		return nil, err
	}
	for _, bc := range chroots {
		if bc.Chroot == chroot {
			return &BuildChrootRef{ResultURL: bc.ResultURL, State: bc.State}, nil
		}
	}
	return nil, fmt.Errorf("no build chroot %q for build %d", chroot, buildID)
}

// BuildChrootRef is a minimal reference used by reproduce.
type BuildChrootRef struct {
	ResultURL string
	State     string
}
