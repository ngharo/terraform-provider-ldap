// Copyright (c) ngharo <root@ngha.ro>
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/go-ldap/ldap/v3"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"golang.org/x/text/encoding/unicode"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &LdapEntryResource{}
var _ resource.ResourceWithImportState = &LdapEntryResource{}

// NewLdapEntryResource creates a new instance of the LDAP entry resource.
func NewLdapEntryResource() resource.Resource {
	return &LdapEntryResource{}
}

// LdapEntryResource defines the resource implementation for managing LDAP entries.
// It maintains a connection to the LDAP server and provides CRUD operations.
type LdapEntryResource struct {
	client *ldap.Conn
}

// LdapEntryResourceModel describes the resource data model for LDAP entries.
// It maps the Terraform schema to Go types for state management.
type LdapEntryResourceModel struct {
	DN              types.String `tfsdk:"dn"`                    // Distinguished Name - unique identifier for the LDAP entry
	Attributes      types.Map    `tfsdk:"attributes"`            // Map of List[String] - regular LDAP attributes stored in state
	AttributesWO    types.Map    `tfsdk:"attributes_wo"`         // Map of List[String] - write-only sensitive attributes (not stored in state)
	AttributesWOVer types.Int64  `tfsdk:"attributes_wo_version"` // Version trigger for attributes_wo changes
	Id              types.String `tfsdk:"id"`                    // Resource identifier (same as DN)
}

// Metadata sets the resource type name for the LDAP entry resource.
func (r *LdapEntryResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_entry"
}

