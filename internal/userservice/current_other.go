//go:build !darwin && !linux

package userservice

import (
	"errors"
	"github.com/chenchaoyi/gtmux/internal/state"
	"path/filepath"
)

type unsupported struct{ home string }

func (m unsupported) Dir() string {
	return filepath.Join(state.ConfigDir(), "unsupported-services")
}
func (m unsupported) Path(label string) string { return filepath.Join(m.Dir(), label) }
func (unsupported) Check() error {
	return errors.New("background services are not supported on this platform; use foreground serve/tunnel")
}
func (m unsupported) Write(string, string, []string, string) error            { return m.Check() }
func (m unsupported) Start(string) error                                      { return m.Check() }
func (m unsupported) Stop(string) error                                       { return m.Check() }
func (m unsupported) Restart(string) error                                    { return m.Check() }
func (unsupported) Active(string) bool                                        { return false }
func (m unsupported) Connector([]byte) (string, error)                        { return "", m.Check() }
func (m unsupported) ReplaceConnector([]byte, string, string) ([]byte, error) { return nil, m.Check() }
func (unsupported) NeedsLinger() bool                                         { return false }
func Current(home string) Manager                                             { return unsupported{home: home} }
