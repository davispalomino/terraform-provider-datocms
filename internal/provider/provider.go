// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
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
	APIToken  types.String `tfsdk:"api_token"`
	APITokens types.Map    `tfsdk:"api_tokens"`
	BaseURL   types.String `tfsdk:"base_url"`
}

// DatoCMSClient holds the configured API client data shared with resources
// and data sources. APIToken is the default token, used by resources that do
// not set the project attribute; APITokens maps project keys (as declared in
// the api_tokens provider attribute) to their tokens.
type DatoCMSClient struct {
	APIToken   string
	APITokens  map[string]string
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
				MarkdownDescription: "Default DatoCMS API token, used by resources that do not set the `project` attribute. Can also be set via the `DATOCMS_API_TOKEN` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"api_tokens": schema.MapAttribute{
				ElementType:         types.StringType,
				MarkdownDescription: "Map of project keys to DatoCMS API tokens, to manage multiple DatoCMS projects from a single provider configuration. Resources select an entry through their `project` attribute; the map key is an arbitrary label of your choice (for example the project name). Resources without a `project` attribute keep using `api_token`/`DATOCMS_API_TOKEN`.",
				Optional:            true,
				Sensitive:           true,
			},
			"base_url": schema.StringAttribute{
				MarkdownDescription: "Base URL of the DatoCMS API. Defaults to `" + defaultBaseURL + "`. Must use the `https` scheme; `http` is only allowed for `localhost`/`127.0.0.1` (local testing).",
				Optional:            true,
			},
		},
	}
}

// validateBaseURL ensures the configured base URL uses https, allowing plain
// http only for localhost/127.0.0.1 (local testing).
func validateBaseURL(baseURL string) error {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("could not parse %q as a URL: %w", baseURL, err)
	}

	switch parsed.Scheme {
	case "https":
		return nil
	case "http":
		host := parsed.Hostname()
		if host == "localhost" || host == "127.0.0.1" {
			return nil
		}
		return fmt.Errorf("base_url %q must use the https scheme; http is only allowed for localhost/127.0.0.1", baseURL)
	default:
		return fmt.Errorf("base_url %q must use the https scheme (http is only allowed for localhost/127.0.0.1)", baseURL)
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

	apiTokens := map[string]string{}
	if !data.APITokens.IsNull() && !data.APITokens.IsUnknown() {
		resp.Diagnostics.Append(data.APITokens.ElementsAs(ctx, &apiTokens, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	if apiToken == "" && len(apiTokens) == 0 {
		resp.Diagnostics.AddError(
			"Missing DatoCMS API token",
			"Set the api_token provider attribute (or the DATOCMS_API_TOKEN environment variable), or configure per-project tokens with the api_tokens attribute.",
		)
		return
	}

	baseURL := defaultBaseURL
	if !data.BaseURL.IsNull() {
		baseURL = data.BaseURL.ValueString()
	}

	if err := validateBaseURL(baseURL); err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("base_url"),
			"Invalid base_url",
			err.Error(),
		)
		return
	}

	client := &DatoCMSClient{
		APIToken:   apiToken,
		APITokens:  apiTokens,
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
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
