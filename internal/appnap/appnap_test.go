package appnap

import "testing"

// Begin/End must be safe to call in any order and any number of times: the
// NodeService wiring pairs them with embedded start/stop, but teardown paths
// can run without a prior start (and darwin's activity token must never be
// double-ended — Foundation raises on a stale token).
func TestBeginEndIdempotent(t *testing.T) {
	End() // end with no begin: no-op
	Begin("test sync")
	Begin("test sync again") // second begin: keeps the existing assertion
	End()
	End() // second end: no-op
}
