// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccProductDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProductDataSourceConfig(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.polar_product.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("tf-acc-test-product-ds"),
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

func testAccProductDataSourceConfig() string {
	return `
resource "polar_product" "test" {
  name = "tf-acc-test-product-ds"

  prices = [{
    amount_type  = "fixed"
    price_amount = 500
  }]
}

data "polar_product" "test" {
  id = polar_product.test.id
}
`
}
