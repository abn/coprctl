// Package runtime abstracts the container runtime (podman or docker) for the
// local preflight feature. Detection order: --runtime flag, then COPRCTL_RUNTIME,
// then podman, then docker, then fail-soft.
package runtime

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	stdruntime "runtime"
	"strings"
)

// Runtime runs container operations.
type Runtime interface {
	// Name returns the runtime name (podman or docker).
	Name() string
	// Rootless reports whether the runtime runs rootless.
	Rootless() bool
	// Available returns nil if the runtime is usable.
	Available() error
	// Run executes a container image and streams combined output.
	Run(ctx context.Context, spec RunSpec) error
	// Build builds an image from a Containerfile.
	Build(ctx context.Context, spec BuildSpec) error
}

// RunSpec describes a container run.
type RunSpec struct {
	Image    string
	WorkDir  string // host directory mounted as the working directory
	Mount    string // container mount target (default /work)
	Env      []string
	Args     []string // command and arguments inside the container
	Platform string   // optional --platform
	Network  string   // "", "none", "host"
	Stdout   io.Writer
}

// BuildSpec describes a container image build.
type BuildSpec struct {
	Context string
	File    string
	Tag     string
}

// cliRuntime is a generic runtime backed by a CLI binary.
type cliRuntime struct {
	bin      string
	rootless bool
}

func (r *cliRuntime) Name() string   { return r.bin }
func (r *cliRuntime) Rootless() bool { return r.rootless }

func (r *cliRuntime) Available() error {
	if _, err := exec.LookPath(r.bin); err != nil {
		return fmt.Errorf("%s not found", r.bin)
	}
	return nil
}

func (r *cliRuntime) Run(ctx context.Context, spec RunSpec) error {
	args := []string{"run", "--rm"}
	if spec.Platform != "" {
		args = append(args, "--platform", spec.Platform)
	}
	if spec.Network == "none" {
		args = append(args, "--network", "none")
	}
	if spec.WorkDir != "" {
		mount := spec.Mount
		if mount == "" {
			mount = "/work"
		}
		args = append(args, "-v", mountArgAt(spec.WorkDir, mount), "-w", mount)
	}
	for _, e := range spec.Env {
		args = append(args, "-e", e)
	}
	args = append(args, spec.Image)
	args = append(args, spec.Args...)

	cmd := exec.CommandContext(ctx, r.bin, args...)
	cmd.Stdout = spec.Stdout
	cmd.Stderr = spec.Stdout
	return cmd.Run()
}

func (r *cliRuntime) Build(ctx context.Context, spec BuildSpec) error {
	args := []string{"build", "-f", spec.File, "-t", spec.Tag, spec.Context}
	cmd := exec.CommandContext(ctx, r.bin, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("image build failed: %w: %s", err, buf.String())
	}
	return nil
}

// mountArgAt builds a host:container mount argument. The host path is
// slash-normalized (filepath.ToSlash) and the SELinux :z relabel applies only
// on Linux.
func mountArgAt(hostDir, mount string) string {
	host := filepath.ToSlash(hostDir)
	label := ""
	if stdruntime.GOOS == "linux" {
		label = ":z"
	}
	return host + ":" + mount + label
}

// Detect returns the best available runtime.
func Detect(prefer string) (Runtime, error) {
	name := prefer
	if name == "" {
		name = strings.ToLower(strings.TrimSpace(os.Getenv("COPRCTL_RUNTIME")))
	}
	candidates := []string{}
	switch name {
	case "podman":
		candidates = []string{"podman"}
	case "docker":
		candidates = []string{"docker"}
	default:
		candidates = []string{"podman", "docker"}
	}
	for _, c := range candidates {
		r := &cliRuntime{bin: c, rootless: c == "podman"}
		if r.Available() == nil {
			return r, nil
		}
	}
	return nil, fmt.Errorf("no container runtime available (podman or docker required for preflight)")
}
