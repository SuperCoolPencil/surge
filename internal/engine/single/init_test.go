package single

import (
	"os"
)

func init() {
	// Allow tests to connect to localhost/private IPs
	os.Setenv("SURGE_ALLOW_PRIVATE_IPS", "true")
}
