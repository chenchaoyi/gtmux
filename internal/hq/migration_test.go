package hq

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func migrationArchiveFixture() memoryArchive {
	return memoryArchive{encrypted: true, digest: strings.Repeat("a", 64), files: map[string]memoryFile{
		"AGENTS.md": {[]byte("OLD MANAGED RULES"), 0600}, "notes/board.md": {[]byte("OLD PANE %21"), 0600},
		"LOCAL.md": {[]byte("old personal requirements"), 0600}, "knowledge/tools/check.sh": {[]byte("#!/bin/sh\nfalse\n"), 0700},
		"knowledge/.pending-distill.jsonl": {[]byte("old pending lead"), 0600},
		"knowledge/.ledger.jsonl":          {[]byte("{\"v\":3,\"op\":\"add\",\"id\":\"pitfalls/a\",\"topic\":\"pitfalls\",\"title\":\"old lesson\",\"at\":1,\"seq\":1}\n"), 0600},
	}}
}
func TestMigrationStageExcludesBoardAndControlState(t *testing.T) {
	root := seedMemory(t)
	a := migrationArchiveFixture()
	before, _ := os.ReadFile(filepath.Join(root, "LOCAL.md"))
	if _, err := stageMigration(a, migrationOptions{}); err == nil {
		t.Fatal("empty selection allowed")
	}
	m, err := stageMigration(a, migrationOptions{Knowledge: true, Tools: true})
	if err != nil {
		t.Fatal(err)
	}
	_, staged, err := readMigration(m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(staged.files) != 2 || staged.files["knowledge/.ledger.jsonl"].data == nil {
		t.Fatalf("wrong stage: %+v", m.Files)
	}
	for _, name := range []string{"LOCAL.md", "notes/board.md", "AGENTS.md", "knowledge/.pending-distill.jsonl"} {
		if _, ok := staged.files[name]; ok {
			t.Fatalf("unselected state copied: %s", name)
		}
	}
	st, err := os.Stat(filepath.Join(m.Path, "knowledge/tools/check.sh"))
	if err != nil || st.Mode().Perm() != 0600 {
		t.Fatal("tool kept executable permissions")
	}
	after, _ := os.ReadFile(filepath.Join(root, "LOCAL.md"))
	if !bytes.Equal(before, after) {
		t.Fatal("stage changed live HQ")
	}
	list, err := listMigrations()
	if err != nil || len(list) != 1 || list[0].ID != m.ID || list[0].Path != m.Path {
		t.Fatalf("staged work cannot resume: %+v %v", list, err)
	}
}
func TestMigrationPlainAcknowledgementAndTampering(t *testing.T) {
	seedMemory(t)
	a := migrationArchiveFixture()
	a.encrypted = false
	if _, err := stageMigration(a, migrationOptions{Local: true}); err == nil {
		t.Fatal("plain archive accepted without acknowledgement")
	}
	m, err := stageMigration(a, migrationOptions{Local: true, AllowPlain: true})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(m.Path, "LOCAL.md")
	if err := os.WriteFile(path, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readMigration(m.ID); err == nil {
		t.Fatal("tampered stage was accepted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(MemoryRoot(), "LOCAL.md"), path); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readMigration(m.ID); err == nil {
		t.Fatal("symlink stage was accepted")
	}
	if _, _, err := readMigration("../../outside"); err == nil {
		t.Fatal("unsafe stage ID accepted")
	}
}
func TestMigrationPersonalRequirementsRequireFreshReviewAndBackup(t *testing.T) {
	root := seedMemory(t)
	a := migrationArchiveFixture()
	dst := filepath.Join(root, "LOCAL.md")
	current, _ := os.ReadFile(dst)
	if err := os.WriteFile(dst, []byte("changed after preview"), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := applyMigrationLocal("stage", a, contentDigest(current))
	if err == nil || r.Committed {
		t.Fatal("stale review overwrote current requirements")
	}
	current, _ = os.ReadFile(dst)
	r, err = applyMigrationLocal("stage", a, contentDigest(current))
	if err != nil || !r.Committed || r.Imported != 1 {
		t.Fatalf("replace: %+v %v", r, err)
	}
	backup, _ := os.ReadFile(r.Backup)
	if !bytes.Equal(backup, current) {
		t.Fatal("replacement lost old requirements")
	}
	now, _ := os.ReadFile(dst)
	if !bytes.Equal(now, a.files["LOCAL.md"].data) {
		t.Fatal("wrong requirements applied")
	}
	r, err = applyMigrationLocal("stage", a, contentDigest(current))
	if err != nil || r.Imported != 0 || r.Skipped != 1 || r.Backup != "" {
		t.Fatal("replacement retry is not idempotent")
	}
}
func TestMemoryArchiveValidationCompletesBeforeMovingCurrent(t *testing.T) {
	root := seedMemory(t)
	for _, bad := range []string{"unsafe", "duplicate", "symlink", "checksum", "oversize"} {
		t.Run(bad, func(t *testing.T) {
			var buf bytes.Buffer
			gz := gzip.NewWriter(&buf)
			tw := tar.NewWriter(gz)
			first := []byte("replacement")
			if err := tw.WriteHeader(&tar.Header{Name: "LOCAL.md", Size: int64(len(first)), Mode: 0600}); err != nil {
				t.Fatal(err)
			}
			tw.Write(first)
			switch bad {
			case "unsafe":
				tw.WriteHeader(&tar.Header{Name: "../../outside", Size: 1, Mode: 0600})
				tw.Write([]byte("x"))
			case "duplicate":
				tw.WriteHeader(&tar.Header{Name: "LOCAL.md", Size: 1, Mode: 0600})
				tw.Write([]byte("x"))
			case "symlink":
				tw.WriteHeader(&tar.Header{Name: "knowledge/link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"})
			case "oversize":
				tw.WriteHeader(&tar.Header{Name: "notes/large", Size: archiveFileMaxBytes + 1, Mode: 0600})
			}
			tw.Close()
			gz.Close()
			data := buf.Bytes()
			if bad == "checksum" {
				data = append([]byte{}, data...)
				data[len(data)-8] ^= 1
			}
			archive := filepath.Join(t.TempDir(), "bad.tar.gz")
			if err := os.WriteFile(archive, data, 0600); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(filepath.Join(root, "LOCAL.md"))
			if _, err := ImportMemory(archive); err == nil {
				t.Fatal("invalid archive restored")
			}
			after, _ := os.ReadFile(filepath.Join(root, "LOCAL.md"))
			if !bytes.Equal(before, after) {
				t.Fatal("late archive failure moved or replaced current records")
			}
			displaced, _ := filepath.Glob(root + ".replaced-*")
			if len(displaced) != 0 {
				t.Fatal("current home moved before full validation")
			}
		})
	}
}
func TestMigrationManifestRejectsUnselectedPaths(t *testing.T) {
	seedMemory(t)
	m, err := stageMigration(migrationArchiveFixture(), migrationOptions{Local: true})
	if err != nil {
		t.Fatal(err)
	}
	m.Files["notes/board.md"] = strings.Repeat("a", 64)
	b, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(m.Path, "manifest.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readMigration(m.ID); err == nil {
		t.Fatal("manifest widened selected scope")
	}
}

func TestMigrationRunningHQAndConcurrentWriteGuards(t *testing.T) {
	seedMemory(t)
	if err := memoryWriteAllowedFor("%fixture", func(string) (string, string) { return "codex", "" }); err == nil {
		t.Fatal("running HQ without session ID accepted")
	}
	if err := memoryWriteAllowedFor("%fixture", func(string) (string, string) { return "claude", "idle-session" }); err == nil {
		t.Fatal("idle HQ accepted")
	}
	if err := memoryWriteAllowedFor("%fixture", func(string) (string, string) { return "", "" }); err != nil {
		t.Fatal(err)
	}
	release, err := acquireMigrationApplyLock()
	if err != nil {
		t.Fatal(err)
	}
	a := migrationArchiveFixture()
	before, _ := os.ReadFile(filepath.Join(MemoryRoot(), "LOCAL.md"))
	r, err := applyMigrationLocal("stage", a, contentDigest(before))
	if err == nil || r.Committed {
		t.Fatal("concurrent apply was not refused")
	}
	if _, err := restoreMemoryArchive(a); err == nil {
		t.Fatal("concurrent restore was not refused")
	}
	after, _ := os.ReadFile(filepath.Join(MemoryRoot(), "LOCAL.md"))
	if !bytes.Equal(before, after) {
		t.Fatal("concurrent operation changed live records")
	}
	release()
	if _, err := applyMigrationLocal("stage", a, contentDigest(before)); err != nil {
		t.Fatal(err)
	}
}

func TestMigrationCLIRejectsWrongModeOptionsAndAppliesReviewedOnly(t *testing.T) {
	seedMemory(t)
	for _, args := range [][]string{
		{"--review", strings.Repeat("a", 32), "--include", "local"},
		{"--from", "/missing", "--reviewed"},
		{"--apply", strings.Repeat("a", 32), "--stage"},
	} {
		if migrationCmd(args) == 0 {
			t.Fatal("unrelated options silently accepted")
		}
	}
	m, err := stageMigration(migrationArchiveFixture(), migrationOptions{Knowledge: true})
	if err != nil {
		t.Fatal(err)
	}
	if migrationCmd([]string{"--apply", m.ID, "--entries", "pitfalls/a"}) == 0 {
		t.Fatal("unreviewed knowledge accepted")
	}
	if migrationCmd([]string{"--apply", m.ID, "--entries", "pitfalls/a", "--reviewed"}) != 0 {
		t.Fatal("reviewed import failed")
	}
	if migrationCmd([]string{"--apply", m.ID, "--entries", "pitfalls/a", "--reviewed"}) != 0 {
		t.Fatal("repeat import failed")
	}
}

func TestMemoryPlainExportIsPrivateAndDoesNotTouchSharedPart(t *testing.T) {
	seedMemory(t)
	path := filepath.Join(t.TempDir(), "hq.tar.gz")
	if err := os.WriteFile(path+".part", []byte("another exporter"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ExportMemory(path); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil || st.Mode().Perm() != 0600 {
		t.Fatal("plain private evidence exported with public permissions")
	}
	b, _ := os.ReadFile(path + ".part")
	if string(b) != "another exporter" {
		t.Fatal("fixed temporary path overwritten")
	}
}

func TestMemoryRestorePreviewDoesNotDependOnDamagedDestination(t *testing.T) {
	root := seedMemory(t)
	if err := os.MkdirAll(filepath.Join(root, "knowledge"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "knowledge/.ledger.jsonl"), []byte("{broken}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "LOCAL.md"), []byte{0xff}, 0600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(root, "knowledge/.ledger.jsonl"))
	p, err := previewRestore(migrationArchiveFixture())
	if err != nil || len(p.Entries) != 1 || !p.HasLocal {
		t.Fatalf("backup recovery preview blocked: %+v %v", p, err)
	}
	after, _ := os.ReadFile(filepath.Join(root, "knowledge/.ledger.jsonl"))
	if !bytes.Equal(before, after) {
		t.Fatal("restore preview changed damaged target")
	}
}
