package app

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	embedded "github.com/0x3639/znn-sdk-go/api/embedded"
	"github.com/zenon-network/go-zenon/common/types"
	api "github.com/zenon-network/go-zenon/rpc/api"
)

// formatBaseAmount renders a base-unit integer string as a human decimal string
// with the given number of decimals (trailing zeros trimmed), e.g.
// formatBaseAmount("10000000000", 8) == "100".
func formatBaseAmount(base string, decimals int) string {
	neg := strings.HasPrefix(base, "-")
	digits := base
	if neg {
		digits = base[1:]
	}
	for len(digits) <= decimals {
		digits = "0" + digits
	}
	intPart := digits[:len(digits)-decimals]
	frac := strings.TrimRight(digits[len(digits)-decimals:], "0")
	out := intPart
	if frac != "" {
		out = intPart + "." + frac
	}
	if neg {
		out = "-" + out
	}
	return out
}

// NomService exposes Network-of-Momentum embedded-contract reads and builds
// state-changing templates that it hands to TxService for confirm/publish.
// No key material passes through NomService.
type NomService struct {
	node   *NodeService
	wallet *WalletService
	tx     *TxService
}

func newNomService(node *NodeService, wallet *WalletService, tx *TxService) *NomService {
	return &NomService{node: node, wallet: wallet, tx: tx}
}

// GetPlasmaInfo returns the active address's plasma snapshot.
func (s *NomService) GetPlasmaInfo() (PlasmaInfo, error) {
	client := s.node.currentClient()
	if client == nil {
		return PlasmaInfo{}, errors.New("not connected")
	}
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return PlasmaInfo{}, errLocked
	}
	info, err := client.PlasmaApi.Get(addr)
	if err != nil {
		return PlasmaInfo{}, err
	}
	qsr := "0"
	if info.QsrAmount != nil {
		qsr = info.QsrAmount.String()
	}
	return PlasmaInfo{QsrFused: qsr, CurrentPlasma: info.CurrentPlasma, MaxPlasma: info.MaxPlasma}, nil
}

// GetFusionEntries returns the active address's fusion entries with derived revocability.
func (s *NomService) GetFusionEntries() ([]FusionEntry, error) {
	client := s.node.currentClient()
	if client == nil {
		return nil, errors.New("not connected")
	}
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return nil, errLocked
	}
	list, err := client.PlasmaApi.GetEntriesByAddress(addr, 0, 50)
	if err != nil {
		return nil, err
	}
	m, err := client.LedgerApi.GetFrontierMomentum()
	if err != nil {
		return nil, err
	}
	out := []FusionEntry{}
	for _, e := range list.List {
		out = append(out, fusionEntryDTO(e, m.Height))
	}
	return out, nil
}

// EstimatePlasma returns the plasma a QSR amount would yield (pure SDK helper).
// qsr is in whole QSR (not base units): GetPlasmaByQsr expects whole QSR.
func (s *NomService) EstimatePlasma(qsr string) (uint64, error) {
	client := s.node.currentClient()
	if client == nil {
		return 0, errors.New("not connected")
	}
	amt, ok := new(big.Int).SetString(qsr, 10)
	if !ok || amt.Sign() < 0 {
		return 0, errors.New("invalid qsr amount")
	}
	return client.PlasmaApi.GetPlasmaByQsr(amt).Uint64(), nil
}

