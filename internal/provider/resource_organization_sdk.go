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
	"net/url"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/polarsource/polar-go/models/components"
)

// supplementalHTTPClient is a dedicated client for raw HTTP calls that bypass the SDK.
// Uses an explicit timeout to prevent indefinite hangs.
var supplementalHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
}

// --- Discover the single organization scoped to the access token ---
// Polar access tokens are org-scoped, so listing orgs should return exactly one.

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
// Only sets fields the user included in their config (non-null).
// subscription_settings is excluded — it's handled via raw HTTP due to SDK gaps.

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

	// Customer email settings: handled via raw HTTP (SDK missing subscription_cycled_after_trial field)

	return &update, diags
}

// --- Map SDK Organization response to Terraform state ---
// Only populates fields the user opted into (non-null in state).
// This lets users manage a subset of settings without TF fighting over the rest.

func mapOrganizationResponseToState(ctx context.Context, org *components.Organization, data *OrganizationResourceModel, diags *diag.Diagnostics) {
	data.ID = types.StringValue(org.ID)
	data.Slug = types.StringValue(org.Slug)

	// Profile fields: only set if user included them in config.
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

	// Subscription settings: mapped via mapSupplementalSettings
	// (SDK missing prevent_trial_abuse, entire block sent via raw HTTP)

	if data.NotificationSettings != nil {
		data.NotificationSettings = &NotificationSettingsModel{
			NewOrder:        types.BoolValue(org.NotificationSettings.NewOrder),
			NewSubscription: types.BoolValue(org.NotificationSettings.NewSubscription),
		}
	}

	// Customer email settings: mapped via mapSupplementalSettings
	// (SDK missing subscription_cycled_after_trial field, same gap as subscription_settings)
}

// --- Raw HTTP for SDK gap workarounds ---
// The Speakeasy-generated SDK omits certain fields:
// - subscription_settings: missing `prevent_trial_abuse`
// - customer_email_settings: missing `subscription_cycled_after_trial`
// We bypass the SDK with raw HTTP PATCH/GET for these settings blocks.

type subscriptionSettingsJSON struct {
	AllowMultipleSubscriptions   bool   `json:"allow_multiple_subscriptions"`
	AllowCustomerUpdates         bool   `json:"allow_customer_updates"`
	ProrationBehavior            string `json:"proration_behavior"`
	BenefitRevocationGracePeriod int64  `json:"benefit_revocation_grace_period"`
	PreventTrialAbuse            bool   `json:"prevent_trial_abuse"`
}

type customerEmailSettingsJSON struct {
	OrderConfirmation            bool `json:"order_confirmation"`
	SubscriptionCancellation     bool `json:"subscription_cancellation"`
	SubscriptionConfirmation     bool `json:"subscription_confirmation"`
	SubscriptionCycled           bool `json:"subscription_cycled"`
	SubscriptionCycledAfterTrial bool `json:"subscription_cycled_after_trial"`
	SubscriptionPastDue          bool `json:"subscription_past_due"`
	SubscriptionRevoked          bool `json:"subscription_revoked"`
	SubscriptionUncanceled       bool `json:"subscription_uncanceled"`
	SubscriptionUpdated          bool `json:"subscription_updated"`
}

// orgSupplementalUpdatePayload is the raw HTTP PATCH body for fields the SDK doesn't support.
type orgSupplementalUpdatePayload struct {
	SubscriptionSettings  *subscriptionSettingsJSON  `json:"subscription_settings,omitempty"`
	CustomerEmailSettings *customerEmailSettingsJSON `json:"customer_email_settings,omitempty"`
}

// orgSupplementalGetResponse extracts settings from the org GET response.
type orgSupplementalGetResponse struct {
	SubscriptionSettings  *subscriptionSettingsJSON  `json:"subscription_settings"`
	CustomerEmailSettings *customerEmailSettingsJSON `json:"customer_email_settings"`
}

// buildSupplementalPayload builds the raw HTTP payload for settings the SDK can't send.
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
	if data.CustomerEmailSettings != nil {
		payload.CustomerEmailSettings = &customerEmailSettingsJSON{
			OrderConfirmation:            data.CustomerEmailSettings.OrderConfirmation.ValueBool(),
			SubscriptionCancellation:     data.CustomerEmailSettings.SubscriptionCancellation.ValueBool(),
			SubscriptionConfirmation:     data.CustomerEmailSettings.SubscriptionConfirmation.ValueBool(),
			SubscriptionCycled:           data.CustomerEmailSettings.SubscriptionCycled.ValueBool(),
			SubscriptionCycledAfterTrial: data.CustomerEmailSettings.SubscriptionCycledAfterTrial.ValueBool(),
			SubscriptionPastDue:          data.CustomerEmailSettings.SubscriptionPastDue.ValueBool(),
			SubscriptionRevoked:          data.CustomerEmailSettings.SubscriptionRevoked.ValueBool(),
			SubscriptionUncanceled:       data.CustomerEmailSettings.SubscriptionUncanceled.ValueBool(),
			SubscriptionUpdated:          data.CustomerEmailSettings.SubscriptionUpdated.ValueBool(),
		}
	}
	return payload
}

