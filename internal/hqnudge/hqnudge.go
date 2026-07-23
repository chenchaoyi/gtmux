// Package hqnudge delivers a compact event line into the live gtmux HQ pane WITHOUT
// ever clobbering or auto-submitting a half-typed draft the user is composing there —
// and without ever losing the line if the delivery fails.
//
// The bug it first fixed: nudges were injected with `send-keys … Enter`. If the user
// was mid-typing in HQ when a nudge fired, the nudge text concatenated onto the draft
// AND the trailing Enter submitted the user's half-written command. Data loss.
//
// The guard: before typing, read the HQ input box (reusing the #393 dispatch region
// detector via dispatch.DraftOf) and check the pane isn't in tmux copy-mode. Deliver
// ONLY when the box is confirmed empty over TWO frames a short interval apart and the
// pane is not scrolling; otherwise the nudge is queued to disk and NOTHING is typed. A
// queued nudge is flushed (coalesced) on the next empty box — the next injection
// attempt, HQ's own turn-end (Stop, box reliably empty), or the serve fast tick.
//
// INVARIANT: no code path here submits USER content — Enter is never sent into a box
// holding anything except gtmux's OWN stranded wake batch, and only after the batch
// (head AND trailing id) is confirmed still intact in the draft (the Enter-only
// repair below).
//
// The hook is a short-lived process per event, so the queue is PERSISTENT (a dir of
// one-file-per-nudge under state.Dir()), not in-memory. Draining claims each file with
// an atomic rename so two concurrent hooks can't double-deliver one nudge.
//
// Delivery is ACKED (hq-wake-reliability), three layers deep (agent-drivers P2):
// the DRIVER RECEIPT first — the HQ session's own UserPromptSubmit event carrying
// the batch id (the hook records it; deterministic, immune to the history scrolling
// out of capture reach) — then the screen read (id in history and not in the draft).
// An id still sitting in the DRAFT is the third, precise verdict: the paste landed
// but the Enter was swallowed. That batch is NOT requeued (our own paste would block
// every later drain at the draft-guard — the failure mode that once stranded the
// channel until the 10-minute stale escalation): its claims are parked as `.stuck`
// and the next drain (the 3s serve fast tick) re-sends ONLY Enter, after confirming
// the draft still holds the batch intact — never a re-paste, never a new id. A
// paste/Enter error or a missing ack renames the claim back and a later drain
// retries it. A claim whose drainer died is reclaimed after orphanClaimAge. Because
// a screen is not a transactional sink, delivery is at-least-once: every batch
// carries a short `#<id>` that is STABLE across a re-send, so HQ can recognize (and
// the playbook tells it to ignore) a duplicate.
package hqnudge

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/driver"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// twoFrameGap is the interval between the two empty-box reads. It shrinks (cannot
// eliminate) the window where the user starts typing between the check and the send.
// It doubles as the settle gap before the delivery ack read.
const twoFrameGap = 300 * time.Millisecond

// orphanClaimAge is how long a `.sending` claim may sit before a later drain reclaims
// it. A live drain lives for a paste + Enter + one ack frame (well under a second);
// a claim older than this belongs to a drainer that DIED — the hook is a short-lived
// process and tmux can kill its pane mid-drain. Without reclaim those nudges are
// invisible to both hasPending and drainInto forever (they scan `.txt` only).
const orphanClaimAge = 60 * time.Second

// maxBatchLines and maxBatchChars cap ONE coalesced delivery. A backlog used to join
// unboundedly — and a multi-kilobyte paste is exactly the payload shape that fails to
// land (and that an agent TUI folds into a "[Pasted text +N lines]" placeholder, which
// would hide the id our ack looks for). The remainder stays queued and drains on the
// next (3s) tick, so a cap costs latency at worst, never an event.
const (
	maxBatchLines = 8
	maxBatchChars = 800
)

// maxQueueEntries bounds the queue. Past it, the lowest-priority OLDEST entry is
// evicted: a queue this deep means HQ has been away for hours, and dropping a standing
// warning that will re-fire on the next tick beats failing to paste a goal-changed.
const maxQueueEntries = 200

