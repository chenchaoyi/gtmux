package hq

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.Unsetenv("CODEX_THREAD_ID")
	os.Unsetenv("CODEX_SESSION_ID")
	os.Exit(m.Run())
}
