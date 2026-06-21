package app

import (
	"time"

	"github.com/zenon-network/go-zenon/protocol"
)

// heightSample is one (time, height) observation for ETA rate calculation.
type heightSample struct {
	T      time.Time
	Height uint64
}

// mapSyncState maps go-zenon's SyncState to a UI string.
func mapSyncState(st protocol.SyncState) string {
	switch st {
	case protocol.Syncing:
		return "syncing"
	case protocol.SyncDone:
		return "synced"
	default: // Unknown, NotEnoughPeers
		return "starting"
	}
}

// computeSync derives percent + ETA from height samples and the node's reported
// current/target heights. With target==0 (peers not reporting yet) there is no
// percent or ETA; ETA is also omitted when the rate is non-positive or already
// at/above target.
func computeSync(samples []heightSample, current, target uint64, peers int, state string) SyncStatus {
	s := SyncStatus{State: state, CurrentHeight: current, TargetHeight: target, Peers: peers}
	if target == 0 {
		return s
	}
	s.Percent = float64(current) / float64(target) * 100
	if s.Percent > 100 {
		s.Percent = 100
	}
	if current >= target || len(samples) < 2 {
		return s
	}
	first, last := samples[0], samples[len(samples)-1]
	dt := last.T.Sub(first.T).Seconds()
	if dt <= 0 || last.Height <= first.Height {
		return s
	}
	rate := float64(last.Height-first.Height) / dt // blocks/sec
	if rate <= 0 {
		return s
	}
	s.EtaSeconds = int64(float64(target-current) / rate)
	return s
}
