// Copyright (c) ngharo <root@ngha.ro>
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestResolveDuration(t *testing.T) {
	testCases := []struct {
		name          string
		configVal     types.String
		envVar        string
		envValue      string
		defaultVal    time.Duration
		expected      time.Duration
		expectError   bool
		errorContains string
	}{
		{
			name:       "config unset, env unset -> default",
			configVal:  types.StringNull(),
			envVar:     "LDAP_TEST_TIMEOUT",
			defaultVal: 60 * time.Second,
			expected:   60 * time.Second,
		},
		{
			name:       "config unset, env set -> env value",
			configVal:  types.StringNull(),
			envVar:     "LDAP_TEST_TIMEOUT",
			envValue:   "90s",
			defaultVal: 60 * time.Second,
			expected:   90 * time.Second,
		},
		{
			name:       "config set overrides env",
			configVal:  types.StringValue("30s"),
			envVar:     "LDAP_TEST_TIMEOUT",
			envValue:   "90s",
			defaultVal: 60 * time.Second,
			expected:   30 * time.Second,
		},
		{
			name:       "config set to empty string -> default (env ignored, config wins)",
			configVal:  types.StringValue(""),
			envVar:     "LDAP_TEST_TIMEOUT",
			envValue:   "10s",
			defaultVal: 60 * time.Second,
			expected:   60 * time.Second,
		},
		{
			name:       "zero duration disables timeout",
			configVal:  types.StringValue("0s"),
			envVar:     "LDAP_TEST_TIMEOUT",
			defaultVal: 60 * time.Second,
			expected:   0,
		},
		{
			name:       "complex duration",
			configVal:  types.StringValue("1m30s"),
			envVar:     "LDAP_TEST_TIMEOUT",
			defaultVal: 60 * time.Second,
			expected:   90 * time.Second,
		},
		{
			name:          "invalid duration",
			configVal:     types.StringValue("not-a-duration"),
			envVar:        "LDAP_TEST_TIMEOUT",
			defaultVal:    60 * time.Second,
			expectError:   true,
			errorContains: "Invalid duration",
		},
		{
			name:          "negative duration",
			configVal:     types.StringValue("-5s"),
			envVar:        "LDAP_TEST_TIMEOUT",
			defaultVal:    60 * time.Second,
			expectError:   true,
			errorContains: "must be >= 0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.envVar, tc.envValue)

			got, diags := resolveDuration(tc.configVal, tc.envVar, tc.defaultVal, "test_timeout")

			if tc.expectError {
				if !diags.HasError() {
					t.Fatalf("expected error diagnostics, got none (result: %s)", got)
				}
				if len(diags) == 0 || !contains(diags, tc.errorContains) {
					t.Errorf("expected diagnostic containing %q, got: %v", tc.errorContains, diags)
				}
				return
			}

			if diags.HasError() {
				t.Fatalf("unexpected error diagnostics: %v", diags)
			}
			if got != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, got)
			}
		})
	}
}

// contains reports whether any diagnostic's summary or detail contains substr.
func contains(diags diag.Diagnostics, substr string) bool {
	for _, d := range diags {
		if strings.Contains(d.Summary(), substr) || strings.Contains(d.Detail(), substr) {
			return true
		}
	}
	return false
}

func TestCancelledFromContext(t *testing.T) {
	t.Run("active context produces no diagnostics", func(t *testing.T) {
		diags := cancelledFromContext(context.Background())
		if diags.HasError() {
			t.Errorf("expected no diagnostics for active context, got: %v", diags)
		}
	})

	t.Run("cancelled context produces error diagnostic", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		diags := cancelledFromContext(ctx)
		if !diags.HasError() {
			t.Fatal("expected error diagnostics for cancelled context, got none")
		}
		if len(diags) != 1 || diags[0].Summary() != "LDAP operation cancelled" {
			t.Errorf("unexpected diagnostics: %v", diags)
		}
	})

	t.Run("expired context produces error diagnostic", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		<-ctx.Done()

		diags := cancelledFromContext(ctx)
		if !diags.HasError() {
			t.Fatal("expected error diagnostics for expired context, got none")
		}
	})
}
