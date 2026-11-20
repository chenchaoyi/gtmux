package app

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/artifact"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func localPreviewFixture(t *testing.T) artifact.File {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	source := filepath.Join(t.TempDir(), "report.html")
	if err := os.WriteFile(source, []byte("<p>snapshot</p>"), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := artifact.Add(source, "original-session", "original-home")
	if err != nil {
		t.Fatal(err)
	}
	return file
}
func TestLAP001LocalShowReadsSnapshotAfterOriginalChanges(t *testing.T) {
	file := localPreviewFixture(t)
	for _, change := range []string{"replace", "delete"} {
		if change == "replace" {
			if err := os.WriteFile(file.Source, []byte("different original"), 0600); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.Remove(file.Source); err != nil {
				t.Fatal(err)
			}
		}
		for _, ref := range []string{file.ID, file.Reference()} {
			code := 0
			out := captureStdout(t, func() { code = cmdArtifact([]string{"show", ref, "--json"}) })
			var got artifact.File
			if code != 0 || json.Unmarshal([]byte(out), &got) != nil || got.SHA256 != file.SHA256 || got.SessionID != file.SessionID || got.HomeID != file.HomeID {
				t.Fatal(code, out)
			}
		}
	}
}
func TestLAP001LocalShowRejectsBadRemovedAndCorruptFiles(t *testing.T) {
	file := localPreviewFixture(t)
	for _, ref := range []string{"", "../report.html", "gtmux-file://" + file.ID, file.Reference() + "?x=y", strings.ToUpper(file.ID), strings.Repeat("0", 32)} {
		if code := cmdArtifact([]string{"show", ref}); code == 0 {
			t.Fatal("accepted", ref)
		}
	}
	if err := os.WriteFile(filepath.Join(state.Dir(), "artifacts", file.ID+".data"), []byte("tampered bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if code := cmdArtifact([]string{"show", file.Reference()}); code == 0 {
		t.Fatal("corrupt snapshot accepted")
	}
	if err := artifact.Remove(file.ID); err != nil {
		t.Fatal(err)
	}
	if code := cmdArtifact([]string{"show", file.Reference()}); code == 0 {
		t.Fatal("removed snapshot accepted")
	}
}
func TestLAP002RegistrationAndOpenUseSameLocalURL(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	source := filepath.Join(t.TempDir(), "report.html")
	if err := os.WriteFile(source, []byte("<p>Fixture</p>"), 0600); err != nil {
		t.Fatal(err)
	}
	session := "report + 50% & notes"
	args := []string{"add", source, "--session-id", session}
	run := func(args []string) string {
		t.Helper()
		return captureStdout(t, func() {
			if code := cmdArtifact(args); code != 0 {
				t.Fatalf("artifact returned %d", code)
			}
		})
	}
	var receipt struct {
		artifact.File
		Reference string `json:"reference"`
		URL       string `json:"local_preview_url"`
	}
	if err := json.Unmarshal([]byte(run(append(args, "--json"))), &receipt); err != nil {
		t.Fatal(err)
	}
	markdown := run(args)
	links := artifact.Links(markdown)
	if len(links) != 2 || links[0] != receipt.File.Reference() || links[0] != receipt.Reference || links[1] != receipt.URL {
		t.Fatal(markdown)
	}
	if got := strings.TrimSpace(run([]string{"open", receipt.Reference, "--print-url"})); got != receipt.URL {
		t.Fatal(got)
	}
	old := artifactPreviewLaunch
	t.Cleanup(func() { artifactPreviewLaunch = old })
	var opened string
	artifactPreviewLaunch = func(endpoint string) error { opened = endpoint; return nil }
	run([]string{"open", receipt.ID, "--port", "12345"})
	u, err := url.Parse(opened)
	if err != nil || u.Scheme != "http" || u.Host != "127.0.0.1:12345" || u.Path != "/" || u.User != nil || u.RawQuery != "" {
		t.Fatal(opened, err)
	}
	_, raw, ok := strings.Cut(opened, "#")
	q, err := url.ParseQuery(raw)
	if !ok || err != nil || len(q) != 3 || q.Get("session") != session || q.Get("document") != receipt.ID || !q.Has("home") || q.Get("home") != "" {
		t.Fatal(q, err)
	}
	artifactPreviewLaunch = func(string) error { return errors.New("synthetic opener failure") }
	if cmdArtifact([]string{"open", receipt.ID}) == 0 {
		t.Fatal("opener failure hidden")
	}
	for _, args := range [][]string{{"open", receipt.ID, "--port", "0"}, {"open", receipt.ID, "--port", "65536"}, {"open", receipt.ID, "--json"}, {"show", receipt.ID, "--print-url"}} {
		if cmdArtifact(args) != 2 {
			t.Fatal(args)
		}
	}
}
