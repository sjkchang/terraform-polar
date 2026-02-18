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

func TestAccMeterDataSource_basic(t *testing.T) {
	rName := fmt.Sprintf("tf-acc-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccMeterDataSourceConfig(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.polar_meter.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(rName),
					),
				},
			},
		},
	})
}

func testAccMeterDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "polar_meter" "test" {
  name = %q

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
`, name)
}
