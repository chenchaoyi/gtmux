// Package hqnudge delivers a compact event line into the live gtmux HQ pane WITHOUT
// ever clobbering or auto-submitting a half-typed draft the user is composing there.
// Failed delivery is retried; queue eviction and exhausted confirmations are journaled.
//
// The bug it first fixed: nudges were injected with `send-keys … Enter`. If the user
// was mid-typing in HQ when a nudge fired, the nudge text concatenated onto the draft
// AND the trailing Enter submitted the user's half-written command. Data loss.
//
// The guard: before typing, read the HQ input box (through dispatch.BoxEmpty — the one
// answer to "may I type here?" for every writer, over the #393 region detector and the
// faint-aware DraftOfColored) and check the pane isn't in tmux copy-mode. Deliver
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
// The journal records a submission boundary before input mutation. An unknown
// outcome keeps the original generation for late evidence and cannot authorize
// a replay. A positively refused native attempt may be scheduled with a bounded
// backoff. A terminal repair can press Enter only on an exact owned draft.
package hqnudge

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/driver"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqinput"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/resume"
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

// sendingSuffix marks a queue entry claimed by a drainer.
const sendingSuffix = ".sending"

// stuckSuffix marks a claim whose batch is sitting UNSUBMITTED in the HQ box (paste
// landed, Enter swallowed — the id proves it). Parked, not requeued: the Enter-only
// repair owns it until the delivery is confirmed or the repair is abandoned.
const stuckSuffix = ".stuck"

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
	// foreground names the pane's foreground command, so a drain can tell a shell
	// waiting for an agent to start from the agent itself. nil → unknown → proceed.
	nativePrepare func(pane string) func(payload, id string) (dispatch.Result, bool)
	nativeSubmit  func(pane, payload, id string) (dispatch.Result, bool) // isolated legacy transport seam
	foreground    func(pane string) string
	identity      func(pane string) string
	busy          func(pane string) bool
}

var prod = io{
	nativePrepare: prepareNativeWake,
	identity:      liveWakeIdentity,
	busy: func(pane string) bool {
		for _, path := range []string{state.ActivePath(pane), state.WaitingPath(pane)} {
			if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
				return true
			}
		}
		return false
	},
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
	foreground: func(pane string) string {
		if _, ok := resume.CodexFrontendForPane(pane); ok {
			return "codex"
		}
		return strings.TrimSpace(tmux.Display(pane, "#{pane_current_command}"))
	},
}

// agentUp reports whether something other than a bare shell holds the HQ pane — the
// precondition for typing a wake into it at all.
//
// The draft guard cannot answer this: a shell prompt has no input box, so BoxEmpty
// fails open there (its job is to protect a send, never to block one), and the queue
// drained into bash. Measured 2026-09-16 19:56:20: `gtmux hq --here` stamped %31 and
// typed the agent command; the 3s tick found the stamp, found the "box" empty, and
// pasted four backlog batches into the shell eight seconds before the agent's
// SessionStart — dropped as unconfirmed, and the commander watched them land ("感觉hq
// 还没启动起来，就塞了一大堆的信息进去"). A shell in the HQ pane means the agent is not
// up yet (or has quit); either way the batch keeps until it is. Unknown → proceed.
func agentUp(x io, pane string) bool {
	if x.foreground == nil {
		return true
	}
	return !isShellFg(x.foreground(pane))
}

// isShellFg mirrors hq.isShellCommand (this package sits below hq): a bare
// login/interactive shell, with or without the login-shell dash.
func isShellFg(name string) bool {
	switch strings.TrimPrefix(name, "-") {
	case "bash", "zsh", "fish", "sh", "dash", "tcsh", "ksh":
		return true
	}
	return false
}

// Deliver types msg into the HQ pane, guarding a half-typed draft. msg is queued to
// disk first; if the HQ box is confirmed empty over two frames, the queue (msg plus
// any backlog) is flushed coalesced. If the box holds a draft, msg stays queued and
// NOTHING is typed. Public entry — every HQ injection goes through here.
func Deliver(pane, msg string) {
	if pane == "" || strings.TrimSpace(msg) == "" {
		return
	}
	enqueue(prod, msg)
	withHQInput(func() { drain(prod, pane) })
}

