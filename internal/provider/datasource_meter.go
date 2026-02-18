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

// Compile-time interface conformance check.
var _ datasource.DataSource = &MeterDataSource{}

func NewMeterDataSource() datasource.DataSource {
	return &MeterDataSource{}
}

// MeterDataSource is a read-only data source for looking up a meter by ID.
// Useful for referencing an unmanaged meter in a metered-unit price or meter-credit benefit.
type MeterDataSource struct {
	client *polargo.Polar
}

type MeterDataSourceModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

func (d *MeterDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_meter"
}

func (d *MeterDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a Polar meter by ID. Use this to reference an unmanaged meter from a metered-unit price or meter-credit benefit.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The meter ID.",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the meter.",
				Computed:            true,
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
	var data MeterDataSourceModel
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

	data.ID = types.StringValue(result.Meter.ID)
	data.Name = types.StringValue(result.Meter.Name)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
