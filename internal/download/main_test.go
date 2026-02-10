package download

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Enable private IP scanning for all tests in this package
	// This is required for connecting to mock servers on localhost
	_ = os.Setenv("SURGE_ALLOW_PRIVATE_IPS", "true")
	os.Exit(m.Run())
}
