// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/polarsource/polar-go/models/components"
)

var _ datasource.DataSource = &OrganizationDataSource{}

func NewOrganizationDataSource() datasource.DataSource {
	return &OrganizationDataSource{}
}

type OrganizationDataSource struct {
	provider *PolarProviderData
}

type OrganizationDataSourceModel struct {
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

func (d *OrganizationDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization"
}

func (d *OrganizationDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches the Polar organization associated with the configured access token. " +
			"If `id` is omitted, the organization is auto-discovered.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The organization ID. If omitted, auto-discovers via the access token.",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the organization.",
				Computed:            true,
			},
			"slug": schema.StringAttribute{
				MarkdownDescription: "The organization slug.",
				Computed:            true,
			},
			"avatar_url": schema.StringAttribute{
				MarkdownDescription: "The organization avatar URL.",
				Computed:            true,
			},
			"email": schema.StringAttribute{
				MarkdownDescription: "The organization contact email.",
				Computed:            true,
			},
			"website": schema.StringAttribute{
				MarkdownDescription: "The organization website URL.",
				Computed:            true,
			},
			"socials": schema.ListNestedAttribute{
				MarkdownDescription: "List of social links for the organization.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"platform": schema.StringAttribute{
							MarkdownDescription: "The social platform.",
							Computed:            true,
						},
						"url": schema.StringAttribute{
							MarkdownDescription: "The URL for the social link.",
							Computed:            true,
						},
					},
				},
			},
			"feature_settings": schema.SingleNestedAttribute{
				MarkdownDescription: "Feature flags for the organization.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"issue_funding_enabled": schema.BoolAttribute{
						MarkdownDescription: "Whether issue funding is enabled.",
						Computed:            true,
					},
					"seat_based_pricing_enabled": schema.BoolAttribute{
						MarkdownDescription: "Whether seat-based pricing is enabled.",
						Computed:            true,
					},
					"revops_enabled": schema.BoolAttribute{
						MarkdownDescription: "Whether RevOps features are enabled.",
						Computed:            true,
					},
					"wallets_enabled": schema.BoolAttribute{
						MarkdownDescription: "Whether wallets are enabled.",
						Computed:            true,
					},
				},
			},
			"subscription_settings": schema.SingleNestedAttribute{
				MarkdownDescription: "Subscription behavior settings.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"allow_multiple_subscriptions": schema.BoolAttribute{
						MarkdownDescription: "Whether customers can hold multiple active subscriptions.",
						Computed:            true,
					},
					"allow_customer_updates": schema.BoolAttribute{
						MarkdownDescription: "Whether customers can self-manage their subscriptions.",
						Computed:            true,
					},
					"proration_behavior": schema.StringAttribute{
						MarkdownDescription: "How mid-cycle subscription changes are billed.",
						Computed:            true,
					},
					"benefit_revocation_grace_period": schema.Int64Attribute{
						MarkdownDescription: "Number of days before benefits are revoked after subscription cancellation.",
						Computed:            true,
					},
					"prevent_trial_abuse": schema.BoolAttribute{
						MarkdownDescription: "Whether to prevent trial abuse by restricting repeat trials.",
						Computed:            true,
					},
				},
			},
			"notification_settings": schema.SingleNestedAttribute{
				MarkdownDescription: "Email notification preferences.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"new_order": schema.BoolAttribute{
						MarkdownDescription: "Whether to send notifications for new orders.",
						Computed:            true,
					},
					"new_subscription": schema.BoolAttribute{
						MarkdownDescription: "Whether to send notifications for new subscriptions.",
						Computed:            true,
					},
				},
			},
			"customer_email_settings": schema.SingleNestedAttribute{
				MarkdownDescription: "Controls which transactional emails are sent to customers.",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"order_confirmation": schema.BoolAttribute{
						MarkdownDescription: "Whether to send order confirmation emails.",
						Computed:            true,
					},
					"subscription_cancellation": schema.BoolAttribute{
						MarkdownDescription: "Whether to send subscription cancellation emails.",
						Computed:            true,
					},
					"subscription_confirmation": schema.BoolAttribute{
						MarkdownDescription: "Whether to send subscription confirmation emails.",
						Computed:            true,
					},
					"subscription_cycled": schema.BoolAttribute{
						MarkdownDescription: "Whether to send subscription renewal emails.",
						Computed:            true,
					},
					"subscription_past_due": schema.BoolAttribute{
						MarkdownDescription: "Whether to send subscription past-due emails.",
						Computed:            true,
					},
					"subscription_revoked": schema.BoolAttribute{
						MarkdownDescription: "Whether to send subscription revoked emails.",
						Computed:            true,
					},
					"subscription_uncanceled": schema.BoolAttribute{
						MarkdownDescription: "Whether to send subscription uncanceled emails.",
						Computed:            true,
					},
					"subscription_updated": schema.BoolAttribute{
						MarkdownDescription: "Whether to send subscription updated emails.",
						Computed:            true,
					},
				},
			},
		},
	}
}

