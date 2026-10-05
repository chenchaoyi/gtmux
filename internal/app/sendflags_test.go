package app

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// A mistyped option used to be sent as text: `gtmux send %5 --body-file <path>` (relay's
// flag, not send's) arrived in the other agent's box as "--body-file <path>" (%6,
// 2026-10-06). An unknown --option is now refused before anything is touched.
func TestSendRefusesAnUnknownOption(t *testing.T) {
	i18n.SetLang("en")
	t.Cleanup(func() { i18n.SetLang("") })
	for _, args := range [][]string{
		{"%5", "--body-file", "/tmp/report.txt"},
		{"%5", "--body-file=/tmp/report.txt"},
		{"%5", "--message-flie", "x.txt"},
		{"%5", "hello", "--no-verfy"},
		{"--bodyfile", "%5", "hi"},
	} {
		var code int
		errOut := captureStderr(t, func() { _, code = parseSendArgs(args) })
		if code != 2 || !strings.Contains(errOut, "unknown option") || !strings.Contains(errOut, "nothing was sent") {
			t.Errorf("%q: code %d, stderr %q", args, code, errOut)
		}
	}
}

// What stays legal: text after a lone --, a -- inside text, single-dash words, and option
// values that look like options (they belong to the option before them).
func TestSendStillTakesTextThatLooksLikeAnOption(t *testing.T) {
	cases := []struct {
		args []string
		rest []string
		key  string
		file string
	}{
		{[]string{"%5", "--", "--body-file", "is", "relay's"}, []string{"%5", "--body-file", "is", "relay's"}, "", ""},
		{[]string{"--no-verify", "%5", "--", "--x"}, []string{"%5", "--x"}, "", ""},
		{[]string{"%5", "use", "A", "--", "not", "--B"}, []string{"%5", "use", "A", "--", "not", "--B"}, "", ""},
		{[]string{"%5", "fix", "the", "-v", "flag", "-"}, []string{"%5", "fix", "the", "-v", "flag", "-"}, "", ""},
		{[]string{"%5", "--message-file", "--odd-name.txt"}, []string{"%5"}, "", "--odd-name.txt"},
		{[]string{"%5", "--key", "--weird"}, []string{"%5"}, "--weird", ""},
		{[]string{"%5", "--message-file=-"}, []string{"%5"}, "", "-"},
	}
	for _, c := range cases {
		sa, code := parseSendArgs(c.args)
		if code != 0 || !reflect.DeepEqual(sa.rest, c.rest) || sa.key != c.key || sa.msgFile != c.file {
			t.Errorf("%q: code %d rest %q key %q file %q", c.args, code, sa.rest, sa.key, sa.msgFile)
		}
	}
}

// In a private tmux server: a refused option delivers nothing, and the legal ways to send
// text that starts with -- deliver it. cat echoes a line once Enter ends it.
func TestSendOptionsOnARealPane(t *testing.T) {
	l := newHoldLab(t)
	id := l.run("new-window", "-d", "-P", "-F", "#{pane_id}", "-t", "lab", "-n", "flags", "cat")
	time.Sleep(300 * time.Millisecond)

	captureStderr(t, func() {
		if code := cmdSend([]string{id, "--body-file", "/tmp/report.txt"}); code != 2 {
			t.Errorf("unknown option: exit %d", code)
		}
	})
	time.Sleep(300 * time.Millisecond)
	if strings.Contains(l.screen(id), "body-file") {
		t.Fatalf("a refused option reached the pane:\n%s", l.screen(id))
	}

	if code := cmdSend([]string{id, "--no-verify", "--", "--body-file is relay's flag"}); code != 0 {
		t.Fatalf("after --: exit %d", code)
	}
	l.waitFor(id, func(s string) bool { return strings.Count(s, "--body-file is relay's flag") == 2 })

	msg := filepath.Join(l.dir, "msg.txt")
	if err := os.WriteFile(msg, []byte("--from-a-file stays text"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := cmdSend([]string{id, "--no-verify", "--message-file", msg}); code != 0 {
		t.Fatalf("--message-file: exit %d", code)
	}
	l.waitFor(id, func(s string) bool { return strings.Count(s, "--from-a-file stays text") == 2 })
}
