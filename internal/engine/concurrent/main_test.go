package concurrent

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Enable private IP access for tests
	os.Setenv("SURGE_ALLOW_PRIVATE_IPS", "true")
	os.Exit(m.Run())
}