// wakeFailLimit is how many consecutive unconfirmed deliveries mark the channel
// degraded, and staleQueueSecs how long a due entry may sit undelivered while an HQ is
// live before the same verdict is reached (the stuck-draft case: a delivery whose Enter
// was swallowed leaves our own paste in the box, so no further attempt is even made).
const (
	wakeFailLimit  = 3
	staleQueueSecs = 10 * 60
)

// maxAckAttempts bounds how many times an entry may be PASTED without its delivery
// being confirmed. It exists because the two failure modes are not alike:
//
//   - a paste/Enter ERROR means nothing landed, so an unbounded retry is free and
//     correct — it costs no duplicate and recovers the moment tmux does;
//   - a missed ACK means the batch may well have landed and only the confirmation was
//     lost. Retrying forever would then re-paste a line HQ can already see, once per
//     drain, for as long as the queue lives.
//
// So an unconfirmed entry is re-sent at most twice more (carrying the same id, which
// the playbook tells HQ to read as a re-send) and then dropped — with wake-degraded
// already raised. A bounded, ANNOUNCED loss beats an unbounded spam loop, and beats
// the silent loss this whole change exists to end.
const maxAckAttempts = 3

// sendingSuffix marks a queue entry claimed by a drainer.
const sendingSuffix = ".sending"

// stuckSuffix marks a claim whose batch is sitting UNSUBMITTED in the HQ box (paste
// landed, Enter swallowed — the id proves it). Parked, not requeued: the Enter-only
// repair owns it until the delivery is confirmed or the repair is abandoned.
const stuckSuffix = ".stuck"

// enterRepairMaxAttempts bounds how many times the repair may re-send Enter without
// the delivery being confirmed. Past it the batch is handed back to the normal
// unacked path (bounded re-send with the SAME id, degraded counting) — a TUI that
// eats three Enters is not going to take the fourth.
const enterRepairMaxAttempts = 3

// queueDir holds one file per pending HQ nudge (`<due>-<pid>.p<prio>.txt`).
func queueDir() string { return filepath.Join(state.Dir(), "hq-nudges") }

// failCountPath holds the consecutive delivery-failure counter (text int).
func failCountPath() string { return filepath.Join(queueDir(), "fail-count") }

// io is the injectable I/O surface — real tmux in production, fakes in tests.
type io struct {
	capture      func(pane string) string // visible-screen capture (the input box is at its foot)
	captureColor func(pane string) string // COLOR capture for the draft-guard (drops CC's faint ghost text); nil → falls back to capture
	captureFull  func(pane string) string // capture + scrollback margin (the ack read)
	paste        func(pane, text string) error
	enter        func(pane string) error
	sleep        func() // wait the two-frame interval
	nowNano      func() int64
	inMode       func(pane string) bool // pane is in tmux copy/view-mode (scrolling)
	// receipt asks the HQ agent's driver for the delivery receipt: has the pane's
	// session recorded a prompt-submit carrying needle (the batch id) since the
	// given unix second? nil / NoEvidence → the screen ack judges (I2). The driver
	// kill-switches act here: receipt off resolves to NoEvidence.
	receipt func(pane, needle string, since int64) driver.Verdict
}

var prod = io{
	capture:      tmux.CapturePane,
	captureColor: tmux.CaptureFullColor,
	captureFull:  tmux.CaptureFull,
	paste:        tmux.Paste,
	enter:        func(pane string) error { return tmux.SendKey(pane, "Enter") },
	sleep:        func() { time.Sleep(twoFrameGap) },
	nowNano:      func() int64 { return time.Now().UnixNano() },
	inMode:       tmux.InMode,
	receipt: func(pane, needle string, since int64) driver.Verdict {
		d := driver.For(strings.TrimSpace(tmux.Display(pane, "#{pane_current_command}")))
		if d.Receipt == nil {
			return driver.NoEvidence
		}
		return d.Receipt(pane, needle, since)
	},
}

// Deliver types msg into the HQ pane, guarding a half-typed draft. msg is queued to
// disk first; if the HQ box is confirmed empty over two frames, the queue (msg plus
// any backlog) is flushed coalesced. If the box holds a draft, msg stays queued and
// NOTHING is typed. Public entry — every HQ injection goes through here.
func Deliver(pane, msg string) { deliver(prod, pane, msg) }