// PrepareFuse builds a Fuse template for the beneficiary and hands it to TxService
// for confirm-what-you-sign. Inputs are validated before any node use.
func (s *NomService) PrepareFuse(beneficiary, qsrAmount string) (CallPreview, error) {
	addr, err := types.ParseAddress(beneficiary)
	if err != nil {
		return CallPreview{}, fmt.Errorf("invalid beneficiary: %w", err)
	}
	amt, ok := new(big.Int).SetString(qsrAmount, 10)
	if !ok || amt.Sign() <= 0 {
		return CallPreview{}, errors.New("invalid QSR amount")
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.PlasmaApi.Fuse(addr, amt)
	// The callExpect zts MUST match the SDK template's TokenStandard or
	// TxService.ConfirmPublish's assertMatches rejects the block. The SDK's
	// PlasmaApi.Fuse builds the block with TokenStandard: types.QsrTokenStandard.
	return s.tx.prepareCall(template, callExpect{to: types.PlasmaContract, zts: types.QsrTokenStandard, amount: amt, data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Fuse %s QSR for %s", formatBaseAmount(qsrAmount, 8), beneficiary))
}

// PrepareCancelFuse builds a Cancel template for a fusion id (no funds move; the
// fused QSR returns to the sender on confirmation).
func (s *NomService) PrepareCancelFuse(id string) (CallPreview, error) {
	hash, err := types.HexToHash(id)
	if err != nil {
		return CallPreview{}, fmt.Errorf("invalid fusion id: %w", err)
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.PlasmaApi.Cancel(hash)
	// The callExpect zts MUST match the SDK template's TokenStandard or
	// TxService.ConfirmPublish's assertMatches rejects the block. The SDK's
	// PlasmaApi.Cancel builds the block with TokenStandard: types.ZnnTokenStandard
	// (Amount common.Big0) — NOT QSR, unlike Fuse. amount big.NewInt(0)
	// Cmp-equals common.Big0.
	return s.tx.prepareCall(template, callExpect{to: types.PlasmaContract, zts: types.ZnnTokenStandard, amount: big.NewInt(0), data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Cancel fusion %s", id))
}

const stakeTimeUnitSec int64 = 2_592_000 // 30 days; go-zenon StakeTimeUnitSec

// frontierUnix returns the unix-seconds timestamp of an RPC frontier momentum.
// The momentum's *time.Time Timestamp is json:"-" (nil over RPC); the wire value
// is the TimestampUnix uint64 field (json:"timestamp").
func frontierUnix(m *api.Momentum) int64 {
	return int64(m.TimestampUnix)
}

// stakeEntryDTO maps an SDK StakeEntry, deriving duration (months) and maturity
// from chain time (nowUnix = frontier momentum timestamp).
func stakeEntryDTO(e *embedded.StakeEntry, nowUnix int64) StakeEntry {
	amt := "0"
	if e.Amount != nil {
		amt = e.Amount.String()
	}
	months := 0
	if e.ExpirationTimestamp > e.StartTimestamp {
		months = int((e.ExpirationTimestamp - e.StartTimestamp) / stakeTimeUnitSec)
	}
	return StakeEntry{
		Id:                  e.Id.String(),
		Amount:              amt,
		StartTimestamp:      e.StartTimestamp,
		ExpirationTimestamp: e.ExpirationTimestamp,
		DurationMonths:      months,
		IsMatured:           nowUnix >= e.ExpirationTimestamp,
	}
}

// GetStakeList returns the active address's stakes with derived maturity.
func (s *NomService) GetStakeList() (StakeInfo, error) {
	client := s.node.currentClient()
	if client == nil {
		return StakeInfo{}, errors.New("not connected")
	}
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return StakeInfo{}, errLocked
	}
	list, err := client.StakeApi.GetEntriesByAddress(addr, 0, 50)
	if err != nil {
		return StakeInfo{}, err
	}
	m, err := client.LedgerApi.GetFrontierMomentum()
	if err != nil {
		return StakeInfo{}, err
	}
	now := frontierUnix(m)
	total := "0"
	if list.TotalAmount != nil {
		total = list.TotalAmount.String()
	}
	out := StakeInfo{TotalAmount: total, Entries: []StakeEntry{}}
	for _, e := range list.List {
		out.Entries = append(out.Entries, stakeEntryDTO(e, now))
	}
	return out, nil
}

// GetUncollectedReward returns the active address's uncollected stake reward.
func (s *NomService) GetUncollectedReward() (RewardInfo, error) {
	client := s.node.currentClient()
	if client == nil {
		return RewardInfo{}, errors.New("not connected")
	}
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return RewardInfo{}, errLocked
	}
	r, err := client.StakeApi.GetUncollectedReward(addr)
	if err != nil {
		return RewardInfo{}, err
	}
	znn, qsr := "0", "0"
	if r.ZnnAmount != nil {
		znn = r.ZnnAmount.String()
	}
	if r.QsrAmount != nil {
		qsr = r.QsrAmount.String()
	}
	return RewardInfo{Znn: znn, Qsr: qsr}, nil
}

// PrepareStake builds a Stake template (ZNN for durationMonths*30 days) and hands
// it to TxService. Inputs validated before any node use.
func (s *NomService) PrepareStake(amountZnn, durationMonths string) (CallPreview, error) {
	amt, ok := new(big.Int).SetString(amountZnn, 10)
	if !ok || amt.Cmp(big.NewInt(100_000_000)) < 0 { // StakeMinAmount = 1 ZNN
		return CallPreview{}, errors.New("stake amount must be at least 1 ZNN")
	}
	months, err := strconv.Atoi(durationMonths)
	if err != nil || months < 1 || months > 12 {
		return CallPreview{}, errors.New("stake duration must be 1 to 12 months")
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.StakeApi.Stake(int64(months)*stakeTimeUnitSec, amt)
	return s.tx.prepareCall(template,
		callExpect{to: types.StakeContract, zts: types.ZnnTokenStandard, amount: amt, data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Stake %s ZNN for %d months", formatBaseAmount(amountZnn, 8), months))
}

// PrepareCancelStake builds a Cancel template for a matured stake id.
func (s *NomService) PrepareCancelStake(id string) (CallPreview, error) {
	hash, err := types.HexToHash(id)
	if err != nil {
		return CallPreview{}, fmt.Errorf("invalid stake id: %w", err)
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.StakeApi.Cancel(hash)
	return s.tx.prepareCall(template,
		callExpect{to: types.StakeContract, zts: types.ZnnTokenStandard, amount: big.NewInt(0), data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Cancel stake %s", id))
}

// PrepareCollectReward builds a CollectReward template (claims accrued ZNN/QSR).
func (s *NomService) PrepareCollectReward() (CallPreview, error) {
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.StakeApi.CollectReward()
	return s.tx.prepareCall(template,
		callExpect{to: types.StakeContract, zts: types.ZnnTokenStandard, amount: big.NewInt(0), data: append([]byte(nil), template.Data...)},
		"Collect staking rewards")
}

// fusionEntryDTO maps an SDK FusionEntry, deriving revocability from the frontier height.
func fusionEntryDTO(e *embedded.FusionEntry, currentHeight uint64) FusionEntry {
	qsr := "0"
	if e.QsrAmount != nil {
		qsr = e.QsrAmount.String()
	}
	return FusionEntry{
		Id:               e.Id.String(),
		Beneficiary:      e.Beneficiary.String(),
		QsrAmount:        qsr,
		ExpirationHeight: e.ExpirationHeight,
		IsRevocable:      currentHeight >= e.ExpirationHeight,
	}
}
