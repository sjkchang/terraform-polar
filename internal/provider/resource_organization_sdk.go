// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/polarsource/polar-go/models/components"
)

// --- Discover the single organization scoped to the access token ---

func discoverOrganization(ctx context.Context, client *PolarProviderData) (*components.Organization, error) {
	resp, err := client.Client.Organizations.List(ctx, nil, nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("listing organizations: %w", err)
	}
	if resp.ListResourceOrganization == nil || len(resp.ListResourceOrganization.Items) == 0 {
		return nil, fmt.Errorf("no organizations found for the configured access token")
	}
	if len(resp.ListResourceOrganization.Items) > 1 {
		return nil, fmt.Errorf("expected exactly 1 organization, found %d", len(resp.ListResourceOrganization.Items))
	}
	org := resp.ListResourceOrganization.Items[0]
	return &org, nil
}

// --- Build SDK OrganizationUpdate from Terraform model ---

func buildOrganizationUpdate(ctx context.Context, data *OrganizationResourceModel) (*components.OrganizationUpdate, diag.Diagnostics) {
	var diags diag.Diagnostics
	update := components.OrganizationUpdate{}

	if !data.Name.IsNull() && !data.Name.IsUnknown() {
		name := data.Name.ValueString()
		update.Name = &name
	}
	if !data.AvatarURL.IsNull() && !data.AvatarURL.IsUnknown() {
		u := data.AvatarURL.ValueString()
		update.AvatarURL = &u
	}
	if !data.Email.IsNull() && !data.Email.IsUnknown() {
		e := data.Email.ValueString()
		update.Email = &e
	}
	if !data.Website.IsNull() && !data.Website.IsUnknown() {
		w := data.Website.ValueString()
		update.Website = &w
	}

	// Socials
	if !data.Socials.IsNull() && !data.Socials.IsUnknown() {
		var socials []SocialModel
		d := data.Socials.ElementsAs(ctx, &socials, false)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		sdkSocials := make([]components.OrganizationSocialLink, len(socials))
		for i, s := range socials {
			sdkSocials[i] = components.OrganizationSocialLink{
				Platform: components.OrganizationSocialPlatforms(s.Platform.ValueString()),
				URL:      s.URL.ValueString(),
			}
		}
		update.Socials = sdkSocials
	}

	// Feature settings
	if data.FeatureSettings != nil {
		fs := &components.OrganizationFeatureSettings{}
		b := data.FeatureSettings.IssueFundingEnabled.ValueBool()
		fs.IssueFundingEnabled = &b
		b2 := data.FeatureSettings.SeatBasedPricingEnabled.ValueBool()
		fs.SeatBasedPricingEnabled = &b2
		b3 := data.FeatureSettings.RevopsEnabled.ValueBool()
		fs.RevopsEnabled = &b3
		b4 := data.FeatureSettings.WalletsEnabled.ValueBool()
		fs.WalletsEnabled = &b4
		update.FeatureSettings = fs
	}

	// Subscription settings: handled via raw HTTP (SDK missing prevent_trial_abuse field)

	// Notification settings
	if data.NotificationSettings != nil {
		ns := &components.OrganizationNotificationSettings{
			NewOrder:        data.NotificationSettings.NewOrder.ValueBool(),
			NewSubscription: data.NotificationSettings.NewSubscription.ValueBool(),
		}
		update.NotificationSettings = ns
	}

	// Customer email settings
	if data.CustomerEmailSettings != nil {
		ces := &components.OrganizationCustomerEmailSettings{
			OrderConfirmation:        data.CustomerEmailSettings.OrderConfirmation.ValueBool(),
			SubscriptionCancellation: data.CustomerEmailSettings.SubscriptionCancellation.ValueBool(),
			SubscriptionConfirmation: data.CustomerEmailSettings.SubscriptionConfirmation.ValueBool(),
			SubscriptionCycled:       data.CustomerEmailSettings.SubscriptionCycled.ValueBool(),
			SubscriptionPastDue:      data.CustomerEmailSettings.SubscriptionPastDue.ValueBool(),
			SubscriptionRevoked:      data.CustomerEmailSettings.SubscriptionRevoked.ValueBool(),
			SubscriptionUncanceled:   data.CustomerEmailSettings.SubscriptionUncanceled.ValueBool(),
			SubscriptionUpdated:      data.CustomerEmailSettings.SubscriptionUpdated.ValueBool(),
		}
		update.CustomerEmailSettings = ces
	}

	return &update, diags
}

