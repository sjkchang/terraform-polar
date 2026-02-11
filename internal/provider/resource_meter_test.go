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

func TestAccMeterResource_count(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with count aggregation
			{
				Config: testAccMeterConfig("tf-acc-test-meter-count", "and", "name", "eq", "api_call", "count", ""),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_meter.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("tf-acc-test-meter-count"),
					),
					statecheck.ExpectKnownValue(
						"polar_meter.test",
						tfjsonpath.New("filter").AtMapKey("conjunction"),
						knownvalue.StringExact("and"),
					),
					statecheck.ExpectKnownValue(
						"polar_meter.test",
						tfjsonpath.New("aggregation").AtMapKey("func"),
						knownvalue.StringExact("count"),
					),
				},
			},
			// ImportState
			{
				ResourceName:     "polar_meter.test",
				ImportState:      true,
				ImportStateVerify: true,
			},
			// Update name
			{
				Config: testAccMeterConfig("tf-acc-test-meter-count-updated", "and", "name", "eq", "api_call", "count", ""),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_meter.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact("tf-acc-test-meter-count-updated"),
					),
				},
			},
			// Delete (archive) testing automatically occurs in TestCase
		},
	})
}

func TestAccMeterResource_sum(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with sum aggregation
			{
				Config: testAccMeterConfig("tf-acc-test-meter-sum", "and", "type", "eq", "usage", "sum", "amount"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_meter.test",
						tfjsonpath.New("aggregation").AtMapKey("func"),
						knownvalue.StringExact("sum"),
					),
					statecheck.ExpectKnownValue(
						"polar_meter.test",
						tfjsonpath.New("aggregation").AtMapKey("property"),
						knownvalue.StringExact("amount"),
					),
				},
			},
		},
	})
}

func TestAccMeterResource_metadata(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with metadata
			{
				Config: testAccMeterConfigWithMetadata("tf-acc-test-meter-meta", `{ env = "test" }`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_meter.test",
						tfjsonpath.New("metadata").AtMapKey("env"),
						knownvalue.StringExact("test"),
					),
				},
			},
			// Update metadata
			{
				Config: testAccMeterConfigWithMetadata("tf-acc-test-meter-meta", `{ env = "staging" }`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_meter.test",
						tfjsonpath.New("metadata").AtMapKey("env"),
						knownvalue.StringExact("staging"),
					),
				},
			},
		},
	})
}

func testAccMeterConfig(name, conjunction, property, operator, value, aggFunc, aggProperty string) string {
	aggAttr := fmt.Sprintf(`
  aggregation = {
    func = %q
  }`, aggFunc)

	if aggProperty != "" {
		aggAttr = fmt.Sprintf(`
  aggregation = {
    func     = %q
    property = %q
  }`, aggFunc, aggProperty)
	}

	return fmt.Sprintf(`
resource "polar_meter" "test" {
  name = %q

  filter = {
    conjunction = %q
    clauses = [{
      property = %q
      operator = %q
      value    = %q
    }]
  }
%s
}
`, name, conjunction, property, operator, value, aggAttr)
}

func testAccMeterConfigWithMetadata(name, metadata string) string {
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

  metadata = %s
}
`, name, metadata)
}
