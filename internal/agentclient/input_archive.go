package agentclient

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/chenchaoyi/gtmux/internal/state"
)

const (
	archiveCatalogName  = ".retired-catalog.json"
	archiveCatalogLimit = 192 << 10
	archivePageBytes    = 256 << 10
	archiveBudget       = 64 << 20
	archiveKeyBytes     = 16
	archiveReservation  = inputScanLimit * archiveKeyBytes
)

var (
	ErrInputArchiveIntegrity = errors.New("terminal input archive cannot be verified")
	ErrInputArchiveCapacity  = errors.Join(ErrInputFull, errors.New("terminal input archive has no reservation space; run gtmux doctor"))
)

type inputArchivePage struct {
	Bytes int    `json:"bytes"`
	SHA   string `json:"sha256"`
}

type inputArchiveCatalog struct {
	Version  int                           `json:"version"`
	Prefixes map[string][]inputArchivePage `json:"prefixes"`
}

type inputArchiveSeal struct {
	Payload json.RawMessage `json:"payload"`
	SHA     string          `json:"sha256"`
}

func archiveDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func archiveKey(index string) (string, [archiveKeyBytes]byte, bool) {
	var id [archiveKeyBytes]byte
	key, ok := strings.CutSuffix(index, ".index.json")
	if !ok || !instancePattern.MatchString(key) {
		return "", id, false
	}
	if _, err := hex.Decode(id[:], []byte(key)); err != nil {
		return "", id, false
	}
	return key[:2], id, true
}

func archiveHex(s string, length int) bool {
	return len(s) == length && strings.Trim(s, "0123456789abcdef") == ""
}

func (a inputArchiveCatalog) occupancy() int {
	total := 0
	for _, pages := range a.Prefixes {
		for _, page := range pages {
			total += page.Bytes
		}
	}
	return total
}

func (a inputArchiveCatalog) valid() bool {
	if a.Version != 1 || a.Prefixes == nil || len(a.Prefixes) > 256 {
		return false
	}
	total := 0
	for prefix, pages := range a.Prefixes {
		if !archiveHex(prefix, 2) || len(pages) == 0 || len(pages) > archiveBudget/archivePageBytes {
			return false
		}
		for n, p := range pages {
			if p.Bytes <= 0 || p.Bytes > archivePageBytes || p.Bytes%archiveKeyBytes != 0 ||
				(n < len(pages)-1 && p.Bytes != archivePageBytes) || !archiveHex(p.SHA, 64) {
				return false
			}
			total += p.Bytes
			if total > archiveBudget {
				return false
			}
		}
	}
	return true
}

func (q *inputQueue) archiveCatalog(commit bool) (inputArchiveCatalog, bool, error) {
	var catalog inputArchiveCatalog
	f, err := q.open(archiveCatalogName, unix.O_RDONLY)
	if errors.Is(err, os.ErrNotExist) && q.missing(retiredInputDir) {
		return inputArchiveCatalog{Version: 1, Prefixes: make(map[string][]inputArchivePage)}, false, nil
	}
	if err != nil {
		return catalog, false, errors.Join(ErrInputArchiveIntegrity, err)
	}
	defer f.Close()
	data, err := readOpenInputBytes(f, archiveCatalogLimit, true)
	var seal inputArchiveSeal
	fields := inputRecordFields{names: []string{"payload", "sha256"}, required: 2}
	if err != nil || decodeInputRecord(data, &seal, fields) != nil || archiveDigest(seal.Payload) != seal.SHA ||
		json.Unmarshal(seal.Payload, &catalog) != nil || !catalog.valid() {
		return catalog, false, ErrInputArchiveIntegrity
	}
	if commit {
		if err := q.syncDocument(f); err != nil {
			return catalog, false, err
		}
	}
	return catalog, true, nil
}

func (q *inputQueue) writeArchiveCatalog(catalog inputArchiveCatalog, replace bool) error {
	payload, err := json.Marshal(catalog)
	if err != nil {
		return err
	}
	body, err := json.Marshal(inputArchiveSeal{Payload: payload, SHA: archiveDigest(payload)})
	if err != nil || len(body) > archiveCatalogLimit {
		return ErrInputArchiveIntegrity
	}
	if !replace {
		return q.publishDocument(archiveCatalogName, body)
	}
	tmp := ".input-archive-stage-" + rand.Text()
	defer q.remove(tmp)
	if err := q.createSynced(tmp, body); err != nil {
		return err
	}
	if err := unix.Renameat(q.fd(), tmp, q.fd(), archiveCatalogName); err != nil {
		return err
	}
	return q.ops.syncDir(q.dir)
}

func archivePageName(prefix string, n int) string {
	return fmt.Sprintf("%s-%03d.ids", prefix, n)
}

func committedArchivePage(f *os.File, expected inputArchivePage) ([]byte, error) {
	data, err := readOpenInputBytes(f, int64(expected.Bytes+archiveKeyBytes), true)
	if err != nil || len(data) < expected.Bytes || archiveDigest(data[:expected.Bytes]) != expected.SHA {
		return nil, ErrInputArchiveIntegrity
	}
	return data[:expected.Bytes], nil
}

