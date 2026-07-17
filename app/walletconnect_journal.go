package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/zenon-network/go-zenon/chain/nom"
)

// The WalletConnect publication journal (WC-01) is the durable exactly-once
// boundary for dapp-initiated transactions. A signed block is persisted here
// BEFORE its first broadcast attempt, so an uncertain RPC outcome or an app
// exit can never lose the only copy of a possibly-published block — and a
// replayed request (SignClient redelivers pending requests across restarts)
// resolves to the stored outcome instead of building a fresh block against the
// advanced frontier. It lives in the backend data directory: the WebView is
// never the funds-safety authority.

const (
	wcJournalFile       = "walletconnect-publications.json"
	wcJournalMaxRecords = 32
)

type wcPublicationState string

const (
	// wcStateSigned: the finalized signed block is persisted; broadcast has not
	// been CONFIRMED accepted. After a failed broadcast attempt the record stays
	// in this state — the outcome is unknown, never "definitely failed".
	wcStateSigned wcPublicationState = "signed"
	// wcStatePublished: the node accepted the broadcast, or reconciliation
	// found the block on chain.
	wcStatePublished wcPublicationState = "published"
)

type wcPublicationRecord struct {
	IntentHash string             `json:"intentHash"`
	State      wcPublicationState `json:"state"`
	BlockJSON  json.RawMessage    `json:"blockJson,omitempty"` // the exact signed block; public material only
	Hash       string             `json:"hash"`
	CreatedAt  int64              `json:"createdAt"`
}

// wcRequestIdentity binds a hold to the WalletConnect request it answers.
type wcRequestIdentity struct {
	Topic      string
	ID         uint64
	IntentHash string
}

func wcJournalKey(topic string, requestID uint64) string {
	return fmt.Sprintf("%s#%d", topic, requestID)
}

// walletConnectIntentHash canonically hashes the VALIDATED, reconstructed
// intent (never raw dapp JSON), so a reused request id carrying different
// funds-moving fields fails closed.
func walletConnectIntentHash(template *nom.AccountBlock) string {
	h := sha256.New()
	fmt.Fprintf(h, "%d|%d|%s|%s|%s|%s|",
		template.ChainIdentifier, template.BlockType,
		template.Address.String(), template.ToAddress.String(),
		template.Amount.String(), template.TokenStandard.String())
	h.Write(template.Data)
	return hex.EncodeToString(h.Sum(nil))
}

type wcJournal struct {
	mu  sync.Mutex
	dir func() (string, error)
}

func newWCJournal(dir func() (string, error)) *wcJournal {
	return &wcJournal{dir: dir}
}

func (j *wcJournal) path() (string, error) {
	d, err := j.dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, wcJournalFile), nil
}

// loadLocked reads all records; a missing file is an empty journal.
func (j *wcJournal) loadLocked() (map[string]wcPublicationRecord, error) {
	p, err := j.path()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return map[string]wcPublicationRecord{}, nil
	}
	if err != nil {
		return nil, err
	}
	m := map[string]wcPublicationRecord{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("corrupt WalletConnect publication journal: %w", err)
	}
	return m, nil
}

// saveLocked persists atomically: temp file in the same directory, fsync, then
// rename — an interrupted write can never leave truncated JSON behind (same
// pattern as settings.json).
func (j *wcJournal) saveLocked(m map[string]wcPublicationRecord) error {
	p, err := j.path()
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), "wcjournal-*.tmp")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), p)
}

func (j *wcJournal) load() (map[string]wcPublicationRecord, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.loadLocked()
}

func (j *wcJournal) get(key string) (wcPublicationRecord, bool, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	m, err := j.loadLocked()
	if err != nil {
		return wcPublicationRecord{}, false, err
	}
	rec, ok := m[key]
	return rec, ok, nil
}

func (j *wcJournal) put(key string, rec wcPublicationRecord) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	m, err := j.loadLocked()
	if err != nil {
		return err
	}
	m[key] = rec
	j.evictLocked(m, key)
	return j.saveLocked(m)
}

// evictLocked bounds retention. Published (delivered-or-deliverable) records
// evict before signed/unknown ones — an unknown outcome is duplicate
// protection and must be the last thing dropped. The just-written key is never
// evicted.
func (j *wcJournal) evictLocked(m map[string]wcPublicationRecord, keep string) {
	for len(m) > wcJournalMaxRecords {
		type kv struct {
			key string
			rec wcPublicationRecord
		}
		candidates := make([]kv, 0, len(m))
		for k, r := range m {
			if k == keep {
				continue
			}
			candidates = append(candidates, kv{k, r})
		}
		if len(candidates) == 0 {
			return
		}
		sort.Slice(candidates, func(a, b int) bool {
			ra, rb := candidates[a].rec, candidates[b].rec
			if (ra.State == wcStatePublished) != (rb.State == wcStatePublished) {
				return ra.State == wcStatePublished
			}
			return ra.CreatedAt < rb.CreatedAt
		})
		delete(m, candidates[0].key)
	}
}

func (j *wcJournal) markPublished(key string) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	m, err := j.loadLocked()
	if err != nil {
		return err
	}
	rec, ok := m[key]
	if !ok {
		return fmt.Errorf("no journal record for %s", key)
	}
	rec.State = wcStatePublished
	m[key] = rec
	return j.saveLocked(m)
}

func (j *wcJournal) delete(key string) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	m, err := j.loadLocked()
	if err != nil {
		return err
	}
	if _, ok := m[key]; !ok {
		return nil
	}
	delete(m, key)
	return j.saveLocked(m)
}
