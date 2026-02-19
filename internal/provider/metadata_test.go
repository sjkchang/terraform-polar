// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func strPtr(s string) *string       { return &s }
func int64Ptr(i int64) *int64       { return &i }
func float64Ptr(f float64) *float64 { return &f }
func boolPtr(b bool) *bool          { return &b }

func identityExtract(f metadataFields) metadataFields { return f }

// extractResult is a helper that converts sdkMetadataToMap output to map[string]string.
func extractResult(t *testing.T, ctx context.Context, input map[string]metadataFields) map[string]string {
	t.Helper()
	var diags diag.Diagnostics
	result := sdkMetadataToMap(ctx, input, identityExtract, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if result.IsNull() {
		t.Fatal("expected non-null map, got null")
	}
	var m map[string]string
	d := result.ElementsAs(ctx, &m, false)
	if d.HasError() {
		t.Fatalf("unexpected diagnostics extracting map: %v", d)
	}
	return m
}

func TestSdkMetadataToMap_nilReturnsEmptyMap(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics

	result := sdkMetadataToMap[metadataFields](ctx, nil, identityExtract, &diags)

	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if result.IsNull() {
		t.Error("expected non-null empty map for nil metadata, got null")
	}
	if len(result.Elements()) != 0 {
		t.Errorf("expected 0 elements, got %d", len(result.Elements()))
	}
}

func TestSdkMetadataToMap_emptyMapReturnsEmptyMap(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics

	empty := map[string]metadataFields{}
	result := sdkMetadataToMap(ctx, empty, identityExtract, &diags)

	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if result.IsNull() {
		t.Error("expected non-null empty map for empty metadata, got null")
	}
	if len(result.Elements()) != 0 {
		t.Errorf("expected 0 elements, got %d", len(result.Elements()))
	}
}

func TestSdkMetadataToMap_strPassthrough(t *testing.T) {
	ctx := context.Background()
	m := extractResult(t, ctx, map[string]metadataFields{
		"key": {Str: strPtr("hello")},
	})
	if m["key"] != "hello" {
		t.Errorf("got %q, want %q", m["key"], "hello")
	}
}

func TestSdkMetadataToMap_typeCoercion(t *testing.T) {
	// These test what the provider produces when the API returns metadata
	// values as non-string types. We always send metadata as strings, so
	// this only matters if the API internally coerces types.
	tests := []struct {
		name  string
		field metadataFields
		want  string
	}{
		{"integer", metadataFields{Integer: int64Ptr(123)}, "123"},
		{"negative integer", metadataFields{Integer: int64Ptr(-42)}, "-42"},
		{"zero integer", metadataFields{Integer: int64Ptr(0)}, "0"},
		{"float", metadataFields{Number: float64Ptr(1.5)}, "1.5"},
		{"float whole number", metadataFields{Number: float64Ptr(100.0)}, "100"},
		{"float zero", metadataFields{Number: float64Ptr(0.0)}, "0"},
		{"boolean true", metadataFields{Boolean: boolPtr(true)}, "true"},
		{"boolean false", metadataFields{Boolean: boolPtr(false)}, "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			m := extractResult(t, ctx, map[string]metadataFields{"key": tt.field})
			if m["key"] != tt.want {
				t.Errorf("got %q, want %q", m["key"], tt.want)
			}
		})
	}
}

func TestSdkMetadataToMap_numberPrecision(t *testing.T) {
	// If the API returns a float for what the user wrote as "1.50",
	// FormatFloat(1.5, 'f', -1, 64) produces "1.5" (not "1.50").
	// This documents the known formatting normalization.
	tests := []struct {
		name      string
		userWrote string
		apiFloat  float64
		readBack  string
		drifts    bool
	}{
		{"1.5 roundtrips cleanly", "1.5", 1.5, "1.5", false},
		{"trailing zero is lost", "1.50", 1.5, "1.5", true},
		{"trailing zeros are lost", "1.500", 1.5, "1.5", true},
		{"100 roundtrips cleanly", "100", 100.0, "100", false},
		{"0.5 roundtrips cleanly", "0.5", 0.5, "0.5", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			m := extractResult(t, ctx, map[string]metadataFields{
				"key": {Number: float64Ptr(tt.apiFloat)},
			})
			got := m["key"]
			if got != tt.readBack {
				t.Errorf("FormatFloat produced %q, expected %q", got, tt.readBack)
			}
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

// --- Plan-phase validation unit tests (no API calls) ---

func TestMetadataValidation_keyTooLong(t *testing.T) {
	longKey := strings.Repeat("k", 41)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "polar_product" "test" {
  name = "test"
  prices = [{ amount_type = "free" }]
  metadata = {
    %q = "value"
  }
}
`, longKey),
				ExpectError: regexp.MustCompile(`Metadata key too long`),
			},
		},
	})
}

func TestMetadataValidation_valueTooLong(t *testing.T) {
	longValue := strings.Repeat("v", 501)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "polar_product" "test" {
  name = "test"
  prices = [{ amount_type = "free" }]
  metadata = {
    key = %q
  }
}
`, longValue),
				ExpectError: regexp.MustCompile(`Metadata value too long`),
			},
		},
	})
}

func TestMetadataValidation_tooManyEntries(t *testing.T) {
	var entries strings.Builder
	for i := 0; i < 51; i++ {
		fmt.Fprintf(&entries, "    key%d = \"val\"\n", i)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "polar_product" "test" {
  name = "test"
  prices = [{ amount_type = "free" }]
  metadata = {
%s  }
}
`, entries.String()),
				ExpectError: regexp.MustCompile(`Too many metadata entries`),
			},
		},
	})
}
