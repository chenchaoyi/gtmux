package userservice

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// CheckStorageStopped rejects managed jobs that could restart during an offline
// move, including jobs that have no child PID at the time of the check.
func CheckStorageStopped(home string, labels ...string) error {
	uid := strconv.Itoa(os.Getuid())
	switch manager := Current(home).(type) {
	case Launchd:
		return stoppedLaunchdStorage(storageProbe, manager, uid, labels)
	case Systemd:
		for _, label := range labels {
			if err := stoppedSystemdUnit(storageProbe, label+".service"); err != nil {
				return err
			}
		}
	}
	return nil
}

func storageProbe(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(cmd.Environ(), "LC_ALL=C")
	// Use manager diagnostics only for the absent-job classification. The
	// complete output can include service arguments and must not be logged.
	return cmd.CombinedOutput()
}

type storageManagerProbe func(string, ...string) ([]byte, error)

// An unloaded standard LaunchAgent can still load on the next GUI login.
func stoppedLaunchdStorage(run storageManagerProbe, manager Launchd, uid string, labels []string) error {
	var installed []string
	for _, label := range labels {
		if _, err := os.Lstat(manager.Path(label)); err == nil {
			installed = append(installed, label)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("could not inspect the installed launchd job %s", label)
		}
	}
	for _, domain := range []string{"gui/" + uid, "user/" + uid} {
		for _, label := range labels {
			if err := stoppedLaunchdJob(run, domain, label); err != nil {
				return err
			}
		}
		if len(installed) == 0 {
			continue
		}
		output, err := run("launchctl", "print-disabled", domain)
		if err != nil {
			return fmt.Errorf("could not verify persistent launchd state in %s", domain)
		}
		for _, label := range installed {
			if !persistentlyDisabled(output, label) {
				return fmt.Errorf("persistently disable the installed launchd job %s/%s before changing storage", domain, label)
			}
		}
	}
	return nil
}

func persistentlyDisabled(output []byte, label string) bool {
	count := 0
	disabled := false
	for _, line := range strings.Split(string(output), "\n") {
		key, value, found := strings.Cut(strings.TrimSpace(line), "=>")
		if found && strings.TrimSpace(key) == strconv.Quote(label) {
			count++
			value = strings.TrimSpace(value)
			disabled = value == "disabled" || value == "true"
		}
	}
	return count == 1 && disabled
}

func stoppedLaunchdJob(run storageManagerProbe, domain, label string) error {
	address := domain + "/" + label
	output, err := run("launchctl", "print", address)
	if err == nil {
		return fmt.Errorf("unload the launchd job %s before changing storage", address)
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 113 &&
		(strings.Contains(string(output), "Could not find service "+strconv.Quote(label)+" in domain ") ||
			strings.Contains(string(output), "Could not find domain for")) {
		return nil
	}
	return fmt.Errorf("could not confirm the launchd job %s is unloaded", address)
}

func stoppedSystemdUnit(run storageManagerProbe, unit string) error {
	properties := []string{"LoadState", "ActiveState", "SubState", "UnitFileState", "Job"}
	out, err := run("systemctl", "--user", "show", "--all", "--property="+strings.Join(properties, ","), unit)
	unknown := fmt.Errorf("could not confirm the systemd user unit %s is stopped and disabled", unit)
	if err != nil {
		return unknown
	}
	props := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSuffix(string(out), "\n"), "\n") {
		key, value, found := strings.Cut(line, "=")
		_, duplicate := props[key]
		if !found || key == "" || duplicate {
			return unknown
		}
		props[key] = value
	}
	for _, key := range properties {
		if _, ok := props[key]; !ok {
			return unknown
		}
	}
	if (props["Job"] == "" || props["Job"] == "0") &&
		props["ActiveState"] == "inactive" && props["SubState"] == "dead" {
		switch props["LoadState"] {
		case "not-found":
			if props["UnitFileState"] == "" {
				return nil
			}
		case "loaded", "masked":
			switch props["UnitFileState"] {
			case "disabled", "masked", "masked-runtime":
				return nil
			}
		}
	}
	if props["LoadState"] != "" && props["ActiveState"] != "" {
		return fmt.Errorf("stop and disable the systemd user unit %s before changing storage", unit)
	}
	return unknown
}
