package hqpane

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakePanes installs a fixture pane list for the duration of a test.
func fakePanes(t *testing.T, lines ...string) {
	t.Helper()
	prev := lister
	lister = func() []string { return lines }
	t.Cleanup(func() { lister = prev })
}

// pane renders one `list-panes` record: id, the stamped HQ home, current path,
// start path.
func pane(id, stamp, cwd, start string) string {
	return strings.Join([]string{id, stamp, cwd, start}, "\t")
}

// hqHome creates the HQ home under a temp HOME and returns its path.
func hqHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "gtmux", "hq")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestFind_ByCurrentPath(t *testing.T) {
	dir := hqHome(t)
	fakePanes(t, pane("%1", "", "/somewhere/else", "/somewhere/else"), pane("%7", "", dir, dir))
	if got := Find(); got != "%7" {
		t.Fatalf("the pane sitting in the HQ home is the supervisor; got %q", got)
	}
}

// The regression this package exists for: tmux reports the PHYSICAL path, HQHome() is
// built from $HOME. One symlink anywhere on the way used to make every wake resolve
// "no HQ" and vanish without a trace.
func TestFind_SymlinkedHomeStillResolves(t *testing.T) {
	dir := hqHome(t)
	physical, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	// A path that differs only by symlink resolution (macOS: /var → /private/var,
	// dotfiles: ~/.config → ~/repos/dotfiles/config).
	if physical == dir {
		// The temp dir was not itself symlinked — build the alias explicitly.
		link := filepath.Join(t.TempDir(), "hq-link")
		if err := os.Symlink(dir, link); err != nil {
			t.Fatal(err)
		}
		physical = link
	}
	fakePanes(t, pane("%7", "", physical, physical))
	if got := Find(); got != "%7" {
		t.Fatalf("a symlinked HQ home must still resolve (this loses every wake); got %q", got)
	}
}

func TestFind_ByStampAfterACd(t *testing.T) {
	dir := hqHome(t)
	// HQ cd'd into a repo: neither path names the home any more — only the stamp does.
	fakePanes(t, pane("%7", dir, "/repos/gtmux", "/repos/gtmux"))
	if got := Find(); got != "%7" {
		t.Fatalf("the stamp identifies HQ regardless of cwd; got %q", got)
	}
}

// The stamp names the home it serves, so it can only ever resolve THAT install's
// supervisor: a tmux server shared with another gtmux ($HOME) — or with a test —
// must never resolve to the other's HQ pane, which a bare "this is an hq" flag would.
func TestFind_StampIsScopedToItsOwnHome(t *testing.T) {
	other := filepath.Join(t.TempDir(), ".config", "gtmux", "hq")
	hqHome(t) // a DIFFERENT home than the pane's stamp
	fakePanes(t, pane("%7", other, other, other))
	if got := Find(); got != "" {
		t.Fatalf("another install's supervisor is not ours; got %q", got)
	}
}

func TestFind_ByStartPath(t *testing.T) {
	dir := hqHome(t)
	fakePanes(t, pane("%7", "", "/repos/gtmux", dir))
	if got := Find(); got != "%7" {
		t.Fatalf("a pane STARTED in the HQ home is the supervisor; got %q", got)
	}
}

func TestFind_NoSupervisor(t *testing.T) {
	hqHome(t)
	fakePanes(t, pane("%1", "", "/repos/gtmux", "/repos/gtmux"))
	if got := Find(); got != "" {
		t.Fatalf("no HQ pane → no resolution; got %q", got)
	}
}

func TestFindOther_NeverSelfWakes(t *testing.T) {
	dir := hqHome(t)
	fakePanes(t, pane("%7", dir, dir, dir))
	got, self := FindOther("%7")
	if got != "" || !self {
		t.Fatalf("gtmux must never wake HQ about HQ itself; got %q self=%v", got, self)
	}
	got, self = FindOther("%1")
	if got != "%7" || self {
		t.Fatalf("an event about another pane resolves HQ normally; got %q self=%v", got, self)
	}
}

// "HQ is this pane" and "no HQ resolved" are both an empty pane id but opposite
// instructions: stay silent versus hold the wake for a later drain.
func TestFindOther_SelfIsNotTheSameAsMissing(t *testing.T) {
	hqHome(t)
	fakePanes(t, pane("%1", "", "/repos/gtmux", "/repos/gtmux"))
	if got, self := FindOther("%1"); got != "" || self {
		t.Fatalf("no HQ at all is not a self-wake; got %q self=%v", got, self)
	}
}

func TestSeenRecently_StampedOnResolve(t *testing.T) {
	dir := hqHome(t)
	if SeenRecently() {
		t.Fatal("a fresh machine has never seen an HQ")
	}
	fakePanes(t, pane("%7", dir, dir, dir))
	Find()
	if !SeenRecently() {
		t.Fatal("resolving an HQ must stamp it — that stamp is what holds a wake later")
	}
	// The pane goes away: within the window a wake is HELD, past it dropped.
	fakePanes(t)
	if got := Find(); got != "" {
		t.Fatalf("no HQ now; got %q", got)
	}
	if !SeenRecently() {
		t.Fatal("an HQ seen moments ago is still recent — hold the wake, don't drop it")
	}
	old := time.Now().Add(-SeenWindow - time.Minute)
	if err := os.Chtimes(seenStampPath(), old, old); err != nil {
		t.Fatal(err)
	}
	if SeenRecently() {
		t.Fatal("past the window there is genuinely no supervisor — queue nothing")
	}
}
