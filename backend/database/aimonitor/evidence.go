package aimonitor

import "time"

// Reuse bounded per-bucket interval proofs, not the range-wide status. A covered
// day refers only to the selected portion of that day. No additional DB scans.
func applyDailyCoverage(days []Day, trends []Trend, q Query, step float64, loc *time.Location) {
	for i := range days {
		days[i].Coverage = "unknown"
		date, err := time.ParseInLocation("2006-01-02", days[i].Date, loc)
		if err != nil {
			continue
		}
		start, end := max(q.From, float64(date.Unix())), min(q.To, float64(date.AddDate(0, 0, 1).Unix()))
		if end <= start {
			continue
		}
		cursor := start
		covered := true
		seen := false
		for _, point := range trends {
			a, b := max(start, point.Timestamp), min(end, point.Timestamp+step)
			if b <= a {
				continue
			}
			seen = true
			if a > cursor+0.001 || point.Coverage != "covered" {
				covered = false
			}
			cursor = max(cursor, b)
		}
		if seen {
			days[i].Coverage = "partial"
			if covered && cursor >= end {
				days[i].Coverage = "covered"
			}
		}
	}
}
