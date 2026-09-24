// Copyright (c) ngharo <root@ngha.ro>
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"os"
	"testing"

	"github.com/go-ldap/ldap/v3"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories is used to instantiate a provider during acceptance testing.
// The factory function is called for each Terraform CLI command to create a provider
// server that the CLI can connect to and interact with.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"ldap": providerserver.NewProtocol6WithError(New("test")()),
}

func testAccPreCheck(t *testing.T) {
	// Fail fast with a clear message if the containerized test LDAP server is
	// not running, instead of surfacing connection errors in every resource
	// operation.
	conn, err := ldap.DialURL(testAccLdapURL)
	if err != nil {
		t.Fatalf("LDAP test server unavailable at %s (start it with `make test`): %v", testAccLdapURL, err)
	}
	conn.Close()
}

// testAccSkipIfNotEnabled skips the test unless acceptance testing is enabled.
// Call at the very top of tests that perform setup work (e.g. seeding LDAP)
// before resource.Test: resource.Test only checks TF_ACC once invoked, so
// setup code placed before it would otherwise run (and fail) in unit-test runs.
func testAccSkipIfNotEnabled(t *testing.T) {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' set")
	}
}
