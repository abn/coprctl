package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

// buildDebugSkill is the curated debugging workflow skill. It references the
// same command registry as the main skill but adds the debug-and-reproduce
// procedure, including local reproduction via rpmbuilder and the log-detective
// helper.
func buildDebugSkill(root *cobra.Command, _ *App) string {
	var md strings.Builder
	md.WriteString("---\n")
	md.WriteString("name: coprctl-debug\n")
	md.WriteString("description: Debug a failing Copr build - find why it failed, reproduce\n")
	md.WriteString("  it locally with rpmbuilder or mock, and test a fix before pushing.\n")
	md.WriteString("---\n\n")
	md.WriteString("# Debugging a failing Copr build\n\n")
	md.WriteString("Use this skill when a Copr build fails and you need to find the cause,\n")
	md.WriteString("reproduce it locally, and verify a fix before pushing.\n\n")
	md.WriteString("## 1. Find why the build failed\n\n")
	md.WriteString("Do not ingest a whole build log. Run the failures summariser first:\n\n")
	md.WriteString("```bash\n")
	md.WriteString("coprctl log failures BUILD_ID\n")
	md.WriteString("```\n\n")
	md.WriteString("It extracts the failing region from each failed chroot. For a quick\n")
	md.WriteString("plain-language analysis of the root cause, ask the log-detective helper:\n\n")
	md.WriteString("```bash\n")
	md.WriteString("coprctl log detective BUILD_ID/CHROOT\n")
	md.WriteString("```\n\n")
	md.WriteString("## 2. Reproduce locally\n\n")
	md.WriteString("Get the exact reproduction recipe Copr wrote into the build log:\n\n")
	md.WriteString("```bash\n")
	md.WriteString("coprctl build reproduce BUILD_ID/CHROOT\n")
	md.WriteString("```\n\n")
	md.WriteString("This prints the `copr-rpmbuild --task-url ...` invocation. If you have a\n")
	md.WriteString("container runtime, `coprctl try` runs a local clean-room preflight build:\n\n")
	md.WriteString("```bash\n")
	md.WriteString("coprctl try ./rpm --chroot fedora-rawhide-x86_64\n")
	md.WriteString("```\n\n")
	md.WriteString("`try` resolves the Copr chroot to an rpmbuilder image and runs the\n")
	md.WriteString("source-build then chroot-build stages, reporting coverage and fidelity.\n")
	md.WriteString("When no container runtime is available, `build reproduce` with mock is the\n")
	md.WriteString("fallback (needs mock and privileges).\n\n")
	md.WriteString("## 3. Test a fix before pushing\n\n")
	md.WriteString("- For tito projects: commit the spec change (tito ignores uncommitted\n")
	md.WriteString("  changes), then run `coprctl try` in the project. Squash WIP commits\n")
	md.WriteString("  before tagging; never push intermediate debug commits.\n")
	md.WriteString("- Declare all dependencies in `BuildRequires`/`Requires`; do not install\n")
	md.WriteString("  them manually in the container.\n")
	md.WriteString("- Iterate until the local build is clean, then rebuild in Copr:\n\n")
	md.WriteString("```bash\n")
	md.WriteString("coprctl build rebuild OWNER/PROJECT/PKG --preflight\n")
	md.WriteString("```\n\n")
	md.WriteString("## Rules\n")
	md.WriteString("1. Start with `coprctl log failures BUILD_ID`, never a full log dump.\n")
	md.WriteString("2. Reproduce locally before queueing another Copr build.\n")
	md.WriteString("3. A local pass is a filter, not a proof: read the fidelity report from\n")
	md.WriteString("   `coprctl try` and name what was not reproduced.\n")
	md.WriteString("4. Commit before `try` on a tito project; squash WIP before push.\n")
	md.WriteString("5. Use `coprctl log detective` for a second opinion on the root cause.\n")
	md.WriteString("\n## Reference\n\n")
	md.WriteString("All commands are part of the `coprctl` skill; print it with\n")
	md.WriteString("`coprctl skill print coprctl`.\n")
	return md.String()
}