func deliver(x io, pane, msg string) {
	if pane == "" || strings.TrimSpace(msg) == "" {
		return
	}
	enqueue(x, msg) // to disk; nothing typed yet
	repairStranded(x, pane)
	if boxEmpty(x, pane) {
		drainInto(x, pane) // one coalesced send — the only place a batch is typed
	}
}

// Enqueue queues msg for a LATER drain without attempting delivery — the no-HQ-right-
// now path (hq-wake-reliability): a wake whose HQ pane momentarily doesn't resolve is
// held, not dropped, and lands on the next drain that finds one. Callers gate this on
// "an HQ was seen recently" so a machine that simply runs no HQ queues nothing.
func Enqueue(msg string) {
	if strings.TrimSpace(msg) == "" {
		return
	}
	enqueue(prod, msg)
}

// DeliverKeyedAt queues msg under a REPLACEABLE key with a due time (unix nanos):
// a later call with the same key REPLACES the pending payload instead of adding a
// second line, and the entry is not typed before it is due. This is the per-pane
// done merge window (hq-perception-v2): rapid-fire completions of one pane collapse
// to one line carrying the newest payload, delivered when the window closes (by the
// next Deliver/Drain after dueNano). Same draft-guard invariants as Deliver.
func DeliverKeyedAt(pane, key, msg string, dueNano int64) {
	deliverKeyedAt(prod, pane, key, msg, dueNano)
}

func deliverKeyedAt(x io, pane, key, msg string, dueNano int64) {
	if pane == "" || strings.TrimSpace(msg) == "" || strings.TrimSpace(key) == "" {
		return
	}
	enqueueKeyed(x, key, msg, dueNano)
	repairStranded(x, pane)
	if boxEmpty(x, pane) {
		drainInto(x, pane) // flushes whatever is DUE (this entry only once due)
	}
}

// Drain flushes any queued nudges into the HQ pane when its box is empty (coalesced).
// Called on HQ's own turn-end (box reliably empty) and on the serve fast tick, so a
// queued nudge lands even with no further Deliver call. No-op on a drafted box or an
// empty queue.
func Drain(pane string) { drain(prod, pane) }

func drain(x io, pane string) {
	if pane == "" || !hasPending() {
		return // cheap gate: skip the capture/sleep when nothing is queued
	}
	repairStranded(x, pane) // a stranded batch is finished BEFORE the draft-guard sees it
	if boxEmpty(x, pane) {
		drainInto(x, pane)
	}
}

// Pending reports whether any nudge is queued behind a draft. Cheap (a dir scan, no
// tmux) so callers can gate the more expensive Drain (which captures the pane) on it.
// A stranded claim counts as pending — otherwise the gate would hide the very entries
// the orphan reclaim exists to rescue.
func Pending() bool { return hasPending() }

func hasPending() bool {
	for _, e := range readQueue() {
		if strings.HasSuffix(e.Name(), ".txt") || strings.HasSuffix(e.Name(), stuckSuffix) ||
			isOrphanClaim(e) {
			return true
		}
	}
	return false
}

