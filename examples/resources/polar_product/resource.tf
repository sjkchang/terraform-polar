# One-time product with a fixed price
resource "polar_product" "ebook" {
  name        = "Terraform Guide eBook"
  description = "A comprehensive guide to Terraform best practices."

  prices = [{
    amount_type  = "fixed"
    price_amount = 1999
  }]
}

# Free one-time product
resource "polar_product" "free_sample" {
  name = "Free Sample"

  prices = [{
    amount_type = "free"
  }]
}

# Monthly subscription product
resource "polar_product" "pro_plan" {
  name               = "Pro Plan"
  description        = "Access to all premium features."
  recurring_interval = "month"

  prices = [{
    amount_type  = "fixed"
    price_amount = 999
  }]

  metadata = {
    tier = "pro"
  }
}

# Pay-what-you-want product with custom pricing
resource "polar_product" "donation" {
  name        = "Support Us"
  description = "Pay what you want to support our work."

  prices = [{
    amount_type    = "custom"
    minimum_amount = 100
    maximum_amount = 100000
    preset_amount  = 1000
  }]
}

# Usage-based subscription with metered pricing
resource "polar_product" "api_access" {
  name               = "API Access"
  description        = "Pay per API call."
  recurring_interval = "month"

  prices = [{
    amount_type = "metered_unit"
    meter_id    = polar_meter.api_calls.id
    unit_amount = "0.01"
    cap_amount  = 50000
  }]
}
