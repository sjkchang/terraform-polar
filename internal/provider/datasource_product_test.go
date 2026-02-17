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

func TestAccProductDataSource_basic(t *testing.T) {
	rName := fmt.Sprintf("tf-acc-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProductDataSourceConfig(rName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.polar_product.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(rName),
					),
					statecheck.ExpectKnownValue(
						"data.polar_product.test",
						tfjsonpath.New("prices"),
						knownvalue.ListSizeExact(1),
					),
				},
			},
		},
	})
}

func testAccProductDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "polar_product" "test" {
  name = %q

  prices = [{
    amount_type  = "fixed"
    price_amount = 500
  }]
}

data "polar_product" "test" {
  id = polar_product.test.id
}
`, name)
}
