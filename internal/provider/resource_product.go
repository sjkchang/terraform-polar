// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/polarsource/polar-go"
	"github.com/polarsource/polar-go/models/apierrors"
	"github.com/polarsource/polar-go/models/components"
)

var _ resource.Resource = &ProductResource{}
var _ resource.ResourceWithImportState = &ProductResource{}

func NewProductResource() resource.Resource {
	return &ProductResource{}
}

type ProductResource struct {
	client *polargo.Polar
}

// --- Terraform model types ---

type ProductResourceModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Description       types.String `tfsdk:"description"`
	RecurringInterval types.String `tfsdk:"recurring_interval"`
	Prices            []PriceModel `tfsdk:"prices"`
	Metadata          types.Map    `tfsdk:"metadata"`
	Medias            types.List   `tfsdk:"medias"`
	IsArchived        types.Bool   `tfsdk:"is_archived"`
}

type PriceModel struct {
	AmountType    types.String `tfsdk:"amount_type"`
	PriceCurrency types.String `tfsdk:"price_currency"`
	// Fixed
	PriceAmount types.Int64 `tfsdk:"price_amount"`
	// Custom (pay-what-you-want)
	MinimumAmount types.Int64 `tfsdk:"minimum_amount"`
	MaximumAmount types.Int64 `tfsdk:"maximum_amount"`
	PresetAmount  types.Int64 `tfsdk:"preset_amount"`
	// Metered unit
	MeterID    types.String `tfsdk:"meter_id"`
	UnitAmount types.String `tfsdk:"unit_amount"`
	CapAmount  types.Int64  `tfsdk:"cap_amount"`
}

// --- Resource interface ---

func (r *ProductResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_product"
}

func (r *ProductResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Polar product. Products represent purchasable items with one or more prices. Set `recurring_interval` for subscription products, or omit it for one-time purchases. Destroying a product archives it.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The product ID.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the product.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "The description of the product.",
				Optional:            true,
			},
			"recurring_interval": schema.StringAttribute{
				MarkdownDescription: "The billing interval for recurring products. Must be one of: `month`, `year`, `week`, `day`. Omit for one-time products. Changing this forces a new resource (the existing product is archived, not deleted).",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					requiresReplaceWithArchiveWarning("product",
						"Existing subscribers will remain on the archived product and will NOT be automatically migrated to the new one. "+
							"For subscription products, you may want to migrate subscribers before archiving."),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("month", "year", "week", "day"),
				},
			},
			"prices": schema.ListNestedAttribute{
				MarkdownDescription: "List of prices for this product. At least one price is required. Each price uses `amount_type` to determine which fields apply.",
				Required:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"amount_type": schema.StringAttribute{
							MarkdownDescription: "The price type. Must be one of: `fixed`, `free`, `custom`, `metered_unit`.",
							Required:            true,
							Validators: []validator.String{
								stringvalidator.OneOf("fixed", "free", "custom", "metered_unit"),
							},
						},
						"price_currency": schema.StringAttribute{
							MarkdownDescription: "The currency code (e.g. `usd`). Defaults to `usd`. Applies to `fixed`, `custom`, and `metered_unit` types.",
							Optional:            true,
							Computed:            true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						// Fixed
						"price_amount": schema.Int64Attribute{
							MarkdownDescription: "The price amount in cents. Required when `amount_type` is `fixed`.",
							Optional:            true,
						},
						// Custom (pay-what-you-want)
						"minimum_amount": schema.Int64Attribute{
							MarkdownDescription: "The minimum amount in cents the customer can pay. For `custom` type.",
							Optional:            true,
						},
						"maximum_amount": schema.Int64Attribute{
							MarkdownDescription: "The maximum amount in cents the customer can pay. For `custom` type.",
							Optional:            true,
						},
						"preset_amount": schema.Int64Attribute{
							MarkdownDescription: "The initial amount in cents shown to the customer. For `custom` type.",
							Optional:            true,
						},
						// Metered unit
						"meter_id": schema.StringAttribute{
							MarkdownDescription: "The ID of the meter associated with this price. Required when `amount_type` is `metered_unit`.",
							Optional:            true,
						},
						"unit_amount": schema.StringAttribute{
							MarkdownDescription: "The price per unit in cents (supports up to 12 decimal places). Required when `amount_type` is `metered_unit`.",
							Optional:            true,
						},
						"cap_amount": schema.Int64Attribute{
							MarkdownDescription: "Maximum amount in cents that can be charged regardless of units consumed. For `metered_unit` type.",
							Optional:            true,
						},
					},
				},
			},
			"metadata": schema.MapAttribute{
				MarkdownDescription: "Key-value metadata.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"medias": schema.ListAttribute{
				MarkdownDescription: "List of media file IDs attached to the product.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"is_archived": schema.BoolAttribute{
				MarkdownDescription: "Whether the product is archived. Defaults to `false`. Set to `true` to archive a product while keeping it in Terraform state.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
		},
	}
}

func (r *ProductResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*polargo.Polar)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *polargo.Polar, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *ProductResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ProductResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq, diags := buildProductCreateRequest(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.Products.Create(ctx, *createReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating product",
			fmt.Sprintf("Could not create product: %s", err),
		)
		return
	}

	tflog.Trace(ctx, "created product", map[string]interface{}{
		"id": result.Product.ID,
	})

	plannedPrices := data.Prices
	mapProductResponseToState(ctx, result.Product, &data, &resp.Diagnostics)
	preserveUnitAmountFormatting(data.Prices, plannedPrices)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProductResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ProductResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	priorPrices := data.Prices
	result, err := r.client.Products.Get(ctx, data.ID.ValueString())
	if err != nil {
		var notFound *apierrors.ResourceNotFound
		if isNotFound(err, &notFound) {
			tflog.Trace(ctx, "product not found, removing from state", map[string]interface{}{
				"id": data.ID.ValueString(),
			})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading product",
			fmt.Sprintf("Could not read product %s: %s", data.ID.ValueString(), err),
		)
		return
	}

	mapProductResponseToState(ctx, result.Product, &data, &resp.Diagnostics)
	preserveUnitAmountFormatting(data.Prices, priorPrices)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProductResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ProductResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fetch current product to get existing price IDs for reuse
	current, err := r.client.Products.Get(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading product for update",
			fmt.Sprintf("Could not read product %s: %s", data.ID.ValueString(), err),
		)
		return
	}

	updateReq, diags := buildProductUpdateRequest(ctx, &data, current.Product.Prices)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	plannedPrices := data.Prices
	result, err := r.client.Products.Update(ctx, data.ID.ValueString(), *updateReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating product",
			fmt.Sprintf("Could not update product %s: %s", data.ID.ValueString(), err),
		)
		return
	}

	mapProductResponseToState(ctx, result.Product, &data, &resp.Diagnostics)
	preserveUnitAmountFormatting(data.Prices, plannedPrices)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProductResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ProductResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Products don't have a DELETE endpoint; archive instead
	isArchived := true
	_, err := r.client.Products.Update(ctx, data.ID.ValueString(), components.ProductUpdate{
		IsArchived: &isArchived,
	})
	if err != nil {
		var notFound *apierrors.ResourceNotFound
		if isNotFound(err, &notFound) {
			return
		}
		resp.Diagnostics.AddError(
			"Error archiving product",
			fmt.Sprintf("Could not archive product %s: %s", data.ID.ValueString(), err),
		)
		return
	}

	tflog.Trace(ctx, "archived product", map[string]interface{}{
		"id": data.ID.ValueString(),
	})
}

func (r *ProductResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

