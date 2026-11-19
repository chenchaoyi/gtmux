package agentclient

import "golang.org/x/sys/unix"

func renameNewInputFile(dir int, oldName, newName string) error {
	return unix.Renameat2(dir, oldName, dir, newName, unix.RENAME_NOREPLACE)
}
