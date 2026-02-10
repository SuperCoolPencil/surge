package concurrent

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Allow private IPs for tests as we use localhost
	os.Setenv("SURGE_ALLOW_PRIVATE_IPS", "true")
	os.Exit(m.Run())
}
