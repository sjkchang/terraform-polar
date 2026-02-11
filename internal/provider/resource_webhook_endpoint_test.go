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

func TestAccWebhookEndpointResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: testAccWebhookEndpointConfig("https://example.com/webhook/test-create", "raw", `["order.created"]`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_webhook_endpoint.test",
						tfjsonpath.New("url"),
						knownvalue.StringExact("https://example.com/webhook/test-create"),
					),
					statecheck.ExpectKnownValue(
						"polar_webhook_endpoint.test",
						tfjsonpath.New("format"),
						knownvalue.StringExact("raw"),
					),
					statecheck.ExpectKnownValue(
						"polar_webhook_endpoint.test",
						tfjsonpath.New("enabled"),
						knownvalue.Bool(true),
					),
				},
			},
			// ImportState
			{
				ResourceName:            "polar_webhook_endpoint.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"secret"},
			},
			// Update URL and events
			{
				Config: testAccWebhookEndpointConfig("https://example.com/webhook/test-update", "raw", `["order.created", "subscription.created"]`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_webhook_endpoint.test",
						tfjsonpath.New("url"),
						knownvalue.StringExact("https://example.com/webhook/test-update"),
					),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccWebhookEndpointResource_disabled(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create disabled endpoint
			{
				Config: testAccWebhookEndpointConfigWithEnabled("https://example.com/webhook/test-disabled", "raw", `["order.created"]`, false),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_webhook_endpoint.test",
						tfjsonpath.New("enabled"),
						knownvalue.Bool(false),
					),
				},
			},
			// Enable it
			{
				Config: testAccWebhookEndpointConfigWithEnabled("https://example.com/webhook/test-disabled", "raw", `["order.created"]`, true),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_webhook_endpoint.test",
						tfjsonpath.New("enabled"),
						knownvalue.Bool(true),
					),
				},
			},
		},
	})
}

func testAccWebhookEndpointConfig(url, format, events string) string {
	return fmt.Sprintf(`
resource "polar_webhook_endpoint" "test" {
  url    = %[1]q
  format = %[2]q
  events = %[3]s
}
`, url, format, events)
}

func testAccWebhookEndpointConfigWithEnabled(url, format, events string, enabled bool) string {
	return fmt.Sprintf(`
resource "polar_webhook_endpoint" "test" {
  url     = %[1]q
  format  = %[2]q
  events  = %[3]s
  enabled = %[4]t
}
`, url, format, events, enabled)
}
