// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-ldap/ldap/v3"
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

func NewLdapEntryResource() resource.Resource {
	return &LdapEntryResource{}
}

// LdapEntryResource defines the resource implementation.
type LdapEntryResource struct {
	client *ldap.Conn
}

// LdapEntryResourceModel describes the resource data model.
type LdapEntryResourceModel struct {
	DN              types.String `tfsdk:"dn"`
	ObjectClass     types.List   `tfsdk:"object_class"`
	Attributes      types.Map    `tfsdk:"attributes"`            // Map of List[String]
	AttributesWO    types.Map    `tfsdk:"attributes_wo"`         // Map of List[String] - write-only
	AttributesWOVer types.Int64  `tfsdk:"attributes_wo_version"` // Version trigger for attributes_wo
	Id              types.String `tfsdk:"id"`
}

func (r *LdapEntryResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_entry"
}

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
			"object_class": schema.ListAttribute{
				MarkdownDescription: "List of object classes for the LDAP entry. Object classes define the schema and required attributes for the entry. Common values include `person`, `organizationalPerson`, `inetOrgPerson`, `groupOfNames`, and `organizationalUnit`.",
				Required:            true,
				ElementType:         types.StringType,
			},
			"attributes": schema.MapAttribute{
				MarkdownDescription: "Map of LDAP attributes for the entry. The keys are attribute names and values are lists of attribute values. For single-valued attributes, provide a list with one element. For multi-valued attributes like `member` in groups, provide a list with multiple elements. Note that `objectClass` is automatically managed and should not be included here.",
				Optional:            true,
				ElementType:         types.ListType{ElemType: types.StringType},
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

func (r *LdapEntryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data LdapEntryResourceModel
	var configData LdapEntryResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read config for write-only attributes
	resp.Diagnostics.Append(req.Config.Get(ctx, &configData)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Convert object classes from List to []string
	var objectClasses []string
	diags := data.ObjectClass.ElementsAs(ctx, &objectClasses, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert attributes from Map to map[string][]string
	attributes := make(map[string][]string)
	if !data.Attributes.IsNull() {
		attrsMap := make(map[string]types.List)
		diags := data.Attributes.ElementsAs(ctx, &attrsMap, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		for key, valueList := range attrsMap {
			var values []string
			diags := valueList.ElementsAs(ctx, &values, false)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			attributes[key] = values
		}
	}

	// Convert write-only attributes from config (not plan)
	if !configData.AttributesWO.IsNull() {
		attrsWOMap := make(map[string]types.List)
		diags := configData.AttributesWO.ElementsAs(ctx, &attrsWOMap, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		for key, valueList := range attrsWOMap {
			var values []string
			diags := valueList.ElementsAs(ctx, &values, false)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}

			// Special handling for unicodePwd attribute (Active Directory)
			if strings.EqualFold(key, "unicodePwd") {
				encodedValues := make([]string, len(values))
				for i, val := range values {
					encoded, err := encodeUnicodePwd(val)
					if err != nil {
						resp.Diagnostics.AddError(
							"Error encoding unicodePwd",
							fmt.Sprintf("Unable to encode unicodePwd value: %s", err),
						)
						return
					}
					encodedValues[i] = encoded
				}
				attributes[key] = encodedValues
			} else {
				attributes[key] = values
			}
		}
	}

	// Add objectClass attribute
	attributes["objectClass"] = objectClasses

	// Create LDAP add request
	addReq := ldap.NewAddRequest(data.DN.ValueString(), nil)
	for attr, values := range attributes {
		addReq.Attribute(attr, values)
	}

	// Execute LDAP add operation
	err := r.client.Add(addReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating LDAP entry",
			fmt.Sprintf("Unable to create LDAP entry %s: %s", data.DN.ValueString(), err),
		)
		return
	}

	// Set ID to DN
	data.Id = data.DN

	// Write logs using the tflog package
	// Documentation: https://terraform.io/plugin/log
	tflog.Trace(ctx, fmt.Sprintf("created an LDAP entry: %s", data.Id))

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LdapEntryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data LdapEntryResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// FIXME FIXME FIXME
	// Build set of write-only attribute keys to exclude from state
	// Since Read doesn't have access to config, we need to track which attributes
	// were originally write-only. However, write-only attributes are never stored
	// in state by the framework, so we can't read them here. The framework handles
	// this automatically - write-only attributes won't appear in state.
	// But LDAP might return them, so we need to exclude common write-only attrs.
	writeOnlyKeys := make(map[string]bool)
	// Common password attributes that should not be read into state
	writeOnlyKeys["userpassword"] = true
	writeOnlyKeys["unicodepwd"] = true

	// Search for the LDAP entry
	searchReq := ldap.NewSearchRequest(
		data.DN.ValueString(),
		ldap.ScopeBaseObject,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		"(objectClass=*)",
		[]string{"*"},
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

	// Update object classes
	objectClasses := entry.GetAttributeValues("objectClass")
	objectClassList, objClassDiags := types.ListValueFrom(ctx, types.StringType, objectClasses)
	resp.Diagnostics.Append(objClassDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.ObjectClass = objectClassList

	// Update attributes (excluding objectClass and write-only attributes)
	attrsMap := make(map[string][]string)
	for _, attr := range entry.Attributes {
		attrNameLower := strings.ToLower(attr.Name)
		// Exclude objectClass and any write-only attributes
		if attrNameLower != "objectclass" && !writeOnlyKeys[attrNameLower] {
			attrsMap[attr.Name] = attr.Values
		}
	}

	attributesMap, attrsDiags := types.MapValueFrom(ctx, types.ListType{ElemType: types.StringType}, attrsMap)
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
	var data LdapEntryResourceModel
	var configData LdapEntryResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read config for write-only attributes
	resp.Diagnostics.Append(req.Config.Get(ctx, &configData)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Get current state for comparison
	var currentData LdapEntryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &currentData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert new object classes from List to []string
	var newObjectClasses []string
	diags := data.ObjectClass.ElementsAs(ctx, &newObjectClasses, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert new attributes from Map to map[string][]string
	newAttrsMap := make(map[string][]string)
	if !data.Attributes.IsNull() {
		attrsMap := make(map[string]types.List)
		diags := data.Attributes.ElementsAs(ctx, &attrsMap, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		for key, valueList := range attrsMap {
			var values []string
			diags := valueList.ElementsAs(ctx, &values, false)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			newAttrsMap[key] = values
		}
	}

	// Check if attributes_wo_version has changed
	versionChanged := !data.AttributesWOVer.Equal(currentData.AttributesWOVer)

	// Convert write-only attributes from config (not plan) only if version changed
	if versionChanged && !configData.AttributesWO.IsNull() {
		attrsWOMap := make(map[string]types.List)
		diags := configData.AttributesWO.ElementsAs(ctx, &attrsWOMap, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		for key, valueList := range attrsWOMap {
			var values []string
			diags := valueList.ElementsAs(ctx, &values, false)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}

			// Special handling for unicodePwd attribute (Active Directory)
			if strings.EqualFold(key, "unicodePwd") {
				encodedValues := make([]string, len(values))
				for i, val := range values {
					encoded, err := encodeUnicodePwd(val)
					if err != nil {
						resp.Diagnostics.AddError(
							"Error encoding unicodePwd",
							fmt.Sprintf("Unable to encode unicodePwd value: %s", err),
						)
						return
					}
					encodedValues[i] = encoded
				}
				newAttrsMap[key] = encodedValues
			} else {
				newAttrsMap[key] = values
			}
		}
	}

	// Get current object classes and attributes for comparison
	var currentObjectClasses []string
	diags = currentData.ObjectClass.ElementsAs(ctx, &currentObjectClasses, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	currentAttrsMap := make(map[string][]string)
	if !currentData.Attributes.IsNull() {
		attrsMap := make(map[string]types.List)
		diags := currentData.Attributes.ElementsAs(ctx, &attrsMap, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		for key, valueList := range attrsMap {
			var values []string
			diags := valueList.ElementsAs(ctx, &values, false)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			currentAttrsMap[key] = values
		}
	}

	// Create LDAP modify request
	modifyReq := ldap.NewModifyRequest(data.DN.ValueString(), nil)

	// Update objectClass if changed
	if !stringSlicesEqual(currentObjectClasses, newObjectClasses) {
		modifyReq.Replace("objectClass", newObjectClasses)
	}

	// Update attributes
	for key, newValues := range newAttrsMap {
		if currentValues, exists := currentAttrsMap[key]; !exists || !stringSlicesEqual(currentValues, newValues) {
			modifyReq.Replace(key, newValues)
		}
	}

	// Remove attributes that are no longer present
	for key := range currentAttrsMap {
		if _, exists := newAttrsMap[key]; !exists {
			modifyReq.Delete(key, nil)
		}
	}

	// Execute LDAP modify operation if there are changes
	if len(modifyReq.Changes) > 0 {
		err := r.client.Modify(modifyReq)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error updating LDAP entry",
				fmt.Sprintf("Unable to update LDAP entry %s: %s", data.DN.ValueString(), err),
			)
			return
		}
	}

	// Set ID to DN
	data.Id = data.DN

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *LdapEntryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data LdapEntryResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create LDAP delete request
	delReq := ldap.NewDelRequest(data.DN.ValueString(), nil)

	// Execute LDAP delete operation
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
	resource.ImportStatePassthroughID(ctx, path.Root("dn"), req, resp)
}

// Helper function to compare string slices.
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
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
