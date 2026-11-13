package server

import (
	"errors"
	"github.com/chenchaoyi/gtmux/internal/restorejob"
	"net/http"
	"strings"
	"testing"
)

func TestRestoreOwnerContractNeverStartsOnRead(t *testing.T) {
	reads, starts := 0, 0
	id := "1800000000-request-1234567890"
	rev := strings.Repeat("a", 64)
	em := NewEnrollManager(nil, nil)
	owner, _ := em.Redeem(em.Mint(), "owner")
	guest := em.MintGuest("guest", []string{"%7"}, []string{"%7"}, 0)
	deps := Deps{Enroll: em, RestorePreview: func() (restorejob.Snapshot, error) {
		reads++
		return restorejob.Snapshot{Plan: restorejob.Plan{Revision: rev}}, nil
	}, RestoreStatus: func(string) (*restorejob.Job, error) { reads++; return &restorejob.Job{ID: id}, nil }, RestoreStart: func(gotID, gotRev string) (*restorejob.Job, error) {
		starts++
		if gotID != id || gotRev != rev {
			t.Fatal(gotID, gotRev)
		}
		return &restorejob.Job{ID: id}, nil
	}}
	h := New(Config{Token: testToken}, deps).Handler()
	body := `{"request_id":"` + id + `","revision":"` + rev + `"}`
	for _, path := range []string{"/api/restore", "/api/restore/job?id=" + id} {
		if r := do(t, h, http.MethodGet, path, guest.Token); r.Code != 403 {
			t.Fatal(r.Code)
		}
		if r := do(t, h, http.MethodGet, path, owner.Token); r.Code != 200 {
			t.Fatal(r.Code)
		}
	}
	if starts != 0 || reads != 2 {
		t.Fatal(reads, starts)
	}
	if r := post(t, h, "/api/restore", guest.Token, body); r.Code != 403 {
		t.Fatal(r.Code)
	}
	if starts != 0 {
		t.Fatal("guest started restore")
	}
	if r := post(t, h, "/api/restore", owner.Token, body); r.Code != 202 {
		t.Fatal(r.Code)
	}
	for _, b := range []string{`{}`, body + body, strings.Replace(body, `"revision"`, `"command"`, 1), `{"request_id":"short"}`, strings.Repeat("x", 1025)} {
		n := starts
		if r := post(t, h, "/api/restore", testToken, b); r.Code != 400 || starts != n {
			t.Fatal(r.Code, starts, n)
		}
	}
	if r := do(t, h, http.MethodDelete, "/api/restore", testToken); r.Code != 405 {
		t.Fatal(r.Code)
	}
}
func TestRestoreKnownFailureCodesStayDistinct(t *testing.T) {
	for _, v := range []struct {
		code   string
		status int
	}{{"plan_changed", 409}, {"request_changed", 409}, {"restore_busy", 409}, {"request_expired", 409}, {"not_found", 404}, {"storage_unavailable", 503}} {
		h := New(Config{Token: testToken}, Deps{RestorePreview: func() (restorejob.Snapshot, error) { return restorejob.Snapshot{}, &restorejob.Error{Code: v.code} }}).Handler()
		r := do(t, h, http.MethodGet, "/api/restore", testToken)
		if r.Code != v.status || !strings.Contains(r.Body.String(), v.code) {
			t.Fatal(v, r.Code, r.Body)
		}
	}
	h := New(Config{Token: testToken}, Deps{RestorePreview: func() (restorejob.Snapshot, error) {
		return restorejob.Snapshot{}, errors.New("private path must not leak")
	}}).Handler()
	r := do(t, h, http.MethodGet, "/api/restore", testToken)
	if r.Code != 503 || strings.Contains(r.Body.String(), "private path") {
		t.Fatal(r.Code, r.Body)
	}
}
