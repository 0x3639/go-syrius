package app

import (
	"fmt"
	"strings"

	"github.com/zenon-network/go-zenon/common/types"
)

// ListContacts returns the saved address-book entries (never nil).
func (c *ConfigService) ListContacts() ([]Contact, error) {
	s, err := c.GetSettings()
	if err != nil {
		return nil, err
	}
	if s.Contacts == nil {
		return []Contact{}, nil
	}
	return s.Contacts, nil
}

// canonicalAddress returns the canonical (lowercase bech32) form of a z1
// address. Bech32 is case-insensitive, so "Z1…" and "z1…" name the same
// account; comparing raw strings would treat them as distinct entries. Input
// that does not parse (a legacy or corrupt stored entry) is matched verbatim.
func canonicalAddress(a string) string {
	parsed, err := types.ParseAddress(a)
	if err != nil {
		return a
	}
	return parsed.String()
}

// AddContact validates and saves an address-book entry, replacing any existing
// entry with the same address. The address is validated as a real z1 address
// and stored in canonical form, so case variants of one address cannot create
// duplicate entries.
func (c *ConfigService) AddContact(name, address string) ([]Contact, error) {
	name = strings.TrimSpace(name)
	parsed, err := types.ParseAddress(strings.TrimSpace(address))
	if err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}
	address = parsed.String()
	if name == "" {
		return nil, fmt.Errorf("contact name must not be empty")
	}
	var out []Contact
	err = c.updateSettings(func(s *Settings) error {
		replaced := false
		for i := range s.Contacts {
			if canonicalAddress(s.Contacts[i].Address) == address {
				s.Contacts[i].Name = name
				replaced = true
				break
			}
		}
		if !replaced {
			s.Contacts = append(s.Contacts, Contact{Name: name, Address: address})
		}
		out = s.Contacts
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteContact removes the address-book entry with the given address,
// matching case variants of the same bech32 address.
func (c *ConfigService) DeleteContact(address string) ([]Contact, error) {
	target := canonicalAddress(strings.TrimSpace(address))
	var kept []Contact
	err := c.updateSettings(func(s *Settings) error {
		kept = make([]Contact, 0, len(s.Contacts))
		for _, ct := range s.Contacts {
			if canonicalAddress(ct.Address) != target {
				kept = append(kept, ct)
			}
		}
		s.Contacts = kept
		return nil
	})
	if err != nil {
		return nil, err
	}
	return kept, nil
}
