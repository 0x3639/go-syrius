//go:build !windows

package app

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestImportRejectsNonRegularSource: a FIFO (or device, socket) must be
// rejected without ever blocking the bound call. The source is opened once
// with O_NONBLOCK (a blocking open of a FIFO with no writer never returns) and
// then fstat'd on that descriptor, so the rejection is bound to the file that
// was actually opened rather than to a separate path lookup.
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
		t.Fatal("ImportKeystore blocked on a FIFO: open must be non-blocking and the opened descriptor fstat-checked before reading")
	}
}
