package app

// Event names emitted to the frontend.
const (
	EventNodeStatus   = "node:status"
	EventMomentumTick = "momentum:tick"
	EventWalletLocked = "wallet:locked"
	EventNodeSync     = "node:sync"
)

const defaultEmbeddedNodeURL = "ws://127.0.0.1:35998"

// EmbeddedInfo describes the embedded node's data on disk.
type EmbeddedInfo struct {
	Running   bool   `json:"running"`
	DataDir   string `json:"dataDir"`
	SizeBytes int64  `json:"sizeBytes"`
}

// SyncStatus is the embedded sync snapshot pushed via EventNodeSync.
type SyncStatus struct {
	State         string  `json:"state"`
	CurrentHeight uint64  `json:"currentHeight"`
	TargetHeight  uint64  `json:"targetHeight"`
	Percent       float64 `json:"percent"`
	EtaSeconds    int64   `json:"etaSeconds"`
	Peers         int     `json:"peers"`
}

// Settings is the persisted user configuration.
type Settings struct {
	// Deprecated: read-only for migration from the pre-4a single-URL format.
	NodeURL          string `json:"nodeUrl,omitempty"`
	NodeMode         string `json:"nodeMode"`
	RemoteNodeURL    string `json:"remoteNodeUrl"`
	LocalNodeURL     string `json:"localNodeUrl"`
	Theme            string `json:"theme"`
	LastWallet       string `json:"lastWallet"`
	ActiveAccount    int    `json:"activeAccount"`
	AllowMainnetSend bool   `json:"allowMainnetSend"`
	ChainID          uint64 `json:"chainId"`
	AutoReceive      bool   `json:"autoReceive"`
	// ShowGovernance reveals the (experimental, testnet-only) Governance tab in
	// the navigation. Off by default.
	ShowGovernance bool `json:"showGovernance"`
	// AccountLabels maps "<wallet>:<index>" to a human label for an account.
	AccountLabels map[string]string `json:"accountLabels"`
	// AccountCounts maps a wallet id to how many accounts (derivation indices)
	// the user has revealed. Unset/below the default falls back to accountRange.
	AccountCounts map[string]int `json:"accountCounts"`
	// Contacts is the address book (saved name → address entries).
	Contacts []Contact `json:"contacts"`
}

// Contact is a saved address-book entry.
type Contact struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

// ActiveNodeURL returns the URL for the current NodeMode.
func (s Settings) ActiveNodeURL() string {
	switch s.NodeMode {
	case "local":
		return s.LocalNodeURL
	case "embedded":
		return defaultEmbeddedNodeURL
	default:
		return s.RemoteNodeURL
	}
}

// NodeConfig is the node mode + per-mode URLs for the settings UI.
type NodeConfig struct {
	Mode      string `json:"mode"`
	RemoteURL string `json:"remoteUrl"`
	LocalURL  string `json:"localUrl"`
}

// WalletMeta identifies a keystore without exposing secrets.
type WalletMeta struct {
	ID          string `json:"id"`   // keystore filename (stable storage id)
	Name        string `json:"name"` // editable display name
	BaseAddress string `json:"baseAddress"`
}

// AccountInfo is one derived account.
type AccountInfo struct {
	Index   int    `json:"index"`
	Address string `json:"address"`
	Label   string `json:"label"`
}

// NodeStatus is the connection/sync snapshot pushed via EventNodeStatus.
type NodeStatus struct {
	Mode      string `json:"mode"`
	Connected bool   `json:"connected"`
	Syncing   bool   `json:"syncing"`
	Height    uint64 `json:"height"`
	Peers     int    `json:"peers"`
	ChainID   uint64 `json:"chainId"`
}

// TokenBalance is one token's balance for the active address.
type TokenBalance struct {
	Zts      string `json:"zts"`
	Symbol   string `json:"symbol"`
	Decimals int    `json:"decimals"`
	Amount   string `json:"amount"` // base-unit decimal string
}

