// This package exists to wrap our e2e provisioning and test framework so that it
// can be run via 'go test ./e2e'. See './framework/framework.go'
package e2e

import (
	"os"
	"testing"

	"github.com/hashicorp/nomad/e2e/framework"

	_ "github.com/nomad-node-problem-detector/e2e/npd"
)

func TestE2E(t *testing.T) {
	if os.Getenv("NOMAD_E2E") == "" {
		t.Skip("Skipping e2e tests, NOMAD_E2E not set")
	} else {
		framework.Run(t)
	}
}
