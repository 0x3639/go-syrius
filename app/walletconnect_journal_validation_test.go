package app

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zenon-network/go-zenon/chain/nom"
	"github.com/zenon-network/go-zenon/common/types"
)

// Use the real account-block JSON schema even in journal-only tests.
func journalTestRecord(t testing.TB, payload int) wcPublicationRecord {
	t.Helper()
	hash := fmt.Sprintf("%064x", payload)
	block := &nom.AccountBlock{
		Version: 1, ChainIdentifier: 1, BlockType: nom.BlockTypeUserSend,
		Hash: types.HexToHashPanic(hash), Amount: big.NewInt(int64(payload)),
		Address:   types.ParseAddressPanic("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz"),
		ToAddress: types.BridgeContract, TokenStandard: types.ZnnTokenStandard,
		Data: []byte{byte(payload)},
	}
	raw, err := json.Marshal(block)
	if err != nil {
		t.Fatal(err)
	}
	return wcPublicationRecord{IntentHash: fmt.Sprintf("intent-%d", payload), State: wcStateSigned, Hash: hash, BlockJSON: raw, CreatedAt: int64(payload)}
}

type journalCorruptionCase struct {
	name, body string
	valid      bool
}

func journalCorruptionCases(t testing.TB) []journalCorruptionCase {
	t.Helper()
	rec := journalTestRecord(t, 1)
	rec.Topic, rec.RequestID = "topic", 1
	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	valid := string(raw)
	entry := `"topic#1":` + valid
	changed := func(old, new string) string {
		t.Helper()
		if !strings.Contains(entry, old) {
			t.Fatalf("missing corruption fixture target: %s", old)
		}
		return "{" + strings.Replace(entry, old, new, 1) + "}"
	}
	return []journalCorruptionCase{
		{"missing", "", true}, {"empty-object", "{}", true},
		{"null-document", "null", false}, {"truncated", "{", false},
		{"null-record", `{"topic#1":null}`, false},
		{"empty-record", `{"topic#1":{}}`, false},
		{"unknown-state", `{"topic#1":{"topic":"topic","requestId":1,"intentHash":"intent","hash":"hash","state":"maybe"}}`, false},
		{"unknown-state-with-payload", changed(`"state":"signed"`, `"state":"maybe"`), false},
		{"valid-record", "{" + entry + "}", true},
		{"legacy-ownership", `{"topic#1":` + strings.ReplaceAll(strings.ReplaceAll(valid, `"topic":"topic",`, ""), `"requestId":1,`, "") + "}", true},
		{"duplicate-key", "{" + entry + "," + entry + "}", false},
		{"foreign-topic", changed(`"topic":"topic"`, `"topic":"other"`), false},
		{"foreign-request", changed(`"requestId":1`, `"requestId":2`), false},
		{"noncanonical-key", changed(`topic#1`, `topic#01`), false},
		{"missing-payload", changed(`"blockJson":`+string(rec.BlockJSON), `"blockJson":null`), false},
		{"different-payload-hash", changed(`"blockJson":{"version":1,"chainIdentifier":1,"blockType":2,"hash":"`+rec.Hash+`"`, `"blockJson":{"version":1,"chainIdentifier":1,"blockType":2,"hash":"`+strings.Repeat("2", 64)+`"`), false},
		{"trailing-document", "{" + entry + "}{}", false},
		{"duplicate-state", changed(`"state":"signed"`, `"state":"signed","state":"published"`), false},
		{"duplicate-state-alias", changed(`"state":"signed"`, `"state":"signed","State":"published"`), false},
		{"malformed-chain", changed(`"chainIdentifier":1`, `"chainIdentifier":"bad"`), false},
		{"malformed-nonce", changed(`"nonce":"0000000000000000"`, `"nonce":"bad"`), false},
		{"missing-block-hash", changed(`"hash":"`+rec.Hash+`"`, `"ignoredHash":"`+rec.Hash+`"`), false},
		{"malformed-amount", changed(`"amount":"1"`, `"amount":"bad"`), false},
		{"malformed-published-amount", strings.Replace(changed(`"amount":"1"`, `"amount":"bad"`), `"state":"signed"`, `"state":"published"`, 1), false},
		{"empty-amount", changed(`"amount":"1"`, `"amount":""`), false},
		{"missing-amount", changed(`"amount":"1"`, `"ignoredAmount":"1"`), false},
		{"null-amount", changed(`"amount":"1"`, `"amount":null`), false},
		{"numeric-amount", changed(`"amount":"1"`, `"amount":1`), false},
		{"fractional-amount", changed(`"amount":"1"`, `"amount":"1.5"`), false},
		{"negative-amount", changed(`"amount":"1"`, `"amount":"-1"`), false},
		{"zero-amount", changed(`"amount":"1"`, `"amount":"0"`), true},
		{"large-amount", changed(`"amount":"1"`, `"amount":"900719925474099312345678901234567890"`), true},
	}
}

func TestWCJournalCorruptStorageIsPreserved(t *testing.T) {
	for _, tc := range journalCorruptionCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, wcJournalFile)
			if tc.body != "" {
				if err := os.WriteFile(path, []byte(tc.body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			j := newWCJournal(func() (string, error) { return dir, nil })
			_, err := j.load()
			if (err == nil) != tc.valid {
				t.Errorf("load accepted=%v, want %v: %v", err == nil, tc.valid, err)
			}
			if !tc.valid {
				for _, action := range []struct {
					name string
					run  func() error
				}{
					{"put", func() error { return j.put("topic#1", journalTestRecord(t, 1)) }},
					{"ack", func() error { return j.deletePublished("topic#1") }},
				} {
					// Test each operation against the original corrupt bytes, even
					// when the other operation accidentally succeeds and rewrites it.
					if err := os.WriteFile(path, []byte(tc.body), 0600); err != nil {
						t.Fatal(err)
					}
					if action.run() == nil {
						t.Errorf("%s accepted corrupt storage", action.name)
					}
					raw, err := os.ReadFile(path)
					if err != nil || string(raw) != tc.body {
						t.Errorf("%s changed corrupt journal: %v", action.name, err)
					}
				}
			}
		})
	}
}
