package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/artifact"
	"github.com/chenchaoyi/gtmux/internal/diag"
)

var artifactPreviewLaunch = launchArtifactPreview

func artifactReferenceID(reference string) (string, error) {
	id := strings.TrimPrefix(reference, "gtmux-file:")
	if len(id) != 32 || strings.Trim(id, "0123456789abcdef") != "" {
		return "", errors.New("a canonical gtmux-file:ID or 32-character ID is required")
	}
	return id, nil
}

// Local owner reads use the same verified immutable bytes as paired devices.
// The source path is descriptive metadata, never a read fallback or opener target.
func localArtifact(reference string) (artifact.File, error) {
	id, err := artifactReferenceID(reference)
	if err != nil {
		return artifact.File{}, err
	}
	file, content, err := artifact.Open(id)
	if err != nil {
		return artifact.File{}, err
	}
	content.Close()
	return file, nil
}

// The fragment is an address. The browser supplies its own paired credential.
func localArtifactPreviewURL(file artifact.File, port int) (string, error) {
	status, fresh := diag.ReadStatus("serve")
	if !fresh || status.State != "running" || status.Component != "serve" || status.Updated.After(time.Now()) {
		return "", errors.New("no current local workspace; start the workspace and try again")
	}
	base, boot, err := previewEndpoint(status.Detail, port)
	if err != nil {
		return "", err
	}
	// The private heartbeat and live service must identify the same process.
	// Health is public: the probe never reads or sends a browser/owner token.
	transport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Get(base + "/api/health")
	if err != nil {
		return "", errors.New("local workspace is unavailable; start the workspace and try again")
	}
	defer response.Body.Close()
	var health struct{ Status, Service, Boot string }
	body, err := io.ReadAll(io.LimitReader(response.Body, 4097))
	if response.StatusCode != http.StatusOK || err != nil || len(body) > 4096 || json.Unmarshal(body, &health) != nil ||
		health.Status != "ok" || health.Service != "gtmux" || health.Boot != boot {
		return "", errors.New("local workspace identity could not be verified")
	}
	fields := url.Values{"document": {file.ID}, "session": {file.SessionID}, "home": {file.HomeID}}
	return base + "/#" + fields.Encode(), nil
}

func previewEndpoint(detail map[string]any, requested int) (string, string, error) {
	port, ok := detail["port"].(float64)
	if !ok || port < 1 || port > 65535 || float64(int(port)) != port {
		return "", "", errors.New("invalid port in local workspace status")
	}
	if requested != 0 && requested != int(port) {
		return "", "", errors.New("requested port differs from the active workspace port")
	}
	bind, ok := detail["bind"].(string)
	if !ok {
		return "", "", errors.New("missing local workspace bind address")
	}
	host := "127.0.0.1"
	switch bind {
	case "", "localhost", "0.0.0.0", "127.0.0.1":
	case "::1", "::":
		host = "::1"
	default:
		return "", "", errors.New("the workspace is not bound to a loopback-accessible address")
	}
	boot, ok := detail["boot"].(string)
	if !ok || len(boot) != 16 || strings.Trim(boot, "0123456789abcdef") != "" {
		return "", "", errors.New("invalid local workspace boot identity")
	}
	return "http://" + net.JoinHostPort(host, strconv.Itoa(int(port))), boot, nil
}

func launchArtifactPreview(endpoint string) error {
	var command string
	switch runtime.GOOS {
	case "darwin":
		command = "open"
	case "linux":
		command = "xdg-open"
	default:
		return errors.New("no local browser opener; use --print-url")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, command, endpoint).Run(); err != nil {
		return errors.New("browser could not be opened; use --print-url or open the local workspace")
	}
	return nil
}

func cmdArtifactView(args []string) int {
	if len(args) < 2 {
		return 2
	}
	action, reference := args[0], args[1]
	if action != "show" && action != "open" {
		return 2
	}
	structured, printURL, port := false, false, 0
	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--json":
			if action != "show" {
				return 2
			}
			structured = true
		case "--print-url":
			if action != "open" {
				return 2
			}
			printURL = true
		case "--port":
			if action != "open" || i+1 >= len(args) {
				return 2
			}
			i++
			value, err := strconv.Atoi(args[i])
			if err != nil || value < 1 || value > 65535 {
				return 2
			}
			port = value
		default:
			return 2
		}
	}
	file, err := localArtifact(reference)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Document unavailable:", err)
		return 1
	}
	if action == "show" {
		if structured {
			_ = json.NewEncoder(os.Stdout).Encode(struct {
				artifact.File
				Reference string `json:"reference"`
			}{file, file.Reference()})
		} else {
			fmt.Printf("%q\nType: %s · %d bytes\nReference: %s\nConversation: %q\nHome: %q\nSource: %q\nSHA256: %s\n", file.Name, file.Kind, file.Bytes, file.Reference(), file.SessionID, file.HomeID, file.Source, file.SHA256)
		}
		return 0
	}
	endpoint, err := localArtifactPreviewURL(file, port)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if printURL {
		fmt.Println(endpoint)
		return 0
	}
	if err := artifactPreviewLaunch(endpoint); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
