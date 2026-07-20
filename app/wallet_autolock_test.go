package app

import (
	"testing"
	"time"
)

func isLocked(w *WalletService) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.keystore == nil
}

// The watchdog locks the wallet once inactivity exceeds the configured timeout.
// Tick is shortened via the autoLockTick test seam; "inactivity" is simulated
// by backdating lastActivity past the default 5-minute timeout.
func TestAutoLock_LocksAfterInactivity(t *testing.T) {
	w := newTestWalletService(t)
	unlockTestWallet(t, w)
	w.autoMu.Lock()
	w.autoLockTick = 10 * time.Millisecond
	w.autoMu.Unlock()
	w.startAutoLock()

	w.autoMu.Lock()
	w.lastActivity = time.Now().Add(-6 * time.Minute)
	w.autoMu.Unlock()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if isLocked(w) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("wallet did not auto-lock after inactivity")
}

// NoteActivity defers expiry; while locked it is a no-op.
func TestAutoLock_NoteActivityDefersExpiry(t *testing.T) {
	w := newTestWalletService(t)
	unlockTestWallet(t, w)
	w.startAutoLock()

	w.autoMu.Lock()
	w.lastActivity = time.Now().Add(-6 * time.Minute)
	w.autoMu.Unlock()
	if !w.autoLockExpired(time.Now()) {
		t.Fatal("backdated activity must read as expired")
	}
	w.NoteActivity()
	if w.autoLockExpired(time.Now()) {
		t.Fatal("NoteActivity must defer expiry")
	}

	w.stopAutoLock()
	if err := w.Lock(); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	w.autoMu.Lock()
	w.lastActivity = time.Time{}
	w.autoMu.Unlock()
	w.NoteActivity() // locked → must NOT record activity
	w.autoMu.Lock()
	stamped := !w.lastActivity.IsZero()
	w.autoMu.Unlock()
	if stamped {
		t.Fatal("NoteActivity while locked must be a no-op")
	}
}

// 0 = Never: even ancient activity never reads as expired.
func TestAutoLock_ZeroMeansNever(t *testing.T) {
	w := newTestWalletService(t)
	unlockTestWallet(t, w)
	w.startAutoLock()
	t.Cleanup(w.stopAutoLock)
	if err := w.SetAutoLockMinutes(0); err != nil {
		t.Fatalf("SetAutoLockMinutes(0): %v", err)
	}
	w.autoMu.Lock()
	w.lastActivity = time.Now().Add(-24 * time.Hour)
	w.autoMu.Unlock()
	if w.autoLockExpired(time.Now()) {
		t.Fatal("0 (Never) must never expire")
	}
}

// The setter validates against the preset set, persists, and updates the live
// timeout; manual Lock stops the watchdog.
func TestAutoLock_SetterValidatesPersistsAndLockStops(t *testing.T) {
	w := newTestWalletService(t)
	unlockTestWallet(t, w)
	w.startAutoLock()

	if err := w.SetAutoLockMinutes(7); err == nil {
		t.Fatal("7 is not a preset; must be rejected")
	}
	if err := w.SetAutoLockMinutes(15); err != nil {
		t.Fatalf("SetAutoLockMinutes(15): %v", err)
	}
	s, err := w.config.GetSettings()
	if err != nil || s.AutoLockMinutes == nil || *s.AutoLockMinutes != 15 {
		t.Fatalf("15 must persist, got %v (err %v)", s.AutoLockMinutes, err)
	}
	w.autoMu.Lock()
	live := w.autoLockMins
	w.autoMu.Unlock()
	if live != 15 {
		t.Fatalf("live timeout must update, got %d", live)
	}

	if err := w.Lock(); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	w.autoMu.Lock()
	stopped := w.autoLockStop == nil
	w.autoMu.Unlock()
	if !stopped {
		t.Fatal("manual Lock must stop the watchdog")
	}
}
