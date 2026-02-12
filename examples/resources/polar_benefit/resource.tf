# Custom benefit with a note
resource "polar_benefit" "welcome_email" {
  type        = "custom"
  description = "Welcome email with onboarding instructions"

  custom_properties = {
    note = "Check your email for a welcome message with setup instructions."
  }
}

# Meter credit benefit (requires a polar_meter resource)
resource "polar_benefit" "api_credits" {
  type        = "meter_credit"
  description = "100 API credits per month"

  meter_credit_properties = {
    meter_id = polar_meter.api_calls.id
    units    = 100
    rollover = false
  }
}

# License keys benefit
resource "polar_benefit" "software_license" {
  type        = "license_keys"
  description = "Software license key"

  license_keys_properties = {
    prefix      = "MYAPP"
    limit_usage = 3

    activations = {
      limit               = 5
      enable_customer_admin = true
    }
  }
}