// TxRecord is one account block in history.
// TxPage is one page of history rows plus whether a next page exists.
type TxPage struct {
	Records []TxRecord `json:"records"`
	HasMore bool       `json:"hasMore"`
}

type TxRecord struct {
	Hash           string `json:"hash"`
	Direction      string `json:"direction"` // "in" | "out" | "pair"
	Method         string `json:"method"`    // embedded-contract method (e.g. CollectReward), or ""
	Counterparty   string `json:"counterparty"`
	Token          string `json:"token"`
	Amount         string `json:"amount"`
	Decimals       int    `json:"decimals"`
	MomentumHeight uint64 `json:"momentumHeight"`
	Confirmed      bool   `json:"confirmed"`
	Timestamp      int64  `json:"timestamp"`
}

const defaultNodeURL = "wss://my.hc1node.com:35998"
const defaultLocalNodeURL = "ws://127.0.0.1:35998"

// Phase 2 transaction event names.
const (
	EventTxPowProgress = "tx:pow-progress"
	EventTxPublished   = "tx:published"
	EventTxReceived    = "tx:received"
	// EventAutoReceiving carries a bool: auto-receive is (true) / is no longer
	// (false) actively receiving — drives the "Receiving…" UI indicator.
	EventAutoReceiving = "auto-receive:active"
	// EventAutoReceiveError carries {hash, error}: a single auto-receive attempt
	// failed. Surfaced so a silently stalled auto-receive is visible to the user.
	EventAutoReceiveError = "auto-receive:error"
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
	Decimals   int    `json:"decimals"`
	UsedPlasma uint64 `json:"usedPlasma"`
	Difficulty uint64 `json:"difficulty"`
	Hash       string `json:"hash"`
	NeedsPoW   bool   `json:"needsPoW"`
	HoldID     uint64 `json:"holdId"` // identity of the backend hold; lets a cancel target exactly this block
}

// CallPreview is the confirm-what-you-sign preview for an embedded-contract call,
// rendered from the built, signed block plus a human action summary.
type CallPreview struct {
	ToAddress  string `json:"toAddress"`
	Zts        string `json:"zts"`
	Symbol     string `json:"symbol"`
	Amount     string `json:"amount"`
	Decimals   int    `json:"decimals"`
	Hash       string `json:"hash"`
	Summary    string `json:"summary"`
	UsedPlasma uint64 `json:"usedPlasma"`
	Difficulty uint64 `json:"difficulty"`
	NeedsPoW   bool   `json:"needsPoW"`
	HoldID     uint64 `json:"holdId"` // identity of the backend hold; lets a cancel target exactly this block
}

// PlasmaInfo is the active address's plasma snapshot.
type PlasmaInfo struct {
	QsrFused      string `json:"qsrFused"`
	CurrentPlasma uint64 `json:"currentPlasma"`
	MaxPlasma     uint64 `json:"maxPlasma"`
}

// FusionEntry is one QSR fusion. IsRevocable is derived (frontier >= expiration).
type FusionEntry struct {
	Id               string `json:"id"`
	Beneficiary      string `json:"beneficiary"`
	QsrAmount        string `json:"qsrAmount"`
	ExpirationHeight uint64 `json:"expirationHeight"`
	IsRevocable      bool   `json:"isRevocable"`
}

// StakeInfo is the active address's stake snapshot.
type StakeInfo struct {
	TotalAmount string       `json:"totalAmount"`
	Entries     []StakeEntry `json:"entries"`
}

// StakeEntry is one ZNN stake; IsMatured is derived (frontier time >= expiration).
type StakeEntry struct {
	Id                  string `json:"id"`
	Amount              string `json:"amount"`
	StartTimestamp      int64  `json:"startTimestamp"`
	ExpirationTimestamp int64  `json:"expirationTimestamp"`
	DurationMonths      int    `json:"durationMonths"`
	IsMatured           bool   `json:"isMatured"`
}

// RewardInfo is uncollected reward (base-unit decimal strings).
type RewardInfo struct {
	Znn string `json:"znn"`
	Qsr string `json:"qsr"`
}

