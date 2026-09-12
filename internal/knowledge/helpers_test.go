package knowledge

import (
	"io"
	"os"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// asHQ makes this test process look like the supervisor: a temp HOME with an HQ home, and
// cwd inside it (the same cwd-keyed role rule the radar and the pull stamp use).
func asHQ(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(state.HQHome(), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(state.HQHome())
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	b, _ := io.ReadAll(r)
	return string(b)
}

// noHeader stands in for hq's drain-liveness banner: the ledger's own tests do not judge
// the distill cadence.
func noHeader(int64) string { return "" }
