package observability

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm/logger"
)

func TestDatabaseLoggerDoesNotEvaluateSQLOrExposeDriverValues(t *testing.T) {
	var out bytes.Buffer
	l := NewDatabaseLogger(slog.New(slog.NewJSONHandler(&out, nil)), time.Second)
	err := &pgconn.PgError{Code: "23505", Message: "SECRET_MESSAGE", Detail: "SECRET_PASSWORD", Hint: "SECRET_COOKIE"}
	l.Trace(context.Background(), time.Now(), func() (string, int64) { t.Fatal("SQL must not be evaluated"); return "SECRET_SQL", 1 }, err)
	text := out.String()
	if strings.Contains(text, "SECRET_") || !strings.Contains(text, `"sql_state":"23505"`) {
		t.Fatalf("unsafe or incomplete log: %s", text)
	}
	if !strings.Contains(text, `"level":"WARN"`) {
		t.Fatal("constraint conflict treated as a server failure")
	}
	filtered, ok := l.(interface {
		ParamsFilter(context.Context, string, ...interface{}) (string, []interface{})
	})
	if !ok {
		t.Fatal("missing parameter filter")
	}
	_, params := filtered.ParamsFilter(context.Background(), "SELECT $1", "SECRET_PARAMETER")
	if len(params) != 0 {
		t.Fatal("parameters retained")
	}
}

func TestDatabaseLoggerSlowAndSilentModes(t *testing.T) {
	var out bytes.Buffer
	l := NewDatabaseLogger(slog.New(slog.NewJSONHandler(&out, nil)), time.Millisecond)
	callback := func() (string, int64) { t.Fatal("expanded SQL must not be evaluated"); return "", 0 }
	l.Trace(context.Background(), time.Now().Add(-time.Second), callback, nil)
	if !strings.Contains(out.String(), "slow database operation") {
		t.Fatal("slow operation missing")
	}
	out.Reset()
	l.LogMode(logger.Silent).Trace(context.Background(), time.Now().Add(-time.Second), callback, nil)
	if out.Len() != 0 {
		t.Fatal("silent mode logged")
	}
}
