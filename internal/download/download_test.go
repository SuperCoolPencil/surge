package download_test

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Allow private IPs for testing against local mock server
	os.Setenv("SURGE_ALLOW_PRIVATE_IPS", "true")
	os.Exit(m.Run())
}
