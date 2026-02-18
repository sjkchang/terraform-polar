// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories is used to instantiate a provider during acceptance testing.
// The factory function is called for each Terraform CLI command to create a provider
// server that the CLI can connect to and interact with.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"polar": providerserver.NewProtocol6WithError(New("test")()),
}

func testAccPreCheck(t *testing.T) {
	if os.Getenv("POLAR_ACCESS_TOKEN") == "" {
		t.Fatal("POLAR_ACCESS_TOKEN must be set for acceptance tests")
	}
	// Default to sandbox for acceptance tests if not explicitly set.
	if os.Getenv("POLAR_SERVER") == "" {
		t.Setenv("POLAR_SERVER", "sandbox")
	}
}
