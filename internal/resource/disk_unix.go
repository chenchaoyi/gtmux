//go:build darwin || linux

package resource

import "golang.org/x/sys/unix"

func diskFreePortable(path string) (int, int, bool) {
	var st unix.Statfs_t
	if unix.Statfs(path, &st) != nil || st.Blocks == 0 {
		return 0, 0, false
	}
	free := uint64(st.Bavail) * uint64(st.Bsize) / (1 << 30)
	used := st.Blocks - st.Bfree
	denominator := used + uint64(st.Bavail)
	if denominator == 0 {
		return int(free), 0, true
	}
	return int(free), int(float64(used) / float64(denominator) * 100), true
}
