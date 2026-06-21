package app

import (
	"math/big"
	"testing"

	embedded "github.com/0x3639/znn-sdk-go/api/embedded"
	"github.com/zenon-network/go-zenon/common/types"
)

func TestFusionEntryDTORevocable(t *testing.T) {
	addr, _ := types.ParseAddress("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz")
	id := types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	e := &embedded.FusionEntry{QsrAmount: big.NewInt(10_000_000_000), Beneficiary: addr, ExpirationHeight: 100, Id: id}

	// frontier below expiration → not revocable
	d := fusionEntryDTO(e, 50)
	if d.IsRevocable {
		t.Fatal("should not be revocable below expiration")
	}
	if d.Beneficiary != addr.String() || d.ExpirationHeight != 100 {
		t.Fatalf("bad mapping: %+v", d)
	}
	// frontier at/above expiration → revocable
	if !fusionEntryDTO(e, 100).IsRevocable {
		t.Fatal("should be revocable at expiration")
	}
	if !fusionEntryDTO(e, 150).IsRevocable {
		t.Fatal("should be revocable above expiration")
	}
}

func TestPrepareFuseValidatesInput(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	// Bad beneficiary and bad amount are rejected BEFORE any node/client use.
	if _, err := s.PrepareFuse("not-an-address", "100"); err == nil {
		t.Fatal("expected invalid beneficiary to be rejected")
	}
	if _, err := s.PrepareFuse("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz", "0"); err == nil {
		t.Fatal("expected zero amount to be rejected")
	}
	if _, err := s.PrepareFuse("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz", "abc"); err == nil {
		t.Fatal("expected non-numeric amount to be rejected")
	}
}

func TestPrepareCancelFuseValidatesInput(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	if _, err := s.PrepareCancelFuse("not-a-hash"); err == nil {
		t.Fatal("expected invalid id to be rejected")
	}
}
