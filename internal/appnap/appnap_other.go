//go:build !darwin || !cgo

// Package appnap holds a macOS App Nap prevention assertion while the embedded
// node runs. On non-darwin platforms (and cgo-less builds) there is no App Nap,
// so Begin/End are no-ops.
package appnap

// Begin starts an App Nap prevention assertion (no-op on this platform).
func Begin(reason string) {}

// End releases the active assertion (no-op on this platform).
func End() {}
