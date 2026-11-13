package codexbridge

import (
	"encoding/json"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func recoveryFixture(t *testing.T) (*frontendBinding, *observer, *time.Time, *[]string) {
	t.Helper()
	o, _ := readyObserver(t)
	now := time.Unix(1800000000, 0)
	var calls []string
	b := newFrontendBinding(resume.CodexFrontend{Instance: "fixture", Sequence: 1}, func() bool { return true }, nil)
	b.now = func() time.Time { return now }
	b.end = func(resume.CodexFrontend) {}
	b.refresh = func(f resume.CodexFrontend) (resume.CodexFrontend, error) {
		calls = append(calls, "refresh:"+f.SessionID)
		return f, nil
	}
	b.bind = func(f resume.CodexFrontend, id, cwd string) (resume.CodexFrontend, error) {
		calls = append(calls, "bind:"+id)
		f.SessionID = id
		return f, nil
	}
	o.bind = b.stage
	o.selectRoot = b.selectRoot
	o.resetHome = b.resetHome
	return b, o, &now, &calls
}
func TestCBA002TransientRootBindingRecoversFromTheActualObserver(t *testing.T) {
	b, o, clock, calls := recoveryFixture(t)
	first := b.bind
	failed := false
	b.bind = func(f resume.CodexFrontend, id, cwd string) (resume.CodexFrontend, error) {
		if !failed {
			failed = true
			*calls = append(*calls, "unavailable:"+id)
			return f, state.ErrLeaseHeld
		}
		return first(f, id, cwd)
	}
	o.request([]byte(`{"id":1,"method":"thread/resume"}`))
	o.response(rootResponse(1, "current-root"))
	if b.attachment.SessionID != "" {
		t.Fatal("unknown proof granted association")
	}
	*clock = clock.Add(time.Second)
	b.tick()
	if b.attachment.SessionID != "current-root" {
		t.Fatalf("lost successful root after a transient failure: attachment=%q calls=%v", b.attachment.SessionID, *calls)
	}
	b.tick()
	if len(*calls) != 3 || (*calls)[1] != "bind:current-root" || (*calls)[2] != "refresh:current-root" {
		t.Fatal("not one bind followed by normal heartbeat", *calls)
	}
}
func TestCBA003NewSelectionAndEpochRetirePendingRecovery(t *testing.T) {
	for _, mode := range []string{"selection", "epoch", "home", "late"} {
		t.Run(mode, func(t *testing.T) {
			b, o, clock, calls := recoveryFixture(t)
			b.bind = func(f resume.CodexFrontend, id, cwd string) (resume.CodexFrontend, error) {
				*calls = append(*calls, id)
				return f, state.ErrLeaseHeld
			}
			o.request([]byte(`{"id":1,"method":"thread/resume"}`))
			o.response(rootResponse(1, "old"))
			switch mode {
			case "selection":
				o.request([]byte(`{"id":2,"method":"thread/start"}`))
			case "epoch":
				b.current = func() bool { return false }
			case "home":
				o.request([]byte(`{"id":2,"method":"initialize"}`))
				o.response([]byte(`{"id":2,"result":{"codexHome":"/wrong"}}`))
			case "late":
				o.request([]byte(`{"id":2,"method":"thread/start"}`))
				o.response(rootResponse(1, "old"))
			}
			*clock = clock.Add(time.Second)
			b.tick()
			if len(*calls) != 1 || b.attachment.SessionID != "" {
				t.Fatal("stale pending root retried", *calls)
			}
		})
	}
}
func TestCBA003PendingNewRootNeverRefreshesOldRoot(t *testing.T) {
	b, o, clock, calls := recoveryFixture(t)
	b.attachment.SessionID = "old"
	b.bind = func(f resume.CodexFrontend, id, cwd string) (resume.CodexFrontend, error) {
		*calls = append(*calls, "unavailable:"+id)
		return f, state.ErrLeaseHeld
	}
	o.request([]byte(`{"id":1,"method":"thread/fork"}`))
	o.response(rootResponse(1, "new"))
	*clock = clock.Add(time.Second)
	b.tick()
	for _, c := range *calls {
		if c == "refresh:old" {
			t.Fatal("failed new-root publication kept old root current", *calls)
		}
	}
}
func TestCBA003RetryPacingDeadlineAndHardRefusal(t *testing.T) {
	for _, hard := range []bool{false, true} {
		t.Run(map[bool]string{false: "transient", true: "hard"}[hard], func(t *testing.T) {
			b, o, clock, calls := recoveryFixture(t)
			b.bind = func(f resume.CodexFrontend, id, cwd string) (resume.CodexFrontend, error) {
				*calls = append(*calls, id)
				return f, &resume.FrontendBindingError{Code: "proof", Retryable: !hard}
			}
			o.request([]byte(`{"id":1,"method":"thread/resume"}`))
			o.response(rootResponse(1, "root"))
			for range 100 {
				b.tick()
			}
			if len(*calls) != 1 {
				t.Fatal("busy retry", *calls)
			}
			for range 60 {
				*clock = clock.Add(time.Second)
				b.tick()
			}
			n := len(*calls)
			if hard && n != 1 || !hard && (n < 2 || n > 8) {
				t.Fatal("attempt budget", n)
			}
			*clock = clock.Add(time.Hour)
			b.tick()
			if len(*calls) != n {
				t.Fatal("retry survived deadline")
			}
		})
	}
}
func TestCBA003ReceiptFailureCommitsThenOnlyHeartbeats(t *testing.T) {
	b, o, clock, calls := recoveryFixture(t)
	b.bind = func(f resume.CodexFrontend, id, cwd string) (resume.CodexFrontend, error) {
		*calls = append(*calls, "bind:"+id)
		f.SessionID = id
		return f, &resume.CodexBindingReceiptError{Err: errors.New("receipt write")}
	}
	o.request([]byte(`{"id":1,"method":"thread/start"}`))
	o.response(rootResponse(1, "committed"))
	*clock = clock.Add(time.Second)
	b.tick()
	if b.attachment.SessionID != "committed" || len(*calls) != 2 || (*calls)[1] != "refresh:committed" {
		t.Fatal(*calls)
	}
}
func TestCBA003ClosedConnectionCannotPublishLater(t *testing.T) {
	b, o, clock, calls := recoveryFixture(t)
	var current atomic.Bool
	current.Store(true)
	b.current = current.Load
	b.bind = func(f resume.CodexFrontend, id, cwd string) (resume.CodexFrontend, error) {
		*calls = append(*calls, id)
		return f, state.ErrLeaseHeld
	}
	o.request([]byte(`{"id":1,"method":"thread/resume"}`))
	o.response(rootResponse(1, "root"))
	b.close()
	*clock = clock.Add(time.Second)
	b.tick()
	if len(*calls) != 1 {
		t.Fatal("closed connection retried", *calls)
	}
}

func TestCBA003InitializeInvalidatesEarlierRootAndEarlierHomeResponse(t *testing.T) {
	b, o, clock, calls := recoveryFixture(t)
	o.request([]byte(`{"id":1,"method":"thread/resume"}`))
	o.request([]byte(`{"id":2,"method":"initialize"}`))
	o.request([]byte(`{"id":3,"method":"initialize"}`))
	if o.homeVerified() {
		t.Fatal("reinitialize retained old home authority")
	}
	o.response([]byte(`{"id":3,"result":{"codexHome":"/wrong"}}`))
	home, _ := json.Marshal(os.Getenv("CODEX_HOME"))
	o.response([]byte(`{"id":2,"result":{"codexHome":` + string(home) + `}}`))
	if o.homeVerified() {
		t.Fatal("late initialize restored superseded home proof")
	}
	o.response(rootResponse(1, "old-root"))
	*clock = clock.Add(time.Second)
	b.tick()
	if len(*calls) != 0 {
		t.Fatal("pre-initialize root published", *calls)
	}
}

func TestCBA003NewVerifiedRootReplacesOnlyItsOwnPendingSelection(t *testing.T) {
	b, o, clock, calls := recoveryFixture(t)
	bind := b.bind
	b.bind = func(f resume.CodexFrontend, id, cwd string) (resume.CodexFrontend, error) {
		if id == "old" {
			*calls = append(*calls, "unavailable:old")
			return f, state.ErrLeaseHeld
		}
		return bind(f, id, cwd)
	}
	o.request([]byte(`{"id":1,"method":"thread/resume"}`))
	o.response(rootResponse(1, "old"))
	o.request([]byte(`{"id":2,"method":"thread/resume"}`))
	o.response(rootResponse(2, "new"))
	*clock = clock.Add(time.Second)
	b.tick()
	if b.attachment.SessionID != "new" || len(*calls) != 3 || (*calls)[2] != "refresh:new" {
		t.Fatal("newest verified selection lost", *calls)
	}
}

func TestCBA003NoAttemptAtDeadlineOrAfterEpochChange(t *testing.T) {
	for _, mode := range []string{"deadline", "epoch"} {
		t.Run(mode, func(t *testing.T) {
			b, o, clock, calls := recoveryFixture(t)
			b.bind = func(f resume.CodexFrontend, id, cwd string) (resume.CodexFrontend, error) {
				*calls = append(*calls, id)
				return f, state.ErrLeaseHeld
			}
			o.request([]byte(`{"id":1,"method":"thread/start"}`))
			o.response(rootResponse(1, "root"))
			if mode == "deadline" {
				*clock = clock.Add(bindingRecoveryBudget)
			} else {
				b.current = func() bool { return false }
				*clock = clock.Add(time.Second)
			}
			b.tick()
			if len(*calls) != 1 || b.pending != nil {
				t.Fatal("expired identity retried", *calls)
			}
		})
	}
}

func TestCBA003UnrelatedTrafficCannotReplacePendingRoot(t *testing.T) {
	b, o, clock, calls := recoveryFixture(t)
	bind := b.bind
	first := true
	b.bind = func(f resume.CodexFrontend, id, cwd string) (resume.CodexFrontend, error) {
		if first {
			first = false
			*calls = append(*calls, "unavailable:"+id)
			return f, state.ErrLeaseHeld
		}
		return bind(f, id, cwd)
	}
	o.request([]byte(`{"id":1,"method":"thread/resume"}`))
	o.response(rootResponse(1, "real"))
	for _, request := range []string{
		`{"id":2,"method":"thread/start","params":{"ephemeral":true}}`,
		`{"id":3,"method":"thread/read"}`,
		`{"id":4,"method":"plugin/list"}`,
		`{"id":5,"method":"turn/start"}`,
	} {
		o.request([]byte(request))
	}
	for id := 2; id <= 5; id++ {
		o.response(rootResponse(id, "foreign"))
	}
	o.response([]byte(`{"method":"thread/started","params":{"thread":{"id":"foreign"}}}`))
	*clock = clock.Add(time.Second)
	b.tick()
	if b.attachment.SessionID != "real" || len(*calls) != 2 || (*calls)[1] != "bind:real" {
		t.Fatal("unrelated traffic changed the pending root", *calls)
	}
}

func TestCBA002SameRootReconnectRecoversWithoutRefreshingUnverifiedProof(t *testing.T) {
	b, o, clock, calls := recoveryFixture(t)
	b.attachment.SessionID = "same-root"
	bind := b.bind
	first := true
	b.bind = func(f resume.CodexFrontend, id, cwd string) (resume.CodexFrontend, error) {
		if first {
			first = false
			*calls = append(*calls, "unavailable:"+id)
			return f, &resume.FrontendBindingError{Code: "terminal-proof-unavailable", Retryable: true}
		}
		return bind(f, id, cwd)
	}
	o.request([]byte(`{"id":1,"method":"thread/resume"}`))
	o.response(rootResponse(1, "same-root"))
	b.tick()
	if len(*calls) != 1 {
		t.Fatal("unverified attachment heartbeat continued", *calls)
	}
	*clock = clock.Add(time.Second)
	b.tick()
	b.tick()
	if len(*calls) != 3 || (*calls)[1] != "bind:same-root" || (*calls)[2] != "refresh:same-root" {
		t.Fatal(*calls)
	}
}
