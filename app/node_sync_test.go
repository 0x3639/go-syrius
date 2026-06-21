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
