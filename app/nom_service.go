package app

import (
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"

	embedded "github.com/0x3639/znn-sdk-go/api/embedded"
	"github.com/zenon-network/go-zenon/common/types"
	api "github.com/zenon-network/go-zenon/rpc/api"
	constants "github.com/zenon-network/go-zenon/vm/constants"
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
	return int64(m.TimestampUnix) // #nosec G115 -- unix-seconds timestamp; no realistic int64 overflow
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

// pillarSummaryDTO maps an SDK PillarInfo to the delegation-picker summary.
func pillarSummaryDTO(p *embedded.PillarInfo) PillarSummary {
	weight := "0"
	if p.Weight != nil {
		weight = p.Weight.String()
	}
	return PillarSummary{
		Name:                  p.Name,
		Rank:                  int(p.Rank),
		Weight:                weight,
		DelegateRewardPercent: int(p.GiveDelegateRewardPercentage),
		ProducerAddress:       p.ProducerAddress.String(),
	}
}

// sortPillarsByRank orders pillars by ascending rank (in place).
func sortPillarsByRank(ps []PillarSummary) {
	sort.Slice(ps, func(i, j int) bool { return ps[i].Rank < ps[j].Rank })
}

// GetPillarList returns all pillars (rank-sorted) for the delegation picker.
func (s *NomService) GetPillarList() ([]PillarSummary, error) {
	client := s.node.currentClient()
	if client == nil {
		return nil, errors.New("not connected")
	}
	out := []PillarSummary{}
	var pageIndex uint32 = 0
	const pageSize uint32 = 100
	for {
		list, err := client.PillarApi.GetAll(pageIndex, pageSize)
		if err != nil {
			return nil, err
		}
		for _, p := range list.List {
			out = append(out, pillarSummaryDTO(p))
		}
		if len(out) >= list.Count || len(list.List) == 0 {
			break
		}
		pageIndex++
	}
	sortPillarsByRank(out)
	return out, nil
}

// GetDelegation returns the active address's current pillar delegation.
// An empty Name means not delegated.
func (s *NomService) GetDelegation() (DelegationInfo, error) {
	client := s.node.currentClient()
	if client == nil {
		return DelegationInfo{}, errors.New("not connected")
	}
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return DelegationInfo{}, errLocked
	}
	d, err := client.PillarApi.GetDelegatedPillar(addr)
	if err != nil {
		return DelegationInfo{}, err
	}
	if d == nil {
		return DelegationInfo{}, nil
	}
	weight := "0"
	if d.Weight != nil {
		weight = d.Weight.String()
	}
	return DelegationInfo{Name: d.Name, Status: int(d.Status), Weight: weight}, nil
}