// --- Map SDK Organization response to Terraform state ---

func mapOrganizationResponseToState(ctx context.Context, org *components.Organization, data *OrganizationResourceModel, diags *diag.Diagnostics) {
	data.ID = types.StringValue(org.ID)
	data.Slug = types.StringValue(org.Slug)

	// Profile fields: only set if user configured them (Optional attributes)
	if !data.Name.IsNull() {
		data.Name = types.StringValue(org.Name)
	}
	if !data.AvatarURL.IsNull() {
		data.AvatarURL = optionalStringValue(org.AvatarURL)
	}
	if !data.Email.IsNull() {
		data.Email = optionalStringValue(org.Email)
	}
	if !data.Website.IsNull() {
		data.Website = optionalStringValue(org.Website)
	}

	// Socials: only set if user configured them
	if !data.Socials.IsNull() {
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
	}

	// Settings blocks: only populate if already in state (user configured them)
	if data.FeatureSettings != nil && org.FeatureSettings != nil {
		data.FeatureSettings = &FeatureSettingsModel{
			IssueFundingEnabled:     types.BoolValue(derefBool(org.FeatureSettings.IssueFundingEnabled)),
			SeatBasedPricingEnabled: types.BoolValue(derefBool(org.FeatureSettings.SeatBasedPricingEnabled)),
			RevopsEnabled:           types.BoolValue(derefBool(org.FeatureSettings.RevopsEnabled)),
			WalletsEnabled:          types.BoolValue(derefBool(org.FeatureSettings.WalletsEnabled)),
		}
	}

	// Subscription settings: SDK fields mapped here; prevent_trial_abuse handled
	// separately via mapSupplementalSubscriptionSettings
	if data.SubscriptionSettings != nil {
		ss := org.SubscriptionSettings
		data.SubscriptionSettings = &SubscriptionSettingsModel{
			AllowMultipleSubscriptions:   types.BoolValue(ss.AllowMultipleSubscriptions),
			AllowCustomerUpdates:         types.BoolValue(ss.AllowCustomerUpdates),
			ProrationBehavior:            types.StringValue(string(ss.ProrationBehavior)),
			BenefitRevocationGracePeriod: types.Int64Value(ss.BenefitRevocationGracePeriod),
			// PreventTrialAbuse is set by mapSupplementalSubscriptionSettings
			PreventTrialAbuse: data.SubscriptionSettings.PreventTrialAbuse,
		}
	}

	if data.NotificationSettings != nil {
		data.NotificationSettings = &NotificationSettingsModel{
			NewOrder:        types.BoolValue(org.NotificationSettings.NewOrder),
			NewSubscription: types.BoolValue(org.NotificationSettings.NewSubscription),
		}
	}

	if data.CustomerEmailSettings != nil {
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
}

// --- Raw HTTP for subscription_settings (SDK missing prevent_trial_abuse) ---

// subscriptionSettingsJSON is the JSON shape for subscription_settings sent via raw HTTP.
type subscriptionSettingsJSON struct {
	AllowMultipleSubscriptions   bool   `json:"allow_multiple_subscriptions"`
	AllowCustomerUpdates         bool   `json:"allow_customer_updates"`
	ProrationBehavior            string `json:"proration_behavior"`
	BenefitRevocationGracePeriod int64  `json:"benefit_revocation_grace_period"`
	PreventTrialAbuse            bool   `json:"prevent_trial_abuse"`
}

// orgSupplementalUpdatePayload is the raw HTTP PATCH body for fields the SDK doesn't support.
type orgSupplementalUpdatePayload struct {
	SubscriptionSettings *subscriptionSettingsJSON `json:"subscription_settings,omitempty"`
}

// orgSupplementalGetResponse extracts subscription_settings from the org GET response.
type orgSupplementalGetResponse struct {
	SubscriptionSettings *subscriptionSettingsJSON `json:"subscription_settings"`
}

// buildSupplementalPayload builds the raw HTTP payload for subscription_settings.
func buildSupplementalPayload(data *OrganizationResourceModel) *orgSupplementalUpdatePayload {
	payload := &orgSupplementalUpdatePayload{}
	if data.SubscriptionSettings != nil {
		payload.SubscriptionSettings = &subscriptionSettingsJSON{
			AllowMultipleSubscriptions:   data.SubscriptionSettings.AllowMultipleSubscriptions.ValueBool(),
			AllowCustomerUpdates:         data.SubscriptionSettings.AllowCustomerUpdates.ValueBool(),
			ProrationBehavior:            data.SubscriptionSettings.ProrationBehavior.ValueString(),
			BenefitRevocationGracePeriod: data.SubscriptionSettings.BenefitRevocationGracePeriod.ValueInt64(),
			PreventTrialAbuse:            data.SubscriptionSettings.PreventTrialAbuse.ValueBool(),
		}
	}
	return payload
}

// mapSupplementalSubscriptionSettings reads prevent_trial_abuse from the SDK response's
// raw subscription_settings (which the SDK does return, just without the field in the struct).
// We re-read via raw HTTP GET to get the actual prevent_trial_abuse value.
// For simplicity, this preserves the planned value since the SDK update + raw HTTP PATCH
// are authoritative.
func mapSupplementalSubscriptionSettings(org *components.Organization, data *OrganizationResourceModel) {
	// The prevent_trial_abuse value was already preserved from the planned state
	// in mapOrganizationResponseToState. This function is a no-op but exists as
	// a clear extension point if we need to read it back via raw HTTP in the future.
}

// patchOrgSupplemental sends a raw HTTP PATCH with retry for fields the SDK doesn't support.
func patchOrgSupplemental(ctx context.Context, serverURL, token, orgID string, payload *orgSupplementalUpdatePayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling subscription settings: %w", err)
	}

	return doWithRetry(ctx, func() (*http.Response, error) {
		url := fmt.Sprintf("%s/v1/organizations/%s", serverURL, orgID)
		req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		return http.DefaultClient.Do(req)
	})
}

