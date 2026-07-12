package events

import (
	"os"
	"syscall"
)

// inode returns an open file's inode number (0 if unavailable) — used to detect
// rotation (the path now points at a different file). Unix-only via syscall (no
// cgo). gtmux ships on macOS/Linux.
func inode(f *os.File) uint64 {
	fi, err := f.Stat()
	if err != nil {
		return 0
	}
	return statInode(fi)
}

func statInode(fi os.FileInfo) uint64 {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Ino)
	}
	return 0
}