// Degraded reports whether the wake channel is failing to reach HQ: either
// wakeFailLimit consecutive deliveries went unconfirmed, or a due entry has been stuck
// in the queue past staleQueueSecs (a swallowed Enter leaves our own paste in the box,
// which blocks every later attempt at the draft guard — so the failure counter alone
// would never climb). Callers gate on a live HQ pane: no HQ means nothing to wake, not
// a degradation. now is unix seconds.
func Degraded(now int64) bool {
	if readFailCount() >= wakeFailLimit {
		return true
	}
	oldest := int64(0)
	for _, e := range readQueue() {
		if !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		if dueOf(e.Name()) > now*int64(time.Second) {
			continue // still inside its merge window — not stuck, just not due
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if ts := info.ModTime().Unix(); oldest == 0 || ts < oldest {
			oldest = ts
		}
	}
	return oldest > 0 && now-oldest > staleQueueSecs
}

// readQueue lists the queue dir (empty on any error — an absent dir is an empty queue).
func readQueue() []os.DirEntry {
	entries, _ := os.ReadDir(queueDir())
	out := entries[:0]
	for _, e := range entries {
		if !e.IsDir() && e.Name() != filepath.Base(failCountPath()) &&
			e.Name() != filepath.Base(repairPath()) {
			out = append(out, e)
		}
	}
	return out
}

// isOrphanClaim reports whether e is a claim old enough that its owner must be
// gone: a `.sending` whose drainer died, or a `.stuck` whose repair marker was
// lost (a live repair rewrites/removes its marker well inside this window).
func isOrphanClaim(e os.DirEntry) bool {
	switch {
	case strings.HasSuffix(e.Name(), sendingSuffix):
	case strings.HasSuffix(e.Name(), stuckSuffix):
		if _, ok := readRepair(); ok {
			return false // the repair loop owns it
		}
	default:
		return false
	}
	info, err := e.Info()
	return err == nil && time.Since(info.ModTime()) > orphanClaimAge
}

// reclaimOrphans returns stranded claims to the queue. Run at the head of every drain.
// A reclaim can race a live-but-slow drainer, in which case the batch delivers twice —
// with the SAME batch id, which is the documented (and recognizable) duplicate. The
// alternative it replaces was a permanent silent loss.
func reclaimOrphans() {
	for _, e := range readQueue() {
		if !isOrphanClaim(e) {
			continue
		}
		src := filepath.Join(queueDir(), e.Name())
		_ = os.Rename(src, claimBase(src))
	}
}

// claimBase strips a claim suffix (`.sending` / `.stuck`), returning the entry's
// queued (`.txt`) path/name.
func claimBase(name string) string {
	return strings.TrimSuffix(strings.TrimSuffix(name, sendingSuffix), stuckSuffix)
}

// boxEmpty reports whether the HQ input box is empty, confirmed over TWO frames. A
// non-empty first frame returns false immediately (no sleep, no send). A capture with
// no locatable input region (structured == false) is treated as NOT empty — we only
// ever type into a confirmed-empty, structured box.
func boxEmpty(x io, pane string) bool {
	// Copy/view-mode: the user is scrolling, and injected keys are eaten as copy-mode
	// NAV commands (`f` → jump-forward, yellow residue) instead of reaching the box.
	// Treat it exactly like a non-empty draft — do not inject; queue, deliver on exit.
	if x.inMode != nil && x.inMode(pane) {
		return false
	}
	// COLOR-aware draft read: exclude CC's faint suggested-next-command ghost text, which
	// a plain capture would misread as a half-typed draft and HOLD the nudge behind a
	// phantom. DraftOfColored is a safe superset of DraftOf (identity on plain text), so a
	// test injecting a plain capture is unchanged. Fall back to `capture` when no color
	// capture is wired.
	cap := x.captureColor
	if cap == nil {
		cap = x.capture
	}
	if draft, structured := dispatch.DraftOfColored(cap(pane)); !structured || strings.TrimSpace(draft) != "" {
		return false
	}
	x.sleep()
	draft, structured := dispatch.DraftOfColored(cap(pane))
	return structured && strings.TrimSpace(draft) == ""
}

// enqueue writes one pending-nudge file. The name is time-ordered (fixed-width nanos)
// so a lexical sort on drain is oldest-first; the pid suffix avoids collisions between
// concurrent hook processes, and the `.p<n>` field carries the wake class's priority.
func enqueue(x io, msg string) {
	if err := os.MkdirAll(queueDir(), 0o755); err != nil {
		return
	}
	evictOverflow()
	name := fmt.Sprintf("%019d-%d.p%d.txt", x.nowNano(), os.Getpid(), hqwake.PriorityOf(msg))
	_ = os.WriteFile(filepath.Join(queueDir(), name), []byte(msg), 0o644)
}

// enqueueKeyed writes/REPLACES the pending entry for key. The name embeds the due
// time in the same fixed-width-nanos prefix (so lexical order stays time order and
// drain's due check can read it back); a replacement keeps the ORIGINAL due (window
// position) and only swaps the payload for the newest one.
func enqueueKeyed(x io, key, msg string, dueNano int64) {
	if err := os.MkdirAll(queueDir(), 0o755); err != nil {
		return
	}
	// The field is matched with its trailing dot rather than `.txt`, so an entry that
	// has been re-sent (`.k-<key>.a1.txt`) is still this key's slot.
	field := keyField(key) + "."
	for _, e := range readQueue() {
		if strings.Contains(e.Name(), field) && strings.HasSuffix(e.Name(), ".txt") {
			// Replace in place: newest payload, original due position.
			_ = os.WriteFile(filepath.Join(queueDir(), e.Name()), []byte(msg), 0o644)
			return
		}
	}
	evictOverflow()
	name := fmt.Sprintf("%019d-0.p%d%s.txt", dueNano, hqwake.PriorityOf(msg), keyField(key))
	_ = os.WriteFile(filepath.Join(queueDir(), name), []byte(msg), 0o644)
}

// evictOverflow keeps the queue under maxQueueEntries by dropping the LOWEST-priority
// OLDEST entry (never a claim in flight). Deliberate priority inversion: a standing
// resource warning re-fires on the next tick, a goal-changed never does — and the
// oldest standing warning is also the most stale reading of the two.
func evictOverflow() {
	names := queuedNames(func(string) bool { return true })
	for len(names) >= maxQueueEntries {
		i := len(names) - 1
		for lowest := prioOf(names[i]); i > 0 && prioOf(names[i-1]) == lowest; i-- {
		}
		_ = os.Remove(filepath.Join(queueDir(), names[i]))
		names = append(names[:i], names[i+1:]...)
	}
}

// queuedNames returns the `.txt` entry names passing keep, ordered for delivery:
// highest priority first, oldest first within a priority.
func queuedNames(keep func(name string) bool) []string {
	var names []string
	for _, e := range readQueue() {
		if strings.HasSuffix(e.Name(), ".txt") && keep(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names) // oldest-first (fixed-width nanos prefix)
	sort.SliceStable(names, func(i, j int) bool { return prioOf(names[i]) < prioOf(names[j]) })
	return names
}

// keyField renders a merge key as its filename field.
func keyField(key string) string { return ".k-" + sanitizeKey(key) }

// sanitizeKey makes a queue-filename-safe key (pane ids like "%14" pass through;
// path separators cannot). Dots go too: they separate the name's OWN fields, so a
// dotted key could otherwise be misread as a priority or an attempt counter.
func sanitizeKey(key string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', '.', 0:
			return '_'
		}
		return r
	}, key)
}

