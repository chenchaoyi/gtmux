package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func todaysLog(t *testing.T) string {
	t.Helper()
	b, _ := os.ReadFile(filepath.Join(state.LogsDir(), time.Now().Format("2006-01-02")+".jsonl"))
	return string(b)
}

// The question 2026-09-19 could not answer: a phone's pairing failed, and nothing on the
// Mac said why. Each of the three reasons is now its own line.
func TestARejectedPairingSaysWhy(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	en := NewEnrollManager(nil, nil)
	now := time.Now()
	en.now = func() time.Time { return now }
	s := New(Config{Addr: "x", Token: "master"}, Deps{Enroll: en})
	h := s.Handler()
	redeem := func(code string) int {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/enroll",
			strings.NewReader(`{"enrollCode":"`+code+`","name":"iPhone"}`)))
		return rr.Code
	}

	used := en.Mint()
	if redeem(used) != http.StatusOK {
		t.Fatal("a fresh code did not redeem")
	}
	redeem(used) // second use
	stale := en.Mint()
	en.now = func() time.Time { return now.Add(6 * time.Minute) }
	redeem(stale)              // expired
	redeem("0000000000000000") // never issued by this boot

	log := todaysLog(t)
	for _, why := range []string{`"reason":"used"`, `"reason":"expired"`, `"reason":"unknown"`} {
		if !strings.Contains(log, why) {
			t.Errorf("no act.pair entry with %s:\n%s", why, log)
		}
	}
	if !strings.Contains(log, `"event":"act.pair"`) || !strings.Contains(log, `"outcome":"ok"`) {
		t.Errorf("the successful pairing was not recorded:\n%s", log)
	}
	if strings.Contains(log, used) || strings.Contains(log, stale) {
		t.Error("a pairing code reached the log")
	}
}

// A send is recorded with its size and a short hash; the text never reaches the log.
func TestASendIsRecordedWithoutItsText(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var sent string
	s := New(Config{Addr: "x", Token: "master"}, Deps{
		Send: func(id, text, key string, enter bool, sendID string) error { sent = text; return nil },
	})
	const secret = "please deploy the thing to production now"
	req := httptest.NewRequest(http.MethodPost, "/api/send", strings.NewReader(`{"id":"%7","text":"`+secret+`"}`))
	req.Header.Set("Authorization", "Bearer master")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || sent != secret {
		t.Fatalf("send = %d, delivered %q", rr.Code, sent)
	}
	log := todaysLog(t)
	if !strings.Contains(log, `"event":"act.send"`) || !strings.Contains(log, `"actor":"owner"`) ||
		!strings.Contains(log, `"target":"%7"`) {
		t.Errorf("the send was not recorded as an action by the owner on %%7:\n%s", log)
	}
	if strings.Contains(log, "production") {
		t.Error("the message text reached the log")
	}
}

// A scanner hitting the tunnel with bad tokens must not be able to fill the log.
func TestRejectedTokensAreAggregated(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	authRejects = rejects{}
	s := New(Config{Addr: "x", Token: "master"}, Deps{Enroll: NewEnrollManager(nil, nil)})
	h := s.Handler()
	for i := 0; i < 500; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
		req.Header.Set("Authorization", "Bearer nope")
		h.ServeHTTP(httptest.NewRecorder(), req)
	}
	if n := strings.Count(todaysLog(t), "auth.rejected"); n > 2 {
		t.Errorf("500 rejections in a minute wrote %d entries, want at most 2", n)
	}
}

func TestAPanickingHandlerIsRecordedAndAnswered(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := New(Config{Addr: "x", Token: "master"}, Deps{})
	h := s.recoverPanics(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/x", nil))
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("a panic answered %d, want 500", rr.Code)
	}
	if log := todaysLog(t); !strings.Contains(log, "handler.panic") || !strings.Contains(log, "boom") {
		t.Errorf("the panic was not recorded:\n%s", log)
	}
}

func TestDeviceActorsSayWhatTheDeviceIs(t *testing.T) {
	cases := map[string]EnrolledDevice{
		"phone:3f9c20e1":   {ID: "3f9c20e1aa", Platform: "iOS 26.6"},
		"browser:77aa11bb": {ID: "77aa11bbcc", Platform: "Safari · macOS"},
		"guest:12345678":   {ID: "1234567890", Scope: scopeGuest},
		"device:abcdef12":  {ID: "abcdef1234", Name: "work-mbp"},
	}
	for want, d := range cases {
		if got := deviceActor(d); got != want {
			t.Errorf("deviceActor(%+v) = %q, want %q", d, got, want)
		}
	}
}
