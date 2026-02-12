// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccProductResource_oneTimeFixed(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create one-time product with fixed price
			{
				Config: testAccProductOneTimeFixedConfig("tf-acc-test-product-fixed", 999),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_product.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("tf-acc-test-product-fixed"),
					),
	statecheck.ExpectKnownValue(
						"polar_product.test",
						tfjsonpath.New("prices"),
						knownvalue.ListSizeExact(1),
					),
				},
			},
			// ImportState
			{
				ResourceName:      "polar_product.test",
				ImportState:        true,
				ImportStateVerify:  true,
			},
			// Update name and price
			{
				Config: testAccProductOneTimeFixedConfig("tf-acc-test-product-updated", 1999),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_product.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("tf-acc-test-product-updated"),
					),
				},
			},
			// Delete (archive) testing automatically occurs in TestCase
		},
	})
}

func TestAccProductResource_oneTimeFree(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProductOneTimeFreeConfig("tf-acc-test-product-free"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_product.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("tf-acc-test-product-free"),
					),
				},
			},
		},
	})
}

func TestAccProductResource_recurring(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create recurring monthly product
			{
				Config: testAccProductRecurringConfig("tf-acc-test-product-monthly", "month", 499),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_product.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("tf-acc-test-product-monthly"),
					),
	statecheck.ExpectKnownValue(
						"polar_product.test",
						tfjsonpath.New("recurring_interval"),
						knownvalue.StringExact("month"),
					),
				},
			},
			// ImportState
			{
				ResourceName:      "polar_product.test",
				ImportState:        true,
				ImportStateVerify:  true,
			},
			// Update name
			{
				Config: testAccProductRecurringConfig("tf-acc-test-product-monthly-upd", "month", 999),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_product.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("tf-acc-test-product-monthly-upd"),
					),
				},
			},
		},
	})
}

func TestAccProductResource_withDescription(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProductWithDescriptionConfig("tf-acc-test-product-desc", "A test product", 500),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_product.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact("A test product"),
					),
				},
			},
		},
	})
}

func TestAccProductResource_metadata(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProductWithMetadataConfig("tf-acc-test-product-meta", `{ env = "test" }`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_product.test",
						tfjsonpath.New("metadata").AtMapKey("env"),
						knownvalue.StringExact("test"),
					),
				},
			},
			// Update metadata
			{
				Config: testAccProductWithMetadataConfig("tf-acc-test-product-meta", `{ env = "staging" }`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_product.test",
						tfjsonpath.New("metadata").AtMapKey("env"),
						knownvalue.StringExact("staging"),
					),
				},
			},
		},
	})
}

func TestAccProductResource_customPrice(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProductCustomPriceConfig("tf-acc-test-product-custom", 500, 5000, 1000),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_product.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("tf-acc-test-product-custom"),
					),
					statecheck.ExpectKnownValue(
						"polar_product.test",
						tfjsonpath.New("prices").AtSliceIndex(0).AtMapKey("amount_type"),
						knownvalue.StringExact("custom"),
					),
					statecheck.ExpectKnownValue(
						"polar_product.test",
						tfjsonpath.New("prices").AtSliceIndex(0).AtMapKey("minimum_amount"),
						knownvalue.Int64Exact(500),
					),
					statecheck.ExpectKnownValue(
						"polar_product.test",
						tfjsonpath.New("prices").AtSliceIndex(0).AtMapKey("maximum_amount"),
						knownvalue.Int64Exact(5000),
					),
					statecheck.ExpectKnownValue(
						"polar_product.test",
						tfjsonpath.New("prices").AtSliceIndex(0).AtMapKey("preset_amount"),
						knownvalue.Int64Exact(1000),
					),
				},
			},
			// ImportState
			{
				ResourceName:      "polar_product.test",
				ImportState:        true,
				ImportStateVerify:  true,
			},
			// Update custom price amounts
			{
				Config: testAccProductCustomPriceConfig("tf-acc-test-product-custom", 1000, 10000, 2500),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_product.test",
						tfjsonpath.New("prices").AtSliceIndex(0).AtMapKey("preset_amount"),
						knownvalue.Int64Exact(2500),
					),
				},
			},
		},
	})
}

