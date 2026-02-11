package concurrent

import (
	"os"
)

func init() {
	// Allow private IPs for testing (localhost)
	os.Setenv("SURGE_ALLOW_PRIVATE_IPS", "true")
}