// UnreceivedBlock is one inbound, not-yet-received transaction.
type UnreceivedBlock struct {
	FromHash    string `json:"fromHash"`
	FromAddress string `json:"fromAddress"`
	Token       string `json:"token"`
	Amount      string `json:"amount"`
	Decimals    int    `json:"decimals"`
}

// PillarSummary is one pillar in the delegation picker list.
type PillarSummary struct {
	Name                  string `json:"name"`
	Rank                  int    `json:"rank"`
	Weight                string `json:"weight"`
	DelegateRewardPercent int    `json:"delegateRewardPercent"`
	ProducerAddress       string `json:"producerAddress"`
}

// DelegationInfo is the active address's current pillar delegation.
// An empty Name means the address is not delegated.
type DelegationInfo struct {
	Name   string `json:"name"`
	Status int    `json:"status"`
	Weight string `json:"weight"`
}

// SentinelInfo is the active address's sentinel. An empty Owner means the
// address has no sentinel.
type SentinelInfo struct {
	Owner                 string `json:"owner"`
	RegistrationTimestamp int64  `json:"registrationTimestamp"`
	IsRevocable           bool   `json:"isRevocable"`
	RevokeCooldown        int64  `json:"revokeCooldown"`
	Active                bool   `json:"active"`
}

// OwnedPillarInfo describes the pillar owned by the active address. An empty
// Name means the address owns no pillar.
type OwnedPillarInfo struct {
	Name                  string `json:"name"`
	OwnerAddress          string `json:"ownerAddress"`
	ProducerAddress       string `json:"producerAddress"`
	RewardAddress         string `json:"rewardAddress"`
	GiveMomentumRewardPct int    `json:"giveMomentumRewardPct"`
	GiveDelegateRewardPct int    `json:"giveDelegateRewardPct"`
	IsRevocable           bool   `json:"isRevocable"`
	RevokeCooldown        int64  `json:"revokeCooldown"`
}

// TokenInfo is one ZTS token's metadata. An empty TokenStandard means not found.
type TokenInfo struct {
	Name          string `json:"name"`
	Symbol        string `json:"symbol"`
	Domain        string `json:"domain"`
	TokenStandard string `json:"tokenStandard"`
	Owner         string `json:"owner"`
	TotalSupply   string `json:"totalSupply"`
	MaxSupply     string `json:"maxSupply"`
	Decimals      int    `json:"decimals"`
	IsMintable    bool   `json:"isMintable"`
	IsBurnable    bool   `json:"isBurnable"`
	IsUtility     bool   `json:"isUtility"`
}

// VoteBreakdownDTO is the Yes/No/Total Pillar-vote tally for a project or phase.
type VoteBreakdownDTO struct {
	Total uint32 `json:"total"`
	Yes   uint32 `json:"yes"`
	No    uint32 `json:"no"`
}

// PhaseDTO is one Accelerator-Z phase with its vote tally.
type PhaseDTO struct {
	Id                string           `json:"id"`
	ProjectId         string           `json:"projectId"`
	Name              string           `json:"name"`
	Description       string           `json:"description"`
	Url               string           `json:"url"`
	ZnnFundsNeeded    string           `json:"znnFundsNeeded"`
	QsrFundsNeeded    string           `json:"qsrFundsNeeded"`
	CreationTimestamp int64            `json:"creationTimestamp"`
	AcceptedTimestamp int64            `json:"acceptedTimestamp"`
	Status            int              `json:"status"`
	Votes             VoteBreakdownDTO `json:"votes"`
}

// ProjectDTO is one Accelerator-Z project with its phases and vote tally.
type ProjectDTO struct {
	Id                  string           `json:"id"`
	Owner               string           `json:"owner"`
	Name                string           `json:"name"`
	Description         string           `json:"description"`
	Url                 string           `json:"url"`
	ZnnFundsNeeded      string           `json:"znnFundsNeeded"`
	QsrFundsNeeded      string           `json:"qsrFundsNeeded"`
	CreationTimestamp   int64            `json:"creationTimestamp"`
	LastUpdateTimestamp int64            `json:"lastUpdateTimestamp"`
	Status              int              `json:"status"`
	Votes               VoteBreakdownDTO `json:"votes"`
	Phases              []PhaseDTO       `json:"phases"`
}

