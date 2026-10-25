package app

import (
	"encoding/json"
	"os"
	"syscall"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/server"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestConfigurationAndCredentialsArePrivateOnRewrite(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	previous := syscall.Umask(0)
	defer syscall.Umask(previous)
	paths := []string{configPath(), devicesPath(), sharePath(), pushTokensPath()}
	for _, stale := range []bool{false, true} {
		if stale {
			for _, p := range paths {
				if err := os.Chmod(p, 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
		if err := writeConfigMap(map[string]any{"fixture": "value"}); err != nil {
			t.Fatal(err)
		}
		saveDevices([]server.EnrolledDevice{})
		saveShareState(server.ShareState{})
		savePushTokens([]server.DeviceToken{})
		for _, p := range paths {
			info, err := os.Stat(p)
			if err != nil || info.Mode().Perm() != 0600 {
				t.Errorf("stale=%v %s: %v %v", stale, p, info, err)
			}
			data, err := os.ReadFile(p)
			if err != nil || !json.Valid(data) {
				t.Fatalf("format changed: %s %v", p, err)
			}
		}
		info, err := os.Stat(state.ConfigDir())
		if err != nil || info.Mode().Perm() != 0700 {
			t.Fatal("config directory is not private", err)
		}
	}
}
