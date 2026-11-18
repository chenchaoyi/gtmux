package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentenv"
	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

type nativeRestoreBinary struct {
	Lookup, Physical, SHA, PackageSHA string
	Device, Inode                     uint64
	Mode, UID                         uint32
	Size, MTime                       int64
}
type nativeRestorePaneState struct {
	Pane, Loc, Cwd, TTY, Command, RootBirth, RootCommand string
	RootPID                                              int
}
type nativeRestoreCapability int

const (
	nativeCapabilityUnknown nativeRestoreCapability = iota
	nativeCapabilityUnsupported
	nativeCapabilitySupported
)

type nativeRestorePlan struct {
	Selection      *nativeRestoreSelection
	Identity       resume.NativeRestoreIdentity
	Binary         nativeRestoreBinary
	Pane           nativeRestorePaneState
	Record, Before resume.Record
	Incumbent      resume.NativeRestoreIncumbent
	Command        string
}

var nativeRestoreReadIncumbent = resume.ReadNativeRestoreIncumbent
var nativeRestoreReadProcess = launchhost.ReadAgent
var nativeRestoreFindBinary = nativeCodexBinary
var nativeRestoreReadPane = readNativeRestorePane
var nativeRestoreInspectBinary = inspectNativeRestoreBinary
var nativeRestoreProbe = probeNativeRestoreCapabilityContext
var nativeRestoreHelp = runNativeRestoreHelp
var nativeRestoreBoot = resume.NativeRestoreBootID
var nativeRestoreReserve = resume.ReserveNativeRestoreAttempt
var nativeRestoreUpdate = resume.UpdateNativeRestoreAttempt
var nativeRestoreSend = tmux.SendText

func readNativeRestorePane(pane, loc string) (nativeRestorePaneState, bool) {
	var p nativeRestorePaneState
	raw, err := tmux.Run("display-message", "-p", "-t", pane,
		"#{pane_id}\t#{session_name}:#{window_index}.#{pane_index}\t#{pane_pid}\t#{pane_tty}\t#{pane_current_path}\t#{pane_current_command}")
	fields := strings.Split(strings.TrimSuffix(raw, "\n"), "\t")
	if err != nil || len(fields) != 6 || fields[0] != pane || fields[1] != loc || !isShellCommand(fields[5]) {
		return p, false
	}
	pid, err := strconv.Atoi(fields[2])
	if err != nil || pid <= 1 || fields[3] == "" || fields[4] == "" {
		return p, false
	}
	root, ok := nativeRestoreReadProcess(pid)
	if !ok || root.PID != pid || root.Start == "" || !isShellCommand(root.Comm) {
		return p, false
	}
	p = nativeRestorePaneState{Pane: pane, Loc: loc, RootPID: pid, TTY: fields[3],
		Cwd: fields[4], Command: fields[5], RootBirth: root.Start, RootCommand: root.Comm}
	return p, true
}

type nativeRestoreOutput struct {
	body     bytes.Buffer
	limit    int
	overflow bool
}

