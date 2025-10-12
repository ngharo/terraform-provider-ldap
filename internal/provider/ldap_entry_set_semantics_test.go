// Copyright (c) ngharo <root@ngha.ro>
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccLdapEntryResource_SetSemantics(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create entry with multi-valued attributes in specific order
			{
				Config: testAccLdapEntryResourceConfigSetSemanticsOriginal(),
			},
			// Apply same config with attributes in different order - should show no changes
			{
				Config:             testAccLdapEntryResourceConfigSetSemanticsDifferentOrder(),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

func testAccLdapEntryResourceConfigSetSemanticsOriginal() string {
	return `
provider "ldap" {
  url = "ldap://localhost:3389"
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

resource "ldap_entry" "test_semantics" {
  dn = "cn=semantics-test,ou=users,dc=example,dc=com"
  attributes = {
    objectClass = ["person", "organizationalPerson", "inetOrgPerson"]
    cn = ["semantics-test"]
    sn = ["Test"]
  }
}
`
}

func testAccLdapEntryResourceConfigSetSemanticsDifferentOrder() string {
	return `
provider "ldap" {
  url = "ldap://localhost:3389"
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

resource "ldap_entry" "test_semantics" {
  dn = "cn=semantics-test,ou=users,dc=example,dc=com"
  attributes = {
    # Same objectClass values but in different order
    objectClass = ["inetOrgPerson", "person", "organizationalPerson"]
    cn = ["semantics-test"]
    sn = ["Test"]
  }
}
`
}

func TestAccLdapEntryResource_SetSemanticsMultipleAttributes(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create group with multiple members in specific order
			{
				Config: testAccLdapEntryResourceConfigGroupMembersOriginal(),
			},
			// Apply same config with members in different order - should show no changes
			{
				Config:             testAccLdapEntryResourceConfigGroupMembersDifferentOrder(),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

func testAccLdapEntryResourceConfigGroupMembersOriginal() string {
	return `
provider "ldap" {
  url = "ldap://localhost:3389"
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

resource "ldap_entry" "test_group" {
  dn = "cn=test-group,ou=groups,dc=example,dc=com"
  attributes = {
    objectClass = ["top", "groupOfNames"]
    cn = ["test-group"]
    member = [
      "cn=user1,ou=users,dc=example,dc=com",
      "cn=user2,ou=users,dc=example,dc=com",
      "cn=user3,ou=users,dc=example,dc=com"
    ]
  }
}
`
}

func testAccLdapEntryResourceConfigGroupMembersDifferentOrder() string {
	return `
provider "ldap" {
  url = "ldap://localhost:3389"
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

resource "ldap_entry" "test_group" {
  dn = "cn=test-group,ou=groups,dc=example,dc=com"
  attributes = {
    objectClass = ["top", "groupOfNames"]
    cn = ["test-group"]
    # Same members but in different order
    member = [
      "cn=user3,ou=users,dc=example,dc=com",
      "cn=user1,ou=users,dc=example,dc=com",
      "cn=user2,ou=users,dc=example,dc=com"
    ]
  }
}
`
}
