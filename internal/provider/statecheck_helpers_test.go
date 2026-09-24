// Copyright (c) ngharo <root@ngha.ro>
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/go-ldap/ldap/v3"
	"github.com/hashicorp/terraform-json"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
)

// Test LDAP server coordinates (started via `make test`).
const (
	testAccLdapURL    = "ldap://localhost:3389"
	testAccLdapBindDN = "cn=Manager,dc=example,dc=com"
	testAccLdapBindPW = "secret"
)

// testAccLdapConn dials and binds a direct connection to the test LDAP
// server, for use by statechecks that verify server-side truth.
func testAccLdapConn() (*ldap.Conn, error) {
	conn, err := ldap.DialURL(testAccLdapURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to test LDAP server: %w", err)
	}
	if err := conn.Bind(testAccLdapBindDN, testAccLdapBindPW); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to bind to test LDAP server: %w", err)
	}
	return conn, nil
}

// stateResourceAtAddress locates a single resource in a parsed state file.
func stateResourceAtAddress(state *tfjson.State, address string) (*tfjson.StateResource, error) {
	if state == nil || state.Values == nil || state.Values.RootModule == nil {
		return nil, fmt.Errorf("no state available")
	}
	for _, r := range state.Values.RootModule.Resources {
		if r.Address == address {
			return r, nil
		}
	}
	return nil, fmt.Errorf("not found in state: %s", address)
}

// stateStringAttr reads a string attribute value from a state resource.
func stateStringAttr(r *tfjson.StateResource, name string) (string, error) {
	v, ok := r.AttributeValues[name]
	if !ok || v == nil {
		return "", fmt.Errorf("no %s found for %s", name, r.Address)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s for %s is not a string: %T", name, r.Address, v)
	}
	return s, nil
}

// ldapValuePresentCheck verifies server-side that the value asserted by an
// ldap_value resource is actually present in the entry's attribute. It reads
// dn/attribute/value from state so a single check works for any resource
// address.
type ldapValuePresentCheck struct {
	resourceAddress string
}

func (c ldapValuePresentCheck) CheckState(ctx context.Context, req statecheck.CheckStateRequest, resp *statecheck.CheckStateResponse) {
	r, err := stateResourceAtAddress(req.State, c.resourceAddress)
	if err != nil {
		resp.Error = err
		return
	}

	dn, err := stateStringAttr(r, "dn")
	if err != nil {
		resp.Error = err
		return
	}
	attribute, err := stateStringAttr(r, "attribute")
	if err != nil {
		resp.Error = err
		return
	}
	value, err := stateStringAttr(r, "value")
	if err != nil {
		resp.Error = err
		return
	}

	conn, err := testAccLdapConn()
	if err != nil {
		resp.Error = err
		return
	}
	defer conn.Close()

	sr, err := LdapSearch(ctx, conn, dn, "base", "(objectClass=*)", []string{attribute})
	if err != nil {
		resp.Error = fmt.Errorf("failed to search LDAP for %s: %w", dn, err)
		return
	}
	if len(sr.Entries) == 0 {
		resp.Error = fmt.Errorf("entry %s not found", dn)
		return
	}

	for _, attr := range sr.Entries[0].Attributes {
		if attr.Name != attribute {
			continue
		}
		for _, v := range attr.Values {
			if v == value {
				return
			}
		}
	}

	resp.Error = fmt.Errorf("value %q not found in attribute %q on %s", value, attribute, dn)
}

// stateCheckLdapValuePresent asserts that the ldap_value at the given address
// exists on the LDAP server.
//
// steps and future tests, even though current callers all use one address.
//
//nolint:unparam // resourceAddress is a parameter by design for reuse across
func stateCheckLdapValuePresent(resourceAddress string) statecheck.StateCheck {
	return ldapValuePresentCheck{resourceAddress: resourceAddress}
}

// ldapEntryAttributeExistsCheck verifies server-side that an attribute exists
// on the entry managed by an ldap_entry resource.
type ldapEntryAttributeExistsCheck struct {
	resourceAddress string
	attribute       string
}

func (c ldapEntryAttributeExistsCheck) CheckState(ctx context.Context, req statecheck.CheckStateRequest, resp *statecheck.CheckStateResponse) {
	r, err := stateResourceAtAddress(req.State, c.resourceAddress)
	if err != nil {
		resp.Error = err
		return
	}

	dn, err := stateStringAttr(r, "dn")
	if err != nil {
		resp.Error = err
		return
	}

	conn, err := testAccLdapConn()
	if err != nil {
		resp.Error = err
		return
	}
	defer conn.Close()

	exists, _, err := AttributeExistsInLDAP(ctx, conn, dn, c.attribute)
	if err != nil {
		resp.Error = fmt.Errorf("error searching for entry %s: %w", dn, err)
		return
	}
	if !exists {
		resp.Error = fmt.Errorf("attribute %s does not exist on LDAP entry %s", c.attribute, dn)
	}
}

// stateCheckLdapEntryAttributeExists asserts that the named attribute exists
// on the ldap_entry at the given address.
func stateCheckLdapEntryAttributeExists(resourceAddress, attribute string) statecheck.StateCheck {
	return ldapEntryAttributeExistsCheck{resourceAddress: resourceAddress, attribute: attribute}
}

// noResourceAttrCheck asserts that an attribute is entirely absent from state
// (as opposed to being present with a null value). This is the statecheck
// equivalent of resource.TestCheckNoResourceAttr, used to verify write-only
// attributes are never persisted.
type noResourceAttrCheck struct {
	resourceAddress string
	attribute       string
}

func (c noResourceAttrCheck) CheckState(ctx context.Context, req statecheck.CheckStateRequest, resp *statecheck.CheckStateResponse) {
	r, err := stateResourceAtAddress(req.State, c.resourceAddress)
	if err != nil {
		resp.Error = err
		return
	}

	if v, ok := r.AttributeValues[c.attribute]; ok && v != nil {
		resp.Error = fmt.Errorf("%s: attribute %q unexpectedly found in state (value: %v)", c.resourceAddress, c.attribute, v)
	}
}

// stateCheckNoResourceAttr asserts that the named attribute is absent from
// state for the resource at the given address.
func stateCheckNoResourceAttr(resourceAddress, attribute string) statecheck.StateCheck {
	return noResourceAttrCheck{resourceAddress: resourceAddress, attribute: attribute}
}
