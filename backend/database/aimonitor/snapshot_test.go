package aimonitor

import (
	"context"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"strings"
	"testing"
	"time"
)

type snapshotWriter struct {
	logger.Interface
	after  func()
	fired  bool
	totals map[string]int
}

func (l *snapshotWriter) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	query, _ := fc()
	if strings.HasPrefix(query, "SELECT COALESCE(SUM(calls)") && !strings.Contains(query, "GROUP BY") {
		if l.totals == nil {
			l.totals = map[string]int{}
		}
		l.totals[query]++
	}
	if !l.fired && strings.Contains(query, "AS total_token_count") && strings.Contains(query, "FROM ai_samples") {
		l.fired = true
		l.after()
	}
}

func TestOverviewConcurrentInsertUsesOneSnapshot(t *testing.T) {
	for _, count := range []int{1, MaxQueryRows + 1} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			s := fixture(t)
			if err := s.DB.Exec("PRAGMA journal_mode=WAL").Error; err != nil {
				t.Fatal(err)
			}
			now := time.Unix(1780000000, 0)
			sample := StoredSample{Node: "node", Sample: validReport(now).Samples[0]}
			rows := make([]StoredSample, count)
			for i := range rows {
				rows[i] = sample
				rows[i].SampleID = fmt.Sprint(i)
			}
			if err := s.DB.CreateInBatches(rows, 500).Error; err != nil {
				t.Fatal(err)
			}
			hook := &snapshotWriter{Interface: logger.Default.LogMode(logger.Silent)}
			hook.after = func() {
				row := sample
				row.SampleID = "concurrent"
				if err := s.DB.Create(&row).Error; err != nil {
					t.Fatal(err)
				}
			}
			reader := NewStore(s.DB.Session(&gorm.Session{Logger: hook}))
			v, err := reader.Overview(Query{Preset: "30D", Timezone: "UTC"}, now)
			if err != nil {
				t.Fatal(err)
			}
			if !hook.fired {
				t.Fatal("writer was not exercised")
			}
			for _, reads := range hook.totals {
				if reads > 1 {
					t.Errorf("repeated identical totals scan: %d", reads)
				}
			}
			t.Logf("diagnostics: %+v", v.Diagnostics)
			wantCalls, wantTokens := int64(count)*2, int64(count)*160
			check := func(label string, calls, tokens int64) {
				t.Helper()
				if calls != wantCalls || tokens != wantTokens {
					t.Errorf("%s calls/tokens = %d/%d; snapshot wants %d/%d", label, calls, tokens, wantCalls, wantTokens)
				}
			}
			if v.Diagnostics.RequestID == "" || v.Diagnostics.QueryCount == 0 || v.Diagnostics.QueryElapsedMS <= 0 || v.Diagnostics.ElapsedMS <= 0 {
				t.Fatal("missing bounded query diagnostics")
			}
			check("range", v.RangeUsage.CollectedCalls, v.RangeUsage.CollectedTokens)
			check("pricing", v.PricingCoverage.Calls, v.PricingCoverage.TotalTokenCount)
			check("price counts", v.PricingCoverage.PricedCalls+v.PricingCoverage.UnpricedCalls, v.PricingCoverage.TotalTokenCount)
			for label, groups := range map[string][]Aggregate{"models": v.Models, "tasks": v.Tasks} {
				var c, n int64
				for _, r := range groups {
					c += r.Calls
					n += r.TotalTokenCount
				}
				check(label, c, n)
			}
			var dc, dt, tc, tt int64
			for _, r := range v.Daily {
				dc += r.Calls
				dt += r.TotalTokenCount
				var c, n int64
				for _, m := range r.Models {
					c += m.Calls
					n += m.TotalTokenCount
				}
				if c != r.Calls || n != r.TotalTokenCount {
					t.Fatal("daily model mismatch")
				}
			}
			for _, r := range v.Trends {
				tc += r.Calls
				tt += r.TotalTokenCount
			}
			check("daily", dc, dt)
			check("internal trends", tc, tt)
			var persisted int64
			s.DB.Model(&StoredSample{}).Count(&persisted)
			if persisted != int64(count+1) {
				t.Fatal("concurrent writer did not commit")
			}
		})
	}
}
