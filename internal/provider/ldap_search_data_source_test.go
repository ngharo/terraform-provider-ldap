// Copyright (c) ngharo <root@ngha.ro>
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccLdapSearchDataSource_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLdapSearchDataSourceConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					// Verify the search finds the base entry (base DN exists in container)
					statecheck.ExpectKnownValue(
						"data.ldap_search.base_search",
						tfjsonpath.New("results").AtSliceIndex(0).AtMapKey("dn"),
						knownvalue.StringExact("dc=example,dc=com"),
					),
				},
			},
		},
	})
}

func testAccLdapSearchDataSourceConfig() string {
	return `
provider "ldap" {
  url = "ldap://localhost:3389"
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

# Search for the base entry (base DN already exists in container)
data "ldap_search" "base_search" {
  basedn = "dc=example,dc=com"
  scope = "base"
  filter = "(objectClass=*)"
}
`
}

// TestAccLdapSearchDataSource_InvalidScope verifies that an invalid scope is
// rejected at plan time by the schema validator instead of failing deep inside
// the LDAP request.
func TestAccLdapSearchDataSource_InvalidScope(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccLdapSearchDataSourceConfigInvalidScope(),
				ExpectError: regexp.MustCompile(`Invalid Attribute Value Match`),
			},
		},
	})
}

func testAccLdapSearchDataSourceConfigInvalidScope() string {
	return `
provider "ldap" {
  url = "ldap://localhost:3389"
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

data "ldap_search" "invalid_scope" {
  basedn = "dc=example,dc=com"
  scope = "everything"
  filter = "(objectClass=*)"
}
`
}
