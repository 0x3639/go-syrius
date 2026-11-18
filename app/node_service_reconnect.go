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

// retryEmbeddedConnection is one bounded attempt. The operation lock prevents
// a mode change from interleaving with it; the generation fence also covers
// shutdown, which stops the embedded node without taking opMu.
func (n *NodeService) retryEmbeddedConnection(gen uint64) (uint64, bool) {
	n.opMu.Lock()
	defer n.opMu.Unlock()
	n.mu.Lock()
	current := n.connGen == gen && n.mode == "embedded" && n.embedded != nil &&
		n.client == nil && !n.embeddedStopping && !n.embeddedWedged
	h := n.embedded
	if !current {
		n.mu.Unlock()
		return gen, false
	}
	// Reserve the candidate in the same critical section as the eligibility
	// check. A shutdown between this point and the dial invalidates it instead
	// of allowing setNode to create a newer intent after the node has stopped.
	n.disconnectLocked()
	n.emitStatusLocked(false)
	gen = n.connGen
	n.mu.Unlock()

	err := n.dialNode(h.WSURL(), gen)
	n.mu.RLock()
	current = n.connGen == gen && n.mode == "embedded" && n.embedded != nil &&
		!n.embeddedStopping && !n.embeddedWedged
	n.mu.RUnlock()
	if !current {
		return gen, false
	}
	if err != nil {
		return gen, true
	}
	n.startSyncPoller()
	return gen, false
}