// Schema defines the schema for the LDAP entry resource, including attributes for DN,
// regular attributes, write-only attributes, and versioning for write-only changes.
func (r *LdapEntryResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an LDAP entry. Each entry is identified by its Distinguished Name (DN) and contains object classes and attributes.",

		Attributes: map[string]schema.Attribute{
			"dn": schema.StringAttribute{
				MarkdownDescription: "The distinguished name (DN) of the LDAP entry. This uniquely identifies the entry in the LDAP directory tree. Changing this forces a new resource to be created.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"attributes": schema.MapAttribute{
				MarkdownDescription: "Map of LDAP attributes for the entry. The keys are attribute names and values are lists of attribute values. For single-valued attributes, provide a list with one element. For multi-valued attributes like `member` in groups, provide a list with multiple elements. The `objectClass` attribute is required and defines the schema for the entry.",
				Required:            true,
				ElementType:         types.ListType{ElemType: types.StringType},
				PlanModifiers: []planmodifier.Map{
					AttributesSetSemanticsModifier{},
				},
			},
			"attributes_wo": schema.MapAttribute{
				MarkdownDescription: "Write-only map of LDAP attributes for the entry containing sensitive values. The keys are attribute names and values are lists of attribute values. These attributes are never stored in Terraform state and are only used during resource creation and updates. Use this for sensitive data like passwords, API keys, or other secrets. Must be used in conjunction with `attributes_wo_version`. Requires Terraform 1.11 or later. NOTE: `unicodePwd` will be automatically encoded as UTF-16LE for Active Directory.",
				Optional:            true,
				WriteOnly:           true,
				ElementType:         types.ListType{ElemType: types.StringType},
			},
			"attributes_wo_version": schema.Int64Attribute{
				MarkdownDescription: "Version number for write-only attributes. Increment this value (e.g., 1, 2, 3) whenever you want to update the `attributes_wo` values on the LDAP server. Since write-only attributes are not stored in state, Terraform cannot detect changes to them. Changing this version number triggers the provider to send the current `attributes_wo` values to the LDAP server during updates.",
				Optional:            true,
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier for this resource, which is the same as the DN.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Configure initializes the resource with the LDAP client connection from the provider.
func (r *LdapEntryResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ldap.Conn)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ldap.Conn, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

// Create creates a new LDAP entry with the specified DN and attributes.
// Has special encoding support for Active Directory's unicodePwd attribute.
func (r *LdapEntryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan LdapEntryResourceModel
	var config LdapEntryResourceModel

	// Retrieve values from plan
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve values from config (for write-only attributes)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// LDAP Request Attributes
	attributes := make(map[string][]string)

	tfMapToLDAPAttrs(ctx, &resp.Diagnostics, &plan.Attributes, attributes)
	if !config.AttributesWO.IsNull() {
		tfMapToLDAPAttrs(ctx, &resp.Diagnostics, &config.AttributesWO, attributes)
	}

	// Special handling for unicodePwd attribute (Active Directory)
	if value, ok := attributes["unicodePwd"]; ok {
		encoded, err := encodeUnicodePwd(value[0])

		if err != nil {
			resp.Diagnostics.AddError("Error encoding unicodePwd", fmt.Sprintf("Unable to encode unicodePwd value: %s", err))
			return
		}

		attributes["unicodePwd"] = []string{encoded}
	}

	// Create LDAP add request
	addReq := ldap.NewAddRequest(plan.DN.ValueString(), nil)
	for attr, values := range attributes {
		addReq.Attribute(attr, values)
	}

	// Execute LDAP add operation
	err := r.client.Add(addReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating LDAP entry",
			fmt.Sprintf("Unable to create LDAP entry %s: %s", plan.DN.ValueString(), err),
		)
		return
	}
	tflog.Trace(ctx, fmt.Sprintf("created an LDAP entry: %s", plan.Id))

	plan.Id = plan.DN

	// Save plan into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *LdapEntryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data LdapEntryResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Build list of attributes to request from LDAP
	// Only request attributes that are managed in the resource configuration
	var attributesToRequest []string

	var attrsMap map[string]types.List
	diags := data.Attributes.ElementsAs(ctx, &attrsMap, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	for attrName := range attrsMap {
		attributesToRequest = append(attributesToRequest, attrName)
	}

	// During import, state is empty, check if import specified which attributes to fetch
	if len(attributesToRequest) == 0 {
		// Check private state for import attributes
		privateData, diags := req.Private.GetKey(ctx, "import_attributes")
		resp.Diagnostics.Append(diags...)

		if len(privateData) > 0 {
			var importData map[string][]string
			if err := json.Unmarshal(privateData, &importData); err == nil {
				if attrs, ok := importData["import_attributes"]; ok {
					attributesToRequest = attrs
				}
			}
		}

		// If still empty, default to objectClass only
		if len(attributesToRequest) == 0 {
			attributesToRequest = []string{"objectClass"}
		}
	}

	// Search for the LDAP entry - only request managed attributes
	searchReq := ldap.NewSearchRequest(
		data.DN.ValueString(),
		ldap.ScopeBaseObject,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		"(objectClass=*)",
		attributesToRequest,
		nil,
	)

	sr, err := r.client.Search(searchReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading LDAP entry",
			fmt.Sprintf("Unable to read LDAP entry %s: %s", data.DN.ValueString(), err),
		)
		return
	}

	// Check if entry exists
	if len(sr.Entries) == 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	entry := sr.Entries[0]

	// Update attributes from LDAP response
	// Since we only requested managed attributes, we can include all returned attributes
	readAttrsMap := make(map[string][]string)
	for _, attr := range entry.Attributes {
		readAttrsMap[attr.Name] = attr.Values
	}

	attributesMap, attrsDiags := types.MapValueFrom(ctx, types.ListType{ElemType: types.StringType}, readAttrsMap)
	resp.Diagnostics.Append(attrsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Attributes = attributesMap

	// Set ID to DN
	data.Id = data.DN

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LdapEntryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan LdapEntryResourceModel
	var config LdapEntryResourceModel
	var state LdapEntryResourceModel

	// Retrieve values from plan
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve values from config (write-only attributes)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve values from state
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	attributes := make(map[string][]string)
	tfMapToLDAPAttrs(ctx, &resp.Diagnostics, &plan.Attributes, attributes)

	versionChanged := !plan.AttributesWOVer.Equal(state.AttributesWOVer)

	// Convert write-only attributes from config only if version changed
	if versionChanged && !config.AttributesWO.IsNull() {
		tfMapToLDAPAttrs(ctx, &resp.Diagnostics, &config.Attributes, attributes)

		// Special handling for unicodePwd attribute (Active Directory)
		if value, ok := attributes["unicodePwd"]; ok {
			encoded, err := encodeUnicodePwd(value[0])

			if err != nil {
				resp.Diagnostics.AddError("Error encoding unicodePwd", fmt.Sprintf("Unable to encode unicodePwd value: %s", err))
				return
			}

			attributes["unicodePwd"] = []string{encoded}
		}
	}

	// Get attributes from state for comparisons
	// Needed to build up LDAP replace and delete ops
	currentAttrs := make(map[string][]string)
	tfMapToLDAPAttrs(ctx, &resp.Diagnostics, &state.Attributes, currentAttrs)

	// Create LDAP modify request
	modifyReq := ldap.NewModifyRequest(plan.DN.ValueString(), nil)

	// Update changed attributes
	for key, newValues := range attributes {
		if currentValues, exists := currentAttrs[key]; !exists || !stringSlicesEqual(currentValues, newValues) {
			modifyReq.Replace(key, newValues)
		}
	}

	// Remove attributes that are no longer present
	for key := range currentAttrs {
		if _, exists := attributes[key]; !exists {
			modifyReq.Delete(key, nil)
		}
	}

	// Execute LDAP modify operation if there are changes
	if len(modifyReq.Changes) > 0 {
		err := r.client.Modify(modifyReq)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error updating LDAP entry",
				fmt.Sprintf("Unable to update LDAP entry %s: %s", plan.DN.ValueString(), err),
			)
			return
		}
	}

	plan.Id = plan.DN

	// Save updated plan into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *LdapEntryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data LdapEntryResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	delReq := ldap.NewDelRequest(data.DN.ValueString(), nil)

	err := r.client.Del(delReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting LDAP entry",
			fmt.Sprintf("Unable to delete LDAP entry %s: %s", data.DN.ValueString(), err),
		)
		return
	}
}

