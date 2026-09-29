package metric

import (
	"fmt"
	"math"
	"time"
)

// RollupTier describes one downsampled resolution: raw points (or the next
// finer tier) are aggregated into buckets Interval wide, and those buckets are
// kept for Retention. A policy lists tiers from finest to coarsest, e.g.
//
//	1m kept 7d  ->  5m kept 30d  ->  1h kept 1y
//
// As data ages it falls off the finer tiers and survives only in coarser ones,
// so storage shrinks with age — the "data gets sparser as it gets older"
// behavior of a downsampling TSDB.
//
// RollupTier describes a downsampling resolution: the original points (or the next finer level) will be aggregated into
// Interval wide buckets that retain Retention. Strategies are arranged in hierarchical order from fine to coarse, for example:
//
//	1m reserved 7d -> 5m reserved 30d -> 1h reserved 1y
//
// As data gets older, it expires from the finer levels and is only retained in the coarser levels, so the amount of storage increases.
// Data age decrease, which is the behavior of "older data becomes sparser" in downsampling TSDB.
type RollupTier struct {
	// Interval is the bucket width for this rollup tier.
	//
	// Interval is the barrel width of this rollup layer.
	Interval time.Duration `json:"interval"`
	// Retention is how long buckets in this tier are kept.
	//
	// Retention is the retention time of the bucket in this layer.
	Retention time.Duration `json:"retention"`
}

// RollupPolicy is the full retention ladder for a store: how long raw points
// live, then a chain of progressively coarser tiers. Compact materializes the
// tiers and enforces every retention window.
//
// RollupPolicy describes the complete retention ladder: how long the original point is retained, and how it becomes progressively thicker in the future.
// rollup layers; Compact materializes these layers and enforces retention windows.
type RollupPolicy struct {
	// RawRetention is how long raw points are kept before Compact deletes them
	// (after they have been rolled into the finest tier). Zero means "never
	// delete raw" — rollups are still built, but raw is retained.
	//
	// RawRetention is how long Compact retains raw points before deleting them (after they have entered the smallest
	// after the rollup layer). A value of zero means "never remove the original point"; the rollup will still build, but the original point will remain.
	RawRetention time.Duration `json:"raw_retention"`
	// Tiers are ordered finest-first. Each Interval must be a positive integer
	// multiple of the previous tier's Interval (so a coarse bucket is composed
	// of whole finer buckets), and each Retention must be >= the previous
	// tier's Retention (coarse data outlives fine data).
	//
	// Tiers are sorted from fine to coarse. Each Interval must be a positive integer multiple of the previous Interval
	// (In this way, the thick bucket is composed of a complete thin bucket), each Retention must >= the previous layer of Retention
	// (Coarse data lives longer than fine data).
	Tiers []RollupTier `json:"tiers"`
	// Compression tunes the per-bucket t-digest (size vs. percentile accuracy).
	// <=1 uses the default (100).
	//
	// Compression adjusts the t-digest (trade-off between size and percentile precision) of each bucket.
	// <=1 uses the default value (100).
	Compression float64 `json:"compression"`
}

// Enabled reports whether the policy actually defines any rollup tiers.
//
// Enabled indicates whether the policy actually defines a rollup hierarchy.
func (p RollupPolicy) Enabled() bool { return len(p.Tiers) > 0 }

// compression returns the effective t-digest compression for a rollup policy.
//
// compression returns the actual t-digest compression parameters used by the rollup policy.
func (p RollupPolicy) compression() float64 {
	if p.Compression <= 1 {
		return defaultTDigestCompression
	}
	return p.Compression
}

func (p RollupPolicy) rawCutoff(now time.Time) time.Time {
	if p.RawRetention <= 0 || len(p.Tiers) == 0 {
		return time.Time{}
	}
	cutoff := now.UTC().Add(-p.RawRetention)
	interval := p.Tiers[0].Interval.Nanoseconds()
	return time.Unix(0, floorDivNano(cutoff.UnixNano(), interval)).UTC()
}

