package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestConfigureTrustedProxiesOnlyAcceptsForwardedClientIPFromTrustedNetwork(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name    string
		trusted string
		remote  string
		want    string
	}{
		{name: "untrusted caller cannot spoof", trusted: "", remote: "203.0.113.10:1234", want: "203.0.113.10"},
		{name: "trusted proxy forwards client IP", trusted: "172.18.0.0/16", remote: "172.18.0.5:1234", want: "203.0.113.10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			if err := ConfigureTrustedProxies(router, tt.trusted); err != nil {
				t.Fatalf("configure trusted proxies: %v", err)
			}
			router.GET("/", func(c *gin.Context) { c.String(http.StatusOK, c.ClientIP()) })

			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.RemoteAddr = tt.remote
			request.Header.Set("X-Forwarded-For", "203.0.113.10")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if got := response.Body.String(); got != tt.want {
				t.Fatalf("client IP = %q, want %q", got, tt.want)
			}
		})
	}
}
