// Copyright (c) ngharo <root@ngha.ro>
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"net"
	"regexp"
	"testing"
	"time"

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

// TestAccProvider_RequestTimeout verifies that request_timeout bounds an LDAP
// request against a stalled server: the endpoint accepts the TCP connection but
// never replies, so the bind must fail after the configured timeout instead of
// hanging Terraform indefinitely.
func TestAccProvider_RequestTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start stalling listener: %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			// Accept and hold the connection without responding.
			_ = conn
		}
	}()

	stalledURL := fmt.Sprintf("ldap://%s", listener.Addr().String())

	start := time.Now()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccProviderConfigRequestTimeout(stalledURL, "1s"),
				ExpectError: regexp.MustCompile(`Unable to bind to LDAP server`),
			},
		},
	})

	elapsed := time.Since(start)
	// The 1s request timeout should fire well before the 10s guard below;
	// without SetTimeout the bind would block indefinitely.
	if elapsed > 10*time.Second {
		t.Errorf("bind took %s, expected it to time out within ~1s", elapsed)
	}
}

func testAccProviderConfigRequestTimeout(url, requestTimeout string) string {
	return fmt.Sprintf(`
provider "ldap" {
  url = %[1]q
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
  request_timeout = %[2]q
}

resource "ldap_entry" "test" {
  dn = "cn=default-test,ou=users,dc=example,dc=com"
  attributes = {
    objectClass = ["person"]
    cn = ["default-test"]
    sn = ["Test"]
  }
}
`, url, requestTimeout)
}

// TestAccProvider_InvalidRequestTimeout verifies that a malformed duration is
// rejected with a clear error instead of being silently ignored.
func TestAccProvider_InvalidRequestTimeout(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccProviderConfigInvalidTimeout(),
				ExpectError: regexp.MustCompile(`Invalid request_timeout`),
			},
		},
	})
}

func testAccProviderConfigInvalidTimeout() string {
	return `
provider "ldap" {
  url = "ldap://localhost:3389"
  bind_dn = "cn=Manager,dc=example,dc=com"
  bind_password = "secret"
  request_timeout = "not-a-duration"
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
