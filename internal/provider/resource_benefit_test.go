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

func TestAccBenefitResource_custom(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create
			{
				Config: testAccBenefitCustomConfig("tf-acc-test-custom-benefit", "Check your email for details"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_benefit.test",
						tfjsonpath.New("type"),
						knownvalue.StringExact("custom"),
					),
					statecheck.ExpectKnownValue(
						"polar_benefit.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact("tf-acc-test-custom-benefit"),
					),
					statecheck.ExpectKnownValue(
						"polar_benefit.test",
						tfjsonpath.New("custom_properties").AtMapKey("note"),
						knownvalue.StringExact("Check your email for details"),
					),
				},
			},
			// ImportState
			{
				ResourceName:      "polar_benefit.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update description and note
			{
				Config: testAccBenefitCustomConfig("tf-acc-test-custom-updated", "Updated instructions"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_benefit.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact("tf-acc-test-custom-updated"),
					),
					statecheck.ExpectKnownValue(
						"polar_benefit.test",
						tfjsonpath.New("custom_properties").AtMapKey("note"),
						knownvalue.StringExact("Updated instructions"),
					),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccBenefitResource_customNoNote(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBenefitCustomConfigNoNote("tf-acc-test-custom-no-note"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_benefit.test",
						tfjsonpath.New("type"),
						knownvalue.StringExact("custom"),
					),
					statecheck.ExpectKnownValue(
						"polar_benefit.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact("tf-acc-test-custom-no-note"),
					),
				},
			},
		},
	})
}

func TestAccBenefitResource_meterCredit(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create meter + meter credit benefit
			{
				Config: testAccBenefitMeterCreditConfig("tf-acc-test-mc-benefit", 100, true),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_benefit.test",
						tfjsonpath.New("type"),
						knownvalue.StringExact("meter_credit"),
					),
					statecheck.ExpectKnownValue(
						"polar_benefit.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact("tf-acc-test-mc-benefit"),
					),
					statecheck.ExpectKnownValue(
						"polar_benefit.test",
						tfjsonpath.New("meter_credit_properties").AtMapKey("units"),
						knownvalue.Int64Exact(100),
					),
					statecheck.ExpectKnownValue(
						"polar_benefit.test",
						tfjsonpath.New("meter_credit_properties").AtMapKey("rollover"),
						knownvalue.Bool(true),
					),
				},
			},
			// ImportState
			{
				ResourceName:      "polar_benefit.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update units and rollover
			{
				Config: testAccBenefitMeterCreditConfig("tf-acc-test-mc-benefit", 200, false),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_benefit.test",
						tfjsonpath.New("meter_credit_properties").AtMapKey("units"),
						knownvalue.Int64Exact(200),
					),
					statecheck.ExpectKnownValue(
						"polar_benefit.test",
						tfjsonpath.New("meter_credit_properties").AtMapKey("rollover"),
						knownvalue.Bool(false),
					),
				},
			},
		},
	})
}

func TestAccBenefitResource_licenseKeys(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create
			{
				Config: testAccBenefitLicenseKeysConfig("tf-acc-test-lk-benefit", "TFTEST", 5),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_benefit.test",
						tfjsonpath.New("type"),
						knownvalue.StringExact("license_keys"),
					),
					statecheck.ExpectKnownValue(
						"polar_benefit.test",
						tfjsonpath.New("license_keys_properties").AtMapKey("prefix"),
						knownvalue.StringExact("TFTEST"),
					),
					statecheck.ExpectKnownValue(
						"polar_benefit.test",
						tfjsonpath.New("license_keys_properties").AtMapKey("limit_usage"),
						knownvalue.Int64Exact(5),
					),
				},
			},
			// ImportState
			{
				ResourceName:      "polar_benefit.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update prefix
			{
				Config: testAccBenefitLicenseKeysConfig("tf-acc-test-lk-updated", "TFUPD", 10),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_benefit.test",
						tfjsonpath.New("license_keys_properties").AtMapKey("prefix"),
						knownvalue.StringExact("TFUPD"),
					),
				},
			},
		},
	})
}

// --- Config helpers ---

func testAccBenefitCustomConfig(description, note string) string {
	return fmt.Sprintf(`
resource "polar_benefit" "test" {
  type        = "custom"
  description = %q

  custom_properties = {
    note = %q
  }
}
`, description, note)
}

func testAccBenefitCustomConfigNoNote(description string) string {
	return fmt.Sprintf(`
resource "polar_benefit" "test" {
  type        = "custom"
  description = %q

  custom_properties = {}
}
`, description)
}

func testAccBenefitMeterCreditConfig(description string, units int64, rollover bool) string {
	return fmt.Sprintf(`
resource "polar_meter" "test" {
  name = "tf-acc-test-mc-meter"

  filter = {
    conjunction = "and"
    clauses = [{
      property = "name"
      operator = "eq"
      value    = "api_call"
    }]
  }

  aggregation = {
    func = "count"
  }
}

resource "polar_benefit" "test" {
  type        = "meter_credit"
  description = %q

  meter_credit_properties = {
    meter_id = polar_meter.test.id
    units    = %d
    rollover = %t
  }
}
`, description, units, rollover)
}

func testAccBenefitLicenseKeysConfig(description, prefix string, limitUsage int64) string {
	return fmt.Sprintf(`
resource "polar_benefit" "test" {
  type        = "license_keys"
  description = %q

  license_keys_properties = {
    prefix      = %q
    limit_usage = %d
  }
}
`, description, prefix, limitUsage)
}
