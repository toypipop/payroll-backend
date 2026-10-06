package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// ConfigureTrustedProxies limits forwarded client metadata to requests that
// arrived through a known reverse-proxy network. An empty value trusts no
// proxy, so direct callers cannot spoof X-Forwarded-* headers.
func ConfigureTrustedProxies(router *gin.Engine, rawCIDRs string) error {
	parts := strings.Split(rawCIDRs, ",")
	trusted := make([]string, 0, len(parts))
	for _, part := range parts {
		if cidr := strings.TrimSpace(part); cidr != "" {
			trusted = append(trusted, cidr)
		}
	}
	return router.SetTrustedProxies(trusted)
}
