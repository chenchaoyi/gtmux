//go:build !darwin && !linux

package resource

func diskFreePortable(string) (int, int, bool) { return 0, 0, false }
