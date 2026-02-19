// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func typesStringNull() types.String { return types.StringNull() }

func TestNumericStringsEqual(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"0.5", "0.50", true},
		{"0.5", "0.500000000000", true},
		{"1", "1.0", true},
		{"100", "100", true},
		{"0.50", "0.50", true},
		{"0.5", "0.6", false},
		{"1", "2", false},
		{"abc", "0.5", false},
		{"0.5", "abc", false},
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := numericStringsEqual(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("numericStringsEqual(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestNormalizeDecimalString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"0.500000000000", "0.5"},
		{"0.50", "0.5"},
		{"1.0", "1"},
		{"100", "100"},
		{"0.123456789", "0.123456789"},
		{"abc", "abc"}, // non-numeric passes through unchanged
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeDecimalString(tt.input)
			if got != tt.want {
				t.Errorf("normalizeDecimalString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// meteredPrice builds a metered_unit PriceModel for testing.
func meteredPrice(meterID, unitAmount string) PriceModel {
	return PriceModel{
		AmountType:    types.StringValue("metered_unit"),
		MeterID:       types.StringValue(meterID),
		UnitAmount:    types.StringValue(unitAmount),
		PriceCurrency: types.StringNull(),
		PriceAmount:   types.Int64Null(),
		MinimumAmount: types.Int64Null(),
		MaximumAmount: types.Int64Null(),
		PresetAmount:  types.Int64Null(),
		CapAmount:     types.Int64Null(),
	}
}

func TestPreserveUnitAmountFormatting(t *testing.T) {
	tests := []struct {
		name       string
		current    string
		prior      string
		wantResult string
	}{
		{
			name:       "preserves user formatting when numerically equal",
			current:    "0.5",
			prior:      "0.50",
			wantResult: "0.50",
		},
		{
			name:       "keeps current when numerically different",
			current:    "0.6",
			prior:      "0.50",
			wantResult: "0.6",
		},
		{
			name:       "identical values unchanged",
			current:    "0.5",
			prior:      "0.5",
			wantResult: "0.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prices := []PriceModel{meteredPrice("m1", tt.current)}
			priors := []PriceModel{meteredPrice("m1", tt.prior)}
			preserveUnitAmountFormatting(prices, priors)
			got := prices[0].UnitAmount.ValueString()
			if got != tt.wantResult {
				t.Errorf("got %q, want %q", got, tt.wantResult)
			}
		})
	}
}

func TestPreserveUnitAmountFormatting_nullHandling(t *testing.T) {
	// Non-metered prices have null UnitAmount — should not panic
	prices := []PriceModel{{
		AmountType: types.StringValue("free"),
		UnitAmount: typesStringNull(),
	}}
	priors := []PriceModel{{
		AmountType: types.StringValue("free"),
		UnitAmount: typesStringNull(),
	}}
	preserveUnitAmountFormatting(prices, priors)

	if !prices[0].UnitAmount.IsNull() {
		t.Error("expected null to remain null")
	}
}

func TestPreserveUnitAmountFormatting_lengthMismatch(t *testing.T) {
	// Fewer priors than current — extra price should be untouched
	prices := []PriceModel{
		meteredPrice("m1", "0.5"),
		meteredPrice("m2", "1.0"),
	}
	priors := []PriceModel{
		meteredPrice("m1", "0.50"),
	}
	preserveUnitAmountFormatting(prices, priors)

	if prices[0].UnitAmount.ValueString() != "0.50" {
		t.Errorf("first price: got %q, want %q", prices[0].UnitAmount.ValueString(), "0.50")
	}
	if prices[1].UnitAmount.ValueString() != "1.0" {
		t.Errorf("second price: got %q, want %q (should be untouched)", prices[1].UnitAmount.ValueString(), "1.0")
	}
}

func TestPreserveUnitAmountFormatting_reorder(t *testing.T) {
	// Prices reordered vs prior state — should match by content, not index
	prices := []PriceModel{
		meteredPrice("m2", "1"),
		meteredPrice("m1", "0.5"),
	}
	priors := []PriceModel{
		meteredPrice("m1", "0.50"),
		meteredPrice("m2", "1.00"),
	}
	preserveUnitAmountFormatting(prices, priors)

	if got := prices[0].UnitAmount.ValueString(); got != "1.00" {
		t.Errorf("first price (m2): got %q, want %q", got, "1.00")
	}
	if got := prices[1].UnitAmount.ValueString(); got != "0.50" {
		t.Errorf("second price (m1): got %q, want %q", got, "0.50")
	}
}
