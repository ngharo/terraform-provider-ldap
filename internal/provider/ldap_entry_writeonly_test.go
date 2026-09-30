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
				ExpectError: regexp.MustCompile(`Attribute "attributes_wo_version" must be specified when "attributes_wo"`),
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
				ExpectError: regexp.MustCompile(`Attribute "attributes_wo" must be specified when "attributes_wo_version"`),
			},
		},
	})
}

// TestAccLdapEntryResource_WriteOnlyVersionUnknownAtPlan is a regression test
// for the bug where validation rejected a known, non-empty attributes_wo
// combined with an attributes_wo_version whose value was unknown at validation
// time. This is what the provider sees when ldap_entry is wrapped in a module
// and attributes_wo_version is passed in through an input variable: Terraform
// validates the child module's resource configuration before the variable has
// been evaluated, delivering an unknown value. The same unknown value can be
// produced at the root module by deriving attributes_wo_version from a
// resource that does not exist yet.
//
// The coupling between attributes_wo and attributes_wo_version is enforced with
// the framework's AlsoRequires validators, which delay validation until all
// involved attributes have known values. The plan and apply should therefore
// succeed.
func TestAccLdapEntryResource_WriteOnlyVersionUnknownAtPlan(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLdapEntryDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccLdapEntryResourceConfigWriteOnlyUnknownVersion(
					"cn=dep-unknown-version,dc=example,dc=com",
					"cn=writeonly-unknown-version,dc=example,dc=com",
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"ldap_entry.test_writeonly",
						tfjsonpath.New("dn"),
						knownvalue.StringExact("cn=writeonly-unknown-version,dc=example,dc=com"),
					),
					// The version is derived from the dependency entry's id (its DN),
					// which contains two commas: length(split(...)) is therefore 3.
					statecheck.ExpectKnownValue(
						"ldap_entry.test_writeonly",
						tfjsonpath.New("attributes_wo_version"),
						knownvalue.Int64Exact(3),
					),
					stateCheckLdapEntryAttributeExists("ldap_entry.test_writeonly", "userPassword"),
				},
			},
		},
	})
}

// testAccLdapEntryResourceConfigWriteOnlyUnknownVersion derives
// attributes_wo_version from the id of an ldap_entry created in the same apply,
// so the version is unknown during the initial plan. This mirrors the unknown
// value the provider receives during validation when the version is sourced
// from a module input variable.
func testAccLdapEntryResourceConfigWriteOnlyUnknownVersion(depDn, dn string) string {
	return fmt.Sprintf(`
provider "ldap" {
  url = %q
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

resource "ldap_entry" "dependency" {
  dn = %q
  attributes = {
    objectClass = ["person", "organizationalPerson", "inetOrgPerson"]
    cn          = ["dep-unknown-version"]
    sn          = ["User"]
  }
}

resource "ldap_entry" "test_writeonly" {
  dn = %q
  attributes = {
    objectClass = ["person", "organizationalPerson", "inetOrgPerson"]
    cn          = ["writeonly-unknown-version"]
    sn          = ["User"]
  }
  attributes_wo = {
    userPassword = ["secret123"]
  }
  # Root input variables supplied by the test harness are already known during
  # planning, so they do not reproduce the bug. Referencing an uncreated resource
  # gives the provider the same unknown version it receives when validating a
  # child module before its input variables have been evaluated.
  attributes_wo_version = length(split(",", ldap_entry.dependency.id))
}
`, testAccLdapURL, depDn, dn)
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