// ProjectListDTO is one page of Accelerator-Z projects.
type ProjectListDTO struct {
	Count int          `json:"count"`
	List  []ProjectDTO `json:"list"`
}

// ActionDTO is one governance action with its current-round vote tally and the
// per-round thresholds the node computed for it. Reuses VoteBreakdownDTO.
type ActionDTO struct {
	Id                    string           `json:"id"`
	Owner                 string           `json:"owner"`
	Name                  string           `json:"name"`
	Description           string           `json:"description"`
	Url                   string           `json:"url"`
	Destination           string           `json:"destination"`
	Data                  string           `json:"data"` // base64 ABI call data
	Type                  int              `json:"type"` // 1 Spork, 2 Normal
	Round                 int              `json:"round"`
	// CurrentVoteId is the votable hash for the action's CURRENT round — the id a
	// vote must target (it equals Id only in round 0, then ratchets per round).
	CurrentVoteId         string           `json:"currentVoteId"`
	Status                int              `json:"status"` // 0 Voting,1 Approved,2 Rejected,3 NoDecision
	Executed              bool             `json:"executed"`
	Expired               bool             `json:"expired"`
	CreationTimestamp     int64            `json:"creationTimestamp"`
	RoundStartTimestamp   int64            `json:"roundStartTimestamp"`
	ActivePillarThreshold uint32           `json:"activePillarThreshold"`
	DirectionalThreshold  uint32           `json:"directionalThreshold"`
	VotingPeriod          int64            `json:"votingPeriod"`
	Votes                 VoteBreakdownDTO `json:"votes"`
}

// ActionListDTO is one page of governance actions.
type ActionListDTO struct {
	Count int         `json:"count"`
	List  []ActionDTO `json:"list"`
}

// ProposeFieldDTO is one input field the Propose form renders for an action kind.
type ProposeFieldDTO struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Type        string `json:"type"` // text|number|bool|address|hash|amount|base64|list
	Placeholder string `json:"placeholder"`
	Required    bool   `json:"required"`
	// Min/Max are byte-length bounds (0 = unbounded), matching the on-chain
	// length checks. Drive the form's inline hint + maxlength and the
	// server-side validateFieldLengths guard.
	Min int `json:"min"`
	Max int `json:"max"`
}

// ProposeKindDTO is one proposable governance action kind + its input schema.
type ProposeKindDTO struct {
	Kind   string            `json:"kind"`  // stable id, e.g. "spork.create"
	Label  string            `json:"label"`
	Group  string            `json:"group"` // Spork|Bridge|Liquidity|Custom
	Fields []ProposeFieldDTO `json:"fields"`
}

// PillarVoteState is one owned pillar's vote on a votable item; Vote == -1 means
// the pillar has not voted yet.
type PillarVoteState struct {
	Pillar string `json:"pillar"`
	Vote   int    `json:"vote"` // -1 not voted, 0 yes, 1 no, 2 abstain
}

// VotableItem is a project or phase currently open for pillar voting, annotated
// with the active address's owned-pillar vote state.
type VotableItem struct {
	Kind           string            `json:"kind"` // "project" | "phase"
	Id             string            `json:"id"`
	ProjectId      string            `json:"projectId"`
	ProjectName    string            `json:"projectName"`
	Name           string            `json:"name"`
	ZnnFundsNeeded string            `json:"znnFundsNeeded"`
	QsrFundsNeeded string            `json:"qsrFundsNeeded"`
	Votes          VoteBreakdownDTO  `json:"votes"`
	MyVotes        []PillarVoteState `json:"myVotes"`
	NeedsMyVote    bool              `json:"needsMyVote"`
}
