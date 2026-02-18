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
	if pd := extractProviderData(req.ProviderData, &resp.Diagnostics); pd != nil {
		r.client = pd.Client
	}
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
		m, d := metadataToCreateSDK(ctx, data.Metadata, components.CreateMeterCreateMetadataStr)
		resp.Diagnostics.Append(d...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.Metadata = m
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

	writeTime := latestTimestamp(result.Meter)
	meter, err := pollForConsistency(ctx, "meter", result.Meter.ID, writeTime, func() (*components.Meter, error) {
		r, err := r.client.Meters.Get(ctx, result.Meter.ID)
		if err != nil {
			return nil, err
		}
		return r.Meter, nil
	}, &resp.Diagnostics)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error waiting for meter visibility",
			fmt.Sprintf("Meter %s was created but not immediately readable: %s", result.Meter.ID, err),
		)
		return
	}

	mapMeterResponseToState(ctx, meter, &data, &resp.Diagnostics)
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
		if handleNotFoundRemove(ctx, err, "meter", data.ID.ValueString(), &resp.State) {
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
		m, d := metadataToCreateSDK(ctx, data.Metadata, components.CreateMeterUpdateMetadataStr)
		resp.Diagnostics.Append(d...)
		if resp.Diagnostics.HasError() {
			return
		}
		updateReq.Metadata = m
	}

	result, err := r.client.Meters.Update(ctx, data.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating meter",
			fmt.Sprintf("Could not update meter %s: %s", data.ID.ValueString(), err),
		)
		return
	}

	writeTime := latestTimestamp(result.Meter)
	meter, err := pollForConsistency(ctx, "meter", result.Meter.ID, writeTime, func() (*components.Meter, error) {
		r, err := r.client.Meters.Get(ctx, result.Meter.ID)
		if err != nil {
			return nil, err
		}
		return r.Meter, nil
	}, &resp.Diagnostics)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading meter after update",
			fmt.Sprintf("Could not read meter %s: %s", result.Meter.ID, err),
		)
		return
	}

	mapMeterResponseToState(ctx, meter, &data, &resp.Diagnostics)
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
		if isNotFound(err) {
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
	data.Filter = sdkFilterToModel(meter.Filter, diags)
	data.Aggregation = sdkAggregationToModel(meter.Aggregation, diags)

	data.Metadata = sdkMetadataToMap(ctx, meter.Metadata, func(v components.MeterMetadata) metadataFields {
		return metadataFields{Str: v.Str, Integer: v.Integer, Number: v.Number, Boolean: v.Boolean}
	}, diags)
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

func sdkFilterToModel(filter components.Filter, diags *diag.Diagnostics) *FilterModel {
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
			default:
				diags.AddWarning(
					"Unknown filter clause value type",
					fmt.Sprintf("Filter clause for property %q has an unrecognized value type. The value was set to empty.", clause.Property),
				)
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

func sdkAggregationToModel(agg components.MeterAggregation, diags *diag.Diagnostics) *AggregationModel {
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
	default:
		diags.AddWarning(
			"Unknown aggregation type",
			"The API returned an aggregation type not recognized by this provider version. Some fields may be empty.",
		)
		model.Func = types.StringNull()
		model.Property = types.StringNull()
	}
	return model
}

