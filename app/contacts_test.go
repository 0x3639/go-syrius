package app

import "testing"

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
