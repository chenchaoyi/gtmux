package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func bindServePort(bind string, port int) (net.Listener, *state.Lease, error) {
	lease, err := state.TryLease(fmt.Sprintf("serve-%d", port))
	if err != nil {
		return nil, nil, err
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(bind, strconv.Itoa(port)))
	if err != nil {
		lease.Close()
		return nil, nil, err
	}
	return listener, lease, nil
}

// Reusing a listener requires both gtmux identity and a working existing owner
// credential. Never mint a new token merely because an unrelated port is occupied.
func existingRadarToken(port int) (string, error) {
	base := "http://127.0.0.1:" + strconv.Itoa(port)
	tr := &http.Transport{Proxy: nil}
	defer tr.CloseIdleConnections()
	client := &http.Client{Timeout: time.Second, Transport: tr, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Get(base + "/api/health")
	if err != nil {
		return "", err
	}
	var health struct{ Status, Service string }
	decodeErr := json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&health)
	response.Body.Close()
	if decodeErr != nil || response.StatusCode != 200 || health.Status != "ok" || health.Service != "gtmux" {
		return "", fmt.Errorf("port %d is not a ready gtmux API", port)
	}
	b, err := os.ReadFile(filepath.Join(state.ConfigDir(), "serve-token"))
	if err != nil {
		return "", fmt.Errorf("existing serve credential unavailable")
	}
	token := strings.TrimSpace(string(b))
	if token == "" {
		return "", fmt.Errorf("existing serve credential unavailable")
	}
	request, err := http.NewRequest(http.MethodGet, base+"/api/devices", nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err = client.Do(request)
	if err != nil {
		return "", err
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		return "", fmt.Errorf("existing serve does not accept the saved owner credential")
	}
	return token, nil
}
