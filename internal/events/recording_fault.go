package events

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// This small status file is independent of both the journal and its counter.
// Each invalidation gets a new generation, including concurrent failures.
// There is no event body, source identity or instruction count in the file.
func generationKey(tag byte) string {
	var nonce [16]byte
	rand.Read(nonce[:])
	return string(tag) + hex.EncodeToString(nonce[:])
}

func recordingGenerationPath() string { return filepath.Join(state.StatusDir(), "event-coverage") }

// Only a failure of the ordinary status store needs an independent HQ
// witness. It is a single content-free generation, never a copy of the log.
func hqRecordingFaultPath() string { return filepath.Join(state.HQHome(), "event-coverage-fault") }

func recordingGeneration(initialize bool) (string, error) {
	path := recordingGenerationPath()
	f, err := state.OpenPrivateRead(path)
	if initialize && errors.Is(err, os.ErrNotExist) {
		w, e := state.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e == nil {
			_, e = w.WriteString(generationKey('0'))
			e = errors.Join(e, w.Sync(), w.Close())
		}
		if e != nil && !errors.Is(e, os.ErrExist) {
			return "", e
		}
		f, err = state.OpenPrivateRead(path)
	}
	if err != nil {
		return "", err
	}
	defer f.Close()
	key, err := readGeneration(f)
	if err != nil {
		return "", err
	}
	fallback, err := state.OpenPrivateRead(hqRecordingFaultPath())
	if errors.Is(err, os.ErrNotExist) {
		return key, nil
	}
	if err != nil {
		return "", err
	}
	defer fallback.Close()
	other, err := readGeneration(fallback)
	return key + "/" + other, err
}

func readGeneration(f *os.File) (string, error) {
	b, err := io.ReadAll(io.LimitReader(f, 64))
	if err != nil || len(b) != 33 || (b[0] != '0' && b[0] != '1') {
		return "", errors.New("invalid event coverage generation")
	}
	if _, err = hex.DecodeString(string(b[1:])); err != nil {
		return "", errors.New("invalid event coverage nonce")
	}
	return string(b), nil
}

func invalidateRecording() error {
	path := recordingGenerationPath()
	key := []byte(generationKey('1'))
	err := state.WritePrivateFile(path, key)
	if err == nil {
		return nil
	}
	// The parent may be read-only while its already allocated status file
	// is writable. Preserve the changed generation across reopen in that case.
	f, e := state.OpenFile(path, os.O_WRONLY, 0600)
	if e != nil {
		return fallbackRecordingFault(key, errors.Join(err, e))
	}
	n, e := f.WriteAt(key, 0)
	if n != len(key) {
		e = errors.Join(e, io.ErrShortWrite)
	}
	if e = errors.Join(e, f.Sync(), f.Close()); e != nil {
		return fallbackRecordingFault(key, e)
	}
	return nil
}

func fallbackRecordingFault(key []byte, primary error) error {
	if err := state.WritePrivateFile(hqRecordingFaultPath(), key); err != nil {
		return fmt.Errorf("cannot persist event coverage failure: %w", errors.Join(primary, err))
	}
	return nil
}
