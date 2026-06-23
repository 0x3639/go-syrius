package app

import (
	"math/big"
	"testing"

	embedded "github.com/0x3639/znn-sdk-go/api/embedded"
	nom "github.com/zenon-network/go-zenon/chain/nom"
	"github.com/zenon-network/go-zenon/common/types"
	constants "github.com/zenon-network/go-zenon/vm/constants"
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

func TestStakeEntryDTO(t *testing.T) {
	addr, _ := types.ParseAddress("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz")
	id := types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	start := int64(1_700_000_000)
	const unit = int64(2_592_000)
	// 3-month stake
	e := &embedded.StakeEntry{
		Amount:              big.NewInt(500_000_000), // 5 ZNN
		StartTimestamp:      start,
		ExpirationTimestamp: start + 3*unit,
		Address:             addr,
		Id:                  id,
	}
	// before expiration → not matured
	d := stakeEntryDTO(e, start+unit)
	if d.IsMatured {
		t.Fatal("should not be matured before expiration")
	}
	if d.DurationMonths != 3 {
		t.Fatalf("DurationMonths = %d, want 3", d.DurationMonths)
	}
	if d.Amount != "500000000" || d.Id != id.String() {
		t.Fatalf("bad mapping: %+v", d)
	}
	// at/after expiration → matured
	if !stakeEntryDTO(e, start+3*unit).IsMatured {
		t.Fatal("should be matured at expiration")
	}
	if !stakeEntryDTO(e, start+10*unit).IsMatured {
		t.Fatal("should be matured after expiration")
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

func TestPrepareStakeValidatesInput(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	// amount below 1 ZNN min, non-numeric amount, and bad duration are rejected before any node use.
	if _, err := s.PrepareStake("50000000", "3"); err == nil { // 0.5 ZNN < 1 ZNN min
		t.Fatal("expected below-min amount to be rejected")
	}
	if _, err := s.PrepareStake("abc", "3"); err == nil {
		t.Fatal("expected non-numeric amount to be rejected")
	}
	if _, err := s.PrepareStake("100000000", "0"); err == nil {
		t.Fatal("expected duration 0 to be rejected")
	}
	if _, err := s.PrepareStake("100000000", "13"); err == nil {
		t.Fatal("expected duration 13 to be rejected")
	}
	if _, err := s.PrepareCancelStake("not-a-hash"); err == nil {
		t.Fatal("expected bad id to be rejected")
	}
}

func TestStakeTemplateTokenStandards(t *testing.T) {
	api := embedded.NewStakeApi(nil) // builders construct blocks from args; no client deref
	id := types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	for name, b := range map[string]*nom.AccountBlock{
		"stake":   api.Stake(stakeTimeUnitSec, big.NewInt(100_000_000)),
		"cancel":  api.Cancel(id),
		"collect": api.CollectReward(),
	} {
		if b.ToAddress != types.StakeContract {
			t.Fatalf("%s: ToAddress=%v want StakeContract", name, b.ToAddress)
		}
		if b.TokenStandard != types.ZnnTokenStandard {
			t.Fatalf("%s: TokenStandard=%v want ZNN", name, b.TokenStandard)
		}
	}
}

func TestPillarSummaryDTO(t *testing.T) {
	owner, _ := types.ParseAddress("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz")
	p := &embedded.PillarInfo{
		Name:                         "Pillar-A",
		Rank:                         3,
		GiveDelegateRewardPercentage: 90,
		ProducerAddress:              owner,
		Weight:                       big.NewInt(1_500_000_000_000),
	}
	d := pillarSummaryDTO(p)
	if d.Name != "Pillar-A" || d.Rank != 3 || d.DelegateRewardPercent != 90 {
		t.Fatalf("bad mapping: %+v", d)
	}
	if d.Weight != "1500000000000" || d.ProducerAddress != owner.String() {
		t.Fatalf("bad weight/producer: %+v", d)
	}
	// nil Weight → "0"
	if pillarSummaryDTO(&embedded.PillarInfo{Name: "B"}).Weight != "0" {
		t.Fatal("nil weight should map to 0")
	}
}

func TestSortPillarsByRank(t *testing.T) {
	in := []PillarSummary{{Name: "c", Rank: 5}, {Name: "a", Rank: 1}, {Name: "b", Rank: 3}}
	sortPillarsByRank(in)
	if in[0].Name != "a" || in[1].Name != "b" || in[2].Name != "c" {
		t.Fatalf("not sorted by rank: %+v", in)
	}
}

// TestPlasmaTemplateTokenStandards locks in the SDK template token-standard
// expectations our callExpects rely on. The callExpect zts passed to
// prepareCall MUST equal the SDK template's TokenStandard, or
// TxService.ConfirmPublish's assertMatches rejects the published block.
//
// Built against the REAL SDK template builders: PlasmaApi.Fuse / .Cancel
// construct a *nom.AccountBlock from the receiver only (they do not touch
// pa.client), so embedded.NewPlasmaApi(nil) runs fully offline. We avoid
// rpc_client.NewRpcClient here because it dials (server.Dial) at construction
// and cannot run offline.
//
// Fuse uses QSR; Cancel uses ZNN — a real, asymmetric SDK behavior. This test
// fails if either the SDK changes or our PrepareFuse/PrepareCancelFuse zts
// drifts away from the template.
func TestPlasmaTemplateTokenStandards(t *testing.T) {
	pa := embedded.NewPlasmaApi(nil)
	addr, _ := types.ParseAddress("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz")
	id := types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")

	// Fuse template uses QSR — PrepareFuse's callExpect.zts must match.
	fuse := pa.Fuse(addr, big.NewInt(100))
	if fuse.TokenStandard != types.QsrTokenStandard {
		t.Fatalf("Fuse template zts=%v, want QSR %v", fuse.TokenStandard, types.QsrTokenStandard)
	}

	// Cancel template uses ZNN (NOT QSR) — PrepareCancelFuse's callExpect.zts
	// must match. This is the bug this test guards against.
	cancel := pa.Cancel(id)
	if cancel.TokenStandard != types.ZnnTokenStandard {
		t.Fatalf("Cancel template zts=%v, want ZNN %v", cancel.TokenStandard, types.ZnnTokenStandard)
	}

	// Sanity: the two standards are genuinely distinct, otherwise the above
	// assertions would be vacuous.
	if types.QsrTokenStandard == types.ZnnTokenStandard {
		t.Fatal("QSR and ZNN token standards must be distinct")
	}
}

func TestPrepareDelegateValidatesInput(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	// empty / whitespace name rejected before any node use.
	if _, err := s.PrepareDelegate(""); err == nil {
		t.Fatal("expected empty name to be rejected")
	}
	if _, err := s.PrepareDelegate("   "); err == nil {
		t.Fatal("expected whitespace name to be rejected")
	}
}

func TestPillarTemplateTokenStandards(t *testing.T) {
	api := embedded.NewPillarApi(nil) // builders construct blocks from args; no client deref
	for name, b := range map[string]*nom.AccountBlock{
		"delegate":   api.Delegate("Pillar-A"),
		"undelegate": api.Undelegate(),
		"collect":    api.CollectReward(),
	} {
		if b.ToAddress != types.PillarContract {
			t.Fatalf("%s: ToAddress=%v want PillarContract", name, b.ToAddress)
		}
		if b.TokenStandard != types.ZnnTokenStandard {
			t.Fatalf("%s: TokenStandard=%v want ZNN", name, b.TokenStandard)
		}
		if b.Amount == nil || b.Amount.Sign() != 0 {
			t.Fatalf("%s: Amount=%v want 0", name, b.Amount)
		}
	}
}

func TestPrepareDepositQsrValidatesInput(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	// zero / negative / unparseable rejected before any node use.
	for _, bad := range []string{"0", "-1", "", "abc"} {
		if _, err := s.PrepareDepositQsr(bad); err == nil {
			t.Fatalf("expected %q to be rejected", bad)
		}
	}
}

func TestSentinelTemplateTokenStandards(t *testing.T) {
	api := embedded.NewSentinelApi(nil) // builders construct blocks from args/constants; no client deref
	znn := types.ZnnTokenStandard.String()
	qsr := types.QsrTokenStandard.String()
	cases := []struct {
		name     string
		b        *nom.AccountBlock
		wantZts  string
		wantZero bool // Amount must be exactly 0
	}{
		{"deposit", api.DepositQsr(big.NewInt(123)), qsr, false},
		{"register", api.Register(), znn, false},
		{"revoke", api.Revoke(), znn, true},
		{"withdraw", api.WithdrawQsr(), znn, true},
		{"collect", api.CollectReward(), znn, true},
	}
	for _, c := range cases {
		if c.b.ToAddress != types.SentinelContract {
			t.Fatalf("%s: ToAddress=%v want SentinelContract", c.name, c.b.ToAddress)
		}
		if c.b.TokenStandard.String() != c.wantZts {
			t.Fatalf("%s: TokenStandard=%v want %v", c.name, c.b.TokenStandard.String(), c.wantZts)
		}
		if c.wantZero && (c.b.Amount == nil || c.b.Amount.Sign() != 0) {
			t.Fatalf("%s: Amount=%v want 0", c.name, c.b.Amount)
		}
	}
	// Register must carry the 5,000 ZNN collateral (5000 * 1e8).
	if api.Register().Amount.String() != "500000000000" {
		t.Fatalf("register amount=%v want 500000000000", api.Register().Amount)
	}
}

func TestSentinelDTO(t *testing.T) {
	owner, _ := types.ParseAddress("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz")
	s := &embedded.SentinelInfo{
		Owner:                 owner,
		RegistrationTimestamp: 1718000000,
		IsRevocable:           true,
		RevokeCooldown:        0,
		Active:                true,
	}
	d := sentinelDTO(s)
	if d.Owner != owner.String() || d.RegistrationTimestamp != 1718000000 {
		t.Fatalf("bad mapping: %+v", d)
	}
	if !d.IsRevocable || !d.Active {
		t.Fatalf("bad flags: %+v", d)
	}
	// no sentinel: nil → empty Owner
	if sentinelDTO(nil).Owner != "" {
		t.Fatal("nil should map to empty Owner")
	}
	// no sentinel: zero RegistrationTimestamp → empty Owner (treated as none)
	if sentinelDTO(&embedded.SentinelInfo{Owner: owner}).Owner != "" {
		t.Fatal("zero RegistrationTimestamp should map to empty Owner")
	}
}

func TestTokenInfoDTO(t *testing.T) {
	owner, _ := types.ParseAddress("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz")
	zts, _ := types.ParseZTS("zts1znnxxxxxxxxxxxxx9z4ulx")
	tok := &embedded.Token{
		Name: "Test Token", Symbol: "TEST", Domain: "test.org",
		TotalSupply: big.NewInt(1000), MaxSupply: big.NewInt(2000),
		Decimals: 8, Owner: owner, TokenStandard: zts,
		IsMintable: true, IsBurnable: false, IsUtility: true,
	}
	d := tokenInfoDTO(tok)
	if d.Name != "Test Token" || d.Symbol != "TEST" || d.Domain != "test.org" {
		t.Fatalf("bad strings: %+v", d)
	}
	if d.TotalSupply != "1000" || d.MaxSupply != "2000" || d.Decimals != 8 {
		t.Fatalf("bad supply/decimals: %+v", d)
	}
	if d.Owner != owner.String() || d.TokenStandard != zts.String() {
		t.Fatalf("bad owner/zts: %+v", d)
	}
	if !d.IsMintable || d.IsBurnable || !d.IsUtility {
		t.Fatalf("bad flags: %+v", d)
	}
	// nil supplies → "0"
	z := tokenInfoDTO(&embedded.Token{Name: "X"})
	if z.TotalSupply != "0" || z.MaxSupply != "0" {
		t.Fatalf("nil supplies should be 0: %+v", z)
	}
}

func TestPrepareIssueTokenValidatesInput(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	// each call must be rejected BEFORE any node use (node is not connected in this test,
	// but validation runs first, so we assert a validation error, not "not connected").
	cases := []struct {
		name                   string
		tn, ts, td, total, max string
		decimals               int
		mintable               bool
	}{
		{"empty name", "", "TEST", "", "100", "100", 8, false},
		{"bad name char", "bad name", "TEST", "", "100", "100", 8, false},
		{"empty symbol", "Tok", "", "", "100", "100", 8, false},
		{"lowercase symbol", "Tok", "test", "", "100", "100", 8, false},
		{"bad domain", "Tok", "TEST", "not_a_domain", "100", "100", 8, false},
		{"decimals too high", "Tok", "TEST", "", "100", "100", 19, false},
		{"decimals negative", "Tok", "TEST", "", "100", "100", -1, false},
		{"maxSupply zero", "Tok", "TEST", "", "0", "0", 8, true},
		{"max < total", "Tok", "TEST", "", "200", "100", 8, true},
		{"non-mintable max != total", "Tok", "TEST", "", "100", "200", 8, false},
		{"unparseable total", "Tok", "TEST", "", "abc", "100", 8, true},
	}
	for _, c := range cases {
		if _, err := s.PrepareIssueToken(c.tn, c.ts, c.td, c.total, c.max, c.decimals, c.mintable, true, false); err == nil {
			t.Fatalf("%s: expected validation error", c.name)
		}
	}
	// a valid set must pass validation and fail only on the not-connected node.
	_, err := s.PrepareIssueToken("Valid-Token", "VALID", "valid.org", "100", "100", 8, false, true, false)
	if err == nil || err.Error() != "not connected" {
		t.Fatalf("valid input should pass validation and hit not-connected; got %v", err)
	}
}

func TestPrepareMintBurnUpdateValidateInput(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	good := types.ZnnTokenStandard.String()
	addr := "z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz"
	// Mint: bad zts, non-positive amount, bad receiver
	if _, err := s.PrepareMint("bad", "1", addr); err == nil {
		t.Fatal("mint: bad zts must error")
	}
	if _, err := s.PrepareMint(good, "0", addr); err == nil {
		t.Fatal("mint: zero amount must error")
	}
	if _, err := s.PrepareMint(good, "1", "notanaddr"); err == nil {
		t.Fatal("mint: bad receiver must error")
	}
	// Burn: bad zts, non-positive amount
	if _, err := s.PrepareBurn("bad", "1"); err == nil {
		t.Fatal("burn: bad zts must error")
	}
	if _, err := s.PrepareBurn(good, "-1"); err == nil {
		t.Fatal("burn: negative amount must error")
	}
	// Update: bad zts, bad owner
	if _, err := s.PrepareUpdateToken("bad", addr, true, true); err == nil {
		t.Fatal("update: bad zts must error")
	}
	if _, err := s.PrepareUpdateToken(good, "notanaddr", true, true); err == nil {
		t.Fatal("update: bad owner must error")
	}
}

func TestTokenTemplateTokenStandards(t *testing.T) {
	api := embedded.NewTokenApi(nil) // builders construct blocks from args/constants; no client deref
	zts := types.ZnnTokenStandard
	recv, _ := types.ParseAddress("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz")
	amt := big.NewInt(123)

	issue := api.IssueToken("Tok", "TEST", "", big.NewInt(100), big.NewInt(100), 8, true, true, false)
	if issue.ToAddress != types.TokenContract || issue.TokenStandard != types.ZnnTokenStandard {
		t.Fatalf("issue: wrong to/zts: %+v", issue)
	}
	if issue.Amount.String() != constants.TokenIssueAmount.String() {
		t.Fatalf("issue amount=%v want %v", issue.Amount, constants.TokenIssueAmount)
	}

	mint := api.Mint(zts, amt, recv)
	if mint.ToAddress != types.TokenContract || mint.TokenStandard != types.ZnnTokenStandard || mint.Amount.Sign() != 0 {
		t.Fatalf("mint: wrong to/zts/amount: %+v", mint)
	}

	update := api.UpdateToken(zts, recv, true, true)
	if update.ToAddress != types.TokenContract || update.TokenStandard != types.ZnnTokenStandard || update.Amount.Sign() != 0 {
		t.Fatalf("update: wrong to/zts/amount: %+v", update)
	}

	// BURN is the dynamic one: zts = the token being burned, amount = the burn amount.
	burn := api.Burn(zts, amt)
	if burn.ToAddress != types.TokenContract {
		t.Fatalf("burn: wrong to: %+v", burn)
	}
	if burn.TokenStandard != zts {
		t.Fatalf("burn: TokenStandard=%v want the burned token %v", burn.TokenStandard, zts)
	}
	if burn.Amount.Cmp(amt) != 0 {
		t.Fatalf("burn: Amount=%v want %v", burn.Amount, amt)
	}
}
