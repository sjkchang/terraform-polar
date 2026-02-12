// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/polarsource/polar-go"
	"github.com/polarsource/polar-go/models/apierrors"
	"github.com/polarsource/polar-go/models/components"
)

var _ resource.Resource = &MeterResource{}
var _ resource.ResourceWithImportState = &MeterResource{}

func NewMeterResource() resource.Resource {
	return &MeterResource{}
}

type MeterResource struct {
	client *polargo.Polar
}

// Model types shared between meter resource and data source.

type MeterResourceModel struct {
	ID          types.String      `tfsdk:"id"`
	Name        types.String      `tfsdk:"name"`
	Filter      *FilterModel      `tfsdk:"filter"`
	Aggregation *AggregationModel `tfsdk:"aggregation"`
	Metadata    types.Map         `tfsdk:"metadata"`
}

type FilterModel struct {
	Conjunction types.String        `tfsdk:"conjunction"`
	Clauses     []FilterClauseModel `tfsdk:"clauses"`
}

type FilterClauseModel struct {
	Property types.String `tfsdk:"property"`
	Operator types.String `tfsdk:"operator"`
	Value    types.String `tfsdk:"value"`
}

type AggregationModel struct {
	Func     types.String `tfsdk:"func"`
	Property types.String `tfsdk:"property"`
}

func (r *MeterResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_meter"
}

