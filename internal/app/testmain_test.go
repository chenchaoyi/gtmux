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
	os.Exit(m.Run())
}
