package compat

import (
	"os"
	"strings"
	"testing"

	"github.com/zenon-network/go-zenon/wallet"
)

// Default location of the local syrius keystore used for the compatibility
// check. This is a gitignored secrets file (see repo .gitignore) and is NEVER
// committed — it may be a real, funded wallet. Override with env vars:
//
//	ZNN_COMPAT_KEYSTORE  full path to a syrius keystore file
//	ZNN_COMPAT_PASSWORD  the keystore password (else read from the password file)
const (
	defaultKeystorePath = "../../secrets/pillar.json"
	defaultPasswordPath  = "../../secrets/pillar-password.txt"
)

// TestSyriusKeystoreRoundTrip proves go-syrius can open a real syrius keystore
// and derive the same index-0 address syrius recorded as its baseAddress.
//
// It uses go-zenon's wallet package directly (the canonical implementation),
// NOT znn-sdk-go's wallet, which cannot read real syrius keystores: the SDK
// JSON-wraps the entropy payload while go-zenon/syrius encrypt raw entropy.
// The expected address comes from the keystore file itself, so no secret is
// embedded in committed code. Skips when no local keystore is present.
func TestSyriusKeystoreRoundTrip(t *testing.T) {
	keystorePath := os.Getenv("ZNN_COMPAT_KEYSTORE")
	if keystorePath == "" {
		keystorePath = defaultKeystorePath
	}
	if _, err := os.Stat(keystorePath); err != nil {
		t.Skip("no local syrius keystore present (set ZNN_COMPAT_KEYSTORE to run)")
	}

	password := os.Getenv("ZNN_COMPAT_PASSWORD")
	if password == "" {
		raw, err := os.ReadFile(defaultPasswordPath)
		if err != nil {
			t.Skipf("no password available (set ZNN_COMPAT_PASSWORD or provide %s)", defaultPasswordPath)
		}
		password = strings.TrimSpace(string(raw))
	}

	kf, err := wallet.ReadKeyFile(keystorePath)
	if err != nil {
		t.Fatalf("ReadKeyFile: %v", err)
	}

	ks, err := kf.Decrypt(password)
	if err != nil {
		t.Fatalf("Decrypt (wrong password, or keystore format incompatible): %v", err)
	}

	_, kp, err := ks.DeriveForIndexPath(0)
	if err != nil {
		t.Fatalf("DeriveForIndexPath(0): %v", err)
	}

	if kp.Address != kf.BaseAddress {
		t.Fatalf("index-0 address mismatch:\n  derived %s\n  baseAddr %s\n(derivation is NOT syrius-compatible)", kp.Address.String(), kf.BaseAddress.String())
	}
	t.Logf("keystore opened; index-0 address matches baseAddress %s", kf.BaseAddress.String())
}
