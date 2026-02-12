// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// metadataFields holds the union value pointers common to all SDK metadata types.
type metadataFields struct {
	Str     *string
	Integer *int64
	Number  *float64
	Boolean *bool
}

// metadataToCreateSDK converts a Terraform types.Map (string values) to an SDK metadata map.
// The factory function creates the type-specific SDK metadata value from a string.
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

// sdkMetadataToMap converts an SDK metadata map back to a Terraform types.Map.
// The extract function pulls the union value pointers from the type-specific SDK metadata.
func sdkMetadataToMap[T any](ctx context.Context, metadata map[string]T, extract func(T) metadataFields, diags *diag.Diagnostics) types.Map {
	if len(metadata) == 0 {
		return types.MapNull(types.StringType)
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
