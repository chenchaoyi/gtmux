package sessionpolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestCopiedThreadCannotInheritConsentOrExternalFence(t *testing.T) {
	setup(t)
	fixture(t, "cli", "codex-tui")
	first := os.Getenv("CODEX_HOME")
	firstID := transcript.CodexHomeID()
	if err := MarkExternal("cli", 50); err != nil {
		t.Fatal(err)
	}
	if ObserveEvent("", "Codex", "cli", "terminal", 100) || LearnEvent("", "Codex", "cli", "terminal", 100) {
		t.Fatal("creation source bypassed external consent")
	}
	grant, err := saveAt("cli", Settings{HQ: true, Notify: true, Knowledge: true}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if !LearnEvent("", "Codex", "cli", "unconfirmed", 101) {
		t.Fatal("explicit future consent not honored")
	}
	t.Setenv("CODEX_HOME", t.TempDir())
	fixture(t, "cli", "codex-tui")
	if Get("cli").HQ || LearnEvent("", "Codex", "cli", "unconfirmed", 101, firstID) {
		t.Fatal("copied thread inherited another home's consent")
	}
	if RequiresConsent("cli", "terminal") {
		t.Fatal("external observation crossed homes")
	}
	t.Setenv("CODEX_HOME", first)
	grant.HQ = false
	if _, err := saveAt("cli", grant, 110); err != nil {
		t.Fatal(err)
	}
	if !RequiresConsent("cli", "terminal") || LearnEvent("", "Codex", "cli", "terminal", 200) {
		t.Fatal("revocation or retirement lost durable boundary")
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(first, alias); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", alias)
	if transcript.CodexHomeID() != firstID {
		t.Fatal("symlink changed configuration identity")
	}
	if err := os.WriteFile(fencePath("cli"), []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if !RequiresConsent("cli", "terminal") {
		t.Fatal("corrupt boundary granted implicit learning")
	}
}

func TestIDOnlyRequestsFailOnForeignRecordedScope(t *testing.T) {
	setup(t)
	if !ValidRequest("desk", "") {
		t.Fatal("unambiguous legacy request refused")
	}
	foreign := strings.Repeat("d", 32)
	if err := native.Save(native.Record{Agent: "codex", SessionID: "desk", HomeID: foreign}); err != nil {
		t.Fatal(err)
	}
	if ValidRequest("desk", "") || ValidRequest("desk", foreign) {
		t.Fatal("foreign record silently resolved to current home")
	}
	if !ValidRequest("desk", transcript.CodexHomeID()) {
		t.Fatal("explicit current home refused")
	}
}
