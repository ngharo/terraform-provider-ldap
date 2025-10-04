// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/go-ldap/ldap/v3"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &LdapSearchDataSource{}

func NewLdapSearchDataSource() datasource.DataSource {
	return &LdapSearchDataSource{}
}

// LdapSearchDataSource defines the data source implementation.
type LdapSearchDataSource struct {
	conn *ldap.Conn
}

// LdapSearchDataSourceModel describes the data source data model.
type LdapSearchDataSourceModel struct {
	BaseDN              types.String `tfsdk:"basedn"`
	Scope               types.String `tfsdk:"scope"`
	Filter              types.String `tfsdk:"filter"`
	RequestedAttributes types.List   `tfsdk:"requested_attributes"`
	Results             types.List   `tfsdk:"results"`
}

// LdapSearchResultModel describes a single search result.
type LdapSearchResultModel struct {
	DN         types.String `tfsdk:"dn"`
	Attributes types.Map    `tfsdk:"attributes"`
}

func (d *LdapSearchDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_search"
}

func (d *LdapSearchDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "LDAP search data source that performs searches similar to the ldapsearch command line tool.",

		Attributes: map[string]schema.Attribute{
			"basedn": schema.StringAttribute{
				MarkdownDescription: "Specifies the base DN that should be used for the search.",
				Required:            true,
			},
			"scope": schema.StringAttribute{
				MarkdownDescription: "Specifies the scope that to use for search requests. The value should be one of 'base', 'one', or 'sub'. If this argument is not provided, a default of 'sub' will be used.",
				Optional:            true,
			},
			"filter": schema.StringAttribute{
				MarkdownDescription: "Specifies a filter to use when processing a search.",
				Required:            true,
			},
			"requested_attributes": schema.ListAttribute{
				MarkdownDescription: "Specifies which attribute(s) should be included in entries that match the search criteria. The value may be an attribute name or OID, a special token like '*' to indicate all user attributes or '+' to indicate all operational attributes, or an object class name prefixed by an '@' symbol to indicate all attributes associated with the specified object class. Multiple attributes may be requested.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"results": schema.ListNestedAttribute{
				MarkdownDescription: "A list of search results. Each result contains the DN and attributes.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"dn": schema.StringAttribute{
							MarkdownDescription: "The distinguished name of the entry.",
							Computed:            true,
						},
						"attributes": schema.MapAttribute{
							MarkdownDescription: "The attributes of the entry with their values.",
							Computed:            true,
							ElementType:         types.ListType{ElemType: types.StringType},
						},
					},
				},
			},
		},
	}
}

func (d *LdapSearchDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	conn, ok := req.ProviderData.(*ldap.Conn)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *ldap.Conn, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.conn = conn
}

func (d *LdapSearchDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data LdapSearchDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Set default scope if not provided
	scope := "sub"
	if !data.Scope.IsNull() {
		scope = data.Scope.ValueString()
	}

	// Convert scope string to ldap scope constant
	var ldapScope int
	switch scope {
	case "base":
		ldapScope = ldap.ScopeBaseObject
	case "one":
		ldapScope = ldap.ScopeSingleLevel
	case "sub":
		ldapScope = ldap.ScopeWholeSubtree
	default:
		resp.Diagnostics.AddError(
			"Invalid Scope",
			fmt.Sprintf("Scope must be one of 'base', 'one', or 'sub', got: %s", scope),
		)
		return
	}

	// Get requested attributes
	var attributes []string
	if !data.RequestedAttributes.IsNull() {
		resp.Diagnostics.Append(data.RequestedAttributes.ElementsAs(ctx, &attributes, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Perform LDAP search
	searchRequest := ldap.NewSearchRequest(
		data.BaseDN.ValueString(),
		ldapScope,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		data.Filter.ValueString(),
		attributes,
		nil,
	)

	searchResult, err := d.conn.Search(searchRequest)
	if err != nil {
		resp.Diagnostics.AddError(
			"LDAP Search Error",
			fmt.Sprintf("Unable to perform LDAP search: %s", err),
		)
		return
	}

	// Convert search results to Terraform types
	results := make([]LdapSearchResultModel, 0, len(searchResult.Entries))
	for _, entry := range searchResult.Entries {
		// Create attributes map
		attributes := make(map[string][]string)
		for _, attr := range entry.Attributes {
			attributes[attr.Name] = attr.Values
		}

		// Convert attributes to types.Map
		attributesMap, diags := types.MapValueFrom(ctx, types.ListType{ElemType: types.StringType}, attributes)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		result := LdapSearchResultModel{
			DN:         types.StringValue(entry.DN),
			Attributes: attributesMap,
		}

		results = append(results, result)
	}

	// Convert to types.List
	resultsList, diags := types.ListValueFrom(ctx, types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"dn":         types.StringType,
			"attributes": types.MapType{ElemType: types.ListType{ElemType: types.StringType}},
		},
	}, results)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Results = resultsList

	// Set computed scope value
	data.Scope = types.StringValue(scope)

	// Write logs using the tflog package
	tflog.Trace(ctx, fmt.Sprintf("performed LDAP search with base DN: %s, scope: %s, filter: %s",
		data.BaseDN.ValueString(), scope, data.Filter.ValueString()))

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
