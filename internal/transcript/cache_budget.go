package transcript

import "time"

const (
	cacheEntryCap = 16
	cacheByteCap  = 32 << 20
	cacheIdleTTL  = 24 * time.Hour
)

// Cache metadata is protected by Loader.mu, like the parsed snapshots themselves.
// Eviction discards only a reusable parse: the source history remains authoritative.
func (l *Loader) cacheTime() time.Time {
	if l.cacheNow != nil {
		return l.cacheNow()
	}
	return time.Now()
}

func (l *Loader) cached(path string) *cacheEntry {
	now := l.cacheTime()
	for key, e := range l.m {
		if now.Sub(e.lastUse) >= cacheIdleTTL {
			delete(l.m, key)
		}
	}
	e := l.m[path]
	if e != nil {
		l.cacheClock++
		e.lastUse, e.used = now, l.cacheClock
	}
	return e
}

func (l *Loader) retain(e *cacheEntry) {
	if l.m == nil {
		l.m = make(map[string]*cacheEntry)
	}
	e.retainedBytes = cachedTurnBytes(e.turns) + int64(2*len(e.path)+256)
	for _, record := range e.pendingContexts {
		e.retainedBytes += int64(128 + len(record.Text) + len(record.Source) + len(record.Title) + len(record.Time))
	}
	if e.retainedBytes > cacheByteCap {
		// Returning a large requested history is valid; retaining it is optional.
		// Never truncate public content merely to make a cache entry fit.
		delete(l.m, e.path)
		return
	}
	l.cacheClock++
	e.lastUse, e.used = l.cacheTime(), l.cacheClock
	l.m[e.path] = e
	for {
		var total int64
		var oldest *cacheEntry
		for _, entry := range l.m {
			total += entry.retainedBytes
			if oldest == nil || entry.used < oldest.used {
				oldest = entry
			}
		}
		if len(l.m) <= cacheEntryCap && total <= cacheByteCap {
			return
		}
		delete(l.m, oldest.path)
	}
}

// Count duplicate string views conservatively and allow for slice/struct overhead.
// This is a retention budget, not a promise about Go's allocator or process RSS.
func cachedTurnBytes(turns []Turn) int64 {
	var n int64
	for _, t := range turns {
		n += int64(512 + len(t.Prompt) + len(t.Response) + len(t.Agent) + len(t.Time))
		if t.From != nil {
			n += int64(128 + len(t.From.Kind) + len(t.From.Label) + len(t.From.Agent) + len(t.From.Pane))
		}
		if t.Break != nil {
			n += int64(64 + len(t.Break.Kind))
		}
		for _, s := range t.Segments {
			n += int64(128 + len(s.Text))
			if s.Context != nil {
				n += int64(128 + len(s.Context.Text) + len(s.Context.Title) + len(s.Context.Source) + len(s.Context.Time))
			}
			for _, step := range s.Steps {
				n += int64(160 + len(step.Kind) + len(step.Title) + len(step.Detail) + len(step.Operation) + len(step.Input) + len(step.InputKind) + len(step.Workdir))
			}
		}
	}
	return n
}
