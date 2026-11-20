package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
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
	m, err := decodeWCJournal(raw)
	if err != nil {
		return nil, fmt.Errorf("corrupt WalletConnect publication journal: %w", err)
	}
	return m, nil
}

// decodeWCJournal distinguishes an empty journal from lost or ambiguous data.
// In particular JSON null is valid JSON but is not an empty journal: treating
// it as one loses replay protection (and a later map assignment would panic).
func decodeWCJournal(raw []byte) (map[string]wcPublicationRecord, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	start, err := dec.Token()
	if err != nil || start != json.Delim('{') {
		return nil, fmt.Errorf("expected a journal object")
	}
	m := make(map[string]wcPublicationRecord)
	for dec.More() {
		token, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := token.(string)
		if !ok {
			return nil, fmt.Errorf("invalid journal key")
		}
		if _, exists := m[key]; exists {
			return nil, fmt.Errorf("duplicate journal key %q", key)
		}
		if len(m) >= wcJournalMaxRecords {
			return nil, fmt.Errorf("journal exceeds its record limit")
		}
		var rawRecord json.RawMessage
		if err := dec.Decode(&rawRecord); err != nil {
			return nil, err
		}
		rec, err := decodeWCRecord(rawRecord)
		if err != nil {
			return nil, fmt.Errorf("invalid record %q: %w", key, err)
		}
		if err := validateWCRecord(key, &rec); err != nil {
			return nil, err
		}
		m[key] = rec
	}
	if end, err := dec.Token(); err != nil || end != json.Delim('}') {
		return nil, fmt.Errorf("incomplete journal object")
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("unexpected trailing journal data")
	}
	return m, nil
}

// Unmarshalling a struct silently chooses the last repeated field. Check the
// record first so "signed" cannot be shadowed by "published" before an ack.
func decodeWCRecord(raw []byte) (wcPublicationRecord, error) {
	var rec wcPublicationRecord
	dec := json.NewDecoder(bytes.NewReader(raw))
	start, err := dec.Token()
	if err != nil || start != json.Delim('{') {
		return rec, fmt.Errorf("expected a record object")
	}
	seen := make(map[string]bool)
	for dec.More() {
		token, err := dec.Token()
		if err != nil {
			return rec, err
		}
		key, ok := token.(string)
		if !ok {
			return rec, fmt.Errorf("invalid record field")
		}
		// encoding/json also accepts case-insensitive aliases of struct fields.
		for _, field := range []string{"topic", "requestId", "intentHash", "state", "blockJson", "hash", "createdAt"} {
			if strings.EqualFold(key, field) {
				key = field
				break
			}
		}
		if seen[key] {
			return rec, fmt.Errorf("duplicate record field %q", key)
		}
		seen[key] = true
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return rec, err
		}
	}
	// The regular decoder checks the closing delimiter, field types and trailing
	// data, retaining BlockJSON instead of normalizing its signed contents.
	err = json.Unmarshal(raw, &rec)
	return rec, err
}

func validateWCRecord(key string, rec *wcPublicationRecord) error {
	topic, id, ok := parseWCJournalKey(key)
	if !ok || topic == "" || id == 0 || wcJournalKey(topic, id) != key {
		return fmt.Errorf("invalid journal identity %q", key)
	}
	if (rec.Topic != "" && rec.Topic != topic) || (rec.RequestID != 0 && rec.RequestID != id) {
		return fmt.Errorf("journal ownership disagrees with key %q", key)
	}
	// Legacy records omitted ownership; the key supplies it. Conflicting
	// ownership is corruption, not a legacy record to silently rewrite.
	rec.Topic, rec.RequestID = topic, id
	if rec.State != wcStateSigned && rec.State != wcStatePublished {
		return fmt.Errorf("invalid publication state for %q", key)
	}
	if rec.IntentHash == "" || rec.Hash == "" {
		return fmt.Errorf("missing publication identity for %q", key)
	}
	// The SDK silently turns missing or malformed amount strings into zero.
	// Validate the original field first, preserving genuine zero-value calls.
	var encoded struct {
		Amount string `json:"amount"`
	}
	if err := json.Unmarshal(rec.BlockJSON, &encoded); err != nil {
		return fmt.Errorf("invalid journaled amount for %q: %w", key, err)
	}
	amount, ok := new(big.Int).SetString(encoded.Amount, 10)
	if !ok || amount.Sign() < 0 {
		return fmt.Errorf("invalid journaled amount for %q", key)
	}
	var block nom.AccountBlock
	if err := json.Unmarshal(rec.BlockJSON, &block); err != nil || block.Hash.String() != rec.Hash {
		return fmt.Errorf("missing or inconsistent journaled block for %q", key)
	}
	return nil
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
	if err := validateWCRecord(key, &rec); err != nil {
		return err
	}
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

// deletePublished removes a record ONLY once it is resolved (published). A
// signed record is a possibly-published block whose retention is the sole
// duplicate-protection guard; the frontend is untrusted, so an acknowledge for
// an unresolved record must be refused, not honored (it would let a
// redelivered request build and broadcast a second block for the same intent).
func (j *wcJournal) deletePublished(key string) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	m, err := j.loadLocked()
	if err != nil {
		return err
	}
	rec, ok := m[key]
	if !ok {
		return nil
	}
	if rec.State != wcStatePublished {
		return fmt.Errorf("the publication outcome for this request is unresolved; reconcile it before acknowledging")
	}
	delete(m, key)
	return j.saveLocked(m)
}
