//go:build integration

package app

import (
	"testing"
)

// TestNomFuseCancelIntegration fuses a small amount of QSR on testnet and (if the
// resulting entry is immediately revocable) cancels it. Heavy + needs a funded
// testnet keystore in secrets/; opt-in.
func TestNomFuseCancelIntegration(t *testing.T) {
	t.Skip("manual: requires a funded testnet keystore and a configured node; wire via the spike harness when running Gate-5a")
}
