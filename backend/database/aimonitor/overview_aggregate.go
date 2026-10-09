package aimonitor

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

type sqlTotals struct {
	Calls                  int64
	InputTokens            int64
	OutputTokens           int64
	CacheReadTokens        int64
	CacheWriteTokens       int64
	ReasoningTokens        int64
	TotalTokenCount        int64
	APIEquivalentCostUSD   sql.NullFloat64
	UnpricedCalls          int64
	PricedCalls            int64
	HermesEstimatedCostUSD sql.NullFloat64
	RecordedActualCostUSD  sql.NullFloat64
}

type sqlAggregate struct {
	Calls                  int64
	InputTokens            int64
	OutputTokens           int64
	CacheReadTokens        int64
	CacheWriteTokens       int64
	ReasoningTokens        int64
	TotalTokenCount        int64
	APIEquivalentCostUSD   sql.NullFloat64
	UnpricedCalls          int64
	PricedCalls            int64
	HermesEstimatedCostUSD sql.NullFloat64
	RecordedActualCostUSD  sql.NullFloat64
	Bucket                 float64
	Model                  string
	Provider               string
	BillingMode            string
	Task                   string
}

func (r sqlAggregate) totals() Totals {
	return sqlTotals{Calls: r.Calls, InputTokens: r.InputTokens, OutputTokens: r.OutputTokens, CacheReadTokens: r.CacheReadTokens, CacheWriteTokens: r.CacheWriteTokens, ReasoningTokens: r.ReasoningTokens, TotalTokenCount: r.TotalTokenCount, APIEquivalentCostUSD: r.APIEquivalentCostUSD, UnpricedCalls: r.UnpricedCalls, PricedCalls: r.PricedCalls, HermesEstimatedCostUSD: r.HermesEstimatedCostUSD, RecordedActualCostUSD: r.RecordedActualCostUSD}.totals()
}

func (r sqlTotals) totals() Totals {
	v := Totals{Calls: r.Calls, InputTokens: r.InputTokens, OutputTokens: r.OutputTokens, CacheReadTokens: r.CacheReadTokens, CacheWriteTokens: r.CacheWriteTokens, ReasoningTokens: r.ReasoningTokens, TotalTokenCount: r.TotalTokenCount, UnpricedCalls: r.UnpricedCalls, PricedCalls: r.PricedCalls}
	if r.APIEquivalentCostUSD.Valid {
		value := r.APIEquivalentCostUSD.Float64
		v.APIEquivalentCostUSD = &value
	}
	if r.HermesEstimatedCostUSD.Valid {
		value := r.HermesEstimatedCostUSD.Float64
		v.HermesEstimatedCostUSD = &value
	}
	if r.RecordedActualCostUSD.Valid {
		value := r.RecordedActualCostUSD.Float64
		v.RecordedActualCostUSD = &value
	}
	return v
}

