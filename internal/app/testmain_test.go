package app

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// A test subprocess is not the real agent thread that launched go test.
	// Identity scenarios set their own synthetic runtime IDs explicitly.
	_ = os.Unsetenv("CODEX_THREAD_ID")
	_ = os.Unsetenv("CODEX_SESSION_ID")
	// Provider installer tests opt into their own disposable config overrides.
	_ = os.Unsetenv("OPENCODE_CONFIG_DIR")
	_ = os.Unsetenv("XDG_CONFIG_HOME")
	os.Exit(m.Run())
}
