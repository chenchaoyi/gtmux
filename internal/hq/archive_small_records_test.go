package hq

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestAutomaticGeneratedRelayVariants(t *testing.T) {
	for _, name := range []string{
		"notes/done-abcdef12-relays-final.json",
		"notes/tick-abcdef12-relays-verified.json",
		"notes/rotate-abcd1234-relays-after.json",
		"notes/asks-abcdef01-relays-current.json",
		"notes/demo-followup-relays-verified.json",
	} {
		if automaticFile(name, nil) {
			t.Errorf("generated bulk observation was included: %s", name)
		}
		if !automaticFile(name, []string{name}) {
			t.Errorf("explicit unique-record override was ignored: %s", name)
		}
	}
	for _, name := range []string{
		"notes/relay-plan.json", "notes/custom-relays-summary.json",
		"notes/project-abcdef12-relays-analysis.json",
		"notes/done-abcdef12-relays-final.md", "knowledge/relays-final.json",
	} {
		if !automaticFile(name, nil) {
			t.Errorf("ordinary structured note or conclusion was dropped: %s", name)
		}
	}
}

func TestAutomaticManySmallNotesRoundTrip(t *testing.T) {
	root := seedMemory(t)
	const count = 6750
	for i := 0; i < count; i++ {
		putRecord(t, root, fmt.Sprintf("notes/unique/%05d.json", i), []byte(fmt.Sprintf(`{"record":%d}`, i)))
	}
	putRecord(t, root, "notes/done-abcdef12-relays-final.json", []byte(`{"raw":"process observation"}`))
	p, wrote, err := Snapshot(snapshotTime())
	if err != nil || !wrote {
		t.Fatalf("many small records failed: wrote=%v err=%v", wrote, err)
	}
	a, err := readMemoryArchive(p, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := a.files["notes/done-abcdef12-relays-final.json"]; ok {
		t.Fatal("bulk relay observation was retained in the core point")
	}
	moved, err := ImportMemory(p)
	if err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(moved, "notes/done-abcdef12-relays-final.json")); err != nil || string(b) != `{"raw":"process observation"}` {
		t.Fatalf("core restore lost the displaced original process material: %v", err)
	}
	for i := 0; i < count; i++ {
		name := filepath.Join(root, "notes/unique", fmt.Sprintf("%05d.json", i))
		b, err := os.ReadFile(name)
		if err != nil || string(b) != fmt.Sprintf(`{"record":%d}`, i) {
			t.Fatalf("unique structured record %d did not round trip: %v", i, err)
		}
	}
}

func TestSmallRecordImportEntryBoundary(t *testing.T) {
	root := seedMemory(t)
	valid := smallRecordArchive(t, 10000)
	a, err := readMemoryArchive(valid, "")
	if err != nil || len(a.files) != 10000 {
		t.Fatalf("the inclusive entry boundary failed: %d %v", len(a.files), err)
	}
	invalid := smallRecordArchive(t, 10001)
	if _, err := ImportMemory(invalid); err == nil {
		t.Fatal("entry overflow was published")
	}
	if b, err := os.ReadFile(filepath.Join(root, "LOCAL.md")); err != nil || string(b) != "the operator's own charter" {
		t.Fatalf("failed entry validation changed the current home: %v", err)
	}
}

func smallRecordArchive(b testing.TB, count int) string {
	b.Helper()
	p := filepath.Join(b.TempDir(), "records.tar.gz")
	f, err := os.Create(p)
	if err != nil {
		b.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	body := []byte(`{"unique":true}`)
	for i := 0; i < count; i++ {
		if err := tw.WriteHeader(&tar.Header{Name: fmt.Sprintf("notes/%05d.json", i), Mode: 0600, Size: int64(len(body))}); err != nil {
			b.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			b.Fatal(err)
		}
	}
	for _, close := range []func() error{tw.Close, gz.Close, f.Close} {
		if err := close(); err != nil {
			b.Fatal(err)
		}
	}
	return p
}

func benchmarkSmallRecords(b *testing.B, count int) {
	p := smallRecordArchive(b, count)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a, err := readMemoryArchive(p, "")
		if err != nil || len(a.files) != count {
			b.Fatalf("read %d records: %d %v", count, len(a.files), err)
		}
	}
}

func BenchmarkSmallRecordArchive4800(b *testing.B)  { benchmarkSmallRecords(b, 4800) }
func BenchmarkSmallRecordArchive7000(b *testing.B)  { benchmarkSmallRecords(b, 7000) }
func BenchmarkSmallRecordArchive10000(b *testing.B) { benchmarkSmallRecords(b, 10000) }
