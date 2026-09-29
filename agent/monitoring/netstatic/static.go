package netstatic

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nuomiiiii/lite-agent/utils"
	gnet "github.com/shirou/gopsutil/v4/net"
)

/*
Track traffic for each interface. Keep DataPreserveDay days of samples, collected every DetectInterval seconds.

By default, save to net_static.json in the current directory.
The config field in net_static.json stores current settings; use defaults if absent.
Unix timestamps are in seconds.

Keep operations in memory where possible to avoid frequent I/O.

Read and write files only on startup, shutdown, or save.
*/
var (
	DefaultDataPreserveDay = 31.0      // Days of samples retained; older data is removed.
	DefaultDetectInterval  = 2.0       // Collection interval in seconds.
	DefaultSaveInterval    = 60.0 * 10 // Save interval in seconds; save at this interval rather than DetectInterval to limit disk I/O.
	SaveFilePath           = "./net_static.json"
)

var (
	staticCache map[string][]TrafficData // Unpersisted samples keyed by interface; collect at DetectInterval, merge tx/rx at SaveInterval, then clear cache.
	config      NetStaticConfig
)

// NetStatic holds network interface traffic statistics.
type NetStatic struct {
	Interfaces map[string][]TrafficData `json:"interfaces"` // key: interface name
	Config     NetStaticConfig          `json:"config"`
}

type NetStaticConfig struct {
	DataPreserveDay float64  `json:"data_preserve_day"` // Days of samples retained; older data is removed.
	DetectInterval  float64  `json:"detect_interval"`   // Collection interval in seconds.
	SaveInterval    float64  `json:"save_interval"`     // Save interval in seconds to limit disk I/O.
	Nics            []string `json:"nics"`              // Only monitor named interfaces; empty means all interfaces.
}

type TrafficData struct {
	Timestamp uint64 `json:"timestamp"`
	Tx        uint64 `json:"tx"` // Difference between samples n and n-1.
	Rx        uint64 `json:"rx"` // Difference between samples n and n-1.
}

var (
	mu           sync.RWMutex
	running      bool
	detectTicker *time.Ticker
	saveTicker   *time.Ticker
	stopCh       chan struct{}

	// In-memory persisted state (mirrors the file; disk access only at startup, save, or stop).
	store NetStatic

	// Previous cumulative byte counters (for delta calculation).
	lastCounters = map[string]struct{ Tx, Rx uint64 }{}

	resetClock atomic.Value // resetClockState
)

type resetClockState struct {
	Day      int
	Clock    string
	Timezone string
}

// SetResetClock records the monthly reset instant used when flushing samples.
// Day 0 disables split-on-boundary.
func SetResetClock(day int, clock, timezone string) {
	resetClock.Store(resetClockState{Day: day, Clock: clock, Timezone: timezone})
}

func currentResetClock() resetClockState {
	value := resetClock.Load()
	if value == nil {
		return resetClockState{}
	}
	state, _ := value.(resetClockState)
	return state
}

func nowUnix() uint64 { return uint64(time.Now().Unix()) }

// isNicAllowed checks the interface allowlist; an empty or nil allowlist permits all interfaces.
func isNicAllowed(name string) bool {
	if len(config.Nics) == 0 {
		return true
	}
	for _, n := range config.Nics {
		if n == name {
			return true
		}
	}
	return false
}

func ensureInitLocked() {
	if store.Interfaces == nil {
		store.Interfaces = make(map[string][]TrafficData)
	}
	if staticCache == nil {
		staticCache = make(map[string][]TrafficData)
	}
	if config.DataPreserveDay == 0 {
		config.DataPreserveDay = DefaultDataPreserveDay
	}
	if config.DetectInterval == 0 {
		config.DetectInterval = DefaultDetectInterval
	}
	if config.SaveInterval == 0 {
		config.SaveInterval = DefaultSaveInterval
	}
}

func loadFromFileLocked() error {
	// Use defaults when no file exists.
	f, err := os.Open(SaveFilePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			ensureInitLocked()
			store.Config = configOrDefault(config)
			return nil
		}
		return err
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		ensureInitLocked()
		store.Config = configOrDefault(config)
		return nil
	}
	var ns NetStatic
	if err := json.Unmarshal(data, &ns); err != nil {
		// If the file is corrupt, back it up and use defaults without blocking startup.
		_ = os.Rename(SaveFilePath, SaveFilePath+".bak")
		ensureInitLocked()
		store.Config = configOrDefault(config)
		return nil
	}
	store = ns
	config = configOrDefault(ns.Config)
	ensureInitLocked()
	// Remove expired data on startup.
	purgeExpiredLocked()
	return nil
}

