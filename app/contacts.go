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

// AddContact validates and saves an address-book entry, replacing any existing
// entry with the same address. The address is validated as a real z1 address.
func (c *ConfigService) AddContact(name, address string) ([]Contact, error) {
	name = strings.TrimSpace(name)
	address = strings.TrimSpace(address)
	if name == "" {
		return nil, fmt.Errorf("contact name must not be empty")
	}
	if _, err := types.ParseAddress(address); err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}
	s, err := c.GetSettings()
	if err != nil {
		return nil, err
	}
	replaced := false
	for i := range s.Contacts {
		if s.Contacts[i].Address == address {
			s.Contacts[i].Name = name
			replaced = true
			break
		}
	}
	if !replaced {
		s.Contacts = append(s.Contacts, Contact{Name: name, Address: address})
	}
	if err := c.SetSettings(s); err != nil {
		return nil, err
	}
	return s.Contacts, nil
}

// DeleteContact removes the address-book entry with the given address.
func (c *ConfigService) DeleteContact(address string) ([]Contact, error) {
	s, err := c.GetSettings()
	if err != nil {
		return nil, err
	}
	kept := make([]Contact, 0, len(s.Contacts))
	for _, ct := range s.Contacts {
		if ct.Address != address {
			kept = append(kept, ct)
		}
	}
	s.Contacts = kept
	if err := c.SetSettings(s); err != nil {
		return nil, err
	}
	return kept, nil
}
