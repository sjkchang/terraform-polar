// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func typesStringValue(s string) types.String { return types.StringValue(s) }
func typesStringNull() types.String          { return types.StringNull() }

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
			prices := []PriceModel{{UnitAmount: typesStringValue(tt.current)}}
			priors := []PriceModel{{UnitAmount: typesStringValue(tt.prior)}}
			preserveUnitAmountFormatting(prices, priors)
			got := prices[0].UnitAmount.ValueString()
			if got != tt.wantResult {
				t.Errorf("got %q, want %q", got, tt.wantResult)
			}
		})
	}
}

func TestPreserveUnitAmountFormatting_nullHandling(t *testing.T) {
	// Should not panic when UnitAmount is null
	prices := []PriceModel{{UnitAmount: typesStringNull()}}
	priors := []PriceModel{{UnitAmount: typesStringNull()}}
	preserveUnitAmountFormatting(prices, priors)

	if !prices[0].UnitAmount.IsNull() {
		t.Error("expected null to remain null")
	}
}

func TestPreserveUnitAmountFormatting_lengthMismatch(t *testing.T) {
	// Fewer priors than current — should not panic
	prices := []PriceModel{
		{UnitAmount: typesStringValue("0.5")},
		{UnitAmount: typesStringValue("1.0")},
	}
	priors := []PriceModel{
		{UnitAmount: typesStringValue("0.50")},
	}
	preserveUnitAmountFormatting(prices, priors)

	if prices[0].UnitAmount.ValueString() != "0.50" {
		t.Errorf("first price: got %q, want %q", prices[0].UnitAmount.ValueString(), "0.50")
	}
	if prices[1].UnitAmount.ValueString() != "1.0" {
		t.Errorf("second price: got %q, want %q (should be untouched)", prices[1].UnitAmount.ValueString(), "1.0")
	}
}