// PrepareDelegate builds a Delegate template (delegates the account's ZNN weight
// to the named pillar; no funds move) and hands it to TxService. Name validated
// before any node use.
func (s *NomService) PrepareDelegate(name string) (CallPreview, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return CallPreview{}, errors.New("pillar name is required")
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.PillarApi.Delegate(name)
	return s.tx.prepareCall(template,
		callExpect{to: types.PillarContract, zts: types.ZnnTokenStandard, amount: big.NewInt(0), data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Delegate to %s", name))
}

// PrepareUndelegate builds an Undelegate template (removes the current delegation).
func (s *NomService) PrepareUndelegate() (CallPreview, error) {
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.PillarApi.Undelegate()
	return s.tx.prepareCall(template,
		callExpect{to: types.PillarContract, zts: types.ZnnTokenStandard, amount: big.NewInt(0), data: append([]byte(nil), template.Data...)},
		"Undelegate from current pillar")
}

// PrepareCollectPillarReward builds a CollectReward template (claims accrued
// delegation rewards).
func (s *NomService) PrepareCollectPillarReward() (CallPreview, error) {
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.PillarApi.CollectReward()
	return s.tx.prepareCall(template,
		callExpect{to: types.PillarContract, zts: types.ZnnTokenStandard, amount: big.NewInt(0), data: append([]byte(nil), template.Data...)},
		"Collect delegation rewards")
}

// GetPillarReward returns the active address's uncollected delegation reward.
func (s *NomService) GetPillarReward() (RewardInfo, error) {
	client := s.node.currentClient()
	if client == nil {
		return RewardInfo{}, errors.New("not connected")
	}
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return RewardInfo{}, errLocked
	}
	r, err := client.PillarApi.GetUncollectedReward(addr)
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

// pillarNameRe matches go-zenon's pillar name rule: alphanumerics with single
// '-', '.', or '_' allowed only between alphanumerics.
var pillarNameRe = regexp.MustCompile(`^([a-zA-Z0-9]+[-._]?)*[a-zA-Z0-9]$`)

// validatePillarName mirrors go-zenon's checkPillarNameStatic (1–40 chars + regex).
// The node re-validates authoritatively; this is the first gate.
func validatePillarName(name string) error {
	if len(name) == 0 || len(name) > 40 {
		return errors.New("pillar name must be 1–40 characters")
	}
	if !pillarNameRe.MatchString(name) {
		return errors.New("pillar name may use only letters, digits, and single - . _ between them")
	}
	return nil
}

// ownedPillarDTO maps the first pillar owned by the address to the DTO. An empty
// slice (or nil first element) maps to an empty Name (= owns no pillar).
func ownedPillarDTO(list []*embedded.PillarInfo) OwnedPillarInfo {
	if len(list) == 0 || list[0] == nil {
		return OwnedPillarInfo{}
	}
	p := list[0]
	return OwnedPillarInfo{
		Name:                  p.Name,
		OwnerAddress:          p.OwnerAddress.String(),
		ProducerAddress:       p.ProducerAddress.String(),
		RewardAddress:         p.WithdrawAddress.String(),
		GiveMomentumRewardPct: int(p.GiveMomentumRewardPercentage),
		GiveDelegateRewardPct: int(p.GiveDelegateRewardPercentage),
		IsRevocable:           p.IsRevocable,
		RevokeCooldown:        p.RevokeCooldown,
	}
}

// GetMyPillar returns the pillar owned by the active address (empty Name = none).
func (s *NomService) GetMyPillar() (OwnedPillarInfo, error) {
	client := s.node.currentClient()
	if client == nil {
		return OwnedPillarInfo{}, errors.New("not connected")
	}
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return OwnedPillarInfo{}, errLocked
	}
	list, err := client.PillarApi.GetByOwner(addr)
	if err != nil {
		return OwnedPillarInfo{}, err
	}
	return ownedPillarDTO(list), nil
}

// GetPillarDepositedQsr returns the active address's QSR escrowed toward pillar
// registration (base-unit decimal string; "0" if none).
func (s *NomService) GetPillarDepositedQsr() (string, error) {
	client := s.node.currentClient()
	if client == nil {
		return "", errors.New("not connected")
	}
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return "", errLocked
	}
	q, err := client.PillarApi.GetDepositedQsr(addr)
	if err != nil {
		return "", err
	}
	if q == nil {
		return "0", nil
	}
	return q.String(), nil
}

// GetPillarQsrCost returns the current QSR cost to register the next pillar
// (base-unit decimal string). This QSR is burned on registration.
func (s *NomService) GetPillarQsrCost() (string, error) {
	client := s.node.currentClient()
	if client == nil {
		return "", errors.New("not connected")
	}
	cost, err := client.PillarApi.GetQsrRegistrationCost()
	if err != nil {
		return "", err
	}
	if cost == nil {
		return "0", nil
	}
	return cost.String(), nil
}

// CheckPillarName validates the name locally then asks the node whether it is
// available (true = free to register).
func (s *NomService) CheckPillarName(name string) (bool, error) {
	name = strings.TrimSpace(name)
	if err := validatePillarName(name); err != nil {
		return false, err
	}
	client := s.node.currentClient()
	if client == nil {
		return false, errors.New("not connected")
	}
	avail, err := client.PillarApi.CheckNameAvailability(name)
	if err != nil {
		return false, err
	}
	if avail == nil {
		return false, nil
	}
	return *avail, nil
}

