package app

// maxPagedPages is the hard safety cap on node-driven pagination, mirroring
// SearchTokens' maxPages (nom_service.go). At the standard pageSize of 50 it
// admits 2500 items — far above any legitimate on-chain list.
const maxPagedPages = 50

// maxPagedItems bounds the ACCUMULATED item count. The page cap alone is not
// enough: page contents are node-supplied, so a single oversized page (or 50
// of them) could still grow the result without limit.
const maxPagedItems = maxPagedPages * 50

// collectPaged pages through fetch until the node's claimed total is reached,
// an empty page arrives, or a hard cap trips (maxPagedPages pages or
// maxPagedItems accumulated items). The total AND the page contents are
// NODE-SUPPLIED and untrusted: without the caps, a malicious node returning an
// inflated total with endless (or oversized) non-empty pages drives the loop —
// and the wallet's memory — unbounded (audit GS-03).
// On cap, the items collected so far are returned (mirrors SearchTokens).
func collectPaged[T any](fetch func(pageIndex uint32) (page []T, total int, err error)) ([]T, error) {
	out := []T{}
	for pageIndex := uint32(0); pageIndex < maxPagedPages; pageIndex++ {
		page, total, err := fetch(pageIndex)
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
		if len(out) >= maxPagedItems {
			out = out[:maxPagedItems]
			break
		}
		if len(out) >= total || len(page) == 0 {
			break
		}
	}
	return out, nil
}