// withMetricRetention applies the metric definition's retention to a rollup
// policy. Fine tiers keep their configured short windows, while the final tier
// follows the metric so definitions are never capped by a store-wide setting.
// Coarser tiers that do not extend retention are omitted because their buckets
// can be derived from the retained finer tier on demand.
func (p RollupPolicy) withMetricRetention(retention time.Duration) RollupPolicy {
	if retention <= 0 || len(p.Tiers) == 0 {
		return p
	}

	out := p
	out.Tiers = make([]RollupTier, 0, len(p.Tiers))
	for i, tier := range p.Tiers {
		if i == len(p.Tiers)-1 || tier.Retention > retention {
			tier.Retention = retention
		}
		if len(out.Tiers) > 0 && tier.Retention <= out.Tiers[len(out.Tiers)-1].Retention {
			continue
		}
		out.Tiers = append(out.Tiers, tier)
	}
	return out
}

// Validate enforces the structural rules that make cascading composition and
// retention well-defined.
//
// Validate checks whether the policy structure satisfies the constraints required for cascade synthesis and preservation semantics.
func (p RollupPolicy) Validate() error {
	if len(p.Tiers) == 0 {
		return nil // a store with no tiers simply does no rollup work
	}
	if p.RawRetention < 0 {
		return fmt.Errorf("%w: raw retention cannot be negative", ErrInvalidArgument)
	}
	var prev RollupTier
	for i, tr := range p.Tiers {
		if tr.Interval <= 0 {
			return fmt.Errorf("%w: tier %d interval must be positive", ErrInvalidArgument, i)
		}
		if tr.Retention <= 0 {
			return fmt.Errorf("%w: tier %d retention must be positive", ErrInvalidArgument, i)
		}
		if i == 0 {
			// The finest tier rolls up raw points. Require raw retention (when
			// set) to cover at least two of its buckets so the in-progress and
			// most-recent sealed bucket still have their raw backing when Compact
			// runs and deletes old raw.
			if p.RawRetention > 0 && p.RawRetention < 2*tr.Interval {
				return fmt.Errorf("%w: raw retention must be >= 2x the finest tier interval", ErrInvalidArgument)
			}
		} else {
			if tr.Interval <= prev.Interval {
				return fmt.Errorf("%w: tier %d interval must be larger than tier %d", ErrInvalidArgument, i, i-1)
			}
			if tr.Interval%prev.Interval != 0 {
				return fmt.Errorf("%w: tier %d interval must be a multiple of tier %d interval", ErrInvalidArgument, i, i-1)
			}
			if tr.Retention < prev.Retention {
				return fmt.Errorf("%w: tier %d retention must be >= tier %d retention", ErrInvalidArgument, i, i-1)
			}
		}
		prev = tr
	}
	return nil
}

// rollupBucket is the in-memory accumulator for one (metric, entity,
// resolution, bucket) cell. It carries exactly the summaries that can be
// re-aggregated losslessly when composing coarser tiers — count/sum/sumSq for
// avg & population stddev, min/max, first/last by timestamp — plus a t-digest
// so arbitrary percentiles survive downsampling with bounded error.
//
// rollupBucket is an in-memory accumulator of a single (metric, entity, resolution, bucket) unit. it is only carried in
// Summaries that can be losslessly reaggregated when synthesizing coarser levels: count/sum/sumSq for mean and population standard deviation,
// min/max, first/last recorded by time, plus a t-digest, so that any percentile can be
// Bounded errors are preserved after downsampling.
type rollupBucket struct {
	// count is the total number of raw points represented.
	//
	// count is the total number of raw points represented by this bucket.
	count int64
	// lossCount is populated only for the merged SQLite ping latency series.
	// The total sample count remains in count, so packet-loss ratios and latency
	// statistics can both be reconstructed exactly from one physical series.
	lossCount int64
	// sum is the sum of represented values.
	//
	// sum is the sum of the values represented by this bucket.
	sum float64
	// sumSq is the sum of squared values for population stddev.
	//
	// sumSq is the sum of squares used for the population standard deviation.
	sumSq float64
	// min is the minimum represented value.
	//
	// min is the minimum value represented by this bucket.
	min float64
	// max is the maximum represented value.
	//
	// max is the maximum value represented by this bucket.
	max float64
	// firstVal is the value with the earliest timestamp.
	//
	// firstVal is the earliest value of the timestamp.
	firstVal float64
	// firstTS is the earliest timestamp in nanoseconds.
	//
	// firstTS is the nanosecond value of the earliest timestamp.
	firstTS int64
	// lastVal is the value with the latest timestamp.
	//
	// lastVal is the latest value of the timestamp.
	lastVal float64
	// lastTS is the latest timestamp in nanoseconds.
	//
	// lastTS is the nanosecond value of the latest timestamp.
	lastTS int64
	// digest estimates percentiles for represented values.
	//
	// digest is used to estimate the percentile of values represented by this bucket.
	digest *TDigest
	// tagsHash carries the stable tag-set fingerprint for this rollup cell.
	//
	// tagsHash carries the fingerprint of the stable tag set for this rollup unit.
	tagsHash string
	// tagsJSON carries the canonical tag map written back to the tags column.
	//
	// tagsJSON carries the canonical tag map written back to the tags column.
	tagsJSON string
}

