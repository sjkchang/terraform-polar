// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	"github.com/polarsource/polar-go/models/components"
)

var _ resource.Resource = &OrganizationResource{}
var _ resource.ResourceWithImportState = &OrganizationResource{}

func NewOrganizationResource() resource.Resource {
	return &OrganizationResource{}
}

type OrganizationResource struct {
	provider *PolarProviderData
}

// --- Terraform model types ---

type OrganizationResourceModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Slug      types.String `tfsdk:"slug"`
	AvatarURL types.String `tfsdk:"avatar_url"`
	Email     types.String `tfsdk:"email"`
	Website   types.String `tfsdk:"website"`
	Socials   types.List   `tfsdk:"socials"`

	FeatureSettings       *FeatureSettingsModel       `tfsdk:"feature_settings"`
	SubscriptionSettings  *SubscriptionSettingsModel  `tfsdk:"subscription_settings"`
	NotificationSettings  *NotificationSettingsModel  `tfsdk:"notification_settings"`
	CustomerEmailSettings *CustomerEmailSettingsModel `tfsdk:"customer_email_settings"`
}

type SocialModel struct {
	Platform types.String `tfsdk:"platform"`
	URL      types.String `tfsdk:"url"`
}

type FeatureSettingsModel struct {
	IssueFundingEnabled     types.Bool `tfsdk:"issue_funding_enabled"`
	SeatBasedPricingEnabled types.Bool `tfsdk:"seat_based_pricing_enabled"`
	RevopsEnabled           types.Bool `tfsdk:"revops_enabled"`
	WalletsEnabled          types.Bool `tfsdk:"wallets_enabled"`
}

type SubscriptionSettingsModel struct {
	AllowMultipleSubscriptions   types.Bool   `tfsdk:"allow_multiple_subscriptions"`
	AllowCustomerUpdates         types.Bool   `tfsdk:"allow_customer_updates"`
	ProrationBehavior            types.String `tfsdk:"proration_behavior"`
	BenefitRevocationGracePeriod types.Int64  `tfsdk:"benefit_revocation_grace_period"`
	PreventTrialAbuse            types.Bool   `tfsdk:"prevent_trial_abuse"`
}

type NotificationSettingsModel struct {
	NewOrder        types.Bool `tfsdk:"new_order"`
	NewSubscription types.Bool `tfsdk:"new_subscription"`
}

type CustomerEmailSettingsModel struct {
	OrderConfirmation        types.Bool `tfsdk:"order_confirmation"`
	SubscriptionCancellation types.Bool `tfsdk:"subscription_cancellation"`
	SubscriptionConfirmation types.Bool `tfsdk:"subscription_confirmation"`
	SubscriptionCycled       types.Bool `tfsdk:"subscription_cycled"`
	SubscriptionPastDue      types.Bool `tfsdk:"subscription_past_due"`
	SubscriptionRevoked      types.Bool `tfsdk:"subscription_revoked"`
	SubscriptionUncanceled   types.Bool `tfsdk:"subscription_uncanceled"`
	SubscriptionUpdated      types.Bool `tfsdk:"subscription_updated"`
}

// --- Resource interface ---

func (r *OrganizationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization"
}

