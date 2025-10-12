// Copyright (c) ngharo <root@ngha.ro>
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"
)

func TestStringSlicesEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected bool
	}{
		{
			name:     "empty slices",
			a:        []string{},
			b:        []string{},
			expected: true,
		},
		{
			name:     "nil slices",
			a:        nil,
			b:        nil,
			expected: true,
		},
		{
			name:     "one nil one empty",
			a:        nil,
			b:        []string{},
			expected: true,
		},
		{
			name:     "equal single elements",
			a:        []string{"a"},
			b:        []string{"a"},
			expected: true,
		},
		{
			name:     "different single elements",
			a:        []string{"a"},
			b:        []string{"b"},
			expected: false,
		},
		{
			name:     "same elements same order",
			a:        []string{"a", "b", "c"},
			b:        []string{"a", "b", "c"},
			expected: true,
		},
		{
			name:     "same elements different order",
			a:        []string{"c", "a", "b"},
			b:        []string{"a", "b", "c"},
			expected: true,
		},
		{
			name:     "different lengths",
			a:        []string{"a", "b"},
			b:        []string{"a", "b", "c"},
			expected: false,
		},
		{
			name:     "duplicates in both - same count",
			a:        []string{"a", "a", "b"},
			b:        []string{"b", "a", "a"},
			expected: true,
		},
		{
			name:     "duplicates - different count",
			a:        []string{"a", "a", "b"},
			b:        []string{"a", "b", "b"},
			expected: false,
		},
		{
			name:     "case sensitive",
			a:        []string{"a", "B"},
			b:        []string{"A", "b"},
			expected: false,
		},
		{
			name:     "ldap objectClass example",
			a:        []string{"person", "organizationalPerson", "inetOrgPerson"},
			b:        []string{"inetOrgPerson", "person", "organizationalPerson"},
			expected: true,
		},
		{
			name:     "ldap member attribute example",
			a:        []string{"cn=user1,ou=users,dc=example,dc=com", "cn=user2,ou=users,dc=example,dc=com"},
			b:        []string{"cn=user2,ou=users,dc=example,dc=com", "cn=user1,ou=users,dc=example,dc=com"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stringSlicesEqual(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("stringSlicesEqual(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestEncodeUnicodePwd(t *testing.T) {
	tests := []struct {
		name        string
		password    string
		expectError bool
	}{
		{
			name:        "simple password",
			password:    "password123",
			expectError: false,
		},
		{
			name:        "empty password",
			password:    "",
			expectError: false,
		},
		{
			name:        "password with special characters",
			password:    "P@ssw0rd!#$%",
			expectError: false,
		},
		{
			name:        "password with spaces",
			password:    "my password 123",
			expectError: false,
		},
		{
			name:        "password with unicode",
			password:    "пароль123",
			expectError: false,
		},
		{
			name:        "password with emoji",
			password:    "pass🔒word",
			expectError: false,
		},
		{
			name:        "very long password",
			password:    "ThisIsAVeryLongPasswordThatExceedsNormalLengthRequirements1234567890ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz",
			expectError: false,
		},
		{
			name:        "password with quotes",
			password:    `password"with"quotes`,
			expectError: false,
		},
		{
			name:        "password with backslash",
			password:    `password\with\backslash`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := encodeUnicodePwd(tt.password)

			if tt.expectError {
				if err == nil {
					t.Errorf("encodeUnicodePwd(%q) expected error, got nil", tt.password)
				}
				return
			}

			if err != nil {
				t.Errorf("encodeUnicodePwd(%q) unexpected error: %v", tt.password, err)
				return
			}

			// Verify result is not empty
			if result == "" {
				t.Errorf("encodeUnicodePwd(%q) returned empty string", tt.password)
			}

			// Verify result is different from input (it should be encoded)
			if result == tt.password {
				t.Errorf("encodeUnicodePwd(%q) returned same string, expected UTF-16LE encoded", tt.password)
			}

			// Verify result contains UTF-16LE encoded content (should have null bytes for ASCII)
			// For ASCII characters, UTF-16LE will have null bytes
			if len(tt.password) > 0 && len(result) < len(tt.password) {
				t.Errorf("encodeUnicodePwd(%q) result too short: got %d bytes, want >= %d", tt.password, len(result), len(tt.password))
			}
		})
	}
}
