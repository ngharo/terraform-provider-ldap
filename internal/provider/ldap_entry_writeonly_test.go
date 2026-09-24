// Copyright (c) ngharo <root@ngha.ro>
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
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
					// Verify write-only attributes are NOT in state, and that the
					// attribute was created on the LDAP server.
					stateCheckNoResourceAttr("ldap_entry.test_writeonly", "attributes_wo"),
					stateCheckLdapEntryAttributeExists("ldap_entry.test_writeonly", "userPassword"),
				},
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
					// Verify write-only attributes are still NOT in state, and that
					// the attribute still exists on the LDAP server.
					stateCheckNoResourceAttr("ldap_entry.test_writeonly", "attributes_wo"),
					stateCheckLdapEntryAttributeExists("ldap_entry.test_writeonly", "userPassword"),
				},
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
					stateCheckNoResourceAttr("ldap_entry.test_writeonly", "attributes_wo"),
				},
			},
		},
	})
}

// TestAccLdapEntryResource_WriteOnlyMissingVersion verifies that a non-empty
// attributes_wo without attributes_wo_version is rejected at plan time.
func TestAccLdapEntryResource_WriteOnlyMissingVersion(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccLdapEntryResourceConfigWriteOnlyMissingVersion("cn=writeonly-noversion,dc=example,dc=com"),
				ExpectError: regexp.MustCompile(`Missing attributes_wo_version`),
			},
		},
	})
}

// TestAccLdapEntryResource_WriteOnlyVersionMissingAttributes verifies that
// attributes_wo_version without attributes_wo is rejected at plan time.
func TestAccLdapEntryResource_WriteOnlyVersionMissingAttributes(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccLdapEntryResourceConfigWriteOnlyVersionOnly("cn=writeonly-novo,dc=example,dc=com"),
				ExpectError: regexp.MustCompile(`Missing attributes_wo`),
			},
		},
	})
}

func testAccLdapEntryResourceConfigWriteOnlyMissingVersion(dn string) string {
	return fmt.Sprintf(`
provider "ldap" {
  url = "ldap://localhost:3389"
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

resource "ldap_entry" "test_writeonly" {
  dn = %[1]q
  attributes = {
    objectClass = ["person", "organizationalPerson", "inetOrgPerson"]
    cn = ["writeonly-noversion"]
    sn = ["User"]
  }
  attributes_wo = {
    userPassword = ["secret123"]
  }
}
`, dn)
}

func testAccLdapEntryResourceConfigWriteOnlyVersionOnly(dn string) string {
	return fmt.Sprintf(`
provider "ldap" {
  url = "ldap://localhost:3389"
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

resource "ldap_entry" "test_writeonly" {
  dn = %[1]q
  attributes = {
    objectClass = ["person", "organizationalPerson", "inetOrgPerson"]
    cn = ["writeonly-novo"]
    sn = ["User"]
  }
  attributes_wo_version = 1
}
`, dn)
}

func testAccLdapEntryResourceConfigWithWriteOnly(dn, password string, version int) string {
	return fmt.Sprintf(`
provider "ldap" {
  url = "ldap://localhost:3389"
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
  url = "ldap://localhost:3389"
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
