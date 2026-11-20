package app

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/zenon-network/go-zenon/chain/nom"
	"github.com/zenon-network/go-zenon/common/types"
	"github.com/zenon-network/go-zenon/vm/embedded/definition"
)

func FuzzWCJournalDecode(f *testing.F) {
	for _, seed := range []string{
		`{}`, `null`, `{"topic#1":null}`, `{"topic#1":{}}`,
		`{"topic#1":{"intentHash":"intent","state":"signed","hash":"hash","blockJson":{"hash":"hash"}}}`,
		`{"topic#1":{"topic":"other","requestId":1,"intentHash":"intent","state":"published","hash":"hash","blockJson":{"hash":"hash"}}}`,
		`{"topic#1":{"intentHash":"intent","state":"signed","hash":"hash","blockJson":{"hash":"hash","n":1e400}}}`,
	} {
		f.Add([]byte(seed))
	}
	f.Add(journalLargeNumberFixture(f))
	f.Fuzz(checkWCJournalRoundTrip)
}

func journalLargeNumberFixture(t testing.TB) []byte {
	t.Helper()
	rec := journalTestRecord(t, 1)
	rec.BlockJSON = json.RawMessage(strings.TrimSuffix(string(rec.BlockJSON), "}") + `,"n":1e400,"precise":9007199254740993}`)
	raw, err := json.Marshal(map[string]wcPublicationRecord{"topic#1": rec})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestWCJournalLargeNumbersRoundTrip(t *testing.T) {
	raw := journalLargeNumberFixture(t)
	if _, err := decodeWCJournal(raw); err != nil {
		t.Fatal(err)
	}
	checkWCJournalRoundTrip(t, raw)
}

func checkWCJournalRoundTrip(t *testing.T, raw []byte) {
	if len(raw) > 64*1024 {
		t.Skip()
	}
	records, err := decodeWCJournal(raw)
	if err != nil {
		return
	}
	if records == nil || len(records) > wcJournalMaxRecords {
		t.Fatal("accepted unusable journal")
	}
	// Accepted storage must survive the exact write/read cycle used by the
	// application, without losing entries, ownership, or block JSON content.
	encoded, err := json.Marshal(records)
	if err != nil {
		t.Fatal(err)
	}
	again, err := decodeWCJournal(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(raw) || !json.Valid(encoded) {
		t.Fatal("decoder accepted non-JSON")
	}
	for key, rec := range records {
		other := again[key]
		var a, b any
		left := json.NewDecoder(bytes.NewReader(rec.BlockJSON))
		right := json.NewDecoder(bytes.NewReader(other.BlockJSON))
		left.UseNumber()
		right.UseNumber()
		if left.Decode(&a) != nil || right.Decode(&b) != nil || !reflect.DeepEqual(a, b) {
			t.Fatal("journal round trip changed signed payload")
		}
		rec.BlockJSON, other.BlockJSON = nil, nil
		if !reflect.DeepEqual(rec, other) {
			t.Fatal("journal round trip changed record identity")
		}
	}
}

func FuzzWalletConnectIntent(f *testing.F) {
	wrap := definition.ABIBridge.PackMethodPanic(definition.WrapTokenMethodName, uint32(2), uint32(1), "0x1111111111111111111111111111111111111111")
	f.Add(wrap, "100000000", types.ZnnTokenStandard.String())
	f.Add(wrap, "9007199254740993", types.QsrTokenStandard.String())
	f.Add([]byte{}, "0", "invalid")
	f.Add([]byte{255, 255, 255, 255}, "-1", types.ZnnTokenStandard.String())
	f.Fuzz(func(t *testing.T, data []byte, amount, token string) {
		if len(data)+len(amount)+len(token) > 64*1024 {
			t.Skip()
		}
		from := types.ParseAddressPanic("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz")
		req := WalletConnectSendRequest{FromAddress: from.String(), AccountBlock: WalletConnectAccountBlockInput{
			Version: 1, ChainIdentifier: 1, BlockType: uint64(nom.BlockTypeUserSend),
			ToAddress: types.BridgeContract.String(), Amount: amount, TokenStandard: token,
			Data: base64.StdEncoding.EncodeToString(data),
		}}
		block, expect, effect, err := walletConnectValidateIntent(req, from)
		if err != nil {
			return
		}
		value, ok := new(big.Int).SetString(amount, 10)
		if !ok || block.Amount == nil || block.Amount.Cmp(value) != 0 || block.Address != from || block.ToAddress != types.BridgeContract || !reflect.DeepEqual(block.Data, data) {
			t.Fatal("accepted request changed the intended effect")
		}
		if err := assertMatches(block, expect); err != nil {
			t.Fatal(err)
		}
		switch effect.Method {
		case definition.WrapTokenMethodName:
			if value.Sign() <= 0 {
				t.Fatal("accepted unfunded Wrap")
			}
		case definition.RedeemUnwrapMethodName:
			if value.Sign() != 0 || block.TokenStandard != types.ZnnTokenStandard {
				t.Fatal("accepted funded or non-ZNN Redeem")
			}
		default:
			t.Fatal("accepted privileged or unknown bridge method")
		}
	})
}
