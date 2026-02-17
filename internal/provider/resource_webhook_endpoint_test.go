// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccWebhookEndpointResource_basic(t *testing.T) {
	rSuffix := acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)
	createURL := fmt.Sprintf("https://example.com/webhook/tf-acc-%s", rSuffix)
	updateURL := fmt.Sprintf("https://example.com/webhook/tf-acc-%s-updated", rSuffix)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: testAccWebhookEndpointConfig(createURL, "raw", `["order.created"]`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_webhook_endpoint.test",
						tfjsonpath.New("url"),
						knownvalue.StringExact(createURL),
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
				Config: testAccWebhookEndpointConfig(updateURL, "raw", `["order.created", "subscription.created"]`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"polar_webhook_endpoint.test",
						tfjsonpath.New("url"),
						knownvalue.StringExact(updateURL),
					),
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccWebhookEndpointResource_disabled(t *testing.T) {
	rSuffix := acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)
	webhookURL := fmt.Sprintf("https://example.com/webhook/tf-acc-%s-disabled", rSuffix)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create disabled endpoint
			{
				Config: testAccWebhookEndpointConfigWithEnabled(webhookURL, "raw", `["order.created"]`, false),
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
				Config: testAccWebhookEndpointConfigWithEnabled(webhookURL, "raw", `["order.created"]`, true),
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
