package app

import (
	"strings"
	"testing"

	"github.com/zenon-network/go-zenon/common/types"
)

func TestContacts(t *testing.T) {
	t.Setenv("GO_SYRIUS_DATA_DIR", t.TempDir())
	c := newConfigService()

	if list, err := c.ListContacts(); err != nil || len(list) != 0 {
		t.Fatalf("expected empty contacts, got %v (err %v)", list, err)
	}

	const addr = "z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz"
	list, err := c.AddContact("Alice", addr)
	if err != nil {
		t.Fatalf("AddContact: %v", err)
	}
	if len(list) != 1 || list[0].Name != "Alice" || list[0].Address != addr {
		t.Fatalf("unexpected contacts: %+v", list)
	}

	if _, err := c.AddContact("", addr); err == nil {
		t.Fatal("expected empty name to fail")
	}
	if _, err := c.AddContact("Bad", "not-an-address"); err == nil {
		t.Fatal("expected invalid address to fail")
	}

	// Re-adding the same address updates the name in place (no duplicate).
	list, _ = c.AddContact("Alice2", addr)
	if len(list) != 1 || list[0].Name != "Alice2" {
		t.Fatalf("expected name update without dup, got %+v", list)
	}

	if list, err = c.DeleteContact(addr); err != nil || len(list) != 0 {
		t.Fatalf("expected empty after delete, got %+v (err %v)", list, err)
	}
}

// Bech32 is case-insensitive: an uppercase variant of a saved address is the
// SAME account and must update the existing entry (and be deletable by either
// spelling), not create a second one that the send flow could diverge from.
func TestContactsCaseInsensitiveAddress(t *testing.T) {
	t.Setenv("GO_SYRIUS_DATA_DIR", t.TempDir())
	c := newConfigService()

	const lower = "z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz"
	upper := strings.ToUpper(lower)
	if _, err := types.ParseAddress(upper); err != nil {
		t.Fatalf("premise: uppercase bech32 must parse, got %v", err)
	}

	if _, err := c.AddContact("Alice", lower); err != nil {
		t.Fatalf("AddContact: %v", err)
	}
	list, err := c.AddContact("Mallory", upper)
	if err != nil {
		t.Fatalf("AddContact upper: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("case variant must upsert, not duplicate: %+v", list)
	}
	if list[0].Name != "Mallory" || list[0].Address != lower {
		t.Fatalf("expected canonical address with updated name, got %+v", list[0])
	}

	// Uppercase input is stored canonically even for a fresh entry.
	if list, _ = c.AddContact("Bob", strings.ToUpper("z1qqjnwjjpnue8xmmpanz6csze6tcmtzzdtfsww7")); len(list) != 2 || list[1].Address != "z1qqjnwjjpnue8xmmpanz6csze6tcmtzzdtfsww7" {
		t.Fatalf("fresh entry must be stored canonically, got %+v", list)
	}

	// Delete matches case variants too.
	if list, err = c.DeleteContact(upper); err != nil || len(list) != 1 {
		t.Fatalf("delete by case variant must remove the entry, got %+v (err %v)", list, err)
	}
}
