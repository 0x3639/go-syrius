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
