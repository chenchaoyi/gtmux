package hq

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/relay"
)

var relayObservationName = regexp.MustCompile(`(^|-)relays?(-|\.)`)

func explicitlyIncluded(name string, include []string) bool {
	for _, p := range include {
		if name == p || strings.HasPrefix(name, p+"/") {
			return true
		}
	}
	return false
}

// A legacy name is only a hint. A complete relay export must confirm the scope;
// malformed, unknown and ordinary notes remain protected, even if they are large.
func legacyRelayObservation(path, name string, info os.FileInfo) bool {
	if !strings.HasPrefix(name, "notes/") || strings.ToLower(filepath.Ext(name)) != ".json" ||
		!relayObservationName.MatchString(strings.ToLower(filepath.Base(name))) ||
		!info.Mode().IsRegular() || info.Size() > archiveFileMaxBytes {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil || !before.Mode().IsRegular() || !os.SameFile(info, before) ||
		before.Size() != info.Size() || !before.ModTime().Equal(info.ModTime()) {
		return false
	}
	matched := relay.IsObservationExport(io.LimitReader(f, archiveFileMaxBytes+1))
	after, err := f.Stat()
	return matched && err == nil && after.Size() == before.Size() &&
		after.ModTime().Equal(before.ModTime()) && after.Mode() == before.Mode()
}

// ProcessMaterialState is a conservative read-only inventory, not a deletion plan.
// Recognition never changes the file, promotes a snapshot or grants input rights.
type ProcessMaterialState struct {
	Bytes                    int64
	Files                    int
	RelayBytes               int64
	AttachmentBytes          int64
	SourceBytes              int64
	UnclassifiedArchiveBytes int64
	Incomplete               bool
}

func ReadProcessMaterialState() ProcessMaterialState {
	return processMaterialState(MemoryRoot())
}

func processMaterialState(root string) ProcessMaterialState {
	var s ProcessMaterialState
	notes := filepath.Join(root, "notes")
	_ = filepath.Walk(notes, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if !os.IsNotExist(err) {
				s.Incomplete = true
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			s.Incomplete = true
			return nil
		}
		name := filepath.ToSlash(rel)
		ext := strings.ToLower(filepath.Ext(name))
		isRelay := ext == ".json" && relayObservationName.MatchString(strings.ToLower(filepath.Base(name))) &&
			(!automaticFile(name, nil) || legacyRelayObservation(path, name, info))
		switch {
		case isRelay:
			s.RelayBytes += info.Size()
		case !strings.HasPrefix(name, "notes/reports/") && archiveMaterialExtension(ext):
			s.UnclassifiedArchiveBytes += info.Size()
			return nil // An archive may contain unique HQ recovery history.
		case !automaticFile(name, nil):
			s.AttachmentBytes += info.Size()
		default:
			switch ext {
			case ".go", ".swift", ".py", ".js", ".cjs", ".ts", ".tsx", ".sh", ".diff", ".html":
				s.SourceBytes += info.Size()
			default:
				return nil
			}
		}
		s.Bytes += info.Size()
		s.Files++
		return nil
	})
	return s
}

func archiveMaterialExtension(ext string) bool {
	switch ext {
	case ".zip", ".gz", ".tgz", ".bundle", ".ipa", ".pdf":
		return true
	default:
		return false
	}
}
