// Copyright (c) ngharo <root@ngha.ro>
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/go-ldap/ldap/v3"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccLdapEntryResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLdapEntryDestroy,
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
				},
			},
			// ImportState testing
			{
				ResourceName:            "ldap_entry.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"attributes"},
			},
			// Update and Read testing - add attributes
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
					statecheck.ExpectKnownValue(
						"ldap_entry.test",
						tfjsonpath.New("attributes"),
						knownvalue.MapSizeExact(5),
					),
				},
			},
			// Update and Read testing - remove attributes
			{
				Config: testAccLdapEntryResourceConfigAttributeRemoval("cn=test,dc=example,dc=com"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"ldap_entry.test",
						tfjsonpath.New("dn"),
						knownvalue.StringExact("cn=test,dc=example,dc=com"),
					),
					// Ensure description is no longer present in the state
					statecheck.ExpectKnownValue(
						"ldap_entry.test",
						tfjsonpath.New("attributes"),
						knownvalue.MapSizeExact(4),
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
  url = "ldap://localhost:3389"
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

resource "ldap_entry" "test" {
  dn = %[1]q
  attributes = {
    objectClass = ["person", "organizationalPerson", "inetOrgPerson"]
    cn = ["test"]
    sn = ["user"]
    mail = ["test@example.com"]
  }
}
`, dn)
}

func testAccLdapEntryResourceConfigUpdated(dn string) string {
	return fmt.Sprintf(`
provider "ldap" {
  url = "ldap://localhost:3389"
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

resource "ldap_entry" "test" {
  dn = %[1]q
  attributes = {
    objectClass = ["person", "organizationalPerson", "inetOrgPerson"]
    cn = ["test"]
    sn = ["user"]
    mail = ["test.updated@example.com"]
    description = ["Updated user"]
  }
}
`, dn)
}

func testAccLdapEntryResourceConfigAttributeRemoval(dn string) string {
	return fmt.Sprintf(`
provider "ldap" {
  url = "ldap://localhost:3389"
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

resource "ldap_entry" "test" {
  dn = %[1]q
  attributes = {
    objectClass = ["person", "organizationalPerson", "inetOrgPerson"]
    cn = ["test"]
    sn = ["user"]
    mail = ["test.updated@example.com"]
    # description attribute removed
  }
}
`, dn)
}

func TestAccLdapEntryResource_EmptyAttributes(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLdapEntryDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create a user with empty mail
			{
				Config: testAccLdapEntryResourceConfigEmptyAttribute(``),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"ldap_entry.test_user",
						tfjsonpath.New("dn"),
						knownvalue.StringExact("uid=testuser,dc=example,dc=com"),
					),
					statecheck.ExpectKnownValue(
						"ldap_entry.test_user",
						tfjsonpath.New("attributes").AtMapKey("mail"),
						knownvalue.ListSizeExact(0),
					),
				},
			},
			// Step 2: Externally add mail attribute, then apply to bring it back to desired state
			{
				PreConfig: func() {
					// Simulate external modification by adding mail attribute directly to LDAP
					conn, err := ldap.DialURL("ldap://localhost:3389")
					if err != nil {
						t.Fatalf("failed to connect to LDAP server: %v", err)
					}
					defer conn.Close()

					err = conn.Bind("cn=Manager,dc=example,dc=com", "secret")
					if err != nil {
						t.Fatalf("failed to bind to LDAP server: %v", err)
					}

					modifyReq := ldap.NewModifyRequest("uid=testuser,dc=example,dc=com", nil)
					modifyReq.Add("mail", []string{"external@example.com"})
					err = conn.Modify(modifyReq)
					if err != nil {
						t.Fatalf("failed to add mail attribute: %v", err)
					}
				},
				Config: testAccLdapEntryResourceConfigEmptyAttribute(``),
				ConfigStateChecks: []statecheck.StateCheck{
					// Verify that after apply, mail is back to empty (desired state enforced)
					statecheck.ExpectKnownValue(
						"ldap_entry.test_user",
						tfjsonpath.New("attributes").AtMapKey("mail"),
						knownvalue.ListSizeExact(0),
					),
				},
			},
		},
	})
}
func TestAccLdapEntryResource_EmptyAttributesTransition(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLdapEntryDestroy,
		Steps: []resource.TestStep{
			// Step 1: Create a user with two mails
			{
				Config: testAccLdapEntryResourceConfigEmptyAttribute(`"foo@example.com", "bar@example.com"`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"ldap_entry.test_user",
						tfjsonpath.New("dn"),
						knownvalue.StringExact("uid=testuser,dc=example,dc=com"),
					),
					statecheck.ExpectKnownValue(
						"ldap_entry.test_user",
						tfjsonpath.New("attributes").AtMapKey("mail"),
						knownvalue.ListSizeExact(2),
					),
				},
			},
			// Step 2: Update user to one mail
			{
				Config: testAccLdapEntryResourceConfigEmptyAttribute(`"foo@example.com"`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"ldap_entry.test_user",
						tfjsonpath.New("dn"),
						knownvalue.StringExact("uid=testuser,dc=example,dc=com"),
					),
					statecheck.ExpectKnownValue(
						"ldap_entry.test_user",
						tfjsonpath.New("attributes").AtMapKey("mail"),
						knownvalue.ListSizeExact(1),
					),
				},
			},
			// Step 3: Update user to no mail
			{
				Config: testAccLdapEntryResourceConfigEmptyAttribute(""),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"ldap_entry.test_user",
						tfjsonpath.New("dn"),
						knownvalue.StringExact("uid=testuser,dc=example,dc=com"),
					),
					statecheck.ExpectKnownValue(
						"ldap_entry.test_user",
						tfjsonpath.New("attributes").AtMapKey("mail"),
						knownvalue.ListSizeExact(0),
					),
				},
			},
		},
	})
}

func testAccLdapEntryResourceConfigEmptyAttribute(mails string) string {
	return fmt.Sprintf(`
provider "ldap" {
  url = "ldap://localhost:3389"
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

resource "ldap_entry" "test_user" {
  dn = "uid=testuser,dc=example,dc=com"
  attributes = {
    objectClass = ["person", "organizationalPerson", "inetOrgPerson"]
    cn = ["Test User"]
    sn = ["User"]
    uid = ["testuser"]
    mail = [%s]
  }
}
`, mails)
}

func testAccCheckLdapEntryDestroy(s *terraform.State) error {
	// Create LDAP connection to verify entries are destroyed
	conn, err := ldap.DialURL("ldap://localhost:3389")
	if err != nil {
		return fmt.Errorf("failed to connect to LDAP server: %w", err)
	}
	defer conn.Close()

	// Bind to LDAP server
	err = conn.Bind("cn=Manager,dc=example,dc=com", "secret")
	if err != nil {
		return fmt.Errorf("failed to bind to LDAP server: %w", err)
	}

	// Check each resource in the state
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "ldap_entry" {
			continue
		}

		dn := rs.Primary.ID

		// Search for the entry
		searchReq := ldap.NewSearchRequest(
			dn,
			ldap.ScopeBaseObject,
			ldap.NeverDerefAliases,
			0,
			0,
			false,
			"(objectClass=*)",
			[]string{"dn"},
			nil,
		)

		result, err := conn.Search(searchReq)
		if err != nil {
			// If we get an LDAP error indicating the entry doesn't exist, that's expected
			if ldapErr, ok := err.(*ldap.Error); ok {
				if ldapErr.ResultCode == ldap.LDAPResultNoSuchObject {
					// Entry successfully destroyed
					continue
				}
			}
			return fmt.Errorf("error searching for entry %s: %w", dn, err)
		}

		// If we found entries, the destroy failed
		if len(result.Entries) > 0 {
			return fmt.Errorf("LDAP entry %s still exists on server after destroy", dn)
		}
	}

	return nil
}
