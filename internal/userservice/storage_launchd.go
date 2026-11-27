package userservice

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
	"howett.net/plist"
)

// StorageLaunchd parks only recognized gtmux profiles outside LaunchAgents.
// Temporary maintenance never changes launchd overrides.
type StorageLaunchd struct {
	Home       string
	Executable string
	Labels     []string
	Probe      func(string, ...string) ([]byte, error)
}

type StorageLaunchdJob struct {
	Label    string
	Identity state.StorageServiceProfileIdentity
	Digest   [32]byte
	Override [2]string
	Loaded   bool
	Running  bool
	Stage    string
	Reload   bool
	Kicked   bool
	Parked   bool   `json:"-"`
	PID      int    `json:"-"`
	Program  string `json:"-"`
}

func (m StorageLaunchd) probe(name string, args ...string) ([]byte, error) {
	if m.Probe != nil {
		return m.Probe(name, args...)
	}
	return storageProbe(name, args...)
}

func storageLaunchdDomains() [2]string {
	uid := strconv.Itoa(os.Getuid())
	return [2]string{"gui/" + uid, "user/" + uid}
}

func (m StorageLaunchd) CheckMenuBar() error {
	return stoppedLaunchdStorage(m.probe, Launchd{Home: m.Home}, strconv.Itoa(os.Getuid()), []string{"com.gtmux.menubar"})
}

func (m StorageLaunchd) Inspect() ([]StorageLaunchdJob, error) {
	var jobs []StorageLaunchdJob
	for _, label := range m.Labels {
		job, found, err := m.inspect(label)
		if err != nil {
			return nil, err
		}
		if found {
			jobs = append(jobs, job)
		}
	}
	return jobs, nil
}

