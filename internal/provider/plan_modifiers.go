// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// requiresReplaceWithArchiveWarning returns a plan modifier that behaves like
// RequiresReplace but also emits a warning explaining that the old resource
// will be archived (not deleted) and will no longer be tracked by Terraform.
//
// The warning parameter should describe resource-specific implications of
// the archive, for example what happens to existing subscribers.
func requiresReplaceWithArchiveWarning(warning string) planmodifier.String {
	return &archiveReplaceModifier{
		warning: warning,
	}
}

type archiveReplaceModifier struct {
	warning string
}

func (m *archiveReplaceModifier) Description(_ context.Context) string {
	return fmt.Sprintf("If the value of this attribute changes, Terraform will archive the existing %s and create a new one.", "product")
}

func (m *archiveReplaceModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m *archiveReplaceModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// Do nothing on resource creation or destruction.
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	// Do nothing if the value hasn't changed.
	if req.PlanValue.Equal(req.StateValue) {
		return
	}

	// Trigger replacement.
	resp.RequiresReplace = true

	// Emit a warning about archive behavior.
	resp.Diagnostics.AddWarning(
		fmt.Sprintf("Changing this field will archive the existing %s", "product"),
		fmt.Sprintf(
			"Polar does not support changing this field on an existing %s. "+
				"Terraform will archive the current %s and create a new one.\n\n"+
				"The archived %s will remain in your Polar account but will no longer be tracked by Terraform. "+
				"To keep it tracked, consider creating a new resource and setting is_archived = true on the "+
				"old one instead.\n\n%s\n\n"+
				"You can add lifecycle { prevent_destroy = true } to this resource to prevent accidental archival.",
			"product", "product", "product", m.warning,
		),
	)
}
