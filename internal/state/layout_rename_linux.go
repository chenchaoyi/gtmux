package state

import "golang.org/x/sys/unix"

func renameLayoutEntry(from int, old string, to int, name string) error {
	return unix.Renameat2(from, old, to, name, unix.RENAME_NOREPLACE)
}
