package codexbridge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/chenchaoyi/gtmux/internal/state"
)

const trustMetadataLimit = 64 << 10

type worktreeTrustProof struct {
	Cwd, Checkout, MainRoot, OriginalMain string
	Fingerprint                           string
}

// This is filesystem evidence for the existing backend trust decision, never a
// trust grant. Unsupported layouts retain Codex's original confirmation flow.
func readWorktreeTrust(ctx context.Context, cwd string) (worktreeTrustProof, bool) {
	var zero worktreeTrustProof
	if ctx.Err() != nil || !filepath.IsAbs(cwd) || strings.ContainsAny(cwd, "\x00\r\n") {
		return zero, false
	}
	canonical, err := filepath.EvalSymlinks(filepath.Clean(cwd))
	if err != nil {
		return zero, false
	}
	cwdSeal, ok := ownedTrustDirectory(canonical)
	if !ok {
		return zero, false
	}
	checkout := canonical
	for depth := 0; depth < 64; depth++ {
		if ctx.Err() != nil {
			return zero, false
		}
		_, err = os.Lstat(filepath.Join(checkout, ".git"))
		if err == nil {
			break
		}
		parent := filepath.Dir(checkout)
		if !os.IsNotExist(err) || parent == checkout {
			return zero, false
		}
		checkout = parent
		if depth == 63 {
			return zero, false
		}
	}
	pointer, pointerSeal, ok := readTrustMetadata(filepath.Join(checkout, ".git"))
	if !ok || !strings.HasPrefix(pointer, "gitdir:") {
		return zero, false
	}
	rawGitDir := strings.TrimSpace(strings.TrimPrefix(pointer, "gitdir:"))
	if rawGitDir == "" {
		return zero, false
	}
	if !filepath.IsAbs(rawGitDir) {
		rawGitDir = filepath.Join(checkout, rawGitDir)
	}
	rawGitDir = filepath.Clean(rawGitDir)
	gitDir, err := filepath.EvalSymlinks(rawGitDir)
	if err != nil {
		return zero, false
	}
	gitSeal, ok := ownedTrustDirectory(rawGitDir)
	if !ok {
		return zero, false
	}
	worktrees := filepath.Dir(gitDir)
	if filepath.Base(worktrees) != "worktrees" {
		return zero, false
	}
	common := filepath.Dir(worktrees)
	if filepath.Base(common) != ".git" {
		return zero, false
	}
	commonSeal, ok := ownedTrustDirectory(common)
	if !ok {
		return zero, false
	}
	backlink, backlinkSeal, ok := readTrustMetadata(filepath.Join(gitDir, "gitdir"))
	if !ok {
		return zero, false
	}
	if !filepath.IsAbs(backlink) {
		backlink = filepath.Join(gitDir, backlink)
	}
	if filepath.Base(backlink) != ".git" {
		return zero, false
	}
	registered, err := filepath.EvalSymlinks(filepath.Dir(backlink))
	if err != nil || registered != checkout {
		return zero, false
	}
	commonLink, linkSeal, ok := readTrustMetadata(filepath.Join(gitDir, "commondir"))
	if !ok {
		return zero, false
	}
	if !filepath.IsAbs(commonLink) {
		commonLink = filepath.Join(gitDir, commonLink)
	}
	linked, err := filepath.EvalSymlinks(commonLink)
	if err != nil || linked != common {
		return zero, false
	}
	main := filepath.Dir(common)
	mainSeal, ok := ownedTrustDirectory(filepath.Join(main, ".git"))
	if !ok {
		return zero, false
	}
	actual, err := filepath.EvalSymlinks(filepath.Join(main, ".git"))
	if err != nil || actual != common {
		return zero, false
	}
	originalMain := filepath.Dir(filepath.Dir(filepath.Dir(rawGitDir)))
	originalCommon, err := filepath.EvalSymlinks(filepath.Join(originalMain, ".git"))
	if err != nil || originalCommon != common {
		return zero, false
	}
	if ctx.Err() != nil {
		return zero, false
	}
	encoded, _ := json.Marshal([]string{canonical, checkout, main, originalMain, cwdSeal, pointerSeal, gitSeal, commonSeal, backlinkSeal, linkSeal, mainSeal})
	sum := sha256.Sum256(encoded)
	return worktreeTrustProof{canonical, checkout, main, originalMain, hex.EncodeToString(sum[:])}, true
}

func ownedTrustDirectory(path string) (string, bool) {
	i, e := os.Lstat(path)
	if e != nil || !i.IsDir() || i.Mode()&os.ModeSymlink != 0 {
		return "", false
	}
	s, ok := i.Sys().(*syscall.Stat_t)
	if !ok || s.Uid != uint32(os.Geteuid()) {
		return "", false
	}
	b, _ := json.Marshal([]any{s.Dev, s.Ino})
	return string(b), true
}

func readTrustMetadata(path string) (string, string, bool) {
	f, e := state.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if e != nil {
		return "", "", false
	}
	defer f.Close()
	i, e := f.Stat()
	if e != nil || !i.Mode().IsRegular() || i.Size() <= 0 || i.Size() > trustMetadataLimit {
		return "", "", false
	}
	s, ok := i.Sys().(*syscall.Stat_t)
	if !ok || s.Uid != uint32(os.Geteuid()) {
		return "", "", false
	}
	b, e := io.ReadAll(io.LimitReader(f, trustMetadataLimit+1))
	if e != nil || len(b) > trustMetadataLimit {
		return "", "", false
	}
	at, e := os.Lstat(path)
	if e != nil || !os.SameFile(i, at) || i.Size() != at.Size() || !i.ModTime().Equal(at.ModTime()) || at.Mode()&os.ModeSymlink != 0 {
		return "", "", false
	}
	value := strings.TrimSpace(string(b))
	if value == "" || strings.ContainsAny(value, "\x00\r\n") {
		return "", "", false
	}
	sum := sha256.Sum256(b)
	seal, _ := json.Marshal([]any{s.Dev, s.Ino, i.ModTime().UnixNano(), hex.EncodeToString(sum[:])})
	return value, string(seal), true
}