func overviewFilters(q Query) (string, []any) {
	clauses, args := []string{}, []any{}
	if q.Node != "" {
		clauses, args = append(clauses, "node = ?"), append(args, q.Node)
	}
	if q.Model != "" {
		clauses, args = append(clauses, "model = ?"), append(args, q.Model)
	}
	if q.Task != "" {
		clauses, args = append(clauses, "task = ?"), append(args, q.Task)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " AND " + strings.Join(clauses, " AND "), args
}

func aggregateColumns(raw bool) string {
	total := "total_token_count"
	unpriced, priced := "unpriced_calls", "priced_calls"
	if raw {
		total = "input_tokens + output_tokens + cache_read_tokens + cache_write_tokens"
		unpriced = "CASE WHEN api_equivalent_cost_usd IS NULL THEN calls ELSE 0 END"
		priced = "CASE WHEN api_equivalent_cost_usd IS NULL THEN 0 ELSE calls END"
	}
	return "COALESCE(SUM(calls),0) AS calls, COALESCE(SUM(input_tokens),0) AS input_tokens, COALESCE(SUM(output_tokens),0) AS output_tokens, COALESCE(SUM(cache_read_tokens),0) AS cache_read_tokens, COALESCE(SUM(cache_write_tokens),0) AS cache_write_tokens, COALESCE(SUM(reasoning_tokens),0) AS reasoning_tokens, COALESCE(SUM(" + total + "),0) AS total_token_count, SUM(api_equivalent_cost_usd) AS api_equivalent_cost_usd, COALESCE(SUM(" + unpriced + "),0) AS unpriced_calls, COALESCE(SUM(" + priced + "),0) AS priced_calls, SUM(hermes_estimated_cost_usd) AS hermes_estimated_cost_usd, SUM(recorded_actual_cost_usd) AS recorded_actual_cost_usd"
}

func (s *Store) aggregateSource(q Query, from, to float64, selectPrefix, groupBy string, prefixArgs ...any) ([]sqlAggregate, error) {
	filters, filterArgs := overviewFilters(q)
	sources := []struct {
		table string
		at    string
		raw   bool
		where string
		args  []any
	}{
		{"ai_samples", "ended_at", true, "ended_at > ? AND ended_at <= ?", []any{from, to}},
		{"ai_hourly_rollups", "hour", false, "hour >= ? AND hour + 3600 <= ?", []any{from, to}},
	}
	all := []sqlAggregate{}
	for _, source := range sources {
		selects := aggregateColumns(source.raw)
		if selectPrefix != "" {
			selects = strings.ReplaceAll(selectPrefix, "{at}", source.at) + ", " + selects
		}
		query := "SELECT " + selects + " FROM " + source.table + " WHERE " + source.where + filters
		if groupBy != "" {
			query += " GROUP BY " + groupBy
		}
		query += " LIMIT ?"
		limit := MaxResponseGroups
		// The documented overview trend resolution has at most 1,440 minute buckets
		// for 24 hours and 720 hourly buckets for 30 days.
		if groupBy == "bucket" {
			limit = 1440
		}
		args := append([]any{}, prefixArgs...)
		args = append(args, source.args...)
		args = append(args, filterArgs...)
		args = append(args, limit+1)
		var rows []sqlAggregate
		if err := s.DB.Raw(query, args...).Scan(&rows).Error; err != nil {
			return nil, err
		}
		if len(rows) > limit {
			return nil, ErrQueryLimit
		}
		all = append(all, rows...)
	}
	return all, nil
}

func combineTotals(rows []sqlAggregate) (Totals, error) {
	var result Totals
	for _, row := range rows {
		total := row.totals()
		if !safeAdd(result, total) {
			return Totals{}, fmt.Errorf("AI totals exceed safe JSON integer range")
		}
		result.add(total)
	}
	return result, nil
}

func (s *Store) aggregateTotals(q Query, from, to float64) (Totals, error) {
	rows, err := s.aggregateSource(q, from, to, "", "")
	if err != nil {
		return Totals{}, err
	}
	return combineTotals(rows)
}

func (s *Store) aggregateModels(q Query, from, to float64) ([]Aggregate, error) {
	rows, err := s.aggregateSource(q, from, to, "model, provider, billing_mode", "model, provider, billing_mode")
	if err != nil {
		return nil, err
	}
	out := map[string]*Aggregate{}
	for _, row := range rows {
		key := row.Model + "\x00" + row.Provider + "\x00" + row.BillingMode
		if out[key] == nil {
			out[key] = &Aggregate{Model: row.Model, Provider: row.Provider, BillingMode: row.BillingMode}
		}
		total := row.totals()
		if !safeAdd(out[key].Totals, total) {
			return nil, fmt.Errorf("AI totals exceed safe JSON integer range")
		}
		out[key].Totals.add(total)
	}
	if len(out) > MaxResponseGroups {
		return nil, ErrQueryLimit
	}
	result := make([]Aggregate, 0, len(out))
	for _, row := range out {
		result = append(result, *row)
	}
	sortAggregates(result)
	return result, nil
}

func (s *Store) aggregateTasks(q Query, from, to float64) ([]Aggregate, error) {
	rows, err := s.aggregateSource(q, from, to, "task", "task")
	if err != nil {
		return nil, err
	}
	out := map[string]*Aggregate{}
	for _, row := range rows {
		if out[row.Task] == nil {
			out[row.Task] = &Aggregate{Task: row.Task}
		}
		total := row.totals()
		if !safeAdd(out[row.Task].Totals, total) {
			return nil, fmt.Errorf("AI totals exceed safe JSON integer range")
		}
		out[row.Task].Totals.add(total)
	}
	if len(out) > MaxResponseGroups {
		return nil, ErrQueryLimit
	}
	result := make([]Aggregate, 0, len(out))
	for _, row := range out {
		result = append(result, *row)
	}
	sortAggregates(result)
	return result, nil
}

func (s *Store) aggregateTrends(q Query, from, to, step float64) ([]Trend, error) {
	rows, err := s.aggregateSource(q, from, to, "CAST({at} / ? AS INTEGER) * ? AS bucket", "bucket", step, step)
	if err != nil {
		return nil, err
	}
	out := map[float64]*Trend{}
	for _, row := range rows {
		if out[row.Bucket] == nil {
			out[row.Bucket] = &Trend{Timestamp: row.Bucket}
		}
		total := row.totals()
		if !safeAdd(out[row.Bucket].Totals, total) {
			return nil, fmt.Errorf("AI totals exceed safe JSON integer range")
		}
		out[row.Bucket].Totals.add(total)
	}
	result := make([]Trend, 0, len(out))
	for _, row := range out {
		result = append(result, *row)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Timestamp < result[j].Timestamp })
	return result, nil
}

