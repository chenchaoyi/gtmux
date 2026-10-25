package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/dsh"
	"github.com/chenchaoyi/gtmux/internal/state"
	"gopkg.in/yaml.v3"
)

const dstProfileBegin = "# gtmux:dsh-terminal:begin\n"
const dstProfileEnd = "# gtmux:dsh-terminal:end\n"

func dstProfilePath() string {
	return filepath.Join(dsh.Home(), "profiles", "dsh-tui", "cordis.patch.yml")
}

// Keep user text and custom YAML tags byte-for-byte. Only an owned block is
// replaced. Flow-style nonempty roots require an explicit manual conversion.
func dstProfileContent(original []byte, block string) ([]byte, error) {
	text := string(original)
	if strings.Count(text, dstProfileBegin) > 1 || strings.Count(text, dstProfileEnd) > 1 {
		return nil, fmt.Errorf("ambiguous gtmux profile block")
	}
	start, end := strings.Index(text, dstProfileBegin), strings.Index(text, dstProfileEnd)
	if start >= 0 || end >= 0 {
		if start < 0 || end < start {
			return nil, fmt.Errorf("incomplete gtmux profile block")
		}
		text = text[:start] + text[end+len(dstProfileEnd):]
	}
	var doc yaml.Node
	decoder := yaml.NewDecoder(strings.NewReader(text))
	if err := decoder.Decode(&doc); err != nil && err != io.EOF {
		return nil, err
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("profile must contain one YAML document")
	}
	if len(doc.Content) > 0 {
		root := doc.Content[0]
		if root.Kind != yaml.SequenceNode && root.Tag != "!!null" {
			return nil, fmt.Errorf("profile patch must be a YAML sequence")
		}
		if root.Kind == yaml.SequenceNode && root.Style&yaml.FlowStyle != 0 {
			if len(root.Content) > 0 {
				return nil, fmt.Errorf("convert the profile patch root to a block YAML sequence before installing")
			}
			lines := strings.Split(text, "\n")
			i := root.Line - 1
			if strings.TrimSpace(lines[i]) != "[]" {
				return nil, fmt.Errorf("unsupported empty profile root")
			}
			lines = append(lines[:i], lines[i+1:]...)
			text = strings.Join(lines, "\n")
		}
	}
	if block != "" {
		if text != "" && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		text += dstProfileBegin + block + dstProfileEnd
	} else {
		var remaining yaml.Node
		if yaml.Unmarshal([]byte(text), &remaining) != nil {
			return nil, fmt.Errorf("invalid remaining profile")
		}
		if len(remaining.Content) == 0 || remaining.Content[0].Tag == "!!null" {
			text += "[]\n"
		}
	}
	return []byte(text), nil
}

func prepareDSTProfile(inst agentInstaller, install bool) (string, []byte, []byte, error) {
	path := dstProfilePath()
	// Do not invent a community-client installation when it is absent.
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "package.json")); os.IsNotExist(err) {
		return "", nil, nil, nil
	}
	original, err := readDSHArtifact(path)
	if err != nil && !os.IsNotExist(err) {
		return "", nil, nil, err
	}
	if len(original) > 1<<20 {
		return "", nil, nil, fmt.Errorf("dst profile patch exceeds read limit")
	}
	if fi, e := os.Lstat(path); e == nil && !fi.Mode().IsRegular() {
		return "", nil, nil, fmt.Errorf("refusing linked dst profile patch")
	}
	block := ""
	if install {
		block = strings.TrimPrefix(dshTerminalArtifacts(inst)[dshTerminalOverlayPath(inst)], dshTerminalOverlayMarker)
	}
	next, err := dstProfileContent(original, block)
	return path, original, next, err
}

func writeDSTProfile(path string, original, next []byte) error {
	if path == "" || bytes.Equal(original, next) {
		return nil
	}
	if len(original) > 0 {
		sum := sha256.Sum256(original)
		backup := path + ".gtmux-backup-" + hex.EncodeToString(sum[:8])
		if _, err := os.Lstat(backup); os.IsNotExist(err) {
			f, err := state.OpenFile(backup, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil {
				return err
			}
			_, err = f.Write(original)
			closeErr := f.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
	current, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if !bytes.Equal(current, original) {
		return fmt.Errorf("dst profile changed during installation")
	}
	f, err := state.CreateTemp(filepath.Dir(path), ".gtmux-profile-*")
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
	return os.Rename(f.Name(), path)
}
