// Copyright (c) HashiCorp, Inc.
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

func TestAccLdapEntryResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
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
					statecheck.ExpectKnownValue(
						"ldap_entry.test",
						tfjsonpath.New("object_class"),
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.StringExact("person"),
							knownvalue.StringExact("organizationalPerson"),
							knownvalue.StringExact("inetOrgPerson"),
						}),
					),
				},
			},
			// ImportState testing
			{
				ResourceName:            "ldap_entry.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"object_class", "attributes"},
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
  object_class = ["person", "organizationalPerson", "inetOrgPerson"]
  attributes = {
    cn = "test"
    sn = "user"
    mail = "test@example.com"
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
  object_class = ["person", "organizationalPerson", "inetOrgPerson"]
  attributes = {
    cn = "test"
    sn = "user"
    mail = "test.updated@example.com"
    description = "Updated user"
  }
}
`, dn)
}