func (b *nativeRestoreOutput) Write(p []byte) (int, error) {
	if b.body.Len()+len(p) > b.limit {
		b.overflow = true
		return len(p), nil
	}
	_, _ = b.body.Write(p)
	return len(p), nil
}
func runNativeRestoreHelp(binary string, args ...string) (string, bool) {
	return runNativeRestoreHelpContext(context.Background(), binary, args...)
}
func runNativeRestoreHelpContext(parent context.Context, binary string, args ...string) (string, bool) {
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.WaitDelay = 100 * time.Millisecond
	out := nativeRestoreOutput{limit: 128 << 10}
	stderr := nativeRestoreOutput{limit: 32 << 10}
	cmd.Stdout, cmd.Stderr = &out, &stderr
	err := cmd.Run()
	return out.body.String(), err == nil && ctx.Err() == nil && !out.overflow && !stderr.overflow
}
func completeNativeRestoreHelp(text, usage string) bool {
	return strings.Contains(text, "Usage: "+usage) && strings.Contains(text, "\nOptions:\n") &&
		strings.Contains(text, "--help") && strings.Contains(text, "--version") &&
		strings.HasSuffix(strings.TrimSpace(text), "Print version")
}
func nativeRestoreOption(text, option string) bool {
	_, options, ok := strings.Cut(text, "\nOptions:\n")
	if !ok {
		return false
	}
	for _, line := range strings.Split(options, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == option {
			return true
		}
	}
	return false
}
func probeNativeRestoreCapability(binary nativeRestoreBinary) nativeRestoreCapability {
	return probeNativeRestoreCapabilityWithHelp(binary, nativeRestoreHelp)
}
func probeNativeRestoreCapabilityContext(ctx context.Context, binary nativeRestoreBinary) nativeRestoreCapability {
	return probeNativeRestoreCapabilityWithHelp(binary, func(path string, args ...string) (string, bool) {
		if ctx.Err() != nil {
			return "", false
		}
		return runNativeRestoreHelpContext(ctx, path, args...)
	})
}
func probeNativeRestoreCapabilityWithHelp(binary nativeRestoreBinary, helpFn func(string, ...string) (string, bool)) nativeRestoreCapability {
	root, ok := helpFn(binary.Physical, "--help")
	if !ok || !completeNativeRestoreHelp(root, "codex [OPTIONS]") ||
		!strings.Contains(root, "\nCommands:\n") {
		return nativeCapabilityUnknown
	}
	hasResume := false
	for _, line := range strings.Split(root, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == "resume" {
			hasResume = true
		}
	}
	help, ok := helpFn(binary.Physical, "resume", "--help")
	if !ok || !hasResume || !completeNativeRestoreHelp(help, "codex resume [OPTIONS]") ||
		!strings.Contains(help, "[SESSION_ID]") {
		return nativeCapabilityUnknown
	}
	rootFlag, resumeFlag := nativeRestoreOption(root, "--no-daemon"), nativeRestoreOption(help, "--no-daemon")
	if rootFlag != resumeFlag {
		return nativeCapabilityUnknown
	}
	if !rootFlag {
		return nativeCapabilityUnsupported
	}
	combined, ok := helpFn(binary.Physical, "--no-daemon", "resume", "--help")
	if !ok || !completeNativeRestoreHelp(combined, "codex resume [OPTIONS]") ||
		!nativeRestoreOption(combined, "--no-daemon") {
		return nativeCapabilityUnknown
	}
	return nativeCapabilitySupported
}

