// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/polarsource/polar-go/models/components"
)

func TestAccMeterResource_count(t *testing.T) {
	rName := fmt.Sprintf("tf-acc-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with count aggregation
			{
				Config: testAccMeterConfig(rName, "and", "name", "eq", "api_call", "count", ""),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_meter.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(rName),
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
				ResourceName:      "polar_meter.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update name
			{
				Config: testAccMeterConfig(rName+"-updated", "and", "name", "eq", "api_call", "count", ""),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_meter.test",
						tfjsonpath.New("name"),
						knownvalue.StringExact(rName+"-updated"),
					),
				},
			},
			// Delete (archive) testing automatically occurs in TestCase
		},
	})
}

func TestAccMeterResource_sum(t *testing.T) {
	rName := fmt.Sprintf("tf-acc-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with sum aggregation
			{
				Config: testAccMeterConfig(rName, "and", "type", "eq", "usage", "sum", "amount"),
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
	rName := fmt.Sprintf("tf-acc-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with metadata
			{
				Config: testAccMeterConfigWithMetadata(rName, `{ env = "test" }`),
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
				Config: testAccMeterConfigWithMetadata(rName, `{ env = "staging" }`),
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

func TestSdkFilterToModel_valueTypeCoercion(t *testing.T) {
	// Filter values are always sent as strings (CreateValueStr). If the API
	// returns them as a different union variant, verify the string conversion
	// roundtrips correctly.
	tests := []struct {
		name  string
		value components.Value
		want  string
	}{
		{"string passthrough", components.CreateValueStr("hello"), "hello"},
		{"string numeric", components.CreateValueStr("123"), "123"},
		{"string with leading zero", components.CreateValueStr("00123"), "00123"},
		{"integer", components.CreateValueInteger(123), "123"},
		{"negative integer", components.CreateValueInteger(-42), "-42"},
		{"boolean true", components.CreateValueBoolean(true), "true"},
		{"boolean false", components.CreateValueBoolean(false), "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			filter := components.Filter{
				Conjunction: "and",
				Clauses: []components.Clauses{
					components.CreateClausesFilterClause(components.FilterClause{
						Property: "test_prop",
						Operator: "eq",
						Value:    tt.value,
					}),
				},
			}
			model := sdkFilterToModel(filter, &diags)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if len(model.Clauses) != 1 {
				t.Fatalf("expected 1 clause, got %d", len(model.Clauses))
			}
			got := model.Clauses[0].Value.ValueString()
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSdkFilterToModel_valueTypeDrift(t *testing.T) {
	// Documents cases where API type coercion would cause drift.
	// If user writes "00123" and API returns Integer(123), the roundtrip
	// produces "123" — a different string.
	tests := []struct {
		name      string
		userWrote string
		apiValue  components.Value
		drifts    bool
	}{
		{"string preserved exactly", "hello", components.CreateValueStr("hello"), false},
		{"integer matches simple number", "123", components.CreateValueInteger(123), false},
		{"integer loses leading zeros", "00123", components.CreateValueInteger(123), true},
		{"integer loses plus sign", "+123", components.CreateValueInteger(123), true},
		{"boolean matches lowercase", "true", components.CreateValueBoolean(true), false},
		{"boolean loses capitalization", "True", components.CreateValueBoolean(true), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			filter := components.Filter{
				Conjunction: "and",
				Clauses: []components.Clauses{
					components.CreateClausesFilterClause(components.FilterClause{
						Property: "p",
						Operator: "eq",
						Value:    tt.apiValue,
					}),
				},
			}
			model := sdkFilterToModel(filter, &diags)
			got := model.Clauses[0].Value.ValueString()
			hasDrift := got != tt.userWrote
			if hasDrift != tt.drifts {
				if tt.drifts {
					t.Errorf("expected drift from %q to %q but got no drift", tt.userWrote, got)
				} else {
					t.Errorf("unexpected drift: user wrote %q, read back %q", tt.userWrote, got)
				}
			}
		})
	}
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
