// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccLdapEntryResource_InvalidDN(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccLdapEntryResourceConfigInvalidDN(),
				ExpectError: regexp.MustCompile("Invalid DN|LDAP Result Code"),
			},
		},
	})
}

func TestAccLdapEntryResource_MissingRequiredAttribute(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLdapEntryResourceConfigMissingRequired(),
				// output may contain newlines, hence `\s+` for whitespace
				ExpectError: regexp.MustCompile(`object\s+class\s+'person'\s+requires\s+attribute\s+'sn'`),
			},
		},
	})
}

func TestAccLdapEntryResource_DuplicateEntry(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// First create an entry
			{
				Config: testAccLdapEntryResourceConfigDuplicate("cn=duplicate,ou=users,dc=example,dc=com"),
			},
			// Try to create the same entry again - should fail
			{
				Config:      testAccLdapEntryResourceConfigDuplicateConflict("cn=duplicate,ou=users,dc=example,dc=com"),
				ExpectError: regexp.MustCompile("already exists|Entry Already Exists"),
			},
		},
	})
}

func testAccLdapEntryResourceConfigInvalidDN() string {
	return `
provider "ldap" {
  host = "localhost"
  port = 3389
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

resource "ldap_entry" "invalid" {
  dn = "invalid-dn-format"
  attributes = {
    objectClass = ["person"]
    cn = ["test"]
    sn = ["user"]
  }
}
`
}

func testAccLdapEntryResourceConfigMissingRequired() string {
	return `
provider "ldap" {
  host = "localhost"
  port = 3389
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

resource "ldap_entry" "missing_required" {
  dn = "cn=missing,ou=users,dc=example,dc=com"
  attributes = {
    objectClass = ["person"]
    cn = ["missing"]
    # Missing required 'sn' attribute for person objectClass
  }
}
`
}

func testAccLdapEntryResourceConfigDuplicate(dn string) string {
	return fmt.Sprintf(`
provider "ldap" {
  host = "localhost"
  port = 3389
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

resource "ldap_entry" "original" {
  dn = %[1]q
  attributes = {
    objectClass = ["person"]
    cn = ["duplicate"]
    sn = ["Original"]
  }
}
`, dn)
}

func testAccLdapEntryResourceConfigDuplicateConflict(dn string) string {
	return fmt.Sprintf(`
provider "ldap" {
  host = "localhost"
  port = 3389
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

resource "ldap_entry" "original" {
  dn = %[1]q
  attributes = {
    objectClass = ["person"]
    cn = ["duplicate"]
    sn = ["Original"]
  }
}

resource "ldap_entry" "conflict" {
  dn = %[1]q
  attributes = {
    objectClass = ["person"]
    cn = ["duplicate"]
    sn = ["Conflict"]
  }
}
`, dn)
}
