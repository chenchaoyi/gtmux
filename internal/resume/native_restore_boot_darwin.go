//go:build darwin

package resume

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

type nativeBootOutput struct {
	body     []byte
	overflow bool
}

func (b *nativeBootOutput) Write(p []byte) (int, error) {
	if len(b.body)+len(p) > 128 {
		b.overflow = true
	} else {
		b.body = append(b.body, p...)
	}
	return len(p), nil
}
func platformNativeRestoreBoot() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/usr/sbin/sysctl", "-n", "kern.bootsessionuuid")
	cmd.WaitDelay = 100 * time.Millisecond
	var output nativeBootOutput
	cmd.Stdout = &output
	err := cmd.Run()
	value := strings.ToLower(strings.TrimSpace(string(output.body)))
	if err != nil || output.overflow || !validNativeBootID(value) {
		return "", errNativeBootUnavailable
	}
	return value, nil
}
