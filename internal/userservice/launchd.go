package userservice

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Launchd struct {
	Home string
	Run  Runner
}

func (m Launchd) Dir() string              { return filepath.Join(m.Home, "Library", "LaunchAgents") }
func (m Launchd) Path(label string) string { return filepath.Join(m.Dir(), label+".plist") }
func (m Launchd) Check() error             { return bounded(m.Run, "launchctl", "list") }
func (m Launchd) Start(path string) error  { return bounded(m.Run, "launchctl", "load", "-w", path) }
func (m Launchd) Stop(path string) error {
	label := strings.TrimSuffix(filepath.Base(path), ".plist")
	if !m.Active(label) {
		return m.Check()
	}
	return bounded(m.Run, "launchctl", "unload", "-w", path)
}
func (m Launchd) Active(label string) bool { return bounded(m.Run, "launchctl", "list", label) == nil }
func (m Launchd) Restart(label string) error {
	return bounded(m.Run, "launchctl", "kickstart", "-k", "gui/"+strconv.Itoa(os.Getuid())+"/"+label)
}
func (m Launchd) NeedsLinger() bool { return false }
func xmlEscape(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
func (m Launchd) Write(path, label string, args []string, log string) error {
	b := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>` + xmlEscape(label) + `</string>
  <key>ProgramArguments</key><array>
`
	for _, arg := range args {
		b += "    <string>" + xmlEscape(arg) + "</string>\n"
	}
	b += `  </array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>` + xmlEscape(log) + `</string>
  <key>StandardErrorPath</key><string>` + xmlEscape(log) + `</string>
</dict></plist>
`
	if err := prepareCapture(log); err != nil {
		return err
	}
	return write(path, []byte(b))
}
func (m Launchd) Connector(data []byte) (string, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	key := ""
	inArgs, next := false, false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", errors.New("invalid service")
		}
		switch node := tok.(type) {
		case xml.StartElement:
			if node.Name.Local == "key" {
				if dec.DecodeElement(&key, &node) != nil {
					return "", errors.New("invalid service")
				}
			}
			if node.Name.Local == "array" && key == "ProgramArguments" {
				inArgs = true
			}
			if node.Name.Local == "string" && inArgs {
				var v string
				if dec.DecodeElement(&v, &node) != nil {
					return "", errors.New("invalid service")
				}
				if next && v != "" {
					return v, nil
				}
				next = v == "--token"
			}
		case xml.EndElement:
			if node.Name.Local == "array" {
				inArgs = false
			}
		}
	}
	return "", errors.New("connector token unavailable")
}
func (m Launchd) ReplaceConnector(data []byte, old, next string) ([]byte, error) {
	needle := []byte("<string>" + xmlEscape(old) + "</string>")
	if bytes.Count(data, needle) != 1 {
		return nil, errors.New("ambiguous connector token")
	}
	return bytes.Replace(data, needle, []byte("<string>"+xmlEscape(next)+"</string>"), 1), nil
}
