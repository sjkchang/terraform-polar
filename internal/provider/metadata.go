// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// --- Metadata conversion ---
// Terraform stores metadata as map[string]string (all values are strings).
// The Polar SDK uses typed union values (string | int | float | bool) per key.
// These helpers bridge the two representations.

// metadataFields is a common shape to extract union values from any SDK metadata type.
type metadataFields struct {
	Str     *string
	Integer *int64
	Number  *float64
	Boolean *bool
}

// metadataToCreateSDK converts TF map[string]string → SDK metadata map.
// The factory creates the type-specific SDK union variant from a string value.
func metadataToCreateSDK[T any](ctx context.Context, metadata types.Map, factory func(string) T) (map[string]T, diag.Diagnostics) {
	var stringMap map[string]string
	diags := metadata.ElementsAs(ctx, &stringMap, false)
	if diags.HasError() {
		return nil, diags
	}
	result := make(map[string]T, len(stringMap))
	for k, v := range stringMap {
		result[k] = factory(v)
	}
	return result, diags
}

// sdkMetadataToMap converts SDK metadata → TF map[string]string.
// The extract function pulls the union value pointers from the SDK type.
// Always returns a non-null map (empty {} for nil input) so Optional+Computed
// metadata attributes don't oscillate between null and {} across plan/apply.
func sdkMetadataToMap[T any](ctx context.Context, metadata map[string]T, extract func(T) metadataFields, diags *diag.Diagnostics) types.Map {
	if len(metadata) == 0 {
		result, d := types.MapValueFrom(ctx, types.StringType, map[string]string{})
		diags.Append(d...)
		return result
	}
	stringMap := make(map[string]string, len(metadata))
	for k, v := range metadata {
		f := extract(v)
		switch {
		case f.Str != nil:
			stringMap[k] = *f.Str
		case f.Integer != nil:
			stringMap[k] = strconv.FormatInt(*f.Integer, 10)
		case f.Number != nil:
			stringMap[k] = strconv.FormatFloat(*f.Number, 'f', -1, 64)
		case f.Boolean != nil:
			stringMap[k] = strconv.FormatBool(*f.Boolean)
		}
	}
	result, d := types.MapValueFrom(ctx, types.StringType, stringMap)
	diags.Append(d...)
	return result
}
