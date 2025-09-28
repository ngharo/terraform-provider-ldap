// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/go-ldap/ldap/v3"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure LdapProvider satisfies various provider interfaces.
var _ provider.Provider = &LdapProvider{}
var _ provider.ProviderWithFunctions = &LdapProvider{}
var _ provider.ProviderWithEphemeralResources = &LdapProvider{}

// LdapProvider defines the provider implementation.
type LdapProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// LdapProviderModel describes the provider data model.
type LdapProviderModel struct {
	Host     types.String `tfsdk:"host"`
	Port     types.Int64  `tfsdk:"port"`
	BindDN   types.String `tfsdk:"bind_dn"`
	BindPW   types.String `tfsdk:"bind_password"`
	UseTLS   types.Bool   `tfsdk:"use_tls"`
}

func (p *LdapProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "ldap"
	resp.Version = p.version
}

func (p *LdapProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				MarkdownDescription: "LDAP server hostname or IP address",
				Optional:            true,
			},
			"port": schema.Int64Attribute{
				MarkdownDescription: "LDAP server port (default: 389 for LDAP, 636 for LDAPS)",
				Optional:            true,
			},
			"bind_dn": schema.StringAttribute{
				MarkdownDescription: "Distinguished name for binding to LDAP server",
				Optional:            true,
			},
			"bind_password": schema.StringAttribute{
				MarkdownDescription: "Password for binding to LDAP server",
				Optional:            true,
				Sensitive:           true,
			},
			"use_tls": schema.BoolAttribute{
				MarkdownDescription: "Use TLS for LDAP connection",
				Optional:            true,
			},
		},
	}
}

func (p *LdapProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data LdapProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Set default values
	host := "localhost"
	port := int64(389)
	bindDN := ""
	bindPW := ""
	useTLS := false

	if !data.Host.IsNull() {
		host = data.Host.ValueString()
	}
	if !data.Port.IsNull() {
		port = data.Port.ValueInt64()
	}
	if !data.BindDN.IsNull() {
		bindDN = data.BindDN.ValueString()
	}
	if !data.BindPW.IsNull() {
		bindPW = data.BindPW.ValueString()
	}
	if !data.UseTLS.IsNull() {
		useTLS = data.UseTLS.ValueBool()
	}

	// Create LDAP connection
	var conn *ldap.Conn
	var err error

	if useTLS {
		conn, err = ldap.DialTLS("tcp", fmt.Sprintf("%s:%d", host, port), nil)
	} else {
		conn, err = ldap.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
	}

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to connect to LDAP server",
			fmt.Sprintf("Error connecting to LDAP server %s:%d: %s", host, port, err),
		)
		return
	}

	// Bind to LDAP server if credentials provided
	if bindDN != "" {
		err = conn.Bind(bindDN, bindPW)
		if err != nil {
			conn.Close()
			resp.Diagnostics.AddError(
				"Unable to bind to LDAP server",
				fmt.Sprintf("Error binding to LDAP server with DN %s: %s", bindDN, err),
			)
			return
		}
	}

	// Provide LDAP connection to resources and data sources
	resp.DataSourceData = conn
	resp.ResourceData = conn
}

func (p *LdapProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewExampleResource,
		NewLdapEntryResource,
	}
}

func (p *LdapProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{
		NewExampleEphemeralResource,
	}
}

func (p *LdapProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewExampleDataSource,
	}
}

func (p *LdapProvider) Functions(ctx context.Context) []func() function.Function {
	return []func() function.Function{
		NewExampleFunction,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &LdapProvider{
			version: version,
		}
	}
}
