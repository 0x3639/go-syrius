package app

import (
	"errors"
	"testing"
)

// A malicious node can return an inflated Count with endless non-empty pages
// (GS-03); collectPaged must stop at the hard cap instead of looping forever.
func TestCollectPaged_CapsMaliciousCount(t *testing.T) {
	calls := 0
	out, err := collectPaged(func(pageIndex uint32) ([]int, int, error) {
		calls++
		return []int{1, 2, 3}, 1 << 30, nil // always-full page, absurd total
	})
	if err != nil {
		t.Fatalf("capped collection must not error: %v", err)
	}
	if calls != maxPagedPages {
		t.Fatalf("must stop at the page cap: %d calls, want %d", calls, maxPagedPages)
	}
	if len(out) != maxPagedPages*3 {
		t.Fatalf("unexpected item count %d", len(out))
	}
}

func TestCollectPaged_NormalTermination(t *testing.T) {
	// Terminates when the claimed total is reached.
	pages := [][]int{{1, 2}, {3}}
	out, err := collectPaged(func(i uint32) ([]int, int, error) {
		if int(i) >= len(pages) {
			t.Fatal("fetched past the final page")
		}
		return pages[i], 3, nil
	})
	if err != nil || len(out) != 3 {
		t.Fatalf("got %v (err %v), want 3 items", out, err)
	}
	// Terminates on an empty page even when the claimed total is never reached.
	out, err = collectPaged(func(i uint32) ([]int, int, error) {
		if i == 0 {
			return []int{9}, 100, nil
		}
		return nil, 100, nil
	})
	if err != nil || len(out) != 1 {
		t.Fatalf("empty page must terminate: %v (err %v)", out, err)
	}
}

func TestCollectPaged_PropagatesError(t *testing.T) {
	boom := errors.New("rpc failed")
	if _, err := collectPaged(func(uint32) ([]int, int, error) { return nil, 0, boom }); !errors.Is(err, boom) {
		t.Fatalf("want fetch error, got %v", err)
	}
}
