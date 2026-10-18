package memdiag

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"
	"sync"
	"time"
)

const (
	maxProfileBytes   = 8 << 20
	maxProfileRecords = 16384
	profileInterval   = 5 * time.Second
)

// Snapshot records allocator counters, not OS RSS/footprint. HeapAlloc includes
// unreachable objects not yet collected; heap profiles lag natural GC cycles.
type Snapshot struct {
	PID                                                                     int
	Version, GoVersion                                                      string
	StartedAt, CapturedAt                                                   time.Time
	HeapAlloc, HeapInuse, HeapIdle, HeapReleased, HeapSys, Sys              uint64
	TotalAlloc, HeapObjects, Mallocs, Frees, NextGC, LastGC                 uint64
	StackInuse, StackSys, MSpanSys, MCacheSys, BuckHashSys, GCSys, OtherSys uint64
	NumGC, NumForcedGC                                                      uint32
	NumGoroutine, MemProfileRate                                            int
}

type handler struct {
	version     string
	started     time.Time
	mu          sync.Mutex
	lastProfile time.Time
	profile     func(io.Writer, string) error
	records     func() int
}

func newHandler(version string) *handler {
	return &handler{version: version, started: time.Now().UTC(),
		profile: func(w io.Writer, name string) error { return pprof.Lookup(name).WriteTo(w, 0) },
		records: func() int { n, _ := runtime.MemProfile(nil, true); return n }}
}

func (h *handler) snapshot() Snapshot {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return Snapshot{PID: os.Getpid(), Version: h.version, GoVersion: runtime.Version(),
		StartedAt: h.started, CapturedAt: time.Now().UTC(), HeapAlloc: m.HeapAlloc,
		HeapInuse: m.HeapInuse, HeapIdle: m.HeapIdle, HeapReleased: m.HeapReleased,
		HeapSys: m.HeapSys, Sys: m.Sys, TotalAlloc: m.TotalAlloc, HeapObjects: m.HeapObjects,
		Mallocs: m.Mallocs, Frees: m.Frees, NextGC: m.NextGC, LastGC: m.LastGC,
		StackInuse: m.StackInuse, StackSys: m.StackSys, MSpanSys: m.MSpanSys,
		MCacheSys: m.MCacheSys, BuckHashSys: m.BuckHashSys, GCSys: m.GCSys, OtherSys: m.OtherSys,
		NumGC: m.NumGC, NumForcedGC: m.NumForcedGC, NumGoroutine: runtime.NumGoroutine(), MemProfileRate: runtime.MemProfileRate}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet || r.URL.RawQuery != "" || r.URL.ForceQuery || r.ContentLength > 0 || len(r.TransferEncoding) > 0 {
		http.Error(w, "GET without query or body required", http.StatusBadRequest)
		return
	}
	if r.URL.Path != "/stats" && r.URL.Path != "/heap" && r.URL.Path != "/allocs" {
		http.NotFound(w, r)
		return
	}
	if !h.mu.TryLock() {
		http.Error(w, "memory diagnostic read already in progress", http.StatusTooManyRequests)
		return
	}
	defer h.mu.Unlock()
	if r.Context().Err() != nil {
		return
	}
	if r.URL.Path != "/stats" {
		if time.Since(h.lastProfile) < profileInterval {
			w.Header().Set("Retry-After", "5")
			http.Error(w, "wait between profiles", http.StatusTooManyRequests)
			return
		}
		h.lastProfile = time.Now()
		if h.records() > maxProfileRecords {
			http.Error(w, "profile exceeds sampling-record budget", http.StatusRequestEntityTooLarge)
			return
		}
	}
	m := h.snapshot()
	w.Header().Set("X-Gtmux-PID", strconv.Itoa(m.PID))
	w.Header().Set("X-Gtmux-Version", m.Version)
	w.Header().Set("X-Gtmux-Captured-At", m.CapturedAt.Format(time.RFC3339Nano))
	w.Header().Set("X-Gtmux-NumGC", strconv.FormatUint(uint64(m.NumGC), 10))
	if r.URL.Path == "/stats" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(m)
		return
	}
	// Buffer only a bounded encoded profile so a failure never returns a truncated
	// successful download. pprof's internal work is not context-cancellable; the
	// record preflight, single flight and cooldown bound practical capture cost.
	buf := &profileBuffer{ctx: r.Context(), limit: maxProfileBytes}
	if err := h.profile(buf, r.URL.Path[1:]); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errProfileBudget) {
			status = http.StatusRequestEntityTooLarge
		}
		http.Error(w, "memory profile unavailable or over budget", status)
		return
	}
	if r.Context().Err() != nil {
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	_, _ = w.Write(buf.Bytes())
}

var errProfileBudget = errors.New("encoded profile budget exceeded")

type profileBuffer struct {
	bytes.Buffer
	ctx   context.Context
	limit int
}

func (b *profileBuffer) Write(p []byte) (int, error) {
	if err := b.ctx.Err(); err != nil {
		return 0, err
	}
	if len(p) > b.limit-b.Len() {
		return 0, errProfileBudget
	}
	return b.Buffer.Write(p)
}
