package embeddednode

import (
	"path/filepath"
	"testing"
)

func TestBuildConfigLoopbackAndGenesis(t *testing.T) {
	cfg := buildConfig("/tmp/data")
	if cfg.DataPath != filepath.Join("/tmp/data", "embedded") {
		t.Fatalf("DataPath = %q", cfg.DataPath)
	}
	if cfg.GenesisFile != "" {
		t.Fatalf("GenesisFile must be empty to use embedded genesis, got %q", cfg.GenesisFile)
	}
	if cfg.Producer != nil {
		t.Fatalf("Producer must be nil")
	}
	if !cfg.RPC.EnableWS || cfg.RPC.EnableHTTP {
		t.Fatalf("WS must be enabled and HTTP disabled: %+v", cfg.RPC)
	}
	if cfg.RPC.WSHost != "127.0.0.1" || cfg.RPC.WSPort != EmbeddedWSPort {
		t.Fatalf("WS must bind loopback:%d, got %s:%d", EmbeddedWSPort, cfg.RPC.WSHost, cfg.RPC.WSPort)
	}
	if len(cfg.Net.Seeders) == 0 {
		t.Fatalf("expected built-in seeders to be preserved")
	}
	// An EMPTY origin list is go-zenon's localhost-any-port default, which lets
	// any browser page served from localhost drive the RPC. The config must pin
	// a sentinel origin no browser can present; no-Origin (native) clients —
	// the wallet's own — always pass the validator regardless.
	if len(cfg.RPC.WSOrigins) == 0 {
		t.Fatal("WSOrigins must not be empty (empty ⇒ localhost browser pages allowed)")
	}
	for _, o := range cfg.RPC.WSOrigins {
		if o == "*" || o == "http://localhost" {
			t.Fatalf("WSOrigins must not admit browser origins, got %q", o)
		}
	}
}
