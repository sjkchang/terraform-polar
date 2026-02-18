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

var _ datasource.DataSource = &MeterDataSource{}

func NewMeterDataSource() datasource.DataSource {
	return &MeterDataSource{}
}

type MeterDataSource struct {
	client *polargo.Polar
}

func (d *MeterDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_meter"
}

func (d *MeterDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a Polar meter by ID.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The meter ID.",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the meter.",
				Computed:            true,
			},
			"filter": schema.SingleNestedAttribute{
				MarkdownDescription: "Filter applied on incoming events.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"conjunction": schema.StringAttribute{
						MarkdownDescription: "Logical conjunction for combining clauses.",
						Computed:            true,
					},
					"clauses": schema.ListNestedAttribute{
						MarkdownDescription: "List of filter clauses.",
						Computed:            true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"property": schema.StringAttribute{
									MarkdownDescription: "The event property to filter on.",
									Computed:            true,
								},
								"operator": schema.StringAttribute{
									MarkdownDescription: "The comparison operator.",
									Computed:            true,
								},
								"value": schema.StringAttribute{
									MarkdownDescription: "The value to compare against.",
									Computed:            true,
								},
							},
						},
					},
				},
			},
			"aggregation": schema.SingleNestedAttribute{
				MarkdownDescription: "Aggregation function for the meter.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"func": schema.StringAttribute{
						MarkdownDescription: "The aggregation function.",
						Computed:            true,
					},
					"property": schema.StringAttribute{
						MarkdownDescription: "The event property to aggregate.",
						Computed:            true,
					},
				},
			},
			"metadata": schema.MapAttribute{
				MarkdownDescription: "Key-value metadata.",
				Computed:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (d *MeterDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if pd := extractProviderData(req.ProviderData, &resp.Diagnostics); pd != nil {
		d.client = pd.Client
	}
}

func (d *MeterDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data MeterResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := d.client.Meters.Get(ctx, data.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.Diagnostics.AddError(
				"Meter not found",
				fmt.Sprintf("No meter found with ID %s.", data.ID.ValueString()),
			)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading meter",
			fmt.Sprintf("Could not read meter %s: %s", data.ID.ValueString(), err),
		)
		return
	}

	mapMeterResponseToState(ctx, result.Meter, &data, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