// dueOf parses the fixed-width nanos prefix of a queue filename (its due time).
// Malformed names read as 0 (always due) so a legacy entry can never be stranded.
func dueOf(name string) int64 {
	i := strings.IndexByte(name, '-')
	if i <= 0 {
		return 0
	}
	n, err := strconv.ParseInt(name[:i], 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// A queue filename is `<due:019d>-<pid>.p<prio>[.k-<key>][.a<n>].txt`. Everything a
// drain needs to order, hold, and identify an entry is in the name — the queue is a
// directory of files claimed by rename, and a sidecar index would need its own crash
// story. Every field is optional on read: an entry written by an OLDER gtmux (no
// `.p`, no `.a`) parses as the default priority, zero attempts, and due now, so an
// in-flight upgrade drains its backlog instead of stranding it.

// prioOf parses the `.p<n>` field. The key is excluded from the search so a key can
// never be mistaken for the priority.
func prioOf(name string) int {
	if i := strings.Index(name, ".k-"); i >= 0 {
		name = name[:i]
	}
	i := strings.Index(name, ".p")
	if i < 0 || i+2 >= len(name) {
		return hqwake.PriorityDefault
	}
	n, err := strconv.Atoi(string(name[i+2]))
	if err != nil {
		return hqwake.PriorityDefault
	}
	return n
}

// attemptsOf parses the `.a<n>` field: how many times this entry has been pasted
// without its delivery being confirmed. A key can never be confused for it —
// sanitizeKey strips dots.
func attemptsOf(name string) int {
	base := identityOf(name)
	if base == strings.TrimSuffix(name, ".txt") {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSuffix(name, ".txt")[len(base)+2:])
	if err != nil {
		return 0
	}
	return n
}

// identityOf strips the attempt counter (and the extension), leaving the entry's
// STABLE identity. batchID hashes these, so a re-send of the same entries produces
// the same id no matter how many attempts it took.
func identityOf(name string) string {
	base := strings.TrimSuffix(name, ".txt")
	i := strings.LastIndex(base, ".a")
	if i < 0 {
		return base
	}
	if _, err := strconv.Atoi(base[i+2:]); err != nil {
		return base
	}
	return base[:i]
}

// withAttempt renames an entry to carry attempt n.
func withAttempt(name string, n int) string {
	return fmt.Sprintf("%s.a%d.txt", identityOf(name), n)
}

// batchID is the batch's short identity: 6 hex of the claimed entries' identities
// plus the payload. Those identities carry each entry's nanos, so a RE-SEND of an
// unconfirmed batch (which reclaims the same files) hashes identically, while a
// genuinely new batch with the same text does not. It is what the ack matches on —
// the line's own head is shared with every earlier wake of the same class about the
// same pane; the id is not.
func batchID(claims []string, payload string) string {
	h := sha256.New()
	for _, c := range claims {
		h.Write([]byte(identityOf(strings.TrimSuffix(filepath.Base(c), sendingSuffix))))
		h.Write([]byte{0})
	}
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))[:6]
}

