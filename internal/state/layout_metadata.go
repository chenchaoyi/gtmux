package state

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

type layoutJournalProof struct {
	file layoutEntry
	hash [sha256.Size]byte
}

func layoutRegularAt(parent int, name string) (layoutEntry, error) {
	var stat unix.Stat_t
	if err := unix.Fstatat(parent, name, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return layoutEntry{}, err
	}
	if int(stat.Uid) != os.Geteuid() || stat.Mode&unix.S_IFMT != unix.S_IFREG ||
		stat.Mode&0o077 != 0 || stat.Nlink != 1 {
		return layoutEntry{}, errors.New("storage layout journal must be an owned private file")
	}
	return layoutEntry{Dev: uint64(stat.Dev), Inode: uint64(stat.Ino)}, nil
}

// Bind the running plan to the staged file before its no-replace publication.
func publishLayoutTransaction(t *layoutTransaction, data []byte) error {
	return writePrivateFile(filepath.Join(Dir(), layoutTransition), data, func(parent int, source string, to int, name string) error {
		file, err := layoutRegularAt(parent, source)
		if err != nil {
			return err
		}
		if err := renameLayoutEntry(parent, source, to, name); err != nil {
			return err
		}
		t.proof = layoutJournalProof{file: file, hash: sha256.Sum256(data)}
		return nil
	})
}

func verifyLayoutJournal(t *layoutTransaction) error {
	current, err := readLayoutTransaction()
	if err != nil {
		return fmt.Errorf("storage layout journal requires review: %w", err)
	}
	if current == nil || current.proof != t.proof {
		return errors.New("storage layout journal identity or contents changed")
	}
	return nil
}

func retireLayoutJournal(t *layoutTransaction) error {
	parent, err := privateDirectory(Dir(), false)
	if err != nil {
		return err
	}
	defer unix.Close(parent)
	if err := unix.Fsync(parent); err != nil {
		return err
	}
	if err := verifyLayoutJournal(t); err != nil {
		return err
	}
	if current, err := layoutRegularAt(parent, layoutTransition); err != nil || current != t.proof.file {
		return errors.New("storage layout journal identity changed before retirement")
	}
	if err := unix.Unlinkat(parent, layoutTransition, 0); err != nil {
		return err
	}
	return unix.Fsync(parent)
}

type layoutScaffold struct {
	Root   layoutEntry
	Groups []layoutEntry
}

func readLayoutScaffold(path string) (*layoutScaffold, error) {
	fd, err := privateDirectory(path, false)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer unix.Close(fd)
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return nil, err
	}
	root, err := layoutDirectoryIdentity(stat)
	if err != nil {
		return nil, err
	}
	s := &layoutScaffold{Root: root}
	for _, group := range layoutGroups() {
		id, err := layoutDirectoryAt(fd, group)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("storage group requires review: %s: %w", group, err)
		}
		id.Name = group
		s.Groups = append(s.Groups, id)
	}
	return s, nil
}

func layoutDirectoryRecord(entry layoutEntry) bool {
	return entry.Inode != 0 && entry.Mode == uint32(os.ModeDir)
}

func (s *layoutScaffold) valid() bool {
	if s == nil || s.Root.Name != "" || !layoutDirectoryRecord(s.Root) {
		return false
	}
	groups := layoutGroups()
	if len(s.Groups) != len(groups) {
		return false
	}
	for i, group := range groups {
		if s.Groups[i].Name != group || !layoutDirectoryRecord(s.Groups[i]) {
			return false
		}
	}
	return true
}

func (s *layoutScaffold) verify(path string, complete bool) error {
	actual, err := readLayoutScaffold(path)
	if err != nil {
		return err
	}
	if actual == nil {
		if !complete {
			return nil
		}
		return errors.New("recorded storage scaffold is missing")
	}
	if actual.Root != s.Root || complete && len(actual.Groups) != len(s.Groups) {
		return errors.New("recorded storage scaffold identity changed")
	}
	expected := make(map[string]layoutEntry, len(s.Groups))
	for _, group := range s.Groups {
		expected[group.Name] = group
	}
	for _, group := range actual.Groups {
		if group != expected[group.Name] {
			return fmt.Errorf("recorded storage group identity changed: %s", group.Name)
		}
	}
	return nil
}
