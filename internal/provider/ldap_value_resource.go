// Copyright (c) ngharo <root@ngha.ro>
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/go-ldap/ldap/v3"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &LdapValueResource{}
var _ resource.ResourceWithImportState = &LdapValueResource{}

func NewLdapValueResource() resource.Resource {
	return &LdapValueResource{}
}

// LdapValueResource defines the resource implementation for asserting the
// presence of a single value within a multi-valued LDAP attribute.
type LdapValueResource struct {
	client *ldap.Conn
}

// LdapValueResourceModel describes the resource data model for LDAP values.
type LdapValueResourceModel struct {
	DN        types.String `tfsdk:"dn"`
	Attribute types.String `tfsdk:"attribute"`
	Value     types.String `tfsdk:"value"`
	Id        types.String `tfsdk:"id"`
}

func (r *LdapValueResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_value"
}

func (r *LdapValueResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Asserts that a single value is present in a multi-valued LDAP attribute, without taking ownership of the attribute's other values.

The common use case is asserting membership in a group (e.g. adding a single ` + "`member`" + ` DN to a ` + "`groupOfNames`" + ` entry) without conflicting with other ` + "`ldap_value`" + ` resources, or externally managed values, asserting membership in the same attribute.

On create, the value is added to the attribute if not already present. On delete, only that specific value is removed from the attribute; the attribute and its other values are left untouched. If the asserted value is externally removed from the attribute, this resource will detect the drift and plan to re-add it.

Changing ` + "`dn`" + `, ` + "`attribute`" + `, or ` + "`value`" + ` forces a new resource to be created.

### Do not combine with ` + "`ldap_entry`" + ` on the same attribute
` + "`ldap_entry`" + ` treats its ` + "`attributes`" + ` map as the complete, authoritative set of values for each attribute it declares. If an ` + "`ldap_entry`" + ` resource and one or more ` + "`ldap_value`" + ` resources both target the same DN and attribute, ` + "`ldap_entry`" + ` will detect the values added by ` + "`ldap_value`" + ` as drift and plan to remove them on every apply. Point ` + "`ldap_value`" + ` at attributes that no ` + "`ldap_entry`" + ` resource declares (for example, a group entry provisioned outside of this attribute's management, or in a separate module/workspace).

Additionally, LDAP servers reject removing the last remaining value of an attribute required by the entry's object class (e.g. ` + "`groupOfNames.member`" + `). Ensure at least one value (managed elsewhere) always remains, or deletion of the final ` + "`ldap_value`" + ` will fail with an object class violation.`,

		Attributes: map[string]schema.Attribute{
			"dn": schema.StringAttribute{
				MarkdownDescription: "The distinguished name (DN) of the LDAP entry containing the attribute. Changing this forces a new resource to be created.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"attribute": schema.StringAttribute{
				MarkdownDescription: "The name of the multi-valued LDAP attribute (e.g. `member`). Changing this forces a new resource to be created.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"value": schema.StringAttribute{
				MarkdownDescription: "The value that must be present in the attribute (e.g. the DN of a group member). Changing this forces a new resource to be created.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier for this resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *LdapValueResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = GetLdapConnection(req.ProviderData, &resp.Diagnostics, "Resource")
}

// Create adds the value to the attribute if it is not already present.
func (r *LdapValueResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan LdapValueResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	dn := plan.DN.ValueString()
	attribute := plan.Attribute.ValueString()
	value := plan.Value.ValueString()

	modifyReq := ldap.NewModifyRequest(dn, nil)
	modifyReq.Add(attribute, []string{value})

	err := r.client.Modify(modifyReq)
	if err != nil {
		var ldapErr *ldap.Error
		if errors.As(err, &ldapErr) && ldapErr.ResultCode == ldap.LDAPResultAttributeOrValueExists {
			tflog.Trace(ctx, fmt.Sprintf("value %q already present in attribute %q on %s", value, attribute, dn))
		} else {
			resp.Diagnostics.AddError(
				"Error adding LDAP value",
				fmt.Sprintf("Unable to add value %q to attribute %q on %s: %s", value, attribute, dn, err),
			)
			return
		}
	}

	plan.Id = types.StringValue(buildLdapValueId(dn, attribute, value))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read checks whether the value is still present in the attribute.
func (r *LdapValueResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state LdapValueResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	dn := state.DN.ValueString()
	attribute := state.Attribute.ValueString()
	value := state.Value.ValueString()

	sr, err := LdapSearch(r.client, dn, "base", "(objectClass=*)", []string{attribute})
	if err != nil {
		var ldapErr *ldap.Error
		if errors.As(err, &ldapErr) && ldapErr.ResultCode == ldap.LDAPResultNoSuchObject {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading LDAP value",
			fmt.Sprintf("Unable to read attribute %q on %s: %s", attribute, dn, err),
		)
		return
	}

	if len(sr.Entries) == 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	var values []string
	for _, attr := range sr.Entries[0].Attributes {
		if attr.Name == attribute {
			values = attr.Values
			break
		}
	}

	if !slices.Contains(values, value) {
		tflog.Trace(ctx, fmt.Sprintf("value %q no longer present in attribute %q on %s", value, attribute, dn))
		resp.State.RemoveResource(ctx)
		return
	}

	state.Id = types.StringValue(buildLdapValueId(dn, attribute, value))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is only invoked for computed attribute changes since dn, attribute,
// and value all require replacement.
func (r *LdapValueResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan LdapValueResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.Id = types.StringValue(buildLdapValueId(plan.DN.ValueString(), plan.Attribute.ValueString(), plan.Value.ValueString()))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete removes only the asserted value from the attribute, leaving any
// other values in place.
func (r *LdapValueResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state LdapValueResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	dn := state.DN.ValueString()
	attribute := state.Attribute.ValueString()
	value := state.Value.ValueString()

	modifyReq := ldap.NewModifyRequest(dn, nil)
	modifyReq.Delete(attribute, []string{value})

	err := r.client.Modify(modifyReq)
	if err != nil {
		var ldapErr *ldap.Error
		if errors.As(err, &ldapErr) &&
			(ldapErr.ResultCode == ldap.LDAPResultNoSuchObject || ldapErr.ResultCode == ldap.LDAPResultNoSuchAttribute) {
			// Entry or attribute value is already gone; nothing left to do.
			return
		}
		resp.Diagnostics.AddError(
			"Error removing LDAP value",
			fmt.Sprintf("Unable to remove value %q from attribute %q on %s: %s", value, attribute, dn, err),
		)
		return
	}
}

// ImportState expects a JSON object identifying the DN, attribute, and value,
// e.g. {"dn": "...", "attribute": "member", "value": "..."}.
func (r *LdapValueResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	var importSpec struct {
		DN        string `json:"dn"`
		Attribute string `json:"attribute"`
		Value     string `json:"value"`
	}

	if err := json.Unmarshal([]byte(req.ID), &importSpec); err != nil {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf(`Import ID must be a JSON object like {"dn": "...", "attribute": "...", "value": "..."}: %s`, err),
		)
		return
	}

	if importSpec.DN == "" || importSpec.Attribute == "" || importSpec.Value == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			`Import ID must include non-empty "dn", "attribute", and "value" fields`,
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("dn"), importSpec.DN)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("attribute"), importSpec.Attribute)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("value"), importSpec.Value)...)
}

// buildLdapValueId builds a human-readable, informational identifier for the resource.
func buildLdapValueId(dn, attribute, value string) string {
	return fmt.Sprintf("%s/%s=%s", dn, attribute, value)
}
