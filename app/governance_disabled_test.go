package app

import "testing"

// enableGovernance flips the temporary governance kill switch on for one test
// and restores the disabled default afterwards, so the intact governance code
// keeps its coverage while the feature is off. Tests in this package run
// sequentially (no t.Parallel here), so mutating the package var is safe.
func enableGovernance(t *testing.T) {
	t.Helper()
	governanceFeatureEnabled = true
	t.Cleanup(func() { governanceFeatureEnabled = false })
}

// With the kill switch at its shipped default (false), every governance-bound
// method must return errGovernanceDisabled before any validation or node use —
// reads included — so the feature is unreachable even from devtools.
func TestGovernanceDisabled_AllBoundMethodsBlocked(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	valid := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"

	if _, err := s.GetActions(0, 20); err != errGovernanceDisabled {
		t.Fatalf("GetActions: want errGovernanceDisabled, got %v", err)
	}
	if _, err := s.GetAction(valid); err != errGovernanceDisabled {
		t.Fatalf("GetAction: want errGovernanceDisabled, got %v", err)
	}
	if _, err := s.GetProposeKinds(); err != errGovernanceDisabled {
		t.Fatalf("GetProposeKinds: want errGovernanceDisabled, got %v", err)
	}
	if _, err := s.PrepareGovernanceVote(valid, "P1", 0); err != errGovernanceDisabled {
		t.Fatalf("PrepareGovernanceVote: want errGovernanceDisabled, got %v", err)
	}
	if _, err := s.PrepareExecuteAction(valid); err != errGovernanceDisabled {
		t.Fatalf("PrepareExecuteAction: want errGovernanceDisabled, got %v", err)
	}
	if _, err := s.PrepareProposeAction("Act", "d", "https://zenon.org", "spork.create", map[string]string{"name": "S", "description": "d"}); err != errGovernanceDisabled {
		t.Fatalf("PrepareProposeAction: want errGovernanceDisabled, got %v", err)
	}
}
