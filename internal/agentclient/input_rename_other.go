//go:build !darwin && !linux

package agentclient

import "golang.org/x/sys/unix"

func renameNewInputFile(int, string, string) error {
	return unix.ENOTSUP
}