func saveToFileLocked() error {
	// Ensure the directory exists.
	if err := os.MkdirAll(filepath.Dir(SaveFilePath), 0o755); err != nil {
		return err
	}
	// Include the current config when writing.
	store.Config = configOrDefault(config)
	b, err := json.Marshal(store) // Compact JSON (no indentation).
	if err != nil {
		return err
	}
	tmp := SaveFilePath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, SaveFilePath)
}

func configOrDefault(c NetStaticConfig) NetStaticConfig {
	if c.DataPreserveDay == 0 {
		c.DataPreserveDay = DefaultDataPreserveDay
	}
	if c.DetectInterval == 0 {
		c.DetectInterval = DefaultDetectInterval
	}
	if c.SaveInterval == 0 {
		c.SaveInterval = DefaultSaveInterval
	}
	return c
}

func purgeExpiredLocked() {
	// Remove data older than DataPreserveDay.
	ttl := time.Duration(config.DataPreserveDay * 24 * float64(time.Hour))
	cutoff := uint64(time.Now().Add(-ttl).Unix())
	for name, arr := range store.Interfaces {
		// Keep samples at or after the cutoff.
		kept := arr[:0]
		for _, td := range arr {
			if td.Timestamp >= cutoff {
				kept = append(kept, td)
			}
		}
		if len(kept) == 0 {
			delete(store.Interfaces, name)
		} else {
			store.Interfaces[name] = kept
		}
	}
}

func safeDelta(cur, prev uint64) uint64 {
	if cur >= prev {
		return cur - prev
	}
	// Treat counter rollover or reset as a zero increment.
	return 0
}

func sampleOnceLocked() {
	ios, err := gnet.IOCounters(true)
	if err != nil {
		return
	}
	ts := nowUnix()
	for _, io := range ios {
		name := io.Name
		// Only monitor selected interfaces when Nics is configured.
		if !isNicAllowed(name) {
			continue
		}
		curTx := io.BytesSent
		curRx := io.BytesRecv
		prev, ok := lastCounters[name]
		if ok {
			dtx := safeDelta(curTx, prev.Tx)
			drx := safeDelta(curRx, prev.Rx)
			// Do not record the first sample.
			if dtx > 0 || drx > 0 {
				staticCache[name] = append(staticCache[name], TrafficData{Timestamp: ts, Tx: dtx, Rx: drx})
			} else {
				// Ignore zero increments to reduce noise and storage.
			}
		}
		lastCounters[name] = struct{ Tx, Rx uint64 }{Tx: curTx, Rx: curRx}
	}
}

func flushCacheLocked(ts uint64) {
	if len(staticCache) == 0 {
		return
	}
	boundary := resetBoundaryUnix(ts)
	for name, arr := range staticCache {
		for _, rec := range aggregateByReset(arr, ts, boundary) {
			store.Interfaces[name] = append(store.Interfaces[name], rec)
		}
	}
	// Clear the cache.
	staticCache = make(map[string][]TrafficData)
}

func resetBoundaryUnix(flushTs uint64) uint64 {
	clock := currentResetClock()
	if clock.Day < 1 || clock.Day > 31 || flushTs == 0 {
		return 0
	}
	instant := utils.GetLastResetInstant(clock.Day, clock.Clock, clock.Timezone, time.Unix(int64(flushTs), 0).UTC())
	if instant.IsZero() {
		return 0
	}
	unix := instant.Unix()
	if unix <= 0 {
		return 0
	}
	return uint64(unix)
}

func aggregateByReset(arr []TrafficData, flushTs, boundary uint64) []TrafficData {
	if len(arr) == 0 {
		return nil
	}
	if boundary == 0 {
		return sumTraffic(arr, flushTs)
	}
	var before, after TrafficData
	for _, td := range arr {
		if td.Timestamp < boundary {
			before.Tx += td.Tx
			before.Rx += td.Rx
		} else {
			after.Tx += td.Tx
			after.Rx += td.Rx
		}
	}
	var out []TrafficData
	if before.Tx > 0 || before.Rx > 0 {
		ts := flushTs
		if ts >= boundary {
			ts = boundary - 1
		}
		before.Timestamp = ts
		out = append(out, before)
	}
	if after.Tx > 0 || after.Rx > 0 {
		ts := flushTs
		if ts < boundary {
			ts = boundary
		}
		after.Timestamp = ts
		out = append(out, after)
	}
	return out
}

