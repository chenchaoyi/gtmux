package hqnudge

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
)

func TestHWA003IncomingStandingWakeCannotEvictDecisions(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{}
	x := f.io()
	all := func(string) bool { return true }
	for i := 0; i < maxQueueEntries; i++ {
		enqueue(x, fmt.Sprintf("» ◆ gtmux·goal-changed  decision-%d", i))
	}
	before := queuedNames(all)
	enqueue(x, "» · gtmux·self-rotate standing-new")
	if after := queuedNames(all); !slices.Equal(before, after) {
		t.Fatal("lower-ranking newcomer displaced an existing decision")
	}
	enqueue(x, "» ◆ gtmux·goal-changed equal-priority-new")
	after := queuedNames(all)
	if len(after) != maxQueueEntries || slices.Contains(after, before[0]) {
		t.Fatal("equal-priority admission did not replace exactly the oldest decision")
	}
	drops := auditRecords(t, events.AuditEventWakeDropped)
	if len(drops) != 2 || !strings.HasPrefix(drops[0].Summary, events.DropQueueFull+":") ||
		!strings.HasPrefix(drops[1].Summary, events.DropEvicted+":") {
		t.Fatalf("pressure rejection and eviction did not have distinct audit records: %#v", drops)
	}
}

func TestHWA004ConflictNeedsCleanScanAndPositiveReceipt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const base = int64(1700000000)
	nanos := func(s int64) int64 { return s * int64(time.Second) }
	recordIssue(issueClaimConflict, nanos(base))
	recordIssue(issueUnknownInput, nanos(base))
	noteConfirmedDelivery(base + 2)
	if got := ReadHealth(base + 2); len(got.Issues) != 2 {
		t.Fatalf("unrelated confirmation hid unresolved recovery evidence: %#v", got)
	}
	noteCleanRecovery(nanos(base + 3))
	if got := ReadHealth(base + 3); len(got.Issues) != 1 || got.Issues[0].Reason != issueUnknownInput {
		t.Fatalf("verified recovery should clear only the matching conflict: %#v", got)
	}
}
