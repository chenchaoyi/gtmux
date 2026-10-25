package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/state"
)

type bridgeRefreshReceipt struct {
	Schema int    `json:"schema"`
	Agent  string `json:"agent"`
	Status string `json:"status"`
}

// Maintenance is limited to an already installed, recognized gtmux bridge.
// It never enables a new profile, downloads a client, or restarts an agent.
func refreshOwnedDSHBridge() int {
	status, err := refreshDSHBridge(agentInstallers["dsh"])
	if err != nil {
		status = "failed"
	}
	diag.Did("act.maintenance.dsh-bridge", "dsh", diag.Outcome(err), "checked an existing DeepSeek state bridge for maintenance", "status", status)
	_ = json.NewEncoder(os.Stdout).Encode(bridgeRefreshReceipt{1, "dsh", status})
	if err != nil {
		fmt.Fprintln(os.Stderr, "DSH bridge maintenance: "+err.Error())
		return 1
	}
	return 0
}

func refreshDSHBridge(inst agentInstaller) (string, error) {
	module, err := readDSHArtifact(inst.configPath())
	if os.IsNotExist(err) {
		return "not-installed", nil
	}
	if err != nil {
		return "failed", err
	}
	if !strings.HasPrefix(string(module), dshPluginMarker) {
		return "failed", fmt.Errorf("existing bridge is not owned by gtmux")
	}
	if dshBridgeFromNewerRelease(module) {
		return "newer", nil
	}
	bin := installedPluginBinary(module)
	if bin == "" {
		return "failed", fmt.Errorf("existing bridge executable is unrecognized")
	}
	// Refuse redirected provider directories before refreshing their contents.
	for _, dir := range []string{filepath.Dir(inst.configPath()), filepath.Join(filepath.Dir(inst.configPath()), "terminal-dst")} {
		info, err := os.Lstat(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "failed", err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "failed", fmt.Errorf("bridge directory is redirected")
		}
	}
	expected := dshTerminalArtifacts(inst)
	expected[inst.configPath()] = inst.plugin(bin)
	if dshPluginCompatible(module, bin) {
		expected[inst.configPath()] = string(module)
	}
	expected[dshOverlayPath(inst)] = dshOverlayYAML(inst)
	current := true
	original := map[string][]byte{}
	for path, want := range expected {
		have, err := readDSHArtifact(path)
		if err != nil && !os.IsNotExist(err) {
			return "failed", err
		}
		if err == nil {
			owned := dshTerminalOwned(path, have)
			if path == inst.configPath() {
				owned = strings.HasPrefix(string(have), dshPluginMarker)
			}
			if path == dshOverlayPath(inst) {
				owned = strings.HasPrefix(string(have), dshOverlayMarker)
			}
			if !owned {
				return "failed", fmt.Errorf("bridge artifact is not owned by gtmux")
			}
		}
		original[path] = have
		if err != nil || string(have) != want {
			current = false
		}
	}
	if current {
		return "current", nil
	}
	expected[inst.configPath()] = inst.plugin(bin)
	for path, want := range expected {
		if bytes.Equal(original[path], []byte(want)) {
			continue
		}
		if err := refreshDSHArtifact(path, original[path], []byte(want)); err != nil {
			return "failed", err
		}
	}
	for path, want := range expected {
		have, err := readDSHArtifact(path)
		if err != nil || string(have) != want {
			return "failed", fmt.Errorf("bridge refresh verification failed")
		}
	}
	return "updated", nil
}

func installedPluginBinary(module []byte) string {
	for _, line := range strings.Split(string(module), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "const BIN = ") {
			var bin string
			quoted := strings.TrimSuffix(strings.TrimPrefix(line, "const BIN = "), ";")
			if json.Unmarshal([]byte(quoted), &bin) == nil {
				return bin
			}
		}
	}
	return ""
}

// Publish complete files and refuse a concurrently changed target. Replacing an
// inode also avoids writing through a symlink or hard link at the target itself.
func refreshDSHArtifact(path string, original, next []byte) error {
	if err := state.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	check := func() error {
		current, err := readDSHArtifact(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if !bytes.Equal(current, original) {
			return fmt.Errorf("bridge changed during refresh")
		}
		return nil
	}
	if err := check(); err != nil {
		return err
	}
	f, err := state.CreateTemp(filepath.Dir(path), ".gtmux-refresh-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(next); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = check(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
