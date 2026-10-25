package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// Mode changes never hold this lock while asking the network or the user. Recovery
// takes it only to recheck user intent and apply the cloudflared token.
func lockRemoteServices() (func(), error) {
	if err := state.MkdirAll(state.ConfigDir(), 0700); err != nil {
		return nil, err
	}
	f, err := state.OpenFile(filepath.Join(state.ConfigDir(), "remote-services.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
		}
		if (!errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN)) || time.Now().After(deadline) {
			_ = f.Close()
			return nil, err
		}
		time.Sleep(25 * time.Millisecond)
	}
}

type standardSnapshot struct {
	files                        map[string][]byte
	deviceID, address, connector string
}

// Missing identity or credentials means reset/disabled, never first-time enrollment.
func snapshotStandard() (*standardSnapshot, error) {
	if tunnelBackend() != "standard" {
		return nil, errors.New("standard is not enabled")
	}
	paths := []string{tunnelAgentPath(), serveAgentPath(), filepath.Join(state.ConfigDir(), "tunnel-device-id"), filepath.Join(state.ConfigDir(), "serve-token")}
	s := &standardSnapshot{files: make(map[string][]byte)}
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil || len(b) == 0 || len(b) > 65536 {
			return nil, errors.New("standard settings are unavailable")
		}
		s.files[path] = b
	}
	// Both current and legacy addresses remain valid upgrade inputs.
	for _, path := range []string{tunnelURLPath(), state.LegacyTunnelURLPath()} {
		b, err := os.ReadFile(path)
		if err == nil && strings.TrimSpace(string(b)) != "" {
			s.files[path] = b
			s.address = strings.TrimSpace(string(b))
			break
		}
	}
	u, err := url.Parse(s.address)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("invalid saved Standard address")
	}
	s.deviceID = strings.TrimSpace(string(s.files[filepath.Join(state.ConfigDir(), "tunnel-device-id")]))
	if !regexp.MustCompile(`^[a-zA-Z0-9_-]{16,128}$`).MatchString(s.deviceID) {
		return nil, errors.New("invalid saved device identity")
	}
	s.connector, err = serviceManager().Connector(s.files[tunnelAgentPath()])
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *standardSnapshot) stillCurrent() bool {
	if tunnelBackend() != "standard" || readTunnelURL() != s.address {
		return false
	}
	for path, before := range s.files {
		now, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(now, before) {
			return false
		}
	}
	return true
}

// Testable recovery operation; never invokes install, credential generation, QR
// printing or serve reload. The old-bug tombstone follows the same recovery path.
func recoverStandard(ctx context.Context, api, reg string) error {
	s, err := snapshotStandard()
	if err != nil {
		return err
	}
	p, _, err := provisionAtContext(ctx, api, reg, s.deviceID, "", false, true, "/standard/recover")
	if err != nil {
		return err
	}
	if !p.AddressPreserved || p.URL != s.address {
		return errors.New("control plane did not preserve the saved address")
	}
	unlock, err := lockRemoteServices()
	if err != nil {
		return err
	}
	defer unlock()
	if !s.stillCurrent() {
		return errors.New("remote access choice changed during recovery")
	}
	marker := filepath.Join(state.ConfigDir(), "standard-recovery-pending")
	fingerprint := func(b []byte) []byte { return []byte(fmt.Sprintf("%x", sha256.Sum256(b))) }
	pending, _ := os.ReadFile(marker)
	mustReload := bytes.Equal(pending, fingerprint(s.files[tunnelAgentPath()]))
	if p.Token != s.connector {
		old := s.files[tunnelAgentPath()]
		updated, err := serviceManager().ReplaceConnector(old, s.connector, p.Token)
		if err != nil {
			return err
		}
		// Persist application intent before changing the agent. A failed unload/load
		// cannot masquerade as an unchanged healthy token on the next attempt.
		if err := writeRecoveryAgent(marker, fingerprint(updated)); err != nil {
			return err
		}
		mustReload = true
		if err := writeRecoveryAgent(tunnelAgentPath(), updated); err != nil {
			return err
		}
	}
	// Reload even on an unchanged token if launchd no longer has the agent loaded.
	if p.Token == s.connector && !mustReload {
		loaded := serviceLoaded(tunnelAgentLabel)
		if loaded {
			return nil
		}
	}
	for _, action := range []string{"unload", "load"} {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := userService(action, tunnelAgentPath())
		if err != nil && action == "load" {
			return errors.New("could not restart the Standard connector")
		}
	}
	_ = os.Remove(marker)
	return nil
}

func writeRecoveryAgent(path string, data []byte) error {
	f, err := state.CreateTemp(filepath.Dir(path), ".gtmux-recovery-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

type standardRecoveryWatch struct {
	mu              sync.Mutex
	downSince, next time.Time
	delay           time.Duration
	running         bool
}

var standardRecovery standardRecoveryWatch

// reserve implements grace, single-flight and capped exponential retry without
// blocking serve's slow tick. A successful API response is not edge readiness.
func (w *standardRecoveryWatch) reserve(now time.Time, connected bool) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if connected {
		w.downSince, w.next, w.delay = time.Time{}, time.Time{}, 0
		return false
	}
	if w.downSince.IsZero() {
		w.downSince = now
		return false
	}
	if w.running || now.Sub(w.downSince) < connectGrace || now.Before(w.next) {
		return false
	}
	w.running = true
	if w.delay == 0 {
		w.delay = time.Minute
	} else {
		w.delay *= 2
	}
	if w.delay > time.Hour {
		w.delay = time.Hour
	}
	w.next = now.Add(w.delay)
	return true
}
func (w *standardRecoveryWatch) complete() { w.mu.Lock(); w.running = false; w.mu.Unlock() }

func maybeRecoverStandard(connected bool) {
	if !standardRecovery.reserve(time.Now(), connected) {
		return
	}
	go func() {
		defer standardRecovery.complete()
		reg := tunnelRegSecret()
		if reg == "" {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		api := tunnelAPI()
		err := recoverStandard(ctx, api, reg)
		// The fallback uses the same dedicated stable-only endpoint. Older Workers
		// return 404 instead of running their address-rotating /provision recovery.
		if err != nil && ctx.Err() == nil {
			if fallback := tunnelAPIFallback(); fallback != "" && fallback != api {
				err = recoverStandard(ctx, fallback, reg)
			}
		}
		lg := diag.For("serve")
		if err != nil {
			lg.Debug("standard.recovery.retry", "Standard connection recovery will retry; existing access settings are retained")
		} else {
			lg.Info("standard.recovery.checked", "Standard connection settings checked; waiting for an edge connection")
		}
	}()
}
