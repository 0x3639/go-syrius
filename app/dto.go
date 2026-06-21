package app

// Event names emitted to the frontend.
const (
	EventNodeStatus   = "node:status"
	EventMomentumTick = "momentum:tick"
	EventWalletLocked = "wallet:locked"
)

// Settings is the persisted user configuration.
type Settings struct {
	NodeURL          string `json:"nodeUrl"`
	Theme            string `json:"theme"`
	LastWallet       string `json:"lastWallet"`
	ActiveAccount    int    `json:"activeAccount"`
	AllowMainnetSend bool   `json:"allowMainnetSend"`
	AutoReceive      bool   `json:"autoReceive"`
}

// WalletMeta identifies a keystore without exposing secrets.
type WalletMeta struct {
	Name        string `json:"name"`
	BaseAddress string `json:"baseAddress"`
}

// AccountInfo is one derived account.
type AccountInfo struct {
	Index   int    `json:"index"`
	Address string `json:"address"`
}

// NodeStatus is the connection/sync snapshot pushed via EventNodeStatus.
type NodeStatus struct {
	Mode      string `json:"mode"`
	Connected bool   `json:"connected"`
	Syncing   bool   `json:"syncing"`
	Height    uint64 `json:"height"`
	Peers     int    `json:"peers"`
}

// TokenBalance is one token's balance for the active address.
type TokenBalance struct {
	Zts      string `json:"zts"`
	Symbol   string `json:"symbol"`
	Decimals int    `json:"decimals"`
	Amount   string `json:"amount"` // base-unit decimal string
}

// TxRecord is one account block in history.
type TxRecord struct {
	Hash           string `json:"hash"`
	Direction      string `json:"direction"` // "send" | "receive"
	Counterparty   string `json:"counterparty"`
	Token          string `json:"token"`
	Amount         string `json:"amount"`
	MomentumHeight uint64 `json:"momentumHeight"`
	Confirmed      bool   `json:"confirmed"`
	Timestamp      int64  `json:"timestamp"`
}

const defaultNodeURL = "wss://my.hc1node.com:35998"

// Phase 2 transaction event names.
const (
	EventTxPowProgress = "tx:pow-progress"
	EventTxPublished   = "tx:published"
	EventTxReceived    = "tx:received"
)

// mainnetChainID is the Network of Momentum mainnet chain identifier.
const mainnetChainID uint64 = 1

// SendRequest is the frontend's send intent.
type SendRequest struct {
	ToAddress string `json:"toAddress"`
	Zts       string `json:"zts"`
	Amount    string `json:"amount"` // base-unit decimal string
}

// SendPreview is rendered from the built, signed block before broadcast.
type SendPreview struct {
	ToAddress  string `json:"toAddress"`
	Symbol     string `json:"symbol"`
	Zts        string `json:"zts"`
	Amount     string `json:"amount"`
	UsedPlasma uint64 `json:"usedPlasma"`
	Difficulty uint64 `json:"difficulty"`
	Hash       string `json:"hash"`
	NeedsPoW   bool   `json:"needsPoW"`
}

// UnreceivedBlock is one inbound, not-yet-received transaction.
type UnreceivedBlock struct {
	FromHash    string `json:"fromHash"`
	FromAddress string `json:"fromAddress"`
	Token       string `json:"token"`
	Amount      string `json:"amount"`
}
