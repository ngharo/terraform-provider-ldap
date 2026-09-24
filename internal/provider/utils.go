package provider

import (
	"context"
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

func LdapSearch(ctx context.Context, conn *ldap.Conn, baseDN string, scope string, filter string, attributes []string) (*ldap.SearchResult, error) {
	// go-ldap cannot cancel an in-flight request, so refuse to start one on an
	// already-cancelled context.
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("LDAP search for %s cancelled: %w", baseDN, err)
	}

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

// Marshals LDAP search results into []LdapEntry. Conversion diagnostics are
// returned rather than an error so callers can append them directly to their
// response diagnostics.
func MarshalLdapResults(ctx context.Context, sr *ldap.SearchResult, requestedAttributes []string) ([]LdapEntry, diag.Diagnostics) {
	var diags diag.Diagnostics

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
		attributesMap, mapDiags := types.MapValueFrom(ctx, types.ListType{ElemType: types.StringType}, attributes)
		diags.Append(mapDiags...)
		if diags.HasError() {
			return nil, diags
		}

		result := LdapEntry{
			entry:      entry,
			DN:         types.StringValue(entry.DN),
			Attributes: attributesMap,
		}

		results = append(results, result)
	}

	return results, diags
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

// cancelledFromContext returns a non-empty diagnostic when ctx is already
// cancelled or expired, so callers can bail before issuing LDAP calls that
// go-ldap cannot interrupt mid-flight.
func cancelledFromContext(ctx context.Context) diag.Diagnostics {
	var diags diag.Diagnostics
	if err := ctx.Err(); err != nil {
		diags.AddError(
			"LDAP operation cancelled",
			fmt.Sprintf("Aborting before LDAP request because the context is %s", err),
		)
	}
	return diags
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

// AttributeExistsInLDAP checks if an attribute exists on an LDAP entry.
// Returns true if the attribute exists (even if empty), false if it doesn't exist.
// Returns an error if the LDAP query fails.
func AttributeExistsInLDAP(ctx context.Context, conn *ldap.Conn, dn string, attributeName string) (bool, []string, error) {
	sr, err := LdapSearch(ctx, conn, dn, "base", "(objectClass=*)", []string{attributeName})
	if err != nil {
		return false, nil, err
	}

	if len(sr.Entries) == 0 {
		return false, nil, fmt.Errorf("entry not found: %s", dn)
	}

	entry := sr.Entries[0]
	for _, attr := range entry.Attributes {
		if attr.Name == attributeName {
			return true, attr.Values, nil
		}
	}

	return false, nil, nil
}
