// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccMeterDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccMeterDataSourceConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.polar_meter.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("tf-acc-test-meter-ds"),
					),
					statecheck.ExpectKnownValue(
						"data.polar_meter.test",
						tfjsonpath.New("filter").AtMapKey("conjunction"),
						knownvalue.StringExact("and"),
					),
					statecheck.ExpectKnownValue(
						"data.polar_meter.test",
						tfjsonpath.New("aggregation").AtMapKey("func"),
						knownvalue.StringExact("count"),
					),
				},
			},
		},
	})
}

func testAccMeterDataSourceConfig() string {
	return `
resource "polar_meter" "test" {
  name = "tf-acc-test-meter-ds"

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

data "polar_meter" "test" {
  id = polar_meter.test.id
}
`
}