func sumTraffic(arr []TrafficData, ts uint64) []TrafficData {
	var sumTx, sumRx uint64
	for _, td := range arr {
		sumTx += td.Tx
		sumRx += td.Rx
	}
	if sumTx == 0 && sumRx == 0 {
		return nil
	}
	return []TrafficData{{Timestamp: ts, Tx: sumTx, Rx: sumRx}}
}

// startGoroutinesLocked starts collection and save goroutines (the lock must be held).
func startGoroutinesLocked() {
	// Collection goroutine.
	go func() {
		for {
			select {
			case <-detectTicker.C:
				mu.Lock()
				sampleOnceLocked()
				mu.Unlock()
			case <-stopCh:
				return
			}
		}
	}()

	// Save goroutine.
	go func() {
		for {
			select {
			case t := <-saveTicker.C:
				mu.Lock()
				flushCacheLocked(uint64(t.Unix()))
				purgeExpiredLocked()
				_ = saveToFileLocked()
				mu.Unlock()
			case <-stopCh:
				return
			}
		}
	}()
}

// GetNetStatic returns all current traffic statistics.
func GetNetStatic() (*NetStatic, error) {
	mu.RLock()
	defer mu.RUnlock()
	ensureInitLocked()
	// Combine store and cache without folding cache into a single point; return a temporary view.
	merged := NetStatic{Interfaces: map[string][]TrafficData{}, Config: configOrDefault(config)}
	for name, arr := range store.Interfaces {
		cp := make([]TrafficData, len(arr))
		copy(cp, arr)
		merged.Interfaces[name] = cp
	}
	for name, arr := range staticCache {
		merged.Interfaces[name] = append(merged.Interfaces[name], arr...)
	}
	return &merged, nil
}

// StartOrContinue starts or resumes traffic collection.
func StartOrContinue() error {
	mu.Lock()
	defer mu.Unlock()
	if running {
		return nil
	}
	ensureInitLocked()
	// Read history.
	if err := loadFromFileLocked(); err != nil {
		return err
	}
	// Start tickers.
	detectTicker = time.NewTicker(time.Duration(config.DetectInterval * float64(time.Second)))
	saveTicker = time.NewTicker(time.Duration(config.SaveInterval * float64(time.Second)))
	stopCh = make(chan struct{})
	running = true

	// Start goroutines.
	startGoroutinesLocked()
	return nil
}

// Clear removes all traffic statistics.
func Clear() error {
	mu.Lock()
	defer mu.Unlock()
	ensureInitLocked()
	store.Interfaces = make(map[string][]TrafficData)
	staticCache = make(map[string][]TrafficData)
	lastCounters = map[string]struct{ Tx, Rx uint64 }{}
	// Do not save immediately; save at the next interval or on stop.
	return nil
}

// Stop stops traffic collection.
func Stop() error {
	mu.Lock()
	if !running {
		mu.Unlock()
		return nil
	}
	running = false
	if detectTicker != nil {
		detectTicker.Stop()
	}
	if saveTicker != nil {
		saveTicker.Stop()
	}
	close(stopCh)
	// Perform the final flush and save.
	flushCacheLocked(nowUnix())
	purgeExpiredLocked()
	err := saveToFileLocked()
	mu.Unlock()
	return err
}

// GetNetStaticBetween returns traffic samples in a time range (Unix timestamps).
func GetNetStaticBetween(start, end uint64) (*NetStatic, error) {
	mu.RLock()
	defer mu.RUnlock()
	ensureInitLocked()
	res := NetStatic{Interfaces: map[string][]TrafficData{}, Config: configOrDefault(config)}
	inRange := func(ts uint64) bool { return (start == 0 || ts >= start) && (end == 0 || ts <= end) }
	for name, arr := range store.Interfaces {
		var filtered []TrafficData
		for _, td := range arr {
			if inRange(td.Timestamp) {
				filtered = append(filtered, td)
			}
		}
		if len(filtered) > 0 {
			res.Interfaces[name] = filtered
		}
	}
	// Combine the cache.
	for name, arr := range staticCache {
		for _, td := range arr {
			if inRange(td.Timestamp) {
				res.Interfaces[name] = append(res.Interfaces[name], td)
			}
		}
	}
	return &res, nil
}

