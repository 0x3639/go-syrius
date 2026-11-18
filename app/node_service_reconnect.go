package app

import (
	"context"
	"time"
)

// reconnectEmbedded restores RPC reads, momentum events and sync reporting
// without touching the embedded node or requiring an unlocked wallet. A newer
// connection intent (including explicit Disconnect) supersedes this worker.
func (n *NodeService) reconnectEmbedded(gen uint64) {
	n.mu.RLock()
	ctx := n.ctx
	delay := n.embeddedReconnectDelay
	n.mu.RUnlock()
	if ctx == nil {
		ctx = context.Background()
	}
	if delay <= 0 {
		delay = time.Second
	}
	for {
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		var retry bool
		gen, retry = n.retryEmbeddedConnection(gen)
		if !retry {
			return
		}
		delay *= 2
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}
	}
}

// retryEmbeddedConnection reserves and commits under the operation lock, just
// like an explicit transition. Only private network preparation runs outside
// that lock: a newer intent can proceed immediately and supersede the candidate.
func (n *NodeService) retryEmbeddedConnection(gen uint64) (uint64, bool) {
	n.opMu.Lock()
	n.mu.Lock()
	current := n.connGen == gen && n.mode == "embedded" && n.embedded != nil &&
		n.client == nil && !n.embeddedStopping && !n.embeddedWedged
	h := n.embedded
	if !current {
		n.mu.Unlock()
		n.opMu.Unlock()
		return gen, false
	}
	// Reserve the candidate in the same critical section as the eligibility
	// check. A shutdown between this point and the dial invalidates it instead
	// of allowing setNode to create a newer intent after the node has stopped.
	n.disconnectLocked()
	n.emitStatusLocked(false)
	gen = n.connGen
	n.mu.Unlock()
	n.opMu.Unlock()

	url := h.WSURL()
	candidate, err := n.prepareNodeConnection(url)
	n.opMu.Lock()
	defer n.opMu.Unlock()
	n.mu.RLock()
	current = n.connGen == gen && n.mode == "embedded" && n.embedded != nil &&
		!n.embeddedStopping && !n.embeddedWedged
	n.mu.RUnlock()
	if !current {
		if candidate != nil {
			candidate.close()
		}
		return gen, false
	}
	if err != nil {
		return gen, true
	}
	if err := n.installPreparedConnection(candidate, url, gen); err != nil {
		return gen, false
	}
	n.startSyncPoller()
	return gen, false
}
