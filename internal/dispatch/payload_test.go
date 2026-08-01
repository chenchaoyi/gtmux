package dispatch

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// nastyGoal is the payload class that broke the argv channel on 2026-08-01: a Chinese
// instruction carrying backticked code identifiers, `$`, both quote flavours, and
// newlines. Inside double quotes a shell EXECUTES the backticked spans — the real
// dispatch died on `command substitution: syntax error near unexpected token 'done'`,
// which is why the `done` keyword is deliberately preserved here.
const nastyGoal = "把 `hqPlaybookVersion` 从 12 提到 13，并且：\n" +
	"1. 运行 `make check`（注意 $HOME、$(pwd) 与 \"双引号\" 和 '单引号'）\n" +
	"2. 千万别让 shell 执行 `for f in *; do echo $f; done`\n" +
	"3. 收尾时写一句 100% 完成\t（含制表符）"

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "goal.txt")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// The whole point of the file channel: what the caller wrote is what comes back, byte
// for byte. Not "the command exited 0" — the BYTES.
func TestReadPayload_ByteExactRoundTrip(t *testing.T) {
	// The trailing newline is what every heredoc and `printf` appends.
	got, err := ReadPayload(writeTemp(t, nastyGoal+"\n"), nil)
	if err != nil {
		t.Fatalf("ReadPayload: %v", err)
	}
	if got != nastyGoal {
		t.Fatalf("payload was mutated on the way through\n got: %q\nwant: %q", got, nastyGoal)
	}
	// Spelled out, so a future "harmless" normalization trips a named assertion.
	for _, frag := range []string{"`hqPlaybookVersion`", "$HOME", "$(pwd)", `"双引号"`, "'单引号'",
		"do echo $f; done", "\t"} {
		if !strings.Contains(got, frag) {
			t.Errorf("fragment %q did not survive the round-trip", frag)
		}
	}
	if n := strings.Count(got, "\n"); n != 3 {
		t.Errorf("interior newlines = %d, want 3 (structure must survive)", n)
	}
}

// Exactly ONE trailing newline is stripped — the heredoc's. A payload that genuinely
// ends in a blank line keeps it.
func TestReadPayload_TrailingNewlineRule(t *testing.T) {
	cases := []struct{ in, want string }{
		{"one line\n", "one line"},
		{"one line", "one line"},
		{"one line\r\n", "one line"},
		{"trailing blank kept\n\n", "trailing blank kept\n"},
		{"  leading space kept\n", "  leading space kept"},
	}
	for _, c := range cases {
		got, err := ReadPayload(writeTemp(t, c.in), nil)
		if err != nil {
			t.Fatalf("ReadPayload(%q): %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("ReadPayload(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestReadPayload_Stdin(t *testing.T) {
	got, err := ReadPayload("-", strings.NewReader(nastyGoal+"\n"))
	if err != nil {
		t.Fatalf("ReadPayload(-): %v", err)
	}
	if got != nastyGoal {
		t.Fatalf("stdin payload mutated\n got: %q\nwant: %q", got, nastyGoal)
	}
	if _, err := ReadPayload("-", nil); err == nil {
		t.Error("ReadPayload(\"-\", nil) must error rather than dispatch nothing silently")
	}
}

// An empty payload must be REFUSED, not delivered — submitting it would burn a turn on
// nothing and (worse) read as a successful dispatch.
func TestReadPayload_Empty(t *testing.T) {
	for _, in := range []string{"", "\n", "   \n\t\n"} {
		if _, err := ReadPayload(writeTemp(t, in), nil); !errors.Is(err, ErrPayloadEmpty) {
			t.Errorf("ReadPayload(%q) err = %v, want ErrPayloadEmpty", in, err)
		}
	}
}

func TestReadPayload_Oversize(t *testing.T) {
	big := strings.Repeat("x", PayloadMax+1)
	if _, err := ReadPayload(writeTemp(t, big), nil); err == nil {
		t.Fatal("an oversize payload must error, not be silently truncated into a mangled instruction")
	}
	if _, err := ReadPayload(writeTemp(t, strings.Repeat("x", PayloadMax)), nil); err != nil {
		t.Fatalf("a payload exactly at the cap must be accepted: %v", err)
	}
}

func TestReadPayload_MissingFile(t *testing.T) {
	if _, err := ReadPayload(filepath.Join(t.TempDir(), "nope.txt"), nil); err == nil {
		t.Fatal("a missing payload file must error")
	}
}

// recordingIO is a delivery surface that captures exactly what would be pasted into the
// pane. The pane is a plain shell (no locatable draft), so the paste guard proceeds
// without clearing anything, and a driver receipt lands the delivery deterministically.
type recordingIO struct {
	pasted []string
}

func (r *recordingIO) io(text string) IO {
	screen := "user@host project % "
	return IO{
		Capture: func() string { return screen },
		Paste:   func(s string) error { r.pasted = append(r.pasted, s); return nil },
		Enter:   func() error { return nil },
		Events: func(int64) []Ev {
			return []Ev{{Kind: EvSubmit, Head: NormalizeNeedle(text)}}
		},
		Now:   func() int64 { return 100 },
		Sleep: func() {},
	}
}

// The END-TO-END assertion the incident calls for: bytes on disk → the bytes handed to
// the pane. Nothing between the file and the input box may rewrite the instruction.
func TestDeliver_FilePayloadReachesThePaneVerbatim(t *testing.T) {
	goal, err := ReadPayload(writeTemp(t, nastyGoal+"\n"), nil)
	if err != nil {
		t.Fatalf("ReadPayload: %v", err)
	}
	rec := &recordingIO{}
	res := Deliver(rec.io(goal), Opts{Pane: "%1", HookEquipped: true, DeliverTimeout: 10}, goal)
	if !res.Delivered {
		t.Fatalf("delivery should land, got %+v", res)
	}
	if len(rec.pasted) != 1 {
		t.Fatalf("the payload must be pasted exactly once, got %d pastes", len(rec.pasted))
	}
	if rec.pasted[0] != nastyGoal {
		t.Fatalf("the pane received a MUTATED instruction\n got: %q\nwant: %q", rec.pasted[0], nastyGoal)
	}
}
