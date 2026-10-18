package sessionpolicy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dsh"
)

func dshFixture(t *testing.T, id string) {
	t.Helper()
	p := filepath.Join(dsh.Home(), "sessions", "project", id)
	if err := os.MkdirAll(p, 0700); err != nil {
		t.Fatal(err)
	}
	b := `{"type":"session","version":4,"id":"` + id + `","cwd":"/project"}` + "\n" + `{"type":"user/message","time":2000,"data":{"role":"user","source":{"kind":"user"}}}` + "\n"
	if err := os.WriteFile(filepath.Join(p, "session.v4.jsonl"), []byte(b), 0600); err != nil {
		t.Fatal(err)
	}
}
func TestDSHConsentIsScopedIndependentAndFutureOnly(t *testing.T) {
	setup(t)
	t.Setenv("DSH_HOME", t.TempDir())
	dshFixture(t, "desk")
	if _, err := Resolve("desk", ""); err == nil {
		t.Fatal("same id in two agents was guessed")
	}
	if ref, err := Resolve("desk", dsh.HomeID()); err != nil || ref.Agent != "dsh" {
		t.Fatal(ref, err)
	}
	v, err := saveAtFor("dsh", "desk", Settings{HQ: true, Notify: true}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if Get("desk").HQ || GetFor("dsh", "desk").Knowledge {
		t.Fatal("grant crossed agents or implied learning")
	}
	if ObserveEvent("", "dsh", "desk", "unconfirmed", 100, dsh.HomeID()) || ObserveEvent("", "dsh", "desk", "unconfirmed", 110, "foreign") {
		t.Fatal("old or foreign activity admitted")
	}
	if !ObserveEvent("", "DeepSeek Harness", "desk", "unconfirmed", 102, dsh.HomeID()) || LearnEvent("", "dsh", "desk", "unconfirmed", 102, dsh.HomeID()) {
		t.Fatal("independent HQ/learning gates lost")
	}
	v.Knowledge = true
	v, err = saveAtFor("dsh", "desk", v, 200)
	if err != nil {
		t.Fatal(err)
	}
	if LearnEvent("", "dsh", "desk", "unconfirmed", 199, dsh.HomeID()) || !LearnEvent("", "dsh", "desk", "unconfirmed", 202, dsh.HomeID()) {
		t.Fatal("knowledge boundary lost")
	}
	oldHome := dsh.HomeID()
	t.Setenv("DSH_HOME", t.TempDir())
	dshFixture(t, "desk")
	if GetFor("dsh", "desk").HQ || ObserveEvent("", "dsh", "desk", "unconfirmed", 300, oldHome) {
		t.Fatal("copied id inherited old-home grant")
	}
}
func TestDSHRevocationAndStaleSave(t *testing.T) {
	setup(t)
	t.Setenv("DSH_HOME", t.TempDir())
	dshFixture(t, "root")
	v, err := saveAtFor("dsh", "root", Settings{HQ: true, Knowledge: true, Notify: true}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = saveAtFor("dsh", "root", Settings{HQ: true}, 101); err != ErrConflict {
		t.Fatal("stale permission write accepted", err)
	}
	v.HQ = false
	v, err = saveAtFor("dsh", "root", v, 200)
	if err != nil {
		t.Fatal(err)
	}
	if v.Notify || v.Knowledge || ObserveEvent("", "dsh", "root", "", 300, dsh.HomeID()) || LearnEvent("", "dsh", "root", "", 300, dsh.HomeID()) {
		t.Fatal("revocation left dependent permissions")
	}
}
