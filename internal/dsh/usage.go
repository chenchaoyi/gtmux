package dsh

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"
)

// Usage and UsageRecords copy accounting data only for accounting consumers.
// Ordinary radar discovery must not allocate every session's daily maps.
func (x *Catalog) Usage(id string) (TokenUsage, error) {
	s, err := x.Resolve(id)
	if err != nil {
		return TokenUsage{}, err
	}
	if s.Origin == "subagent" {
		return TokenUsage{}, ErrUnsupported
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	if e := x.entries[s.Path]; e != nil {
		return e.usage.Clone(), nil
	}
	return TokenUsage{}, fmt.Errorf("DSH usage unavailable")
}

func (x *Catalog) UsageRecords() (map[string]TokenUsage, error) {
	rows, err := x.List()
	if err != nil {
		return nil, err
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	out := map[string]TokenUsage{}
	for _, s := range rows {
		if e := x.entries[s.Path]; e != nil && e.usage.Seen {
			out[s.Path] = e.usage.Clone()
		}
	}
	return out, nil
}

// TokenBuckets match the disjoint buckets in DSH's native tokenUsage projection.
// Reasoning output is already part of Output and must not be added again.
type TokenBuckets struct {
	Input      int64 `json:"input"`
	Output     int64 `json:"output"`
	CacheRead  int64 `json:"cache_read"`
	CacheWrite int64 `json:"cache_write"`
}

type usageSlot struct {
	Turn, Step int64
	At         int64
	Buckets    TokenBuckets
}

type usagePoint struct{ At, Output int64 }

// TokenUsage contains observations only, never a subscription or cost estimate.
type TokenUsage struct {
	Seen     bool
	Total    TokenBuckets
	Days     map[string]TokenBuckets
	LastAt   int64
	Pressure int64
	Window   int64
	last     *usageSlot
	recent   []usagePoint
}

func (u TokenUsage) Clone() TokenUsage {
	out := u
	out.Days = make(map[string]TokenBuckets, len(u.Days))
	for k, v := range u.Days {
		out.Days[k] = v
	}
	out.last, out.recent = nil, append([]usagePoint(nil), u.recent...)
	return out
}

func (u TokenUsage) Rate(now time.Time) int64 {
	cut := now.Add(-10 * time.Minute).UnixMilli()
	var total, first int64
	for _, p := range u.recent {
		if p.At < cut || p.At > now.UnixMilli() {
			continue
		}
		total += p.Output
		if first == 0 || p.At < first {
			first = p.At
		}
	}
	if first == 0 || total <= 0 {
		return 0
	}
	minutes := now.Sub(time.UnixMilli(first)).Minutes()
	return int64(float64(total) / max(1, minutes))
}

func addBuckets(a, b TokenBuckets) TokenBuckets {
	return TokenBuckets{a.Input + b.Input, a.Output + b.Output, a.CacheRead + b.CacheRead, a.CacheWrite + b.CacheWrite}
}
func subtractBuckets(a, b TokenBuckets) TokenBuckets {
	return TokenBuckets{a.Input - b.Input, a.Output - b.Output, a.CacheRead - b.CacheRead, a.CacheWrite - b.CacheWrite}
}

// Fold follows the official v2 tokenUsage projection's one replacement slot.
// Native durable records are consumed before transcript surface filtering.
func (u *TokenUsage) Fold(e Event) {
	var d struct {
		Turn   *int64          `json:"turn"`
		Step   *int64          `json:"step"`
		Window *int64          `json:"contextWindow"`
		Usage  json.RawMessage `json:"usage"`
		Stream []struct {
			Type  string `json:"type"`
			Chunk struct {
				Type  string          `json:"type"`
				Usage json.RawMessage `json:"usage"`
			} `json:"chunk"`
		} `json:"stream"`
	}
	switch e.Type {
	case "request/context", "llm/retry-started", "assistant/message", "assistant/attempt":
	default:
		return
	}
	if json.Unmarshal(e.Data, &d) != nil {
		return
	}
	if e.Type == "request/context" {
		u.Window = 0
		if d.Window != nil && *d.Window > 0 {
			u.Window = *d.Window
		}
		return
	}
	if d.Turn == nil || d.Step == nil || *d.Turn < 0 || *d.Step < 0 {
		return
	}
	if e.Type == "llm/retry-started" {
		if u.last != nil && u.last.Turn == *d.Turn && u.last.Step == *d.Step {
			u.last = nil
		}
		return
	}
	raw := d.Usage
	if len(raw) == 0 || string(raw) == "null" {
		for i := len(d.Stream) - 1; i >= 0; i-- {
			r := d.Stream[i]
			if r.Type == "chunk" && r.Chunk.Type == "usage" {
				raw = r.Chunk.Usage
				break
			}
		}
	}
	var b struct {
		Input  *int64 `json:"inputTokens"`
		Output *int64 `json:"outputTokens"`
		Read   int64  `json:"cacheReadTokens"`
		Write  int64  `json:"cacheWriteTokens"`
	}
	if json.Unmarshal(raw, &b) != nil || b.Input == nil || b.Output == nil || *b.Input < 0 || *b.Output < 0 || b.Read < 0 || b.Write < 0 {
		return
	}
	// Numbers outside the source's safe integer range cannot be authoritative counts.
	for _, n := range []int64{*b.Input, *b.Output, b.Read, b.Write} {
		if n > 1<<53-1 {
			return
		}
	}
	next := TokenBuckets{*b.Input, *b.Output, b.Read, b.Write}
	at := e.Time
	previous := TokenBuckets{}
	if u.last != nil && u.last.Turn == *d.Turn && u.last.Step == *d.Step {
		previous = u.last.Buckets
		if previous == next {
			return
		}
		at = u.last.At
	}
	delta := subtractBuckets(next, previous)
	total := addBuckets(u.Total, delta)
	for _, n := range []int64{total.Input, total.Output, total.CacheRead, total.CacheWrite} {
		if n < 0 || n > math.MaxInt64/4 {
			return
		}
	}
	u.Total = total
	u.Seen, u.LastAt, u.Pressure = true, e.Time, next.Input+next.CacheRead+next.CacheWrite
	u.last = &usageSlot{*d.Turn, *d.Step, at, next}
	if at <= 0 {
		return
	} // a missing clock cannot be assigned to a day or rate window
	if u.Days == nil {
		u.Days = map[string]TokenBuckets{}
	}
	day := time.UnixMilli(at).Local().Format("2006-01-02")
	u.Days[day] = addBuckets(u.Days[day], delta)
	if len(u.Days) > 370 {
		keys := make([]string, 0, len(u.Days))
		for key := range u.Days {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		delete(u.Days, keys[0])
	}
	if len(u.recent) > 0 && u.recent[len(u.recent)-1].At == at {
		u.recent[len(u.recent)-1].Output += delta.Output
	} else {
		u.recent = append(u.recent, usagePoint{at, delta.Output})
	}
	cut := e.Time - int64(10*time.Minute/time.Millisecond)
	start := 0
	for start < len(u.recent) && u.recent[start].At < cut {
		start++
	}
	if len(u.recent)-start > 1024 {
		start = len(u.recent) - 1024
	}
	if start > 0 {
		u.recent = append([]usagePoint(nil), u.recent[start:]...)
	}
}
