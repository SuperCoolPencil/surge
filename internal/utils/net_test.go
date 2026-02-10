package utils

import (
	"context"
	"net"
	"os"
	"testing"
)

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip       string
		isPrivate bool
	}{
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.1.1", true},
		{"169.254.1.1", true},
		{"0.0.0.0", true},
		{"::1", true},
		{"fe80::1", true},
		{"fc00::1", true},
		{"fd00::1", true}, // Unique local (fc00::/7 includes fd00::/8)

		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"11.0.0.1", false},
		{"172.15.0.1", false},
		{"172.32.0.1", false},
		{"192.169.0.1", false},
	}

	for _, tc := range tests {
		ip := net.ParseIP(tc.ip)
		if got := IsPrivateIP(ip); got != tc.isPrivate {
			t.Errorf("IsPrivateIP(%s) = %v; want %v", tc.ip, got, tc.isPrivate)
		}
	}
}

func TestSafeDialContext_Blocked(t *testing.T) {
	os.Unsetenv("SURGE_ALLOW_PRIVATE_IPS")

	// Localhost should be blocked
	dialer := &net.Dialer{}
	safeDial := SafeDialContext(dialer)

	// Start a dummy listener to ensure connection COULD succeed if not blocked
	// This confirms that we are indeed blocking it, not just failing to connect
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	_, port, _ := net.SplitHostPort(l.Addr().String())

	// Try with IP
	conn, err := safeDial(context.Background(), "tcp", net.JoinHostPort("127.0.0.1", port))
	if err == nil {
		conn.Close()
		t.Fatal("Expected error when dialing 127.0.0.1, got success")
	}

	// Try with localhost
	conn, err = safeDial(context.Background(), "tcp", net.JoinHostPort("localhost", port))
	if err == nil {
		conn.Close()
		t.Fatal("Expected error when dialing localhost, got success")
	}
}

func TestSafeDialContext_Allowed(t *testing.T) {
	os.Setenv("SURGE_ALLOW_PRIVATE_IPS", "true")
	defer os.Unsetenv("SURGE_ALLOW_PRIVATE_IPS")

	// Localhost should NOT be blocked
	dialer := &net.Dialer{}
	safeDial := SafeDialContext(dialer)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	_, port, _ := net.SplitHostPort(l.Addr().String())

	// Try with IP
	conn, err := safeDial(context.Background(), "tcp", net.JoinHostPort("127.0.0.1", port))
	if err != nil {
		t.Fatalf("Expected success when dialing 127.0.0.1 with env var, got error: %v", err)
	}
	conn.Close()
}
