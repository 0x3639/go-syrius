package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	Topic      string             `json:"topic"`
	RequestID  uint64             `json:"requestId"`
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
	raw, err := os.ReadFile(p) // #nosec G304 -- constant filename within the app data dir
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
	// Backfill record ownership from the map key. Records written before the
	// topic/requestId fields existed deserialize with empty/zero ownership; the
	// key (`topic#requestId`) is authoritative, so derive it here. This keeps
	// intent-matching (and the new-id duplicate defense) working for durable
	// records already on disk.
	for k, rec := range m {
		if rec.Topic != "" && rec.RequestID != 0 {
			continue
		}
		if topic, id, ok := parseWCJournalKey(k); ok {
			rec.Topic, rec.RequestID = topic, id
			m[k] = rec
		}
	}
	return m, nil
}

// parseWCJournalKey splits a `topic#requestId` key. A WalletConnect topic is a
// hex string with no '#', so the LAST '#' separates topic from the id.
func parseWCJournalKey(key string) (string, uint64, bool) {
	idx := strings.LastIndex(key, "#")
	if idx < 0 {
		return "", 0, false
	}
	id, err := strconv.ParseUint(key[idx+1:], 10, 64)
	if err != nil {
		return "", 0, false
	}
	return key[:idx], id, true
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
	// Every retained record is duplicate protection: a signed record is a
	// possibly-published block, and a published record is only deleted once
	// its result reached the dapp. Evicting either would let a redelivered
	// request build a fresh block. When the journal is full, REFUSE the new
	// write instead (which refuses the new broadcast — fail closed); updating
	// an existing key stays possible so reconciliation can always progress.
	if _, exists := m[key]; !exists && len(m) >= wcJournalMaxRecords {
		return fmt.Errorf("the WalletConnect publication journal holds %d unresolved outcomes; reconcile or acknowledge them before publishing new requests", len(m))
	}
	m[key] = rec
	return j.saveLocked(m)
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

// findByIntent returns a retained record whose intent matches intentHash within
// the SAME topic (session), excluding excludeKey. It exists so a dapp that
// reissues an identical transfer under a NEW request id — while a prior record
// for that intent is still retained (signed/unresolved, or published but not
// yet acknowledged) — is matched to that record instead of building a second
// block. Scoping to one topic avoids false-positives across unrelated dapps
// that happen to share an identical intent.
func (j *wcJournal) findByIntent(topic, intentHash, excludeKey string) (wcPublicationRecord, bool, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	m, err := j.loadLocked()
	if err != nil {
		return wcPublicationRecord{}, false, err
	}
	for k, rec := range m {
		if k == excludeKey || rec.Topic != topic || rec.IntentHash != intentHash {
			continue
		}
		return rec, true, nil
	}
	return wcPublicationRecord{}, false, nil
}

// findByIntentAnyTopic returns a retained record with the matching intent under
// ANY topic (excluding excludeKey and skipTopic, which the same-topic scan
// already covers). A cross-topic match is NOT auto-replayed — it may be an
// unrelated dapp — so the caller surfaces it as a blocking outcome the user
// must reconcile/clear, which still prevents a duplicate publication.
func (j *wcJournal) findByIntentAnyTopic(intentHash, excludeKey, skipTopic string) (wcPublicationRecord, bool, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	m, err := j.loadLocked()
	if err != nil {
		return wcPublicationRecord{}, false, err
	}
	for k, rec := range m {
		if k == excludeKey || rec.Topic == skipTopic || rec.IntentHash != intentHash {
			continue
		}
		return rec, true, nil
	}
	return wcPublicationRecord{}, false, nil
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