// sentinelDTO maps an SDK SentinelInfo to the DTO. A nil result or a zero
// RegistrationTimestamp means the address has no sentinel (empty Owner).
func sentinelDTO(s *embedded.SentinelInfo) SentinelInfo {
	if s == nil || s.RegistrationTimestamp == 0 {
		return SentinelInfo{}
	}
	return SentinelInfo{
		Owner:                 s.Owner.String(),
		RegistrationTimestamp: s.RegistrationTimestamp,
		IsRevocable:           s.IsRevocable,
		RevokeCooldown:        s.RevokeCooldown,
		Active:                s.Active,
	}
}

// GetSentinel returns the active address's sentinel (empty Owner = none).
func (s *NomService) GetSentinel() (SentinelInfo, error) {
	client := s.node.currentClient()
	if client == nil {
		return SentinelInfo{}, errors.New("not connected")
	}
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return SentinelInfo{}, errLocked
	}
	info, err := client.SentinelApi.GetByOwner(addr)
	if err != nil {
		return SentinelInfo{}, err
	}
	return sentinelDTO(info), nil
}

// GetDepositedQsr returns the active address's QSR escrowed toward registration
// (base-unit decimal string; "0" if none).
func (s *NomService) GetDepositedQsr() (string, error) {
	client := s.node.currentClient()
	if client == nil {
		return "", errors.New("not connected")
	}
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return "", errLocked
	}
	q, err := client.SentinelApi.GetDepositedQsr(addr)
	if err != nil {
		return "", err
	}
	if q == nil {
		return "0", nil
	}
	return q.String(), nil
}

