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
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &LdapEntryResource{}
var _ resource.ResourceWithImportState = &LdapEntryResource{}

func NewLdapEntryResource() resource.Resource {
	return &LdapEntryResource{}
}

// LdapEntryResource defines the resource implementation for managing LDAP entries.
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

// Schema defines the schema for the LDAP entry resource.
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
	r.client = GetLdapConnection(req.ProviderData, &resp.Diagnostics, "Resource")
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

	diags := unmarshalTerraformAttributes(ctx, &plan.Attributes, attributes)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !config.AttributesWO.IsNull() {
		diags = unmarshalTerraformAttributes(ctx, &config.AttributesWO, attributes)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Special handling for unicodePwd attribute (Active Directory)
	resp.Diagnostics.Append(ProcessUnicodePwd(attributes)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create LDAP add request
	addReq := ldap.NewAddRequest(plan.DN.ValueString(), nil)
	for attr, values := range attributes {
		// Skip attributes with empty values - LDAP servers reject empty attributes during creation
		if len(values) > 0 {
			addReq.Attribute(attr, values)
		}
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
	var state LdapEntryResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var attributesToRequest []string

	var attrsMap map[string]types.List
	diags := state.Attributes.ElementsAs(ctx, &attrsMap, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	for attrName := range attrsMap {
		attributesToRequest = append(attributesToRequest, attrName)
	}

	// During import, state is empty, and we don't have access to the config
	// Check if import specified which attributes to fetch via private state
	if len(attributesToRequest) == 0 {
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

	sr, err := LdapSearch(r.client, state.DN.ValueString(), "base", "(objectClass=*)", attributesToRequest)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading LDAP entry",
			fmt.Sprintf("Unable to read LDAP entry %s: %s", state.DN.ValueString(), err),
		)
		return
	}

	results, err := MarshalLdapResults(ctx, sr, attributesToRequest)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error marshaling LDAP results",
			fmt.Sprintf("Unable to marshal LDAP results for %s: %s", state.DN.ValueString(), err),
		)
		return
	}
	if len(results) == 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	entry := results[0]

	state.Attributes = entry.Attributes
	state.Id = state.DN

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
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
	diags := unmarshalTerraformAttributes(ctx, &plan.Attributes, attributes)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	versionChanged := !plan.AttributesWOVer.Equal(state.AttributesWOVer)

	// Convert write-only attributes from config only if version changed
	if versionChanged && !config.AttributesWO.IsNull() {
		diags = unmarshalTerraformAttributes(ctx, &config.AttributesWO, attributes)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		// Special handling for unicodePwd attribute (Active Directory)
		resp.Diagnostics.Append(ProcessUnicodePwd(attributes)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Get attributes from state for comparisons
	// Needed to build up LDAP replace and delete ops
	currentAttrs := make(map[string][]string)
	diags = unmarshalTerraformAttributes(ctx, &state.Attributes, currentAttrs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create LDAP modify request
	modifyReq := ldap.NewModifyRequest(plan.DN.ValueString(), nil)

	// Update changed attributes
	for key, newValues := range attributes {
		if currentValues, exists := currentAttrs[key]; !exists || !stringSlicesEqual(currentValues, newValues) {
			if len(newValues) == 0 {
				// Delete attribute when set to empty list
				// Active Directory and some LDAP servers reject Replace with empty values
				modifyReq.Delete(key, nil)
			} else {
				modifyReq.Replace(key, newValues)
			}
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

	var importSpec struct {
		DN         string   `json:"dn"`
		Attributes []string `json:"attributes"`
	}

	if err := json.Unmarshal([]byte(req.ID), &importSpec); err == nil {
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

	// Check if all managed attributes are equal as sets
	// Null attributes in config are unmanaged and should not trigger diffs
	allEqual := true

	// Compare only managed (non-null) attributes from config
	// Attributes in state that are null in config or don't exist in config are ignored
	for key, configList := range configMap {
		// Skip null attributes - they are unmanaged and should not be compared
		if configList.IsNull() {
			continue
		}

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

	// If all managed attributes are equal, build a plan value that:
	// - Uses state values for managed attributes (to preserve order)
	// - Uses state values for unmanaged (null) attributes in config
	// - Omits attributes that are in state but not in config
	if allEqual {
		planMap := make(map[string]types.List)

		// Only include attributes that are in config
		// Use state values to preserve order and avoid diffs for null attributes
		for key, configList := range configMap {
			if configList.IsNull() {
				// For null attributes, use state value (could be [] or actual values)
				if stateList, existsInState := stateMap[key]; existsInState {
					planMap[key] = stateList
				}
			} else {
				// For managed attributes, use state value if present, otherwise use config
				if stateList, existsInState := stateMap[key]; existsInState {
					planMap[key] = stateList
				} else {
					planMap[key] = configList
				}
			}
		}

		planValue, planDiags := types.MapValueFrom(ctx, types.ListType{ElemType: types.StringType}, planMap)
		diags = append(diags, planDiags...)
		if diags.HasError() {
			return
		}

		resp.PlanValue = planValue
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

// unmarshalTerraformAttributes converts a Terraform Map type to map[string][]string.
// Null values are skipped - they represent unmanaged attributes that should not be modified.
func unmarshalTerraformAttributes(ctx context.Context, tfMap *types.Map, attrs map[string][]string) diag.Diagnostics {
	var diag diag.Diagnostics
	attrsMap := make(map[string]types.List)

	diags := tfMap.ElementsAs(ctx, &attrsMap, false)
	diag.Append(diags...)
	if diag.HasError() {
		return diag
	}

	for key, valueList := range attrsMap {
		tflog.Trace(ctx, fmt.Sprintf("key: %s | type: %s | isnull: %v", key, valueList.Type(ctx), valueList.IsNull()))
		var values []string

		diags := valueList.ElementsAs(ctx, &values, false)
		diag.Append(diags...)
		if diag.HasError() {
			return diag
		}

		attrs[key] = values
	}

	return diag
}
