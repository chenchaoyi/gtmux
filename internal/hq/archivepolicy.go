package hq

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type archiveEntry struct {
	path, name string
	info       os.FileInfo
}

// Automatic recovery protects instructions, knowledge, workspace and notes. Raw
// report payloads and attachments have an explicit process scope; unknown notes
// outside that scope remain protected. Include overrides preserve unique files.
func automaticFile(name string, include []string) bool {
	for _, p := range include {
		if name == p || strings.HasPrefix(name, p+"/") {
			return true
		}
	}
	if !strings.HasPrefix(name, "notes/") {
		return true
	}
	base := filepath.Base(name)
	ext := strings.ToLower(filepath.Ext(name))
	if ext == ".md" || ext == ".txt" {
		return true
	}
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".mp4", ".mov", ".pdf", ".zip", ".gz", ".bundle", ".ipa", ".log":
		return false
	}
	if ext == ".json" && (base == "relays.json" || strings.HasSuffix(base, "-relays.json") || strings.HasPrefix(base, "relays-")) {
		return false
	}
	if strings.HasPrefix(name, "notes/reports/") {
		switch strings.ToLower(base) {
		case "summary.json", "final.json", "index.json", "owner.json", "retained.json", "retention.json":
			return true
		}
		return false
	}
	return true
}

func selectArchive(root string, core bool, include []string) ([]archiveEntry, error) {
	var entries []archiveEntry
	var total int64
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	err = filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		if name == "." || strings.HasPrefix(name, "workspace/.") || fi.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if fi.IsDir() {
			return nil
		} // restore recreates parents; empty dirs carry no records
		if core && !automaticFile(name, include) {
			return nil
		}
		if !fi.Mode().IsRegular() {
			return fmt.Errorf("unsupported HQ file type: %s", name)
		}
		if fi.Size() > archiveFileMaxBytes {
			return fmt.Errorf("HQ file exceeds restore limit (32 MiB): %s", name)
		}
		total += fi.Size() + 1024 // tar headers/padding are additionally bounded while writing
		if total > archiveMaxBytes || len(entries) >= archiveMaxEntries {
			return errors.New("HQ records exceed restore limits (128 MiB, 5,000 entries); destination unchanged")
		}
		entries = append(entries, archiveEntry{p, name, fi})
		return nil
	})
	return entries, err
}

type archiveLimitWriter struct {
	dst       io.Writer
	remaining int64
}

func (w *archiveLimitWriter) Write(b []byte) (int, error) {
	if int64(len(b)) > w.remaining {
		return 0, errors.New("HQ archive exceeds the selected packing limit")
	}
	n, e := w.dst.Write(b)
	w.remaining -= int64(n)
	return n, e
}

func writeSelectedArchive(w io.Writer, root string, core bool, include []string) error {
	return writeSelectedArchiveLimit(w, root, core, include, archiveMaxBytes)
}

func writeSelectedArchiveLimit(w io.Writer, root string, core bool, include []string, compressedLimit int64) error {
	entries, err := selectArchive(root, core, include)
	if err != nil {
		return err
	}
	gz := gzip.NewWriter(&archiveLimitWriter{w, compressedLimit})
	tw := tar.NewWriter(&archiveLimitWriter{gz, archiveMaxBytes})
	for _, e := range entries {
		h, err := tar.FileInfoHeader(e.info, "")
		if err != nil {
			return err
		}
		h.Name = e.name
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		if err := copyArchiveEntry(tw, e); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

func exportOutsideHQ(dst string) error {
	root, err := filepath.EvalSymlinks(MemoryRoot())
	if err != nil {
		return err
	}
	parent, err := filepath.Abs(filepath.Dir(dst))
	if err != nil {
		return err
	}
	if real, e := filepath.EvalSymlinks(parent); e == nil {
		parent = real
	}
	rel, err := filepath.Rel(root, parent)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("export destination must be outside HQ records")
	}
	return nil
}

func copyArchiveEntry(w io.Writer, e archiveEntry) error {
	f, err := os.Open(e.path)
	if err != nil {
		return err
	}
	actual, err := f.Stat()
	if err != nil || !actual.Mode().IsRegular() || !os.SameFile(e.info, actual) {
		_ = f.Close()
		return fmt.Errorf("HQ source changed during export: %s", e.name)
	}
	_, err = io.CopyN(w, f, e.info.Size())
	after, statErr := f.Stat()
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if statErr != nil {
		return statErr
	}
	if closeErr != nil {
		return closeErr
	}
	if after.Size() != e.info.Size() || !after.ModTime().Equal(e.info.ModTime()) || after.Mode() != e.info.Mode() {
		return fmt.Errorf("HQ source changed during export: %s", e.name)
	}
	return nil
}
func selectedContentDigest(root string, include []string) (string, error) {
	entries, err := selectArchive(root, true, include)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	for _, e := range entries {
		writeMemoryDigestHeader(h, e.name, e.info.Size(), e.info.Mode())
		if err := copyArchiveEntry(h, e); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
