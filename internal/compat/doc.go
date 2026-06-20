// Package compat holds tests proving behavioural compatibility with the
// original Flutter syrius wallet.
//
// TestSyriusKeystoreRoundTrip reads a real syrius keystore from the gitignored
// secrets/ folder at runtime (never from committed testdata) and confirms that
// go-syrius opens it and derives the same index-0 address syrius recorded.
//
// It uses go-zenon's wallet package directly rather than znn-sdk-go's wallet:
// the SDK cannot read real syrius keystores (it JSON-wraps the entropy payload,
// while syrius/go-zenon encrypt raw BIP-39 entropy). go-zenon's wallet is the
// canonical implementation, so it is guaranteed compatible and needs no SDK
// changes.
//
// The keystore may be a funded wallet, so it is never committed; only
// secret-free test code lives here. The test skips when no local keystore is
// present, keeping the offline/CI suite green.
package compat