func (r *OrganizationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Polar organization's settings. The organization must already exist (created via the Polar UI). " +
			"The access token is scoped to a single organization — this resource adopts it on create and releases it from state on destroy. " +
			"Only include the settings blocks you want Terraform to manage; omitted blocks are left untouched.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The organization ID.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the organization.",
				Optional:            true,
			},
			"slug": schema.StringAttribute{
				MarkdownDescription: "The organization slug (read-only).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"avatar_url": schema.StringAttribute{
				MarkdownDescription: "The organization avatar URL.",
				Optional:            true,
			},
			"email": schema.StringAttribute{
				MarkdownDescription: "The organization contact email.",
				Optional:            true,
			},
			"website": schema.StringAttribute{
				MarkdownDescription: "The organization website URL.",
				Optional:            true,
			},
			"socials": schema.ListNestedAttribute{
				MarkdownDescription: "List of social links for the organization.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"platform": schema.StringAttribute{
							MarkdownDescription: "The social platform. Must be one of: `x`, `github`, `facebook`, `instagram`, `youtube`, `tiktok`, `linkedin`, `other`.",
							Required:            true,
							Validators: []validator.String{
								stringvalidator.OneOf("x", "github", "facebook", "instagram", "youtube", "tiktok", "linkedin", "other"),
							},
						},
						"url": schema.StringAttribute{
							MarkdownDescription: "The URL for the social link.",
							Required:            true,
						},
					},
				},
			},
			"feature_settings": schema.SingleNestedAttribute{
				MarkdownDescription: "Feature flags for the organization. Omit to leave feature settings unmanaged.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"issue_funding_enabled": schema.BoolAttribute{
						MarkdownDescription: "Whether issue funding is enabled.",
						Required:            true,
					},
					"seat_based_pricing_enabled": schema.BoolAttribute{
						MarkdownDescription: "Whether seat-based pricing is enabled.",
						Required:            true,
					},
					"revops_enabled": schema.BoolAttribute{
						MarkdownDescription: "Whether RevOps features are enabled.",
						Required:            true,
					},
					"wallets_enabled": schema.BoolAttribute{
						MarkdownDescription: "Whether wallets are enabled.",
						Required:            true,
					},
				},
			},
			"subscription_settings": schema.SingleNestedAttribute{
				MarkdownDescription: "Subscription behavior settings. Omit to leave subscription settings unmanaged.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"allow_multiple_subscriptions": schema.BoolAttribute{
						MarkdownDescription: "Whether customers can hold multiple active subscriptions.",
						Required:            true,
					},
					"allow_customer_updates": schema.BoolAttribute{
						MarkdownDescription: "Whether customers can self-manage their subscriptions.",
						Required:            true,
					},
					"proration_behavior": schema.StringAttribute{
						MarkdownDescription: "How mid-cycle subscription changes are billed. Must be `invoice` or `prorate`.",
						Required:            true,
						Validators: []validator.String{
							stringvalidator.OneOf("invoice", "prorate"),
						},
					},
					"benefit_revocation_grace_period": schema.Int64Attribute{
						MarkdownDescription: "Number of days before benefits are revoked after subscription cancellation.",
						Required:            true,
					},
					"prevent_trial_abuse": schema.BoolAttribute{
						MarkdownDescription: "Whether to prevent trial abuse by restricting repeat trials.",
						Required:            true,
					},
				},
			},
			"notification_settings": schema.SingleNestedAttribute{
				MarkdownDescription: "Email notification preferences for the organization. Omit to leave notification settings unmanaged.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"new_order": schema.BoolAttribute{
						MarkdownDescription: "Whether to send notifications for new orders.",
						Required:            true,
					},
					"new_subscription": schema.BoolAttribute{
						MarkdownDescription: "Whether to send notifications for new subscriptions.",
						Required:            true,
					},
				},
			},
			"customer_email_settings": schema.SingleNestedAttribute{
				MarkdownDescription: "Controls which transactional emails are sent to customers. Omit to leave customer email settings unmanaged.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"order_confirmation": schema.BoolAttribute{
						MarkdownDescription: "Whether to send order confirmation emails.",
						Required:            true,
					},
					"subscription_cancellation": schema.BoolAttribute{
						MarkdownDescription: "Whether to send subscription cancellation emails.",
						Required:            true,
					},
					"subscription_confirmation": schema.BoolAttribute{
						MarkdownDescription: "Whether to send subscription confirmation emails.",
						Required:            true,
					},
					"subscription_cycled": schema.BoolAttribute{
						MarkdownDescription: "Whether to send subscription renewal emails.",
						Required:            true,
					},
					"subscription_past_due": schema.BoolAttribute{
						MarkdownDescription: "Whether to send subscription past-due emails.",
						Required:            true,
					},
					"subscription_revoked": schema.BoolAttribute{
						MarkdownDescription: "Whether to send subscription revoked emails.",
						Required:            true,
					},
					"subscription_uncanceled": schema.BoolAttribute{
						MarkdownDescription: "Whether to send subscription uncanceled emails.",
						Required:            true,
					},
					"subscription_updated": schema.BoolAttribute{
						MarkdownDescription: "Whether to send subscription updated emails.",
						Required:            true,
					},
				},
			},
		},
	}
}

