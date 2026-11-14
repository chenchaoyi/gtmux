package resume

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// A layout address is mutable; the foreground process incarnation is not. This
// comparison is used only by the authenticated producer preparing a publication.
// Readers continue to require the exact published target and five-second proof.
func sameFrontendIncarnation(a, b CodexBindingTarget) bool {
	a.Loc = b.Loc
	return a == b
}

func producerFrontendLocation(f CodexFrontend) (CodexFrontend, error) {
	target, ok := frontendTarget(f.Pane)
	if !ok {
		return f, &FrontendBindingError{Code: "terminal-proof-unavailable", Retryable: true}
	}
	if target.Loc == "" || !sameFrontendIncarnation(target, f.CodexBindingTarget) {
		return f, &FrontendBindingError{Code: "terminal-changed"}
	}
	f.Loc = target.Loc
	return f, producerFrontendProof(f)
}

// A second layout change during verification is missing a stable publication
// address, not permission to follow it without checking the other fields again.
func producerFrontendProof(f CodexFrontend) error {
	err := frontendProof(f, true)
	var proof *FrontendBindingError
	if errors.As(err, &proof) && proof.Code == "terminal-changed" {
		if current, ok := frontendTarget(f.Pane); ok && current.Loc != f.Loc && sameFrontendIncarnation(current, f.CodexBindingTarget) {
			return &FrontendBindingError{Code: "terminal-location-changed", Retryable: true}
		}
	}
	return err
}

// Called with the shared publication lease held, before either write. It does
// not repair a reader's history or let an absent/stale proof select a thread.
func validateFrontendRelocation(previous, next CodexFrontend) error {
	fail := func(code string, retry bool) error { return &FrontendBindingError{Code: code, Retryable: retry} }
	saved, ok := readFrontend(previous.Instance)
	if previous.SessionID == "" && !ok {
		// An absent nonce permits its first publication. A present valid receipt
		// instead takes the exact owner/pending-root checks below: the first
		// attempt may have written its own proof before an index IO failure.
		if _, err := os.Lstat(frontendPath(previous.Instance)); !errors.Is(err, os.ErrNotExist) {
			return fail("frontend-owner-changed", false)
		}
	} else {
		if !ok {
			return fail("frontend-record-unavailable", true)
		}
		owner := saved
		owner.UpdatedAt, owner.Loc = previous.UpdatedAt, previous.Loc
		pending := next
		pending.UpdatedAt, pending.Loc = previous.UpdatedAt, previous.Loc
		// A prior failed index write may already have committed this exact observed
		// root receipt. The connection retries that root, never a different selection.
		if owner != previous && owner != pending || saved.UpdatedAt < previous.UpdatedAt {
			return fail("frontend-owner-changed", false)
		}
		// Rebase only this producer's verification snapshot. No persisted proof is
		// changed until identity, owner, cancellation and publication checks pass.
		checked := saved
		checked.Loc = next.Loc
		if err := producerFrontendProof(checked); err != nil {
			return err
		}
	}
	if r, exists := Load(previous.Loc); exists && r.Frontend == previous.Instance &&
		(r.Agent != "codex" || r.HomeID != previous.HomeID || r.SessionID != previous.SessionID) {
		return fail("frontend-owner-changed", false)
	}
	b, err := os.ReadFile(fileFor(next.Loc))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	var destination Record
	if err != nil || json.Unmarshal(b, &destination) != nil {
		return fail("frontend-destination-unavailable", true)
	}
	if destination.Frontend == previous.Instance {
		if destination.Agent != "codex" || destination.HomeID != previous.HomeID || destination.SessionID != previous.SessionID && (!ok || saved.SessionID != next.SessionID || destination.SessionID != saved.SessionID) {
			return fail("frontend-owner-changed", false)
		}
		return nil
	}
	if destination.Frontend == "" {
		return nil // historical layout metadata; the current producer is proven above
	}
	other, exists := readFrontend(destination.Frontend)
	if !exists {
		if path := frontendPath(destination.Frontend); path != "" {
			if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
				return nil // exact missing old proof, not a malformed/unreadable one
			}
		}
		return fail("frontend-destination-unavailable", true)
	}
	if other.Pane == next.Pane && (other.StartedAt > next.StartedAt ||
		other.BridgePID == next.BridgePID && other.BridgeStart == next.BridgeStart && other.Sequence > next.Sequence) {
		return fail("frontend-owner-changed", false)
	}
	if age := time.Now().UnixMilli() - other.UpdatedAt; age > 5000 {
		return nil // an older expired destination is not a live owner
	}
	if err := frontendProof(other, true); err == nil {
		if other.Loc != next.Loc {
			return nil
		} // its verified current address is elsewhere
		return fail("frontend-destination-owned", false)
	} else {
		var proof *FrontendBindingError
		if !errors.As(err, &proof) || proof.Retryable {
			return fail("frontend-destination-unavailable", true)
		}
	}
	return nil // the independently observed old incarnation no longer owns here
}

// Displaced records are history only: no reader or input resolver consults this
// directory. The publisher preserves the destination before replacing its index;
// a displaced producer may recover only its own exact historical metadata after
// proving its unchanged live incarnation under the publication lease.
func frontendLocationHistoryPath(loc, instance string) string {
	if frontendPath(instance) == "" {
		return ""
	}
	return filepath.Join(Dir(), "codex-location-history", fmt.Sprintf("%s-%x.json", instance, sha256.Sum256([]byte(loc))))
}

func preserveFrontendLocationHistory(previous, next CodexFrontend) error {
	r, ok := Load(next.Loc)
	if !ok || r.Frontend == "" || r.Frontend == previous.Instance {
		return nil
	}
	path := frontendLocationHistoryPath(next.Loc, r.Frontend)
	if path == "" {
		return &FrontendBindingError{Code: "frontend-destination-unavailable", Retryable: true}
	}
	if err := state.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return frontendBindingWrite(path, r)
}

func frontendLocationHistory(previous CodexFrontend) Record {
	matches := func(r Record) bool {
		return r.Agent == "codex" && r.Frontend == previous.Instance && r.HomeID == previous.HomeID && r.SessionID == previous.SessionID
	}
	if r, ok := Load(previous.Loc); ok && matches(r) {
		return r
	}
	path := frontendLocationHistoryPath(previous.Loc, previous.Instance)
	if path == "" {
		return Record{}
	}
	var r Record
	if b, err := os.ReadFile(path); err == nil && len(b) <= 8192 && json.Unmarshal(b, &r) == nil && matches(r) {
		return r
	}
	return Record{}
}

// Only moved-root metadata publication retries within the existing observed-root
// budget. It never repeats the successful thread operation or submits input.
func frontendLocationWriteError(err error, moved bool) error {
	if !moved || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var proof *FrontendBindingError
	if errors.As(err, &proof) {
		return err
	}
	return &FrontendBindingError{Code: "frontend-publication-unavailable", Retryable: true}
}
