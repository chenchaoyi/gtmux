package hqnudge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// This slot is outside normal capacity. Old drains only admit .txt entries.
const relayAttentionSuffix = ".hqrelay"

func isRelayAttention(name string) bool {
	return strings.HasSuffix(name, relayAttentionSuffix)
}

func isPendingWake(name string) bool {
	return strings.HasSuffix(name, ".txt") || isRelayAttention(name)
}

func wakeFileStem(name string) string {
	if isRelayAttention(name) {
		return strings.TrimSuffix(name, relayAttentionSuffix)
	}
	return strings.TrimSuffix(name, ".txt")
}

// ReserveRelayAttention prepares one native attention opportunity. The caller
// records its durable source reservation before publication. If the process
// dies between the writes, the source must diagnose it instead of guessing
// whether the opportunity was already submitted.
func ReserveRelayAttention(line string, record func() (bool, error)) (bool, error) {
	release, err := lockQueue()
	if err != nil {
		return false, err
	}
	defer release()
	entries, err := inspectQueue()
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if isRelayAttention(claimBase(e.Name())) {
			return false, nil
		}
	}
	ok, err := record()
	if !ok || err != nil {
		return false, err
	}
	now := time.Now().UnixNano()
	name := fmt.Sprintf("%019d-%d-%s.p%d%s", now, os.Getpid(), diag.NewOpID(), hqwake.PriorityOf(line), relayAttentionSuffix)
	wakeStage("act.wake.scheduled", name, "", diag.OK, "relay-attention")
	if err := state.WritePrivateFile(filepath.Join(queueDir(), name), []byte(line)); err != nil {
		queueFailure(now, err)
		return false, err
	}
	wakeStage("act.wake.enqueued", name, "", diag.OK, "relay-attention")
	return true, nil
}

// Preserve the whole original batch by claiming the extra line separately.
// A positively refused retry can join only its original retry class and target.
func splitRelayAttention(due []string) (ordinary []string, reserve string) {
	for _, name := range due {
		if isRelayAttention(name) {
			if reserve == "" {
				reserve = name
			}
			continue
		}
		ordinary = append(ordinary, name)
	}
	return ordinary, reserve
}

// The native opportunity needs a verified empty composer, even when a
// legacy caller would let an unrecognized screen pass its advisory probe.
func emptyRecognizedWakeBox(x io, pane string) bool {
	if x.inMode != nil && x.inMode(pane) {
		return false
	}
	capture := x.captureColor
	if capture == nil {
		capture = x.capture
	}
	if capture == nil {
		return false
	}
	draft, known := dispatch.DraftOfColored(capture(pane))
	return known && strings.TrimSpace(draft) == ""
}