// PrepareDepositQsr builds a DepositQsr template (escrows QSR toward sentinel
// registration). qsr is a base-unit decimal string, validated before any node use.
func (s *NomService) PrepareDepositQsr(qsr string) (CallPreview, error) {
	amt, ok := new(big.Int).SetString(strings.TrimSpace(qsr), 10)
	if !ok || amt.Sign() <= 0 {
		return CallPreview{}, errors.New("deposit amount must be a positive QSR value")
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.SentinelApi.DepositQsr(amt)
	return s.tx.prepareCall(template,
		callExpect{to: types.SentinelContract, zts: types.QsrTokenStandard, amount: amt, data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Deposit %s QSR for sentinel", formatBaseAmount(amt.String(), 8)))
}

// PrepareRegisterSentinel builds a Register template (sends the 5,000 ZNN
// collateral; requires 50,000 QSR already deposited). Amount is read from the
// SDK template, never hardcoded.
func (s *NomService) PrepareRegisterSentinel() (CallPreview, error) {
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.SentinelApi.Register()
	return s.tx.prepareCall(template,
		callExpect{to: types.SentinelContract, zts: types.ZnnTokenStandard, amount: template.Amount, data: append([]byte(nil), template.Data...)},
		"Register sentinel (5,000 ZNN)")
}

// PrepareCollectSentinelReward builds a CollectReward template.
func (s *NomService) PrepareCollectSentinelReward() (CallPreview, error) {
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.SentinelApi.CollectReward()
	return s.tx.prepareCall(template,
		callExpect{to: types.SentinelContract, zts: types.ZnnTokenStandard, amount: big.NewInt(0), data: append([]byte(nil), template.Data...)},
		"Collect sentinel rewards")
}

// PrepareRevokeSentinel builds a Revoke template (returns the collateral after
// the cooldown).
func (s *NomService) PrepareRevokeSentinel() (CallPreview, error) {
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.SentinelApi.Revoke()
	return s.tx.prepareCall(template,
		callExpect{to: types.SentinelContract, zts: types.ZnnTokenStandard, amount: big.NewInt(0), data: append([]byte(nil), template.Data...)},
		"Revoke sentinel")
}

// PrepareWithdrawQsr builds a WithdrawQsr template (recovers escrowed QSR not
// consumed by registration).
func (s *NomService) PrepareWithdrawQsr() (CallPreview, error) {
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.SentinelApi.WithdrawQsr()
	return s.tx.prepareCall(template,
		callExpect{to: types.SentinelContract, zts: types.ZnnTokenStandard, amount: big.NewInt(0), data: append([]byte(nil), template.Data...)},
		"Withdraw deposited QSR")
}

// GetSentinelReward returns the active address's uncollected sentinel reward.
func (s *NomService) GetSentinelReward() (RewardInfo, error) {
	client := s.node.currentClient()
	if client == nil {
		return RewardInfo{}, errors.New("not connected")
	}
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return RewardInfo{}, errLocked
	}
	r, err := client.SentinelApi.GetUncollectedReward(addr)
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

// tokenInfoDTO maps an SDK Token to the DTO. nil supplies map to "0".
func tokenInfoDTO(t *embedded.Token) TokenInfo {
	// "Not found": SDK GetByZts preallocates a *Token, so the node leaves it
	// zero-valued (zero TokenStandard) for a missing ZTS. The zero standard would
	// otherwise bech32-encode to a non-empty string and read as a real token, so
	// map it to an empty DTO (empty TokenStandard signals not-found to the frontend).
	if t == nil || t.TokenStandard == types.ZeroTokenStandard {
		return TokenInfo{}
	}
	total, max := "0", "0"
	if t.TotalSupply != nil {
		total = t.TotalSupply.String()
	}
	if t.MaxSupply != nil {
		max = t.MaxSupply.String()
	}
	return TokenInfo{
		Name:          t.Name,
		Symbol:        t.Symbol,
		Domain:        t.Domain,
		TokenStandard: t.TokenStandard.String(),
		Owner:         t.Owner.String(),
		TotalSupply:   total,
		MaxSupply:     max,
		Decimals:      int(t.Decimals),
		IsMintable:    t.IsMintable,
		IsBurnable:    t.IsBurnable,
		IsUtility:     t.IsUtility,
	}
}

// GetMyTokens returns the tokens owned by the active address.
func (s *NomService) GetMyTokens() ([]TokenInfo, error) {
	client := s.node.currentClient()
	if client == nil {
		return nil, errors.New("not connected")
	}
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return nil, errLocked
	}
	out := []TokenInfo{}
	var pageIndex uint32 = 0
	const pageSize uint32 = 50
	for {
		list, err := client.TokenApi.GetByOwner(addr, pageIndex, pageSize)
		if err != nil {
			return nil, err
		}
		for _, t := range list.List {
			out = append(out, tokenInfoDTO(t))
		}
		if len(out) >= list.Count || len(list.List) == 0 {
			break
		}
		pageIndex++
	}
	return out, nil
}

// GetTokenByZts returns one token's metadata. zts is validated before any node
// use; an empty TokenStandard in the result means not found.
func (s *NomService) GetTokenByZts(zts string) (TokenInfo, error) {
	parsed, err := types.ParseZTS(strings.TrimSpace(zts))
	if err != nil {
		return TokenInfo{}, fmt.Errorf("invalid ZTS: %w", err)
	}
	client := s.node.currentClient()
	if client == nil {
		return TokenInfo{}, errors.New("not connected")
	}
	tok, err := client.TokenApi.GetByZts(parsed)
	if err != nil {
		return TokenInfo{}, err
	}
	if tok == nil {
		return TokenInfo{}, nil
	}
	return tokenInfoDTO(tok), nil
}

var (
	tokenNameRe   = regexp.MustCompile(`^([a-zA-Z0-9]+[-._]?)*[a-zA-Z0-9]$`)
	tokenSymbolRe = regexp.MustCompile(`^[A-Z0-9]+$`)
	tokenDomainRe = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9-]{0,61}[A-Za-z0-9]\.)+[A-Za-z]{2,}$`)
)

// PrepareIssueToken validates every field against the on-chain rules (before any
// node use), then builds an IssueToken template. The 1 ZNN fee is read from the
// template, never hardcoded.
func (s *NomService) PrepareIssueToken(name, symbol, domain, totalSupply, maxSupply string, decimals int, isMintable, isBurnable, isUtility bool) (CallPreview, error) {
	if l := len(name); l == 0 || l > 40 || !tokenNameRe.MatchString(name) {
		return CallPreview{}, errors.New("invalid token name (1-40 chars, letters/digits with single -._ separators)")
	}
	if l := len(symbol); l == 0 || l > 10 || !tokenSymbolRe.MatchString(symbol) {
		return CallPreview{}, errors.New("invalid token symbol (1-10 chars, A-Z and 0-9 only)")
	}
	if symbol == "ZNN" || symbol == "QSR" {
		return CallPreview{}, errors.New("token symbol ZNN/QSR is reserved")
	}
	if len(domain) != 0 && (len(domain) > 128 || !tokenDomainRe.MatchString(domain)) {
		return CallPreview{}, errors.New("invalid token domain")
	}
	if decimals < 0 || decimals > 18 {
		return CallPreview{}, errors.New("decimals must be 0 to 18")
	}
	total, ok := new(big.Int).SetString(totalSupply, 10)
	if !ok || total.Sign() < 0 {
		return CallPreview{}, errors.New("invalid total supply")
	}
	max, ok := new(big.Int).SetString(maxSupply, 10)
	if !ok || max.Sign() <= 0 {
		return CallPreview{}, errors.New("max supply must be greater than 0")
	}
	if max.Cmp(constants.TokenMaxSupplyBig) > 0 {
		return CallPreview{}, errors.New("max supply exceeds the maximum")
	}
	if max.Cmp(total) < 0 {
		return CallPreview{}, errors.New("max supply must be >= total supply")
	}
	if !isMintable && max.Cmp(total) != 0 {
		return CallPreview{}, errors.New("non-mintable token requires max supply == total supply")
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.TokenApi.IssueToken(name, symbol, domain, total, max, uint8(decimals), isMintable, isBurnable, isUtility)
	return s.tx.prepareCall(template,
		callExpect{to: types.TokenContract, zts: types.ZnnTokenStandard, amount: template.Amount, data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Issue token %s", symbol))
}

// PrepareMint builds a Mint template (owner-only on-chain). Inputs validated first.
func (s *NomService) PrepareMint(zts, amount, receiver string) (CallPreview, error) {
	parsedZts, err := types.ParseZTS(strings.TrimSpace(zts))
	if err != nil {
		return CallPreview{}, fmt.Errorf("invalid ZTS: %w", err)
	}
	amt, ok := new(big.Int).SetString(strings.TrimSpace(amount), 10)
	if !ok || amt.Sign() <= 0 {
		return CallPreview{}, errors.New("mint amount must be greater than 0")
	}
	recv, err := types.ParseAddress(strings.TrimSpace(receiver))
	if err != nil {
		return CallPreview{}, fmt.Errorf("invalid receiver: %w", err)
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.TokenApi.Mint(parsedZts, amt, recv)
	return s.tx.prepareCall(template,
		callExpect{to: types.TokenContract, zts: types.ZnnTokenStandard, amount: big.NewInt(0), data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Mint %s %s to %s", amt.String(), parsedZts.String(), recv.String()))
}

// PrepareBurn builds a Burn template. The burned token IS the block's token
// standard and the amount is carried by the block.
func (s *NomService) PrepareBurn(zts, amount string) (CallPreview, error) {
	parsedZts, err := types.ParseZTS(strings.TrimSpace(zts))
	if err != nil {
		return CallPreview{}, fmt.Errorf("invalid ZTS: %w", err)
	}
	amt, ok := new(big.Int).SetString(strings.TrimSpace(amount), 10)
	if !ok || amt.Sign() <= 0 {
		return CallPreview{}, errors.New("burn amount must be greater than 0")
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.TokenApi.Burn(parsedZts, amt)
	return s.tx.prepareCall(template,
		callExpect{to: types.TokenContract, zts: parsedZts, amount: amt, data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Burn %s %s", amt.String(), parsedZts.String()))
}

// PrepareUpdateToken builds an UpdateToken template (transfer owner / one-way
// disable mint/burn). Inputs validated first.
func (s *NomService) PrepareUpdateToken(zts, newOwner string, isMintable, isBurnable bool) (CallPreview, error) {
	parsedZts, err := types.ParseZTS(strings.TrimSpace(zts))
	if err != nil {
		return CallPreview{}, fmt.Errorf("invalid ZTS: %w", err)
	}
	owner, err := types.ParseAddress(strings.TrimSpace(newOwner))
	if err != nil {
		return CallPreview{}, fmt.Errorf("invalid owner: %w", err)
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.TokenApi.UpdateToken(parsedZts, owner, isMintable, isBurnable)
	return s.tx.prepareCall(template,
		callExpect{to: types.TokenContract, zts: types.ZnnTokenStandard, amount: big.NewInt(0), data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Update token %s", parsedZts.String()))
}
