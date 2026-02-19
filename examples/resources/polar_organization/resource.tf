# Adopt and configure the organization scoped to the access token
resource "polar_organization" "main" {
  name    = "My Organization"
  website = "https://example.com"

  feature_settings = {
    issue_funding_enabled      = false
    seat_based_pricing_enabled = false
    revops_enabled             = false
    wallets_enabled            = false
  }

  subscription_settings = {
    allow_multiple_subscriptions    = true
    allow_customer_updates          = true
    proration_behavior              = "prorate"
    benefit_revocation_grace_period = 7
    prevent_trial_abuse             = true
  }
}
