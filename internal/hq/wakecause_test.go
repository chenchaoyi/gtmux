package hq

import (
	"strings"
	"testing"
)

// The alarm fired four times on 2026-09-02 over a pane that was simply scrolled into
// copy-mode. Every word of it was true and none of it was actionable: it sent the reader
// looking for a stuck draft that was not there, while 22 knocks sat held for 1h40m and
// one keypress released all of them.
func TestWakeNotifyMessageSaysWhatToDo(t *testing.T) {
	copyMode := wakeNotifyMessage("the HQ pane %6 is scrolled into copy-mode (press q there)")
	if !strings.Contains(copyMode, "copy-mode") || !strings.Contains(strings.ToLower(copyMode), "q") {
		t.Errorf("copy-mode alarm = %q — it must name the mode and the key that leaves it", copyMode)
	}
	if strings.Contains(copyMode, "stuck draft") {
		t.Error("copy-mode alarm still sends the reader after a draft that is not there")
	}

	draft := wakeNotifyMessage("the HQ pane %6 has unsent text in its input box")
	if !strings.Contains(draft, "unsent text") {
		t.Errorf("draft alarm = %q", draft)
	}

	// No cause visible is the honest fallback — the wording gtmux has always used, not a
	// guess dressed up as a diagnosis.
	if !strings.Contains(wakeNotifyMessage(""), "stuck draft") {
		t.Error("with no cause read, the alarm must fall back rather than invent one")
	}
}
