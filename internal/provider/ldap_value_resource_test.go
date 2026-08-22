// Copyright (c) ngharo <root@ngha.ro>
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/go-ldap/ldap/v3"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// testAccCreateLdapValueTestGroup creates a groupOfNames entry directly via
// LDAP (outside of Terraform), seeded with a permanent member so the schema's
// "member" MUST constraint is satisfied without any ldap_entry resource
// managing the same attribute that ldap_value asserts against. It registers
// a cleanup to remove the group once the test finishes.
func testAccCreateLdapValueTestGroup(t *testing.T, dn, seedMemberDN string) {
	t.Helper()

	conn, err := ldap.DialURL("ldap://localhost:3389")
	if err != nil {
		t.Fatalf("failed to connect to LDAP server: %v", err)
	}
	defer conn.Close()

	if err := conn.Bind("cn=Manager,dc=example,dc=com", "secret"); err != nil {
		t.Fatalf("failed to bind to LDAP server: %v", err)
	}

	addReq := ldap.NewAddRequest(dn, nil)
	addReq.Attribute("objectClass", []string{"top", "groupOfNames"})
	addReq.Attribute("member", []string{seedMemberDN})
	if err := conn.Add(addReq); err != nil {
		t.Fatalf("failed to create test group %s: %v", dn, err)
	}

	t.Cleanup(func() {
		conn, err := ldap.DialURL("ldap://localhost:3389")
		if err != nil {
			t.Logf("cleanup: failed to connect to LDAP server: %v", err)
			return
		}
		defer conn.Close()

		if err := conn.Bind("cn=Manager,dc=example,dc=com", "secret"); err != nil {
			t.Logf("cleanup: failed to bind to LDAP server: %v", err)
			return
		}

		if err := conn.Del(ldap.NewDelRequest(dn, nil)); err != nil {
			t.Logf("cleanup: failed to delete test group %s: %v", dn, err)
		}
	})
}

func TestAccLdapValueResource(t *testing.T) {
	groupDN := "cn=valuetest,ou=groups,dc=example,dc=com"
	seedMemberDN := "cn=seed,dc=example,dc=com"
	memberDN := "cn=Manager,dc=example,dc=com"

	testAccCreateLdapValueTestGroup(t, groupDN, seedMemberDN)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLdapValueDestroy,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccLdapValueResourceConfig(groupDN, memberDN),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"ldap_value.test",
						tfjsonpath.New("dn"),
						knownvalue.StringExact(groupDN),
					),
					statecheck.ExpectKnownValue(
						"ldap_value.test",
						tfjsonpath.New("attribute"),
						knownvalue.StringExact("member"),
					),
					statecheck.ExpectKnownValue(
						"ldap_value.test",
						tfjsonpath.New("value"),
						knownvalue.StringExact(memberDN),
					),
				},
				Check: testAccCheckLdapValuePresent(groupDN, "member", memberDN),
			},
			// ImportState testing
			{
				ResourceName: "ldap_value.test",
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return fmt.Sprintf(`{"dn": %q, "attribute": "member", "value": %q}`, groupDN, memberDN), nil
				},
				ImportStateVerify: true,
			},
		},
	})
}

func testAccLdapValueResourceConfig(groupDN, memberDN string) string {
	return fmt.Sprintf(`
provider "ldap" {
  url = "ldap://localhost:3389"
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
}

resource "ldap_value" "test" {
  dn        = %[1]q
  attribute = "member"
  value     = %[2]q
}
`, groupDN, memberDN)
}

func TestAccLdapValueResource_DriftDetection(t *testing.T) {
	groupDN := "cn=valuedrift,ou=groups,dc=example,dc=com"
	seedMemberDN := "cn=seed,dc=example,dc=com"
	memberDN := "cn=Manager,dc=example,dc=com"

	testAccCreateLdapValueTestGroup(t, groupDN, seedMemberDN)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLdapValueDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccLdapValueResourceConfig(groupDN, memberDN),
				Check:  testAccCheckLdapValuePresent(groupDN, "member", memberDN),
			},
			// Externally remove the value, then expect a plan to re-add it.
			{
				PreConfig: func() {
					conn, err := ldap.DialURL("ldap://localhost:3389")
					if err != nil {
						t.Fatalf("failed to connect to LDAP server: %v", err)
					}
					defer conn.Close()

					err = conn.Bind("cn=Manager,dc=example,dc=com", "secret")
					if err != nil {
						t.Fatalf("failed to bind to LDAP server: %v", err)
					}

					modifyReq := ldap.NewModifyRequest(groupDN, nil)
					modifyReq.Delete("member", []string{memberDN})
					if err := conn.Modify(modifyReq); err != nil {
						t.Fatalf("failed to remove member externally: %v", err)
					}
				},
				Config: testAccLdapValueResourceConfig(groupDN, memberDN),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				Check: testAccCheckLdapValuePresent(groupDN, "member", memberDN),
			},
		},
	})
}

