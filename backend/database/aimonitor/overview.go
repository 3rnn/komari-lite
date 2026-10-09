package aimonitor

import (
	"encoding/json"
	"fmt"
	"github.com/komari-monitor/komari/database/models"
	"gorm.io/gorm"
	"sort"
	"time"
)

const MaxQueryRows = 50000
const MaxOverviewNodes = 1000

type Query struct {
	From     float64 `json:"from"`
	To       float64 `json:"to"`
	Timezone string  `json:"timezone"`
	Node     string  `json:"node"`
	Model    string  `json:"model"`
	Task     string  `json:"task"`
}
type Aggregate struct {
	Totals
	Model       string   `json:"model,omitempty"`
	Provider    string   `json:"provider,omitempty"`
	BillingMode string   `json:"billing_mode,omitempty"`
	Task        string   `json:"task,omitempty"`
	AverageTPS  *float64 `json:"average_tps"`
	CostShare   *float64 `json:"cost_share"`
}
type Trend struct {
	Timestamp float64 `json:"timestamp"`
	Totals
}
type Day struct {
	Date string `json:"date"`
	Totals
	AverageTPS *float64    `json:"average_tps"`
	Models     []Aggregate `json:"models"`
}
type Live struct {
	TPS                       *float64 `json:"tps"`
	InputTPS                  *float64 `json:"input_tps"`
	OutputTPS                 *float64 `json:"output_tps"`
	CacheReadTPS              *float64 `json:"cache_read_tps"`
	RPS                       *float64 `json:"rps"`
	RPM                       *float64 `json:"rpm"`
	TPM                       *float64 `json:"tpm"`
	RollingCalls              int64    `json:"rolling_calls"`
	RollingTokens             int64    `json:"rolling_tokens"`
	Calls24h                  int64    `json:"calls_24h"`
	TodayTokens               int64    `json:"today_tokens"`
	TodayAPIEquivalentCostUSD *float64 `json:"today_api_equivalent_cost_usd"`
	UnpricedCalls             int64    `json:"unpriced_calls"`
	Status                    string   `json:"status"`
}
type Account struct {
	Quota
	NodeSources  []string `json:"node_sources"`
	DisplayLabel string   `json:"display_label"`
}
type Node struct {
	UUID                    string   `json:"uuid"`
	Name                    string   `json:"name"`
	Enabled                 bool     `json:"enabled"`
	Status                  string   `json:"status"`
	CurrentModel            string   `json:"current_model"`
	LastSuccessAt           *float64 `json:"last_success_at"`
	LastReportAt            *float64 `json:"last_report_at"`
	SamplingIntervalSeconds float64  `json:"sampling_interval_seconds"`
	DeliveryStatus          string   `json:"delivery_status"`
}
type Overview struct {
	Range           Query             `json:"range"`
	Definitions     map[string]string `json:"definitions"`
	Live            Live              `json:"live"`
	Accounts        []Account         `json:"accounts"`
	Trends          []Trend           `json:"trends"`
	Daily           []Day             `json:"daily"`
	Models          []Aggregate       `json:"models"`
	Tasks           []Aggregate       `json:"tasks"`
	Nodes           []Node            `json:"nodes"`
	PricingCoverage Totals            `json:"pricing_coverage"`
}