// newRollupBucket creates an empty rollup accumulator.
//
// newRollupBucket creates an empty rollup accumulator.
func newRollupBucket(compression float64) *rollupBucket {
	return newRollupBucketWithDigest(compression, true)
}

// newRollupBucketWithDigest creates a query or compaction accumulator. Query
// paths only need the digest for percentile aggregations.
func newRollupBucketWithDigest(compression float64, includeDigest bool) *rollupBucket {
	bucket := &rollupBucket{
		min:     0,
		max:     0,
		firstTS: 0,
		lastTS:  0,
	}
	if includeDigest {
		bucket.digest = NewTDigest(compression)
	}
	return bucket
}

// rollupDigestOptional reports the one lossless case where a rollup does not
// need a percentile digest: a merged ping-latency bucket containing only loss
// sentinels. The decision is based on the stored sample counts, not on an
// assumed probe interval, so 1-second, 5-second, and custom schedules behave
// identically.
func rollupDigestOptional(metricName string, bucket *rollupBucket) bool {
	return metricName == sqliteMergedPingLatencyMetric && bucket != nil &&
		bucket.count > 0 && bucket.lossCount == bucket.count
}

// addPoint folds a raw observation into the bucket.
//
// addPoint adds a raw observation into the current bucket.
func (b *rollupBucket) addPoint(value float64, tsNano int64) {
	if b.count == 0 {
		b.min, b.max = value, value
		b.firstVal, b.firstTS = value, tsNano
		b.lastVal, b.lastTS = value, tsNano
	} else {
		if value < b.min {
			b.min = value
		}
		if value > b.max {
			b.max = value
		}
		if tsNano < b.firstTS {
			b.firstVal, b.firstTS = value, tsNano
		}
		if tsNano > b.lastTS {
			b.lastVal, b.lastTS = value, tsNano
		}
	}
	b.count++
	b.sum += value
	b.sumSq += value * value
	if b.digest != nil {
		b.digest.Add(value, 1)
	}
}

func (b *rollupBucket) addMetricPoint(metricName string, value float64, tsNano int64) {
	if metricName != sqliteMergedPingLatencyMetric {
		b.addPoint(value, tsNano)
		return
	}

	validBefore := b.count - b.lossCount
	if b.count == 0 || tsNano < b.firstTS {
		b.firstVal, b.firstTS = value, tsNano
	}
	if b.count == 0 || tsNano > b.lastTS {
		b.lastVal, b.lastTS = value, tsNano
	}
	b.count++
	if value < 0 {
		// Negative ping latency is the long-standing loss sentinel. Keep it in
		// first/last for API compatibility, but exclude it from latency summaries.
		b.lossCount++
		return
	}
	if validBefore == 0 {
		b.min, b.max = value, value
	} else {
		if value < b.min {
			b.min = value
		}
		if value > b.max {
			b.max = value
		}
	}
	b.sum += value
	b.sumSq += value * value
	if b.digest != nil {
		b.digest.Add(value, 1)
	}
}

