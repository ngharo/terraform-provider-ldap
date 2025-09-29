// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccLdapIntegration_EntryAndSearch(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLdapIntegrationConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					// Verify the entry was created
					statecheck.ExpectKnownValue(
						"ldap_entry.user",
						tfjsonpath.New("dn"),
						knownvalue.StringExact("cn=testuser,ou=users,dc=example,dc=com"),
					),
					statecheck.ExpectKnownValue(
						"ldap_entry.group",
						tfjsonpath.New("dn"),
						knownvalue.StringExact("cn=testgroup,ou=groups,dc=example,dc=com"),
					),
					// Verify the search found the created entries
					statecheck.ExpectKnownValue(
						"data.ldap_search.users",
						tfjsonpath.New("results").AtSliceIndex(0).AtMapKey("dn"),
						knownvalue.StringExact("cn=testuser,ou=users,dc=example,dc=com"),
					),
					statecheck.ExpectKnownValue(
						"data.ldap_search.users",
						tfjsonpath.New("results").AtSliceIndex(0).AtMapKey("attributes").AtMapKey("cn").AtSliceIndex(0),
						knownvalue.StringExact("testuser"),
					),
					statecheck.ExpectKnownValue(
						"data.ldap_search.users",
						tfjsonpath.New("results").AtSliceIndex(0).AtMapKey("attributes").AtMapKey("mail").AtSliceIndex(0),
						knownvalue.StringExact("testuser@example.com"),
					),
					// Verify group search
					statecheck.ExpectKnownValue(
						"data.ldap_search.groups",
						tfjsonpath.New("results").AtSliceIndex(0).AtMapKey("dn"),
						knownvalue.StringExact("cn=testgroup,ou=groups,dc=example,dc=com"),
					),
					// Verify member search finds the user
					statecheck.ExpectKnownValue(
						"data.ldap_search.group_members",
						tfjsonpath.New("results").AtSliceIndex(0).AtMapKey("attributes").AtMapKey("member").AtSliceIndex(0),
						knownvalue.StringExact("cn=testuser,ou=users,dc=example,dc=com"),
					),
				},
			},
		},
	})
}

func TestAccLdapIntegration_SearchScopes(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLdapIntegrationSearchScopesConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					// Verify base scope search finds only the base entry
					statecheck.ExpectKnownValue(
						"data.ldap_search.base_search",
						tfjsonpath.New("results").AtSliceIndex(0).AtMapKey("dn"),
						knownvalue.StringExact("dc=example,dc=com"),
					),
					// Verify one level search finds organizational units
					statecheck.ExpectKnownValue(
						"data.ldap_search.one_level_search",
						tfjsonpath.New("results"),
						knownvalue.ListSizeExact(2), // Should find 'users' and 'groups' OUs
					),
					// Verify subtree search finds the created user
					statecheck.ExpectKnownValue(
						"data.ldap_search.subtree_search",
						tfjsonpath.New("results").AtSliceIndex(0).AtMapKey("dn"),
						knownvalue.StringExact("cn=scopeuser,ou=users,dc=example,dc=com"),
					),
				},
			},
		},
	})
}

func TestAccLdapIntegration_ComplexFilters(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLdapIntegrationComplexFiltersConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					// Verify AND filter finds users with email
					statecheck.ExpectKnownValue(
						"data.ldap_search.users_with_email",
						tfjsonpath.New("results").AtSliceIndex(0).AtMapKey("attributes").AtMapKey("mail").AtSliceIndex(0),
						knownvalue.StringExact("filteruser@example.com"),
					),
					// Verify OR filter finds multiple users
					statecheck.ExpectKnownValue(
						"data.ldap_search.users_or_groups",
						tfjsonpath.New("results"),
						knownvalue.ListSizeExact(2), // Should find both user and group
					),
					// Verify NOT filter excludes certain entries
					statecheck.ExpectKnownValue(
						"data.ldap_search.not_groups",
						tfjsonpath.New("results").AtSliceIndex(0).AtMapKey("attributes").AtMapKey("objectClass").AtSliceIndex(0),
						knownvalue.StringExact("person"),
					),
				},
			},
		},
	})
}

