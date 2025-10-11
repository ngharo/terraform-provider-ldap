// Copyright (c) ngharo <root@ngha.ro>
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/go-ldap/ldap/v3"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccLdapEntryResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLdapEntryDestroy,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccLdapEntryResourceConfig("cn=test,dc=example,dc=com"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"ldap_entry.test",
						tfjsonpath.New("dn"),
						knownvalue.StringExact("cn=test,dc=example,dc=com"),
					),
					statecheck.ExpectKnownValue(
						"ldap_entry.test",
						tfjsonpath.New("id"),
						knownvalue.StringExact("cn=test,dc=example,dc=com"),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:            "ldap_entry.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"attributes"},
			},
			// Update and Read testing
			{
				Config: testAccLdapEntryResourceConfigUpdated("cn=test,dc=example,dc=com"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"ldap_entry.test",
						tfjsonpath.New("dn"),
						knownvalue.StringExact("cn=test,dc=example,dc=com"),
					),
					statecheck.ExpectKnownValue(
						"ldap_entry.test",
						tfjsonpath.New("id"),
						knownvalue.StringExact("cn=test,dc=example,dc=com"),
					),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccLdapEntryResourceConfig(dn string) string {
	return fmt.Sprintf(`
provider "ldap" {
  host = "localhost"
  port = 3389
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

resource "ldap_entry" "test" {
  dn = %[1]q
  attributes = {
    objectClass = ["person", "organizationalPerson", "inetOrgPerson"]
    cn = ["test"]
    sn = ["user"]
    mail = ["test@example.com"]
  }
}
`, dn)
}

func testAccLdapEntryResourceConfigUpdated(dn string) string {
	return fmt.Sprintf(`
provider "ldap" {
  host = "localhost"
  port = 3389
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

resource "ldap_entry" "test" {
  dn = %[1]q
  attributes = {
    objectClass = ["person", "organizationalPerson", "inetOrgPerson"]
    cn = ["test"]
    sn = ["user"]
    mail = ["test.updated@example.com"]
    description = ["Updated user"]
  }
}
`, dn)
}

func testAccCheckLdapEntryDestroy(s *terraform.State) error {
	// Create LDAP connection to verify entries are destroyed
	conn, err := ldap.DialURL("ldap://localhost:3389")
	if err != nil {
		return fmt.Errorf("failed to connect to LDAP server: %w", err)
	}
	defer conn.Close()

	// Bind to LDAP server
	err = conn.Bind("cn=Manager,dc=example,dc=com", "secret")
	if err != nil {
		return fmt.Errorf("failed to bind to LDAP server: %w", err)
	}

	// Check each resource in the state
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "ldap_entry" {
			continue
		}

		dn := rs.Primary.ID

		// Search for the entry
		searchReq := ldap.NewSearchRequest(
			dn,
			ldap.ScopeBaseObject,
			ldap.NeverDerefAliases,
			0,
			0,
			false,
			"(objectClass=*)",
			[]string{"dn"},
			nil,
		)

		result, err := conn.Search(searchReq)
		if err != nil {
			// If we get an LDAP error indicating the entry doesn't exist, that's expected
			if ldapErr, ok := err.(*ldap.Error); ok {
				if ldapErr.ResultCode == ldap.LDAPResultNoSuchObject {
					// Entry successfully destroyed
					continue
				}
			}
			return fmt.Errorf("error searching for entry %s: %w", dn, err)
		}

		// If we found entries, the destroy failed
		if len(result.Entries) > 0 {
			return fmt.Errorf("LDAP entry %s still exists on server after destroy", dn)
		}
	}

	return nil
}
