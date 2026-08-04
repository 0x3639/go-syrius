package app

import (
	"testing"
	"time"

	"github.com/zenon-network/go-zenon/protocol"
)

func TestComputeSyncPercentAndEta(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	samples := []heightSample{
		{T: base, Height: 100},
		{T: base.Add(10 * time.Second), Height: 200}, // 10 blocks/sec
	}
	s := computeSync(samples, 200, 1200, 5, "syncing")
	if s.TargetHeight != 1200 || s.CurrentHeight != 200 {
		t.Fatalf("heights: %+v", s)
	}
	// percent = 200/1200*100 ≈ 16.67
	if s.Percent < 16.6 || s.Percent > 16.7 {
		t.Fatalf("percent = %v", s.Percent)
	}
	// eta = (1200-200)/10 = 100s
	if s.EtaSeconds != 100 {
		t.Fatalf("eta = %d", s.EtaSeconds)
	}
	if s.Peers != 5 || s.State != "syncing" {
		t.Fatalf("misc: %+v", s)
	}
}

func TestComputeSyncNoTargetNoEta(t *testing.T) {
	s := computeSync(nil, 50, 0, 0, "starting")
	if s.Percent != 0 || s.EtaSeconds != 0 {
		t.Fatalf("target==0 must yield no percent/eta: %+v", s)
	}
}

func TestComputeSyncDoneNoEta(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	samples := []heightSample{{T: base, Height: 1000}, {T: base.Add(time.Second), Height: 1200}}
	s := computeSync(samples, 1200, 1200, 8, "synced")
	if s.EtaSeconds != 0 {
		t.Fatalf("current>=target must yield no eta: %+v", s)
	}
	if s.Percent < 99.99 {
		t.Fatalf("percent should be ~100: %v", s.Percent)
	}
}

func TestMapSyncState(t *testing.T) {
	if mapSyncState(protocol.Syncing) != "syncing" {
		t.Fatal("Syncing")
	}
	if mapSyncState(protocol.SyncDone) != "synced" {
		t.Fatal("SyncDone")
	}
	if mapSyncState(protocol.NotEnoughPeers) != "starting" {
		t.Fatal("NotEnoughPeers")
	}
	if mapSyncState(protocol.Unknown) != "starting" {
		t.Fatal("Unknown")
	}
}

func TestStallTrackerFlagsFrozenHeight(t *testing.T) {
	var st stallTracker
	t0 := time.Now()
	if st.observe(t0, 100, "syncing") {
		t.Fatal("first sample must establish baseline, not stall")
	}
	if st.observe(t0.Add(time.Minute), 100, "syncing") {
		t.Fatal("stalled before syncStallAfter elapsed")
	}
	if !st.observe(t0.Add(syncStallAfter), 100, "syncing") {
		t.Fatal("frozen height past syncStallAfter must report stalled")
	}
	// Any advance resets the clock.
	if st.observe(t0.Add(syncStallAfter+time.Second), 101, "syncing") {
		t.Fatal("advancing height must clear the stall")
	}
	if st.observe(t0.Add(syncStallAfter+2*time.Second), 101, "syncing") {
		t.Fatal("stall must not re-trigger immediately after an advance")
	}
}

func TestStallTrackerNeverFlagsSynced(t *testing.T) {
	var st stallTracker
	t0 := time.Now()
	st.observe(t0, 100, "synced")
	if st.observe(t0.Add(2*syncStallAfter), 100, "synced") {
		t.Fatal("a synced node with a quiet chain is not stalled")
	}
}

func TestStallTrackerRebaselinesOnHeightRegression(t *testing.T) {
	var st stallTracker
	t0 := time.Now()
	st.observe(t0, 100, "syncing")
	// An embedded DB rollback / reorg walks the height backwards. That is a new
	// baseline, not a stall.
	if st.observe(t0.Add(time.Minute), 90, "syncing") {
		t.Fatal("a height regression must re-baseline, not report stalled")
	}
	// The window restarts from the regression sample: syncStallAfter past t0 is
	// only syncStallAfter-1m past the regression.
	if st.observe(t0.Add(syncStallAfter), 90, "syncing") {
		t.Fatal("the stall window must restart from the regression sample")
	}
	if !st.observe(t0.Add(time.Minute+syncStallAfter), 90, "syncing") {
		t.Fatal("frozen at the regressed height past syncStallAfter must report stalled")
	}
}

func TestStallTrackerErrorsConvergeToStalled(t *testing.T) {
	var st stallTracker
	t0 := time.Now()
	if st.observeError(t0) {
		t.Fatal("errors before any baseline must not report stalled")
	}
	st.observe(t0, 100, "syncing")
	// The first error only starts the error-run clock.
	if st.observeError(t0.Add(time.Minute)) {
		t.Fatal("the first error must start the clock, not report stalled")
	}
	if st.observeError(t0.Add(time.Minute + syncStallAfter - time.Second)) {
		t.Fatal("stalled before the error run reached syncStallAfter")
	}
	if !st.observeError(t0.Add(time.Minute + syncStallAfter)) {
		t.Fatal("persistent errors past syncStallAfter must report stalled")
	}
	// A sample that gets through ends the run and clears the clock.
	st.observe(t0.Add(time.Minute+syncStallAfter+time.Second), 101, "syncing")
	if st.observeError(t0.Add(time.Minute + syncStallAfter + 2*time.Second)) {
		t.Fatal("a successful sample must reset the error clock")
	}
}

func TestStallTrackerErrorsOnQuietSyncedChain(t *testing.T) {
	var st stallTracker
	t0 := time.Now()
	st.observe(t0, 100, "synced")
	st.observe(t0.Add(2*syncStallAfter), 100, "synced")
	// Quiet time is not error time: one failed sample after a long synced idle
	// stretch says nothing yet.
	if st.observeError(t0.Add(2*syncStallAfter + time.Second)) {
		t.Fatal("one error after a quiet synced stretch must not report stalled")
	}
	// A sustained run still converges to stalled (spec §6).
	if !st.observeError(t0.Add(3*syncStallAfter + 2*time.Second)) {
		t.Fatal("a sustained error run must report stalled even from synced")
	}
}