// mapSupplementalSettings reads fields the SDK omits from the API via raw HTTP
// GET and updates state. Handles both subscription_settings and customer_email_settings.
func mapSupplementalSettings(ctx context.Context, serverURL, token, orgID string, data *OrganizationResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if data.SubscriptionSettings == nil && data.CustomerEmailSettings == nil {
		return diags
	}

	supplemental, err := getOrgSupplemental(ctx, serverURL, token, orgID)
	if err != nil {
		diags.AddWarning(
			"Could not read supplemental settings",
			fmt.Sprintf("Failed to read supplemental settings via raw HTTP: %s. Values in state may be stale.", err),
		)
		return diags
	}

	if data.SubscriptionSettings != nil && supplemental.SubscriptionSettings != nil {
		ss := supplemental.SubscriptionSettings
		data.SubscriptionSettings = &SubscriptionSettingsModel{
			AllowMultipleSubscriptions:   types.BoolValue(ss.AllowMultipleSubscriptions),
			AllowCustomerUpdates:         types.BoolValue(ss.AllowCustomerUpdates),
			ProrationBehavior:            types.StringValue(ss.ProrationBehavior),
			BenefitRevocationGracePeriod: types.Int64Value(ss.BenefitRevocationGracePeriod),
			PreventTrialAbuse:            types.BoolValue(ss.PreventTrialAbuse),
		}
	}
	if data.CustomerEmailSettings != nil && supplemental.CustomerEmailSettings != nil {
		ces := supplemental.CustomerEmailSettings
		data.CustomerEmailSettings = &CustomerEmailSettingsModel{
			OrderConfirmation:            types.BoolValue(ces.OrderConfirmation),
			SubscriptionCancellation:     types.BoolValue(ces.SubscriptionCancellation),
			SubscriptionConfirmation:     types.BoolValue(ces.SubscriptionConfirmation),
			SubscriptionCycled:           types.BoolValue(ces.SubscriptionCycled),
			SubscriptionCycledAfterTrial: types.BoolValue(ces.SubscriptionCycledAfterTrial),
			SubscriptionPastDue:          types.BoolValue(ces.SubscriptionPastDue),
			SubscriptionRevoked:          types.BoolValue(ces.SubscriptionRevoked),
			SubscriptionUncanceled:       types.BoolValue(ces.SubscriptionUncanceled),
			SubscriptionUpdated:          types.BoolValue(ces.SubscriptionUpdated),
		}
	}
	return diags
}

// patchOrgSupplemental sends a raw HTTP PATCH with retry for fields the SDK doesn't support.
func patchOrgSupplemental(ctx context.Context, serverURL, token, orgID string, payload *orgSupplementalUpdatePayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling supplemental settings: %w", err)
	}

	return doWithRetry(ctx, func() (*http.Response, error) {
		url := fmt.Sprintf("%s/v1/organizations/%s", serverURL, url.PathEscape(orgID))
		req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		return supplementalHTTPClient.Do(req)
	})
}

// getOrgSupplemental reads settings the SDK omits via raw HTTP GET with retry.
func getOrgSupplemental(ctx context.Context, serverURL, token, orgID string) (*orgSupplementalGetResponse, error) {
	var result orgSupplementalGetResponse

	err := doWithRetry(ctx, func() (*http.Response, error) {
		reqURL := fmt.Sprintf("%s/v1/organizations/%s", serverURL, url.PathEscape(orgID))
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := supplementalHTTPClient.Do(req)
		if err != nil {
			return nil, err
		}
		// On success, decode and let doWithRetry handle body close
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				return nil, fmt.Errorf("reading response body: %w", readErr)
			}
			if decodeErr := json.Unmarshal(body, &result); decodeErr != nil {
				return nil, fmt.Errorf("decoding response: %w", decodeErr)
			}
			// Return nil response — body already consumed and closed
			return nil, nil
		}
		return resp, nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// doWithRetry executes fn with exponential backoff on 429 (rate limit) and 5xx
// (server errors). If fn returns (nil, nil), it means the caller already consumed
// and closed the response body on success (see getOrgSupplemental).
func doWithRetry(ctx context.Context, fn func() (*http.Response, error)) error {
	const maxAttempts = 5
	const initialBackoff = 500 * time.Millisecond

	for attempt := 0; attempt < maxAttempts; attempt++ {
		resp, err := fn()
		if err != nil {
			return err
		}

		// nil response means caller already handled a successful response
		if resp == nil {
			return nil
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			resp.Body.Close()
			return nil
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			respBody = []byte("(failed to read response body)")
		}

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

		// Log full response body at debug level; return only status in user-facing error
		// to avoid leaking internal API details.
		tflog.Debug(ctx, "supplemental HTTP error response", map[string]interface{}{
			"status": resp.StatusCode,
			"body":   string(respBody),
		})
		return fmt.Errorf("supplemental HTTP request failed with status %d", resp.StatusCode)
	}
	return fmt.Errorf("max retries exceeded")
}

// --- Helpers ---

func socialModelAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"platform": types.StringType,
		"url":      types.StringType,
	}
}
