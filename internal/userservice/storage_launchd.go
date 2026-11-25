package userservice

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// StorageLaunchd controls only the installed gtmux LaunchAgents passed to it.
// The original plan is persisted by Doctor before any changes to launchd.
type StorageLaunchd struct {
	Home   string
	Labels []string
	Probe  func(string, ...string) ([]byte, error)
}

type StorageLaunchdJob struct {
	Label    string
	Profile  [32]byte
	Disabled [2]bool // GUI and user domain, respectively
	Loaded   bool
	Running  bool
	PID      int `json:"-"`
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

func (m StorageLaunchd) Inspect() ([]StorageLaunchdJob, error) {
	manager := Launchd{Home: m.Home}
	domains := storageLaunchdDomains()
	var output [2][]byte
	var jobs []StorageLaunchdJob
	for _, label := range m.Labels {
		path := manager.Path(label)
		digest, installed, err := storageLaunchAgentProfile(path)
		if err != nil {
			return nil, err
		}
		if err := stoppedLaunchdJob(m.probe, domains[1], label); err != nil {
			return nil, err
		}
		loaded, pid, err := m.loaded(domains[0], label, path, installed)
		if err != nil {
			return nil, err
		}
		if !installed {
			continue
		}
		job := StorageLaunchdJob{Label: label, Profile: digest, Loaded: loaded, Running: pid > 0, PID: pid}
		for i, domain := range domains {
			if output[i] == nil {
				output[i], err = m.probe("launchctl", "print-disabled", domain)
				if err != nil {
					return nil, fmt.Errorf("could not read launchd settings in %s", domain)
				}
			}
			disabled, known := launchdDisabledState(output[i], label)
			if !known {
				return nil, fmt.Errorf("could not verify the original launchd setting for %s/%s", domain, label)
			}
			job.Disabled[i] = disabled
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func storageLaunchAgentProfile(path string) ([32]byte, bool, error) {
	var empty [32]byte
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return empty, false, nil
	}
	if err != nil {
		return empty, false, err
	}
	const limit = 64 << 10
	// Preserve an independently edited profile. Never bootstrap one whose
	// identity, type or contents could not be read safely.
	if !info.Mode().IsRegular() || info.Size() > limit {
		return empty, false, fmt.Errorf("launchd profile requires review: %s", path)
	}
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return empty, false, err
	}
	file := os.NewFile(uintptr(fd), path)
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return empty, false, fmt.Errorf("launchd profile changed during inspection: %s", path)
	}
	if stat, ok := opened.Sys().(*syscall.Stat_t); !ok || int(stat.Uid) != os.Getuid() {
		return empty, false, fmt.Errorf("launchd profile ownership requires review: %s", path)
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || len(data) > limit {
		return empty, false, fmt.Errorf("could not read launchd profile: %s", path)
	}
	return sha256.Sum256(data), true, nil
}

func (m StorageLaunchd) loaded(domain, label, path string, installed bool) (bool, int, error) {
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
	var source string
	var pid int
	for _, line := range strings.Split(string(output), "\n") {
		if value, ok := strings.CutPrefix(line, "\tpath = "); ok {
			source = value
		}
		if value, ok := strings.CutPrefix(line, "\tpid = "); ok {
			pid, _ = strconv.Atoi(value)
		}
	}
	if !installed || source == "" || filepath.Clean(source) != path {
		return false, 0, fmt.Errorf("launchd job %s does not match its installed profile", address)
	}
	return true, pid, nil
}

func verifyStorageLaunchdPlan(original, current []StorageLaunchdJob) error {
	if len(original) != len(current) {
		return errors.New("the installed gtmux launchd jobs changed")
	}
	for i, job := range original {
		now := current[i]
		if now.Label != job.Label || now.Profile != job.Profile || now.Loaded && !job.Loaded ||
			job.Loaded && job.Disabled[0] {
			return fmt.Errorf("the launchd job %s needs review before its settings can be restored", job.Label)
		}
	}
	return nil
}

func (m StorageLaunchd) mutate(args ...string) error {
	if _, err := m.probe("launchctl", args...); err != nil {
		return fmt.Errorf("launchctl %s failed: %w", strings.Join(args, " "), err)
	}
	return nil
}

func (m StorageLaunchd) Pause(original []StorageLaunchdJob, checkProcesses func([]int) error) error {
	current, err := m.Inspect()
	if err != nil {
		return err
	}
	if err := verifyStorageLaunchdPlan(original, current); err != nil {
		return err
	}
	if err := checkProcesses(StorageLaunchdPIDs(current)); err != nil {
		return err
	}
	domains := storageLaunchdDomains()
	for _, job := range current {
		for i, domain := range domains {
			if !job.Disabled[i] {
				if err := m.mutate("disable", domain+"/"+job.Label); err != nil {
					return err
				}
			}
		}
	}
	for i := len(current) - 1; i >= 0; i-- {
		if current[i].Loaded {
			if err := m.mutate("bootout", domains[0]+"/"+current[i].Label); err != nil {
				return err
			}
		}
	}
	return nil
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

func (m StorageLaunchd) Restore(original []StorageLaunchdJob) error {
	current, err := m.Inspect()
	if err != nil {
		return err
	}
	if err := verifyStorageLaunchdPlan(original, current); err != nil {
		return err
	}
	domains := storageLaunchdDomains()
	for i, job := range original {
		for j, domain := range domains {
			if current[i].Disabled[j] != job.Disabled[j] {
				verb := "enable"
				if job.Disabled[j] {
					verb = "disable"
				}
				if err := m.mutate(verb, domain+"/"+job.Label); err != nil {
					return err
				}
			}
		}
		if job.Loaded && !current[i].Loaded {
			if err := m.mutate("bootstrap", domains[0], (Launchd{Home: m.Home}).Path(job.Label)); err != nil {
				return err
			}
		}
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		current, err = m.Inspect()
		if err != nil {
			return err
		}
		if err := verifyStorageLaunchdPlan(original, current); err != nil {
			return err
		}
		var waiting string
		for i, job := range original {
			if current[i].Disabled != job.Disabled || current[i].Loaded != job.Loaded {
				return fmt.Errorf("could not verify restored launchd state for %s", job.Label)
			}
			if job.Running && !current[i].Running {
				waiting = job.Label
			}
		}
		if waiting == "" {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("the restored launchd job %s has not started", waiting)
		}
		time.Sleep(150 * time.Millisecond)
	}
}