// drainInto delivers the queue as ONE coalesced line. It MUST be called only after
// boxEmpty confirmed an empty box (the invariant). Each file is claimed by an atomic
// rename to `.sending` so a concurrent drainer can't re-deliver it; a lost claim just
// skips that file (at worst a coalesce splits across two lines — never a loss/dup).
// A claim is REMOVED only once the delivery is acked; otherwise it is returned to the
// queue for a later attempt.
func drainInto(x io, pane string) {
	reclaimOrphans()
	due := queuedNames(func(n string) bool { return dueOf(n) <= x.nowNano() })
	claimed, msgs, held := claimBatch(due)
	if len(msgs) == 0 {
		for _, c := range claimed {
			_ = os.Remove(c) // empty/unreadable — there is nothing to deliver or keep
		}
		return
	}

	payload := strings.Join(msgs, " · ")
	if held > 0 {
		payload += fmt.Sprintf(" · +%d more queued", held)
	}
	id := "#" + batchID(claimed, payload)
	payload += " · " + id

	since := x.nowNano()/int64(time.Second) - 1 // receipt window opens just before the paste
	switch deliverPayload(x, pane, payload, id, since) {
	case delivered:
		for _, c := range claimed {
			_ = os.Remove(c)
		}
		writeFailCount(0)
	case sendFailed:
		// Nothing reached the pane — return the batch untouched (no attempt spent,
		// no duplicate risk) and let a later drain retry it as often as it likes.
		for _, c := range claimed {
			_ = os.Rename(c, claimBase(c))
		}
		writeFailCount(readFailCount() + 1)
	case unsubmitted:
		// The precise swallowed-Enter: the paste (its id proves it) sits in the box.
		// Requeuing would strand the whole channel — our own paste blocks every later
		// drain at the draft-guard, and only the 10-minute stale check would escalate
		// (the real incident). Park the claims for the Enter-only repair instead: the
		// next drain (3s fast tick) finishes the delivery. Not a fail-count event yet
		// — an ABANDONED repair counts it.
		markStuck(claimed, payload, id, since)
	case unacked:
		requeueUnacked(claimed)
		writeFailCount(readFailCount() + 1)
	}
}

// claimBatch claims up to one delivery's worth of due entries (highest priority
// first, oldest first within a priority) and returns the claims, their payloads, and
// how many due entries were held back for the next drain.
func claimBatch(due []string) (claimed, msgs []string, held int) {
	chars := 0
	for i, n := range due {
		if len(msgs) >= maxBatchLines || (len(msgs) > 0 && chars > maxBatchChars) {
			return claimed, msgs, len(due) - i
		}
		src := filepath.Join(queueDir(), n)
		dst := src + sendingSuffix
		if os.Rename(src, dst) != nil {
			continue // lost the claim to another drainer
		}
		claimed = append(claimed, dst)
		b, err := os.ReadFile(dst)
		if err != nil {
			continue
		}
		if s := strings.TrimSpace(string(b)); s != "" {
			msgs = append(msgs, s)
			chars += len(s)
		}
	}
	return claimed, msgs, 0
}

