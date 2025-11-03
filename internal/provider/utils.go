package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-ldap/ldap/v3"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"golang.org/x/text/encoding/unicode"
)

type LdapEntry struct {
	entry *ldap.Entry

	DN         types.String `tfsdk:"dn"`
	Attributes types.Map    `tfsdk:"attributes"`
}

func ConvertHumanReadableLDAPScope(scope string) (int, error) {
	var ldapScope int
	switch scope {
	case "base":
		ldapScope = ldap.ScopeBaseObject
	case "one":
		ldapScope = ldap.ScopeSingleLevel
	case "sub":
		ldapScope = ldap.ScopeWholeSubtree
	default:
		return -1, fmt.Errorf("scope must be one of 'base', 'one', or 'sub', got: %s", scope)
	}

	return ldapScope, nil
}

func LdapSearch(conn *ldap.Conn, baseDN string, scope string, filter string, attributes []string) (*ldap.SearchResult, error) {
	searchScope, err := ConvertHumanReadableLDAPScope(scope)
	if err != nil {
		return nil, err
	}

	req := ldap.NewSearchRequest(
		baseDN,
		searchScope,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		filter,
		attributes,
		nil,
	)

	return conn.Search(req)
}

// Marshals LDAP search results into []LdapEntry.
func MarshalLdapResults(ctx context.Context, sr *ldap.SearchResult, requestedAttributes []string) ([]LdapEntry, error) {
	results := make([]LdapEntry, 0, len(sr.Entries))

	for _, entry := range sr.Entries {
		attributes := make(map[string][]string)

		for _, attr := range entry.Attributes {
			attributes[attr.Name] = attr.Values
		}

		// Compare attributes returned by search against those requested.
		// This is a provider logic thing. For user experience, we always represent
		// non-existent attributes as empty lists.
		for _, ra := range requestedAttributes {
			if _, exists := attributes[ra]; !exists {
				tflog.Trace(ctx, fmt.Sprintf("Requested attribute '%s' not found in LDAP response", ra))
				attributes[ra] = []string{}
			}
		}

		// Convert attributes to types.Map
		attributesMap, diags := types.MapValueFrom(ctx, types.ListType{ElemType: types.StringType}, attributes)
		if diags.HasError() {
			return nil, errors.New(diags[len(diags)].Detail())
		}

		result := LdapEntry{
			entry:      entry,
			DN:         types.StringValue(entry.DN),
			Attributes: attributesMap,
		}

		results = append(results, result)
	}

	return results, nil
}

// GetLdapConnection extracts the LDAP connection from provider data.
// Returns nil if providerData is nil (provider not configured) or adds an error diagnostic if the type is unexpected.
func GetLdapConnection(providerData any, diagnostics *diag.Diagnostics, resourceType string) *ldap.Conn {
	// Prevent panic if the provider has not been configured.
	if providerData == nil {
		return nil
	}

	conn, ok := providerData.(*ldap.Conn)
	if !ok {
		diagnostics.AddError(
			fmt.Sprintf("Unexpected %s Configure Type", resourceType),
			fmt.Sprintf("Expected *ldap.Conn, got: %T. Please report this issue to the provider developers.", providerData),
		)
		return nil
	}

	return conn
}

// ProcessUnicodePwd handles special encoding for Active Directory's unicodePwd attribute.
// If the attributes map contains a unicodePwd key, it encodes the password as UTF-16LE
// with double quotes as required by Active Directory. Returns diagnostics on encoding errors.
func ProcessUnicodePwd(attributes map[string][]string) diag.Diagnostics {
	var diags diag.Diagnostics

	if value, ok := attributes["unicodePwd"]; ok && len(value) > 0 {
		encoded, err := encodeUnicodePwd(value[0])
		if err != nil {
			diags.AddError(
				"Error encoding unicodePwd",
				fmt.Sprintf("Unable to encode unicodePwd value: %s", err),
			)
			return diags
		}
		attributes["unicodePwd"] = []string{encoded}
	}

	return diags
}

// encodeUnicodePwd encodes a password for Active Directory's unicodePwd attribute.
// return value is double quoted and encoded as UTF-16LE.
// See: https://ldapwiki.com/wiki/Wiki.jsp?page=UnicodePwd
func encodeUnicodePwd(password string) (string, error) {
	utf16 := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	pwdEncoded, err := utf16.NewEncoder().String(fmt.Sprintf(`"%s"`, password))
	if err != nil {
		return "", err
	}
	return pwdEncoded, nil
}
