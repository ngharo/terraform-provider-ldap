// Copyright (c) ngharo <root@ngha.ro>
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccLdapEntryResource_InetOrgPerson(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing for inetOrgPerson
			{
				Config: testAccLdapEntryResourceConfigInetOrgPerson("cn=john.doe,ou=users,dc=example,dc=com"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"ldap_entry.inetorg_user",
						tfjsonpath.New("dn"),
						knownvalue.StringExact("cn=john.doe,ou=users,dc=example,dc=com"),
					),
					statecheck.ExpectKnownValue(
						"ldap_entry.inetorg_user",
						tfjsonpath.New("id"),
						knownvalue.StringExact("cn=john.doe,ou=users,dc=example,dc=com"),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:                         "ldap_entry.inetorg_user",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateVerifyIgnore:              []string{"attributes"},
				ImportStateVerifyIdentifierAttribute: "dn",
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccLdapEntryResource_Group(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing for group
			{
				Config: testAccLdapEntryResourceConfigGroup("cn=developers,ou=groups,dc=example,dc=com"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"ldap_entry.test_group",
						tfjsonpath.New("dn"),
						knownvalue.StringExact("cn=developers,ou=groups,dc=example,dc=com"),
					),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccLdapEntryResource_MinimalAttributes(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with minimal attributes
			{
				Config: testAccLdapEntryResourceConfigMinimal("cn=minimal,ou=users,dc=example,dc=com"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"ldap_entry.minimal",
						tfjsonpath.New("dn"),
						knownvalue.StringExact("cn=minimal,ou=users,dc=example,dc=com"),
					),
				},
			},
		},
	})
}

func testAccLdapEntryResourceConfigInetOrgPerson(dn string) string {
	return fmt.Sprintf(`
provider "ldap" {
  url = "ldap://localhost:3389"
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

resource "ldap_entry" "inetorg_user" {
  dn = %[1]q
  attributes = {
    objectClass = ["person", "organizationalPerson", "inetOrgPerson"]
    cn = ["john.doe"]
    sn = ["Doe"]
    givenName = ["John"]
    mail = ["john.doe@example.com"]
    telephoneNumber = ["+1-555-123-4567"]
    employeeNumber = ["12345"]
    departmentNumber = ["Engineering"]
    title = ["Software Engineer"]
  }
}
`, dn)
}

func testAccLdapEntryResourceConfigGroup(dn string) string {
	return fmt.Sprintf(`
provider "ldap" {
  url = "ldap://localhost:3389"
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

resource "ldap_entry" "test_group" {
  dn = %[1]q
  attributes = {
    objectClass = ["top", "groupOfNames"]
    cn = ["developers"]
    description = ["Development team group"]
    member = ["cn=john.doe,ou=users,dc=example,dc=com"]
  }
}
`, dn)
}

func testAccLdapEntryResourceConfigMinimal(dn string) string {
	return fmt.Sprintf(`
provider "ldap" {
  url = "ldap://localhost:3389"
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

resource "ldap_entry" "minimal" {
  dn = %[1]q
  attributes = {
    objectClass = ["person"]
    cn = ["minimal"]
    sn = ["User"]
  }
}
`, dn)
}
