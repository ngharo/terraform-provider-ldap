// Copyright (c) HashiCorp, Inc.
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

func TestAccLdapEntryResource_WriteOnlyAttributes(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLdapEntryDestroy,
		Steps: []resource.TestStep{
			// Create with write-only attributes
			{
				Config: testAccLdapEntryResourceConfigWithWriteOnly("cn=writeonly,dc=example,dc=com", "secret123", 1),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"ldap_entry.test_writeonly",
						tfjsonpath.New("dn"),
						knownvalue.StringExact("cn=writeonly,dc=example,dc=com"),
					),
					statecheck.ExpectKnownValue(
						"ldap_entry.test_writeonly",
						tfjsonpath.New("attributes_wo_version"),
						knownvalue.Int64Exact(1),
					),
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify write-only attributes are NOT in state
					resource.TestCheckNoResourceAttr("ldap_entry.test_writeonly", "attributes_wo"),
					// Verify the attribute was created on LDAP server
					testAccCheckLdapAttributeExists("ldap_entry.test_writeonly", "userPassword"),
				),
			},
			// Update write-only attributes by changing version
			{
				Config: testAccLdapEntryResourceConfigWithWriteOnly("cn=writeonly,dc=example,dc=com", "newsecret456", 2),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"ldap_entry.test_writeonly",
						tfjsonpath.New("attributes_wo_version"),
						knownvalue.Int64Exact(2),
					),
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					// Verify write-only attributes are still NOT in state
					resource.TestCheckNoResourceAttr("ldap_entry.test_writeonly", "attributes_wo"),
					// Verify the attribute still exists on LDAP server
					testAccCheckLdapAttributeExists("ldap_entry.test_writeonly", "userPassword"),
				),
			},
			// Update without changing version - write-only attrs should NOT be sent
			{
				Config: testAccLdapEntryResourceConfigWithWriteOnlyAndRegularUpdate("cn=writeonly,dc=example,dc=com", "newsecret456", 2),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"ldap_entry.test_writeonly",
						tfjsonpath.New("attributes_wo_version"),
						knownvalue.Int64Exact(2),
					),
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("ldap_entry.test_writeonly", "attributes_wo"),
				),
			},
		},
	})
}

func testAccLdapEntryResourceConfigWithWriteOnly(dn, password string, version int) string {
	return fmt.Sprintf(`
provider "ldap" {
  host = "localhost"
  port = 3389
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

resource "ldap_entry" "test_writeonly" {
  dn = %[1]q
  attributes = {
    objectClass = ["person", "organizationalPerson", "inetOrgPerson"]
    cn = ["writeonly"]
    sn = ["User"]
  }
  attributes_wo = {
    userPassword = [%[2]q]
  }
  attributes_wo_version = %[3]d
}
`, dn, password, version)
}

func testAccLdapEntryResourceConfigWithWriteOnlyAndRegularUpdate(dn, password string, version int) string {
	return fmt.Sprintf(`
provider "ldap" {
  host = "localhost"
  port = 3389
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

resource "ldap_entry" "test_writeonly" {
  dn = %[1]q
  attributes = {
    objectClass = ["person", "organizationalPerson", "inetOrgPerson"]
    cn = ["writeonly"]
    sn = ["User"]
    description = ["Updated description"]
  }
  attributes_wo = {
    userPassword = [%[2]q]
  }
  attributes_wo_version = %[3]d
}
`, dn, password, version)
}

// testAccCheckLdapAttributeExists checks if a specific attribute exists on an LDAP entry.
func testAccCheckLdapAttributeExists(resourceName, attrName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		dn := rs.Primary.ID

		// Create LDAP connection
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

		// Search for the entry with the specific attribute
		searchReq := ldap.NewSearchRequest(
			dn,
			ldap.ScopeBaseObject,
			ldap.NeverDerefAliases,
			0,
			0,
			false,
			"(objectClass=*)",
			[]string{attrName},
			nil,
		)

		result, err := conn.Search(searchReq)
		if err != nil {
			return fmt.Errorf("error searching for entry %s: %w", dn, err)
		}

		if len(result.Entries) == 0 {
			return fmt.Errorf("LDAP entry %s not found", dn)
		}

		entry := result.Entries[0]
		attrValues := entry.GetAttributeValues(attrName)
		if len(attrValues) == 0 {
			return fmt.Errorf("attribute %s does not exist on LDAP entry %s", attrName, dn)
		}

		return nil
	}
}
