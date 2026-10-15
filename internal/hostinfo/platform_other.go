//go:build !darwin && !linux

package hostinfo

import "runtime"

func gatherPlatform(in *Info) { in.OS = runtime.GOOS }
