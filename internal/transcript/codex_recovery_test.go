package transcript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func TestCodexRecoveryRequiresRealMatchingHistory(t *testing.T) {
	for _, tc := range []struct{ name, dir, suffix, header, want string }{
		{"deferred empty thread", "sessions/2026/01/01", "", "", RecoveryMissing},
		{"current paginated", "sessions/2026/01/01", "", `{"type":"session_meta","payload":{"id":"thread-a","cwd":"/original","history_mode":"paginated"}}`, RecoveryReady},
		{"legacy", "sessions/2025/01/01", "", `{"type":"session_meta","payload":{"session_id":"thread-a"}}`, RecoveryReady},
		{"archived", "archived_sessions", "", `{"type":"session_meta","payload":{"id":"thread-a"}}`, RecoveryReady},
		{"continuation", "sessions/2026/01/01", "_instance-b", `{"type":"session_meta","payload":{"id":"thread-a"}}`, RecoveryReady},
		{"wrong identity", "sessions/2026/01/01", "", `{"type":"session_meta","payload":{"id":"other"}}`, RecoveryInvalid},
		{"wrong record", "sessions/2026/01/01", "", `{"type":"event_msg","payload":{"id":"thread-a"}}`, RecoveryInvalid},
		{"corrupt", "sessions/2026/01/01", "", `{`, RecoveryInvalid},
		{"oversized", "sessions/2026/01/01", "", strings.Repeat("x", 2<<20), RecoveryInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("CODEX_HOME", home)
			if tc.header != "" {
				dir := filepath.Join(home, tc.dir)
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "rollout-2026-thread-a"+tc.suffix+".jsonl"), []byte(tc.header+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if got := CodexRecovery("thread-a"); got.State != tc.want {
				t.Fatalf("got %+v, want %s", got, tc.want)
			}
		})
	}
}

func TestCodexRecoveryCompressedHistory(t *testing.T) {
	for _, tc := range []struct {
		name, dir, suffix, plain, want string
		corrupt                        bool
	}{
		{name: "compressed current", dir: "sessions/2026/01/01", want: RecoveryReady},
		{name: "compressed archived continuation", dir: "archived_sessions", suffix: "_part-b", want: RecoveryReady},
		{name: "corrupt compression", dir: "sessions", corrupt: true, want: RecoveryInvalid},
		{name: "plain wins", dir: "sessions", plain: `{"type":"session_meta","payload":{"id":"thread-a","cwd":"/plain"}}`, want: RecoveryReady},
		{name: "invalid plain cannot borrow compressed sibling", dir: "sessions", plain: `{`, want: RecoveryInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("CODEX_HOME", home)
			dir := filepath.Join(home, tc.dir)
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "rollout-time-thread-a"+tc.suffix+".jsonl")
			encoder, err := zstd.NewWriter(nil, zstd.WithEncoderConcurrency(1))
			if err != nil {
				t.Fatal(err)
			}
			data := encoder.EncodeAll([]byte(`{"type":"session_meta","payload":{"id":"thread-a","cwd":"/compressed"}}`+"\n"), nil)
			encoder.Close()
			if tc.corrupt {
				data = []byte("invalid-zstd")
			}
			if err := os.WriteFile(path+".zst", data, 0600); err != nil {
				t.Fatal(err)
			}
			if tc.plain != "" {
				if err := os.WriteFile(path, []byte(tc.plain+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			got := CodexRecovery("thread-a")
			if got.State != tc.want {
				t.Fatalf("got %+v, want %s", got, tc.want)
			}
			if tc.name == "plain wins" && got.Cwd != "/plain" {
				t.Fatal(got)
			}
			if _, err := os.Stat(path + ".zst"); err != nil {
				t.Fatal("history was modified", err)
			}
		})
	}
}

func TestCodexRecoveryRefusesSymlinkAndOtherHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	dir := filepath.Join(home, "sessions/2026/01/01")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "private.jsonl")
	if err := os.WriteFile(target, []byte(`{"type":"session_meta","payload":{"id":"thread-a"}}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "rollout-time-thread-a.jsonl")); err != nil {
		t.Fatal(err)
	}
	if got := CodexRecovery("thread-a"); got.State != RecoveryInvalid {
		t.Fatal(got)
	}
	t.Setenv("CODEX_HOME", t.TempDir())
	if got := CodexRecovery("thread-a"); got.State != RecoveryMissing {
		t.Fatal(got)
	}
	if got := CodexRecovery("../thread-a"); got.State != RecoveryInvalid {
		t.Fatal(got)
	}
}
