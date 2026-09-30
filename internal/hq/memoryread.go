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
	"path"
	"sort"
	"strings"

	"filippo.io/age"
)

const archiveMaxBytes = 128 << 20
const archiveFileMaxBytes = 32 << 20
const archiveMaxEntries = 5000

type memoryFile struct {
	data []byte
	mode os.FileMode
}
type memoryArchive struct {
	files     map[string]memoryFile
	encrypted bool
	digest    string
}

// readMemoryArchive validates the entire stream before anything can reach the live home.
// The in-memory bound also bounds private staging and decompression on a small disk.
func readMemoryArchive(src, pass string) (memoryArchive, error) {
	a := memoryArchive{files: map[string]memoryFile{}, encrypted: IsEncryptedArchive(src)}
	f, err := os.Open(src)
	if err != nil {
		return a, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return a, err
	}
	if !st.Mode().IsRegular() || st.Size() > archiveMaxBytes {
		return a, errors.New("archive is not a regular file within the 128 MiB limit")
	}
	var input io.Reader = f
	if a.encrypted {
		identity, err := age.NewScryptIdentity(pass)
		if err != nil {
			return a, err
		}
		identity.SetMaxWorkFactor(18)
		input, err = age.Decrypt(f, identity)
		if err != nil {
			var mismatch *age.NoIdentityMatchError
			if errors.As(err, &mismatch) {
				return a, ErrWrongPassphrase
			}
			return a, err
		}
	}
	gz, err := gzip.NewReader(io.LimitReader(input, archiveMaxBytes+1))
	if err != nil {
		return a, err
	}
	defer gz.Close()
	bounded := &io.LimitedReader{R: gz, N: archiveMaxBytes + 1}
	tr := tar.NewReader(bounded)
	seen := map[string]bool{}
	recognized := false
	count := 0
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return a, err
		}
		count++
		if count > archiveMaxEntries {
			return a, errors.New("archive exceeds 5,000 entries")
		}
		if err := safeName(h.Name); err != nil {
			return a, err
		}
		name := path.Clean(h.Name)
		if seen[name] {
			return a, fmt.Errorf("duplicate archive path: %s", name)
		}
		seen[name] = true
		if h.Typeflag != tar.TypeDir && h.Typeflag != tar.TypeReg {
			return a, fmt.Errorf("unsupported archive entry type: %s", name)
		}
		if h.Size < 0 || h.Size > archiveFileMaxBytes {
			return a, fmt.Errorf("archive file exceeds 32 MiB: %s", name)
		}
		if h.Typeflag == tar.TypeDir {
			continue
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			return a, err
		}
		if int64(len(b)) != h.Size {
			return a, io.ErrUnexpectedEOF
		}
		a.files[name] = memoryFile{b, os.FileMode(h.Mode) & 0700}
		if name == "AGENTS.md" || name == "CLAUDE.md" || name == "LOCAL.md" || strings.HasPrefix(name, "notes/") || strings.HasPrefix(name, "knowledge/") {
			recognized = true
		}
	}
	// Tar EOF alone does not verify the gzip footer or the last authenticated age chunk.
	if _, err := io.Copy(io.Discard, bounded); err != nil {
		return a, err
	}
	if bounded.N <= 0 {
		return a, errors.New("archive exceeds the 128 MiB decompression limit")
	}
	if !recognized {
		return a, errors.New("archive carries no HQ records")
	}
	for name := range a.files {
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if _, ok := a.files[parent]; ok {
				return a, fmt.Errorf("archive path is both a file and a directory: %s", parent)
			}
		}
	}
	names := make([]string, 0, len(a.files))
	for name := range a.files {
		names = append(names, name)
	}
	sort.Strings(names)
	hash := sha256.New()
	for _, name := range names {
		fmt.Fprintf(hash, "%d:%s:%d:%o:", len(name), name, len(a.files[name].data), a.files[name].mode)
		hash.Write(a.files[name].data)
	}
	a.digest = hex.EncodeToString(hash.Sum(nil))
	return a, nil
}
