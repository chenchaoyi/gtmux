package app

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// readBundle returns the archive's files by name.
func readBundle(t *testing.T, path string) map[string]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(zr)
	out := map[string]string{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(tr)
		out[strings.TrimPrefix(h.Name, "gtmux-diagnostics/")] = string(b)
	}
	return out
}

// The bundle holds what someone needs to see why gtmux misbehaved, with every credential
// gtmux keeps replaced, and the journal only when asked for.
func TestTheBundlePacksTheRecordAndNoCredential(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GTMUX_RELAY_TOKEN", "")
	t.Chdir(t.TempDir())
	token := "3f9c20e1a7b64d0f9e2c11aa55bb66cc"
	deviceToken := "devtok_9a8b7c6d5e4f3a2b1c0d"
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(state.ConfigDir(), "serve-token"), token+"\n")
	write(filepath.Join(state.ConfigDir(), "devices.json"), `{"devices":[{"id":"d1","token":"`+deviceToken+`"}]}`)
	day := time.Now().Format("2006-01-02")
	// An entry from before the store redacted at write time, as an old line would be.
	write(filepath.Join(state.LogsDir(), day+".jsonl"), `{"ts":"x","event":"serve.start","msg":"token `+token+`"}`+"\n")
	write(filepath.Join(state.StatusDir(), "tunnel.json"), `{"component":"tunnel","state":"connected"}`)
	write(state.CapturePath("serve"), "listening; test: curl -H \"Authorization: Bearer "+token+"\"\n")
	write(filepath.Join(state.Dir(), "serve.log"), "phone paired with "+deviceToken+"\n")
	write(filepath.Join(state.Dir(), "events.jsonl"), `{"event":"UserPromptSubmit","summary":"a private prompt"}`+"\n")

	secs := []dsection{{title: "Logs", rows: []dcheck{{stOK, "log store", "2 KB", "keeps 30 days"}}}}
	out := filepath.Join(t.TempDir(), "report.tgz")
	if rc := doctorBundle(out, false, secs); rc != 0 {
		t.Fatalf("doctorBundle rc=%d", rc)
	}
	files := readBundle(t, out)
	for _, name := range []string{"logs/" + day + ".jsonl", "status/tunnel.json", "captures/serve.stderr",
		"captures/serve.log", "doctor.txt", "versions.txt"} {
		if _, ok := files[name]; !ok {
			t.Errorf("the bundle lacks %s; has %v", name, keys(files))
		}
	}
	if _, ok := files["events.jsonl"]; ok {
		t.Error("the journal was packed without --with-events")
	}
	for name, body := range files {
		for _, secret := range []string{token, deviceToken} {
			if strings.Contains(body, secret) {
				t.Errorf("%s carries a credential:\n%s", name, body)
			}
		}
	}
	if !strings.Contains(files["doctor.txt"], "[ok] log store: 2 KB (keeps 30 days)") {
		t.Errorf("doctor.txt:\n%s", files["doctor.txt"])
	}
	if fi, _ := os.Stat(out); fi.Mode().Perm() != 0o600 {
		t.Errorf("the bundle is %v, want 0600", fi.Mode().Perm())
	}

	// It never overwrites, and the journal goes in when asked for.
	if rc := doctorBundle(out, true, secs); rc != 1 {
		t.Fatalf("an existing file was overwritten (rc=%d)", rc)
	}
	out2 := filepath.Join(t.TempDir(), "with-events.tgz")
	if rc := doctorBundle(out2, true, secs); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	if !strings.Contains(readBundle(t, out2)["events.jsonl"], "a private prompt") {
		t.Error("--with-events did not pack the journal")
	}
}

func keys(m map[string]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
