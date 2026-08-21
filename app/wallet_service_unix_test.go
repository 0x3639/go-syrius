//go:build !windows

package app

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestImportRejectsNonRegularSource: a FIFO (or device, socket, directory)
// must be rejected by a stat check rather than opened — opening a FIFO with
// no writer blocks forever, which would wedge the bound call.
func TestImportRejectsNonRegularSource(t *testing.T) {
	w := newTestWalletService(t)
	fifo := filepath.Join(t.TempDir(), "pipe.dat")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	done := make(chan error, 1)
	go func() { _, err := w.ImportKeystore(fifo, ""); done <- err }()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "regular file") {
			t.Fatalf("expected non-regular-file rejection, got: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ImportKeystore blocked opening a FIFO: source is not stat-checked before reading")
	}
}