func (m StorageLaunchd) inspect(label string) (StorageLaunchdJob, bool, error) {
	job := StorageLaunchdJob{Label: label}
	domains := storageLaunchdDomains()
	source := (Launchd{Home: m.Home}).Path(label)
	path, parked, identity, err := state.FindStorageServiceProfile(label)
	if err != nil {
		return job, false, err
	}
	if err := stoppedLaunchdJob(m.probe, domains[1], label); err != nil {
		return job, false, err
	}
	if path == "" {
		return job, false, stoppedLaunchdJob(m.probe, domains[0], label)
	}
	f, err := state.OpenPrivateRead(path)
	if err != nil {
		return job, false, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return job, false, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Nlink != 1 || uint64(stat.Dev) != identity.Device || uint64(stat.Ino) != identity.Inode {
		return job, false, fmt.Errorf("launchd profile %s changed during inspection", label)
	}
	const limit = 64 << 10
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	argv := storageProfileArguments(label, data)
	if err != nil || len(data) > limit || len(argv) == 0 {
		return job, false, fmt.Errorf("launchd profile %s does not match a supported gtmux installation", label)
	}
	match, held, confirmed, err := state.FindStorageServiceProfile(label)
	if err != nil || match != path || held != parked || confirmed != identity {
		return job, false, fmt.Errorf("launchd profile %s changed during inspection", label)
	}
	loaded, pid, err := m.loaded(domains[0], label, source, argv)
	if err != nil {
		return job, false, err
	}
	if label != "com.gtmux.tunnel" && !m.isCLI(argv[0]) {
		return job, false, fmt.Errorf("launchd profile %s uses an unrecognized gtmux path; for a verified custom installation (GTMUX_BIN_DIR), run doctor --fix using the same gtmux executable that registered the service", label)
	}
	job.Program = argv[0]
	job.Identity, job.Digest = identity, sha256.Sum256(data)
	job.Loaded, job.Running, job.PID, job.Parked = loaded, pid > 0, pid, parked
	for i, domain := range domains {
		output, err := m.probe("launchctl", "print-disabled", domain)
		if err != nil {
			return job, false, fmt.Errorf("could not read launchd settings in %s", domain)
		}
		value, known := launchdOverride(output, label)
		if !known {
			return job, false, fmt.Errorf("could not verify the launchd setting for %s/%s", domain, label)
		}
		job.Override[i] = value
	}
	return job, true, nil
}

// A registered path can point at an older CLI even if Doctor itself is current.
// The version command needs neither a writable layout nor a running service.
func (m StorageLaunchd) CheckPurposeExecutables(jobs []StorageLaunchdJob) error {
	for _, job := range jobs {
		if job.Label == "com.gtmux.tunnel" {
			continue
		}
		path := job.Program
		before, err := os.Stat(path)
		if err == nil && before.Mode().IsRegular() && before.Mode().Perm()&0111 != 0 && m.isCLI(path) {
			out, err := m.probe(path, "version", "--storage-format")
			after, statErr := os.Stat(path)
			if err == nil && statErr == nil && os.SameFile(before, after) && before.Size() == after.Size() &&
				before.ModTime().Equal(after.ModTime()) && string(out) == state.PurposeFormatCapability+"\n" {
				continue
			}
		}
		return fmt.Errorf(i18n.Tr("update the CLI registered for %s (%s) before storage migration", "请先更新 %s 已登记的 CLI（%s），再整理数据目录"), job.Label, path)
	}
	return nil
}

func storageProfileArguments(label string, data []byte) []string {
	var fields map[string]any
	if _, err := plist.Unmarshal(data, &fields); err != nil || fields["Label"] != label {
		return nil
	}
	for name, value := range fields {
		switch name {
		case "Label", "Program", "ProgramArguments", "RunAtLoad", "KeepAlive", "StandardOutPath", "StandardErrorPath":
		case "Disabled":
			if disabled, ok := value.(bool); !ok || disabled {
				return nil
			}
		default:
			return nil
		}
	}
	items, ok := fields["ProgramArguments"].([]any)
	if !ok {
		return nil
	}
	args := make([]string, 0, len(items))
	for _, item := range items {
		s, ok := item.(string)
		if !ok || s == "" || strings.ContainsAny(s, "\r\n") {
			return nil
		}
		args = append(args, s)
	}
	if len(args) < 2 || !filepath.IsAbs(args[0]) || filepath.Clean(args[0]) != args[0] {
		return nil
	}
	if program, present := fields["Program"]; present && program != args[0] {
		return nil
	}
	isPort := func(raw string) bool {
		n, err := strconv.ParseUint(raw, 10, 16)
		return err == nil && n > 0
	}
	switch label {
	case "com.gtmux.serve":
		if filepath.Base(args[0]) == "gtmux" && args[1] == "serve" && (len(args) == 2 ||
			len(args) == 4 && args[2] == "--port" && isPort(args[3]) ||
			len(args) == 6 && args[2] == "--bind" && (args[3] == "127.0.0.1" || args[3] == "0.0.0.0") &&
				args[4] == "--port" && isPort(args[5])) {
			return args
		}
	case "com.gtmux.selftunnel":
		if filepath.Base(args[0]) == "gtmux" && len(args) == 4 && args[1] == "tunnel-client" && args[2] == "--port" && isPort(args[3]) {
			return args
		}
	case "com.gtmux.tunnel":
		if validCloudflaredJob(args) {
			return args
		}
	}
	return nil
}

func (m StorageLaunchd) isCLI(path string) bool {
	if filepath.Base(path) != "gtmux" {
		return false
	}
	switch path {
	case m.Executable, filepath.Join(m.Home, ".local", "bin", "gtmux"),
		filepath.Join(m.Home, "Applications", "Gtmux.app", "Contents", "MacOS", "gtmux"),
		"/Applications/Gtmux.app/Contents/MacOS/gtmux", "/opt/homebrew/bin/gtmux", "/usr/local/bin/gtmux":
		return true
	}
	for _, prefix := range []string{"/opt/homebrew/Cellar/gtmux/", "/usr/local/Cellar/gtmux/"} {
		if rest, ok := strings.CutPrefix(path, prefix); ok {
			_, suffix, _ := strings.Cut(rest, "/")
			return suffix == "bin/gtmux"
		}
	}
	return false
}

func validCloudflaredJob(args []string) bool {
	if filepath.Base(args[0]) != "cloudflared" || args[1] != "tunnel" {
		return false
	}
	// Current installer, plus its earlier forms without metrics or protocol.
	rest := args[2:]
	if len(rest) >= 2 && rest[0] == "--metrics" {
		if rest[1] != "127.0.0.1:49317" {
			return false
		}
		rest = rest[2:]
	}
	if len(rest) == 5 && rest[0] == "run" && rest[1] == "--protocol" {
		if !slices.Contains([]string{"auto", "quic", "http2"}, rest[2]) {
			return false
		}
		return rest[3] == "--token"
	}
	return len(rest) == 3 && rest[0] == "run" && rest[1] == "--token"
}

func (m StorageLaunchd) loaded(domain, label, path string, expected []string) (bool, int, error) {
	address := domain + "/" + label
	output, err := m.probe("launchctl", "print", address)
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 113 &&
			strings.Contains(string(output), "Could not find service "+strconv.Quote(label)+" in domain ") {
			return false, 0, nil
		}
		return false, 0, fmt.Errorf("could not inspect launchd job %s", address)
	}
	var source, program string
	var pid int
	var args []string
	inArgs, seenArgs := false, false
	for _, line := range strings.Split(string(output), "\n") {
		if inArgs {
			if line == "\t}" {
				inArgs = false
				continue
			}
			arg, ok := strings.CutPrefix(line, "\t\t")
			if !ok {
				break
			}
			args = append(args, arg)
			continue
		}
		if value, ok := strings.CutPrefix(line, "\tpath = "); ok {
			if source != "" {
				return false, 0, fmt.Errorf("launchd job %s has ambiguous source paths", address)
			}
			source = value
		}
		if value, ok := strings.CutPrefix(line, "\tprogram = "); ok {
			if program != "" {
				return false, 0, fmt.Errorf("launchd job %s has ambiguous programs", address)
			}
			program = value
		}
		if value, ok := strings.CutPrefix(line, "\tpid = "); ok {
			pid, _ = strconv.Atoi(value)
		}
		if line == "\targuments = {" {
			if seenArgs {
				return false, 0, fmt.Errorf("launchd job %s has ambiguous arguments", address)
			}
			inArgs, seenArgs = true, true
		}
	}
	if inArgs || source != path || program != expected[0] || !slices.Equal(args, expected) {
		return false, 0, fmt.Errorf("launchd job %s does not match its installed gtmux definition", address)
	}
	return true, pid, nil
}

