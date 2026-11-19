package agentclient

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// InputIntent identifies the command that can be staged in a terminal draft.
// Only Publish may make it visible to a receiver.
type InputIntent struct {
	Request  Request
	name     string
	previous Request
	reuse    bool
}

// A pre-gate durable caller can retry in this process, but must keep the
// target it first selected. Reserve memory only; publication remains gated.
type unpublishedInput struct {
	address Request
	expires time.Time
}

var unpublished = struct {
	sync.Mutex
	items map[string]unpublishedInput
}{items: make(map[string]unpublishedInput)}

func rememberInputTarget(r Request, target Attachment, now time.Time) (Request, error) {
	key := r.store.path + "\x00" + r.IndexKey
	unpublished.Lock()
	defer unpublished.Unlock()
	if old, ok := unpublished.items[key]; ok && now.Before(old.expires) {
		if !old.address.MatchesTarget(target) ||
			old.address.store.token != r.store.token {
			return Request{}, ErrInputTargetChanged
		}
		if old.address.TextHash != r.TextHash {
			return Request{}, errors.New("delivery ID was already used for another message")
		}
		r.ID = old.address.ID
	}
	if len(unpublished.items) >= inputLimit {
		for key, old := range unpublished.items {
			if !now.Before(old.expires) {
				delete(unpublished.items, key)
			}
		}
		if _, existing := unpublished.items[key]; !existing && len(unpublished.items) >= inputLimit {
			return Request{}, ErrInputFull
		}
	}
	address := r
	address.Text = ""
	unpublished.items[key] = unpublishedInput{address: address, expires: now.Add(5 * time.Minute)}
	return r, nil
}

