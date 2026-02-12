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

# Seat-based subscription with tiered pricing
resource "polar_product" "team_plan" {
  name               = "Team Plan"
  description        = "Per-seat pricing with volume discounts."
  recurring_interval = "month"

  prices = [{
    amount_type = "seat_based"
    seat_tiers = [
      {
        min_seats      = 1
        max_seats      = 10
        price_per_seat = 1500
      },
      {
        min_seats      = 11
        price_per_seat = 1200
      }
    ]
  }]
}