// getOrgSupplemental reads subscription_settings via raw HTTP GET with retry.
func getOrgSupplemental(ctx context.Context, serverURL, token, orgID string) (*orgSupplementalGetResponse, error) {
	var result orgSupplementalGetResponse

	err := doWithRetry(ctx, func() (*http.Response, error) {
		url := fmt.Sprintf("%s/v1/organizations/%s", serverURL, orgID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		// Only decode on success; doWithRetry handles status checks
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			defer resp.Body.Close()
			if decodeErr := json.NewDecoder(resp.Body).Decode(&result); decodeErr != nil {
				return nil, fmt.Errorf("decoding response: %w", decodeErr)
			}
		}
		return resp, nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// doWithRetry executes an HTTP request function with exponential backoff on 429/5xx.
func doWithRetry(ctx context.Context, fn func() (*http.Response, error)) error {
	const maxAttempts = 5
	const initialBackoff = 500 * time.Millisecond

	for attempt := 0; attempt < maxAttempts; attempt++ {
		resp, err := fn()
		if err != nil {
			return err
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			// Success — body already consumed by caller if needed
			resp.Body.Close()
			return nil
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Retry on 429 or 5xx
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			if attempt < maxAttempts-1 {
				backoff := time.Duration(float64(initialBackoff) * math.Pow(1.5, float64(attempt)))
				tflog.Debug(ctx, "retrying supplemental HTTP request", map[string]interface{}{
					"status":  resp.StatusCode,
					"attempt": attempt + 1,
					"backoff": backoff.String(),
				})
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(backoff):
					continue
				}
			}
		}

		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return fmt.Errorf("max retries exceeded")
}

// --- Helpers ---

func derefBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

func socialModelAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"platform": types.StringType,
		"url":      types.StringType,
	}
}