// requeueUnacked returns each claim for one more attempt, or drops it once it has
// been pasted maxAckAttempts times without a confirmation. See maxAckAttempts: by
// then the line has very probably been on HQ's screen three times over, and the
// wake-degraded escalation has already announced the doubt.
func requeueUnacked(claimed []string) {
	for _, c := range claimed {
		orig := claimBase(filepath.Base(c))
		if n := attemptsOf(orig) + 1; n < maxAckAttempts {
			_ = os.Rename(c, filepath.Join(queueDir(), withAttempt(orig, n)))
		} else {
			_ = os.Remove(c)
		}
	}
}

// outcome is what one delivery attempt achieved. The distinctions drive the retry
// policy: sendFailed (nothing reached the pane) retries freely, unacked (it may
// have landed) retries bounded, and unsubmitted (it demonstrably did NOT submit —
// the paste sits in the box) is repaired with Enter only, never re-sent.
type outcome int

const (
	delivered   outcome = iota // confirmed (driver receipt or HQ's screen)
	sendFailed                 // tmux refused the paste or the submit — nothing landed
	unacked                    // pasted and submitted, but not confirmed
	unsubmitted                // pasted, but the Enter was swallowed: the id sits in the draft
)

// deliverPayload types ONE batch and confirms it landed, three layers deep. Paste
// and submit are separate steps (`send-keys -l` of a long string mid-TUI errors
// "not in a mode" — the failure tmux.Paste exists to avoid). The ack: the DRIVER
// RECEIPT first — the HQ session's own prompt-submit event carrying the batch id,
// deterministic and immune to the history scrolling out of the capture's reach the
// moment HQ starts its turn — then the screen. On screen, an id still in the DRAFT
// is the precise swallowed-Enter verdict (unsubmitted → Enter-only repair); in the
// history and not the draft is delivered; neither is an unconfirmed send.
func deliverPayload(x io, pane, payload, id string, since int64) outcome {
	if err := x.paste(pane, payload); err != nil {
		return sendFailed
	}
	if err := x.enter(pane); err != nil {
		// One retry: a swallowed submit leaves our paste sitting in HQ's box, which
		// blocks every later drain at the draft guard.
		if err = x.enter(pane); err != nil {
			return sendFailed
		}
	}
	x.sleep()
	if receiptConfirmed(x, pane, id, since) {
		return delivered
	}
	history, draft, _ := dispatch.SplitInputRegion(x.captureFull(pane))
	switch {
	case dispatch.ContainsHead(draft, id):
		return unsubmitted
	case dispatch.ContainsHead(history, id):
		return delivered
	default:
		return unacked
	}
}

// receiptConfirmed asks the HQ agent's driver receipt whether the batch id was
// submitted. Positive-only (I2): NoEvidence defers to the screen, never fails.
func receiptConfirmed(x io, pane, id string, since int64) bool {
	return x.receipt != nil && x.receipt(pane, id, since) == driver.Confirmed
}

// ackConfirmed is the shared "did this batch land" check — receipt first, then the
// history-not-draft screen read. Used by the Enter-only repair to confirm its work.
func ackConfirmed(x io, pane, id string, since int64) bool {
	if receiptConfirmed(x, pane, id, since) {
		return true
	}
	history, draft, _ := dispatch.SplitInputRegion(x.captureFull(pane))
	return dispatch.ContainsHead(history, id) && !dispatch.ContainsHead(draft, id)
}

// --- Enter-only repair (agent-drivers P2) ---

