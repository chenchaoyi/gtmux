package knowledge

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The import rules scripts/import-boundaries.sh enforces for check-design.sh: mine never
// imports knowledge, and knowledge imports nothing above the leaves. Run on this repo
// (it must pass) and on synthetic modules that break each rule (it must fail, naming
// the import). No network: the synthetic modules have no dependencies.
func TestImportBoundaries(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go on PATH")
	}
	script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "import-boundaries.sh"))
	if err != nil {
		t.Fatal(err)
	}
	run := func(dir string) (int, string) {
		t.Helper()
		cmd := exec.Command("bash", script, dir)
		cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOWORK=off", "GOPROXY=off", "GOTOOLCHAIN=local")
		out, err := cmd.CombinedOutput()
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode(), string(out)
		} else if err != nil {
			t.Fatal(err)
		}
		return 0, string(out)
	}

	if code, out := run(filepath.Join("..", "..")); code != 0 {
		t.Fatalf("this repo breaks an import boundary (exit %d):\n%s", code, out)
	}

	// module writes a synthetic module whose packages import what imports says.
	module := func(imports map[string][]string) string {
		dir := t.TempDir()
		write := func(rel, body string) {
			p := filepath.Join(dir, rel)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		write("go.mod", "module example.com/m\n\ngo 1.21\n")
		for _, pkg := range []string{"mine", "knowledge", "hq", "radar", "app", "dispatchbridge", "state"} {
			src := "package " + pkg + "\n"
			for _, imp := range imports[pkg] {
				src += "\nimport _ \"example.com/m/internal/" + imp + "\"\n"
			}
			write("internal/"+pkg+"/"+pkg+".go", src)
		}
		return dir
	}
	// A module this toolchain cannot read is "not checked" (2), not a violation (1):
	// check-design.sh fails only on 1 (%12's review of #1403).
	tooNew := module(nil)
	if err := os.WriteFile(filepath.Join(tooNew, "go.mod"), []byte("module example.com/m\n\ngo 99.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out := run(tooNew); code != 2 {
		t.Errorf("a go.mod newer than the toolchain: exit %d, want 2:\n%s", code, out)
	}
	if code, out := run(module(map[string][]string{"mine": {"state"}, "knowledge": {"state"}})); code != 0 {
		t.Fatalf("a clean synthetic module failed (exit %d):\n%s", code, out)
	}
	for _, c := range []struct {
		imports map[string][]string
		want    string
	}{
		{map[string][]string{"mine": {"knowledge"}}, "internal/mine imports internal/knowledge"},
		{map[string][]string{"mine": {"state"}, "state": {"knowledge"}}, "internal/mine imports internal/knowledge"}, // through another package
		{map[string][]string{"knowledge": {"hq"}}, "internal/knowledge imports internal/hq"},
		{map[string][]string{"knowledge": {"radar"}}, "internal/knowledge imports internal/radar"},
	} {
		code, out := run(module(c.imports))
		if code != 1 || !strings.Contains(out, c.want) {
			t.Errorf("%v: exit %d, want 1 naming %q:\n%s", c.imports, code, c.want, out)
		}
	}
}
