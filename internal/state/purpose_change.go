package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sys/unix"
)

// ChangePurposeLayout runs only with an exclusive bootstrap lease. Direct
// writers (the menu bar and plugin) are additionally checked by the caller.
func ChangePurposeLayout(forward bool, checkWriters func() error) error {
	return changePurposeLayout(forward, checkWriters, nil)
}

// Park verified service profiles only after older writers have been fenced.
func ChangePurposeLayoutWithServices(forward bool, checkWriters, park func() error) error {
	return changePurposeLayout(forward, checkWriters, park)
}

func changePurposeLayout(forward bool, checkWriters, park func() error) error {
	lease, err := acquireLayoutLease(unix.LOCK_EX)
	if err != nil {
		return err
	}
	defer lease.Close()
	if park == nil {
		if err := checkWriters(); err != nil {
			return err
		}
	}
	j, err := readWorkflowJournal()
	if err != nil {
		return err
	}
	if j == nil {
		return errors.New("finish grouping the previous layout first")
	}
	if forward && j.Phase == purposeActive {
		return CheckStorageLayout()
	}
	if !forward && j.Phase == workflowGrouped {
		return CheckStorageLayout()
	}
	if forward && j.Version == workflowVersion {
		if err := CheckPurposeSource(); err != nil {
			return err
		}
		for _, key := range []string{"uploads", "logs", "terminal-input"} {
			path := filepath.Join(Dir(), key)
			if err := MkdirAll(path, PrivateDir); err != nil {
				return err
			}
			if err := checkLayoutDir(path); err != nil {
				return err
			}
		}
		j, err = preparePurposeJournal(j, true)
	} else if !forward && j.Phase == purposeActive {
		if err := CheckStorageLayout(); err != nil {
			return err
		}
		j, err = preparePurposeJournal(j, false)
	}
	if err != nil {
		return err
	}
	if err := verifyPurposeMarker(j); err != nil {
		return err
	}
	if forward != (j.Phase == purposeMoving || j.Phase == purposeFinishing) {
		return errors.New("complete the recorded storage direction first")
	}
	if park != nil {
		if !j.HoldServices {
			next := *j
			next.HoldServices = true
			if err := publishWorkflowJournal(&next, j); err != nil {
				return err
			}
			j = &next
		}
		if err := park(); err != nil {
			return err
		}
		if err := checkWriters(); err != nil {
			return err
		}
	}
	if forward {
		return finishPurposeForward(j, checkWriters)
	}
	return finishPurposeReturn(j, checkWriters)
}

func purposePhase(j *workflowJournal, phase string) (*workflowJournal, error) {
	next := *j
	next.Phase = phase
	if phase == purposeActive {
		next.HoldServices = false
	}
	if phase == purposeActive || phase == purposeServicesHeld {
		next.Entries = nil
	}
	if err := publishWorkflowJournal(&next, j); err != nil {
		return nil, err
	}
	return &next, nil
}

func pauseInputBytes(j *workflowJournal) []byte {
	return inputRecordBytes("paused", j.PurposeToken, layoutEntry{}, layoutEntry{})
}

func finishPurposeForward(j *workflowJournal, checkWriters func() error) error {
	var err error
	if j.Phase == purposeMoving {
		if err := publishPurposeDocument(purposeInputRecord, nil, pauseInputBytes(j)); err != nil {
			return err
		}
		if err := runPurposeMoves(j, checkWriters); err != nil {
			return err
		}
		j, err = purposePhase(j, purposeFinishing)
		if err != nil {
			return err
		}
	}
	if err := verifyPurposeMoves(j.Entries, true, true); err != nil {
		return err
	}
	for _, e := range j.Entries {
		if e.Name == "logs" || e.Name == "uploads" {
			if _, err := publishPurposeReference(e.Name, e); err != nil {
				return err
			}
		}
	}
	if _, err := activatePurposeInput(j.PurposeToken); err != nil {
		return err
	}
	if err := checkWriters(); err != nil {
		return err
	}
	if err := checkActivePurposeRoutes(j.PurposeToken); err != nil {
		return err
	}
	if err := advanceLayoutEntry(*j.Marker, filepath.Join(Dir(), workflowLayoutMarker), filepath.Join(Dir(), purposeMarker)); err != nil {
		return err
	}
	if err := prunePurposeDirectories(true); err != nil {
		return err
	}
	if j.HoldServices {
		held, err := purposePhase(j, purposeServicesHeld)
		if err != nil {
			return err
		}
		return verifyStorageBeforeReturningServices(held)
	}
	if _, err := purposePhase(j, purposeActive); err != nil {
		return err
	}
	return CheckStorageLayout()
}

