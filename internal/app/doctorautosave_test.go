package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func saveLayoutFixture(t *testing.T, now time.Time) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "last")
	if err := os.WriteFile(path, []byte("pane\tsession\t1\t1\t0\t1\t/work\t1\tbash\n"), 0600); err != nil {
		t.Fatal(err)
	}
	mt := now.Add(-30 * time.Minute)
	if err := os.Chtimes(path, mt, mt); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAutoSaveRecentSuccessfulUnchangedLayout(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now()
	path := saveLayoutFixture(t, now)
	receipt := newBackstopReceipt(now.Add(-10*time.Minute).Unix(), true, "/tmp/server", "123", path)
	for _, lang := range []string{"en", "zh"} {
		i18n.SetLang(lang)
		row := autoSaveCheck(armedStatusRight, path, receipt, "/tmp/server", "123", now)
		if row.status != stOK {
			t.Fatalf("%s unchanged layout with recent success: %+v", lang, row)
		}
		if !strings.Contains(row.value, "10m") || !strings.Contains(row.note, "30m") {
			t.Fatalf("execution and snapshot ages must stay distinct: %+v", row)
		}
	}
	i18n.SetLang("en")
}

func TestAutoSaveRejectsUnverifiedEvidence(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	i18n.SetLang("en")
	now := time.Now()
	for _, name := range []string{"none", "legacy", "failed", "expired", "future", "other-server", "restarted-server", "changed-layout", "missing-layout", "invalid-layout"} {
		t.Run(name, func(t *testing.T) {
			path := saveLayoutFixture(t, now)
			r := newBackstopReceipt(now.Add(-time.Minute).Unix(), true, "/tmp/server", "123", path)
			switch name {
			case "none":
				r = backstopReceipt{}
			case "legacy":
				r = backstopReceipt{At: now.Add(-time.Minute).Unix(), Result: "ok"}
			case "failed":
				r.Result = "failed"
			case "expired":
				r.At = now.Add(-21 * time.Minute).Unix()
			case "future":
				r.At = now.Add(time.Minute).Unix()
			case "other-server":
				r.Socket = "/tmp/other"
			case "restarted-server":
				r.ServerPID = "456"
			case "changed-layout":
				if err := os.WriteFile(path, []byte("pane\tdifferent\n"), 0600); err != nil {
					t.Fatal(err)
				}
				mt := now.Add(-30 * time.Minute)
				_ = os.Chtimes(path, mt, mt)
			case "missing-layout":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "invalid-layout":
				if err := os.WriteFile(path, []byte("not a layout\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if row := autoSaveCheck(armedStatusRight, path, r, "/tmp/server", "123", now); row.status != stRec {
				t.Fatalf("%s passed without reliable evidence: %+v", name, row)
			}
		})
	}
	// A fresh valid snapshot can stand on its own, without a gtmux receipt.
	path := saveLayoutFixture(t, now)
	_ = os.Chtimes(path, now, now)
	if row := autoSaveCheck(armedStatusRight, path, backstopReceipt{}, "", "", now); row.status != stOK {
		t.Fatalf("fresh snapshot: %+v", row)
	}
	// A healthy backstop must not conceal a missing or duplicated continuum trigger.
	r := newBackstopReceipt(now.Unix(), true, "/tmp/server", "123", path)
	for _, trigger := range []string{"", armedStatusRight + armedStatusRight} {
		if autoSaveCheck(trigger, path, r, "/tmp/server", "123", now).status != stRec {
			t.Fatal("trigger misconfiguration hidden")
		}
	}
}

func TestBackstopReceiptRoundTripAndLegacyRetry(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now()
	path := saveLayoutFixture(t, now)
	want := newBackstopReceipt(now.Unix(), true, "/tmp/server", "123", path)
	writeBackstopReceipt(want)
	got := readBackstopReceipt()
	if got != want || !got.matches(path, "/tmp/server", "123", now) {
		t.Fatalf("receipt lost across restart: %+v", got)
	}
	if at, ok := readBackstopAttempt(); at != want.At || !ok {
		t.Fatal("retry reader lost successful execution")
	}
	_ = state.WriteMarker(backstopAttemptPath(), "1234 ok")
	if at, ok := readBackstopAttempt(); at != 1234 || !ok {
		t.Fatal("legacy retry marker no longer readable")
	}
	if readBackstopReceipt().matches(path, "/tmp/server", "123", now) {
		t.Fatal("legacy marker cannot certify current server/layout")
	}
	_ = state.WriteMarker(backstopAttemptPath(), "broken")
	if at, ok := readBackstopAttempt(); at != 0 || ok {
		t.Fatal("corrupt marker must not certify success")
	}
}

func TestSaveFingerprintBoundsAndReplacedSnapshot(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now()
	path := saveLayoutFixture(t, now)
	receipt := newBackstopReceipt(now.Unix(), true, "/tmp/server", "123", path)
	other := saveLayoutFixture(t, now)
	if receipt.matches(other, "/tmp/server", "123", now) {
		t.Fatal("same content at another path must not share proof")
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", (4<<20)+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, valid := fingerprintSave(path); valid {
		t.Fatal("oversized layout must stay unverified")
	}
	// Malformed JSON must not leave a partially decoded success behind.
	_ = state.WriteMarker(backstopAttemptPath(), `{"at":1234,"result":"ok","snapshot":{"path":"x"},"socket":4}`)
	if at, ok := readBackstopAttempt(); at != 0 || ok {
		t.Fatal("partially decoded corrupt receipt was trusted")
	}
}
