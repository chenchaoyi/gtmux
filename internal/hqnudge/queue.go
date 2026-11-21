package hqnudge

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/state"
	"golang.org/x/sys/unix"
)

// The queue lock protects short on-disk operations only. The HQ input lock can
// span a driver call, so producers must stay able to enqueue while it is held.
func lockQueue() (func(), error) {
	if err := state.MkdirAll(queueDir(), 0700); err != nil {
		return nil, err
	}
	lockDir := filepath.Join(state.Dir(), "hqwake")
	if err := state.MkdirAll(lockDir, 0700); err != nil {
		return nil, err
	}
	file, err := state.OpenFile(filepath.Join(lockDir, "wake-queue.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX); err != nil {
		_ = file.Close()
		return nil, err
	}
	return func() { _ = file.Close() }, nil
}

func pendingKey(field string) string {
	for _, entry := range readQueue() {
		if retry, valid := parseRetry(entry.Name()); valid && retry.Number == 0 && attemptsOf(entry.Name()) == 0 && entry.Type().IsRegular() && queuedKey(entry.Name()) == field {
			return entry.Name()
		}
	}
	return ""
}

func queuedKey(path string) string {
	name := filepath.Base(path)
	if !strings.HasSuffix(name, ".txt") {
		return ""
	}
	at := strings.Index(name, ".k-")
	if at < 0 {
		return ""
	}
	field, _, _ := strings.Cut(name[at+1:], ".")
	return "." + field
}

// A later producer supersedes only positively refused generations. Held and
// uncertain claims are never candidates for coalescing or deletion.
func retireObsoleteRetries(field string, now int64) {
	for _, name := range queuedNames(func(name string) bool { return queuedKey(name) == field }) {
		retry, valid := parseRetry(name)
		if !valid || retry.Number == 0 {
			continue
		}
		path := filepath.Join(queueDir(), name)
		body, err := readNativeBytes(path)
		if err == nil {
			err = os.Remove(path)
		}
		if err != nil {
			queueFailure(now, err)
			continue
		}
		events.AuditWakeDropped(events.DropSuperseded, string(body), time.Now().Unix())
		wakeStage("rejected", path, "", diag.Refused, events.DropSuperseded)
	}
}

func takeQueued(src, dst string) bool {
	release, err := lockQueue()
	if err != nil {
		diag.ReportStoreFailure("hq-wake-queue", err)
		return false
	}
	defer release()
	if _, err := os.Lstat(dst); !errors.Is(err, os.ErrNotExist) {
		return false
	}
	return os.Rename(src, dst) == nil
}

// admitQueued runs under the queue lease. Rank the newcomer before changing
// existing entries and publish it completely before removing a verified victim.
func admitQueued(name, message string, now int64) bool {
	names := queuedNames(func(string) bool { return true })
	path := filepath.Join(queueDir(), name)
	if len(names) >= maxQueueEntries {
		recordIssue(issueQueuePressure, now)
		if prioOf(name) > prioOf(names[len(names)-1]) {
			events.AuditWakeDropped(events.DropQueueFull, message, time.Now().Unix())
			wakeStage("rejected", name, "", diag.Refused, events.DropQueueFull)
			return false
		}
	}
	for len(names) > maxQueueEntries { // an interrupted admission can leave one extra file
		if err := evictQueued(&names); err != nil {
			queueFailure(now, err)
			return false
		}
	}
	if err := state.WritePrivateFile(path, []byte(message)); err != nil {
		queueFailure(now, err)
		return false
	}
	if len(names) >= maxQueueEntries {
		if err := evictQueued(&names); err != nil {
			queueFailure(now, errors.Join(err, os.Remove(path)))
			wakeStage("rejected", name, "", diag.Failed, "eviction-error")
			return false
		}
	}
	wakeStage("enqueued", name, "", diag.OK, "")
	return true
}

func evictQueued(names *[]string) error {
	queued := *names
	i := len(queued) - 1
	for worst := prioOf(queued[i]); i > 0 && prioOf(queued[i-1]) == worst; i-- {
	}
	victim := filepath.Join(queueDir(), queued[i])
	body, err := readNativeBytes(victim)
	if err != nil {
		return err
	}
	if err := os.Remove(victim); err != nil {
		return err
	}
	*names = append(queued[:i], queued[i+1:]...)
	events.AuditWakeDropped(events.DropEvicted, string(body), time.Now().Unix())
	wakeStage("evicted", victim, "", diag.Refused, events.DropEvicted)
	return nil
}

func queueFailure(now int64, err error) {
	diag.ReportStoreFailure("hq-wake-queue", err)
	recordIssue(issueQueueIO, now)
}

func wakeStage(stage, entry, batch string, outcome string, reason string) {
	raw := sha256.Sum256([]byte(identityOf(filepath.Base(entry))))
	diag.For("hq").Act("act.wake."+stage, "system", "hq", outcome, "HQ wake stage",
		"entry", hex.EncodeToString(raw[:12]), "batch", batch, "reason", reason)
}