func finishPurposeReturn(j *workflowJournal, checkWriters func() error) error {
	if j.Phase == purposeReleasing {
		// Obtain the exact original active bytes from the pinned directory and
		// generation, even on a retry after already publishing the pause.
		dir, _, err := workflowPathObject(filepath.Join(Dir(), "sessions", "terminal-input"))
		if err != nil {
			return err
		}
		data, id, err := purposeDocument(filepath.Join(Dir(), purposeInputMarker))
		if err != nil {
			return err
		}
		want, _ := json.Marshal(inputGeneration{2, j.PurposeToken})
		if string(data) != string(want) {
			return errors.New("terminal input generation changed")
		}
		active := inputRecordBytes("active", j.PurposeToken, dir, id)
		if err := publishPurposeDocument(purposeInputRecord, active, pauseInputBytes(j)); err != nil {
			return err
		}
		for _, e := range j.Entries {
			if e.Name == "logs" || e.Name == "uploads" {
				if _, err := os.Lstat(filepath.Join(Dir(), e.Name)); errors.Is(err, os.ErrNotExist) {
					continue
				}
				ref, err := inspectPurposeReference(e.Name, e)
				if err != nil {
					return err
				}
				if err := ref.retire(); err != nil {
					return err
				}
			}
		}
		if err := SyncPrivateDirectory(Dir()); err != nil {
			return err
		}
		j, err = purposePhase(j, purposeReturning)
		if err != nil {
			return err
		}
	}
	if j.Phase == purposeReturning {
		for _, group := range layoutGroups() {
			if err := MkdirAll(filepath.Join(groupedRoot(), group), PrivateDir); err != nil {
				return err
			}
		}
		if err := SyncPrivateDirectory(groupedRoot()); err != nil {
			return err
		}
		if err := SyncPrivateDirectory(Dir()); err != nil {
			return err
		}
		if err := runPurposeMoves(j, checkWriters); err != nil {
			return err
		}
		next, err := purposePhase(j, purposeRestored)
		if err != nil {
			return err
		}
		j = next
	}
	if err := verifyPurposeMoves(j.Entries, false, true); err != nil {
		return err
	}
	if err := advanceLayoutEntry(*j.Marker, filepath.Join(Dir(), purposeMarker), filepath.Join(Dir(), workflowLayoutMarker)); err != nil {
		return err
	}
	data, _ := json.Marshal(inputGeneration{2, j.PurposeToken})
	if err := removePurposeDocument(purposeInputMarker, data); err != nil {
		return err
	}
	if err := removePurposeDocument(purposeInputRecord, pauseInputBytes(j)); err != nil {
		return err
	}
	prior := &workflowJournal{Version: workflowVersion, Phase: workflowGrouped, Marker: j.Marker}
	if err := prunePurposeDirectories(false); err != nil {
		return err
	}
	if j.HoldServices {
		prior.Phase = workflowServiceReady
	}
	if err := checkWorkflowActive(prior); err != nil {
		return err
	}
	if err := publishWorkflowJournal(prior, j); err != nil {
		return err
	}
	return verifyStorageBeforeReturningServices(prior)
}

