//go:build linux

package resume

import (
	"io"
	"os"
	"strings"
)

func platformNativeRestoreBoot() (string, error) {
	f, err := os.Open("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", errNativeBootUnavailable
	}
	defer f.Close()
	body, err := io.ReadAll(io.LimitReader(f, 129))
	value := strings.ToLower(strings.TrimSpace(string(body)))
	if err != nil || len(body) > 128 || !validNativeBootID(value) {
		return "", errNativeBootUnavailable
	}
	return value, nil
}
