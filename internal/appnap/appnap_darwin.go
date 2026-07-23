//go:build darwin && cgo

// Package appnap holds a macOS App Nap prevention assertion while the embedded
// node runs. Without it, macOS throttles the whole GUI process — timers,
// network, CPU priority — whenever the window is occluded or minimized, and
// the in-process go-zenon node syncs at a crawl.
package appnap

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation
#import <Foundation/Foundation.h>

static void *appnap_begin(const char *reason) {
	// UserInitiatedAllowingIdleSystemSleep disables App Nap and automatic
	// termination but deliberately still allows idle SYSTEM sleep: keeping the
	// node responsive must not override the user's lid/energy settings.
	NSActivityOptions opts = NSActivityUserInitiatedAllowingIdleSystemSleep;
	id token = [[NSProcessInfo processInfo]
	    beginActivityWithOptions:opts
	                      reason:[NSString stringWithUTF8String:reason]];
	return (void *)CFBridgingRetain(token);
}

static void appnap_end(void *token) {
	id t = CFBridgingRelease(token);
	[[NSProcessInfo processInfo] endActivity:t];
}
*/
import "C"

import (
	"sync"
	"unsafe"
)

var (
	mu    sync.Mutex
	token unsafe.Pointer // nil when no assertion is active
)

// Begin starts an App Nap prevention assertion. Idempotent: a second Begin
// while one is active keeps the existing assertion.
func Begin(reason string) {
	mu.Lock()
	defer mu.Unlock()
	if token != nil {
		return
	}
	cs := C.CString(reason)
	defer C.free(unsafe.Pointer(cs))
	token = C.appnap_begin(cs)
}

// End releases the active assertion. Idempotent: a no-op when none is active
// (Foundation raises on ending a stale token, so End must never double-end).
func End() {
	mu.Lock()
	defer mu.Unlock()
	if token == nil {
		return
	}
	C.appnap_end(token)
	token = nil
}
