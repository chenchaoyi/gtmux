package transcript

import "testing"

func TestHSP005DirectoryQualifiedContextRequiresVerifiedMetadata(t *testing.T) {
	const cwd = "/synthetic/work tree"
	const env = "<environment_context>\n<cwd>/synthetic/work tree</cwd>\n</environment_context>"
	agents := func(dir string) string {
		return "# AGENTS.md instructions for " + dir + "\nsynthetic repository context"
	}
	for _, tc := range []struct {
		name, metaDir string
		blocks        []string
		wantProof     bool
	}{
		{"exact metadata directory", cwd, []string{agents(cwd), env}, true},
		{"no metadata directory", "", []string{agents(cwd), env}, false},
		{"different metadata directory", "/synthetic/other", []string{agents(cwd), env}, false},
		{"lookalike heading", cwd, []string{agents(cwd + "-other"), env}, false},
		{"relative metadata directory", "relative", []string{agents("relative"), env}, false},
		{"real input in second block", cwd, []string{agents(cwd), "synthetic user input"}, false},
		{"text following a context block", cwd, []string{agents(cwd), env + "\nsynthetic user input"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			message, check := startupContextWitnessFixture(t, tc.metaDir)
			check(t, message("user", "", tc.blocks...), tc.wantProof)
		})
	}
}