func dayRanges(q Query, loc *time.Location) [][3]any {
	start := time.Unix(int64(q.From), 0).In(loc)
	cursor := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, loc)
	out := [][3]any{}
	for float64(cursor.Unix()) < q.To {
		next := cursor.AddDate(0, 0, 1)
		out = append(out, [3]any{cursor.Format("2006-01-02"), float64(cursor.Unix()), float64(next.Unix())})
		cursor = next
	}
	return out
}

func (s *Store) aggregateDaily(q Query, loc *time.Location) ([]Day, error) {
	out := map[string]*Day{}
	models := map[string]map[string]*Aggregate{}
	for _, day := range dayRanges(q, loc) {
		from, to := day[1].(float64), day[2].(float64)
		if from < q.From {
			from = q.From
		}
		if to > q.To {
			to = q.To
		}
		rows, err := s.aggregateSource(q, from, to, "model, provider, billing_mode", "model, provider, billing_mode")
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			continue
		}
		label := day[0].(string)
		if out[label] == nil {
			out[label] = &Day{Date: label, Models: []Aggregate{}}
			models[label] = map[string]*Aggregate{}
		}
		for _, row := range rows {
			key := row.Model + "\x00" + row.Provider + "\x00" + row.BillingMode
			if models[label][key] == nil {
				models[label][key] = &Aggregate{Model: row.Model, Provider: row.Provider, BillingMode: row.BillingMode}
			}
			total := row.totals()
			if !safeAdd(out[label].Totals, total) || !safeAdd(models[label][key].Totals, total) {
				return nil, fmt.Errorf("AI totals exceed safe JSON integer range")
			}
			out[label].Totals.add(total)
			models[label][key].Totals.add(total)
		}
	}
	modelCells := 0
	for _, byModel := range models {
		modelCells += len(byModel)
	}
	if len(out) > 31 || modelCells > 31*MaxResponseGroups {
		return nil, ErrQueryLimit
	}
	result := make([]Day, 0, len(out))
	for label, day := range out {
		for _, model := range models[label] {
			day.Models = append(day.Models, *model)
		}
		if len(day.Models) > MaxResponseGroups {
			return nil, ErrQueryLimit
		}
		sortAggregates(day.Models)
		result = append(result, *day)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Date < result[j].Date })
	return result, nil
}

