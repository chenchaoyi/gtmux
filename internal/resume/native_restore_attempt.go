package resume

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/hqpane"
	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

var ErrNativeRestoreReserved = errors.New("native restore already attempted in this boot")
var nativeRestoreBootProbe = platformNativeRestoreBoot
var nativeRestoreRootRead = launchhost.ReadAgent
var nativeRestoreNow = time.Now

func NativeRestoreBootID() (string, error) { return nativeRestoreBootProbe() }

// NativeRestoreIdentity is private evidence, not a mobile input capability.
type NativeRestoreIdentity struct {
	BootID, Home, HomeID, SessionID, SaveRevision     string
	Pane, Loc, InitialCwd, Cwd, TTY, RootBirth, Shell string
	RootPID                                           int
	BinaryPath, LookupPath, BinarySHA, PackageSHA     string
	BinaryDevice, BinaryInode                         uint64
	BinaryMode, BinaryUID                             uint32
	BinarySize, BinaryMTime                           int64
}

type NativeRestoreAttempt struct {
	Schema     int
	ID, State  string
	Identity   NativeRestoreIdentity
	IssuedAt   int64
	PriorPID   int
	PriorBirth string
}

func nativeRestoreAttemptPath(id NativeRestoreIdentity) string {
	sum := sha256.Sum256([]byte(id.Home + "\x00" + id.SessionID))
	return filepath.Join(state.Dir(), "restore-attempts", hex.EncodeToString(sum[:])+".json")
}

func validNativeRestoreIdentity(id NativeRestoreIdentity) bool {
	if !validNativeBootID(id.BootID) || id.RootPID <= 1 || id.Pane == "" || id.Loc == "" ||
		id.TTY == "" || id.RootBirth == "" || id.Shell == "" || id.SessionID == "" ||
		id.SaveRevision == "" || id.BinarySHA == "" || id.PackageSHA == "" || id.BinaryInode == 0 ||
		id.BinarySize <= 0 || !filepath.IsAbs(id.Home) || !filepath.IsAbs(id.BinaryPath) ||
		!filepath.IsAbs(id.Cwd) || !filepath.IsAbs(id.InitialCwd) {
		return false
	}
	for _, value := range []string{id.Home, id.SessionID, id.Loc, id.Cwd, id.TTY, id.RootBirth, id.BinaryPath, id.LookupPath} {
		if len(value) > 2048 || strings.ContainsAny(value, "\x00\r\n") {
			return false
		}
	}
	home, err := filepath.EvalSymlinks(id.Home)
	return err == nil && home == id.Home && id.HomeID == transcript.CodexHomeIDForPath(home) &&
		id.HomeID == transcript.CodexHomeID()
}

func validNativeBootID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, c := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

func validNativeRestoreAttempt(a NativeRestoreAttempt) bool {
	if a.Schema != 1 || len(a.ID) != 32 || strings.Trim(a.ID, "0123456789abcdef") != "" ||
		a.IssuedAt <= 0 || !validNativeRestoreIdentity(a.Identity) {
		return false
	}
	switch a.State {
	case "reserved", "pending", "prefilled", "unknown", "failed", "not-sent", "confirmed":
		return true
	}
	return false
}

func readNativeRestoreAttempt(path string) (NativeRestoreAttempt, bool, error) {
	var a NativeRestoreAttempt
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return a, false, nil
	}
	if err != nil {
		return a, false, err
	}
	owner, ok := info.Sys().(*syscall.Stat_t)
	if !ok || owner.Uid != uint32(os.Geteuid()) || !info.Mode().IsRegular() ||
		info.Mode().Perm()&0077 != 0 || info.Size() > 8192 {
		return a, false, errors.New("native restore record is unsafe")
	}
	f, err := state.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return a, false, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return a, false, errors.New("native restore record changed")
	}
	body, err := io.ReadAll(io.LimitReader(f, 8193))
	if err != nil || len(body) > 8192 {
		return a, false, errors.New("native restore record unreadable")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&a) != nil || !validNativeRestoreAttempt(a) || path != nativeRestoreAttemptPath(a.Identity) {
		return a, false, errors.New("native restore record invalid")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return a, false, errors.New("native restore record has trailing content")
	}
	return a, true, nil
}

func publishNativeRestoreAttempt(a NativeRestoreAttempt) error {
	if !validNativeRestoreAttempt(a) {
		return errors.New("invalid native restore attempt")
	}
	body, err := json.Marshal(a)
	if err != nil || len(body) > 8192 {
		return errors.New("native restore attempt too large")
	}
	return state.WritePrivateFile(nativeRestoreAttemptPath(a.Identity), body)
}