func StorageLaunchdPIDs(jobs []StorageLaunchdJob) []int {
	var pids []int
	for _, job := range jobs {
		if job.Loaded && job.PID > 0 {
			pids = append(pids, job.PID)
		}
	}
	return pids
}

func SameStorageLaunchdSnapshot(before, after []StorageLaunchdJob) bool {
	return slices.EqualFunc(before, after, func(a, b StorageLaunchdJob) bool {
		a.PID, b.PID = 0, 0
		return a == b
	})
}

var errStorageServiceRemoved = errors.New("launchd profile was removed")

func (m StorageLaunchd) current(job StorageLaunchdJob) (StorageLaunchdJob, error) {
	now, found, err := m.inspect(job.Label)
	if err != nil {
		return now, err
	}
	if !found {
		return now, errStorageServiceRemoved
	}
	if now.Identity != job.Identity || now.Digest != job.Digest {
		return now, fmt.Errorf("the launchd profile for %s changed", job.Label)
	}
	return now, nil
}

func (m StorageLaunchd) mutate(args ...string) error {
	if _, err := m.probe("launchctl", args...); err != nil {
		return fmt.Errorf("launchctl %s failed: %w", strings.Join(args, " "), err)
	}
	return nil
}

func saveStorageJobStage(job *StorageLaunchdJob, stage string, save func() error) error {
	job.Stage = stage
	return save()
}

