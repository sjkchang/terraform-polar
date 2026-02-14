// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// nonNullRaw returns a tftypes.Value that is non-null, indicating the resource exists.
func nonNullRaw() tftypes.Value {
	return tftypes.NewValue(tftypes.Object{}, map[string]tftypes.Value{})
}

// nullRaw returns a null tftypes.Value, indicating the resource does not exist.
func nullRaw() tftypes.Value {
	return tftypes.NewValue(tftypes.Object{}, nil)
}

func TestArchiveReplaceModifier_valueChanged(t *testing.T) {
	modifier := requiresReplaceWithArchiveWarning("product", "Subscribers will remain on the archived product.")

	resp := &planmodifier.StringResponse{}
	modifier.PlanModifyString(context.Background(), planmodifier.StringRequest{
		StateValue: types.StringValue("month"),
		PlanValue:  types.StringValue("year"),
		State:      tfsdk.State{Raw: nonNullRaw()},
		Plan:       tfsdk.Plan{Raw: nonNullRaw()},
	}, resp)

	if !resp.RequiresReplace {
		t.Error("expected RequiresReplace to be true when value changes")
	}
	if resp.Diagnostics.WarningsCount() != 1 {
		t.Fatalf("expected 1 warning, got %d", resp.Diagnostics.WarningsCount())
	}
	summary := resp.Diagnostics.Warnings()[0].Summary()
	if summary != "Changing this field will archive the existing product" {
		t.Errorf("unexpected warning summary: %s", summary)
	}
}

func TestArchiveReplaceModifier_valueUnchanged(t *testing.T) {
	modifier := requiresReplaceWithArchiveWarning("product", "Subscribers will remain on the archived product.")

	resp := &planmodifier.StringResponse{}
	modifier.PlanModifyString(context.Background(), planmodifier.StringRequest{
		StateValue: types.StringValue("month"),
		PlanValue:  types.StringValue("month"),
		State:      tfsdk.State{Raw: nonNullRaw()},
		Plan:       tfsdk.Plan{Raw: nonNullRaw()},
	}, resp)

	if resp.RequiresReplace {
		t.Error("expected RequiresReplace to be false when value is unchanged")
	}
	if resp.Diagnostics.WarningsCount() != 0 {
		t.Errorf("expected no warnings, got %d", resp.Diagnostics.WarningsCount())
	}
}

func TestArchiveReplaceModifier_resourceCreation(t *testing.T) {
	modifier := requiresReplaceWithArchiveWarning("product", "Subscribers will remain on the archived product.")

	resp := &planmodifier.StringResponse{}
	modifier.PlanModifyString(context.Background(), planmodifier.StringRequest{
		StateValue: types.StringValue("month"),
		PlanValue:  types.StringValue("year"),
		State:      tfsdk.State{Raw: nullRaw()},
		Plan:       tfsdk.Plan{Raw: nonNullRaw()},
	}, resp)

	if resp.RequiresReplace {
		t.Error("expected RequiresReplace to be false on resource creation")
	}
	if resp.Diagnostics.WarningsCount() != 0 {
		t.Errorf("expected no warnings on resource creation, got %d", resp.Diagnostics.WarningsCount())
	}
}

func TestArchiveReplaceModifier_resourceDestruction(t *testing.T) {
	modifier := requiresReplaceWithArchiveWarning("product", "Subscribers will remain on the archived product.")

	resp := &planmodifier.StringResponse{}
	modifier.PlanModifyString(context.Background(), planmodifier.StringRequest{
		StateValue: types.StringValue("month"),
		PlanValue:  types.StringValue("year"),
		State:      tfsdk.State{Raw: nonNullRaw()},
		Plan:       tfsdk.Plan{Raw: nullRaw()},
	}, resp)

	if resp.RequiresReplace {
		t.Error("expected RequiresReplace to be false on resource destruction")
	}
	if resp.Diagnostics.WarningsCount() != 0 {
		t.Errorf("expected no warnings on resource destruction, got %d", resp.Diagnostics.WarningsCount())
	}
}
