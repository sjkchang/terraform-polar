// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
)

var _ resource.Resource = &BenefitResource{}
var _ resource.ResourceWithImportState = &BenefitResource{}

func NewBenefitResource() resource.Resource {
	return &BenefitResource{}
}

type BenefitResource struct {
	client *polargo.Polar
}

// --- Terraform model types ---

type BenefitResourceModel struct {
	ID                         types.String                            `tfsdk:"id"`
	Type                       types.String                            `tfsdk:"type"`
	Description                types.String                            `tfsdk:"description"`
	Metadata                   types.Map                               `tfsdk:"metadata"`
	CustomProperties           *BenefitCustomPropertiesModel           `tfsdk:"custom_properties"`
	DiscordProperties          *BenefitDiscordPropertiesModel          `tfsdk:"discord_properties"`
	GitHubRepositoryProperties *BenefitGitHubRepositoryPropertiesModel `tfsdk:"github_repository_properties"`
	DownloadablesProperties    *BenefitDownloadablesPropertiesModel    `tfsdk:"downloadables_properties"`
	LicenseKeysProperties      *BenefitLicenseKeysPropertiesModel      `tfsdk:"license_keys_properties"`
	MeterCreditProperties      *BenefitMeterCreditPropertiesModel      `tfsdk:"meter_credit_properties"`
}

type BenefitCustomPropertiesModel struct {
	Note types.String `tfsdk:"note"`
}

type BenefitDiscordPropertiesModel struct {
	GuildToken types.String `tfsdk:"guild_token"`
	RoleID     types.String `tfsdk:"role_id"`
	KickMember types.Bool   `tfsdk:"kick_member"`
	GuildID    types.String `tfsdk:"guild_id"`
}

type BenefitGitHubRepositoryPropertiesModel struct {
	RepositoryOwner types.String `tfsdk:"repository_owner"`
	RepositoryName  types.String `tfsdk:"repository_name"`
	Permission      types.String `tfsdk:"permission"`
}

type BenefitDownloadablesPropertiesModel struct {
	Files types.List `tfsdk:"files"`
}

type BenefitLicenseKeysPropertiesModel struct {
	Prefix      types.String                      `tfsdk:"prefix"`
	LimitUsage  types.Int64                       `tfsdk:"limit_usage"`
	Expires     *BenefitLicenseKeyExpirationModel `tfsdk:"expires"`
	Activations *BenefitLicenseKeyActivationModel `tfsdk:"activations"`
}

type BenefitLicenseKeyExpirationModel struct {
	TTL       types.Int64  `tfsdk:"ttl"`
	Timeframe types.String `tfsdk:"timeframe"`
}

type BenefitLicenseKeyActivationModel struct {
	Limit               types.Int64 `tfsdk:"limit"`
	EnableCustomerAdmin types.Bool  `tfsdk:"enable_customer_admin"`
}

type BenefitMeterCreditPropertiesModel struct {
	MeterID  types.String `tfsdk:"meter_id"`
	Units    types.Int64  `tfsdk:"units"`
	Rollover types.Bool   `tfsdk:"rollover"`
}

// --- Resource interface ---

func (r *BenefitResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_benefit"
}

