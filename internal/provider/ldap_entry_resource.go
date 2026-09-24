// Copyright (c) ngharo <root@ngha.ro>
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"errors"
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
var _ resource.ResourceWithValidateConfig = &LdapEntryResource{}

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
	Attributes      types.Map    `tfsdk:"attributes"`            // Map of Set[String] - regular LDAP attributes stored in state
	AttributesWO    types.Map    `tfsdk:"attributes_wo"`         // Map of Set[String] - write-only sensitive attributes (not stored in state)
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
		MarkdownDescription: `Manages an LDAP entry. Each entry is identified by its Distinguished Name (DN) and contains attributes.

### Omitted and null attributes
Null or omitted attributes in the configuration are **not read or managed** by the provider.
`,

		Attributes: map[string]schema.Attribute{
			"dn": schema.StringAttribute{
				MarkdownDescription: "The distinguished name (DN) of the LDAP entry. Changing this forces a new resource to be created.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"attributes": schema.MapAttribute{
				MarkdownDescription: "Map of LDAP attributes for the entry. Attribute values are unordered sets; order in the configuration is irrelevant. The `objectClass` attribute is required and defines the schema for the entry.",
				Required:            true,
				ElementType:         types.SetType{ElemType: types.StringType},
			},
			"attributes_wo": schema.MapAttribute{
				MarkdownDescription: "Write-only map of LDAP attributes for the entry containing sensitive values. Attribute values are unordered sets. Must be used in conjunction with `attributes_wo_version`. NOTE: `unicodePwd` will be automatically encoded as UTF-16LE for Active Directory.",
				Optional:            true,
				WriteOnly:           true,
				ElementType:         types.SetType{ElemType: types.StringType},
			},
			"attributes_wo_version": schema.Int64Attribute{
				MarkdownDescription: "Version number for write-only attributes. Changing this version number triggers the provider to send the current `attributes_wo` values to the LDAP server during updates.",
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
	if req.ProviderData == nil {
		return
	}

	r.client = GetLdapConnection(req.ProviderData, &resp.Diagnostics, "Resource")
}

// ValidateConfig enforces the documented coupling between attributes_wo and
// attributes_wo_version. Write-only values are only written to the LDAP server
// when attributes_wo_version changes, so a non-empty attributes_wo without a
// version would be silently ignored after creation, and a version without any
// attributes_wo values has nothing to send. Both configurations are rejected.
func (r *LdapEntryResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	// Write-only attribute values are only delivered to the provider by clients
	// that support them (Terraform 1.11+). Terraform already reports the
	// unsupported-client case as its own error, so there is nothing useful for
	// us to validate here.
	if !req.ClientCapabilities.WriteOnlyAttributesAllowed {
		return
	}

	var config LdapEntryResourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Skip validation when either side is unknown, e.g. pending interpolation or
	// references to resources whose values are not yet known.
	woSet := !config.AttributesWO.IsNull() && !config.AttributesWO.IsUnknown() && len(config.AttributesWO.Elements()) > 0
	versionSet := !config.AttributesWOVer.IsNull() && !config.AttributesWOVer.IsUnknown()

	if woSet && !versionSet {
		resp.Diagnostics.AddAttributeError(
			path.Root("attributes_wo"),
			"Missing attributes_wo_version",
			"`attributes_wo` requires `attributes_wo_version` to be set. Write-only values are only "+
				"written to the LDAP server when `attributes_wo_version` changes, so without it the values "+
				"would be silently ignored after creation.",
		)
	}

	if versionSet && !woSet {
		resp.Diagnostics.AddAttributeError(
			path.Root("attributes_wo_version"),
			"Missing attributes_wo",
			"`attributes_wo_version` requires `attributes_wo` to be set with the values to write.",
		)
	}
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

	if diags := cancelledFromContext(ctx); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	diags := unmarshalTerraformAttributes(ctx, &plan.Attributes, attributes)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(mergeWriteOnlyAttributes(ctx, config.AttributesWO, attributes)...)
	if resp.Diagnostics.HasError() {
		return
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

	if diags := cancelledFromContext(ctx); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	var attrsMap map[string]types.Set
	diags := state.Attributes.ElementsAs(ctx, &attrsMap, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	for attrName, attrValue := range attrsMap {
		// Skip null attributes - they should not be read or refreshed
		if attrValue.IsNull() {
			continue
		}
		attributesToRequest = append(attributesToRequest, attrName)
	}

	// During import, state is empty, and we don't have access to the config.
	// Fall back to whatever ImportState recorded via private state.
	if len(attributesToRequest) == 0 {
		var importDiags diag.Diagnostics
		attributesToRequest, importDiags = resolveImportAttributes(ctx, req)
		resp.Diagnostics.Append(importDiags...)
	}

	sr, err := LdapSearch(ctx, r.client, state.DN.ValueString(), "base", "(objectClass=*)", attributesToRequest)
	if err != nil {
		// The entry was deleted outside of Terraform. Drop it from state so
		// Terraform plans to recreate it instead of erroring out.
		var ldapErr *ldap.Error
		if errors.As(err, &ldapErr) && ldapErr.ResultCode == ldap.LDAPResultNoSuchObject {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading LDAP entry",
			fmt.Sprintf("Unable to read LDAP entry %s: %s", state.DN.ValueString(), err),
		)
		return
	}

	results, diags := MarshalLdapResults(ctx, sr, attributesToRequest)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError(
			"Error reading LDAP entry",
			fmt.Sprintf("Unable to marshal LDAP results for %s", state.DN.ValueString()),
		)
		return
	}
	if len(results) == 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	entry := results[0]

	// Null attributes are not read or managed, but their keys must be
	// preserved in state. Otherwise the refreshed state map would be missing
	// keys that the configuration still declares as null, producing a perpetual
	// in-place diff (config null vs. state key absent) on every plan.
	refreshedAttrs := make(map[string]types.Set)
	diags = entry.Attributes.ElementsAs(ctx, &refreshedAttrs, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	for key, attrValue := range attrsMap {
		if attrValue.IsNull() {
			if _, ok := refreshedAttrs[key]; !ok {
				refreshedAttrs[key] = types.SetNull(types.StringType)
			}
		}
	}

	state.Attributes, diags = types.MapValueFrom(ctx, types.SetType{ElemType: types.StringType}, refreshedAttrs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
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

	// desiredAttrs is the full set of attributes the entry should have after this
	// update, as declared by the plan (and, below, any write-only attributes).
	desiredAttrs := make(map[string][]string)

	if diags := cancelledFromContext(ctx); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	diags := unmarshalTerraformAttributes(ctx, &plan.Attributes, desiredAttrs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	versionChanged := !plan.AttributesWOVer.Equal(state.AttributesWOVer)

	// Convert write-only attributes from config only if version changed
	if versionChanged && !config.AttributesWO.IsNull() {
		resp.Diagnostics.Append(mergeWriteOnlyAttributes(ctx, config.AttributesWO, desiredAttrs)...)
		if resp.Diagnostics.HasError() {
			return
		}

		// Special handling for unicodePwd attribute (Active Directory)
		resp.Diagnostics.Append(ProcessUnicodePwd(desiredAttrs)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// currentAttrs is what the entry is believed to have before this update,
	// needed to diff against desiredAttrs and build the LDAP replace/delete ops below.
	currentAttrs := make(map[string][]string)
	diags = unmarshalTerraformAttributes(ctx, &state.Attributes, currentAttrs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create LDAP modify request
	modifyReq := ldap.NewModifyRequest(plan.DN.ValueString(), nil)

	// Update changed attributes
	for key, newValues := range desiredAttrs {
		if currentValues, exists := currentAttrs[key]; !exists || !stringSlicesEqual(currentValues, newValues) {
			if len(newValues) == 0 {
				// Delete attribute if it exists in LDAP
				// Check state first (fast path), then check LDAP (for null → [] transitions)
				shouldDelete := exists
				if !shouldDelete {
					// Attribute not in state - check if it exists in LDAP.
					// This handles null → [] transitions where the attribute exists but
					// wasn't tracked. Only existence matters here, so the attribute's
					// current values are discarded.
					existsInLDAP, _, err := AttributeExistsInLDAP(ctx, r.client, plan.DN.ValueString(), key)
					if err != nil {
						resp.Diagnostics.AddError(
							"Error checking LDAP attribute existence",
							fmt.Sprintf("Unable to check if attribute %s exists for %s: %s", key, plan.DN.ValueString(), err),
						)
						return
					}
					shouldDelete = existsInLDAP
				}

				if shouldDelete {
					modifyReq.Delete(key, nil)
				}
			} else {
				modifyReq.Replace(key, newValues)
			}
		}
	}

	// Remove attributes that are no longer present
	for key := range currentAttrs {
		if _, exists := desiredAttrs[key]; !exists {
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

	if diags := cancelledFromContext(ctx); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	err := r.client.Del(delReq)
	if err != nil {
		// The entry is already gone (e.g. deleted outside of Terraform);
		// deleting is idempotent, so treat this as success.
		var ldapErr *ldap.Error
		if errors.As(err, &ldapErr) && ldapErr.ResultCode == ldap.LDAPResultNoSuchObject {
			return
		}
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

// Helper function to compare string slices as sets (order-independent).
// LDAP multi-valued attributes are unordered, so we need to compare them as sets.
// Used when diffing plan against state values before issuing LDAP modify ops.
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
// Null values are ignored and not included in the output map.
func unmarshalTerraformAttributes(ctx context.Context, tfMap *types.Map, attrs map[string][]string) diag.Diagnostics {
	var diag diag.Diagnostics
	attrsMap := make(map[string]types.Set)

	diags := tfMap.ElementsAs(ctx, &attrsMap, false)
	diag.Append(diags...)
	if diag.HasError() {
		return diag
	}

	for key, valueList := range attrsMap {
		// Skip null values - they should be ignored
		if valueList.IsNull() {
			continue
		}

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

// mergeWriteOnlyAttributes unmarshals non-null write-only attribute values from woMap
// into attrs. It is a no-op if woMap is null, matching the behavior of simply skipping
// the unmarshal call when there are no write-only values to merge.
func mergeWriteOnlyAttributes(ctx context.Context, woMap types.Map, attrs map[string][]string) diag.Diagnostics {
	if woMap.IsNull() {
		return nil
	}
	return unmarshalTerraformAttributes(ctx, &woMap, attrs)
}

// resolveImportAttributes determines which attributes to fetch when Read runs with no
// tracked state attributes (i.e. immediately after import, before config is available).
// It prefers the attribute list ImportState recorded in private state, falling back to
// objectClass alone if none was recorded.
func resolveImportAttributes(ctx context.Context, req resource.ReadRequest) ([]string, diag.Diagnostics) {
	privateData, diags := req.Private.GetKey(ctx, "import_attributes")

	if len(privateData) > 0 {
		var importData map[string][]string
		if err := json.Unmarshal(privateData, &importData); err == nil {
			if attrs, ok := importData["import_attributes"]; ok && len(attrs) > 0 {
				return attrs, diags
			}
		}
	}

	return []string{"objectClass"}, diags
}
