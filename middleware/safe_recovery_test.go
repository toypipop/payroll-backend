package middleware

import (
	"bytes"
	"log/slog"
	"net"
	"net/http/httptest"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRecoveryAndAccessLogsExcludeRequestAndPanicSecrets(t *testing.T) {
	for _, broken := range []bool{false, true} {
		t.Run(map[bool]string{false: "panic", true: "disconnected"}[broken], func(t *testing.T) {
			gin.SetMode(gin.ReleaseMode)
			var out bytes.Buffer
			previous := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&out, nil)))
			t.Cleanup(func() { slog.SetDefault(previous) })
			r := gin.New()
			r.Use(RequestID(), RequestLogger(), SafeRecovery())
			r.GET("/probe/:id", func(c *gin.Context) {
				if broken {
					panic(&net.OpError{Op: "write", Net: "tcp", Err: &os.SyscallError{Syscall: "write", Err: syscall.EPIPE}})
				}
				panic("SECRET_PANIC")
			})
			req := httptest.NewRequest("GET", "/probe/SECRET_PATH?token=SECRET_QUERY", nil)
			req.Header.Set("Cookie", "auth_token=SECRET_COOKIE")
			req.Header.Set("Authorization", "SECRET_AUTH")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			text := out.String()
			if strings.Contains(text, "SECRET_") || !strings.Contains(text, `"route":"/probe/:id"`) {
				t.Fatalf("unsafe or incomplete log: %s", text)
			}
			if broken {
				if !strings.Contains(text, `"status":499`) {
					t.Fatal("disconnect status missing")
				}
			} else if w.Code != 500 || strings.Contains(w.Body.String(), "SECRET_") {
				t.Fatal("unsafe panic response")
			}
			if w.Header().Get("X-Request-ID") == "" {
				t.Fatal("request ID missing")
			}
		})
	}
}

func TestAccessLogDoesNotRecordUnmatchedPaths(t *testing.T) {
	var out bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&out, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	r := gin.New()
	r.Use(RequestLogger())
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/SECRET_PATH?secret=SECRET_QUERY", nil))
	if strings.Contains(out.String(), "SECRET_") || !strings.Contains(out.String(), "unmatched") {
		t.Fatal("unmatched path was exposed")
	}
}