func TestAccLdapValueResource_AdoptExistingValue(t *testing.T) {
	groupDN := "cn=valueadopt,ou=groups,dc=example,dc=com"
	seedMemberDN := "cn=seed,dc=example,dc=com"
	memberDN := "cn=Manager,dc=example,dc=com"

	testAccCreateLdapValueTestGroup(t, groupDN, seedMemberDN)

	// Add the value externally before Terraform ever creates the resource,
	// simulating a pre-existing value that ldap_value should adopt rather
	// than fail on.
	conn, err := ldap.DialURL("ldap://localhost:3389")
	if err != nil {
		t.Fatalf("failed to connect to LDAP server: %v", err)
	}
	if err := conn.Bind("cn=Manager,dc=example,dc=com", "secret"); err != nil {
		conn.Close()
		t.Fatalf("failed to bind to LDAP server: %v", err)
	}
	modifyReq := ldap.NewModifyRequest(groupDN, nil)
	modifyReq.Add("member", []string{memberDN})
	if err := conn.Modify(modifyReq); err != nil {
		conn.Close()
		t.Fatalf("failed to pre-seed member: %v", err)
	}
	conn.Close()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLdapValueDestroy,
		Steps: []resource.TestStep{
			{
				// memberDN already exists in the attribute (added above).
				// Creating ldap_value for the same value must not fail even
				// though the value already exists in LDAP.
				Config: testAccLdapValueResourceConfig(groupDN, memberDN),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"ldap_value.test",
						tfjsonpath.New("value"),
						knownvalue.StringExact(memberDN),
					),
				},
				Check: testAccCheckLdapValuePresent(groupDN, "member", memberDN),
			},
		},
	})
}

func testAccCheckLdapValuePresent(dn, attribute, value string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn, err := ldap.DialURL("ldap://localhost:3389")
		if err != nil {
			return fmt.Errorf("failed to connect to LDAP server: %w", err)
		}
		defer conn.Close()

		if err := conn.Bind("cn=Manager,dc=example,dc=com", "secret"); err != nil {
			return fmt.Errorf("failed to bind to LDAP server: %w", err)
		}

		sr, err := LdapSearch(conn, dn, "base", "(objectClass=*)", []string{attribute})
		if err != nil {
			return fmt.Errorf("failed to search LDAP: %w", err)
		}

		if len(sr.Entries) == 0 {
			return fmt.Errorf("entry %s not found", dn)
		}

		for _, attr := range sr.Entries[0].Attributes {
			if attr.Name != attribute {
				continue
			}
			for _, v := range attr.Values {
				if v == value {
					return nil
				}
			}
		}

		return fmt.Errorf("value %q not found in attribute %q on %s", value, attribute, dn)
	}
}

func testAccCheckLdapValueDestroy(s *terraform.State) error {
	conn, err := ldap.DialURL("ldap://localhost:3389")
	if err != nil {
		return fmt.Errorf("failed to connect to LDAP server: %w", err)
	}
	defer conn.Close()

	if err := conn.Bind("cn=Manager,dc=example,dc=com", "secret"); err != nil {
		return fmt.Errorf("failed to bind to LDAP server: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "ldap_value" {
			continue
		}

		dn := rs.Primary.Attributes["dn"]
		attribute := rs.Primary.Attributes["attribute"]
		value := rs.Primary.Attributes["value"]

		sr, err := LdapSearch(conn, dn, "base", "(objectClass=*)", []string{attribute})
		if err != nil {
			if ldapErr, ok := err.(*ldap.Error); ok && ldapErr.ResultCode == ldap.LDAPResultNoSuchObject {
				// Entry itself is gone; the value is certainly gone too.
				continue
			}
			return fmt.Errorf("error searching for entry %s: %w", dn, err)
		}

		if len(sr.Entries) == 0 {
			continue
		}

		for _, attr := range sr.Entries[0].Attributes {
			if attr.Name != attribute {
				continue
			}
			for _, v := range attr.Values {
				if v == value {
					return fmt.Errorf("value %q still present in attribute %q on %s after destroy", value, attribute, dn)
				}
			}
		}
	}

	return nil
}