// Drop only empty directories whose names come from the layout registry. A
// profile, unknown child or private recovery file keeps its parent in place.
func prunePurposeDirectories(forward bool) error {
	dirs := map[string]bool{}
	for _, key := range purposeKeys() {
		old, to, _ := purposeLocations(key)
		src := to
		if forward {
			src = old
		}
		for dir := filepath.Dir(src); dir != "."; dir = filepath.Dir(dir) {
			if !forward || dir == layoutDir || dir == workflowLayoutDir || strings.HasPrefix(dir, layoutDir+"/") || strings.HasPrefix(dir, workflowLayoutDir+"/") {
				dirs[dir] = true
			}
		}
	}
	if forward {
		for _, group := range layoutGroups() {
			dirs[filepath.Join(layoutDir, group)] = true
		}
	}
	keys := make([]string, 0, len(dirs))
	for name := range dirs {
		keys = append(keys, name)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	for _, name := range keys {
		path := filepath.Join(Dir(), name)
		id, exists, err := workflowPathObject(path)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		if id.Mode != uint32(os.ModeDir) {
			return fmt.Errorf("unexpected object at %s", name)
		}
		if err := checkLayoutDir(path); err != nil {
			return err
		}
		if err := removeRecoveryDirectory(path, id); err != nil && !errors.Is(err, unix.ENOTEMPTY) && !errors.Is(err, unix.EEXIST) {
			return err
		}
	}
	return nil
}

// CheckPurposeSource is read-only and is also called again under exclusive
// admission, after consent. Fail before parking any service when possible.
func CheckPurposeSource() error {
	j, err := readWorkflowJournal()
	if err != nil {
		return err
	}
	if j == nil || j.Version != workflowVersion || j.Phase != workflowGrouped {
		return errors.New("finish the previous layout first")
	}
	if err := checkWorkflowActive(j); err != nil {
		return err
	}
	if err := checkBaseStorageLayout(); err != nil {
		return err
	}
	// A dedicated earlier terminal-input cutover has its own generation.
	// Do not replace that live receipt or move its marker as a queue.
	for _, name := range []string{purposeInputRecord, "terminal-input-v2", purposeInputMarker, purposeMarker} {
		if _, exists, err := workflowPathObject(filepath.Join(Dir(), name)); err != nil {
			return err
		} else if exists {
			return fmt.Errorf("existing storage generation requires review: %s", name)
		}
	}
	if _, err := planPurposeMoves(true); err != nil {
		return err
	}

	for _, key := range []string{"uploads", "logs", "terminal-input"} {
		path := filepath.Join(Dir(), key)
		if err := checkLayoutDir(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// CancelPurposePreparation returns the old admission only before the first
// move. Once a source has moved, the durable direction must finish first.
func CancelPurposePreparation() error {
	lease, err := acquireLayoutLease(unix.LOCK_EX)
	if err != nil {
		return err
	}
	defer lease.Close()
	j, err := readWorkflowJournal()
	if err != nil {
		return err
	}
	if j == nil || j.Phase != purposeMoving {
		return errors.New("finish the recorded move before rollback")
	}
	if err := verifyPurposeMarker(j); err != nil {
		return err
	}
	if err := verifyPurposeMoves(j.Entries, true, false); err != nil {
		return err
	}
	for _, e := range j.Entries {
		a, b, _ := purposeEndpoints(e.Name, true)
		there, err := layoutEntryLocation(e, a, b)
		if err != nil {
			return err
		}
		if there {
			return errors.New("data has already moved; finish the recorded move before rollback")
		}
	}
	if err := removePurposeDocument(purposeInputRecord, pauseInputBytes(j)); err != nil {
		return err
	}
	prior := &workflowJournal{Version: workflowVersion, Phase: workflowGrouped, Marker: j.Marker}
	if j.HoldServices {
		prior.Phase = workflowServiceReady
	}
	if err := checkWorkflowActive(prior); err != nil {
		return err
	}
	return publishWorkflowJournal(prior, j)
}
