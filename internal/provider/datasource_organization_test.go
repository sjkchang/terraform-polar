// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccOrganizationDataSource_autoDiscover(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "polar_organization" "test" {}`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.polar_organization.test",
						tfjsonpath.New("id"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"data.polar_organization.test",
						tfjsonpath.New("name"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"data.polar_organization.test",
						tfjsonpath.New("slug"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"data.polar_organization.test",
						tfjsonpath.New("notification_settings"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"data.polar_organization.test",
						tfjsonpath.New("subscription_settings"),
						knownvalue.NotNull(),
					),
					statecheck.ExpectKnownValue(
						"data.polar_organization.test",
						tfjsonpath.New("customer_email_settings"),
						knownvalue.NotNull(),
					),
				},
			},
		},
	})
}
