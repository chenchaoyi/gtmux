package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

func TestCFL004CLIAssociationRefusalDoesNotInventHumanDecision(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	old := i18n.Lang()
	i18n.SetLang("en")
	t.Cleanup(func() { i18n.SetLang(old) })
	var code int
	var output string
	stderr := captureStderr(t, func() {
		output = captureStdout(t, func() { code = refusedWaiting("%synthetic", "synthetic body", "codex-frontend-unavailable", nil, true) })
	})
	var receipt sendJSON
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &receipt); err != nil {
		t.Fatal(err)
	}
	if code != 1 || receipt.Delivered || receipt.Evidence != "codex-frontend-unavailable" {
		t.Fatal("refusal receipt changed", code, receipt)
	}
	if !strings.Contains(strings.ToLower(stderr), "association") || strings.Contains(stderr, "permission prompt") || strings.Contains(stderr, "answer it first") {
		t.Fatal("association refusal was falsely described as a human decision", stderr)
	}
	decision := captureStderr(t, func() {
		refusedWaiting("%synthetic", "synthetic body", "the agent is waiting on a decision (a permission prompt or a question)", nil, false)
	})
	if !strings.Contains(decision, "decision") {
		t.Fatal("real human decision lost its distinct refusal", decision)
	}
}

func TestCFL004PhoneSendReportsActualMissingAssociationWithoutInput(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("CODEX_HOME", t.TempDir())
	fake := filepath.Join(dir, "tmux-fixture")
	mutations := filepath.Join(dir, "unexpected-input")
	script := `#!/bin/sh
for last do :; done
case "$*" in
 *display-message*)
  case "$last" in
   '#{pane_id}') printf '%%9';;
   '#{pane_current_command}') printf 'codex';;
   '#{session_name}:#{window_index}.#{pane_index}') printf 'synthetic:0.0';;
   '#{pane_in_mode}') printf '0';;
  esac;;
 *capture-pane*) printf '›\n';;
 *send-keys*|*paste-buffer*|*load-buffer*) printf 'unexpected' >> 'MUTATIONS'; exit 5;;
esac
`
	script = strings.ReplaceAll(script, "MUTATIONS", mutations)
	if err := os.WriteFile(fake, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	old := tmux.Bin
	tmux.Bin = fake
	t.Cleanup(func() { tmux.Bin = old })
	if err := resume.Save("synthetic:0.0", resume.Record{Agent: "codex", SessionID: "synthetic-root", Frontend: strings.Repeat("a", 32)}); err != nil {
		t.Fatal(err)
	}
	_, err := sendToPaneWithReceipt("%9", "synthetic message", "", true, "cfl004-isolated-phone")
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "association") || strings.Contains(err.Error(), "permission prompt") {
		t.Fatal("phone send did not report its actual association failure", err)
	}
	if _, err := os.Stat(mutations); !os.IsNotExist(err) {
		t.Fatal("a refused native send attempted terminal input", err)
	}
}
