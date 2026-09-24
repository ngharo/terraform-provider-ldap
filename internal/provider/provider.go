// Copyright (c) ngharo <root@ngha.ro>
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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

// Default timeouts applied when the user does not configure them explicitly.
// defaultDialTimeout matches go-ldap's package default; defaultRequestTimeout
// bounds each request on a stalled server (go-ldap defaults to no per-request
// timeout).
const (
	defaultDialTimeout    = 60 * time.Second
	defaultRequestTimeout = 60 * time.Second
)

// resolveDuration resolves a duration setting with precedence config >
// environment variable > default. Values are parsed with time.ParseDuration
// (e.g. "30s", "1m", "0s"). A value of 0 disables the timeout.
func resolveDuration(configVal types.String, envVar string, def time.Duration, settingName string) (time.Duration, diag.Diagnostics) {
	var diags diag.Diagnostics

	val := os.Getenv(envVar)
	if !configVal.IsNull() {
		val = configVal.ValueString()
	}
	if val == "" {
		return def, nil
	}

	parsed, err := time.ParseDuration(val)
	if err != nil {
		diags.AddError(
			fmt.Sprintf("Invalid %s", settingName),
			fmt.Sprintf("Invalid duration %q for %s (config attribute or %s environment variable): %s", val, settingName, envVar, err),
		)
		return 0, diags
	}
	if parsed < 0 {
		diags.AddError(
			fmt.Sprintf("Invalid %s", settingName),
			fmt.Sprintf("Duration for %s must be >= 0, got %s", settingName, parsed),
		)
		return 0, diags
	}

	return parsed, nil
}

// LdapProvider defines the provider implementation.
type LdapProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// LdapProviderModel describes the provider data model.
type LdapProviderModel struct {
	URL            types.String `tfsdk:"url"`
	BindDN         types.String `tfsdk:"bind_dn"`
	BindPW         types.String `tfsdk:"bind_password"`
	Insecure       types.Bool   `tfsdk:"insecure"`
	DialTimeout    types.String `tfsdk:"dial_timeout"`
	RequestTimeout types.String `tfsdk:"request_timeout"`
}

func (p *LdapProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "ldap"
	resp.Version = p.version
}

func (p *LdapProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The LDAP provider is used to interact with LDAP (Lightweight Directory Access Protocol) servers. It allows you to manage LDAP entries using Terraform.",
		Attributes: map[string]schema.Attribute{
			"url": schema.StringAttribute{
				MarkdownDescription: "LDAP server URL (e.g., `ldap://localhost:389` or `ldaps://localhost:636`). Can also be set via the `LDAP_URL` environment variable.",
				Required:            true,
			},
			"bind_dn": schema.StringAttribute{
				MarkdownDescription: "Distinguished name for binding to LDAP server. Can also be set via the `LDAP_BIND_DN` environment variable.",
				Optional:            true,
			},
			"bind_password": schema.StringAttribute{
				MarkdownDescription: "Password for binding to LDAP server. Can also be set via the `LDAP_BIND_PASSWORD` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"insecure": schema.BoolAttribute{
				MarkdownDescription: "Whether the server should be accessed without verifying the TLS certificate. Can also be set via the `LDAP_INSECURE` environment variable. Defaults to `false`.",
				Optional:            true,
			},
			"dial_timeout": schema.StringAttribute{
				MarkdownDescription: "Timeout for establishing the LDAP connection (e.g. `30s`). Can also be set via the `LDAP_DIAL_TIMEOUT` environment variable. Defaults to `60s`.",
				Optional:            true,
			},
			"request_timeout": schema.StringAttribute{
				MarkdownDescription: "Timeout for individual LDAP requests (bind, search, add, modify, delete). Can also be set via the `LDAP_REQUEST_TIMEOUT` environment variable. Defaults to `60s`. Set to `0s` to disable (not recommended).",
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

	// Set values with precedence: config > environment variable
	ldapURL := ""
	bindDN := ""
	bindPW := ""
	insecure := false

	// Check environment variables first
	if envURL := os.Getenv("LDAP_URL"); envURL != "" {
		ldapURL = envURL
	}
	if envBindDN := os.Getenv("LDAP_BIND_DN"); envBindDN != "" {
		bindDN = envBindDN
	}
	if envBindPW := os.Getenv("LDAP_BIND_PASSWORD"); envBindPW != "" {
		bindPW = envBindPW
	}
	if envInsecure := os.Getenv("LDAP_INSECURE"); envInsecure != "" {
		if val, err := strconv.ParseBool(envInsecure); err == nil {
			insecure = val
		}
	}

	// Override with config values if provided
	if !data.URL.IsNull() {
		ldapURL = data.URL.ValueString()
	}
	if !data.BindDN.IsNull() {
		bindDN = data.BindDN.ValueString()
	}
	if !data.BindPW.IsNull() {
		bindPW = data.BindPW.ValueString()
	}
	if !data.Insecure.IsNull() {
		insecure = data.Insecure.ValueBool()
	}

	// Connection and request timeouts. The request timeout is applied to every
	// bind/search/add/modify/delete issued on the connection; without it, go-ldap
	// leaves per-request timeouts disabled and a stalled server can hang Terraform
	// indefinitely.
	dialTimeout, diags := resolveDuration(data.DialTimeout, "LDAP_DIAL_TIMEOUT", defaultDialTimeout, "dial_timeout")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	requestTimeout, diags := resolveDuration(data.RequestTimeout, "LDAP_REQUEST_TIMEOUT", defaultRequestTimeout, "request_timeout")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: insecure,
	}

	conn, err := ldap.DialURL(
		ldapURL,
		ldap.DialWithDialer(&net.Dialer{Timeout: dialTimeout}),
		ldap.DialWithTLSConfig(tlsConfig),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to connect to LDAP server",
			fmt.Sprintf("Error connecting to LDAP server at %s: %s", ldapURL, err),
		)
		return
	}

	// Bound every subsequent bind/search/add/modify/delete.
	conn.SetTimeout(requestTimeout)

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
		NewLdapEntryResource,
		NewLdapValueResource,
	}
}

func (p *LdapProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{}
}

func (p *LdapProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewLdapSearchDataSource,
	}
}

func (p *LdapProvider) Functions(ctx context.Context) []func() function.Function {
	return []func() function.Function{}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &LdapProvider{
			version: version,
		}
	}
}
