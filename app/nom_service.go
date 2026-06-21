package app

import (
	"errors"
	"math/big"

	embedded "github.com/0x3639/znn-sdk-go/api/embedded"
)

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
