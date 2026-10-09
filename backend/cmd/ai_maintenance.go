package cmd

import (
	"context"
	"github.com/komari-monitor/komari/database/aimonitor"
	"github.com/komari-monitor/komari/database/dbcore"
	logger "github.com/komari-monitor/komari/utils/log"
	"time"
)

func maintainAIStorage(ctx context.Context, s *aimonitor.Store, now time.Time) error {
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return aimonitor.NewStore(s.DB.WithContext(bounded)).Maintain(now)
}
func compactAIStorage(ctx context.Context) {
	if err := maintainAIStorage(ctx, aimonitor.NewStore(dbcore.GetDBInstance()), time.Now().UTC()); err != nil {
		logger.Errorf("ai-monitoring", "AI retention maintenance failed: %v", err)
	}
}