// repairState is the on-disk record of a stranded batch awaiting its Enter-only
// repair: the batch id (the ack needle), the full payload (to confirm the draft
// still holds the batch INTACT before any Enter), the receipt window origin, and
// how many repair Enters have gone unconfirmed.
type repairState struct {
	ID       string `json:"id"`
	Payload  string `json:"payload"`
	Since    int64  `json:"since"` // unix secs: the original delivery's receipt window
	Attempts int    `json:"attempts"`
}

// repairPath holds the single pending repair (one delivery is in flight at a time —
// a stranded batch blocks the channel until repaired or handed back).
func repairPath() string { return filepath.Join(queueDir(), "enter-repair") }

func readRepair() (repairState, bool) {
	var m repairState
	b, err := os.ReadFile(repairPath())
	if err != nil || json.Unmarshal(b, &m) != nil || m.ID == "" {
		return repairState{}, false
	}
	return m, true
}

func writeRepair(m repairState) {
	b, _ := json.Marshal(m)
	_ = os.WriteFile(repairPath(), b, 0o644)
}

// markStuck parks a batch's claims for the Enter-only repair and records the
// repair state. The claims keep their identity (same files, same batch id).
func markStuck(claimed []string, payload, id string, since int64) {
	for _, c := range claimed {
		_ = os.Rename(c, claimBase(c)+stuckSuffix)
	}
	writeRepair(repairState{ID: id, Payload: payload, Since: since})
}

// stuckClaims lists the parked claims' paths.
func stuckClaims() []string {
	var out []string
	for _, e := range readQueue() {
		if strings.HasSuffix(e.Name(), stuckSuffix) {
			out = append(out, filepath.Join(queueDir(), e.Name()))
		}
	}
	return out
}

// finishRepair clears a completed repair: the delivery is confirmed, so the claims
// and the repair record go, and the channel is healthy again.
func finishRepair() {
	for _, c := range stuckClaims() {
		_ = os.Remove(c)
	}
	_ = os.Remove(repairPath())
	writeFailCount(0)
}

// repairStranded finishes a delivery whose Enter was swallowed: the batch (with its
// id) is sitting intact in the HQ box, so the ONLY correct repair is a bare Enter —
// never a re-paste (it would duplicate the text into the box) and never a new id.
// It runs at the head of every deliver/drain, so the serve fast tick (3s) repairs a
// stranded batch on its next beat instead of the channel jamming behind our own
// paste until the 10-minute stale escalation (the real incident).
//
// The Enter is sent ONLY after the draft is confirmed to still hold the batch whole
// (payload head AND trailing id — the dispatch "draft still intact" discipline); a
// draft the user has edited or cleared is never submitted. In that case — or once
// the repair budget is exhausted — the batch is handed back to the normal unacked
// path (bounded re-send with the SAME id) and the miss is counted toward
// wake-degraded, exactly the pre-repair discipline.
func repairStranded(x io, pane string) {
	m, ok := readRepair()
	if !ok {
		return
	}
	cap := x.captureColor
	if cap == nil {
		cap = x.capture
	}
	draft, structured := dispatch.DraftOfColored(cap(pane))
	if structured && dispatch.ContainsHead(draft, m.Payload) && dispatch.ContainsHead(draft, m.ID) {
		if x.enter(pane) != nil {
			return // tmux refused — nothing changed; the next tick retries freely
		}
		x.sleep()
		if ackConfirmed(x, pane, m.ID, m.Since) {
			finishRepair()
			return
		}
		if m.Attempts++; m.Attempts < enterRepairMaxAttempts {
			writeRepair(m)
			return
		}
		// budget exhausted — fall through to the hand-back below.
	} else if ackConfirmed(x, pane, m.ID, m.Since) {
		// The batch left the box and is confirmed (the user pressed Enter for us, or
		// an earlier repair's ack read raced the TUI's redraw): done.
		finishRepair()
		return
	}
	requeueUnacked(stuckClaims())
	_ = os.Remove(repairPath())
	writeFailCount(readFailCount() + 1)
}

func readFailCount() int {
	n, _ := strconv.Atoi(state.ReadMarker(failCountPath()))
	return n
}

func writeFailCount(n int) {
	if n <= 0 {
		state.Remove(failCountPath())
		return
	}
	_ = state.WriteMarker(failCountPath(), strconv.Itoa(n))
}
