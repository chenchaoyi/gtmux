package hq

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const processObservation = `[{"id":"legacy-custom_01","kind":"report","session":"synthetic","body":"synthetic only","for":"hq","state":"closed","created_at":1}]`

func processFixture(t *testing.T, root, name, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestDPC001AutomaticScopePreservesUnknownNotesAndIncludes(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"AGENTS.md": "instructions", "knowledge/rule.md": "protected knowledge", "workspace/item.json": "{}",
		"notes/relay-old-all.json": processObservation, "notes/tick-old-relay.json": processObservation,
		"notes/task-relays-before.json": processObservation, "notes/ordinary.json": processObservation,
		"notes/relay-partial.json": `[{"body":"retain unknown note"}]`, "notes/relay-empty.json": "[]",
		"notes/relay-broken.json": "[", "notes/board.md": "concise summary",
	}
	for name, body := range files {
		processFixture(t, root, name, body)
	}
	core, err := selectArchive(root, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	selected := map[string]bool{}
	for _, item := range core {
		selected[item.name] = true
	}
	for name := range files {
		want := name != "notes/relay-old-all.json" && name != "notes/tick-old-relay.json" && name != "notes/task-relays-before.json"
		if selected[name] != want {
			t.Fatalf("wrong core selection for %s", name)
		}
	}
	full, err := selectArchive(root, false, nil)
	if err != nil || len(full) != len(files) {
		t.Fatalf("full export narrowed: %d %v", len(full), err)
	}
	var packed bytes.Buffer
	if err := writeSelectedArchive(&packed, root, true, []string{"notes/relay-old-all.json"}); err != nil {
		t.Fatal(err)
	}
	gz, err := gzip.NewReader(&packed)
	if err != nil {
		t.Fatal(err)
	}
	tr, included := tar.NewReader(gz), false
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(tr)
		if err != nil || string(body) != files[h.Name] {
			t.Fatalf("archive content changed: %s %v", h.Name, err)
		}
		included = included || h.Name == "notes/relay-old-all.json"
	}
	if _, err := io.Copy(io.Discard, gz); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil || !included {
		t.Fatalf("explicit include not preserved: %v", err)
	}
	for name, body := range files {
		got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil || string(got) != body {
			t.Fatal("read-only selection modified a source", name)
		}
	}
	if archiveMaxBytes != 128<<20 || archiveFileMaxBytes != 32<<20 || archiveMaxEntries != 10000 {
		t.Fatal("recognition must not raise recovery limits")
	}
	large := filepath.Join(root, "notes", "relay-large-unknown.json")
	processFixture(t, root, "notes/relay-large-unknown.json", processObservation)
	if err := os.Truncate(large, archiveFileMaxBytes+1); err != nil {
		t.Fatal(err)
	}
	if _, err := selectArchive(root, true, nil); err == nil {
		t.Fatal("unknown oversized note must fail honestly, not be dropped")
	}
	if info, err := os.Stat(large); err != nil || info.Size() != archiveFileMaxBytes+1 {
		t.Fatal("limit failure modified protected source")
	}
}

func TestDPC002ProcessMaterialInventoryProtectsCoreAndUnknown(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"notes/relay-old-all.json": processObservation, "notes/reports/job/device.zip": "unique package",
		"notes/reports/job/final.md": "summary", "notes/reports/job/owner.json": "{}",
		"notes/investigation.py": "print('synthetic')", "notes/legacy-knowledge.tar.gz": "unknown recovery history",
		"knowledge/record.md": "core", "workspace/item.json": "core", "LOCAL.md": "user instruction",
		"notes/ordinary.json": "{}", "notes/relay-unknown.json": `[{"body":"keep"}]`,
	}
	for name, body := range files {
		processFixture(t, root, name, body)
	}
	outside := filepath.Join(t.TempDir(), "private.log")
	if err := os.WriteFile(outside, []byte("not HQ"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "notes", "outside.log")); err != nil {
		t.Fatal(err)
	}
	s := processMaterialState(root)
	want := ProcessMaterialState{Files: 3, RelayBytes: int64(len(processObservation)),
		AttachmentBytes: int64(len("unique package")), SourceBytes: int64(len("print('synthetic')")),
		UnclassifiedArchiveBytes: int64(len("unknown recovery history"))}
	want.Bytes = want.RelayBytes + want.AttachmentBytes + want.SourceBytes
	if s != want || !reflect.DeepEqual(s, processMaterialState(root)) {
		t.Fatalf("wrong inventory: %+v want %+v", s, want)
	}
	for name, body := range files {
		got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil || string(got) != body {
			t.Fatal("inventory changed source", name)
		}
	}
	if got := processMaterialState(filepath.Join(t.TempDir(), "absent")); got != (ProcessMaterialState{}) {
		t.Fatalf("new HQ was treated as damaged: %+v", got)
	}
}
