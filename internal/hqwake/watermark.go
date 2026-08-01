package hqwake

// HQ's CONSUMPTION WATERMARK — the position in the event stream HQ has actually read.
//
// It exists because the wake vocabulary cannot be a completeness mechanism. Deciding
// "is this event worth waking HQ for?" needs context only HQ has (it is the one that
// knows it is waiting on %16's install), so gtmux could only ever approximate the
// judgment with an enumerated whitelist of classes — and every scenario the list did not
// anticipate vanished silently. A turn-end that was neither a tracked dispatch nor a
// question was exactly such a hole: the record was in the stream, complete and correct,
// and nothing knocked.
//
// The watermark inverts the question. gtmux stops asking "does this event deserve a
// wake?" and asks "is there anything HQ has not consumed?" — a question it can answer
// with no context at all, from two integers. Classes keep their job of saying what to
// look at FIRST; the watermark is what makes sure everything is looked at eventually.
//
// The invariant: the watermark only ever moves because HQ CONSUMED something (an
// unfiltered delta read, or an explicit `gtmux events --ack`). Nothing gtmux does on its
// own — knocking, ticking, sensing — advances it. That is the whole guarantee: an
// unadvanced watermark is a debt gtmux keeps collecting on.

import (
	"os"
	"path/filepath"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// WatermarkUnset marks "HQ has never recorded a position" — distinct from 0, which is a
// real watermark ("consumed nothing on a stream that has produced nothing"). The
// distinction matters exactly once, at bootstrap: an install upgrading into this
// mechanism has thousands of historical events and no cursor, and must adopt the current
// stream end rather than be told it is 6 000 events behind.
const WatermarkUnset = int64(-1)

// watermarkPath stores the consumed-through sequence (decimal text).
func watermarkPath() string { return filepath.Join(dir(), "consumed-seq") }

// Consumed returns HQ's watermark, or WatermarkUnset when none is recorded.
func Consumed() int64 {
	if state.ReadMarker(watermarkPath()) == "" {
		return WatermarkUnset
	}
	return state.ReadInt64Marker(watermarkPath())
}

// Consume advances the watermark to seq. Monotonic: a lower value is ignored, so an
// out-of-order or stale writer can never REWIND HQ's position and re-raise events it has
// already judged. Returns whether it moved.
func Consume(seq int64) bool {
	if seq < 0 {
		return false
	}
	if cur := Consumed(); cur >= seq {
		return false
	}
	if err := os.MkdirAll(dir(), 0o755); err != nil {
		return false
	}
	return state.WriteInt64Marker(watermarkPath(), seq) == nil
}

// ConsumeRead records an HQ delta read of (from, to] and reports whether the watermark
// moved. The read counts as consumption only when it is CONTIGUOUS with what HQ has
// already consumed (from <= watermark): a read that starts AHEAD of the watermark is a
// skip-ahead peek at the tail, and honoring it would let HQ silently drop the range it
// jumped over — the exact failure this mechanism exists to prevent. The peek is allowed;
// it just leaves the debt standing, so the next knock still names it.
//
// An unset watermark accepts any read (bootstrap: HQ's first pull defines its position).
func ConsumeRead(from, to int64) bool {
	if wm := Consumed(); wm != WatermarkUnset && from > wm {
		return false
	}
	return Consume(to)
}
