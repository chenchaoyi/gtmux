package app_test

import (
	"os/exec"
	"strings"
	"testing"
)

// The package decomposition (openspec change decompose-app-package) declares a strict
// acyclic layering: app → hq → {radar, dispatchbridge} → leaves, and hq NEVER imports
// app. The Go compiler enforces ACYCLICITY (a back-edge that closes a cycle won't
// build), but a violation that stays acyclic — a leaf reaching up to a peer, or hq
// importing app — compiles silently and erodes the decomposition. CLAUDE.md states the
// rule in prose and check-design.sh doesn't cover it; this pins the intended EDGES.
func TestImportLayers(t *testing.T) {
	const mod = "github.com/chenchaoyi/gtmux/internal/"
	// package → import paths it must NOT (transitively) depend on.
	forbidden := map[string][]string{
		"hq":             {"app"},       // the load-bearing rule: hq never imports app
		"radar":          {"app", "hq"}, // leaves/kernels sit below hq
		"dispatchbridge": {"app", "hq"}, //
		"panefocus":      {"app", "hq"}, //
	}
	for pkg, banned := range forbidden {
		deps := transitiveDeps(t, mod+pkg)
		for _, b := range banned {
			if deps[mod+b] {
				t.Errorf("internal/%s transitively imports internal/%s — violates the "+
					"app→hq→{radar,dispatchbridge}→leaves layering (see decompose-app-package)", pkg, b)
			}
		}
	}
}

func transitiveDeps(t *testing.T, pkg string) map[string]bool {
	t.Helper()
	// `go list -deps` is module-aware from any CWD inside the module, so the CWD (the
	// test's package dir) doesn't matter.
	out, err := exec.Command("go", "list", "-deps", pkg).Output()
	if err != nil {
		t.Fatalf("go list -deps %s: %v", pkg, err)
	}
	m := map[string]bool{}
	for _, ln := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		m[strings.TrimSpace(ln)] = true
	}
	return m
}
