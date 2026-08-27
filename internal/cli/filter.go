package cli

import "strings"

// matchesFilter applies name filters for chroot listing. Distro and arch are
// substring filters on the chroot name (which is distro-version-arch).
func matchesFilter(name, glob, distro, arch string) bool {
	if glob != "" && !globMatch(glob, name) {
		return false
	}
	if distro != "" && !strings.Contains(name, distro+"-") {
		return false
	}
	if arch != "" && !strings.HasSuffix(name, "-"+arch) {
		return false
	}
	return true
}

// filterChroots applies name filters and returns the surviving names.
func filterChroots(names []string, glob, distro, arch string) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		if matchesFilter(n, glob, distro, arch) {
			out = append(out, n)
		}
	}
	return out
}

// globMatch is a minimal glob matcher supporting '*' and '?'.
func globMatch(pattern, s string) bool {
	var match func(p, str string) bool
	match = func(p, str string) bool {
		for len(p) > 0 {
			switch p[0] {
			case '*':
				for i := 0; i <= len(str); i++ {
					if match(p[1:], str[i:]) {
						return true
					}
				}
				return false
			case '?':
				if len(str) == 0 {
					return false
				}
				p = p[1:]
				str = str[1:]
			default:
				if len(str) == 0 || p[0] != str[0] {
					return false
				}
				p = p[1:]
				str = str[1:]
			}
		}
		return len(str) == 0
	}
	return match(pattern, s)
}