// Only the existing standalone-package layout is classified here. A same-name
// wrapper/script or explicit launcher retains the old command contract.
func inspectNativeRestoreBinary(lookup string) (nativeRestoreBinary, bool, error) {
	var result nativeRestoreBinary
	if lookup == "" {
		return result, true, errors.New("native executable lookup unavailable")
	}
	absolute, err := filepath.Abs(lookup)
	if err != nil {
		return result, true, err
	}
	physical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return result, true, err
	}
	entry, err := os.Stat(physical)
	if err != nil {
		return result, true, err
	}
	if !entry.Mode().IsRegular() || entry.Mode()&0111 == 0 {
		return result, true, errors.New("resolved executable is not executable regular file")
	}
	packageRoot := filepath.Dir(filepath.Dir(physical))
	if filepath.Base(physical) != "codex" || filepath.Base(filepath.Dir(physical)) != "bin" ||
		!strings.Contains(filepath.ToSlash(packageRoot), "/packages/standalone/releases/") {
		return result, false, nil
	}
	metaPath := filepath.Join(packageRoot, "codex-package.json")
	metaFile, err := os.Open(metaPath)
	if err != nil {
		return result, true, errors.New("native package metadata unavailable")
	}
	meta, readErr := io.ReadAll(io.LimitReader(metaFile, 4097))
	_ = metaFile.Close()
	if readErr != nil || len(meta) > 4096 {
		return result, true, errors.New("native package metadata unavailable")
	}
	var pkg struct {
		LayoutVersion                        int
		Version, Target, Variant, Entrypoint string
	}
	if json.Unmarshal(meta, &pkg) != nil || pkg.LayoutVersion != 1 || pkg.Version == "" ||
		pkg.Variant != "codex" || pkg.Entrypoint != "bin/codex" {
		return result, true, errors.New("native package metadata invalid")
	}
	wantArch := map[string]string{"arm64": "aarch64", "amd64": "x86_64"}[runtime.GOARCH]
	if wantArch == "" || !strings.HasPrefix(pkg.Target, wantArch+"-") ||
		(runtime.GOOS == "darwin" && !strings.HasSuffix(pkg.Target, "-apple-darwin")) ||
		(runtime.GOOS == "linux" && !strings.Contains(pkg.Target, "-linux-")) {
		return result, true, errors.New("native package target unavailable")
	}
	f, err := os.Open(physical)
	if err != nil {
		return result, true, err
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil {
		return result, true, err
	}
	if !os.SameFile(entry, before) || entry.Mode() != before.Mode() || entry.Size() != before.Size() ||
		entry.ModTime() != before.ModTime() || !before.Mode().IsRegular() || before.Mode()&0111 == 0 ||
		before.Size() < 4 || before.Size() > 512<<20 {
		return result, true, errors.New("native executable attributes unavailable")
	}
	var header [4]byte
	if _, err := io.ReadFull(f, header[:]); err != nil {
		return result, true, err
	}
	macho := header == [4]byte{0xcf, 0xfa, 0xed, 0xfe} || header == [4]byte{0xce, 0xfa, 0xed, 0xfe}
	elf := header == [4]byte{0x7f, 'E', 'L', 'F'}
	if (runtime.GOOS == "darwin" && !macho) || (runtime.GOOS == "linux" && !elf) {
		return result, false, nil
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return result, true, err
	}
	digest := sha256.New()
	n, err := io.Copy(digest, io.LimitReader(f, (512<<20)+1))
	after, statErr := f.Stat()
	current, pathErr := os.Stat(physical)
	if err != nil || statErr != nil || pathErr != nil || n != before.Size() ||
		!os.SameFile(before, after) || !os.SameFile(before, current) ||
		before.Size() != after.Size() || before.Size() != current.Size() ||
		before.ModTime() != after.ModTime() || before.ModTime() != current.ModTime() ||
		before.Mode() != after.Mode() || before.Mode() != current.Mode() {
		return result, true, errors.New("native executable changed")
	}
	st, ok := before.Sys().(*syscall.Stat_t)
	if !ok {
		return result, true, errors.New("native executable identity unavailable")
	}
	currentOwner, ownerOK := current.Sys().(*syscall.Stat_t)
	if !ownerOK || currentOwner.Uid != st.Uid {
		return result, true, errors.New("native executable owner changed")
	}
	pkgSum := sha256.Sum256(meta)
	result = nativeRestoreBinary{Lookup: absolute, Physical: physical, Device: uint64(st.Dev),
		Inode: st.Ino, Mode: uint32(before.Mode()), UID: st.Uid, Size: n, MTime: before.ModTime().UnixNano(), SHA: hex.EncodeToString(digest.Sum(nil)),
		PackageSHA: hex.EncodeToString(pkgSum[:])}
	return result, true, nil
}

type nativeRestoreSelection struct {
	Path           string
	Saved          []byte
	ReadError      error
	Incumbent      resume.NativeRestoreIncumbent
	IncumbentError error
}

func (s *nativeRestoreSelection) unchanged(loc string) bool {
	if s == nil {
		return true
	}
	if s.ReadError != nil || s.IncumbentError != nil || resurrectLastSave() != s.Path {
		return false
	}
	body, err := readRestoreSave(s.Path)
	if err != nil || !bytes.Equal(body, s.Saved) {
		return false
	}
	current, err := nativeRestoreReadIncumbent(loc)
	return err == nil && current == s.Incumbent
}

