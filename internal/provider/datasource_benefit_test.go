// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccBenefitDataSource_custom(t *testing.T) {
	rName := fmt.Sprintf("tf-acc-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBenefitDataSourceCustomConfig(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.polar_benefit.test",
						tfjsonpath.New("type"),
						knownvalue.StringExact("custom"),
					),
					statecheck.ExpectKnownValue(
						"data.polar_benefit.test",
						tfjsonpath.New("description"),
						knownvalue.StringExact(rName),
					),
					statecheck.ExpectKnownValue(
						"data.polar_benefit.test",
						tfjsonpath.New("custom_properties").AtMapKey("note"),
						knownvalue.StringExact("Data source test note"),
					),
				},
			},
		},
	})
}

func TestAccBenefitDataSource_licenseKeys(t *testing.T) {
	rName := fmt.Sprintf("tf-acc-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccBenefitDataSourceLicenseKeysConfig(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.polar_benefit.test",
						tfjsonpath.New("type"),
						knownvalue.StringExact("license_keys"),
					),
					statecheck.ExpectKnownValue(
						"data.polar_benefit.test",
						tfjsonpath.New("license_keys_properties").AtMapKey("prefix"),
						knownvalue.StringExact("DSTEST"),
					),
				},
			},
		},
	})
}

// --- Config helpers ---

func testAccBenefitDataSourceCustomConfig(description string) string {
	return fmt.Sprintf(`
resource "polar_benefit" "test" {
  type        = "custom"
  description = %q

  custom_properties = {
    note = "Data source test note"
  }
}

data "polar_benefit" "test" {
  id = polar_benefit.test.id
}
`, description)
}

func testAccBenefitDataSourceLicenseKeysConfig(description string) string {
	return fmt.Sprintf(`
resource "polar_benefit" "test" {
  type        = "license_keys"
  description = %q

  license_keys_properties = {
    prefix      = "DSTEST"
    limit_usage = 3
  }
}

data "polar_benefit" "test" {
  id = polar_benefit.test.id
}
`, description)
}
