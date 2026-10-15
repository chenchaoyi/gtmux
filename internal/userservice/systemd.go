package userservice

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
)

type Systemd struct {
	Home, Config, SearchPath, Socket string
	Run                              Runner
}

func (m Systemd) Dir() string {
	if m.Config != "" && filepath.IsAbs(m.Config) {
		return filepath.Join(m.Config, "systemd", "user")
	}
	return filepath.Join(m.Home, ".config", "systemd", "user")
}
func (m Systemd) Path(label string) string { return filepath.Join(m.Dir(), label+".service") }
func (m Systemd) ctl(args ...string) error {
	return bounded(m.Run, "systemctl", append([]string{"--user"}, args...)...)
}
func (m Systemd) Check() error {
	if err := m.ctl("show-environment"); err != nil {
		return errors.New("systemd user manager unavailable; use a login session with systemd --user, or run `gtmux serve` in the foreground")
	}
	return nil
}
func (m Systemd) Start(path string) error {
	if err := m.ctl("daemon-reload"); err != nil {
		return err
	}
	if err := m.ctl("enable", "--now", filepath.Base(path)); err != nil {
		return err
	}
	return m.ctl("is-active", "--quiet", filepath.Base(path))
}
func (m Systemd) Stop(path string) error { return m.ctl("disable", "--now", filepath.Base(path)) }
func (m Systemd) Active(label string) bool {
	return m.ctl("is-active", "--quiet", label+".service") == nil
}
func (m Systemd) Restart(label string) error { return m.ctl("restart", label+".service") }
func (m Systemd) NeedsLinger() bool          { return true }

type unitSpec struct {
	Label                   string
	Args                    []string
	Log, Home, Path, Socket string
}

// systemd expands percent specifiers even inside quotes, and expands dollars in
// ExecStart. The ':' prefix disables environment expansion of the argument list.
func unitQuote(s string) string {
	s = strings.ReplaceAll(s, "%", "%%")
	b, _ := json.Marshal(s)
	return string(b)
}
func renderUnit(s unitSpec) []byte {
	meta, _ := json.Marshal(s)
	b := "# gtmux-spec: " + base64.StdEncoding.EncodeToString(meta) + "\n[Unit]\nDescription=gtmux user service\n\n[Service]\nType=exec\nRestart=on-failure\nRestartSec=3\nKillMode=process\nUMask=0077\n"
	b += "Environment=" + unitQuote("HOME="+s.Home) + "\nEnvironment=" + unitQuote("PATH="+s.Path) + "\n"
	if s.Socket != "" {
		b += "Environment=" + unitQuote("TMUX="+s.Socket) + "\n"
	}
	b += "ExecStart=:"
	for i, a := range s.Args {
		if i > 0 {
			b += " "
		}
		b += unitQuote(a)
	}
	b += "\nStandardOutput=append:" + strings.ReplaceAll(s.Log, "%", "%%") + "\nStandardError=append:" + strings.ReplaceAll(s.Log, "%", "%%") + "\n\n[Install]\nWantedBy=default.target\n"
	return []byte(b)
}
func (m Systemd) Write(path, label string, args []string, log string) error {
	if !filepath.IsAbs(log) || strings.ContainsAny(log, "\r\n") || strings.TrimSpace(log) != log {
		return errors.New("service log must be an absolute path without line breaks or edge whitespace")
	}
	if len(args) == 0 || !filepath.IsAbs(args[0]) {
		return errors.New("service executable must be an absolute path")
	}
	search := m.SearchPath
	if search == "" {
		search = filepath.Join(m.Home, ".local", "bin") + ":/usr/local/bin:/usr/bin:/bin"
	}
	// Pin the tmux socket for a service launched from a nondefault tmux server.
	// Only the socket prefix is retained: pane IDs and server PIDs are ephemeral.
	socket := m.Socket
	if socket != "" {
		socket = strings.SplitN(socket, ",", 2)[0] + ",0,0"
	}
	return write(path, renderUnit(unitSpec{label, args, log, m.Home, search, socket}))
}
func readUnit(data []byte) (unitSpec, error) {
	var s unitSpec
	line, _, _ := strings.Cut(string(data), "\n")
	encoded, ok := strings.CutPrefix(line, "# gtmux-spec: ")
	if !ok {
		return s, errors.New("unrecognized gtmux unit")
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || json.Unmarshal(raw, &s) != nil || !bytes.Equal(data, renderUnit(s)) {
		return s, errors.New("modified gtmux unit; repair manually before recovery")
	}
	return s, nil
}
func (m Systemd) Connector(data []byte) (string, error) {
	s, err := readUnit(data)
	if err != nil {
		return "", err
	}
	for i, a := range s.Args {
		if a == "--token" && i+1 < len(s.Args) && s.Args[i+1] != "" {
			return s.Args[i+1], nil
		}
	}
	return "", errors.New("connector token unavailable")
}
func (m Systemd) ReplaceConnector(data []byte, old, next string) ([]byte, error) {
	s, err := readUnit(data)
	if err != nil {
		return nil, err
	}
	found := 0
	for i, a := range s.Args {
		if a == "--token" && i+1 < len(s.Args) && s.Args[i+1] == old {
			s.Args[i+1] = next
			found++
		}
	}
	if found != 1 {
		return nil, errors.New("ambiguous connector token")
	}
	return renderUnit(s), nil
}