func (r *BenefitResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Polar benefit. Benefits define value delivered to subscribers, such as custom perks, Discord roles, GitHub repository access, downloadable files, license keys, or meter credits.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The benefit ID.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "The benefit type. Changing this forces a new resource. Must be one of: `custom`, `discord`, `github_repository`, `downloadables`, `license_keys`, `meter_credit`.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("custom", "discord", "github_repository", "downloadables", "license_keys", "meter_credit"),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "The description of the benefit. Displayed on products having this benefit. Maximum 42 characters.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(42),
				},
			},
			"metadata": schema.MapAttribute{
				MarkdownDescription: "Key-value metadata.",
				Optional:            true,
				ElementType:         types.StringType,
			},

			// Type-specific properties (exactly one should match the type)
			"custom_properties": schema.SingleNestedAttribute{
				MarkdownDescription: "Properties for `custom` type benefits.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"note": schema.StringAttribute{
						MarkdownDescription: "A note to display to the subscriber.",
						Optional:            true,
					},
				},
			},
			"discord_properties": schema.SingleNestedAttribute{
				MarkdownDescription: "Properties for `discord` type benefits.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"guild_token": schema.StringAttribute{
						MarkdownDescription: "The Discord bot token for the server.",
						Required:            true,
						Sensitive:           true,
					},
					"role_id": schema.StringAttribute{
						MarkdownDescription: "The Discord role ID to grant.",
						Required:            true,
					},
					"kick_member": schema.BoolAttribute{
						MarkdownDescription: "Whether to kick the member when the benefit is revoked.",
						Required:            true,
					},
					"guild_id": schema.StringAttribute{
						MarkdownDescription: "The Discord server (guild) ID. Computed from the guild token.",
						Computed:            true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
				},
			},
			"github_repository_properties": schema.SingleNestedAttribute{
				MarkdownDescription: "Properties for `github_repository` type benefits.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"repository_owner": schema.StringAttribute{
						MarkdownDescription: "The GitHub repository owner (user or organization).",
						Required:            true,
					},
					"repository_name": schema.StringAttribute{
						MarkdownDescription: "The GitHub repository name.",
						Required:            true,
					},
					"permission": schema.StringAttribute{
						MarkdownDescription: "The permission level to grant. Must be one of: `pull`, `triage`, `push`, `maintain`, `admin`.",
						Required:            true,
						Validators: []validator.String{
							stringvalidator.OneOf("pull", "triage", "push", "maintain", "admin"),
						},
					},
				},
			},
			"downloadables_properties": schema.SingleNestedAttribute{
				MarkdownDescription: "Properties for `downloadables` type benefits.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"files": schema.ListAttribute{
						MarkdownDescription: "List of file IDs available for download.",
						Required:            true,
						ElementType:         types.StringType,
					},
				},
			},
			"license_keys_properties": schema.SingleNestedAttribute{
				MarkdownDescription: "Properties for `license_keys` type benefits.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"prefix": schema.StringAttribute{
						MarkdownDescription: "A prefix for generated license keys.",
						Optional:            true,
					},
					"limit_usage": schema.Int64Attribute{
						MarkdownDescription: "Maximum number of times a license key can be used.",
						Optional:            true,
					},
					"expires": schema.SingleNestedAttribute{
						MarkdownDescription: "Expiration settings for license keys.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"ttl": schema.Int64Attribute{
								MarkdownDescription: "Time-to-live value.",
								Required:            true,
							},
							"timeframe": schema.StringAttribute{
								MarkdownDescription: "The timeframe unit. Must be one of: `year`, `month`, `day`.",
								Required:            true,
								Validators: []validator.String{
									stringvalidator.OneOf("year", "month", "day"),
								},
							},
						},
					},
					"activations": schema.SingleNestedAttribute{
						MarkdownDescription: "Activation settings for license keys.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"limit": schema.Int64Attribute{
								MarkdownDescription: "Maximum number of activations.",
								Required:            true,
							},
							"enable_customer_admin": schema.BoolAttribute{
								MarkdownDescription: "Whether the customer can manage their own activations.",
								Required:            true,
							},
						},
					},
				},
			},
			"meter_credit_properties": schema.SingleNestedAttribute{
				MarkdownDescription: "Properties for `meter_credit` type benefits.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"meter_id": schema.StringAttribute{
						MarkdownDescription: "The ID of the meter to credit.",
						Required:            true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"units": schema.Int64Attribute{
						MarkdownDescription: "The number of units to credit.",
						Required:            true,
					},
					"rollover": schema.BoolAttribute{
						MarkdownDescription: "Whether unused credits roll over to the next period.",
						Required:            true,
					},
				},
			},
		},
	}
}

func (r *BenefitResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *BenefitResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data BenefitResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq, diags := buildBenefitCreateRequest(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.Benefits.Create(ctx, *createReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating benefit",
			fmt.Sprintf("Could not create benefit: %s", err),
		)
		return
	}

	tflog.Trace(ctx, "created benefit", map[string]interface{}{
		"type": data.Type.ValueString(),
	})

	mapBenefitResponseToState(ctx, result.Benefit, &data, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BenefitResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data BenefitResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.Benefits.Get(ctx, data.ID.ValueString())
	if err != nil {
		var notFound *apierrors.ResourceNotFound
		if isNotFound(err, &notFound) {
			tflog.Trace(ctx, "benefit not found, removing from state", map[string]interface{}{
				"id": data.ID.ValueString(),
			})
			resp.State.RemoveResource(ctx)
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

func (r *BenefitResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data BenefitResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq, diags := buildBenefitUpdateRequest(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.Benefits.Update(ctx, data.ID.ValueString(), *updateReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating benefit",
			fmt.Sprintf("Could not update benefit %s: %s", data.ID.ValueString(), err),
		)
		return
	}

	mapBenefitResponseToState(ctx, result.Benefit, &data, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *BenefitResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data BenefitResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Benefits.Delete(ctx, data.ID.ValueString())
	if err != nil {
		var notFound *apierrors.ResourceNotFound
		if isNotFound(err, &notFound) {
			return
		}
		resp.Diagnostics.AddError(
			"Error deleting benefit",
			fmt.Sprintf("Could not delete benefit %s: %s", data.ID.ValueString(), err),
		)
		return
	}

	tflog.Trace(ctx, "deleted benefit", map[string]interface{}{
		"id": data.ID.ValueString(),
	})
}

func (r *BenefitResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
