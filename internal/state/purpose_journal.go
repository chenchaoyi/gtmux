package state

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Version 3 clients reject this version at their ordinary command entry. The
// prior verified v3 marker ties preparation and rollback to this data root.
const (
	purposeVersion      = 4
	purposeMoving       = "move-to-categories"
	purposeFinishing    = "finalize-categories"
	purposeReleasing    = "pause-categories"
	purposeRestored     = "finish-return-from-categories"
	purposeServicesHeld = "return-category-services"
	purposeActive       = "categories-active"
	purposeReturning    = "return-from-categories"
)

func validatePurposeJournal(j *workflowJournal) error {
	if d, err := hex.DecodeString(j.PurposeToken); err != nil || len(d) != 16 {
		return errors.New("invalid purpose layout generation")
	}
	if j.BaseMarker == nil || !layoutDirectoryRecord(*j.BaseMarker) {
		return errors.New("invalid previous layout marker")
	}
	if j.Marker == nil || j.Marker.Inode == 0 || j.Marker.Mode != uint32(os.ModeDir) {
		return errors.New("purpose layout has no valid source marker")
	}
	switch j.Phase {
	case purposeActive, purposeServicesHeld:
		if len(j.Entries) != 0 {
			return errors.New("active layout still has pending moves")
		}
	case purposeMoving, purposeFinishing, purposeReleasing, purposeReturning, purposeRestored:
		if len(j.Entries) > len(purposeKeys()) {
			return errors.New("storage move manifest is too large")
		}
	default:
		return errors.New("unknown purpose layout phase")
	}
	last := ""
	for _, e := range j.Entries {
		if _, _, ok := purposeLocations(e.Name); !ok || e.Name <= last || e.Inode == 0 || e.Mode != 0 && e.Mode != uint32(os.ModeDir) {
			return errors.New("invalid purpose layout entry")
		}
		last = e.Name
	}
	return nil
}

func verifyPurposeMarker(j *workflowJournal) error {
	if err := validatePurposeJournal(j); err != nil {
		return err
	}
	base, ok, err := readLayoutMarker()
	if err != nil {
		return err
	}
	if !ok || base != *j.BaseMarker {
		return errors.New("the rollback marker changed")
	}
	old := filepath.Join(Dir(), workflowLayoutMarker)
	active := filepath.Join(Dir(), purposeMarker)
	moved, err := layoutEntryLocation(*j.Marker, old, active)
	if err != nil {
		return err
	}
	if (j.Phase == purposeActive || j.Phase == purposeServicesHeld) && !moved {
		return errors.New("purpose layout source marker changed")
	}
	return nil
}

// Prepare records the full inventory before the first rename. Callers must hold
// exclusive layout admission and have independently quiesced external writers.
func preparePurposeJournal(previous *workflowJournal, forward bool) (*workflowJournal, error) {
	if previous == nil {
		return nil, errors.New("storage source requires a verified journal")
	}
	if err := verifyWorkflowJournal(previous); err != nil {
		return nil, err
	}
	phase := purposeReleasing
	token := previous.PurposeToken
	if forward {
		if previous.Version != workflowVersion || previous.Phase != workflowGrouped {
			return nil, errors.New("finish the previous storage change first")
		}
		if err := checkWorkflowActive(previous); err != nil {
			return nil, err
		}
		if err := checkBaseStorageLayout(); err != nil {
			return nil, err
		}
		var err error
		token, err = purposeInputToken()
		if err != nil {
			return nil, err
		}
		phase = purposeMoving
	} else {
		if previous.Version != purposeVersion || previous.Phase != purposeActive {
			return nil, errors.New("purpose layout is not active")
		}
		if err := verifyPurposeMarker(previous); err != nil {
			return nil, err
		}
	}
	entries, err := planPurposeMoves(forward)
	if err != nil {
		return nil, err
	}
	base, ok, err := readLayoutMarker()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("previous layout marker is missing")
	}
	next := &workflowJournal{BaseMarker: &base, Version: purposeVersion, Phase: phase, PurposeToken: token, Marker: previous.Marker, Entries: entries}
	if err := validatePurposeJournal(next); err != nil {
		return nil, err
	}
	if err := publishWorkflowJournal(next, previous); err != nil {
		return nil, err
	}
	return next, nil
}

// Resume only entries pinned by the recorded manifest. A fully moved manifest
// is still an offline transition until every external writer has its new route.
func runPurposeMoves(j *workflowJournal, checkWriters func() error) error {
	if err := verifyWorkflowJournal(j); err != nil {
		return err
	}
	if err := verifyPurposeMarker(j); err != nil {
		return err
	}
	if j.Phase != purposeMoving && j.Phase != purposeReturning {
		return errors.New("no pending purpose layout move")
	}
	forward := j.Phase == purposeMoving
	if err := verifyPurposeMoves(j.Entries, forward, false); err != nil {
		return err
	}
	if err := checkWriters(); err != nil {
		return err
	}
	for _, e := range j.Entries {
		if err := advancePurposeMove(e, forward); err != nil {
			return err
		}
	}
	if err := checkWriters(); err != nil {
		return err
	}
	if err := verifyWorkflowJournal(j); err != nil {
		return err
	}
	return verifyPurposeMoves(j.Entries, forward, true)
}

func requireNoPurposeMarker() error {
	if _, err := os.Lstat(filepath.Join(Dir(), purposeMarker)); !errors.Is(err, os.ErrNotExist) {
		return errors.New("storage category marker exists without its verified journal")
	}
	return nil
}

func checkActivePurposeRoutes(token string) error {
	if err := verifyActivePurposeInput(token); err != nil {
		return err
	}
	files, err := boundedWorkflowDirectory(Dir())
	if err != nil {
		return err
	}
	for _, file := range files {
		_, wf := workflowStoreFor(file.Name())
		if _, known := groupedStores[file.Name()]; known || wf {
			return fmt.Errorf("inactive flat path exists: %s", file.Name())
		}
	}

	// Every old private store must have disappeared except for the two
	// registered read references. Fail closed on a recreated writer path.
	for _, key := range purposeKeys() {
		from, to, err := purposeEndpoints(key, true)
		if err != nil {
			return err
		}
		if key == "uploads" || key == "logs" {
			id, exists, err := workflowPathObject(to)
			if err != nil {
				return err
			}
			if !exists {
				return fmt.Errorf("missing storage reference target: %s", key)
			}
			if exists {
				if _, err := inspectPurposeReference(key, id); err != nil {
					return err
				}
				continue
			}
		}
		if _, present, err := workflowPathObject(from); err != nil {
			return err
		} else if present {
			return fmt.Errorf("older writer recreated %s; update the old client and preserve both paths", key)
		}
	}
	return nil
}

// PurposeInputAdmitted checks the global phase in addition to the independent
// input record. Publishing the latter alone cannot expose a half-moved store.
func PurposeInputAdmitted(root, token string) bool {
	if root != Dir() {
		return false
	}
	j, err := readWorkflowJournal()
	return err == nil && j != nil && j.Version == purposeVersion && j.Phase == purposeActive && j.PurposeToken == token && verifyPurposeMarker(j) == nil
}
