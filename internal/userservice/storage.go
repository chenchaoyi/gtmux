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
	manager := Current(home)
	switch manager.(type) {
	case Launchd:
		for _, domain := range []string{"gui/" + uid, "user/" + uid} {
			for _, label := range labels {
				if err := stoppedLaunchdJob(storageProbe, domain, label); err != nil {
					return err
				}
			}
		}
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
	out, err := run("systemctl", "--user", "show", "--all", "--property=LoadState,ActiveState,SubState,UnitFileState,Job", unit)
	props := make(map[string]string)
	for _, line := range strings.Split(string(out), "\n") {
		key, value, found := strings.Cut(line, "=")
		if found {
			props[key] = value
		}
	}
	job, jobReported := props["Job"]
	if err == nil && jobReported && (job == "" || job == "0") &&
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
	if err == nil && props["LoadState"] != "" && props["ActiveState"] != "" {
		return fmt.Errorf("stop and disable the systemd user unit %s before changing storage", unit)
	}
	return fmt.Errorf("could not confirm the systemd user unit %s is stopped and disabled", unit)
}
