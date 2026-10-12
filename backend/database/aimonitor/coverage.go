package aimonitor

import "sort"

// coveredAverage proves the entire requested interval for every participating
// node. It cannot infer absent intervals, split cross-boundary counters or use
// hourly rollups whose detailed coverage proof has been retired.
func coveredAverage(rows []StoredSample, from, to float64) *float64 {
	return coveredAverageBy(rows, from, to, func(row StoredSample) int64 { return row.TotalTokens() })
}

// observedOutputAverage measures model output throughput only across the
// collector's proven observation windows. Unlike a whole-range average, idle
// periods between samples are not treated as zero-token generation time.
func observedOutputAverage(rows []StoredSample, from, to float64) *float64 {
	if len(rows) == 0 || to <= from {
		return nil
	}
	tokens := int64(0)
	intervals := make([]StoredSample, 0, len(rows))
	for _, row := range rows {
		if row.StartedAt < from || row.EndedAt > to || row.EndedAt <= row.StartedAt || row.Coverage == nil || !*row.Coverage || row.ResetCount > 0 || row.OutputTokens < 0 || tokens > MaxSafeJSONInteger-row.OutputTokens {
			return nil
		}
		tokens += row.OutputTokens
		intervals = append(intervals, row)
	}
	sort.Slice(intervals, func(i, j int) bool {
		if intervals[i].StartedAt == intervals[j].StartedAt {
			return intervals[i].EndedAt < intervals[j].EndedAt
		}
		return intervals[i].StartedAt < intervals[j].StartedAt
	})
	observed := 0.0
	start, end := intervals[0].StartedAt, intervals[0].EndedAt
	for _, row := range intervals[1:] {
		if row.StartedAt > end {
			observed += end - start
			start, end = row.StartedAt, row.EndedAt
			continue
		}
		if row.EndedAt > end {
			end = row.EndedAt
		}
	}
	observed += end - start
	if observed <= 0 {
		return nil
	}
	return floatPtrValue(float64(tokens) / observed)
}

func coveredAverageBy(rows []StoredSample, from, to float64, value func(StoredSample) int64) *float64 {
	if len(rows) == 0 || to <= from {
		return nil
	}
	nodes := map[string][]StoredSample{}
	tokens := int64(0)
	for _, row := range rows {
		if row.StartedAt < from || row.EndedAt > to || row.Coverage == nil || !*row.Coverage || row.ResetCount > 0 {
			return nil
		}
		if value(row) < 0 || tokens > MaxSafeJSONInteger-value(row) {
			return nil
		}
		tokens += value(row)
		nodes[row.Node] = append(nodes[row.Node], row)
	}
	for _, samples := range nodes {
		sort.Slice(samples, func(i, j int) bool { return samples[i].StartedAt < samples[j].StartedAt })
		covered := from
		for _, row := range samples {
			if row.StartedAt > covered+0.001 {
				return nil
			}
			if row.EndedAt > covered {
				covered = row.EndedAt
			}
		}
		if covered < to-0.001 {
			return nil
		}
	}
	return floatPtrValue(float64(tokens) / (to - from))
}