func nativeRestoreRevision(rec resume.Record, incumbent resume.NativeRestoreIncumbent) (string, error) {
	path := resurrectLastSave()
	saved, err := readRestoreSave(path)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(struct {
		Path      string
		Saved     []byte
		Record    resume.Record
		Incumbent resume.NativeRestoreIncumbent
	}{path, saved, rec, incumbent})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func prepareNativeRestore(pane, loc string, rec resume.Record, cache map[nativeRestoreBinary]nativeRestoreCapability) (*nativeRestorePlan, bool, string) {
	return prepareNativeRestoreContext(context.Background(), pane, loc, rec, cache)
}
func prepareNativeRestoreContext(ctx context.Context, pane, loc string, rec resume.Record, cache map[nativeRestoreBinary]nativeRestoreCapability) (*nativeRestorePlan, bool, string) {
	if ctx.Err() != nil {
		return nil, true, "cancelled"
	}
	if rec.Agent != "codex" || rec.Launcher != "" || os.Getenv("CODEX_EXEC_SERVER_URL") != "" {
		return nil, false, ""
	}
	binary, handled, err := nativeRestoreInspectBinary(nativeRestoreFindBinary())
	// Errors must win over the classifier's handled bit: only (false,nil) is compatibility.
	if err != nil {
		restoreLogf("restore.resume", "native prepare deferred pane=%s loc=%s executable: %v", pane, loc, err)
		return nil, true, "executable-unavailable"
	}
	if !handled {
		return nil, false, ""
	}
	p, ok := nativeRestoreReadPane(pane, loc)
	if !ok {
		return nil, true, "target-changed"
	}
	home, err := filepath.EvalSymlinks(transcript.CodexHome())
	if err != nil || rec.HomeID != transcript.CodexHomeIDForPath(home) {
		return nil, true, "foreign-home"
	}
	incumbent, err := nativeRestoreReadIncumbent(loc)
	if err != nil {
		restoreLogf("restore.resume", "native prepare deferred pane=%s loc=%s incumbent: %v", pane, loc, err)
		return nil, true, "incumbent-unavailable"
	}
	before := incumbent.Record
	revision, err := nativeRestoreRevision(rec, incumbent)
	if err != nil {
		return nil, true, "target-changed"
	}
	boot, err := nativeRestoreBoot()
	if err != nil {
		return nil, true, "capability-unknown"
	}
	capability, exists := cache[binary]
	if !exists {
		capability = nativeRestoreProbe(ctx, binary)
		cache[binary] = capability
	}
	if ctx.Err() != nil {
		return nil, true, "cancelled"
	}
	if capability == nativeCapabilityUnknown {
		return nil, true, "capability-unknown"
	}
	cwd, err := filepath.EvalSymlinks(rec.Cwd)
	if err != nil {
		return nil, true, "target-changed"
	}
	cwdInfo, err := os.Stat(cwd)
	if err != nil || !cwdInfo.IsDir() {
		return nil, true, "target-changed"
	}
	commandRecord := rec
	commandRecord.Cwd = cwd
	_, command, err := resume.NativeRestoreCommand(commandRecord, binary.Physical, home, capability == nativeCapabilitySupported, agentenv.Active())
	if err != nil {
		return nil, true, "target-changed"
	}
	id := resume.NativeRestoreIdentity{BootID: boot, Home: home, HomeID: rec.HomeID,
		SessionID: rec.SessionID, SaveRevision: revision, Pane: pane, Loc: loc,
		RootPID: p.RootPID, RootBirth: p.RootBirth, Shell: p.RootCommand,
		InitialCwd: p.Cwd, Cwd: cwd, TTY: p.TTY, BinaryPath: binary.Physical,
		LookupPath: binary.Lookup, BinarySHA: binary.SHA, PackageSHA: binary.PackageSHA,
		BinaryDevice: binary.Device, BinaryInode: binary.Inode, BinaryMode: binary.Mode, BinaryUID: binary.UID, BinarySize: binary.Size, BinaryMTime: binary.MTime}
	plan := &nativeRestorePlan{Identity: id, Binary: binary, Pane: p, Record: rec, Before: before, Incumbent: incumbent, Command: command}
	if why := nativeRestorePlanState(plan); why != "" {
		return nil, true, why
	}
	return plan, true, ""
}

func nativeRestorePlanState(p *nativeRestorePlan) string {
	if !p.Selection.unchanged(p.Pane.Loc) {
		return "target-changed"
	}
	binary, handled, err := nativeRestoreInspectBinary(p.Binary.Lookup)
	if err != nil {
		restoreLogf("restore.resume", "native write deferred pane=%s loc=%s executable: %v", p.Pane.Pane, p.Pane.Loc, err)
		return "executable-unavailable"
	}
	if !handled || binary != p.Binary {
		return "target-changed"
	}
	boot, err := nativeRestoreBoot()
	if err != nil || boot != p.Identity.BootID {
		return "target-changed"
	}
	home, err := filepath.EvalSymlinks(transcript.CodexHome())
	if err != nil || home != p.Identity.Home || transcript.CodexHomeID() != p.Record.HomeID {
		return "target-changed"
	}
	cwd, err := filepath.EvalSymlinks(p.Record.Cwd)
	if err != nil || cwd != p.Identity.Cwd {
		return "target-changed"
	}
	info, err := os.Stat(cwd)
	if err != nil || !info.IsDir() {
		return "target-changed"
	}
	record, exists := resume.Resolve(p.Record)
	if !exists || record != p.Record {
		return "target-changed"
	}
	incumbent, err := nativeRestoreReadIncumbent(p.Pane.Loc)
	if err != nil {
		restoreLogf("restore.resume", "native write deferred pane=%s loc=%s incumbent: %v", p.Pane.Pane, p.Pane.Loc, err)
		return "incumbent-unavailable"
	}
	if incumbent != p.Incumbent {
		return "target-changed"
	}
	revision, err := nativeRestoreRevision(p.Record, incumbent)
	if err != nil || revision != p.Identity.SaveRevision {
		return "target-changed"
	}
	if remoteRestoreSessions != nil {
		expected, allowed := remoteRestoreTargets[p.Pane.Loc]
		if !allowed || !remoteRestoreSessions[locSession(p.Pane.Loc)] || expected.Agent != p.Record.Agent ||
			expected.SessionID != p.Record.SessionID || (expected.HomeID != "" && expected.HomeID != p.Record.HomeID) {
			return "target-changed"
		}
	}
	current, ok := nativeRestoreReadPane(p.Pane.Pane, p.Pane.Loc)
	if !ok || current != p.Pane {
		return "target-changed"
	}
	return ""
}

func issueNativeRestore(p *nativeRestorePlan, mode resumeMode, used map[string]bool) (resumeAttempt, bool) {
	return issueNativeRestoreContext(context.Background(), p, mode, used)
}
func issueNativeRestoreContext(ctx context.Context, p *nativeRestorePlan, mode resumeMode, used map[string]bool) (resumeAttempt, bool) {
	result := resumeAttempt{Pane: p.Pane.Pane, Loc: p.Pane.Loc, Record: p.Record, Before: p.Before,
		IssuedAt: time.Now().Unix(), State: "target-changed"}
	if ctx.Err() != nil {
		result.State = "cancelled"
		return result, false
	}
	if used[p.Record.SessionID] {
		return result, false
	}
	if why := nativeRestorePlanState(p); why != "" {
		result.State = why
		return result, false
	}
	if ctx.Err() != nil {
		result.State = "cancelled"
		return result, false
	}
	a, err := nativeRestoreReserve(p.Identity)
	if err != nil {
		result.State = "already-attempted"
		if !errors.Is(err, resume.ErrNativeRestoreReserved) {
			result.State = "reservation-unavailable"
		}
		return result, false
	}
	result.Native = &a
	used[p.Record.SessionID] = true // never release after a partially successful or uncertain write
	if why := nativeRestorePlanState(p); why != "" {
		result.State = why
		if updated, err := nativeRestoreUpdate(a, "not-sent"); err == nil {
			result.Native = &updated
		}
		return result, false
	}
	if ctx.Err() != nil {
		result.State = "cancelled"
		if updated, err := nativeRestoreUpdate(a, "not-sent"); err == nil {
			result.Native = &updated
		}
		return result, false
	}
	if err := nativeRestoreSend(p.Pane.Pane, p.Command, mode == resumeAuto); err != nil {
		result.State = "unknown"
		if updated, err := nativeRestoreUpdate(a, "unknown"); err == nil {
			result.Native = &updated
		}
		return result, false
	}
	next := "pending"
	if mode == resumeType {
		next = "prefilled"
	}
	updated, err := nativeRestoreUpdate(a, next)
	if err != nil {
		result.State = "unknown" // reserved remains on disk; never replay a successful send
		return result, true
	}
	result.Native, result.State = &updated, next
	return result, true
}

func tryNativeRestore(pane, loc string, rec resume.Record, cache map[nativeRestoreBinary]nativeRestoreCapability, mode resumeMode, used map[string]bool) (resumeAttempt, bool, bool) {
	return tryNativeRestoreContext(context.Background(), pane, loc, rec, cache, mode, used)
}
func tryNativeRestoreContext(ctx context.Context, pane, loc string, rec resume.Record, cache map[nativeRestoreBinary]nativeRestoreCapability, mode resumeMode, used map[string]bool, selections ...*nativeRestoreSelection) (resumeAttempt, bool, bool) {
	var selected *nativeRestoreSelection
	if len(selections) > 0 {
		selected = selections[0]
	}
	if !selected.unchanged(loc) {
		return resumeAttempt{Pane: pane, Loc: loc, Record: rec, State: "target-changed"}, true, false
	}
	plan, handled, why := prepareNativeRestoreContext(ctx, pane, loc, rec, cache)
	if !handled {
		return resumeAttempt{}, false, false
	}
	if plan == nil {
		return resumeAttempt{Pane: pane, Loc: loc, Record: rec, State: why}, true, false
	}
	plan.Selection = selected
	attempt, sent := issueNativeRestoreContext(ctx, plan, mode, used)
	return attempt, true, sent
}