func deliver(x io, pane, msg string) {
	if pane == "" || strings.TrimSpace(msg) == "" {
		return
	}
	enqueue(x, msg) // to disk; nothing typed yet
	if !agentUp(x, pane) {
		return // held: the pane is a shell, the agent is not up — the next drain retries
	}
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

// EnqueueKeyed replaces one pending reminder without submitting input. It uses
// the same durable keyed queue as turn-end and periodic wake producers.
func EnqueueKeyed(key, msg string) {
	if strings.TrimSpace(key) == "" || strings.TrimSpace(msg) == "" {
		return
	}
	enqueueKeyed(prod, key, msg, 0)
}

// DeliverKeyedAt queues msg under a REPLACEABLE key with a due time (unix nanos):
// a later call with the same key REPLACES the pending payload instead of adding a
// second line, and the entry is not typed before it is due. This is the per-pane
// done merge window (hq-perception-v2): rapid-fire completions of one pane collapse
// to one line carrying the newest payload, delivered when the window closes (by the
// next Deliver/Drain after dueNano). Same draft-guard invariants as Deliver.
func DeliverKeyedAt(pane, key, msg string, dueNano int64) {
	if pane == "" || strings.TrimSpace(msg) == "" || strings.TrimSpace(key) == "" {
		return
	}
	enqueueKeyed(prod, key, msg, dueNano)
	withHQInput(func() { drain(prod, pane) })
}

func deliverKeyedAt(x io, pane, key, msg string, dueNano int64) {
	if pane == "" || strings.TrimSpace(msg) == "" || strings.TrimSpace(key) == "" {
		return
	}
	enqueueKeyed(x, key, msg, dueNano)
	if !agentUp(x, pane) {
		return // held until the agent holds the pane (agentUp)
	}
	repairStranded(x, pane)
	if boxEmpty(x, pane) {
		drainInto(x, pane) // flushes whatever is DUE (this entry only once due)
	}
}

// Drain flushes any queued nudges into the HQ pane when its box is empty (coalesced).
// Called on HQ's own turn-end (box reliably empty) and on the serve fast tick, so a
// queued nudge lands even with no further Deliver call. No-op on a drafted box or an
// empty queue.
func Drain(pane string) {
	if pane != "" && hasPending() {
		withHQInput(func() { drain(prod, pane) })
	}
}

func withHQInput(work func()) {
	release, acquired, err := hqinput.Acquire()
	if err != nil {
		diag.ReportStoreFailure("hq-input", err)
		recordIssue(issueRecoveryUnsafe, time.Now().UnixNano())
		return
	}
	if !acquired {
		return
	}
	defer release()
	work()
}

func drain(x io, pane string) {
	if pane == "" || !hasPending() {
		return // cheap gate: skip the capture/sleep when nothing is queued
	}
	if !agentUp(x, pane) {
		return // held until the agent holds the pane (agentUp)
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
		if isPendingWake(e.Name()) || strings.HasSuffix(e.Name(), stuckSuffix) ||
			strings.HasSuffix(e.Name(), legacyUncertainSuffix) ||
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
		if !isPendingWake(e.Name()) && !strings.HasSuffix(e.Name(), nativeUncertainSuffix) {
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

// readQueue is used by the active producers and drainer. Health inspection
// calls inspectQueue directly so merely inspecting the queue is read-only.
func readQueue() []os.DirEntry {
	entries, err := inspectQueue()
	if err != nil {
		queueFailure(time.Now().UnixNano(), err)
	}
	return entries
}

func inspectQueue() ([]os.DirEntry, error) {
	dir := queueDir()
	if err := state.CheckPrivateDirectory(dir); err != nil {
		return nil, ignoreNativeMissing(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, ignoreNativeMissing(err)
	}
	out := entries[:0]
	for _, e := range entries {
		if !e.IsDir() && e.Name() != filepath.Base(failCountPath()) &&
			e.Name() != filepath.Base(repairPath()) {
			out = append(out, e)
		}
	}
	return out, nil
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

// claimBase strips a claim suffix (`.sending` / `.stuck`), returning the entry's
// queued (`.txt`) path/name.
func claimBase(name string) string {
	return strings.TrimSuffix(strings.TrimSuffix(name, sendingSuffix), stuckSuffix)
}

// boxEmpty reports whether the HQ input box is empty. The decision itself lives in
// dispatch.BoxEmpty — one implementation for every path that types into a composer —
// and this only adapts the nudge's injectable io onto it.
func boxEmpty(x io, pane string) bool {
	capture := x.captureColor
	if capture == nil {
		capture = x.capture
	}
	return dispatch.BoxEmpty(dispatch.IO{
		CaptureColor: func() string { return capture(pane) },
		InMode:       func() bool { return x.inMode != nil && x.inMode(pane) },
		Sleep:        x.sleep,
	})
}

// enqueue writes one pending-nudge file. The name is time-ordered (fixed-width nanos)
// so a lexical sort on drain is oldest-first; the pid suffix avoids collisions between
// concurrent hook processes, and the `.p<n>` field carries the wake class's priority.
func enqueue(x io, msg string) {
	enqueueWithSuffix(x, msg, msg, ".txt")
}

func enqueueWithSuffix(x io, line, content, suffix string) {
	release, err := lockQueue()
	if err != nil {
		queueFailure(x.nowNano(), err)
		return
	}
	defer release()
	now := x.nowNano()
	name := fmt.Sprintf("%019d-%d-%s.p%d%s", now, os.Getpid(), diag.NewOpID(), hqwake.PriorityOf(line), suffix)
	wakeStage("act.wake.scheduled", name, "", diag.OK, "immediate")
	admitQueuedContent(name, line, content, now)
}

// enqueueKeyed replaces the still-queued generation in place to keep its due
// position. A newly queued generation needs its own physical name even when a
// previous generation of the same key is still held or was parked.
func enqueueKeyed(x io, key, msg string, dueNano int64) {
	release, err := lockQueue()
	if err != nil {
		queueFailure(x.nowNano(), err)
		return
	}
	defer release()
	field := keyField(key)
	now := x.nowNano()
	if name := pendingKey(field); name != "" {
		wakeStage("act.wake.scheduled", name, "", diag.OK, "updated")
		if err := state.WriteFile(filepath.Join(queueDir(), name), []byte(msg), 0600); err != nil {
			queueFailure(now, err)
		} else {
			wakeStage("act.wake.merged", name, "", diag.OK, "pending-key")
			retireObsoleteRetries(field, now)
		}
		return
	}
	name := fmt.Sprintf("%019d-%d-%s.p%d%s.txt", dueNano, os.Getpid(), diag.NewOpID(), hqwake.PriorityOf(msg), field)
	wakeStage("act.wake.scheduled", name, "", diag.OK, "keyed")
	if admitQueued(name, msg, now) {
		retireObsoleteRetries(field, now)
	}
}

// auditDroppedFile journals a queue entry that is about to be removed undelivered.
// Best-effort like the queue itself: an unreadable file still journals the drop, with
// the entry name standing in for the payload it could not read.
func auditDroppedFile(reason, path string) {
	line := ""
	if b, err := os.ReadFile(path); err == nil {
		if wake, err := readWakeLine(path, b); err == nil {
			line = strings.TrimSpace(wake.Line)
		}
	}
	if line == "" {
		line = "(unreadable: " + filepath.Base(path) + ")"
	}
	events.AuditWakeDropped(reason, line, time.Now().Unix())
}

// queuedNames returns the `.txt` entry names passing keep, ordered for delivery:
// highest priority first, oldest first within a priority.
func queuedNames(keep func(name string) bool) []string {
	var names []string
	for _, e := range readQueue() {
		if isPendingWake(e.Name()) && attemptsOf(e.Name()) == 0 && keep(e.Name()) {
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
	field, _, _ := strings.Cut(name[i+2:], ".")
	n, err := strconv.Atoi(field)
	if err != nil {
		return hqwake.PriorityDefault
	}
	return n
}

// attemptsOf parses the `.a<n>` field: how many times this entry has been pasted
// without its delivery being confirmed. A key can never be confused for it —
// sanitizeKey strips dots.
func attemptsOf(name string) int {
	base := wakeFileStem(name)
	i := strings.LastIndex(base, ".a")
	if i < 0 {
		return 0
	}
	n, err := strconv.Atoi(base[i+2:])
	if err != nil {
		return 0
	}
	return n
}

// identityOf strips the attempt counter (and the extension), leaving the entry's
// STABLE identity. batchID hashes these, so a re-send of the same entries produces
// the same id no matter how many attempts it took.
func identityOf(name string) string {
	base := wakeFileStem(name)
	if retry, valid := parseRetry(name); valid && retry.Number > 0 {
		return base[:strings.LastIndex(base, ".r")]
	}
	i := strings.LastIndex(base, ".a")
	if i < 0 {
		return base
	}
	if _, err := strconv.Atoi(base[i+2:]); err != nil {
		return base
	}
	return base[:i]
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
	pending, err := hqinput.RotationPending(pane)
	if err != nil {
		recordIssue(issueRecoveryUnsafe, x.nowNano())
		return
	}
	if pending {
		return // an unsent maintenance request reserves a safe input window
	}
	// Recover possible native submissions before ordinary orphan handling, even
	// if the current pane now selects a legacy route. A restart is not a retry.
	protected, err := recoverNativeIntentsWithProtection()
	if err != nil {
		diag.ReportStoreFailure("hq-native-wake", err)
		if protected == nil {
			recordIssue(issueRecoveryUnsafe, x.nowNano())
			return
		}
		recordIssue(issueClaimConflict, x.nowNano())
	}
	var submit func(string, string) (dispatch.Result, bool)
	if x.nativePrepare != nil {
		submit = x.nativePrepare(pane)
	} else if x.nativeSubmit != nil {
		submit = func(payload, id string) (dispatch.Result, bool) { return x.nativeSubmit(pane, payload, id) }
	}
	if _, err := os.Lstat(repairPath()); !errors.Is(err, os.ErrNotExist) {
		if _, valid := readRepair(); !valid {
			recordIssue(issueRecoveryUnsafe, x.nowNano())
			return
		}
	}
	if !holdOrphanClaims(x, protected) {
		return
	}
	if len(protected) == 0 {
		noteCleanRecovery(x.nowNano())
	}
	if !settleEarlierAttempts(x, pane, protected) {
		return
	}
	if wakeBusy(x, pane) {
		return
	}
	if submit == nil && !validWakeTarget(wakeIdentity(x, pane)) {
		recordIssue(issueTargetUnavailable, x.nowNano())
		return
	}
	// Never rename a queued replacement onto a held foreign/changed claim.
	for _, n := range queuedNames(func(string) bool { return true }) {
		if _, valid := parseRetry(n); !valid {
			recordIssue(issueRecoveryUnsafe, x.nowNano())
			return
		}
	}
	now := x.nowNano()
	due := queuedNames(func(n string) bool { return !protected[n+sendingSuffix] && dueForAttempt(n) <= now })
	regular, reserved := splitRelayAttention(due)
	reservedTarget := ""
	if reserved != "" && submit != nil && emptyRecognizedWakeBox(x, pane) {
		reservedTarget = wakeIdentity(x, pane)
	}
	if !validWakeTarget(reservedTarget) {
		reserved = ""
	}
	// The protected relay gets this window. Ordinary entries can share it
	// when their retry group matches, or keep their original pending slot.
	first := reserved
	if first == "" && len(regular) > 0 {
		first = regular[0]
	}
	if first == "" {
		return
	}
	retry, _ := parseRetry(first)
	group := make([]string, 0, len(regular))
	for _, n := range regular {
		candidate, _ := parseRetry(n)
		if sameRetryGroup(retry, candidate) {
			group = append(group, n)
		}
	}
	claimed, msgs, held := claimBatch(group)
	hasReserved := false
	if reserved != "" {
		if reserveState, _ := parseRetry(reserved); sameRetryGroup(retry, reserveState) {
			extra, line, _ := claimBatch([]string{reserved})
			hasReserved = len(extra) > 0
			claimed, msgs = append(claimed, extra...), append(msgs, line...)
		}
	}
	if len(msgs) == 0 {
		return
	}

	payload := strings.Join(msgs, " · ")
	if held > 0 {
		payload += fmt.Sprintf(" · +%d more queued", held)
	}
	id := "#" + batchID(claimed, payload)
	payload += " · " + id
	for _, path := range claimed {
		wakeStage("act.wake.claimed", claimBase(path), id, diag.OK, "batch")
	}

	if submit != nil || retry.Number > 0 {
		intent, err := prepareNativeIntent(pane, id, payload, claimed)
		if err != nil {
			diag.ReportStoreFailure("hq-native-wake", err)
			recordIssue(issueRecoveryUnsafe, x.nowNano())
			return
		}
		target := wakeIdentity(x, pane)
		intent.SubmittedAt = x.nowNano()
		var result dispatch.Result
		managed := true
		switch {
		case hasReserved && target != reservedTarget, retry.Number > 0 && (submit == nil || target != retry.Target):
			result = dispatch.Result{State: dispatch.StateRefusedWaiting, Submission: dispatch.SubmissionNotSubmitted}
		case hasReserved && (!emptyRecognizedWakeBox(x, pane) || wakeBusy(x, pane)):
			result = dispatch.Result{State: dispatch.StateRefusedDraft, Submission: dispatch.SubmissionNotSubmitted, Evidence: "wake-loaded-idle-required"}
		default:
			result, managed = submit(payload, nativeAttemptID(id, retry))
			if !managed && (retry.Number > 0 || hasReserved) {
				managed = true
				result = dispatch.Result{State: dispatch.StateRefusedWaiting, Submission: dispatch.SubmissionNotSubmitted}
			}
		}
		if managed {
			setNativeWakeOutcome(&intent, result, retry, target, x.nowNano())
			if err := writeNativeIntent(intent); err != nil {
				diag.ReportStoreFailure("hq-native-wake", err)
				recordIssue(issueRecoveryUnsafe, x.nowNano())
				return
			}
			recordWakeOutcome(intent, x.nowNano())
			if err := settleNativeIntent(intent); err != nil {
				diag.ReportStoreFailure("hq-native-wake", err)
				recordIssue(issueRecoveryUnsafe, x.nowNano())
				return
			}
			switch intent.Outcome {
			case "accepted":
				finishAccepted(pane, payload, nil)
			case "delivered":
				finishDelivered(pane, payload, nil)
			case "uncertain", "held":
				writeFailCount(readFailCount() + 1)
			}
			return
		}

		// Only an isolated legacy seam can decline after preparation; no native
		// attempt has occurred. Production selects the route before journaling.
		intent.Outcome = "refused"
		if err := writeNativeIntent(intent); err != nil {
			diag.ReportStoreFailure("hq-native-wake", err)
			return
		}
		if err := os.Remove(nativeIntentPath(intent)); err != nil {
			diag.ReportStoreFailure("hq-native-wake", err)
			return
		}
	}

	attemptLegacyWake(x, pane, payload, id, claimed)
}

// finishDelivered closes a confirmed batch. One audit record per BATCH (never per entry):
// the journal is where "what did gtmux tell HQ at 14:02" becomes answerable — the hook's
// own receipt records only the six-hex id.
func finishDelivered(pane, payload string, claimed []string) {
	noteConfirmedDelivery(time.Now().Unix())
	events.AuditWakeDelivered(pane, payload, time.Now().Unix())
	for _, c := range claimed {
		_ = os.Remove(c)
	}
	writeFailCount(0)
}

// Native acceptance is durable transport evidence, not a conversation receipt
// or consumption of any source event. The exact native spool receipt remains.
func finishAccepted(pane, payload string, claimed []string) {
	events.AuditWakeAccepted(pane, payload, time.Now().Unix())
	for _, c := range claimed {
		_ = os.Remove(c)
	}
	writeFailCount(0)
}

// claimBatch claims up to one delivery's worth of due entries (highest priority
// first, oldest first within a priority) and returns the claims, their payloads, and
// how many due entries were held back for the next drain.
func claimBatch(due []string) (claimed, msgs []string, held int) {
	chars := 0
	roster := sync.OnceValue(readWaitingInventory)
	var dropped []string
	defer func() {
		// A line whose premise died is finished business: remove the claim so no later
		// drain retries it. Done after the loop so the removal can't disturb iteration.
		for _, d := range dropped {
			_ = os.Remove(d)
		}
	}()
	for i, n := range due {
		if len(msgs) >= maxBatchLines || (len(msgs) > 0 && chars > maxBatchChars) {
			return claimed, msgs, len(due) - i
		}
		src := filepath.Join(queueDir(), n)
		dst := src + sendingSuffix
		if !takeQueued(src, dst) {
			continue // no longer pending, or its claim location is protected
		}
		b, err := readNativeBytes(dst)
		w, decodeErr := readWakeLine(dst, b)
		if err = errors.Join(err, decodeErr); err != nil || strings.TrimSpace(w.Line) == "" {
			if err == nil {
				err = errors.New("empty wake claim")
			}
			diag.ReportStoreFailure("hq-wake-queue", err)
			recordIssue(issueRecoveryUnsafe, time.Now().UnixNano())
			for _, c := range claimed {
				// The receiver has not been called. Restore other claims, leaving
				// the unreadable original held for diagnosis.
				diag.ReportStoreFailure("hq-wake-queue", returnClaim(c))
			}
			return nil, nil, 0
		}
		s := strings.TrimSpace(w.Line)
		if retry, _ := parseRetry(n); retry.Number == 0 && strings.HasSuffix(n, waitingWakeSuffix) {
			if reason := revalidateWaiting(w, roster); reason != "" {
				err := events.AuditWaitingWakeDropped(reason, s, w.Source.Origin.Pane, w.Source.Origin.Loc,
					w.Source.Task, w.Source.Owner.Session, w.Source.Owner.Home, time.Now().Unix())
				if err == nil {
					wakeStage("act.wake.dropped", n, "", diag.Refused, reason)
					dropped = append(dropped, dst)
					continue
				}
				// Preserve the notice when its suppression cannot be journaled.
				diag.ReportStoreFailure("hq-wake-queue", err)
			}
		}
		// Delivery-side revalidation (standing-wake-backoff §2.1). A queued line was
		// decided when it was enqueued, which is NOT when it is typed: it can sit behind
		// a half-written draft, behind an earlier batch, or behind the fast tick. The
		// world moves in that gap — a resource tier recovers, an auto-compact zeroes the
		// ctx a warning is about — and the line then arrives asserting a state that no
		// longer exists. Measured: a `ctx→80% in ~4m` warning delivered while the pane's
		// own status bar read 34%, because the harness had compacted mid-flight.
		//
		// The probe runs HERE, at the last possible moment, and its claim is already
		// taken — so a dropped line is removed like a delivered one and can never come
		// back. Classes with no probe registered are untouched.
		keep, replaced := revalidate(s)
		if !keep {
			// The control trace standing-wake-backoff §2.1 promised: a line whose
			// premise died is finished business, and the journal says which line.
			events.AuditWakeDropped(events.DropSuperseded, s, time.Now().Unix())
			dropped = append(dropped, dst)
			continue
		}
		claimed = append(claimed, dst)
		msgs = append(msgs, replaced)
		chars += len(replaced)
	}
	return claimed, msgs, 0
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

// ── delivery-side revalidation (standing-wake-backoff §2.1) ──────────────────

// Revalidator re-checks a queued wake line at the moment it is about to be typed.
// It returns whether to deliver at all, and the line to deliver — so a probe can
// DROP a wake whose premise died in the queue, or RE-RENDER one whose figures moved.
//
// It takes the rendered line because that is all the queue holds: entries are plain
// text files on disk, and a closure cannot survive a serve restart. A probe therefore
// recognizes its own class from the line and re-samples the world itself.
type Revalidator func(line string) (keep bool, out string)

// revalidators are consulted in order; the FIRST one that recognizes the line wins.
// Registered by the subsystem that owns each standing class (internal/hq), which sits
// above this package — injection, not an import, so the layering stays acyclic.
var revalidators []Revalidator

// RegisterRevalidator adds a probe. Not concurrency-guarded because registration
// happens once, at wiring time, before any tick runs.
func RegisterRevalidator(f Revalidator) {
	if f != nil {
		revalidators = append(revalidators, f)
	}
}

// revalidate runs the registered probes. With none registered — or none recognizing
// this line — the line is delivered verbatim, which is the pre-change behavior and the
// right default: a class nobody claimed must not be silently droppable.
func revalidate(line string) (bool, string) {
	for _, f := range revalidators {
		if keep, out := f(line); !keep || out != line {
			if !keep {
				return false, ""
			}
			return true, out
		}
	}
	return true, line
}