func TestAccProductResource_meteredUnit(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProductMeteredUnitConfig("tf-acc-test-product-metered", "0.50", 10000),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_product.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("tf-acc-test-product-metered"),
					),
	statecheck.ExpectKnownValue(
						"polar_product.test",
						tfjsonpath.New("prices").AtSliceIndex(0).AtMapKey("amount_type"),
						knownvalue.StringExact("metered_unit"),
					),
					statecheck.ExpectKnownValue(
						"polar_product.test",
						tfjsonpath.New("prices").AtSliceIndex(0).AtMapKey("unit_amount"),
						knownvalue.StringExact("0.50"),
					),
					statecheck.ExpectKnownValue(
						"polar_product.test",
						tfjsonpath.New("prices").AtSliceIndex(0).AtMapKey("cap_amount"),
						knownvalue.Int64Exact(10000),
					),
				},
			},
			// ImportState
			{
				ResourceName:            "polar_product.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"prices"},
			},
		},
	})
}

func TestAccProductResource_seatBased(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProductSeatBasedConfig("tf-acc-test-product-seats"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_product.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("tf-acc-test-product-seats"),
					),
	statecheck.ExpectKnownValue(
						"polar_product.test",
						tfjsonpath.New("prices").AtSliceIndex(0).AtMapKey("amount_type"),
						knownvalue.StringExact("seat_based"),
					),
				},
			},
			// ImportState
			{
				ResourceName:      "polar_product.test",
				ImportState:        true,
				ImportStateVerify:  true,
			},
		},
	})
}

// --- Config helpers ---

func testAccProductOneTimeFixedConfig(name string, priceAmount int64) string {
	return fmt.Sprintf(`
resource "polar_product" "test" {
  name = %q

  prices = [{
    amount_type    = "fixed"
    price_amount   = %d
  }]
}
`, name, priceAmount)
}

func testAccProductOneTimeFreeConfig(name string) string {
	return fmt.Sprintf(`
resource "polar_product" "test" {
  name = %q

  prices = [{
    amount_type = "free"
  }]
}
`, name)
}

func testAccProductRecurringConfig(name, interval string, priceAmount int64) string {
	return fmt.Sprintf(`
resource "polar_product" "test" {
  name               = %q
  recurring_interval = %q

  prices = [{
    amount_type    = "fixed"
    price_amount   = %d
  }]
}
`, name, interval, priceAmount)
}

func testAccProductWithDescriptionConfig(name, description string, priceAmount int64) string {
	return fmt.Sprintf(`
resource "polar_product" "test" {
  name        = %q
  description = %q

  prices = [{
    amount_type    = "fixed"
    price_amount   = %d
  }]
}
`, name, description, priceAmount)
}

func testAccProductCustomPriceConfig(name string, minAmount, maxAmount, presetAmount int64) string {
	return fmt.Sprintf(`
resource "polar_product" "test" {
  name = %q

  prices = [{
    amount_type    = "custom"
    minimum_amount = %d
    maximum_amount = %d
    preset_amount  = %d
  }]
}
`, name, minAmount, maxAmount, presetAmount)
}

func testAccProductMeteredUnitConfig(name, unitAmount string, capAmount int64) string {
	return fmt.Sprintf(`
resource "polar_meter" "test" {
  name = "tf-acc-test-meter-for-product"

  filter = {
    conjunction = "and"
    clauses = [{
      property = "name"
      operator = "eq"
      value    = "test"
    }]
  }

  aggregation = {
    func = "count"
  }
}

resource "polar_product" "test" {
  name               = %q
  recurring_interval = "month"

  prices = [{
    amount_type = "metered_unit"
    meter_id    = polar_meter.test.id
    unit_amount = %q
    cap_amount  = %d
  }]
}
`, name, unitAmount, capAmount)
}

func testAccProductSeatBasedConfig(name string) string {
	return fmt.Sprintf(`
resource "polar_product" "test" {
  name               = %q
  recurring_interval = "month"

  prices = [{
    amount_type = "seat_based"
    seat_tiers = [
      {
        min_seats      = 1
        max_seats      = 10
        price_per_seat = 1000
      },
      {
        min_seats      = 11
        price_per_seat = 800
      }
    ]
  }]
}
`, name)
}

func testAccProductWithMetadataConfig(name, metadata string) string {
	return fmt.Sprintf(`
resource "polar_product" "test" {
  name = %q

  prices = [{
    amount_type    = "fixed"
    price_amount   = 500
  }]

  metadata = %s
}
`, name, metadata)
}