// GetTotalTraffic returns total traffic by interface (key: name, value: total traffic).
func GetTotalTraffic() (map[string]TrafficData, error) {
	mu.RLock()
	defer mu.RUnlock()
	ensureInitLocked()
	res := map[string]TrafficData{}
	add := func(name string, tx, rx uint64) {
		cur := res[name]
		cur.Tx += tx
		cur.Rx += rx
		res[name] = cur
	}
	for name, arr := range store.Interfaces {
		var tx, rx uint64
		for _, td := range arr {
			tx += td.Tx
			rx += td.Rx
		}
		add(name, tx, rx)
	}
	for name, arr := range staticCache {
		var tx, rx uint64
		for _, td := range arr {
			tx += td.Tx
			rx += td.Rx
		}
		add(name, tx, rx)
	}
	return res, nil
}

// GetTotalTrafficBetween returns traffic totals by interface in a time range (Unix timestamps).
func GetTotalTrafficBetween(start, end uint64) (map[string]TrafficData, error) {
	mu.RLock()
	defer mu.RUnlock()
	ensureInitLocked()
	res := map[string]TrafficData{}
	inRange := func(ts uint64) bool { return (start == 0 || ts >= start) && (end == 0 || ts <= end) }
	add := func(name string, tx, rx uint64) {
		cur := res[name]
		cur.Tx += tx
		cur.Rx += rx
		res[name] = cur
	}
	for name, arr := range store.Interfaces {
		var tx, rx uint64
		for _, td := range arr {
			if inRange(td.Timestamp) {
				tx += td.Tx
				rx += td.Rx
			}
		}
		if tx > 0 || rx > 0 {
			add(name, tx, rx)
		}
	}
	for name, arr := range staticCache {
		var tx, rx uint64
		for _, td := range arr {
			if inRange(td.Timestamp) {
				tx += td.Tx
				rx += td.Rx
			}
		}
		if tx > 0 || rx > 0 {
			add(name, tx, rx)
		}
	}
	return res, nil
}

// SetNewConfig updates the config; zero-valued fields leave the current value unchanged.
func SetNewConfig(newCfg NetStaticConfig) error {
	mu.Lock()
	defer mu.Unlock()
	ensureInitLocked()
	// Merge new settings.
	if newCfg.DataPreserveDay != 0 {
		store.Config.DataPreserveDay = newCfg.DataPreserveDay
	}
	if newCfg.DetectInterval != 0 {
		store.Config.DetectInterval = newCfg.DetectInterval
	}
	if newCfg.SaveInterval != 0 {
		store.Config.SaveInterval = newCfg.SaveInterval
	}
	// Nil Nics means unchanged; a non-nil empty slice means monitor every interface.
	if newCfg.Nics != nil {
		// Copy the slice to prevent external modifications from affecting internal settings.
		tmp := make([]string, len(newCfg.Nics))
		copy(tmp, newCfg.Nics)
		store.Config.Nics = tmp
	}
	// Apply the effective config.
	cfg := configOrDefault(store.Config)
	store.Config = cfg
	config = cfg
	// Reconfigure tickers if running.
	if running {
		// Stop old tickers and goroutines first.
		if detectTicker != nil {
			detectTicker.Stop()
		}
		if saveTicker != nil {
			saveTicker.Stop()
		}
		close(stopCh)

		// Recreate tickers and channels.
		detectTicker = time.NewTicker(time.Duration(cfg.DetectInterval * float64(time.Second)))
		saveTicker = time.NewTicker(time.Duration(cfg.SaveInterval * float64(time.Second)))
		stopCh = make(chan struct{})

		// Restart goroutines.
		startGoroutinesLocked()

		// With a configured allowlist, remove excluded interfaces from cache and previous counters.
		if len(cfg.Nics) > 0 {
			allowed := make(map[string]struct{}, len(cfg.Nics))
			for _, n := range cfg.Nics {
				allowed[n] = struct{}{}
			}
			for name := range lastCounters {
				if _, ok := allowed[name]; !ok {
					delete(lastCounters, name)
				}
			}
			for name := range staticCache {
				if _, ok := allowed[name]; !ok {
					delete(staticCache, name)
				}
			}
		}
	}
	// Write to disk immediately.
	_ = saveToFileLocked()
	// Remove expired data as well.
	purgeExpiredLocked()
	return nil
}

func ForceReplaceRecord(rec map[string][]TrafficData) error {
	mu.Lock()
	defer mu.Unlock()
	ensureInitLocked()
	store.Interfaces = rec
	// Defer writing until the next periodic save or stop.
	// Remove expired data as well.
	purgeExpiredLocked()
	return nil
}
