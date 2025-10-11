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
			// Step 1: Create entries
			{
				Config: testAccLdapIntegrationEntryConfig(),
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
				},
			},
			// Step 2: Search for the created entries
			{
				Config: testAccLdapIntegrationConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
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
			// Step 1: Create entry and test base/one level searches (which don't depend on created entry)
			{
				Config: testAccLdapIntegrationSearchScopesEntryConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"ldap_entry.user",
						tfjsonpath.New("dn"),
						knownvalue.StringExact("cn=scopeuser,ou=users,dc=example,dc=com"),
					),
				},
			},
			// Step 2: Test all search scopes including subtree
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
			// Step 1: Create entries
			{
				Config: testAccLdapIntegrationComplexFiltersEntryConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"ldap_entry.user_with_email",
						tfjsonpath.New("dn"),
						knownvalue.StringExact("cn=filteruser,ou=users,dc=example,dc=com"),
					),
					statecheck.ExpectKnownValue(
						"ldap_entry.group",
						tfjsonpath.New("dn"),
						knownvalue.StringExact("cn=filtergroup,ou=groups,dc=example,dc=com"),
					),
				},
			},
			// Step 2: Test complex filters
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

func testAccLdapIntegrationEntryConfig() string {
	return `
provider "ldap" {
  host = "localhost"
  port = 3389
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

# Create a user entry (base DN and OUs already exist in container)
resource "ldap_entry" "user" {
  dn = "cn=testuser,ou=users,dc=example,dc=com"
  attributes = {
    objectClass = ["person", "organizationalPerson", "inetOrgPerson"]
    cn = ["testuser"]
    sn = ["Test"]
    givenName = ["User"]
    mail = ["testuser@example.com"]
    description = ["Test user for integration testing"]
  }
}

# Create a group entry
resource "ldap_entry" "group" {
  dn = "cn=testgroup,ou=groups,dc=example,dc=com"
  attributes = {
    objectClass = ["top", "groupOfNames"]
    cn = ["testgroup"]
    description = ["Test group for integration testing"]
    member = [ldap_entry.user.dn]
  }
}
`
}

func testAccLdapIntegrationConfig() string {
	return `
provider "ldap" {
  host = "localhost"
  port = 3389
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

# Search for users
data "ldap_search" "users" {
  basedn = "ou=users,dc=example,dc=com"
  filter = "(objectClass=person)"
  requested_attributes = ["cn", "sn", "givenName", "mail", "description"]
}

# Search for groups
data "ldap_search" "groups" {
  basedn = "ou=groups,dc=example,dc=com"
  filter = "(objectClass=groupOfNames)"
  requested_attributes = ["cn", "description"]
}

# Search for group members
data "ldap_search" "group_members" {
  basedn = "ou=groups,dc=example,dc=com"
  filter = "(member=cn=testuser,ou=users,dc=example,dc=com)"
  requested_attributes = ["cn", "member"]
}
`
}

func testAccLdapIntegrationSearchScopesEntryConfig() string {
	return `
provider "ldap" {
  host = "localhost"
  port = 3389
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

# Create a user to find with subtree search (base DN and OUs already exist in container)
resource "ldap_entry" "user" {
  dn = "cn=scopeuser,ou=users,dc=example,dc=com"
  attributes = {
    objectClass = ["person", "organizationalPerson", "inetOrgPerson"]
    cn = ["scopeuser"]
    sn = ["Scope"]
    givenName = ["User"]
  }
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

# Base scope - search only the base DN
data "ldap_search" "base_search" {
  basedn = "dc=example,dc=com"
  scope = "base"
  filter = "(objectClass=*)"
}

# One level scope - search immediate children only
data "ldap_search" "one_level_search" {
  basedn = "dc=example,dc=com"
  scope = "one"
  filter = "(objectClass=organizationalUnit)"
}

# Subtree scope - search entire subtree
data "ldap_search" "subtree_search" {
  basedn = "dc=example,dc=com"
  scope = "sub"
  filter = "(cn=scopeuser)"
}
`
}

func testAccLdapIntegrationComplexFiltersEntryConfig() string {
	return `
provider "ldap" {
  host = "localhost"
  port = 3389
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

# Create test entries (base DN and OUs already exist in container)
resource "ldap_entry" "user_with_email" {
  dn = "cn=filteruser,ou=users,dc=example,dc=com"
  attributes = {
    objectClass = ["person", "organizationalPerson", "inetOrgPerson"]
    cn = ["filteruser"]
    sn = ["Filter"]
    givenName = ["User"]
    mail = ["filteruser@example.com"]
  }
}

resource "ldap_entry" "group" {
  dn = "cn=filtergroup,ou=groups,dc=example,dc=com"
  attributes = {
    objectClass = ["top", "groupOfNames"]
    cn = ["filtergroup"]
    description = ["Filter test group"]
    member = [ldap_entry.user_with_email.dn]
  }
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

# AND filter - users with email addresses
data "ldap_search" "users_with_email" {
  basedn = "dc=example,dc=com"
  filter = "(&(objectClass=person)(mail=*))"
  requested_attributes = ["cn", "mail"]
}

# OR filter - users or groups
data "ldap_search" "users_or_groups" {
  basedn = "dc=example,dc=com"
  filter = "(|(objectClass=person)(objectClass=groupOfNames))"
  requested_attributes = ["cn", "objectClass"]
}

# NOT filter - entries that are not groups
data "ldap_search" "not_groups" {
  basedn = "dc=example,dc=com"
  filter = "(&(objectClass=person)(!(objectClass=groupOfNames)))"
  requested_attributes = ["cn", "objectClass"]
}
`
}
