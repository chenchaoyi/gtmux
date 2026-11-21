package hq

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/notify"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestWAP001LiveReceiptsDriveWakeProofNotices(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const now = 1700000000
	healthDir := filepath.Join(state.Dir(), "hqwake")
	evidenceDir := filepath.Join(state.Dir(), "hq-wake-accepted")
	if err := state.MkdirAll(evidenceDir, 0700); err != nil {
		t.Fatal(err)
	}
	write := func(path, body string) {
		t.Helper()
		if err := state.WritePrivateFile(path, []byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	writeIssue := func(reason string, first, last int) {
		t.Helper()
		write(filepath.Join(healthDir, "wake-health-"+reason), fmt.Sprintf(`{"Reason":%q,"FirstAt":%d,"LastAt":%d}`, reason, first, last))
	}
	writeIssue("accepted-unverified", now-3600, now-1800)
	writeIssue("observation-archived", now-1200, now-60)
	if err := state.WriteInt64Marker(filepath.Join(healthDir, "wake-health-confirmed"), now-20); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(healthDir, "notices-channel.json"),
		fmt.Sprintf(`{"accepted-unverified":%d,"observation-archived":%d}`, now-1200, now-15))

	prior := wakeDesktopNotice
	t.Cleanup(func() { wakeDesktopNotice = prior })
	var seen []notify.Options
	wakeDesktopNotice = func(options notify.Options) bool {
		seen = append(seen, options)
		return true
	}
	check := func(at int64, count int, lastFragment string) {
		t.Helper()
		wakeWatchdog(at)
		rows, _ := events.ReadSince(0)
		if len(seen) != count || len(rows) != count {
			t.Fatalf("at +%ds: desktop notices=%d journal events=%d, want %d; notices=%#v", at-now, len(seen), len(rows), count, seen)
		}
		if count > 0 && (rows[count-1].Event != "gtmux:wake-degraded" ||
			!strings.Contains(seen[count-1].Message, lastFragment) || rows[count-1].Summary != seen[count-1].Message) {
			t.Fatalf("at +%ds: expected matching journal and desktop notice containing %q: %#v", at-now, lastFragment, seen)
		}
	}
	writeReceipt := func(batch string, since time.Time) string {
		t.Helper()
		path := filepath.Join(evidenceDir, batch+".json")
		write(path, fmt.Sprintf(`{"pane":"%%99","batch":"#%s","payload":"synthetic batch","since":%d,"receipt":{"request":%q,"home":"fixture-home","session":"fixture-session","turn":%q}}`,
			batch, since.UnixNano(), "fixture-request-"+batch, "fixture-turn-"+batch))
		return path
	}

	const missingProof = "receiver accepted a wake without independent"
	check(now, 0, "")
	first := writeReceipt("111aaa", time.Unix(now, 250_000_000))
	check(now+6, 0, "")
	check(now+120, 0, "")
	check(now+121, 1, missingProof)
	if !strings.Contains(seen[0].Message, wakeObservedAt(now+121)) {
		t.Fatalf("expected the first time this receipt was overdue: %s", seen[0].Message)
	}
	second := writeReceipt("222bbb", time.Unix(now+122, 0))
	check(now+243, 1, missingProof)
	check(now+121+wakeNoticeInterval+1, 2, missingProof)
	if !strings.Contains(seen[1].Message, wakeObservedAt(now+242)) {
		t.Fatalf("expected the newer receipt's deadline in the notice: %s", seen[1].Message)
	}
	check(now+121+wakeNoticeInterval+2, 2, missingProof)

	for _, path := range []string{first, second} {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	after := now + 121 + wakeNoticeInterval + 3
	check(after, 2, missingProof)
	notices, err := readWakeNotices(filepath.Join(healthDir, "notices-channel.json"))
	if err != nil || notices["accepted-unverified"] != 0 {
		t.Fatalf("the missing-proof notice remains active after settlement: %v, %v", notices, err)
	}
	writeIssue("observation-archived", now-1200, int(after+1))
	check(after+1, 3, "exceeded the active proof window")
	if strings.Contains(seen[2].Message, missingProof) {
		t.Fatalf("archive reactivated the missing-proof issue: %s", seen[2].Message)
	}
}
