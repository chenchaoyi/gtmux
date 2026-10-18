package dsh

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
)

func artifact(t *testing.T, workspace, id, suffix string, parts ...string) string {
	t.Helper()
	dir := filepath.Join(Home(), "sessions", workspace, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "session.v4.jsonl"+suffix)
	var b []byte
	enc, err := zstd.NewWriter(nil, zstd.WithEncoderCRC(true))
	if err != nil {
		t.Fatal(err)
	}
	defer enc.Close()
	for _, s := range parts {
		if suffix == ".zstd" {
			b = append(b, enc.EncodeAll([]byte(s), nil)...)
		} else {
			b = append(b, s...)
		}
	}
	if err := os.WriteFile(file, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return file
}
func head(id string) string {
	b, _ := json.Marshal(Header{Type: "session", Version: 4, ID: id, Cwd: "/project", CreatedAt: 1000})
	return string(b) + "\n"
}

const promptEvent = `{"type":"user/message","seq":1,"time":2000,"data":{"role":"user","source":{"kind":"user"},"content":[{"type":"text","text":"public prompt"}]}}` + "\n"

func TestDiscoveryReadsEveryFrameAndKeepsHomesSeparate(t *testing.T) {
	t.Setenv("DSH_HOME", t.TempDir())
	artifact(t, "project", "one", ".zstd", head("one"), promptEvent, `{"type":"session/title","seq":2,"time":3000,"data":{"title":"Read this conversation"}}`+"\n")
	catalog := &Catalog{}
	rows, err := catalog.List()
	if err != nil || len(rows) != 1 || rows[0].Title != "Read this conversation" {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
	first := rows[0].HomeID
	t.Setenv("DSH_HOME", t.TempDir())
	artifact(t, "project", "one", "", head("one"), promptEvent)
	rows, err = catalog.List()
	if err != nil || len(rows) != 1 || rows[0].HomeID == first || rows[0].Title != "" {
		t.Fatalf("home alias: %+v %v", rows, err)
	}
}
func TestScanOnlyConsumesAppendsAndWaitsForCompleteFrames(t *testing.T) {
	t.Setenv("DSH_HOME", t.TempDir())
	file := artifact(t, "project", "one", ".zstd", head("one"), promptEvent)
	var c Cursor
	count := 0
	reset := func() { count = 0 }
	fold := func([]byte) error { count++; return nil }
	if err := Scan(file, &c, reset, fold); err != nil || count != 2 {
		t.Fatalf("first=%d %v", count, err)
	}
	if err := Scan(file, &c, reset, fold); err != nil || count != 2 {
		t.Fatalf("unchanged re-decoded=%d %v", count, err)
	}
	enc, _ := zstd.NewWriter(nil, zstd.WithEncoderCRC(true))
	defer enc.Close()
	frame := enc.EncodeAll([]byte(promptEvent), nil)
	f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, _ = f.Write(frame[:len(frame)/2])
	before := c.Offset
	if err := Scan(file, &c, reset, fold); err != nil || count != 2 || c.Offset != before {
		t.Fatalf("accepted torn frame: %d %d %v", count, c.Offset, err)
	}
	_, _ = f.Write(frame[len(frame)/2:])
	if err := Scan(file, &c, reset, fold); err != nil || count != 3 {
		t.Fatalf("completed frame=%d %v", count, err)
	}
}
func TestCorruptionUnknownGenerationAndIdentityCannotBecomeEmptyHistory(t *testing.T) {
	for _, mode := range []string{"checksum", "generation", "identity", "duplicate"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("DSH_HOME", t.TempDir())
			h := head("one")
			if mode == "identity" {
				h = head("another")
			}
			file := artifact(t, "project", "one", ".zstd", h, promptEvent)
			if mode == "checksum" {
				b, _ := os.ReadFile(file)
				b[len(b)-1] ^= 1
				_ = os.WriteFile(file, b, 0o600)
			}
			if mode == "generation" {
				_ = os.Rename(file, strings.Replace(file, "v4", "v5", 1))
			}
			if mode == "duplicate" {
				artifact(t, "another-project", "one", "", head("one"), promptEvent)
			}
			c := &Catalog{}
			rows, err := c.List()
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 0 {
				t.Fatalf("unverified rows=%+v", rows)
			}
			if _, err := c.Resolve("one"); err == nil {
				t.Fatal("unverified exact read accepted")
			}
		})
	}
}
func TestInjectedAndSubagentPromptsDoNotCreateUserRows(t *testing.T) {
	t.Setenv("DSH_HOME", t.TempDir())
	artifact(t, "project", "injection", "", head("injection"), strings.ReplaceAll(promptEvent, `"kind":"user"`, `"kind":"injection"`))
	var header Header
	_ = json.Unmarshal([]byte(head("child")), &header)
	header.Origin = "subagent"
	encoded, _ := json.Marshal(header)
	h := string(encoded) + "\n"
	artifact(t, "project", "child", "", h, promptEvent)
	rows, err := (&Catalog{}).List()
	if err != nil || len(rows) != 0 {
		t.Fatalf("rows=%+v %v", rows, err)
	}
}

func TestExactReadIncludesOldSessionsAndRejectsEncodingCopies(t *testing.T) {
	t.Setenv("DSH_HOME", t.TempDir())
	p := artifact(t, "project", "old", "", head("old"), promptEvent)
	old := time.Unix(1, 0)
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 130; i++ {
		id := fmt.Sprintf("new-%d", i)
		artifact(t, "project", id, "", head(id), promptEvent)
	}
	c := &Catalog{}
	xs, err := c.List()
	if err != nil || len(xs) != 128 {
		t.Fatal(len(xs), err)
	}
	if s, err := c.Resolve("old"); err != nil || s.ID != "old" {
		t.Fatal("old exact read unavailable", err)
	}
	artifact(t, "project", "old", ".zstd", head("old"), promptEvent)
	if _, err := c.Resolve("old"); err != ErrAmbiguous {
		t.Fatal("mixed encoding silently selected", err)
	}
}