func (r *MeterResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Polar meter. Meters track usage events and aggregate them for billing purposes.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The meter ID.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the meter, shown on invoices and usage reports.",
				Required:            true,
			},
			"filter": schema.SingleNestedAttribute{
				MarkdownDescription: "Filter to apply on incoming events.",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"conjunction": schema.StringAttribute{
						MarkdownDescription: "Logical conjunction for combining clauses. Must be `and` or `or`.",
						Required:            true,
						Validators: []validator.String{
							stringvalidator.OneOf("and", "or"),
						},
					},
					"clauses": schema.ListNestedAttribute{
						MarkdownDescription: "List of filter clauses.",
						Required:            true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"property": schema.StringAttribute{
									MarkdownDescription: "The event property to filter on.",
									Required:            true,
								},
								"operator": schema.StringAttribute{
									MarkdownDescription: "The comparison operator. Must be one of: `eq`, `ne`, `gt`, `gte`, `lt`, `lte`, `like`, `not_like`.",
									Required:            true,
									Validators: []validator.String{
										stringvalidator.OneOf("eq", "ne", "gt", "gte", "lt", "lte", "like", "not_like"),
									},
								},
								"value": schema.StringAttribute{
									MarkdownDescription: "The value to compare against.",
									Required:            true,
								},
							},
						},
					},
				},
			},
			"aggregation": schema.SingleNestedAttribute{
				MarkdownDescription: "Aggregation function for the meter.",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"func": schema.StringAttribute{
						MarkdownDescription: "The aggregation function. Must be one of: `count`, `sum`, `avg`, `min`, `max`, `unique`.",
						Required:            true,
						Validators: []validator.String{
							stringvalidator.OneOf("count", "sum", "avg", "min", "max", "unique"),
						},
					},
					"property": schema.StringAttribute{
						MarkdownDescription: "The event property to aggregate. Required for all functions except `count`.",
						Optional:            true,
					},
				},
			},
			"metadata": schema.MapAttribute{
				MarkdownDescription: "Key-value metadata.",
				Optional:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (r *MeterResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *MeterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data MeterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := components.MeterCreate{
		Name:        data.Name.ValueString(),
		Filter:      filterModelToSDK(data.Filter),
		Aggregation: aggregationModelToCreateSDK(data.Aggregation),
	}

	if !data.Metadata.IsNull() && !data.Metadata.IsUnknown() {
		metadata, diags := metadataModelToCreateSDK(ctx, data.Metadata)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.Metadata = metadata
	}

	result, err := r.client.Meters.Create(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating meter",
			fmt.Sprintf("Could not create meter: %s", err),
		)
		return
	}

	tflog.Trace(ctx, "created meter", map[string]interface{}{
		"id": result.Meter.ID,
	})

	mapMeterResponseToState(ctx, result.Meter, &data, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MeterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MeterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.Meters.Get(ctx, data.ID.ValueString())
	if err != nil {
		var notFound *apierrors.ResourceNotFound
		if isNotFound(err, &notFound) {
			tflog.Trace(ctx, "meter not found, removing from state", map[string]interface{}{
				"id": data.ID.ValueString(),
			})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading meter",
			fmt.Sprintf("Could not read meter %s: %s", data.ID.ValueString(), err),
		)
		return
	}

	// Treat archived meters as deleted
	if result.Meter.ArchivedAt != nil {
		tflog.Trace(ctx, "meter is archived, removing from state", map[string]interface{}{
			"id": data.ID.ValueString(),
		})
		resp.State.RemoveResource(ctx)
		return
	}

	mapMeterResponseToState(ctx, result.Meter, &data, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MeterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data MeterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := data.Name.ValueString()
	filter := filterModelToSDK(data.Filter)
	aggregation := aggregationModelToUpdateSDK(data.Aggregation)

	updateReq := components.MeterUpdate{
		Name:        &name,
		Filter:      &filter,
		Aggregation: aggregation,
	}

	if !data.Metadata.IsNull() && !data.Metadata.IsUnknown() {
		metadata, diags := metadataModelToUpdateSDK(ctx, data.Metadata)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		updateReq.Metadata = metadata
	}

	result, err := r.client.Meters.Update(ctx, data.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating meter",
			fmt.Sprintf("Could not update meter %s: %s", data.ID.ValueString(), err),
		)
		return
	}

	mapMeterResponseToState(ctx, result.Meter, &data, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MeterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data MeterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Meters don't have a DELETE endpoint; archive instead
	isArchived := true
	_, err := r.client.Meters.Update(ctx, data.ID.ValueString(), components.MeterUpdate{
		IsArchived: &isArchived,
	})
	if err != nil {
		var notFound *apierrors.ResourceNotFound
		if isNotFound(err, &notFound) {
			return
		}
		resp.Diagnostics.AddError(
			"Error archiving meter",
			fmt.Sprintf("Could not archive meter %s: %s", data.ID.ValueString(), err),
		)
		return
	}

	tflog.Trace(ctx, "archived meter", map[string]interface{}{
		"id": data.ID.ValueString(),
	})
}

func (r *MeterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mapMeterResponseToState maps a Meter API response to the Terraform resource model.
func mapMeterResponseToState(ctx context.Context, meter *components.Meter, data *MeterResourceModel, diags *diag.Diagnostics) {
	data.ID = types.StringValue(meter.ID)
	data.Name = types.StringValue(meter.Name)
	data.Filter = sdkFilterToModel(meter.Filter)
	data.Aggregation = sdkAggregationToModel(meter.Aggregation)

	metadataMap, metaDiags := sdkMeterMetadataToMap(ctx, meter.Metadata)
	diags.Append(metaDiags...)
	data.Metadata = metadataMap
}

// --- SDK conversion helpers ---

func filterModelToSDK(filter *FilterModel) components.Filter {
	clauses := make([]components.Clauses, len(filter.Clauses))
	for i, c := range filter.Clauses {
		clauses[i] = components.CreateClausesFilterClause(
			components.FilterClause{
				Property: c.Property.ValueString(),
				Operator: components.FilterOperator(c.Operator.ValueString()),
				Value:    components.CreateValueStr(c.Value.ValueString()),
			},
		)
	}
	return components.Filter{
		Conjunction: components.FilterConjunction(filter.Conjunction.ValueString()),
		Clauses:     clauses,
	}
}

func sdkFilterToModel(filter components.Filter) *FilterModel {
	clauses := make([]FilterClauseModel, 0, len(filter.Clauses))
	for _, c := range filter.Clauses {
		if c.FilterClause != nil {
			clause := c.FilterClause
			var value string
			switch {
			case clause.Value.Str != nil:
				value = *clause.Value.Str
			case clause.Value.Integer != nil:
				value = strconv.FormatInt(*clause.Value.Integer, 10)
			case clause.Value.Boolean != nil:
				value = strconv.FormatBool(*clause.Value.Boolean)
			}
			clauses = append(clauses, FilterClauseModel{
				Property: types.StringValue(clause.Property),
				Operator: types.StringValue(string(clause.Operator)),
				Value:    types.StringValue(value),
			})
		}
		// Skip nested Filter clauses (not supported in flat TF schema)
	}
	return &FilterModel{
		Conjunction: types.StringValue(string(filter.Conjunction)),
		Clauses:     clauses,
	}
}

func aggregationModelToCreateSDK(agg *AggregationModel) components.MeterCreateAggregation {
	funcName := agg.Func.ValueString()
	property := agg.Property.ValueString()

	switch funcName {
	case "count":
		return components.CreateMeterCreateAggregationCount(components.CountAggregation{})
	case "sum":
		return components.CreateMeterCreateAggregationSum(components.PropertyAggregation{
			Func:     components.FuncSum,
			Property: property,
		})
	case "avg":
		return components.CreateMeterCreateAggregationAvg(components.PropertyAggregation{
			Func:     components.FuncAvg,
			Property: property,
		})
	case "min":
		return components.CreateMeterCreateAggregationMin(components.PropertyAggregation{
			Func:     components.FuncMin,
			Property: property,
		})
	case "max":
		return components.CreateMeterCreateAggregationMax(components.PropertyAggregation{
			Func:     components.FuncMax,
			Property: property,
		})
	case "unique":
		return components.CreateMeterCreateAggregationUnique(components.UniqueAggregation{
			Property: property,
		})
	default:
		return components.CreateMeterCreateAggregationCount(components.CountAggregation{})
	}
}

func aggregationModelToUpdateSDK(agg *AggregationModel) *components.Aggregation {
	funcName := agg.Func.ValueString()
	property := agg.Property.ValueString()

	var result components.Aggregation
	switch funcName {
	case "count":
		result = components.CreateAggregationCount(components.CountAggregation{})
	case "sum":
		result = components.CreateAggregationSum(components.PropertyAggregation{
			Func:     components.FuncSum,
			Property: property,
		})
	case "avg":
		result = components.CreateAggregationAvg(components.PropertyAggregation{
			Func:     components.FuncAvg,
			Property: property,
		})
	case "min":
		result = components.CreateAggregationMin(components.PropertyAggregation{
			Func:     components.FuncMin,
			Property: property,
		})
	case "max":
		result = components.CreateAggregationMax(components.PropertyAggregation{
			Func:     components.FuncMax,
			Property: property,
		})
	case "unique":
		result = components.CreateAggregationUnique(components.UniqueAggregation{
			Property: property,
		})
	default:
		result = components.CreateAggregationCount(components.CountAggregation{})
	}
	return &result
}

func sdkAggregationToModel(agg components.MeterAggregation) *AggregationModel {
	model := &AggregationModel{}
	switch {
	case agg.CountAggregation != nil:
		model.Func = types.StringValue("count")
		model.Property = types.StringNull()
	case agg.PropertyAggregation != nil:
		model.Func = types.StringValue(string(agg.PropertyAggregation.Func))
		model.Property = types.StringValue(agg.PropertyAggregation.Property)
	case agg.UniqueAggregation != nil:
		model.Func = types.StringValue("unique")
		model.Property = types.StringValue(agg.UniqueAggregation.Property)
	}
	return model
}

func metadataModelToCreateSDK(ctx context.Context, metadata types.Map) (map[string]components.MeterCreateMetadata, diag.Diagnostics) {
	var stringMap map[string]string
	diags := metadata.ElementsAs(ctx, &stringMap, false)
	if diags.HasError() {
		return nil, diags
	}
	result := make(map[string]components.MeterCreateMetadata, len(stringMap))
	for k, v := range stringMap {
		result[k] = components.CreateMeterCreateMetadataStr(v)
	}
	return result, diags
}

func metadataModelToUpdateSDK(ctx context.Context, metadata types.Map) (map[string]components.MeterUpdateMetadata, diag.Diagnostics) {
	var stringMap map[string]string
	diags := metadata.ElementsAs(ctx, &stringMap, false)
	if diags.HasError() {
		return nil, diags
	}
	result := make(map[string]components.MeterUpdateMetadata, len(stringMap))
	for k, v := range stringMap {
		result[k] = components.CreateMeterUpdateMetadataStr(v)
	}
	return result, diags
}

func sdkMeterMetadataToMap(ctx context.Context, metadata map[string]components.MeterMetadata) (types.Map, diag.Diagnostics) {
	if len(metadata) == 0 {
		return types.MapNull(types.StringType), nil
	}
	stringMap := make(map[string]string, len(metadata))
	for k, v := range metadata {
		switch {
		case v.Str != nil:
			stringMap[k] = *v.Str
		case v.Integer != nil:
			stringMap[k] = strconv.FormatInt(*v.Integer, 10)
		case v.Number != nil:
			stringMap[k] = strconv.FormatFloat(*v.Number, 'f', -1, 64)
		case v.Boolean != nil:
			stringMap[k] = strconv.FormatBool(*v.Boolean)
		}
	}
	return types.MapValueFrom(ctx, types.StringType, stringMap)
}
