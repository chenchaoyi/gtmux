package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// Cloudflare's October 2026 certificate incident fixes differ for native Mac
// downloads. Recognize the installed channel rather than replacing it silently.
const cloudflaredUpdateURL = "https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/downloads/update-cloudflared/"

var cloudflaredRelease = regexp.MustCompile(`^cloudflared version ([0-9]{4})\.([0-9]{1,2})\.([0-9]{1,3})(?:\s|$)`)

func cloudflaredVersionSafe(output, platform string, homebrew bool) (string, error) {
	m := cloudflaredRelease.FindStringSubmatch(strings.TrimSpace(output))
	minimum := "2026.10.2"
	patch := 2
	if platform == "darwin" && !homebrew {
		minimum, patch = "2026.10.3", 3
	}
	if m == nil {
		return "", fmt.Errorf("cannot verify cloudflared release; need %s or later", minimum)
	}
	parts := [3]int{}
	for i := range parts {
		parts[i], _ = strconv.Atoi(m[i+1])
	}
	version := fmt.Sprintf("%d.%d.%d", parts[0], parts[1], parts[2])
	if parts[1] < 1 || parts[1] > 12 || parts[0] < 2026 || parts[0] == 2026 && (parts[1] < 10 || parts[1] == 10 && parts[2] < patch) {
		return version, fmt.Errorf("cloudflared %s needs a security update to %s or later", version, minimum)
	}
	return version, nil
}

// Homebrew can coexist in two prefixes. Update the owner of this executable,
// never whichever unrelated brew happens to appear first in PATH.
func cloudflaredBrew(binary string) string {
	real, err := filepath.EvalSymlinks(binary)
	if err != nil {
		return ""
	}
	formula := filepath.Dir(filepath.Dir(filepath.Dir(real)))
	if filepath.Base(formula) != "cloudflared" || filepath.Base(filepath.Dir(formula)) != "Cellar" {
		return ""
	}
	brew := filepath.Join(filepath.Dir(filepath.Dir(formula)), "bin", "brew")
	if st, err := os.Stat(brew); err == nil && st.Mode().IsRegular() && st.Mode().Perm()&0o111 != 0 {
		return brew
	}
	return ""
}

type cloudflaredVersionOutput struct{ bytes.Buffer }

func (w *cloudflaredVersionOutput) Write(p []byte) (int, error) {
	if w.Len()+len(p) > 4096 {
		return 0, fmt.Errorf("cloudflared version output too large")
	}
	return w.Buffer.Write(p)
}

func checkCloudflared(binary string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "--version")
	cmd.WaitDelay = 200 * time.Millisecond
	var out cloudflaredVersionOutput
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("could not read cloudflared version")
	}
	return cloudflaredVersionSafe(out.String(), runtime.GOOS, cloudflaredBrew(binary) != "")
}

func requireCloudflared(yes bool) string {
	if bin := lookTool("cloudflared"); bin != "" {
		if _, err := checkCloudflared(bin); err == nil {
			return bin
		}
	}
	bin := ensureCloudflared(yes)
	if bin == "" {
		return ""
	}
	if _, err := checkCloudflared(bin); err != nil {
		cloudflaredSecurityError(err)
		return ""
	}
	return bin
}

func cloudflaredSecurityError(err error) {
	i18n.Sae("gtmux tunnel: "+err.Error()+". Update through the original installation channel: "+cloudflaredUpdateURL,
		"gtmux tunnel：无法确认 cloudflared 满足安全版本要求。请通过原安装渠道更新后重试："+cloudflaredUpdateURL)
}

// Routine gtmux installation updates an already-present dependency, but does
// not add an optional tunnel client or change remote-access enrollment.
func updateInstalledCloudflared() int {
	if bin := lookTool("cloudflared"); bin == "" {
		return 0
	} else if _, err := checkCloudflared(bin); err == nil {
		return checkRunningCloudflaredUpdate()
	}
	if requireCloudflared(true) == "" {
		return 1
	}
	return checkRunningCloudflaredUpdate()
}

var cloudflaredBuildVersion = regexp.MustCompile(`^version="([0-9]{4}\.[0-9]{1,2}\.[0-9]{1,3})"$`)

func cloudflaredVersionFromMetrics(body string) string {
	var version string
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, "build_info{") || !strings.HasSuffix(line, "} 1") {
			continue
		}
		labels := strings.TrimSuffix(strings.TrimPrefix(line, "build_info{"), "} 1")
		if strings.Contains(labels, `\`) {
			return ""
		}
		if version != "" {
			return ""
		}
		for _, label := range strings.Split(labels, ",") {
			if !strings.HasPrefix(label, "version=") {
				continue
			}
			m := cloudflaredBuildVersion.FindStringSubmatch(label)
			if m == nil || version != "" {
				return ""
			}
			version = m[1]
		}
		if version == "" {
			return ""
		}
	}
	return version
}

// The dedicated gtmux metrics port identifies its connector. Do not inspect
// unrelated clients on the default ports or mistake file replacement for restart.
func runningCloudflaredVersion() string {
	return readCloudflaredMetrics("http://" + cloudflaredMetricsAddr + "/metrics")
}

func readCloudflaredMetrics(url string) string {
	hc := &http.Client{Timeout: 700 * time.Millisecond, Transport: &http.Transport{Proxy: nil},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer hc.CloseIdleConnections()
	resp, err := hc.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 262145))
	if err != nil || len(body) > 262144 {
		return ""
	}
	return cloudflaredVersionFromMetrics(string(body))
}

func checkRunningCloudflaredUpdate() int {
	if tunnelBackend() != "standard" || !serviceLoaded(tunnelAgentLabel) {
		return 0
	}
	version := runningCloudflaredVersion()
	// Metrics do not attest the running executable's installation channel. Use
	// the stricter Mac floor instead of assuming it is the current Homebrew file.
	if _, err := cloudflaredVersionSafe("cloudflared version "+version, runtime.GOOS, false); err == nil {
		return 0
	}
	i18n.Sae("The installed cloudflared is updated, but the running Standard connector is old or unverified. Restart it with `gtmux tunnel --service --recover`, then check its connection. Long-lived connections will reconnect.",
		"安装的 cloudflared 已更新，但运行中的 Standard 客户端仍旧或尚未确认版本。请运行 gtmux tunnel --service --recover 重启并检查连接；已有长连接需要重新连接。")
	return 1
}
