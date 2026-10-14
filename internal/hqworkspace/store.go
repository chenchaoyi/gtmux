package hqworkspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// Store is local to one HQ home. Tests always supply an isolated directory.
// Atomic snapshots preserve a complete previous version on pre-rename failures.
type Store struct{ dir string }

func New(home string) *Store { return &Store{dir: filepath.Join(home, "workspace")} }

func (s *Store) Read() (Database, error) {
	f, err := os.Open(filepath.Join(s.dir, "records.json"))
	if errors.Is(err, os.ErrNotExist) {
		return Empty(), nil
	}
	if err != nil {
		return Database{}, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 32<<20+1))
	if err != nil {
		return Database{}, err
	}
	if len(b) > 32<<20 {
		return Database{}, errors.New("workspace snapshot exceeds 32MiB; keep an archive before reorganizing records")
	}
	var d Database
	if err := json.Unmarshal(b, &d); err != nil {
		return Database{}, fmt.Errorf("workspace records: %w", err)
	}
	if d.Schema != Schema || d.Matters == nil || d.Messages == nil {
		return Database{}, errors.New("unsupported or incomplete workspace schema")
	}
	if d.Changes == nil {
		d.Changes = []Change{}
	}
	return d, nil
}

func (s *Store) transaction(fn func(*Database) error) error {
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(s.dir, ".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	d, err := s.Read()
	if err != nil {
		return err
	}
	before := d.Revision
	if err := fn(&d); err != nil {
		return err
	}
	if before == d.Revision {
		return nil
	} // idempotent requests do not rewrite records
	return s.write(d)
}

func (s *Store) write(d Database) error {
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}
	if len(b) > 32<<20 {
		return errors.New("workspace snapshot exceeds 32MiB; write refused without changing saved records")
	}
	f, err := os.CreateTemp(s.dir, ".records-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err := f.Chmod(0600); err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), filepath.Join(s.dir, "records.json")); err != nil {
		return err
	}
	// Surface a durability failure even after commit; never claim the mutation was undone.
	dir, err := os.Open(s.dir)
	if err != nil {
		return fmt.Errorf("workspace committed; directory sync: %w", err)
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil {
		return fmt.Errorf("workspace committed; directory sync: %w", err)
	}
	return nil
}

func changed(d *Database, at int64, kind, matter, target, detail string) {
	d.Revision++
	d.Changes = append(d.Changes, Change{Seq: d.Revision, At: at, Kind: kind, Matter: matter, Target: target, Detail: detail})
}

func getMatter(d *Database, id string, revision uint64) (Matter, error) {
	m, ok := d.Matters[id]
	if !ok || m.MergedInto != "" {
		return Matter{}, ErrNotFound
	}
	if revision != 0 && m.Revision != revision {
		return Matter{}, ErrConflict
	}
	return m, nil
}

func saveMatter(d *Database, m Matter, at int64) {
	m.UpdatedAt, m.Revision = at, d.Revision
	d.Matters[m.ID] = m
}
