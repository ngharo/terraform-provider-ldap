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

func TestAccProvider_DefaultConfiguration(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfigDefault(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"ldap_entry.test",
						tfjsonpath.New("dn"),
						knownvalue.StringExact("cn=default-test,ou=users,dc=example,dc=com"),
					),
				},
			},
		},
	})
}

func testAccProviderConfigDefault() string {
	return `
provider "ldap" {
  # Using default URL
  url = "ldap://localhost:3389"
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

resource "ldap_entry" "test" {
  dn = "cn=default-test,ou=users,dc=example,dc=com"
  attributes = {
    objectClass = ["person"]
    cn = ["default-test"]
    sn = ["Test"]
  }
}
`
}
