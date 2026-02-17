// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/polarsource/polar-go"
	"github.com/polarsource/polar-go/models/apierrors"
)

var _ datasource.DataSource = &BenefitDataSource{}

func NewBenefitDataSource() datasource.DataSource {
	return &BenefitDataSource{}
}

type BenefitDataSource struct {
	client *polargo.Polar
}

func (d *BenefitDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_benefit"
}

func (d *BenefitDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a Polar benefit by ID.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The benefit ID.",
				Required:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "The benefit type.",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "The description of the benefit.",
				Computed:            true,
			},
			"metadata": schema.MapAttribute{
				MarkdownDescription: "Key-value metadata.",
				Computed:            true,
				ElementType:         types.StringType,
			},

			// Type-specific properties (populated based on the benefit type)
			"custom_properties": schema.SingleNestedAttribute{
				MarkdownDescription: "Properties for `custom` type benefits.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"note": schema.StringAttribute{
						MarkdownDescription: "A note to display to the subscriber.",
						Computed:            true,
					},
				},
			},
			"discord_properties": schema.SingleNestedAttribute{
				MarkdownDescription: "Properties for `discord` type benefits.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"guild_token": schema.StringAttribute{
						MarkdownDescription: "The Discord bot token for the server.",
						Computed:            true,
						Sensitive:           true,
					},
					"role_id": schema.StringAttribute{
						MarkdownDescription: "The Discord role ID to grant.",
						Computed:            true,
					},
					"kick_member": schema.BoolAttribute{
						MarkdownDescription: "Whether to kick the member when the benefit is revoked.",
						Computed:            true,
					},
					"guild_id": schema.StringAttribute{
						MarkdownDescription: "The Discord server (guild) ID.",
						Computed:            true,
					},
				},
			},
			"github_repository_properties": schema.SingleNestedAttribute{
				MarkdownDescription: "Properties for `github_repository` type benefits.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"repository_owner": schema.StringAttribute{
						MarkdownDescription: "The GitHub repository owner.",
						Computed:            true,
					},
					"repository_name": schema.StringAttribute{
						MarkdownDescription: "The GitHub repository name.",
						Computed:            true,
					},
					"permission": schema.StringAttribute{
						MarkdownDescription: "The permission level granted.",
						Computed:            true,
					},
				},
			},
			"downloadables_properties": schema.SingleNestedAttribute{
				MarkdownDescription: "Properties for `downloadables` type benefits.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"files": schema.ListAttribute{
						MarkdownDescription: "List of file IDs available for download.",
						Computed:            true,
						ElementType:         types.StringType,
					},
				},
			},
			"license_keys_properties": schema.SingleNestedAttribute{
				MarkdownDescription: "Properties for `license_keys` type benefits.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"prefix": schema.StringAttribute{
						MarkdownDescription: "A prefix for generated license keys.",
						Computed:            true,
					},
					"limit_usage": schema.Int64Attribute{
						MarkdownDescription: "Maximum number of times a license key can be used.",
						Computed:            true,
					},
					"expires": schema.SingleNestedAttribute{
						MarkdownDescription: "Expiration settings for license keys.",
						Computed:            true,
						Attributes: map[string]schema.Attribute{
							"ttl": schema.Int64Attribute{
								MarkdownDescription: "Time-to-live value.",
								Computed:            true,
							},
							"timeframe": schema.StringAttribute{
								MarkdownDescription: "The timeframe unit.",
								Computed:            true,
							},
						},
					},
					"activations": schema.SingleNestedAttribute{
						MarkdownDescription: "Activation settings for license keys.",
						Computed:            true,
						Attributes: map[string]schema.Attribute{
							"limit": schema.Int64Attribute{
								MarkdownDescription: "Maximum number of activations.",
								Computed:            true,
							},
							"enable_customer_admin": schema.BoolAttribute{
								MarkdownDescription: "Whether the customer can manage their own activations.",
								Computed:            true,
							},
						},
					},
				},
			},
			"meter_credit_properties": schema.SingleNestedAttribute{
				MarkdownDescription: "Properties for `meter_credit` type benefits.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"meter_id": schema.StringAttribute{
						MarkdownDescription: "The ID of the meter to credit.",
						Computed:            true,
					},
					"units": schema.Int64Attribute{
						MarkdownDescription: "The number of units to credit.",
						Computed:            true,
					},
					"rollover": schema.BoolAttribute{
						MarkdownDescription: "Whether unused credits roll over to the next period.",
						Computed:            true,
					},
				},
			},
		},
	}
}

func (d *BenefitDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	pd, ok := req.ProviderData.(*PolarProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *PolarProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = pd.Client
}

func (d *BenefitDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data BenefitResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := d.client.Benefits.Get(ctx, data.ID.ValueString())
	if err != nil {
		var notFound *apierrors.ResourceNotFound
		if isNotFound(err, &notFound) {
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

	mapBenefitResponseToState(ctx, result.Benefit, &data, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
