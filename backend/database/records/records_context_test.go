package records

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestHistoryContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	now := time.Now()
	if _, err := GetRecordsByClientAndTimeForLoadTypeMaxPointsContext(ctx, "node", now.Add(-time.Hour), now, "cpu", 100); !errors.Is(err, context.Canceled) {
		t.Fatalf("node history: %v", err)
	}
	if _, err := GetRecordsByTimeForLoadTypeMaxPointsContext(ctx, now.Add(-time.Hour), now, "cpu", 100); !errors.Is(err, context.Canceled) {
		t.Fatalf("all history: %v", err)
	}
}