func filtered(db *gorm.DB, q Query) *gorm.DB {
	if q.Node != "" {
		db = db.Where("node = ?", q.Node)
	}
	if q.Model != "" {
		db = db.Where("model = ?", q.Model)
	}
	if q.Task != "" {
		db = db.Where("task = ?", q.Task)
	}
	return db
}
func (s *Store) details(q Query) ([]StoredSample, error) {
	var rows []StoredSample
	err := filtered(s.DB, q).Where("ended_at > ? AND ended_at <= ?", q.From, q.To).Order("ended_at").Limit(MaxQueryRows + 1).Find(&rows).Error
	if len(rows) > MaxQueryRows {
		return nil, ErrQueryLimit
	}
	return rows, err
}
func (s *Store) Overview(q Query, now time.Time) (Overview, error) {
	v := Overview{Range: q, Definitions: map[string]string{
		"cost_label": "Estimated API-Equivalent Cost (USD)", "cost": "Calculated using reference API token pricing. This is not an actual ChatGPT Plus charge.",
		"tokens":        "input_tokens excludes cache reads/writes; total=input+cache_read+cache_write+output; reasoning is an output subset",
		"throughput":    "Sampled token throughput, not generation speed. TPS/RPS use the latest covered 10 seconds; RPM/TPM use a separately covered rolling 60 seconds ending at the latest common sample. Unknown coverage is N/A.",
		"history":       "Collected deltas only. Detailed samples are assigned by ended_at; older hourly rollups are included only for fully selected UTC hour buckets. Historical daily timezone boundaries assign whole buckets; no interpolation.",
		"cost_coverage": "Cost totals sum priced usage only; unpriced_calls is separate. Hermes estimated and recorded actual costs are separate known-value subtotals, not reference costs.",
	}, Live: Live{Status: "unavailable"}, Accounts: []Account{}, Trends: []Trend{}, Daily: []Day{}, Models: []Aggregate{}, Tasks: []Aggregate{}, Nodes: []Node{}}
	loc, err := time.LoadLocation(q.Timezone)
	if err != nil {
		return v, fmt.Errorf("unknown timezone")
	}
	settings, err := s.Settings()
	if err != nil {
		return v, err
	}
	if q.From < float64(now.Unix()-int64(settings.RetentionDays)*86400) {
		return v, fmt.Errorf("AI range exceeds configured retention")
	}
	if !finite(q.From) || !finite(q.To) || q.From >= q.To || q.To-q.From > 30*86400 || q.From < float64(now.Unix()-30*86400) || q.To > float64(now.Unix()+120) || !text(q.Node, 36, false) || !text(q.Model, 128, false) || !text(q.Task, 128, false) {
		return v, fmt.Errorf("AI range must cover at most 30 retained days")
	}
	rows, err := s.details(q)
	if err != nil {
		return v, err
	}
	var rolls []Rollup
	if err := filtered(s.DB, q).Where("hour >= ? AND hour + 3600 <= ?", q.From, q.To).Order("hour").Limit(MaxQueryRows + 1).Find(&rolls).Error; err != nil {
		return v, err
	}
	if len(rolls) > MaxQueryRows {
		return v, ErrQueryLimit
	}
	modelMap := map[string]*Aggregate{}
	taskMap := map[string]*Aggregate{}
	trendMap := map[float64]*Trend{}
	dayMap := map[string]*Day{}
	dayModels := map[string]map[string]*Aggregate{}
	modelRows := map[string][]StoredSample{}
	taskRows := map[string][]StoredSample{}
	dayRows := map[string][]StoredSample{}
	step := 60.0
	if q.To-q.From > 86400 {
		step = 3600
	}
	var aggregationErr error
	add := func(at float64, model, provider, mode, task string, t Totals) {
		if aggregationErr != nil {
			return
		}
		if !safeAdd(v.PricingCoverage, t) {
			aggregationErr = fmt.Errorf("AI totals exceed safe JSON integer range")
			return
		}
		v.PricingCoverage.add(t)
		key := model + "\x00" + provider + "\x00" + mode
		if modelMap[key] == nil {
			modelMap[key] = &Aggregate{Model: model, Provider: provider, BillingMode: mode}
		}
		modelMap[key].Totals.add(t)
		if taskMap[task] == nil {
			taskMap[task] = &Aggregate{Task: task}
		}
		taskMap[task].Totals.add(t)
		timestamp := float64(int64(at/step)) * step
		if trendMap[timestamp] == nil {
			trendMap[timestamp] = &Trend{Timestamp: timestamp}
		}
		trendMap[timestamp].Totals.add(t)
		date := time.Unix(int64(at), 0).In(loc).Format("2006-01-02")
		if dayMap[date] == nil {
			dayMap[date] = &Day{Date: date, Models: []Aggregate{}}
			dayModels[date] = map[string]*Aggregate{}
		}
		dayMap[date].Totals.add(t)
		if dayModels[date][key] == nil {
			dayModels[date][key] = &Aggregate{Model: model, Provider: provider, BillingMode: mode}
		}
		dayModels[date][key].Totals.add(t)
	}
	for _, row := range rows {
		key := row.Model + "\x00" + row.Provider + "\x00" + row.BillingMode
		modelRows[key] = append(modelRows[key], row)
		taskRows[row.Task] = append(taskRows[row.Task], row)
		date := time.Unix(int64(row.EndedAt), 0).In(loc).Format("2006-01-02")
		dayRows[date] = append(dayRows[date], row)
		var totals Totals
		totals.addSample(row)
		add(row.EndedAt, row.Model, row.Provider, row.BillingMode, row.Task, totals)
	}
	for _, row := range rolls {
		add(row.Hour, row.Model, row.Provider, row.BillingMode, row.Task, row.Totals)
	}
	if aggregationErr != nil {
		return v, aggregationErr
	}
	if len(modelMap) > MaxResponseGroups || len(taskMap) > MaxResponseGroups {
		return v, ErrQueryLimit
	}
	for key, a := range modelMap {
		a.AverageTPS = coveredAverage(modelRows[key], q.From, q.To)
		setShare(a, v.PricingCoverage)
		v.Models = append(v.Models, *a)
	}
	for key, a := range taskMap {
		a.AverageTPS = coveredAverage(taskRows[key], q.From, q.To)
		setShare(a, v.PricingCoverage)
		v.Tasks = append(v.Tasks, *a)
	}
	for _, row := range trendMap {
		v.Trends = append(v.Trends, *row)
	}
	for date, row := range dayMap {
		dayStart, _ := time.ParseInLocation("2006-01-02", date, loc)
		dayEnd := dayStart.AddDate(0, 0, 1)
		start, end := float64(dayStart.Unix()), float64(dayEnd.Unix())
		if q.From > start {
			start = q.From
		}
		if q.To < end {
			end = q.To
		}
		row.AverageTPS = coveredAverage(dayRows[date], start, end)
		for _, model := range dayModels[date] {
			row.Models = append(row.Models, *model)
		}
		sortAggregates(row.Models)
		v.Daily = append(v.Daily, *row)
	}
	sortAggregates(v.Models)
	sortAggregates(v.Tasks)
	sort.Slice(v.Trends, func(i, j int) bool { return v.Trends[i].Timestamp < v.Trends[j].Timestamp })
	sort.Slice(v.Daily, func(i, j int) bool { return v.Daily[i].Date < v.Daily[j].Date })
	if v.PricingCoverage.TotalTokenCount > 9_007_199_254_740_991 {
		return v, fmt.Errorf("AI totals exceed safe JSON integer range")
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
	recentQuery := q
	recentQuery.From = float64(now.Unix() - 86400)
	recentQuery.To = float64(now.Unix())
	recent, err := s.details(recentQuery)
	if err != nil {
		return v, err
	}
	var recentRolls []Rollup
	if err := filtered(s.DB, recentQuery).Where("hour >= ? AND hour + 3600 <= ?", recentQuery.From, recentQuery.To).Order("hour").Limit(MaxQueryRows + 1).Find(&recentRolls).Error; err != nil {
		return v, err
	}
	if len(recentRolls)+len(recent) > MaxQueryRows {
		return v, ErrQueryLimit
	}
	today := now.In(loc).Format("2006-01-02")
	var recentTotals Totals
	addRecent := func(at float64, totals Totals) error {
		if !safeAdd(recentTotals, totals) {
			return fmt.Errorf("AI recent totals exceed safe JSON integer range")
		}
		recentTotals.add(totals)
		v.Live.Calls24h += totals.Calls
		if time.Unix(int64(at), 0).In(loc).Format("2006-01-02") == today {
			v.Live.TodayTokens += totals.TotalTokens()
			addMoney(&v.Live.TodayAPIEquivalentCostUSD, totals.APIEquivalentCostUSD)
			v.Live.UnpricedCalls += totals.UnpricedCalls
		}
		return nil
	}
	for _, row := range recent {
		var totals Totals
		totals.addSample(row)
		if err := addRecent(row.EndedAt, totals); err != nil {
			return v, err
		}
	}
	// Match historical whole-bucket semantics; never interpolate retired deltas.
	for _, row := range recentRolls {
		if err := addRecent(row.Hour, row.Totals); err != nil {
			return v, err
		}
	}
	v.Live = computeLive(v.Live, recent, states, v.Nodes, now)
	return v, nil
}
func setShare(a *Aggregate, total Totals) {
	if total.UnpricedCalls == 0 && total.APIEquivalentCostUSD != nil && *total.APIEquivalentCostUSD > 0 && a.APIEquivalentCostUSD != nil {
		v := *a.APIEquivalentCostUSD / *total.APIEquivalentCostUSD
		a.CostShare = &v
	}
}
func sortAggregates(rows []Aggregate) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Calls == rows[j].Calls {
			return rows[i].Model+rows[i].Provider+rows[i].BillingMode+rows[i].Task < rows[j].Model+rows[j].Provider+rows[j].BillingMode+rows[j].Task
		}
		return rows[i].Calls > rows[j].Calls
	})
}
func (s *Store) states(node string) ([]NodeState, error) {
	var states []NodeState
	db := s.DB
	if node != "" {
		db = db.Where("node = ?", node)
	}
	err := db.Order("node").Limit(MaxOverviewNodes + 1).Find(&states).Error
	if len(states) > MaxOverviewNodes {
		return nil, ErrQueryLimit
	}
	return states, err
}
func (s *Store) nodes(states []NodeState, node string, now time.Time) ([]Node, error) {
	result := []Node{}
	if !s.DB.Migrator().HasTable(&models.Client{}) {
		return result, nil
	}
	var clients []models.Client
	db := s.DB.Select("uuid", "name")
	if node != "" {
		db = db.Where("uuid = ?", node)
	}
	if err := db.Order("uuid").Limit(MaxOverviewNodes + 1).Find(&clients).Error; err != nil {
		return nil, err
	}
	if len(clients) > MaxOverviewNodes {
		return nil, ErrQueryLimit
	}
	stateMap := map[string]NodeState{}
	for _, state := range states {
		stateMap[state.Node] = state
	}
	var profiles []models.ClientDeploymentProfile
	db = s.DB
	if node != "" {
		db = db.Where("client = ?", node)
	}
	if err := db.Limit(MaxOverviewNodes + 1).Find(&profiles).Error; err != nil {
		return nil, err
	}
	if len(profiles) > MaxOverviewNodes {
		return nil, ErrQueryLimit
	}
	profileMap := map[string]models.ClientDeploymentProfile{}
	for _, profile := range profiles {
		profileMap[profile.Client] = profile
	}
	for _, client := range clients {
		n := Node{UUID: client.UUID, Name: client.Name, Status: "disabled", DeliveryStatus: "unavailable"}
		profile, exists := profileMap[client.UUID]
		var config struct {
			Enabled bool `json:"enable_hermes_ai"`
		}
		if exists {
			if err := json.Unmarshal([]byte(profile.Config), &config); err != nil {
				return nil, err
			}
			n.Enabled = config.Enabled
			n.DeliveryStatus = profile.DeliveryStatus
			if n.DeliveryStatus == "" {
				n.DeliveryStatus = "saved"
			}
		}
		if n.Enabled {
			n.Status = "unavailable"
		}
		if state, ok := stateMap[client.UUID]; ok {
			n.CurrentModel = state.CurrentModel
			n.LastSuccessAt = state.LastSuccessAt
			at := state.LastReportAt
			n.LastReportAt = &at
			n.SamplingIntervalSeconds = state.SamplingIntervalSeconds
			if n.Enabled && state.Enabled {
				n.Status = state.Status
				if float64(now.Unix())-state.ObservedAt > freshness(state) {
					n.Status = "stale"
				}
			}
		}
		result = append(result, n)
	}
	return result, nil
}
func freshness(s NodeState) float64 {
	if s.SamplingIntervalSeconds*3 > 60 {
		return s.SamplingIntervalSeconds * 3
	}
	return 60
}
func (s *Store) accounts(states []NodeState, now time.Time) ([]Account, error) {
	result := []Account{}
	sources := map[string][]string{}
	for _, state := range states {
		if state.QuotaAccount != "" {
			key := state.QuotaProvider + "\x00" + state.QuotaAccount
			sources[key] = append(sources[key], state.Node)
		}
	}
	// The bounded current state set determines exactly which accounts are visible.
	for key, nodes := range sources {
		stateProvider, stateKey := "", ""
		for _, state := range states {
			if state.QuotaProvider+"\x00"+state.QuotaAccount == key {
				stateProvider = state.QuotaProvider
				stateKey = state.QuotaAccount
				break
			}
		}
		var row AccountQuota
		err := s.DB.First(&row, "provider = ? AND account_key = ?", stateProvider, stateKey).Error
		if err == gorm.ErrRecordNotFound {
			continue
		}
		if err != nil {
			return nil, err
		}
		var q Quota
		if err := json.Unmarshal([]byte(row.Payload), &q); err != nil {
			return nil, err
		}
		if float64(now.Unix())-q.ObservedAt > 1800 && q.Status == "available" {
			q.Status = "stale"
		}
		sort.Strings(nodes)
		result = append(result, Account{Quota: q, NodeSources: nodes, DisplayLabel: q.Provider + " / " + q.Plan + " / " + q.AccountKey})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Provider+result[i].AccountKey < result[j].Provider+result[j].AccountKey
	})
	return result, nil
}
func computeLive(live Live, rows []StoredSample, states []NodeState, nodes []Node, now time.Time) Live {
	end := float64(now.Unix())
	enabled := map[string]bool{}
	if len(nodes) > 0 {
		for _, node := range nodes {
			if node.Enabled {
				enabled[node.UUID] = true
			}
		}
	} else {
		for _, state := range states {
			if state.Enabled {
				enabled[state.Node] = true
			}
		}
	}
	if len(enabled) == 0 {
		return live
	}
	stateMap := map[string]NodeState{}
	for _, state := range states {
		stateMap[state.Node] = state
	}
	for node := range enabled {
		state, ok := stateMap[node]
		if !ok || !state.Enabled || state.Status != "collecting" || float64(now.Unix())-state.ObservedAt > freshness(state) {
			return live
		}
	}
	latest := map[string]float64{}
	for _, row := range rows {
		if enabled[row.Node] && row.EndedAt > latest[row.Node] {
			latest[row.Node] = row.EndedAt
		}
	}
	for node := range enabled {
		at := latest[node]
		if at == 0 || float64(now.Unix())-at > freshness(stateMap[node]) {
			return live
		}
		if at < end {
			end = at
		}
	}
	if total, ok := liveWindow(rows, enabled, end-60, end); ok {
		live.RollingCalls = total.Calls
		live.RollingTokens = total.TotalTokenCount
		live.RPM = floatPtrValue(float64(total.Calls))
		live.TPM = floatPtrValue(float64(total.TotalTokenCount))
		live.Status = "collecting"
	}
	if total, ok := liveWindow(rows, enabled, end-10, end); ok {
		live.TPS = floatPtrValue(float64(total.TotalTokenCount) / 10)
		live.InputTPS = floatPtrValue(float64(total.InputTokens) / 10)
		live.OutputTPS = floatPtrValue(float64(total.OutputTokens) / 10)
		live.CacheReadTPS = floatPtrValue(float64(total.CacheReadTokens) / 10)
		live.RPS = floatPtrValue(float64(total.Calls) / 10)
		live.Status = "collecting"
	}
	return live
}

// Prove each window separately without splitting sampled deltas at boundaries.
func liveWindow(rows []StoredSample, enabled map[string]bool, start, end float64) (Totals, bool) {
	byNode := map[string][]StoredSample{}
	var total Totals
	for _, row := range rows {
		if !enabled[row.Node] || row.EndedAt <= start || row.StartedAt >= end {
			continue
		}
		if row.EndedAt > end || row.StartedAt < start || row.Coverage == nil || !*row.Coverage || row.ResetCount > 0 {
			return Totals{}, false
		}
		byNode[row.Node] = append(byNode[row.Node], row)
		var incoming Totals
		incoming.addSample(row)
		if !safeAdd(total, incoming) {
			return Totals{}, false
		}
		total.add(incoming)
	}
	for node := range enabled {
		samples := byNode[node]
		sort.Slice(samples, func(i, j int) bool { return samples[i].StartedAt < samples[j].StartedAt })
		covered := start
		for _, row := range samples {
			if row.StartedAt > covered+0.001 {
				return Totals{}, false
			}
			if row.EndedAt > covered {
				covered = row.EndedAt
			}
		}
		if covered < end-0.001 {
			return Totals{}, false
		}
	}
	return total, true
}
func floatPtrValue(v float64) *float64 { return &v }
