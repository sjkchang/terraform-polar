// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	polargo "github.com/polarsource/polar-go"
	"github.com/polarsource/polar-go/retry"
)

// Ensure PolarProvider satisfies the provider interface.
var _ provider.Provider = &PolarProvider{}

// PolarProvider defines the provider implementation.
type PolarProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// PolarProviderModel describes the provider data model.
type PolarProviderModel struct {
	AccessToken types.String `tfsdk:"access_token"`
	Server      types.String `tfsdk:"server"`
}

// PolarProviderData wraps the SDK client with additional provider-level
// configuration needed for supplemental raw HTTP calls (SDK gaps).
type PolarProviderData struct {
	Client      *polargo.Polar
	AccessToken string
	ServerURL   string
}

func (p *PolarProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "polar"
	resp.Version = p.version
}

func (p *PolarProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The Polar provider enables Terraform to manage [Polar.sh](https://polar.sh) resources such as products, meters, benefits, and webhook endpoints.",
		Attributes: map[string]schema.Attribute{
			"access_token": schema.StringAttribute{
				MarkdownDescription: "Polar organization access token. Can also be set with the `POLAR_ACCESS_TOKEN` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"server": schema.StringAttribute{
				MarkdownDescription: "The Polar environment to use. Must be `production` or `sandbox`. Defaults to `sandbox`.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("production", "sandbox"),
				},
			},
		},
	}
}

func (p *PolarProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data PolarProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Resolve access token: config value takes precedence over env var
	accessToken := data.AccessToken.ValueString()
	if accessToken == "" {
		accessToken = os.Getenv("POLAR_ACCESS_TOKEN")
	}

	if accessToken == "" {
		resp.Diagnostics.AddError(
			"Missing Polar Access Token",
			"The provider requires a Polar organization access token. "+
				"Set it in the provider configuration or via the POLAR_ACCESS_TOKEN environment variable.",
		)
		return
	}

	// Build SDK client options
	opts := []polargo.SDKOption{
		polargo.WithSecurity(accessToken),
	}

	// Default to sandbox; use production if explicitly set
	server := polargo.ServerSandbox
	if !data.Server.IsNull() && !data.Server.IsUnknown() && data.Server.ValueString() == "production" {
		server = polargo.ServerProduction
	}
	opts = append(opts, polargo.WithServer(server))

	// Retry on 429/5xx with exponential backoff
	opts = append(opts, polargo.WithRetryConfig(retry.Config{
		Strategy: "backoff",
		Backoff: &retry.BackoffStrategy{
			InitialInterval: 500,
			MaxInterval:     30000,
			Exponent:        1.5,
			MaxElapsedTime:  120000,
		},
		RetryConnectionErrors: false,
	}))

	client := polargo.New(opts...)

	// Resolve the base URL for supplemental raw HTTP calls
	serverURL := "https://sandbox-api.polar.sh"
	if server == polargo.ServerProduction {
		serverURL = "https://api.polar.sh"
	}

	providerData := &PolarProviderData{
		Client:      client,
		AccessToken: accessToken,
		ServerURL:   serverURL,
	}
	resp.DataSourceData = providerData
	resp.ResourceData = providerData
}

func (p *PolarProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewWebhookEndpointResource,
		NewMeterResource,
		NewBenefitResource,
		NewProductResource,
		NewOrganizationResource,
	}
}

func (p *PolarProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewMeterDataSource,
		NewBenefitDataSource,
		NewOrganizationDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &PolarProvider{
			version: version,
		}
	}
}
