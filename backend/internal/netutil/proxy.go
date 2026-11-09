// Package netutil holds networking helpers shared by TigerSwap's HTTP services.
package netutil

import (
	"fmt"
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

// ConfigureTrustedProxies tells gin which peers may set the client IP through a
// forwarding header.
//
// gin's default trusted set is every address (0.0.0.0/0 and ::/0), so
// ClientIP() returns whatever `X-Forwarded-For` the caller sent. Every limiter
// keys on that value, so an attacker could rotate the header to mint a fresh
// rate-limit bucket per request and bypass the limit entirely. The header is
// only honoured when the immediate peer is a configured proxy; an unconfigured
// deployment trusts none, and the peer's own address is then the key.
//
// raw is a comma-separated list of CIDRs or bare IPs (a bare IP is treated as a
// /32 or /128). The wildcard is rejected, because trusting every peer is
// exactly the default this exists to replace.
func ConfigureTrustedProxies(router *gin.Engine, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return router.SetTrustedProxies(nil)
	}

	var cidrs []string
	for _, part := range strings.Split(raw, ",") {
		entry := strings.TrimSpace(part)
		if entry == "" {
			continue
		}
		if entry == "*" || entry == "0.0.0.0/0" || entry == "::/0" {
			return fmt.Errorf("trusted proxies must not trust every address (%q)", entry)
		}
		if _, _, err := net.ParseCIDR(entry); err == nil {
			cidrs = append(cidrs, entry)
			continue
		}
		if ip := net.ParseIP(entry); ip != nil {
			cidrs = append(cidrs, entry)
			continue
		}
		return fmt.Errorf("trusted proxy entry %q is not an IP or CIDR", entry)
	}

	if len(cidrs) == 0 {
		return router.SetTrustedProxies(nil)
	}
	return router.SetTrustedProxies(cidrs)
}
