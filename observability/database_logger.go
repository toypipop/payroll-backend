package observability

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type databaseLogger struct {
	output *slog.Logger
	level  logger.LogLevel
	slow   time.Duration
}

func NewDatabaseLogger(output *slog.Logger, slow time.Duration) logger.Interface {
	return &databaseLogger{output: output, level: logger.Warn, slow: slow}
}

func (l *databaseLogger) LogMode(level logger.LogLevel) logger.Interface {
	clone := *l
	clone.level = level
	return &clone
}

// GORM's generic messages and arguments can contain driver/connection details.
func (l *databaseLogger) Info(context.Context, string, ...interface{}) {}
func (l *databaseLogger) Warn(context.Context, string, ...interface{}) {}
func (l *databaseLogger) Error(ctx context.Context, _ string, _ ...interface{}) {
	if l.level >= logger.Error {
		l.output.ErrorContext(ctx, "database operation failed")
	}
}

func (l *databaseLogger) ParamsFilter(_ context.Context, sql string, _ ...interface{}) (string, []interface{}) {
	return sql, nil
}

func (l *databaseLogger) Trace(ctx context.Context, started time.Time, _ func() (string, int64), err error) {
	// Never evaluate/log expanded SQL, including literal SQL and parameter values.
	if l.level == logger.Silent {
		return
	}
	duration := time.Since(started)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) && l.level >= logger.Error {
		attrs := append([]any{"duration", duration.String()}, ErrorAttrs(err)...)
		var state interface{ SQLState() string }
		if errors.As(err, &state) && strings.HasPrefix(state.SQLState(), "23") {
			l.output.WarnContext(ctx, "database constraint conflict", attrs...)
		} else {
			l.output.ErrorContext(ctx, "database operation failed", attrs...)
		}
	} else if l.slow > 0 && duration > l.slow && l.level >= logger.Warn {
		l.output.WarnContext(ctx, "slow database operation", "duration", duration.String())
	}
}
