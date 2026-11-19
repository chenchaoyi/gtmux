package agentclient

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestSPL023FullPageAndNearFullQueueKeepAdmitting(t *testing.T) {
	_, target, _ := attachmentFixture(t)
	now := time.Now()
	prefix := "3c"
	id := func(n int) string { return archiveFixtureID(target.Pane, prefix, fmt.Sprint(n)) }
	stable := settleRetirementFixture(t, target, id(0), "accepted", now)
	settleRetirementFixture(t, target, id(1), "accepted", now)
	reserved, err := Prepare(target, "in flight", "reserved", false, now)
	if err != nil {
		t.Fatal(err)
	}
	q, err := openInputQueue(stable.store, syncInput)
	if err != nil {
		t.Fatal(err)
	}
	defer q.dir.Close()
	archivedOld := id(-1)
	seedFullArchivePage(t, q, target.Pane, archivedOld)

	names, err := inputNames(InputDir())
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < inputLimit-len(names)-(inputReservation-2); n++ {
		if err := os.WriteFile(filepath.Join(InputDir(), fmt.Sprintf("foreign-%04d", n)), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := admitInput(InputDir()); !errors.Is(err, ErrInputFull) {
		t.Fatalf("fixture did not reach its admission boundary: %v", err)
	}
	if err := ClaimNative(reserved); err != nil {
		t.Fatal(err)
	}
	if err := FinishNative(reserved, "accepted", ""); err != nil {
		t.Fatal(err)
	}
	for n := 2; n < 9; n++ {
		later := now.Add(time.Duration(n) * 96 * time.Hour)
		settleRetirementFixture(t, target, id(n), "accepted", later)
	}
	catalog, _, err := q.archiveCatalog(false)
	if err != nil || len(catalog.Prefixes[prefix]) != 2 || catalog.Prefixes[prefix][1].Bytes < 7*archiveKeyBytes {
		t.Fatalf("same prefix did not use shared space: %+v %v", catalog.Prefixes[prefix], err)
	}
	for _, old := range []string{archivedOld, id(0), id(4)} {
		if _, err := Prepare(target, "fixture", old, true, now.Add(40*24*time.Hour)); !errors.Is(err, ErrInputUncertain) {
			t.Fatalf("retired ID became eligible for native input: %v", err)
		}
		if TerminalFallbackError(target.Pane, old) == nil {
			t.Fatal("retired ID could switch to a raw route")
		}
	}
	health := InspectInputArchive()
	if health.NeedsRepair || health.UsedBytes <= archivePageBytes {
		t.Fatalf("doctor did not observe the committed overflow: %+v", health)
	}
}

func TestSPL023ArchiveBudgetKeepsSpaceForSettlement(t *testing.T) {
	_, target, _ := attachmentFixture(t)
	if err := os.MkdirAll(InputDir(), 0700); err != nil {
		t.Fatal(err)
	}
	store, err := resolveInputStore(state.Dir())
	if err != nil {
		t.Fatal(err)
	}
	q, err := openInputQueue(store, syncInput)
	if err != nil {
		t.Fatal(err)
	}
	defer q.dir.Close()
	// A seal alone models committed occupancy. The incoming key belongs to
	// a healthy, unallocated prefix, so absent unrelated pages do not
	// preempt the shared-budget admission check.
	pages := make([]inputArchivePage, (archiveBudget-archiveReservation+archivePageBytes-1)/archivePageBytes)
	for n := range pages {
		pages[n] = inputArchivePage{Bytes: archivePageBytes, SHA: archiveDigest(nil)}
	}
	pages[len(pages)-1].Bytes = archivePageBytes - archiveReservation
	if err := q.writeArchiveCatalog(inputArchiveCatalog{Version: 1, Prefixes: map[string][]inputArchivePage{"ef": pages}}, false); err != nil {
		t.Fatal(err)
	}
	newID := archiveFixtureID(target.Pane, "e0", "current")
	if _, err := Prepare(target, "fixture", newID, false, time.Now()); !errors.Is(err, ErrInputArchiveCapacity) {
		t.Fatalf("consumed the settlement reserve: %v", err)
	}
	names, err := inputNames(InputDir())
	if err != nil || len(names) != 1 || names[0] != archiveCatalogName {
		t.Fatalf("refused admission grew active queue: %v %v", names, err)
	}
	if h := InspectInputArchive(); !h.AdmissionReserved || h.UsedBytes != archiveBudget-archiveReservation {
		t.Fatalf("doctor did not report total capacity: %+v", h)
	}
}

func TestSPL024DamagedCommitBlocksOnlyUnverifiableIDs(t *testing.T) {
	for _, fault := range []string{"missing page", "missing directory", "aligned truncation", "equal length damage", "missing catalogue", "damaged catalogue", "invalid tail"} {
		t.Run(fault, func(t *testing.T) {
			_, target, _ := attachmentFixture(t)
			now := time.Now()
			prefix := "4b"
			id := archiveFixtureID(target.Pane, prefix, "old")
			r := settleRetirementFixture(t, target, id, "accepted", now)
			settleRetirementFixture(t, target, archiveFixtureID(target.Pane, prefix, "also retired"), "accepted", now)
			later := now.Add(96 * time.Hour)
			// Admission ages the original pair. The new accepted request in that
			// prefix still needs a future retirement after the fault.
			backlog := settleRetirementFixture(t, target, archiveFixtureID(target.Pane, prefix, "backlog"), "accepted", later)
			assertInputName(t, r, ".receipt.json", false)
			retired := filepath.Join(InputDir(), retiredInputDir)
			page := filepath.Join(retired, archivePageName(prefix, 0))
			seal := filepath.Join(InputDir(), archiveCatalogName)
			var err error
			switch fault {
			case "missing page":
				err = os.Remove(page)
			case "missing directory":
				err = os.Rename(retired, retired+".backup")
			case "aligned truncation":
				err = os.Truncate(page, archiveKeyBytes)
			case "equal length damage":
				raw, e := os.ReadFile(page)
				if e != nil || len(raw) != 2*archiveKeyBytes {
					t.Fatalf("invalid committed fixture: %v", e)
				}
				raw[0] ^= 1
				err = os.WriteFile(page, raw, 0600)
			case "missing catalogue":
				err = os.Remove(seal)
			case "damaged catalogue":
				raw, e := os.ReadFile(seal)
				if e != nil {
					t.Fatal(e)
				}
				raw[len(raw)/2] ^= 1
				err = os.WriteFile(seal, raw, 0600)
			case "invalid tail":
				err = os.WriteFile(filepath.Join(retired, retirementTail), []byte(`{"invalid":`), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			// Reopen through native and raw APIs, without retaining a queue descriptor.
			if _, err := Prepare(target, "fixture", id, true, later); !errors.Is(err, ErrInputUncertain) {
				t.Fatalf("accepted ID could replay over native transport: %v", err)
			}
			if err := TerminalFallbackError(target.Pane, id); err == nil || !PriorDeliveryUnknown(target.Pane, id) {
				t.Fatal("accepted ID could switch to the raw route")
			}
			if fault == "aligned truncation" {
				exe, err := os.Executable()
				if err != nil {
					t.Fatal(err)
				}
				serialized, err := json.Marshal(target)
				if err != nil {
					t.Fatal(err)
				}
				child := exec.Command(exe, "-test.run=^TestSPL024ArchiveReopenChild$")
				child.Env = append(os.Environ(), "GTMUX_ARCHIVE_FIXTURE_TARGET="+string(serialized), "GTMUX_ARCHIVE_FIXTURE_ID="+id)
				if output, err := child.CombinedOutput(); err != nil {
					t.Fatalf("fresh process lost archive integrity: %v %s", err, output)
				}
			}
			if h := InspectInputArchive(); !h.NeedsRepair {
				t.Fatalf("doctor did not detect archive fault: %+v", h)
			}
			fresh := archiveFixtureID(target.Pane, "e5", "unrelated")
			if strings.Contains(fault, "catalogue") {
				if _, err := Prepare(target, "fixture", fresh, false, later); !errors.Is(err, ErrInputUncertain) {
					t.Fatalf("unverifiable global catalogue authorized a stable send: %v", err)
				}
				fresh = "" // An ordinary new input has no stable-ID replay authority.
			}
			if _, err := Prepare(target, "fresh text", fresh, false, later.Add(96*time.Hour)); err != nil {
				t.Fatalf("unrelated available slot was refused: %v", err)
			}
			assertInputName(t, backlog, ".receipt.json", true)
			if fault == "invalid tail" {
				if data, err := os.ReadFile(filepath.Join(InputDir(), retiredInputDir, retirementTail)); err != nil || string(data) != `{"invalid":` {
					t.Fatalf("invalid checkpoint was changed: %q %v", data, err)
				}
			}
		})
	}
}

func TestSPL024ArchiveReopenChild(t *testing.T) {
	fixture := os.Getenv("GTMUX_ARCHIVE_FIXTURE_TARGET")
	if fixture == "" {
		return
	}
	var target Attachment
	if err := json.Unmarshal([]byte(fixture), &target); err != nil {
		t.Fatal(err)
	}
	id := os.Getenv("GTMUX_ARCHIVE_FIXTURE_ID")
	if _, err := Prepare(target, "fixture", id, true, time.Now()); !errors.Is(err, ErrInputUncertain) {
		t.Fatalf("restarted native path authorized the old ID: %v", err)
	}
	if TerminalFallbackError(target.Pane, id) == nil {
		t.Fatal("restarted raw path authorized the old ID")
	}
}

func TestSPL024UncommittedPageSuffixReconcilesToVerifiedPrefix(t *testing.T) {
	_, target, _ := attachmentFixture(t)
	now := time.Now()
	id := archiveFixtureID(target.Pane, "08", "old")
	second := archiveFixtureID(target.Pane, "08", "next")
	r := settleRetirementFixture(t, target, id, "accepted", now)
	settleRetirementFixture(t, target, second, "accepted", now.Add(96*time.Hour))
	file := filepath.Join(InputDir(), retiredInputDir, archivePageName("08", 0))
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, append(append([]byte(nil), before...), 1, 2, 3), 0600); err != nil {
		t.Fatal(err)
	}
	if TerminalFallbackError(target.Pane, id) == nil {
		t.Fatal("uncommitted append hid the authorised archive prefix")
	}
	settleRetirementFixture(t, target, archiveFixtureID(target.Pane, "ff", "trigger"), "accepted", now.Add(192*time.Hour))
	after, err := os.ReadFile(file)
	if err != nil || len(before) != archiveKeyBytes || len(after) != 2*archiveKeyBytes || string(after[:archiveKeyBytes]) != string(before) {
		t.Fatalf("failed to reconcile a partial append: len=%d %v", len(after), err)
	}
	assertInputName(t, r, ".receipt.json", false)
	if TerminalFallbackError(target.Pane, second) == nil {
		t.Fatal("new retirement lost its protection")
	}
}

func archiveFixtureID(pane, prefix, base string) string {
	for n := 0; ; n++ {
		id := fmt.Sprintf("%s/%d", base, n)
		if strings.HasPrefix(deliveryIndex(pane, id), prefix) {
			return id
		}
	}
}

func seedFullArchivePage(t *testing.T, q *inputQueue, pane, id string) {
	t.Helper()
	prefix, key, valid := archiveKey(deliveryIndex(pane, id))
	if !valid {
		t.Fatal("invalid archive fixture key")
	}
	payload := make([]byte, archivePageBytes)
	for offset := 0; offset < len(payload); offset += archiveKeyBytes {
		payload[offset] = key[0]
		binary.BigEndian.PutUint64(payload[offset+8:offset+16], uint64(offset))
	}
	copy(payload[:archiveKeyBytes], key[:])
	catalog := inputArchiveCatalog{Version: 1, Prefixes: map[string][]inputArchivePage{prefix: {{Bytes: len(payload), SHA: archiveDigest(payload)}}}}
	if err := q.writeArchiveCatalog(catalog, false); err != nil {
		t.Fatal(err)
	}
	r, err := q.openRetired(true)
	if err != nil {
		t.Fatal(err)
	}
	defer r.dir.Close()
	if err := r.publishDocument(archivePageName(prefix, 0), payload); err != nil {
		t.Fatal(err)
	}
}
