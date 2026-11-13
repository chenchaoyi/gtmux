package restorejob

import (
	"errors"
	"fmt"
	"github.com/chenchaoyi/gtmux/internal/state"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func fixture(t *testing.T) (Store, string, string, func() (Plan, error)) {
	t.Helper()
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("XDG_DATA_HOME", filepath.Join(h, "data"))
	now := time.Unix(1800000000, 0)
	s := Store{Now: func() time.Time { return now }}
	id := fmt.Sprintf("%d-request-1234567890", now.Unix())
	rev := strings.Repeat("a", 64)
	p := func() (Plan, error) { return Plan{Revision: rev, Sessions: []Session{{Name: "missing"}}}, nil }
	return s, id, rev, p
}
func code(e error) string {
	var x *Error
	if errors.As(e, &x) {
		return x.Code
	}
	return ""
}
func TestRestoreReservationSurvivesRetryRestartAndChangedPayload(t *testing.T) {
	s, id, rev, p := fixture(t)
	var launches atomic.Int32
	launch := func(string) error {
		launches.Add(1)
		b, e := os.ReadFile(path())
		if e != nil || !strings.Contains(string(b), id) {
			t.Error("launched before durable receipt", e)
		}
		return nil
	}
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			j, e := s.Start(id, rev, p, launch)
			if e != nil || j.ID != id {
				t.Error(j, e)
			}
		}()
	}
	wg.Wait()
	if launches.Load() != 1 {
		t.Fatal("duplicate launch", launches.Load())
	}
	restarted := Store{Now: s.Now}
	j, e := restarted.Start(id, rev, func() (Plan, error) { t.Fatal("retry rebuilt a changed plan"); return Plan{}, nil }, launch)
	if e != nil || j.Phase != "queued" || launches.Load() != 1 {
		t.Fatal(j, e)
	}
	if _, e = s.Start(id, strings.Repeat("b", 64), p, launch); code(e) != "request_changed" {
		t.Fatal(e)
	}
	if _, e = s.Start(strings.Replace(id, "1234567890", "1234567891", 1), rev, p, launch); code(e) != "restore_busy" {
		t.Fatal(e)
	}
	st, e := os.Stat(path())
	if e != nil || st.Mode().Perm() != 0600 {
		t.Fatal("receipt privacy", st, e)
	}
}
func TestRestoreClaimNeverRelaunchesInterruptedOrFailedWork(t *testing.T) {
	s, id, rev, p := fixture(t)
	n := 0
	launch := func(string) error { n++; return nil }
	if _, e := s.Start(id, rev, p, launch); e != nil {
		t.Fatal(e)
	}
	_, lease, e := s.Claim(id)
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = s.Claim(id); code(e) != "already_claimed" {
		t.Fatal(e)
	}
	later := Store{Now: func() time.Time { return s.now().Add(2 * time.Minute) }}
	j, e := later.Get(id)
	if e != nil || j.Phase != "preparing" {
		t.Fatal("live lease treated interrupted", j, e)
	}
	lease.Close()
	j, e = later.Get(id)
	if e != nil || j.Phase != "interrupted" {
		t.Fatal(j, e)
	}
	j, e = later.Start(id, rev, p, launch)
	if e != nil || j.Phase != "interrupted" || n != 1 {
		t.Fatal("interrupted request restarted", j, e, n)
	}
	if _, _, e = s.Claim(id); code(e) != "already_claimed" {
		t.Fatal(e)
	}
}
func TestRestoreLaunchFailureAndExpiredRequestsRemainClosed(t *testing.T) {
	s, id, rev, p := fixture(t)
	n := 0
	j, e := s.Start(id, rev, p, func(string) error { n++; return errors.New("start failed") })
	if e != nil || j.Phase != "failed" {
		t.Fatal(j, e)
	}
	j, e = s.Start(id, rev, p, func(string) error { n++; return nil })
	if e != nil || j.Phase != "failed" || n != 1 {
		t.Fatal(j, e, n)
	}
	old := fmt.Sprintf("%d-request-1234567891", s.now().Add(-25*time.Hour).Unix())
	if _, e = s.Start(old, rev, p, func(string) error { t.Fatal("expired ran"); return nil }); code(e) != "request_expired" {
		t.Fatal(e)
	}
	if _, e = s.Start(strings.Replace(id, "1234567890", "1234567892", 1), strings.Repeat("b", 64), p, func(string) error { t.Fatal("stale ran"); return nil }); code(e) != "plan_changed" {
		t.Fatal(e)
	}
}
func TestRestoreStorageAndCapacityNeverEvictAnUnexpiredReceipt(t *testing.T) {
	s, id, rev, p := fixture(t)
	if e := state.MkdirAll(state.Dir(), 0700); e != nil {
		t.Fatal(e)
	}
	for _, broken := range []string{"corrupt", "null"} {
		if e := state.WritePrivateFile(path(), []byte(broken)); e != nil {
			t.Fatal(e)
		}
		if _, e := s.Start(id, rev, p, func(string) error { t.Fatal("corrupt store ran"); return nil }); code(e) != "storage_unavailable" {
			t.Fatal(e)
		}
	}
	rows := make([]Job, MaxJobs)
	for i := range rows {
		rows[i] = Job{ID: fmt.Sprintf("%d-request-%016d", s.now().Unix(), i), Revision: rev, Phase: "failed", CreatedAt: s.now().Unix()}
	}
	if e := write(rows); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Start(id, rev, p, func(string) error { t.Fatal("capacity launched"); return nil }); code(e) != "capacity" {
		t.Fatal(e)
	}
	rows2, e := read()
	if e != nil || len(rows2) != MaxJobs || rows2[0].ID != rows[0].ID {
		t.Fatal("receipt evicted", e)
	}
}
func TestRestoreProgressIsBoundedAndFinalOnce(t *testing.T) {
	s, id, rev, p := fixture(t)
	_, e := s.Start(id, rev, p, func(string) error { return nil })
	if e != nil {
		t.Fatal(e)
	}
	_, lease, e := s.Claim(id)
	if e != nil {
		t.Fatal(e)
	}
	defer lease.Close()
	if e = s.Update(id, "agents", "", nil, make([]Result, MaxAgents+1)); code(e) != "result_too_large" {
		t.Fatal(e)
	}
	if e = s.Update(id, "partial", "check_results", nil, []Result{{Loc: "missing:0.0", State: "pending"}}); e != nil {
		t.Fatal(e)
	}
	if e = s.Update(id, "completed", "", nil, nil); code(e) != "already_finished" {
		t.Fatal(e)
	}
	j, e := s.Get(id)
	if e != nil || j.Phase != "partial" || j.Agents[0].State != "pending" {
		t.Fatal(j, e)
	}
}