func (q *inputQueue) archivePage(prefix string, n int, meta inputArchivePage) ([]byte, error) {
	f, err := q.open(archivePageName(prefix, n), unix.O_RDONLY)
	if err != nil {
		return nil, errors.Join(ErrInputArchiveIntegrity, err)
	}
	defer f.Close()
	return committedArchivePage(f, meta)
}

func (q *inputQueue) archiveContains(prefix string, id [archiveKeyBytes]byte, pages []inputArchivePage) (bool, error) {
	for n, page := range pages {
		data, err := q.archivePage(prefix, n, page)
		if err != nil {
			return false, err
		}
		for offset := 0; offset < len(data); offset += archiveKeyBytes {
			if bytes.Equal(data[offset:offset+archiveKeyBytes], id[:]) {
				return true, nil
			}
		}
	}
	return false, nil
}

func (q *inputQueue) retiredAccepted(index string) (bool, error) {
	prefix, id, valid := archiveKey(index)
	if !valid {
		return false, ErrInputArchiveIntegrity
	}
	catalog, _, err := q.archiveCatalog(false)
	if err != nil {
		return false, err
	}
	pages := catalog.Prefixes[prefix]
	if len(pages) == 0 {
		return false, nil
	}
	retired, err := q.openRetired(false)
	if err != nil {
		return false, errors.Join(ErrInputArchiveIntegrity, err)
	}
	defer retired.dir.Close()
	return retired.archiveContains(prefix, id, pages)
}

// The lease serializes page writers. A bounded uncommitted append can be
// rolled back to the verified prefix while its accepted source still exists.
func (q *inputQueue) rememberRetiredAcceptance(retired *inputQueue, index string) error {
	prefix, id, valid := archiveKey(index)
	if !valid {
		return ErrInputArchiveIntegrity
	}
	catalog, _, err := q.archiveCatalog(true)
	if err != nil {
		return err
	}
	pages := catalog.Prefixes[prefix]
	present, err := retired.archiveContains(prefix, id, pages)
	if err != nil || present {
		return err
	}
	if catalog.occupancy()+archiveKeyBytes > archiveBudget {
		return ErrInputArchiveCapacity
	}
	n := len(pages) - 1
	if n < 0 || pages[n].Bytes == archivePageBytes {
		n++
		pages = append(pages, inputArchivePage{SHA: archiveDigest(nil)})
	}
	name := archivePageName(prefix, n)
	f, err := retired.open(name, unix.O_RDWR|unix.O_CREAT)
	if err != nil {
		return err
	}
	defer f.Close()
	meta := pages[n]
	data, err := committedArchivePage(f, meta)
	if err != nil {
		return err
	}
	if err := f.Truncate(int64(meta.Bytes)); err != nil {
		return err
	}
	if _, err := f.WriteAt(id[:], int64(meta.Bytes)); err != nil {
		return err
	}
	if err := retired.syncDocument(f); err != nil {
		return err
	}
	pages[n] = inputArchivePage{Bytes: meta.Bytes + archiveKeyBytes, SHA: archiveDigest(append(data, id[:]...))}
	catalog.Prefixes[prefix] = pages
	return q.writeArchiveCatalog(catalog, true)
}

// InputArchiveHealth is a read-only summary. It never repairs or creates state.
type InputArchiveHealth struct {
	UsedBytes, LimitBytes int
	AdmissionReserved     bool
	NeedsRepair           bool
	RecoveryPending       bool
}

func InspectInputArchive() InputArchiveHealth {
	health := InputArchiveHealth{LimitBytes: archiveBudget}
	store, err := resolveInputStore(state.Dir())
	if err != nil {
		health.NeedsRepair = true
		return health
	}
	if store.directory == (inputIdentity{}) {
		return health
	}
	q, err := openInputQueue(store, syncInput)
	if err != nil {
		health.NeedsRepair = true
		return health
	}
	defer q.dir.Close()
	catalog, _, err := q.archiveCatalog(false)
	if err != nil {
		health.NeedsRepair = true
		return health
	}
	health.UsedBytes = catalog.occupancy()
	health.AdmissionReserved = health.UsedBytes >= archiveBudget-archiveReservation
	retired, err := q.openRetired(false)
	if err != nil {
		health.NeedsRepair = health.UsedBytes != 0 || !errors.Is(err, os.ErrNotExist)
		return health
	}
	defer retired.dir.Close()
	for prefix, pages := range catalog.Prefixes {
		for n, page := range pages {
			if _, err := retired.archivePage(prefix, n, page); err != nil {
				health.NeedsRepair = true
				return health
			}
		}
	}
	_, tail, err := retired.readRetirement()
	if tail != nil {
		_ = tail.Close()
	}
	health.RecoveryPending = err == nil
	health.NeedsRepair = err != nil && !errors.Is(err, os.ErrNotExist)
	return health
}