func StageInput(a Attachment, text, deliveryID string, force bool, now time.Time) (InputIntent, error) {
	if !a.InputReady || len(text) > 1<<20 || strings.TrimSpace(text) == "" {
		return InputIntent{}, errors.New("terminal input is unavailable or exceeds its limit")
	}
	store, err := resolveInputStore(state.Dir())
	if err != nil {
		return InputIntent{}, err
	}
	if !a.matchesInputStore(store) {
		return InputIntent{}, ErrInputStoreUnavailable
	}
	sum := sha256.Sum256([]byte(text))
	hash := hex.EncodeToString(sum[:])
	seed := a.HomeID + "\x00" + a.SessionID + "\x00" + a.Instance + "\x00" + fmt.Sprint(a.BindingEpoch) + "\x00" + text
	key := sha256.Sum256([]byte(seed))
	name := hex.EncodeToString(key[:16]) + ".index.json"
	if deliveryID != "" {
		name = deliveryIndex(a.Pane, deliveryID)
	}
	intent := InputIntent{name: name}
	if store.directory != (inputIdentity{}) {
		queue, err := openInputQueue(store, syncInput)
		if err != nil {
			return InputIntent{}, err
		}
		defer queue.dir.Close()
		old, err := queue.loadIndex(name)
		if err == nil {
			if old.TextHash != hash {
				return InputIntent{}, errors.New("delivery ID was already used for another message")
			}
			intent.previous = old
			old.Text, old.store = text, store
			receipt, err := queue.committedReceipt(old)
			if err == nil {
				switch receipt.State {
				case "accepted":
					intent.reuse = deliveryID != "" || !force && now.UnixMilli()-old.CreatedAt <= 15000
				case "refused":
					if !force || deliveryID != "" {
						return InputIntent{}, &InputRefused{Reason: receipt.Error}
					}
				default:
					return InputIntent{}, ErrInputUncertain
				}
			} else {
				if errors.Is(err, ErrInputUncertain) || !errors.Is(err, os.ErrNotExist) || !queue.pendingMatches(old) {
					return InputIntent{}, ErrInputUncertain
				}
				if !old.MatchesTarget(a) {
					return InputIntent{}, ErrInputTargetChanged
				}
				age := now.UnixMilli() - old.CreatedAt
				if deliveryID == "" && (age < 0 || age > 15000) {
					return InputIntent{}, ErrInputUncertain
				}
				intent.reuse = true
			}
			if intent.reuse {
				intent.Request = old
				return intent, nil
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return InputIntent{}, ErrInputUncertain
		}
		// A saturated directory without indices has nothing we can retire.
		// Preserve the fast refusal before native RPC setup in this case.
		names, err := inputDirectoryNames(queue.dir)
		if err != nil {
			return InputIntent{}, err
		}
		if len(names)+inputReservation > inputLimit {
			hasIndex := false
			for _, name := range names {
				if strings.HasSuffix(name, ".index.json") {
					hasIndex = true
					break
				}
			}
			if !hasIndex {
				return InputIntent{}, ErrInputFull
			}
		}
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return InputIntent{}, err
	}
	intent.Request = Request{Version: ProtocolVersion, ID: hex.EncodeToString(nonce[:]), Instance: a.Instance,
		HomeID: a.HomeID, SessionID: a.SessionID, PID: a.PID, Pane: a.Pane, ServerEpoch: a.ServerEpoch,
		BindingEpoch: a.BindingEpoch, Text: text, TextHash: hash, DurableRetry: deliveryID != "",
		IndexKey: strings.TrimSuffix(name, ".index.json"), store: store}
	if deliveryID != "" {
		intent.Request, err = rememberInputTarget(intent.Request, a, now)
		if err != nil {
			return InputIntent{}, err
		}
	}
	return intent, nil
}

func (q *inputQueue) loadIndex(name string) (Request, error) {
	data, err := q.read(name, 8192)
	if err != nil {
		return Request{}, err
	}
	var r Request
	if decodeInputRecord(data, &r, requestFields) != nil || r.Version != ProtocolVersion ||
		!instancePattern.MatchString(r.ID) || r.CreatedAt <= 0 || r.Text != "" ||
		r.IndexKey != "" && r.IndexKey+".index.json" != name {
		return Request{}, ErrInputUncertain
	}
	return r, nil
}

func (q *inputQueue) pendingMatches(r Request) bool {
	body, err := q.read(r.ID+".pending.json", 2<<20)
	var pending Request
	return err == nil && decodeInputRecord(body, &pending, requestFields) == nil && pending == r.persisted() &&
		q.missing(r.ID+".claimed.json") && q.missing(r.ID+".receipt.json")
}

func (i InputIntent) Publish(now time.Time) (Request, error) {
	return i.publish(now, syncInput)
}

func (i InputIntent) publish(now time.Time, ops inputDurability) (Request, error) {
	r := i.Request
	lease, err := state.TryLease("terminal-input")
	if err != nil {
		return Request{}, err
	}
	defer lease.Close()
	store, err := resolveInputStore(state.Dir())
	if err != nil || r.store.path != store.path || r.store.token != store.token ||
		r.store.directory != (inputIdentity{}) && r.store != store {
		return Request{}, ErrInputStoreUnavailable
	}
	if store.directory == (inputIdentity{}) {
		if err := state.MkdirAll(store.path, 0700); err != nil {
			return Request{}, err
		}
		current, err := resolveInputStore(state.Dir())
		if err != nil || current.path != store.path || current.token != store.token || current.directory == (inputIdentity{}) {
			return Request{}, ErrInputStoreUnavailable
		}
		store = current
	}
	r.store = store
	queue, err := openInputQueue(store, ops)
	if err != nil {
		return Request{}, err
	}
	defer queue.dir.Close()
	old, err := queue.loadIndex(i.name)
	if i.previous.ID != "" && (err != nil || old != i.previous) ||
		i.previous.ID == "" && !errors.Is(err, os.ErrNotExist) {
		return Request{}, ErrInputUncertain
	}
	if i.reuse {
		if receipt, err := queue.committedReceipt(r); err == nil {
			if receipt.State == "accepted" {
				return r, nil
			}
			if receipt.State == "refused" {
				return Request{}, &InputRefused{Reason: receipt.Error}
			}
		} else if errors.Is(err, os.ErrNotExist) && !errors.Is(err, ErrInputUncertain) && queue.pendingMatches(r) {
			return r, nil
		}
		return Request{}, ErrInputUncertain
	}
	if old.ID != "" {
		removed, err := queue.reconcileInputAt(i.name, now, true)
		if err != nil {
			return Request{}, err
		}
		if !removed {
			return Request{}, ErrInputUncertain
		}
	}
	if err := queue.pruneInput(now); err != nil {
		return Request{}, err
	}
	if err := admitInput(store.path); err != nil {
		return Request{}, err
	}
	if err := queue.syncParents(); err != nil {
		return Request{}, err
	}
	r.CreatedAt = now.UnixMilli()
	index := r
	index.Text = ""
	indexData, err := json.Marshal(index)
	if err != nil || len(indexData) > 8192 {
		return Request{}, ErrInputStoreUnavailable
	}
	pending, err := json.Marshal(r)
	if err != nil || len(pending) > 2<<20 {
		return Request{}, ErrInputStoreUnavailable
	}
	if err := queue.publishDocument(i.name, indexData); err != nil {
		return Request{}, errors.Join(ErrInputUncertain, err)
	}
	if err := queue.publishDocument(r.ID+".pending.json", pending); err != nil {
		return Request{}, errors.Join(ErrInputUncertain, err)
	}
	return r, r.CheckInputStore()
}