// Stop unloads verified jobs while their original profiles are still at their
// registered paths. The durable reload intent precedes both unload and parking.
func (m StorageLaunchd) Stop(jobs []StorageLaunchdJob, save func() error) error {
	for i := len(jobs) - 1; i >= 0; i-- {
		job := &jobs[i]
		if job.Stage == "changed" {
			continue
		}
		now, err := m.current(*job)
		if errors.Is(err, errStorageServiceRemoved) && (job.Stage == "" || job.Stage == "stopping") {
			if err := saveStorageJobStage(job, "changed", save); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if now.Override != job.Override || now.Loaded && (!job.Loaded || now.Parked) {
			return fmt.Errorf("launchd settings changed for %s; verify the latest choice", job.Label)
		}
		switch job.Stage {
		case "":
			if now.Parked || now.Loaded != job.Loaded || now.Running != job.Running {
				return fmt.Errorf("launchd state changed for %s; review the migration again", job.Label)
			}
			job.Reload = job.Loaded
			if err := saveStorageJobStage(job, "stopping", save); err != nil {
				return err
			}
		case "stopping", "parking":
		case "paused":
			if !now.Parked {
				return fmt.Errorf("the paused profile %s was moved outside Doctor", job.Label)
			}
		default:
			return fmt.Errorf("launchd profile %s has an invalid pause phase", job.Label)
		}
		if now.Loaded {
			if err := m.mutate("bootout", storageLaunchdDomains()[0]+"/"+job.Label); err != nil {
				return err
			}
			now, err = m.current(*job)
			if err != nil {
				return err
			}
			if now.Loaded {
				return fmt.Errorf("launchd job %s is still loaded", job.Label)
			}
		}
	}
	return nil
}

// Park follows exclusive publication of the compatibility fence. It never
// unloads a job or changes its recorded reload intent.
func (m StorageLaunchd) Park(jobs []StorageLaunchdJob, save func() error) error {
	for i := len(jobs) - 1; i >= 0; i-- {
		job := &jobs[i]
		if job.Stage == "changed" {
			continue
		}
		now, err := m.current(*job)
		if err != nil {
			return err
		}
		if now.Override != job.Override || now.Loaded {
			return fmt.Errorf("launchd job %s changed before its profile could be parked", job.Label)
		}
		switch job.Stage {
		case "stopping":
			if err := saveStorageJobStage(job, "parking", save); err != nil {
				return err
			}
		case "parking", "paused":
		default:
			return fmt.Errorf("launchd profile %s has an invalid parking phase", job.Label)
		}
		if err := state.MoveStorageServiceProfile(job.Label, job.Identity, true); err != nil {
			return err
		}
		if err := saveStorageJobStage(job, "paused", save); err != nil {
			return err
		}
	}
	return nil
}

// ReturnProfiles completes name recovery before any CLI can be admitted again.
func (m StorageLaunchd) ReturnProfiles(jobs []StorageLaunchdJob, save func() error) error {
	var result error
	for i := range jobs {
		if err := m.returnProfile(&jobs[i], save); err != nil {
			result = errors.Join(result, err)
		}
	}
	return result
}

func (m StorageLaunchd) returnProfile(job *StorageLaunchdJob, save func() error) error {
	switch job.Stage {
	case "", "done", "changed":
		return nil
	case "stopping", "parking", "paused", "returning", "returned":
	default:
		return fmt.Errorf("launchd profile %s has an invalid return phase", job.Label)
	}
	now, err := m.current(*job)
	if errors.Is(err, errStorageServiceRemoved) && (job.Stage == "stopping" || job.Stage == "returned" ||
		job.Stage == "returning" && state.CheckStorageLayout() == nil) {
		// An old CLI can legitimately remove a profile before the fence
		// starts or after it is lifted. Never recreate a removed profile.
		return saveStorageJobStage(job, "changed", save)
	}
	if err != nil {
		return err
	}
	if now.Override != job.Override {
		job.Reload = false
	}
	if job.Stage != "returned" {
		if err := saveStorageJobStage(job, "returning", save); err != nil {
			return err
		}
		if now.Parked {
			if err := state.MoveStorageServiceProfile(job.Label, job.Identity, false); err != nil {
				return err
			}
		} else if err := state.SyncPrivateDirectory(filepath.Dir((Launchd{Home: m.Home}).Path(job.Label))); err != nil {
			return err
		}
		if err := state.SyncPrivateDirectory(state.StorageServiceRecoveryDir()); err != nil {
			return err
		}
		if err := saveStorageJobStage(job, "returned", save); err != nil {
			return err
		}
	}
	return nil
}

func (m StorageLaunchd) Restore(jobs []StorageLaunchdJob, save func() error) error {
	var result error
	serveReady := true
	for i := range jobs {
		job := &jobs[i]
		if job.Stage == "" || job.Stage == "done" || job.Stage == "changed" {
			continue
		}
		if job.Label != "com.gtmux.serve" && job.Reload && !serveReady {
			result = errors.Join(result, fmt.Errorf("%s is waiting for com.gtmux.serve", job.Label))
			continue
		}
		if err := m.restoreJob(job, save); err != nil {
			if job.Label == "com.gtmux.serve" {
				serveReady = false
			}
			result = errors.Join(result, err)
		}
	}
	return result
}

func (m StorageLaunchd) restoreJob(job *StorageLaunchdJob, save func() error) error {
	if job.Stage != "returned" {
		return fmt.Errorf("launchd profile %s has not been returned", job.Label)
	}
	now, err := m.current(*job)
	if errors.Is(err, errStorageServiceRemoved) {
		return saveStorageJobStage(job, "changed", save)
	}
	if err != nil {
		return err
	}
	if now.Parked {
		return fmt.Errorf("launchd profile %s is still parked", job.Label)
	}
	if now.Override != job.Override {
		return saveStorageJobStage(job, "changed", save)
	}
	if !job.Reload {
		return saveStorageJobStage(job, "done", save)
	}
	gui := storageLaunchdDomains()[0]
	if !now.Loaded {
		if err := m.mutate("bootstrap", gui, (Launchd{Home: m.Home}).Path(job.Label)); err != nil {
			return err
		}
	}
	for attempt := 0; attempt < 2; attempt++ {
		now, err = m.current(*job)
		if err != nil {
			return err
		}
		if now.Override != job.Override {
			return saveStorageJobStage(job, "changed", save)
		}
		if now.Loaded && (!job.Running || now.Running) {
			return saveStorageJobStage(job, "done", save)
		}
		if !now.Loaded || job.Kicked {
			break
		}
		job.Kicked = true
		if err := save(); err != nil {
			return err
		}
		if err := m.mutate("kickstart", gui+"/"+job.Label); err != nil {
			return err
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("launchd job %s needs attention: check launchctl print %s/%s and rerun Doctor", job.Label, gui, job.Label)
}
