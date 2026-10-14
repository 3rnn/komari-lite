package aimonitor

import (
	"context"
	"gorm.io/gorm/logger"
	"time"
)

// Diagnostics contains timings only, never SQL, selectors or account identifiers.
type Diagnostics struct {
	RequestID      string  `json:"request_id"`
	ElapsedMS      float64 `json:"elapsed_ms"`
	QueryElapsedMS float64 `json:"query_elapsed_ms"`
	QueryCount     int     `json:"query_count"`
}
type overviewLogger struct {
	logger.Interface
	stats *Diagnostics
}

func (l *overviewLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	l.stats.QueryCount++
	l.stats.QueryElapsedMS += float64(time.Since(begin).Microseconds()) / 1000
	l.Interface.Trace(ctx, begin, fc, err)
}