func testAccLdapIntegrationConfig() string {
	return `
provider "ldap" {
  host = "localhost"
  port = 3389
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

# Create the base DN first
resource "ldap_entry" "base" {
  dn = "dc=example,dc=com"
  object_class = ["top", "dcObject", "organization"]
  attributes = {
    o = "Example Organization"
    dc = "example"
  }
}

# Create organizational units - implicit dependency on base
resource "ldap_entry" "users_ou" {
  dn = "ou=users,${ldap_entry.base.dn}"
  object_class = ["top", "organizationalUnit"]
  attributes = {
    ou = "users"
  }
}

resource "ldap_entry" "groups_ou" {
  dn = "ou=groups,${ldap_entry.base.dn}"
  object_class = ["top", "organizationalUnit"]
  attributes = {
    ou = "groups"
  }
}

# Create a user entry - implicit dependency on users_ou
resource "ldap_entry" "user" {
  dn = "cn=testuser,${ldap_entry.users_ou.dn}"
  object_class = ["person", "organizationalPerson", "inetOrgPerson"]
  attributes = {
    cn = "testuser"
    sn = "Test"
    givenName = "User"
    mail = "testuser@example.com"
    description = "Test user for integration testing"
  }
}

# Create a group entry - implicit dependency on groups_ou and user
resource "ldap_entry" "group" {
  dn = "cn=testgroup,${ldap_entry.groups_ou.dn}"
  object_class = ["top", "groupOfNames"]
  attributes = {
    cn = "testgroup"
    description = "Test group for integration testing"
    member = ldap_entry.user.dn
  }
}

# Search for users - implicit dependency on user
data "ldap_search" "users" {
  basedn = ldap_entry.users_ou.dn
  filter = "(objectClass=person)"
  requested_attributes = ["cn", "sn", "givenName", "mail", "description"]
}

# Search for groups - implicit dependency on group
data "ldap_search" "groups" {
  basedn = ldap_entry.groups_ou.dn
  filter = "(objectClass=groupOfNames)"
  requested_attributes = ["cn", "description"]
}

# Search for group members - implicit dependency on group
data "ldap_search" "group_members" {
  basedn = ldap_entry.groups_ou.dn
  filter = "(member=${ldap_entry.user.dn})"
  requested_attributes = ["cn", "member"]
}
`
}

func testAccLdapIntegrationSearchScopesConfig() string {
	return `
provider "ldap" {
  host = "localhost"
  port = 3389
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

# Create the base DN first
resource "ldap_entry" "base" {
  dn = "dc=example,dc=com"
  object_class = ["top", "dcObject", "organization"]
  attributes = {
    o = "Example Organization"
    dc = "example"
  }
}

# Create organizational units
resource "ldap_entry" "users_ou" {
  dn = "ou=users,${ldap_entry.base.dn}"
  object_class = ["top", "organizationalUnit"]
  attributes = {
    ou = "users"
  }
}

resource "ldap_entry" "groups_ou" {
  dn = "ou=groups,${ldap_entry.base.dn}"
  object_class = ["top", "organizationalUnit"]
  attributes = {
    ou = "groups"
  }
}

# Create a user to find with subtree search
resource "ldap_entry" "user" {
  dn = "cn=scopeuser,${ldap_entry.users_ou.dn}"
  object_class = ["person", "organizationalPerson", "inetOrgPerson"]
  attributes = {
    cn = "scopeuser"
    sn = "Scope"
    givenName = "User"
  }
}

# Base scope - search only the base DN
data "ldap_search" "base_search" {
  basedn = ldap_entry.base.dn
  scope = "base"
  filter = "(objectClass=*)"
}

# One level scope - search immediate children only
data "ldap_search" "one_level_search" {
  basedn = ldap_entry.base.dn
  scope = "one"
  filter = "(objectClass=organizationalUnit)"
}

# Subtree scope - search entire subtree
data "ldap_search" "subtree_search" {
  basedn = ldap_entry.base.dn
  scope = "sub"
  filter = "(cn=scopeuser)"
}
`
}

func testAccLdapIntegrationComplexFiltersConfig() string {
	return `
provider "ldap" {
  host = "localhost"
  port = 3389
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

# Create the base DN first
resource "ldap_entry" "base" {
  dn = "dc=example,dc=com"
  object_class = ["top", "dcObject", "organization"]
  attributes = {
    o = "Example Organization"
    dc = "example"
  }
}

# Create organizational units
resource "ldap_entry" "users_ou" {
  dn = "ou=users,${ldap_entry.base.dn}"
  object_class = ["top", "organizationalUnit"]
  attributes = {
    ou = "users"
  }
}

resource "ldap_entry" "groups_ou" {
  dn = "ou=groups,${ldap_entry.base.dn}"
  object_class = ["top", "organizationalUnit"]
  attributes = {
    ou = "groups"
  }
}

# Create test entries
resource "ldap_entry" "user_with_email" {
  dn = "cn=filteruser,${ldap_entry.users_ou.dn}"
  object_class = ["person", "organizationalPerson", "inetOrgPerson"]
  attributes = {
    cn = "filteruser"
    sn = "Filter"
    givenName = "User"
    mail = "filteruser@example.com"
  }
}

resource "ldap_entry" "group" {
  dn = "cn=filtergroup,${ldap_entry.groups_ou.dn}"
  object_class = ["top", "groupOfNames"]
  attributes = {
    cn = "filtergroup"
    description = "Filter test group"
    member = ldap_entry.user_with_email.dn
  }
}

# AND filter - users with email addresses
data "ldap_search" "users_with_email" {
  basedn = ldap_entry.base.dn
  filter = "(&(objectClass=person)(mail=*))"
  requested_attributes = ["cn", "mail"]
}

# OR filter - users or groups
data "ldap_search" "users_or_groups" {
  basedn = ldap_entry.base.dn
  filter = "(|(objectClass=person)(objectClass=groupOfNames))"
  requested_attributes = ["cn", "objectClass"]
}

# NOT filter - entries that are not groups
data "ldap_search" "not_groups" {
  basedn = ldap_entry.base.dn
  filter = "(&(objectClass=person)(!(objectClass=groupOfNames)))"
  requested_attributes = ["cn", "objectClass"]
}
`
}