package events

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRetainedSeqIncrementalAndGenerations(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(Path()), 0700); err != nil {
		t.Fatal(err)
	}
	x := &retainedSeqIndex{}
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(Path(), "{\"seq\":3}\n{\"seq\":2}\n")
	if x.latest() != 3 {
		t.Fatal("retained maximum changed to append order")
	}
	parsed := x.parsed
	if x.latest() != 3 || x.parsed != parsed {
		t.Fatal("unchanged poll decoded history")
	}
	f, err := os.OpenFile(Path(), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString("{\"seq\":4")
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if x.latest() != 3 {
		t.Fatal("partial record became a committed sequence")
	}
	f, err = os.OpenFile(Path(), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString("}\n")
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if x.latest() != 4 {
		t.Fatal("completed append was lost")
	}
	if err := os.Rename(Path(), rotatedPath()); err != nil {
		t.Fatal(err)
	}
	if x.latest() != 4 {
		t.Fatal("rotation gap lost retained generation")
	}
	write(Path(), "{\"seq\":5}\n")
	if x.latest() != 5 {
		t.Fatal("new active generation missing")
	}
	if err := os.Remove(rotatedPath()); err != nil {
		t.Fatal(err)
	}
	write(Path(), "{\"seq\":1}\n")
	if x.latest() != 1 {
		t.Fatal("same-size rewrite retained an old maximum")
	}
	write(Path(), "")
	if x.latest() != 0 {
		t.Fatal("truncation retained an old maximum")
	}
}

func TestRetainedSeqDoesNotUseAssignedCounter(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(Path()), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(), []byte("{\"seq\":7}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(seqPath(), []byte("99"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := LatestSeq(); got != 7 {
		t.Fatalf("retained=%d, assigned=%d", got, CurrentSeq())
	}
	f, err := os.OpenFile(Path(), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString("junk\n{\"seq\":3}\n")
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if got := LatestSeq(); got != 3 {
		t.Fatalf("invalidated EOF record stayed retained: %d", got)
	}
}
