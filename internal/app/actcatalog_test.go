package app

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/diag"
)

// The command table's "writes" flag is the checklist for the action trail: a command that
// changes something and records nothing is a change nobody can find afterwards. This is
// the same kind of guard as the help test that fails a dispatched command missing from
// the table.
func TestEveryWritingCommandRecordsAnAct(t *testing.T) {
	recorded := map[string]bool{}
	for _, e := range diag.Catalog {
		for _, c := range e.Commands {
			recorded[c] = true
		}
	}
	for _, c := range helpCommands {
		if c.Writes && !recorded[c.Name] {
			t.Errorf("gtmux %s is marked as writing, and no act in diag.Catalog is recorded by it", c.Name)
		}
	}
	// A command the catalog names must exist, or the list has drifted from the table.
	for _, e := range diag.Catalog {
		for _, c := range e.Commands {
			if findCommand(c) == nil {
				t.Errorf("%s names %q, which is not a command", e.Event, c)
			}
		}
	}
}

// The catalog and the code must agree both ways: an event listed and never written is a
// promise nothing keeps, and an event written and never listed is one `gtmux logs --event`
// and the docs do not know about.
func TestTheActCatalogMatchesTheCode(t *testing.T) {
	root := filepath.Join("..", "..")
	literal := regexp.MustCompile(`"(act\.[a-z][a-z.]*[a-z])"`)
	written := map[string]string{}
	// The Go side, and the menu bar app, which writes to the same store from Swift.
	for _, dir := range []string{filepath.Join(root, "internal"), filepath.Join(root, "macapp", "Sources")} {
		err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			source := strings.HasSuffix(p, ".go") || strings.HasSuffix(p, ".swift")
			if d.IsDir() || !source || strings.HasSuffix(p, "_test.go") ||
				strings.HasSuffix(p, filepath.Join("diag", "catalog.go")) || strings.Contains(p, filepath.Join("internal", "docs")) {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			for _, m := range literal.FindAllStringSubmatch(string(b), -1) {
				written[m[1]] = p
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	listed := map[string]bool{}
	for _, e := range diag.Catalog {
		if listed[e.Event] {
			t.Errorf("%s is listed twice", e.Event)
		}
		listed[e.Event] = true
		if _, ok := written[e.Event]; !ok {
			t.Errorf("%s is in diag.Catalog and nothing writes it", e.Event)
		}
	}
	var missing []string
	for ev, p := range written {
		if !listed[ev] {
			missing = append(missing, ev+" ("+p+")")
		}
	}
	sort.Strings(missing)
	for _, m := range missing {
		t.Errorf("written but not in diag.Catalog: %s", m)
	}
}
