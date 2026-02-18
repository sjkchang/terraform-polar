// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/polarsource/polar-go"
)

var _ datasource.DataSource = &BenefitDataSource{}

func NewBenefitDataSource() datasource.DataSource {
	return &BenefitDataSource{}
}

type BenefitDataSource struct {
	client *polargo.Polar
}

type BenefitDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Type        types.String `tfsdk:"type"`
	Description types.String `tfsdk:"description"`
}

func (d *BenefitDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_benefit"
}

func (d *BenefitDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a Polar benefit by ID. Use this to reference an unmanaged benefit (e.g. created in the dashboard) from a managed product's `benefit_ids`.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The benefit ID.",
				Required:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "The benefit type (`custom`, `discord`, `github_repository`, `downloadables`, `license_keys`, `meter_credit`).",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "The description of the benefit.",
				Computed:            true,
			},
		},
	}
}

func (d *BenefitDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if pd := extractProviderData(req.ProviderData, &resp.Diagnostics); pd != nil {
		d.client = pd.Client
	}
}

func (d *BenefitDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data BenefitDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := d.client.Benefits.Get(ctx, data.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.Diagnostics.AddError(
				"Benefit not found",
				fmt.Sprintf("No benefit found with ID %s.", data.ID.ValueString()),
			)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading benefit",
			fmt.Sprintf("Could not read benefit %s: %s", data.ID.ValueString(), err),
		)
		return
	}

	b := result.Benefit
	data.ID = types.StringValue(benefitID(*b))
	switch {
	case b.BenefitCustom != nil:
		data.Type = types.StringValue("custom")
		data.Description = types.StringValue(b.BenefitCustom.Description)
	case b.BenefitDiscord != nil:
		data.Type = types.StringValue("discord")
		data.Description = types.StringValue(b.BenefitDiscord.Description)
	case b.BenefitGitHubRepository != nil:
		data.Type = types.StringValue("github_repository")
		data.Description = types.StringValue(b.BenefitGitHubRepository.Description)
	case b.BenefitDownloadables != nil:
		data.Type = types.StringValue("downloadables")
		data.Description = types.StringValue(b.BenefitDownloadables.Description)
	case b.BenefitLicenseKeys != nil:
		data.Type = types.StringValue("license_keys")
		data.Description = types.StringValue(b.BenefitLicenseKeys.Description)
	case b.BenefitMeterCredit != nil:
		data.Type = types.StringValue("meter_credit")
		data.Description = types.StringValue(b.BenefitMeterCredit.Description)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
