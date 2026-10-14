package httpx

import (
	"bytes"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestInternalErrorLogAndResponseDoNotExposeDatabaseDetail(t *testing.T) {
	var out bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&out, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	r := gin.New()
	r.GET("/probe/:id", func(c *gin.Context) {
		RespondError(c, &pgconn.PgError{Code: "XX000", Message: "SECRET_MESSAGE", Detail: "SECRET_ACCOUNT"})
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/probe/SECRET_PATH", nil))
	if w.Code != 500 || strings.Contains(out.String()+w.Body.String(), "SECRET_") {
		t.Fatal("internal error exposed private data")
	}
	if !strings.Contains(out.String(), `"sql_state":"XX000"`) {
		t.Fatal("safe error code missing")
	}
}
