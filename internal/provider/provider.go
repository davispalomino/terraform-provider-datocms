// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"net/http"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const defaultBaseURL = "https://site-api.datocms.com"

// Ensure DatoCMSProvider satisfies various provider interfaces.
var _ provider.Provider = &DatoCMSProvider{}

// DatoCMSProvider defines the provider implementation.
type DatoCMSProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// DatoCMSProviderModel describes the provider data model.
type DatoCMSProviderModel struct {
	APIToken types.String `tfsdk:"api_token"`
	BaseURL  types.String `tfsdk:"base_url"`
}

// DatoCMSClient holds the configured API client data shared with resources
// and data sources.
type DatoCMSClient struct {
	APIToken   string
	BaseURL    string
	HTTPClient *http.Client
}

func (p *DatoCMSProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "datocms"
	resp.Version = p.version
}

func (p *DatoCMSProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Interact with the DatoCMS Content Management API. All requests are sent with `X-Api-Version: 3` (the current CMA version); the resource schemas were validated against the official CMA hyperschema (https://site-api.datocms.com/docs/site-api-hyperschema.json) on 2026-08-20. DatoCMS introduces breaking changes only with a new API version, so a future API version may require a new release of this provider.",
		Attributes: map[string]schema.Attribute{
			"api_token": schema.StringAttribute{
				MarkdownDescription: "DatoCMS API token. Can also be set via the `DATOCMS_API_TOKEN` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"base_url": schema.StringAttribute{
				MarkdownDescription: "Base URL of the DatoCMS API. Defaults to `" + defaultBaseURL + "`.",
				Optional:            true,
			},
		},
	}
}

func (p *DatoCMSProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data DatoCMSProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	apiToken := os.Getenv("DATOCMS_API_TOKEN")
	if !data.APIToken.IsNull() {
		apiToken = data.APIToken.ValueString()
	}

	if apiToken == "" {
		resp.Diagnostics.AddError(
			"Missing DatoCMS API token",
			"Set the api_token provider attribute or the DATOCMS_API_TOKEN environment variable.",
		)
		return
	}

	baseURL := defaultBaseURL
	if !data.BaseURL.IsNull() {
		baseURL = data.BaseURL.ValueString()
	}

	client := &DatoCMSClient{
		APIToken:   apiToken,
		BaseURL:    baseURL,
		HTTPClient: http.DefaultClient,
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *DatoCMSProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewAccessTokenResource,
		NewRoleResource,
		NewWebhookResource,
	}
}

func (p *DatoCMSProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &DatoCMSProvider{
			version: version,
		}
	}
}
