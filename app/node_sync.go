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

// syncStallAfter is how long an unsynced node's height may stand still before
// the poller reports "stalled" (spec §6 — the 2026-08-03 wedged-p2p incident
// showed "starting · 100.0%" forever while nothing moved).
const syncStallAfter = 3 * time.Minute

// stallTracker decides when sync progress should be reported as stalled.
// Pure state machine; the poller feeds it wall-clock samples. It runs two
// independent clocks: lastAdvance (height standing still) and firstErr (an
// unbroken run of failing samples).
type stallTracker struct {
	baseline    bool
	lastHeight  uint64
	lastAdvance time.Time
	firstErr    time.Time
}

// observe folds a successful sample. Returns true when the node is unsynced
// and its height has not moved for syncStallAfter. Any height CHANGE — not
// merely an advance — re-baselines: an embedded DB rollback or a reorg walks
// the height backwards, and treating that as "no progress" would pin a false
// stall on a node that is in fact working.
func (st *stallTracker) observe(now time.Time, height uint64, state string) bool {
	// A sample got through, so whatever error run was in flight is over.
	st.firstErr = time.Time{}
	if !st.baseline || height != st.lastHeight {
		st.baseline = true
		st.lastHeight = height
		st.lastAdvance = now
		return false
	}
	return state != "synced" && now.Sub(st.lastAdvance) >= syncStallAfter
}

// observeError reports whether persistent RPC errors should surface as
// stalled. The window is measured over the error RUN, not since the last
// height advance: a synced node on a quiet chain can legitimately sit for
// hours without advancing, and billing that quiet time against its first
// failed sample would flag it stalled instantly. Requires a baseline —
// before that the connection path, not the poller, owns the "not working"
// story.
func (st *stallTracker) observeError(now time.Time) bool {
	if !st.baseline {
		return false
	}
	if st.firstErr.IsZero() {
		st.firstErr = now
		return false
	}
	return now.Sub(st.firstErr) >= syncStallAfter
}
