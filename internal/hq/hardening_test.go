package hq

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestCHH008_BoardUTF8TruncationAndOversizeBudget(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := state.MkdirAll(filepath.Dir(BoardPath()), 0o700); err != nil {
		t.Fatal(err)
	}

	// Place a 3-byte Chinese rune ("中" = E4 B8 AD) straddling boardMaxBytes so raw
	// slicing at boardMaxBytes would leave 1 or 2 incomplete UTF-8 lead/continuation bytes.
	prefix := bytes.Repeat([]byte("a"), boardMaxBytes-1)
	raw := append(prefix, []byte("中文尾部内容")...)
	if err := state.WritePrivateFile(BoardPath(), raw); err != nil {
		t.Fatal(err)
	}
	text, _, ok := Board()
	if !ok {
		t.Fatal("Board() returned ok=false")
	}
	if !utf8.ValidString(text) || strings.ContainsRune(text, utf8.RuneError) {
		t.Fatalf("Board() produced invalid UTF-8 at truncation boundary (len=%d)", len(text))
	}
	if len(text) != boardMaxBytes-1 {
		t.Fatalf("len(Board()) = %d, want %d after trimming split rune", len(text), boardMaxBytes-1)
	}

	if bCount, lCount, over := OversizeBoard(); !over || bCount <= BoardByteBudget || lCount < 1 {
		t.Fatalf("OversizeBoard() = (%d, %d, %v), want over=true", bCount, lCount, over)
	}

	// Line-count budget (>250 short lines, <32 KiB) must also report over=true.
	shortLines := strings.Repeat("- handoff item\n", BoardLineBudget+5)
	if err := state.WritePrivateFile(BoardPath(), []byte(shortLines)); err != nil {
		t.Fatal(err)
	}
	if _, lCount, over := OversizeBoard(); !over || lCount != BoardLineBudget+5 {
		t.Fatalf("OversizeBoard() on >250 lines = (_, %d, %v), want (%d, true)", lCount, over, BoardLineBudget+5)
	}
}

func TestCHH008_ManagedSnapshotPointsSkipsArchiveDecompression(t *testing.T) {
	root := seedMemory(t)
	putRecord(t, root, "notes/board.md", []byte("# current board\n"))
	p, wrote, err := Snapshot(snapshotTime())
	if err != nil || !wrote {
		t.Fatalf("Snapshot: wrote=%v err=%v", wrote, err)
	}

	// Make the .tar.gz unreadable (0000) while keeping Lstat size + mtime intact so
	// listedPoint proves managedSnapshotPoints validates receipt + Lstat without reading tar.gz bytes.
	if err := os.Chmod(p, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(p, 0o600) })

	pts := managedSnapshotPoints()
	if len(pts) != 1 || pts[0].path != p {
		t.Fatalf("managedSnapshotPoints = %+v, want [%s] without opening tar.gz", pts, p)
	}
}

func TestCHH008_PruneDeadCodexFrontendSocketsAndMineErrorBackoff(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := state.RootStorePath("codex-frontends")
	deadDir := filepath.Join(root, "client-dead")
	liveDir := filepath.Join(root, "client-live")
	for _, d := range []string{deadDir, liveDir} {
		if err := state.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}

	evalTmp, err := filepath.EvalSymlinks("/tmp")
	if err != nil {
		t.Fatal(err)
	}
	daemonDir := filepath.Join(evalTmp, "codex-daemon-"+strconv.Itoa(os.Geteuid()))
	if err := os.MkdirAll(daemonDir, 0o700); err != nil {
		t.Fatal(err)
	}
	liveSockTarget := filepath.Join(daemonDir, fmt.Sprintf("gtmux-test-live-%d.sock", os.Getpid()))
	_ = os.Remove(liveSockTarget)
	ln, err := net.Listen("unix", liveSockTarget)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = ln.Close()
		_ = os.Remove(liveSockTarget)
	})

	deadSockTarget := filepath.Join(daemonDir, fmt.Sprintf("gtmux-test-dead-%d.sock", os.Getpid()))
	_ = os.Remove(deadSockTarget)
	if err := os.Symlink(deadSockTarget, filepath.Join(deadDir, "frontend.sock")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(liveSockTarget, filepath.Join(liveDir, "frontend.sock")); err != nil {
		t.Fatal(err)
	}

	old := time.Now().Add(-5 * time.Minute)
	_ = os.Chtimes(deadDir, old, old)
	_ = os.Chtimes(liveDir, old, old)

	pruneCodexFrontendSockets(time.Now())
	if _, err := os.Lstat(deadDir); !os.IsNotExist(err) {
		t.Fatalf("dead client-* socket dir was not reaped: %v", err)
	}
	if _, err := os.Lstat(liveDir); err != nil {
		t.Fatalf("live client-* socket dir was reaped: %v", err)
	}

	// Verify mineSensor records failure backoff when ledger JSON is corrupt, avoiding repeated scans.
	if err := state.MkdirAll(hqKnowledgeDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := state.WritePrivateFile(filepath.Join(mineDir(), "sources.json"), []byte("{corrupt")); err != nil {
		t.Fatal(err)
	}
	events.Append(events.Record{Event: events.AuditEventSend, Summary: "synthetic send head"})
	nowUnix := time.Now().Unix()
	mineSensor(nowUnix)
	attempt := state.ReadInt64Marker(mineAttemptPath())
	if attempt != nowUnix {
		t.Fatalf("mineAttemptPath = %d, want %d after failed pass", attempt, nowUnix)
	}
	// Make events.jsonl unreadable: a second call within 1h must return at the backoff marker.
	_ = os.Chmod(events.Path(), 0o000)
	t.Cleanup(func() { _ = os.Chmod(events.Path(), 0o600) })
	mineSensor(nowUnix + 20)
	if got := state.ReadInt64Marker(mineAttemptPath()); got != nowUnix {
		t.Fatalf("backoff marker overwritten during cooldown: %d != %d", got, nowUnix)
	}
}
