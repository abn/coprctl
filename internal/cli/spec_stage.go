package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

// prepareSpecDir returns a build directory containing a sanitized spec and the
// surrounding sources. When the spec has no rpmbuild-breaking annotations, it
// returns the original directory unchanged. Otherwise it stages a copy in a
// temp dir with the annotation stripped, so release-tooling markers such as
// `# x-release-please-version` never reach rpmbuild while the on-disk spec is
// left untouched. The caller must remove the returned dir when non-empty.
func prepareSpecDir(spec string) (string, error) {
	data, err := os.ReadFile(spec)
	if err != nil {
		return "", err
	}
	hasAnnotation := false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, "# x-release-please-version") {
			hasAnnotation = true
			break
		}
	}
	if !hasAnnotation {
		return filepath.Dir(spec), nil
	}

	src := filepath.Dir(spec)
	stage, err := os.MkdirTemp("", "coprctl-srpm-*")
	if err != nil {
		return "", err
	}
	// Copy every entry except the spec files; the sanitized spec replaces them.
	entries, err := os.ReadDir(src)
	if err != nil {
		os.RemoveAll(stage)
		return "", err
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".spec") {
			continue
		}
		if err := copyTree(filepath.Join(src, e.Name()), filepath.Join(stage, e.Name())); err != nil {
			os.RemoveAll(stage)
			return "", err
		}
	}
	// Write the sanitized spec (strip the release-please annotation).
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "# x-release-please-version"); idx >= 0 {
			lines[i] = strings.TrimSpace(line[:idx])
		}
	}
	specName := filepath.Base(spec)
	if err := os.WriteFile(filepath.Join(stage, specName), []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		os.RemoveAll(stage)
		return "", err
	}
	return stage, nil
}

// copyTree copies a file or directory recursively.
func copyTree(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := copyTree(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	return copyFile(src, dst, info.Mode().Perm())
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
