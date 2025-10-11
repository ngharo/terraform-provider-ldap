// Copyright (c) ngharo <root@ngha.ro>
// SPDX-License-Identifier: MPL-2.0

package provider

import (
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
  host = "localhost"
  port = 3389
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