// mergeStored folds a finer rollup row (already-summarized) into this coarser
// bucket. This is the cascade step: tier i+1 buckets are built by merging the
// tier i rows they span.
//
// mergeStored merges a summarized finer rollup row into the current coarser bucket. Here are the cascading steps:
// Buckets for tier i+1 are constructed by merging the tier i rows they cover.
func (b *rollupBucket) mergeStored(o *rollupBucket) {
	if o.count == 0 {
		return
	}
	bValid := b.count - b.lossCount
	oValid := o.count - o.lossCount
	if bValid == 0 && oValid > 0 {
		b.min, b.max = o.min, o.max
	} else if oValid > 0 {
		if o.min < b.min {
			b.min = o.min
		}
		if o.max > b.max {
			b.max = o.max
		}
	}
	if b.count == 0 {
		b.firstVal, b.firstTS = o.firstVal, o.firstTS
		b.lastVal, b.lastTS = o.lastVal, o.lastTS
	} else {
		if o.firstTS < b.firstTS {
			b.firstVal, b.firstTS = o.firstVal, o.firstTS
		}
		if o.lastTS > b.lastTS {
			b.lastVal, b.lastTS = o.lastVal, o.lastTS
		}
	}
	b.count += o.count
	b.lossCount += o.lossCount
	b.sum += o.sum
	b.sumSq += o.sumSq
	if o.digest != nil {
		if b.digest == nil {
			b.digest = NewTDigest(defaultTDigestCompression)
		}
		b.digest.Merge(o.digest)
	}
}

// value computes the requested aggregation from the bucket summaries. ok=false
// means the aggregation is not derivable from a rollup (only AggRate, which
// needs the ordered raw series).
//
// value Computes the requested aggregate value from the bucket digest; ok=false means that the aggregate cannot be derived by rollup
// (Currently it is mainly AggRate that requires an ordered primitive sequence).
func (b *rollupBucket) value(agg Aggregation) (float64, bool) {
	validCount := b.count - b.lossCount
	switch agg {
	case AggAvg:
		if validCount == 0 {
			return 0, true
		}
		return b.sum / float64(validCount), true
	case AggMin:
		if validCount == 0 {
			return 0, true
		}
		return b.min, true
	case AggMax:
		if validCount == 0 {
			return 0, true
		}
		return b.max, true
	case AggSum:
		return b.sum, true
	case AggCount:
		return float64(b.count), true
	case AggFirst:
		return b.firstVal, true
	case AggLast:
		return b.lastVal, true
	case AggStdDev:
		if validCount == 0 {
			return 0, true
		}
		mean := b.sum / float64(validCount)
		variance := b.sumSq/float64(validCount) - mean*mean
		if variance < 0 {
			variance = 0 // floating-point guard
		}
		return math.Sqrt(variance), true
	case AggRate:
		return 0, false // rate needs the ordered raw series; not in a rollup
	case aggPingLossAvg:
		if b.count == 0 {
			return 0, true
		}
		return float64(b.lossCount) / float64(b.count), true
	case aggPingLossSum:
		return float64(b.lossCount), true
	case aggPingLossMin:
		if b.count > 0 && b.lossCount == b.count {
			return 1, true
		}
		return 0, true
	case aggPingLossMax:
		if b.lossCount > 0 {
			return 1, true
		}
		return 0, true
	case aggPingLossFirst:
		if b.count > 0 && b.firstVal < 0 {
			return 1, true
		}
		return 0, true
	case aggPingLossLast:
		if b.count > 0 && b.lastVal < 0 {
			return 1, true
		}
		return 0, true
	case aggPingLossStdDev:
		if b.count == 0 {
			return 0, true
		}
		p := float64(b.lossCount) / float64(b.count)
		return math.Sqrt(p * (1 - p)), true
	default:
		if frac, ok := parsePingLossPercentile(agg); ok {
			return pingLossPercentile(b.count, b.lossCount, frac), true
		}
		if frac, ok := parsePercentile(agg); ok {
			if b.digest == nil || b.digest.Count() == 0 {
				return 0, true
			}
			return b.digest.Quantile(frac), true
		}
		return 0, false
	}
}