func (r *OrganizationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	pd, ok := req.ProviderData.(*PolarProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *PolarProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.provider = pd
}

func (r *OrganizationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OrganizationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	plannedWebsite := data.Website

	// Discover the single org scoped to the access token
	org, err := discoverOrganization(ctx, r.provider)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error discovering organization",
			fmt.Sprintf("Could not discover organization: %s", err),
		)
		return
	}

	update, diags := buildOrganizationUpdate(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err = r.provider.Client.Organizations.Update(ctx, org.ID, *update)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating organization",
			fmt.Sprintf("Could not update organization %s: %s", org.ID, err),
		)
		return
	}

	// Supplemental raw HTTP for subscription_settings (SDK missing prevent_trial_abuse)
	if data.SubscriptionSettings != nil {
		payload := buildSupplementalPayload(&data)
		if err := patchOrgSupplemental(ctx, r.provider.ServerURL, r.provider.AccessToken, org.ID, payload); err != nil {
			resp.Diagnostics.AddError(
				"Error updating subscription settings",
				fmt.Sprintf("Could not update subscription settings: %s", err),
			)
			return
		}
	}

	// Poll until the read-back matches planned values (eventual consistency)
	consistent, err := r.waitForReadConsistency(ctx, org.ID, &data, &resp.Diagnostics)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading organization after update",
			fmt.Sprintf("Could not read organization %s: %s", org.ID, err),
		)
		return
	}

	tflog.Trace(ctx, "adopted organization", map[string]interface{}{
		"id": consistent.ID,
	})

	mapOrganizationResponseToState(ctx, consistent, &data, &resp.Diagnostics)
	if data.SubscriptionSettings != nil {
		mapSupplementalSubscriptionSettings(consistent, &data)
	}
	preserveURLFormatting(&data.Website, plannedWebsite)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OrganizationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	priorWebsite := data.Website

	result, err := r.provider.Client.Organizations.Get(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading organization",
			fmt.Sprintf("Could not read organization %s: %s", data.ID.ValueString(), err),
		)
		return
	}

	mapOrganizationResponseToState(ctx, result.Organization, &data, &resp.Diagnostics)
	if data.SubscriptionSettings != nil {
		mapSupplementalSubscriptionSettings(result.Organization, &data)
	}
	preserveURLFormatting(&data.Website, priorWebsite)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data OrganizationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	plannedWebsite := data.Website

	update, diags := buildOrganizationUpdate(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.provider.Client.Organizations.Update(ctx, data.ID.ValueString(), *update)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating organization",
			fmt.Sprintf("Could not update organization %s: %s", data.ID.ValueString(), err),
		)
		return
	}

	// Supplemental raw HTTP for subscription_settings (SDK missing prevent_trial_abuse)
	if data.SubscriptionSettings != nil {
		payload := buildSupplementalPayload(&data)
		if err := patchOrgSupplemental(ctx, r.provider.ServerURL, r.provider.AccessToken, data.ID.ValueString(), payload); err != nil {
			resp.Diagnostics.AddError(
				"Error updating subscription settings",
				fmt.Sprintf("Could not update subscription settings: %s", err),
			)
			return
		}
	}

	// Poll until the read-back matches planned values (eventual consistency)
	consistent, err := r.waitForReadConsistency(ctx, data.ID.ValueString(), &data, &resp.Diagnostics)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading organization after update",
			fmt.Sprintf("Could not read organization %s: %s", data.ID.ValueString(), err),
		)
		return
	}

	mapOrganizationResponseToState(ctx, consistent, &data, &resp.Diagnostics)
	if data.SubscriptionSettings != nil {
		mapSupplementalSubscriptionSettings(consistent, &data)
	}
	preserveURLFormatting(&data.Website, plannedWebsite)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Organization cannot be deleted — just remove from state
	tflog.Trace(ctx, "organization released from Terraform management")
}

func (r *OrganizationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// waitForReadConsistency polls Organizations.Get until the response matches
// the planned values, handling eventual consistency after writes. Returns the
// first consistent response. If consistency is not reached after polling,
// returns the last response and emits a warning diagnostic.
func (r *OrganizationResource) waitForReadConsistency(ctx context.Context, orgID string, planned *OrganizationResourceModel, diags *diag.Diagnostics) (*components.Organization, error) {
	var lastOrg *components.Organization

	for i := 0; i < pollMaxAttempts; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(pollInterval):
			}
		}

		result, err := r.provider.Client.Organizations.Get(ctx, orgID)
		if err != nil {
			return nil, err
		}
		lastOrg = result.Organization

		if r.isConsistent(lastOrg, planned) {
			tflog.Trace(ctx, "read-after-write consistent", map[string]interface{}{
				"polls": i + 1,
			})
			return lastOrg, nil
		}
	}

	tflog.Warn(ctx, "read-after-write consistency not reached, using last response", map[string]interface{}{
		"polls": pollMaxAttempts,
	})
	diags.AddWarning(
		"Eventual consistency timeout",
		fmt.Sprintf(
			"Organization %s read-back did not match planned values after %d polls. "+
				"The state may not reflect the latest changes. Run terraform refresh to re-sync.",
			orgID, pollMaxAttempts,
		),
	)
	return lastOrg, nil
}

// isConsistent checks whether the API response matches the user-configured
// planned values for fields that may be subject to eventual consistency.
func (r *OrganizationResource) isConsistent(org *components.Organization, planned *OrganizationResourceModel) bool {
	if !planned.Name.IsNull() && org.Name != planned.Name.ValueString() {
		return false
	}
	if planned.FeatureSettings != nil && org.FeatureSettings != nil {
		if derefBool(org.FeatureSettings.IssueFundingEnabled) != planned.FeatureSettings.IssueFundingEnabled.ValueBool() {
			return false
		}
	}
	if planned.SubscriptionSettings != nil {
		if string(org.SubscriptionSettings.ProrationBehavior) != planned.SubscriptionSettings.ProrationBehavior.ValueString() {
			return false
		}
	}
	if planned.NotificationSettings != nil {
		if org.NotificationSettings.NewOrder != planned.NotificationSettings.NewOrder.ValueBool() {
			return false
		}
	}
	if planned.CustomerEmailSettings != nil {
		if org.CustomerEmailSettings.OrderConfirmation != planned.CustomerEmailSettings.OrderConfirmation.ValueBool() {
			return false
		}
	}
	return true
}

// preserveURLFormatting keeps the user's URL formatting when the API response
// differs only by a trailing slash (e.g. "https://example.com" vs "https://example.com/").
func preserveURLFormatting(current *types.String, prior types.String) {
	if current.IsNull() || prior.IsNull() {
		return
	}
	if strings.TrimRight(current.ValueString(), "/") == strings.TrimRight(prior.ValueString(), "/") {
		*current = prior
	}
}
