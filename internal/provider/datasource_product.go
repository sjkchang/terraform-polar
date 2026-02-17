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

var _ datasource.DataSource = &ProductDataSource{}

func NewProductDataSource() datasource.DataSource {
	return &ProductDataSource{}
}

type ProductDataSource struct {
	client *polargo.Polar
}

func (d *ProductDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_product"
}

func (d *ProductDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a Polar product by ID.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The product ID.",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the product.",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "The description of the product.",
				Computed:            true,
			},
			"recurring_interval": schema.StringAttribute{
				MarkdownDescription: "The billing interval for recurring products.",
				Computed:            true,
			},
			"prices": schema.ListNestedAttribute{
				MarkdownDescription: "List of prices for this product.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"amount_type": schema.StringAttribute{
							MarkdownDescription: "The price type.",
							Computed:            true,
						},
						"price_currency": schema.StringAttribute{
							MarkdownDescription: "The currency code.",
							Computed:            true,
						},
						"price_amount": schema.Int64Attribute{
							MarkdownDescription: "The price amount in cents.",
							Computed:            true,
						},
						"minimum_amount": schema.Int64Attribute{
							MarkdownDescription: "The minimum amount in cents (custom type).",
							Computed:            true,
						},
						"maximum_amount": schema.Int64Attribute{
							MarkdownDescription: "The maximum amount in cents (custom type).",
							Computed:            true,
						},
						"preset_amount": schema.Int64Attribute{
							MarkdownDescription: "The initial amount in cents (custom type).",
							Computed:            true,
						},
						"meter_id": schema.StringAttribute{
							MarkdownDescription: "The meter ID (metered_unit type).",
							Computed:            true,
						},
						"unit_amount": schema.StringAttribute{
							MarkdownDescription: "The price per unit in cents (metered_unit type).",
							Computed:            true,
						},
						"cap_amount": schema.Int64Attribute{
							MarkdownDescription: "Maximum charge in cents (metered_unit type).",
							Computed:            true,
						},
					},
				},
			},
			"metadata": schema.MapAttribute{
				MarkdownDescription: "Key-value metadata.",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"medias": schema.ListAttribute{
				MarkdownDescription: "List of media file IDs attached to the product.",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"is_archived": schema.BoolAttribute{
				MarkdownDescription: "Whether the product is archived.",
				Computed:            true,
			},
		},
	}
}

func (d *ProductDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ProductDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ProductResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := d.client.Products.Get(ctx, data.ID.ValueString())
	if err != nil {
		var notFound *apierrors.ResourceNotFound
		if isNotFound(err, &notFound) {
			resp.Diagnostics.AddError(
				"Product not found",
				fmt.Sprintf("No product found with ID %s.", data.ID.ValueString()),
			)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading product",
			fmt.Sprintf("Could not read product %s: %s", data.ID.ValueString(), err),
		)
		return
	}

	mapProductResponseToState(ctx, result.Product, &data, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
