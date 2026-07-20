package app

import (
	"errors"
	"testing"

	"github.com/zenon-network/go-zenon/common/types"
)

// customZts is a valid, parseable ZTS that is NOT ZNN or QSR, used to exercise
// the node-lookup branch of the decimals resolver.
const customZts = "zts1x27drtpzgj99rjxcm7xmmg"

// failLookup must never be invoked: ZNN/QSR resolve without any node call.
func failLookup(t *testing.T) ztsDecimalsLookup {
	t.Helper()
	return func(types.ZenonTokenStandard) (int, error) {
		t.Fatal("lookup must not be called for ZNN/QSR")
		return 0, nil
	}
}

func TestResolveDecimalsNativeNoLookup(t *testing.T) {
	if d := resolveDecimals(types.ZnnTokenStandard.String(), failLookup(t)); d != 8 {
		t.Fatalf("ZNN decimals = %d, want 8", d)
	}
	if d := resolveDecimals(types.QsrTokenStandard.String(), failLookup(t)); d != 8 {
		t.Fatalf("QSR decimals = %d, want 8", d)
	}
	// A nil lookup still resolves ZNN/QSR to 8 without panicking.
	if d := resolveDecimals(types.ZnnTokenStandard.String(), nil); d != 8 {
		t.Fatalf("ZNN decimals (nil lookup) = %d, want 8", d)
	}
}

func TestResolveDecimalsCustomToken(t *testing.T) {
	parsed, err := types.ParseZTS(customZts)
	if err != nil {
		t.Fatalf("ParseZTS: %v", err)
	}
	called := false
	lookup := func(z types.ZenonTokenStandard) (int, error) {
		called = true
		if z != parsed {
			t.Fatalf("lookup got %s, want %s", z.String(), parsed.String())
		}
		return 6, nil
	}
	if d := resolveDecimals(customZts, lookup); d != 6 {
		t.Fatalf("custom decimals = %d, want 6", d)
	}
	if !called {
		t.Fatal("lookup was not called for a custom token")
	}
}

func TestResolveDecimalsFallsBackOnError(t *testing.T) {
	lookup := func(types.ZenonTokenStandard) (int, error) {
		return 0, errors.New("node down")
	}
	if d := resolveDecimals(customZts, lookup); d != 8 {
		t.Fatalf("decimals on lookup error = %d, want fallback 8", d)
	}
}

func TestResolveDecimalsFallsBackOnBadZts(t *testing.T) {
	if d := resolveDecimals("not-a-zts", failLookup(t)); d != 8 {
		t.Fatalf("decimals on bad zts = %d, want fallback 8", d)
	}
}

func TestDecimalsCacheQueriesOncePerZts(t *testing.T) {
	calls := 0
	lookup := func(types.ZenonTokenStandard) (int, error) {
		calls++
		return 6, nil
	}
	dc := newDecimalsCache(lookup)
	// Three rows sharing the same custom token must query the node once.
	for i := 0; i < 3; i++ {
		if d := dc.get(customZts); d != 6 {
			t.Fatalf("cache.get = %d, want 6", d)
		}
	}
	if calls != 1 {
		t.Fatalf("lookup called %d times, want 1 (cached)", calls)
	}
	// ZNN/QSR never hit the lookup at all.
	if d := dc.get(types.ZnnTokenStandard.String()); d != 8 {
		t.Fatalf("ZNN via cache = %d, want 8", d)
	}
	if calls != 1 {
		t.Fatalf("ZNN must not invoke lookup; calls = %d", calls)
	}
}

func TestResolveDecimalsCheckedNativeNoLookup(t *testing.T) {
	for _, zts := range []string{types.ZnnTokenStandard.String(), types.QsrTokenStandard.String()} {
		d, err := resolveDecimalsChecked(zts, failLookup(t))
		if err != nil || d != 8 {
			t.Fatalf("resolveDecimalsChecked(%s) = %d, %v; want 8, nil", zts, d, err)
		}
	}
}

func TestResolveDecimalsCheckedCustomToken(t *testing.T) {
	lookup := func(types.ZenonTokenStandard) (int, error) { return 2, nil }
	d, err := resolveDecimalsChecked(customZts, lookup)
	if err != nil || d != 2 {
		t.Fatalf("resolveDecimalsChecked = %d, %v; want 2, nil", d, err)
	}
}

func TestResolveDecimalsCheckedFailsInsteadOfGuessing(t *testing.T) {
	// WC-03: a confirmation must never silently render a custom-token amount
	// with assumed decimals. Unresolvable metadata is an error, not an 8.
	lookup := func(types.ZenonTokenStandard) (int, error) { return 0, errors.New("node down") }
	if _, err := resolveDecimalsChecked(customZts, lookup); err == nil {
		t.Fatal("expected error when custom-token decimals cannot be resolved")
	}
	if _, err := resolveDecimalsChecked("not-a-zts", failLookup(t)); err == nil {
		t.Fatal("expected error for an unparseable ZTS")
	}
	if _, err := resolveDecimalsChecked(customZts, nil); err == nil {
		t.Fatal("expected error when no lookup is available")
	}
}

func TestClientTokenDecimalsContractTreatsMissingTokenAsError(t *testing.T) {
	// Round-2 finding 3: a missing token must be an ERROR from the strict
	// resolver's perspective, never a silent (8, nil) guess. This pins the
	// resolveDecimalsChecked contract for the lookup used at prepare time.
	missing := func(types.ZenonTokenStandard) (int, error) {
		return 0, errTokenNotFound
	}
	if _, err := resolveDecimalsChecked(customZts, missing); err == nil {
		t.Fatal("missing token metadata must fail the strict decimals check")
	}
	// The display path still degrades to 8 for the same condition.
	if d := resolveDecimals(customZts, missing); d != 8 {
		t.Fatalf("display fallback = %d, want 8", d)
	}
}

// GS-07: node-supplied token decimals must be bounded to the protocol range
// [0,18] (issuance enforces it on-chain; a lying node must not skew display).
func TestBoundTokenDecimals(t *testing.T) {
	zts := types.ZnnTokenStandard
	for _, ok := range []int{0, 8, 18} {
		if d, err := boundTokenDecimals(ok, zts); err != nil || d != ok {
			t.Fatalf("valid decimals %d rejected: %d, %v", ok, d, err)
		}
	}
	for _, bad := range []int{-1, 19, 200} {
		if _, err := boundTokenDecimals(bad, zts); err == nil {
			t.Fatalf("implausible decimals %d must be rejected", bad)
		}
	}
}