// ReserveNativeRestoreAttempt must precede the first tmux write. Every state in
// the same boot remains occupied, including a crash before the result is saved.
func ReserveNativeRestoreAttempt(id NativeRestoreIdentity) (NativeRestoreAttempt, error) {
	var a NativeRestoreAttempt
	if !validNativeRestoreIdentity(id) {
		return a, errors.New("native restore identity unavailable")
	}
	boot, err := nativeRestoreBootProbe()
	if err != nil || boot != id.BootID {
		return a, errors.New("native restore boot changed")
	}
	lease, err := state.TryLease("restore-native-attempts")
	if err != nil {
		return a, err
	}
	defer lease.Close()
	old, exists, err := readNativeRestoreAttempt(nativeRestoreAttemptPath(id))
	if err != nil {
		return a, err
	}
	if exists && old.Identity.BootID == boot {
		return a, ErrNativeRestoreReserved
	}
	again, err := nativeRestoreBootProbe()
	if err != nil || again != boot {
		return a, errors.New("native restore boot changed under lease")
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return a, err
	}
	a = NativeRestoreAttempt{Schema: 1, ID: hex.EncodeToString(nonce[:]), State: "reserved",
		Identity: id, IssuedAt: nativeRestoreNow().UnixNano()}
	if prior, ok := native.Load(id.SessionID, id.HomeID); ok {
		a.PriorPID, a.PriorBirth = prior.PID, prior.ProcessStart
	}
	if err := publishNativeRestoreAttempt(a); err != nil {
		return NativeRestoreAttempt{}, err
	}
	return a, nil
}

// UpdateNativeRestoreAttempt is a full-record CAS. A late old callback cannot
// overwrite a different attempt/boot or release any reservation.
func UpdateNativeRestoreAttempt(expected NativeRestoreAttempt, next string) (NativeRestoreAttempt, error) {
	a := expected
	switch expected.State {
	case "reserved":
		if next != "pending" && next != "prefilled" && next != "unknown" && next != "failed" && next != "not-sent" {
			return a, errors.New("invalid native restore transition")
		}
	case "pending":
		if next != "confirmed" && next != "unknown" {
			return a, errors.New("invalid native restore transition")
		}
	default:
		return a, errors.New("native restore result is terminal")
	}
	lease, err := state.TryLease("restore-native-attempts")
	if err != nil {
		return a, err
	}
	defer lease.Close()
	current, exists, err := readNativeRestoreAttempt(nativeRestoreAttemptPath(a.Identity))
	if err != nil || !exists || current != expected {
		return a, errors.New("native restore CAS refused")
	}
	boot, err := nativeRestoreBootProbe()
	if err != nil || boot != a.Identity.BootID {
		return a, errors.New("native restore boot changed")
	}
	a.State = next
	if err := publishNativeRestoreAttempt(a); err != nil {
		return expected, err
	}
	return a, nil
}

// ConfirmNativeRestoreAttempt does not send input or grant permissions. It joins
// fresh native hook/process evidence to this exact issued attempt and pane root.
func ConfirmNativeRestoreAttempt(a NativeRestoreAttempt, current Record) bool {
	id := a.Identity
	if a.State != "pending" || !validNativeRestoreAttempt(a) || current.Agent != "codex" ||
		current.Frontend != "" || current.SessionID != id.SessionID || current.HomeID != id.HomeID ||
		!hqpane.SameDir(current.Cwd, id.Cwd) || current.UpdatedAt < a.IssuedAt/int64(time.Second) {
		return false
	}
	r, ok := native.Load(id.SessionID, id.HomeID)
	now := nativeRestoreNow().Unix()
	if !ok || r.UpdatedAt < a.IssuedAt/int64(time.Second) ||
		(r.PID == a.PriorPID && r.ProcessStart == a.PriorBirth) ||
		!processCLIIdentity(transcript.NewCodexPathScope(), r, now) {
		return false
	}
	target, ok := processTarget(r, codexProcessPanes())
	if !ok || target.Pane != id.Pane || target.Loc != id.Loc || target.PanePID != id.RootPID ||
		!hqpane.SameDir(target.Cwd, id.Cwd) || strings.TrimPrefix(target.TTY, "/dev/") != strings.TrimPrefix(id.TTY, "/dev/") {
		return false
	}
	root, ok := nativeRestoreRootRead(id.RootPID)
	if !ok || root.PID != id.RootPID || root.Start != id.RootBirth ||
		strings.TrimPrefix(root.Comm, "-") != strings.TrimPrefix(id.Shell, "-") {
		return false
	}
	fresh, ok := processTarget(r, codexProcessPanes())
	if !ok || fresh != target {
		return false
	}
	_, err := UpdateNativeRestoreAttempt(a, "confirmed")
	return err == nil
}