func (s *Store) aggregateOverview(q Query, now time.Time, loc *time.Location, v Overview) (Overview, error) {
	var err error
	if v.PricingCoverage, err = s.aggregateTotals(q, q.From, q.To); err != nil {
		return v, err
	}
	if v.Models, err = s.aggregateModels(q, q.From, q.To); err != nil {
		return v, err
	}
	if v.Tasks, err = s.aggregateTasks(q, q.From, q.To); err != nil {
		return v, err
	}
	step := 60.0
	if q.To-q.From > 86400 {
		step = 3600
	}
	if v.Trends, err = s.aggregateTrends(q, q.From, q.To, step); err != nil {
		return v, err
	}
	if v.Daily, err = s.aggregateDaily(q, loc); err != nil {
		return v, err
	}

	// The legacy raw path remains the proof source for per-group coverage rates.
	// Work-limit overflow deliberately yields N/A instead of fabricating a rate.
	if rows, detailErr := s.details(q); detailErr == nil {
		models := map[string]*Aggregate{}
		for i := range v.Models {
			models[v.Models[i].Model+"\x00"+v.Models[i].Provider+"\x00"+v.Models[i].BillingMode] = &v.Models[i]
		}
		tasks := map[string]*Aggregate{}
		for i := range v.Tasks {
			tasks[v.Tasks[i].Task] = &v.Tasks[i]
		}
		days := map[string]*Day{}
		for i := range v.Daily {
			days[v.Daily[i].Date] = &v.Daily[i]
		}
		modelRows, taskRows, dayRows := map[string][]StoredSample{}, map[string][]StoredSample{}, map[string][]StoredSample{}
		for _, row := range rows {
			key := row.Model + "\x00" + row.Provider + "\x00" + row.BillingMode
			modelRows[key] = append(modelRows[key], row)
			taskRows[row.Task] = append(taskRows[row.Task], row)
			dayRows[time.Unix(int64(row.EndedAt), 0).In(loc).Format("2006-01-02")] = append(dayRows[time.Unix(int64(row.EndedAt), 0).In(loc).Format("2006-01-02")], row)
		}
		for key, rows := range modelRows {
			if row := models[key]; row != nil {
				row.AverageTPS = coveredAverage(rows, q.From, q.To)
				setShare(row, v.PricingCoverage)
			}
		}
		for key, rows := range taskRows {
			if row := tasks[key]; row != nil {
				row.AverageTPS = coveredAverage(rows, q.From, q.To)
				setShare(row, v.PricingCoverage)
			}
		}
		for date, rows := range dayRows {
			if row := days[date]; row != nil {
				start, _ := time.ParseInLocation("2006-01-02", date, loc)
				from, to := float64(start.Unix()), float64(start.AddDate(0, 0, 1).Unix())
				if from < q.From {
					from = q.From
				}
				if to > q.To {
					to = q.To
				}
				row.AverageTPS = coveredAverage(rows, from, to)
			}
		}
	}
	for i := range v.Models {
		setShare(&v.Models[i], v.PricingCoverage)
	}
	for i := range v.Tasks {
		setShare(&v.Tasks[i], v.PricingCoverage)
	}

	states, err := s.states(q.Node)
	if err != nil {
		return v, err
	}
	v.Nodes, err = s.nodes(states, q.Node, now)
	if err != nil {
		return v, err
	}
	v.Accounts, err = s.accounts(states, now)
	if err != nil {
		return v, err
	}

	recent := q
	recent.From, recent.To = float64(now.Add(-24*time.Hour).Unix()), float64(now.Unix())
	recentTotals, err := s.aggregateTotals(recent, recent.From, recent.To)
	if err != nil {
		return v, err
	}
	v.Live.Calls24h = recentTotals.Calls
	today := now.In(loc)
	dayStart := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, loc)
	todayTotals, err := s.aggregateTotals(q, float64(dayStart.Unix()), float64(now.Unix()))
	if err != nil {
		return v, err
	}
	v.Live.TodayTokens = todayTotals.TotalTokens()
	v.Live.TodayAPIEquivalentCostUSD = todayTotals.APIEquivalentCostUSD
	v.Live.UnpricedCalls = todayTotals.UnpricedCalls

	liveQuery := q
	liveQuery.From, liveQuery.To = float64(now.Add(-60*time.Second).Unix()), float64(now.Unix())
	if rows, liveErr := s.details(liveQuery); liveErr == nil {
		v.Live = computeLive(v.Live, rows, states, v.Nodes, now)
	}
	return v, nil
}

func (s *Store) countDetails(q Query) (int64, error) {
	var count int64
	err := filtered(s.DB.Model(&StoredSample{}), q).Where("ended_at > ? AND ended_at <= ?", q.From, q.To).Count(&count).Error
	return count, err
}
