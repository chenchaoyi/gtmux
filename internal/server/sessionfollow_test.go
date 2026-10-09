package server

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
)

func TestSessionFollowIsOwnerOnlyAndReportsSaveErrors(t *testing.T) {
	calls := 0
	s := New(Config{Token: "master"}, Deps{SessionFollow: func(id string, next *sessionpolicy.Settings, actor string) (sessionpolicy.Settings, error) {
		calls++
		if actor == "anonymous" {
			t.Fatal("authenticated caller lost its audit identity")
		}
		if next != nil {
			return sessionpolicy.Settings{}, sessionpolicy.ErrConflict
		}
		return sessionpolicy.Settings{}, nil
	}})
	en := NewEnrollManager(nil, func([]EnrolledDevice) {})
	s.deps.Enroll = en
	guest := en.MintGuest("guest", []string{"%1"}, nil, 0)
	request := func(method, body, token string) int {
		r := httptest.NewRequest(method, "/api/session-follow?session_id=desk", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w.Code
	}
	for _, method := range []string{"GET", "POST"} {
		if got := request(method, `{"session_id":"desk","settings":{"hq":true,"revision":0}}`, guest.Token); got != 403 {
			t.Fatalf("guest %s=%d", method, got)
		}
	}
	if calls != 0 {
		t.Fatal("guest reached permissions")
	}
	owner, ok := en.Redeem(en.Mint(), "phone")
	if !ok {
		t.Fatal("owner fixture")
	}
	if got := request("GET", "", owner.Token); got != 200 {
		t.Fatalf("paired owner=%d", got)
	}
	if got := request("GET", "", "master"); got != 200 {
		t.Fatal(got)
	}
	if got := request("POST", `{"session_id":"desk","settings":{"hq":true,"revision":0}}`, "master"); got != 409 {
		t.Fatal(got)
	}
	for _, body := range []string{`{}`, `{"session_id":"desk","settings":null}`, `{"session_id":"desk","settings":{"hq":true},"unknown":1}`, `{"session_id":"desk","settings":{}} {}`} {
		if got := request("POST", body, "master"); got != 400 {
			t.Fatalf("malformed %s=%d", body, got)
		}
	}
}

func TestDesktopHubConsentDoesNotReplayOrAlertOtherSessions(t *testing.T) {
	var alerts []Alert
	var cur []AgentStatus
	var tally Tally
	h := newHub(func() []AgentStatus { return cur }, 0, func(a Alert) { alerts = append(alerts, a) })
	h.onTally = func(v Tally) { tally = v }
	a := AgentStatus{Agent: "Codex", SessionID: "desktop", Client: "chatgpt_desktop", StatusOnly: true, Status: "working"}
	b := AgentStatus{Agent: "Claude Code", SessionID: "terminal", Status: "working"}
	cur = []AgentStatus{a, b}
	h.tick()
	a.Status = "waiting"
	cur = []AgentStatus{a, b}
	h.tick()
	if tally.Waiting != 0 || tally.Working != 1 || len(alerts) != 0 {
		t.Fatalf("status-only caused attention: %+v %+v", tally, alerts)
	}
	a.StatusOnly = false
	cur = []AgentStatus{a, b}
	h.tick()
	if tally.Waiting != 1 || len(alerts) != 0 {
		t.Fatal("follow implies notify, or enrollment was not counted")
	}
	a.Notify = true
	cur = []AgentStatus{a, b}
	h.tick()
	if len(alerts) != 0 {
		t.Fatal("enrollment replayed waiting alert")
	}
	a.Status = "working"
	cur = []AgentStatus{a, b}
	h.tick()
	a.Status = "idle"
	cur = []AgentStatus{a, b}
	h.tick()
	if len(alerts) != 1 || alerts[0].SessionID != "desktop" || alerts[0].Client != "chatgpt_desktop" {
		t.Fatalf("native alert lost identity: %+v", alerts)
	}
	alerts = nil
	a.Notify = false
	a.StatusOnly = true
	a.Status = "waiting"
	cur = []AgentStatus{a, b}
	h.tick()
	if len(alerts) != 0 || len(h.waitAlertAt) != 0 {
		t.Fatal("revoked permission kept reminders")
	}
	b.Status = "idle"
	cur = []AgentStatus{a, b}
	h.tick()
	if len(alerts) != 1 || alerts[0].Agent != "Claude Code" {
		t.Fatal("other agent affected")
	}
}

func TestQueuedDesktopPushRespectsRevocation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	relay := &fakeRelay{}
	pm := NewPushManager(relay, nil, nil, "Mac", nil)
	pm.Register(DeviceToken{Origin: originMaster, Token: "tok", Platform: "ios", Env: "sandbox"})
	a := Alert{Kind: "done", Agent: "Codex", SessionID: "not-authorized", Client: "chatgpt_desktop"}
	pm.dispatch(a)
	if len(relay.intents()) != 0 {
		t.Fatal("queued desktop alert ignored current permission")
	}
	if alertCollapseID(a) == alertCollapseID(Alert{SessionID: "another", Client: "chatgpt_desktop"}) {
		t.Fatal("unrelated native banners collapse together")
	}
}
