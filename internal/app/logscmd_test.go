package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestParseWhenReadsDurationsAndDates(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.Local)
	cases := map[string]time.Time{
		"30m":        now.Add(-30 * time.Minute),
		"2h":         now.Add(-2 * time.Hour),
		"3d":         now.AddDate(0, 0, -3),
		"1w":         now.AddDate(0, 0, -7),
		"2026-09-01": time.Date(2026, 9, 1, 0, 0, 0, 0, time.Local),
	}
	for in, want := range cases {
		got, err := parseWhen(in, now)
		if err != nil || !got.Equal(want) {
			t.Errorf("parseWhen(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := parseWhen("yesterday", now); err == nil {
		t.Error("an unreadable time was accepted")
	}
}

func writeStoreLine(t *testing.T, name, line string) {
	t.Helper()
	dir := state.LogsDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, _ = f.WriteString(line + "\n")
}

// "What did the phone do today" is one command, whichever day files and in-day segments
// the entries sit in.
func TestLogsReadsAcrossDaysAndSegmentsInOrder(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	writeStoreLine(t, "2026-09-18.jsonl", `{"ts":"2026-09-18T23:59:00.000+08:00","level":"info","component":"serve","kind":"act","event":"act.send","actor":"phone:3f9c20e1","target":"%7","outcome":"ok"}`)
	writeStoreLine(t, "2026-09-19.jsonl", `{"ts":"2026-09-19T09:00:00.000+08:00","level":"info","component":"serve","kind":"diag","event":"serve.start","msg":"serve started"}`)
	writeStoreLine(t, "2026-09-19.1.jsonl", `{"ts":"2026-09-19T10:00:00.000+08:00","level":"warn","component":"serve","kind":"act","event":"act.pair","actor":"anonymous","outcome":"refused","msg":"a pairing code was not accepted","attrs":{"reason":"expired"}}`)
	writeStoreLine(t, "2026-09-10.jsonl", `{"ts":"2026-09-10T10:00:00.000+08:00","level":"info","component":"serve","kind":"act","event":"act.send","actor":"phone:3f9c20e1","outcome":"ok"}`)

	since := time.Date(2026, 9, 18, 0, 0, 0, 0, time.FixedZone("", 8*3600))
	all := readLogStore(state.LogsDir(), logsFilter{since: since, minLevel: diag.Debug})
	if len(all) != 3 || all[0].Event != "act.send" || all[2].Event != "act.pair" {
		t.Fatalf("got %d entries in the wrong order or window: %+v", len(all), all)
	}
	phone := readLogStore(state.LogsDir(), logsFilter{since: since, minLevel: diag.Debug, actsOnly: true, actor: "phone"})
	if len(phone) != 1 || phone[0].Target != "%7" {
		t.Errorf("--acts --actor phone = %+v", phone)
	}
	warn := readLogStore(state.LogsDir(), logsFilter{since: since, minLevel: diag.Warn})
	if len(warn) != 1 || warn[0].Event != "act.pair" {
		t.Errorf("--level warn = %+v", warn)
	}
	byEvent := readLogStore(state.LogsDir(), logsFilter{since: since, minLevel: diag.Debug, event: "act.*"})
	if len(byEvent) != 2 {
		t.Errorf("--event 'act.*' matched %d, want 2", len(byEvent))
	}
}

func TestALogLineSaysWhoWhatAndHowItEnded(t *testing.T) {
	var b strings.Builder
	printLogEntry(&b, diag.Entry{TS: "2026-09-19T10:00:00.000+08:00", Level: "warn", Component: "serve",
		Kind: diag.KindAct, Event: "act.pair", Actor: "anonymous", Outcome: "refused",
		Msg: "a pairing code was not accepted", Attrs: map[string]any{"reason": "expired", "via": "tunnel"}}, false, false)
	got := b.String()
	for _, want := range []string{"serve", "warn", "act.pair", "anonymous", "refused", "a pairing code was not accepted", "reason=expired", "via=tunnel"} {
		if !strings.Contains(got, want) {
			t.Errorf("the line lacks %q: %s", want, got)
		}
	}
}