func (d *OrganizationDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if pd := extractProviderData(req.ProviderData, &resp.Diagnostics); pd != nil {
		d.provider = pd
	}
}

func (d *OrganizationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OrganizationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If ID is not provided, auto-discover
	if data.ID.IsNull() || data.ID.IsUnknown() {
		org, err := discoverOrganization(ctx, d.provider)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error discovering organization",
				fmt.Sprintf("Could not discover organization: %s", err),
			)
			return
		}
		data.ID = types.StringValue(org.ID)
	}

	result, err := d.provider.Client.Organizations.Get(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading organization",
			fmt.Sprintf("Could not read organization %s: %s", data.ID.ValueString(), err),
		)
		return
	}

	mapOrganizationDataSourceResponseToState(ctx, result.Organization, &data, &resp.Diagnostics)

	// Read prevent_trial_abuse via raw HTTP (not in SDK response struct)
	supplemental, err := getOrgSupplemental(ctx, d.provider.ServerURL, d.provider.AccessToken, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading subscription settings",
			fmt.Sprintf("Could not read subscription settings: %s", err),
		)
		return
	}
	if data.SubscriptionSettings != nil && supplemental.SubscriptionSettings != nil {
		data.SubscriptionSettings.PreventTrialAbuse = types.BoolValue(supplemental.SubscriptionSettings.PreventTrialAbuse)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// mapOrganizationDataSourceResponseToState populates all fields unconditionally (for data sources).
func mapOrganizationDataSourceResponseToState(ctx context.Context, org *components.Organization, data *OrganizationDataSourceModel, diags *diag.Diagnostics) {
	data.ID = types.StringValue(org.ID)
	data.Name = types.StringValue(org.Name)
	data.Slug = types.StringValue(org.Slug)
	data.AvatarURL = optionalStringValue(org.AvatarURL)
	data.Email = optionalStringValue(org.Email)
	data.Website = optionalStringValue(org.Website)

	// Socials
	socialModels := make([]SocialModel, len(org.Socials))
	for i, s := range org.Socials {
		socialModels[i] = SocialModel{
			Platform: types.StringValue(string(s.Platform)),
			URL:      types.StringValue(s.URL),
		}
	}
	socialList, d := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: socialModelAttrTypes()}, socialModels)
	diags.Append(d...)
	data.Socials = socialList

	// Feature settings
	if org.FeatureSettings != nil {
		data.FeatureSettings = &FeatureSettingsModel{
			IssueFundingEnabled:     types.BoolValue(derefBool(org.FeatureSettings.IssueFundingEnabled)),
			SeatBasedPricingEnabled: types.BoolValue(derefBool(org.FeatureSettings.SeatBasedPricingEnabled)),
			RevopsEnabled:           types.BoolValue(derefBool(org.FeatureSettings.RevopsEnabled)),
			WalletsEnabled:          types.BoolValue(derefBool(org.FeatureSettings.WalletsEnabled)),
		}
	}

	// Subscription settings (prevent_trial_abuse set separately via supplemental read)
	ss := org.SubscriptionSettings
	data.SubscriptionSettings = &SubscriptionSettingsModel{
		AllowMultipleSubscriptions:   types.BoolValue(ss.AllowMultipleSubscriptions),
		AllowCustomerUpdates:         types.BoolValue(ss.AllowCustomerUpdates),
		ProrationBehavior:            types.StringValue(string(ss.ProrationBehavior)),
		BenefitRevocationGracePeriod: types.Int64Value(ss.BenefitRevocationGracePeriod),
		PreventTrialAbuse:            types.BoolValue(false), // overwritten by supplemental read
	}

	data.NotificationSettings = &NotificationSettingsModel{
		NewOrder:        types.BoolValue(org.NotificationSettings.NewOrder),
		NewSubscription: types.BoolValue(org.NotificationSettings.NewSubscription),
	}

	data.CustomerEmailSettings = &CustomerEmailSettingsModel{
		OrderConfirmation:        types.BoolValue(org.CustomerEmailSettings.OrderConfirmation),
		SubscriptionCancellation: types.BoolValue(org.CustomerEmailSettings.SubscriptionCancellation),
		SubscriptionConfirmation: types.BoolValue(org.CustomerEmailSettings.SubscriptionConfirmation),
		SubscriptionCycled:       types.BoolValue(org.CustomerEmailSettings.SubscriptionCycled),
		SubscriptionPastDue:      types.BoolValue(org.CustomerEmailSettings.SubscriptionPastDue),
		SubscriptionRevoked:      types.BoolValue(org.CustomerEmailSettings.SubscriptionRevoked),
		SubscriptionUncanceled:   types.BoolValue(org.CustomerEmailSettings.SubscriptionUncanceled),
		SubscriptionUpdated:      types.BoolValue(org.CustomerEmailSettings.SubscriptionUpdated),
	}
}
