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
}
