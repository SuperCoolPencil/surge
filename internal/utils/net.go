package utils

import (
	"context"
	"fmt"
	"net"
	"os"
)

// Private IP ranges
var privateIPBlocks []*net.IPNet

func init() {
	for _, cidr := range []string{
		"127.0.0.0/8",    // IPv4 loopback
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
		"169.254.0.0/16", // RFC3927 link-local
		"::1/128",        // IPv6 loopback
		"fe80::/10",      // IPv6 link-local
		"fc00::/7",       // IPv6 unique local
	} {
		_, block, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(fmt.Errorf("parse error on %q: %v", cidr, err))
		}
		privateIPBlocks = append(privateIPBlocks, block)
	}
}

// IsPrivateIP checks if an IP address belongs to a private network
func IsPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}

	for _, block := range privateIPBlocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// SafeDialContext returns a DialContext function that validates IPs against private ranges
func SafeDialContext(dialer *net.Dialer) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}

		// Resolve IPs
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}

		var safeIPs []string
		allowPrivate := os.Getenv("SURGE_ALLOW_PRIVATE_IPS") == "true"

		for _, ip := range ips {
			if allowPrivate || !IsPrivateIP(ip.IP) {
				safeIPs = append(safeIPs, ip.IP.String())
			}
		}

		if len(safeIPs) == 0 {
			return nil, fmt.Errorf("security: blocked access to private IP for host %s", host)
		}

		// Try to dial each safe IP
		var firstErr error
		for _, ip := range safeIPs {
			// Dial the IP directly
			// The original address was host:port, now we use ip:port
			// net.JoinHostPort will handle adding brackets for IPv6
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip, port))
			if err == nil {
				return conn, nil
			}
			if firstErr == nil {
				firstErr = err
			}
		}

		if firstErr != nil {
			return nil, firstErr
		}
		return nil, fmt.Errorf("failed to dial any IP")
	}
}