func (r *LdapEntryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import ID can be either:
	// 1. Simple DN string: "CN=user,OU=Users,DC=example,DC=com"
	// 2. JSON object: {"dn": "CN=user,OU=Users,DC=example,DC=com", "attributes": ["objectClass", "cn"]}

	var dn string
	var attributesToImport []string

	// Try to parse as JSON first
	var importSpec struct {
		DN         string   `json:"dn"`
		Attributes []string `json:"attributes"`
	}

	if err := json.Unmarshal([]byte(req.ID), &importSpec); err == nil {
		// Successfully parsed as JSON
		dn = importSpec.DN
		attributesToImport = importSpec.Attributes
	} else {
		// Not JSON, treat as simple DN string
		dn = req.ID
		attributesToImport = []string{"objectClass"} // Default to just objectClass
	}

	// Set the DN in state
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("dn"), dn)...)

	// Store the attributes to import in private state so Read can use them
	if len(attributesToImport) > 0 {
		privateData, err := json.Marshal(map[string][]string{"import_attributes": attributesToImport})
		if err != nil {
			resp.Diagnostics.AddError(
				"Error encoding import attributes",
				fmt.Sprintf("Unable to encode import attributes: %s", err),
			)
			return
		}
		resp.Private.SetKey(ctx, "import_attributes", privateData)
	}
}

// AttributesSetSemanticsModifier is a plan modifier that treats list values as sets (order-independent).
// This is necessary because LDAP returns multi-valued attributes in arbitrary order.
type AttributesSetSemanticsModifier struct{}

func (m AttributesSetSemanticsModifier) Description(ctx context.Context) string {
	return "Treats attribute list values as unordered sets"
}

func (m AttributesSetSemanticsModifier) MarkdownDescription(ctx context.Context) string {
	return "Treats attribute list values as unordered sets"
}

func (m AttributesSetSemanticsModifier) PlanModifyMap(ctx context.Context, req planmodifier.MapRequest, resp *planmodifier.MapResponse) {
	// If config and state are both known, compare them as sets
	if req.ConfigValue.IsNull() || req.StateValue.IsNull() {
		return
	}

	if req.ConfigValue.IsUnknown() || req.StateValue.IsUnknown() {
		return
	}

	// Extract config and state as maps of lists
	var configMap map[string]types.List
	var stateMap map[string]types.List

	diags := req.ConfigValue.ElementsAs(ctx, &configMap, false)
	if diags.HasError() {
		return
	}

	diags = req.StateValue.ElementsAs(ctx, &stateMap, false)
	if diags.HasError() {
		return
	}

	// Check if all attributes are equal as sets
	allEqual := true
	if len(configMap) != len(stateMap) {
		allEqual = false
	} else {
		for key, configList := range configMap {
			stateList, ok := stateMap[key]
			if !ok {
				allEqual = false
				break
			}

			var configValues []string
			var stateValues []string

			diags = configList.ElementsAs(ctx, &configValues, false)
			if diags.HasError() {
				return
			}

			diags = stateList.ElementsAs(ctx, &stateValues, false)
			if diags.HasError() {
				return
			}

			// Use order-independent comparison
			if !stringSlicesEqual(configValues, stateValues) {
				allEqual = false
				break
			}
		}
	}

	// If all attributes are equal as sets, use state value to prevent spurious diff
	if allEqual {
		resp.PlanValue = req.StateValue
	}
}

// Helper function to compare string slices as sets (order-independent).
// LDAP multi-valued attributes are unordered, so we need to compare them as sets.
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	// Sort copies to compare as sets
	aSorted := make([]string, len(a))
	bSorted := make([]string, len(b))
	copy(aSorted, a)
	copy(bSorted, b)
	sort.Strings(aSorted)
	sort.Strings(bSorted)

	for i, v := range aSorted {
		if v != bSorted[i] {
			return false
		}
	}
	return true
}

// encodeUnicodePwd encodes a password for Active Directory's unicodePwd attribute.
// The password must be enclosed in double quotes and encoded as UTF-16LE.
// See: https://ldapwiki.com/wiki/Wiki.jsp?page=UnicodePwd
func encodeUnicodePwd(password string) (string, error) {
	utf16 := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	pwdEncoded, err := utf16.NewEncoder().String(fmt.Sprintf(`"%s"`, password))
	if err != nil {
		return "", err
	}
	return pwdEncoded, nil
}

// tfMapToLDAPAttrs converts a Terraform Map type to LDAP attribute format (map[string][]string).
// It extracts the map elements and populates the attrs parameter with the converted values.
func tfMapToLDAPAttrs(ctx context.Context, diag *diag.Diagnostics, tfMap *types.Map, attrs map[string][]string) {
	attrsMap := make(map[string]types.List)

	diags := tfMap.ElementsAs(ctx, &attrsMap, false)
	diag.Append(diags...)
	if diag.HasError() {
		return
	}

	for key, valueList := range attrsMap {
		var values []string

		diags := valueList.ElementsAs(ctx, &values, false)
		diag.Append(diags...)
		if diag.HasError() {
			return
		}

		attrs[key] = values
	}
}
