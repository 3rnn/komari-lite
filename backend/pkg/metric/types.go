package metric

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Driver names a supported database backend.
//
// Driver represents the database backend type used by the metric store.
type Driver string

const (
	// DriverSQLite selects the SQLite backend.
	//
	// DriverSQLite selects the SQLite backend.
	DriverSQLite Driver = "sqlite"
	// DriverMySQL selects the MySQL backend.
	//
	// DriverMySQL selects the MySQL backend.
	DriverMySQL Driver = "mysql"
	// DriverPostgreSQL selects the PostgreSQL backend.
	//
	// DriverPostgreSQL selects the PostgreSQL backend.
	DriverPostgreSQL Driver = "postgresql"
)

// MetricType describes the semantic type of a metric.
//
// MetricType represents the semantic type of the metric.
type MetricType string

const (
	// TypeGauge represents a point-in-time value.
	//
	// TypeGauge represents the value at a certain moment.
	TypeGauge MetricType = "gauge"
	// TypeCounter represents a monotonically increasing counter.
	//
	// TypeCounter represents a monotonically increasing counter.
	TypeCounter MetricType = "counter"
	// TypeHistogram represents histogram-style measurements.
	//
	// TypeHistogram represents a histogram type metric.
	TypeHistogram MetricType = "histogram"
	// TypeSummary represents pre-summarized measurements.
	//
	// TypeSummary represents a presummarized class measure.
	TypeSummary MetricType = "summary"
)

// Aggregation names a supported aggregation operation.
//
// Aggregation represents the aggregation method, such as avg, p95, or rate.
type Aggregation string

const (
	// AggAvg computes the arithmetic mean.
	//
	// AggAvg calculates the arithmetic mean.
	AggAvg Aggregation = "avg"
	// AggMin computes the minimum value.
	//
	// AggMin calculates the minimum value.
	AggMin Aggregation = "min"
	// AggMax computes the maximum value.
	//
	// AggMax calculates the maximum value.
	AggMax Aggregation = "max"
	// AggSum computes the sum of values.
	//
	// AggSum calculates the sum of values.
	AggSum Aggregation = "sum"
	// AggCount counts the number of points.
	//
	// AggCount counts the number of points.
	AggCount Aggregation = "count"
	// AggP50 computes the 50th percentile.
	//
	// AggP50 calculates the 50th percentile.
	AggP50 Aggregation = "p50"
	// AggP95 computes the 95th percentile.
	//
	// AggP95 calculates the 95th percentile.
	AggP95 Aggregation = "p95"
	// AggP99 computes the 99th percentile.
	//
	// AggP99 calculates the 99th percentile.
	AggP99 Aggregation = "p99"
	// AggFirst returns the first value in time order.
	//
	// AggFirst returns the first value in chronological order.
	AggFirst Aggregation = "first"
	// AggLast returns the last value in time order.
	//
	// AggLast returns the chronologically last value.
	AggLast Aggregation = "last"
	// AggRate computes the reset-aware per-second rate.
	//
	// AggRate calculates the rate per second at which resets can be processed.
	AggRate Aggregation = "rate"
	// AggStdDev computes the population standard deviation.
	//
	// AggStdDev calculates the population standard deviation.
	AggStdDev Aggregation = "stddev"
)

// Order controls chronological query ordering.
//
// Order indicates that the query results are arranged in ascending or descending order by time.
type Order string

const (
	// OrderAsc orders points from oldest to newest.
	//
	// OrderAsc sorts the points in order from oldest to newest.
	OrderAsc Order = "asc"
	// OrderDesc orders points from newest to oldest.
	//
	// OrderDesc Arranges points in order from newest to oldest.
	OrderDesc Order = "desc"
)

// Definition describes a metric and its metadata.
//
// Definition describes the metadata and retention policy of a metric.
type Definition struct {
	// Name is the unique metric name.
	//
	// Name is the unique metric name.
	Name string `json:"name"`
	// Description is optional human-readable metric text.
	//
	// Description is an optional human-readable metric description.
	Description string `json:"description,omitempty"`
	// Type describes the metric's semantic type.
	//
	// Type describes the semantic type of the metric.
	Type MetricType `json:"type"`
	// Unit names the value unit, such as bytes or percent.
	//
	// Unit represents a numeric unit, such as bytes or percent.
	Unit string `json:"unit,omitempty"`
	// RetentionDays controls historical data retention for this metric. A value
	// of zero disables persistence and removes existing metric data.
	//
	// RetentionDays controls the number of days to retain historical data for this metric; zero means to disable persistence and clear existing data.
	RetentionDays int `json:"retention_days,omitempty"`
	// Metadata stores caller-defined metric metadata.
	//
	// Metadata saves metric metadata defined by the caller.
	Metadata map[string]string `json:"metadata,omitempty"`
	// CreatedAt records when the metric definition was created.
	//
	// CreatedAt records the time when the metric definition was created.
	CreatedAt time.Time `json:"created_at,omitempty"`
	// UpdatedAt records when the metric definition was last updated.
	//
	// UpdatedAt records the time when the metric definition was last updated.
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// withDefaults fills default values on a metric definition.
//
// withDefaults populates the metric definition with default types.
func (d Definition) withDefaults() Definition {
	if d.Type == "" {
		d.Type = TypeGauge
	}
	return d
}

// Validate checks whether the value is well formed.
//
// Validate checks whether the metric definition is legal.
func (d Definition) Validate() error {
	if strings.TrimSpace(d.Name) == "" {
		return fmt.Errorf("%w: metric name is required", ErrInvalidArgument)
	}
	switch d.Type {
	case "", TypeGauge, TypeCounter, TypeHistogram, TypeSummary:
	default:
		return fmt.Errorf("%w: unsupported metric type %q", ErrInvalidArgument, d.Type)
	}
	if d.RetentionDays < 0 {
		return fmt.Errorf("%w: retention days cannot be negative", ErrInvalidArgument)
	}
	return nil
}

// Point stores one metric sample.
//
// Point represents an metric sample of an entity at a certain moment.
type Point struct {
	// MetricName names the metric this sample belongs to.
	//
	// MetricName indicates the metric name to which the sample belongs.
	MetricName string `json:"metric_name"`
	// EntityID identifies the entity that emitted the sample.
	//
	// EntityID identifies the entity that generated the sample.
	EntityID string `json:"entity_id"`
	// Timestamp is the sample time.
	//
	// Timestamp is the sampling time.
	Timestamp time.Time `json:"timestamp"`
	// Value is the numeric sample value.
	//
	// Value is the sampled value.
	Value float64 `json:"value"`
	// Tags identify the logical series within a metric and entity.
	//
	// Tags identify logical sequences under the same metric and entity.
	Tags map[string]string `json:"tags,omitempty"`
	// Labels carry extra metadata that does not define series identity.
	//
	// Labels carry additional metadata that is not involved in sequence identity determination.
	Labels map[string]string `json:"labels,omitempty"`
}

// Validate checks whether the value is well formed.
//
// Validate checks whether the sample point contains necessary fields.
func (p Point) Validate() error {
	if strings.TrimSpace(p.MetricName) == "" {
		return fmt.Errorf("%w: metric name is required", ErrInvalidArgument)
	}
	if strings.TrimSpace(p.EntityID) == "" {
		return fmt.Errorf("%w: entity id is required", ErrInvalidArgument)
	}
	if p.Timestamp.IsZero() {
		return fmt.Errorf("%w: timestamp is required", ErrInvalidArgument)
	}
	if math.IsNaN(p.Value) || math.IsInf(p.Value, 0) {
		return fmt.Errorf("%w: value must be finite", ErrInvalidArgument)
	}
	return nil
}

// normalized returns a canonical form of the value.
//
// normalized normalizes the sample point time to UTC, and fills in empty labels and label description maps.
func (p Point) normalized() Point {
	p.Timestamp = p.Timestamp.UTC()
	if p.Tags == nil {
		p.Tags = map[string]string{}
	}
	if p.Labels == nil {
		p.Labels = map[string]string{}
	}
	return p
}

// Query loads raw metric points matching a query.
//
// Query describes the original point query conditions, including metrics, entities, time ranges, labels and paging.
type Query struct {
	// MetricName restricts the query to one metric.
	//
	// MetricName limits the query to a single metric.
	MetricName string `json:"metric_name"`
	// EntityID optionally restricts the query to one entity.
	//
	// EntityID optionally limits the query to a single entity.
	EntityID string `json:"entity_id,omitempty"`
	// Start is the inclusive query start time.
	//
	// Start is the query start time including bounds.
	Start time.Time `json:"start"`
	// End is the inclusive query end time.
	//
	// End is the end time of the query including the bounds.
	End time.Time `json:"end"`
	// Tags filters points by exact tag key/value matches.
	//
	// Tags filters sample points by exact matching of tag key values.
	Tags map[string]string `json:"tags,omitempty"`
	// Limit limits the number of raw points returned.
	//
	// Limit limits the number of original points returned.
	Limit int `json:"limit,omitempty"`
	// WorkBudget bounds raw points examined by SQLite V4 (including points in
	// overlapping blocks). Zero leaves ordinary internal query semantics intact.
	// This is an internal resource limit, not a response page size.
	WorkBudget int `json:"-"`
	// Offset skips this many raw points before returning results.
	//
	// Offset skips the specified number of original points before returning the result.
	Offset int `json:"offset,omitempty"`
	// Order controls chronological result ordering.
	//
	// Order controls the chronological order of results.
	Order Order `json:"order,omitempty"`
}

// Validate checks whether the value is well formed.
//
// Validate checks whether the original query conditions are legal.
func (q Query) Validate() error {
	if strings.TrimSpace(q.MetricName) == "" {
		return fmt.Errorf("%w: metric name is required", ErrInvalidArgument)
	}
	if q.Start.IsZero() || q.End.IsZero() {
		return fmt.Errorf("%w: start and end time are required", ErrInvalidArgument)
	}
	if q.End.Before(q.Start) {
		return fmt.Errorf("%w: end time cannot be before start time", ErrInvalidArgument)
	}
	if q.Limit < 0 || q.Offset < 0 || q.WorkBudget < 0 {
		return fmt.Errorf("%w: limit and offset cannot be negative", ErrInvalidArgument)
	}
	switch q.Order {
	case "", OrderAsc, OrderDesc:
	default:
		return fmt.Errorf("%w: unsupported order %q", ErrInvalidArgument, q.Order)
	}
	return nil
}

// normalized returns a canonical form of the value.
//
// normalized normalizes the query time to UTC and sets the default ordering.
func (q Query) normalized() Query {
	q.Start = q.Start.UTC()
	q.End = q.End.UTC()
	if q.Order == "" {
		q.Order = OrderAsc
	}
	return q
}

// AggregateQuery describes a bucketed aggregate query.
//
// AggregateQuery describes an aggregate query that is bucketed by a fixed time interval.
type AggregateQuery struct {
	// Query supplies the raw series filter and time window.
	//
	// Query provides raw sequence filter conditions and time windows.
	Query
	// Aggregation selects the bucket aggregation to compute.
	//
	// Aggregation Select the bucket aggregation method to be calculated.
	Aggregation Aggregation `json:"aggregation"`
	// Interval is the width of each aggregate bucket.
	//
	// Interval is the width of each aggregate bucket.
	Interval time.Duration `json:"interval"`
	// PreserveSeries keeps entity/tag identities as separate aggregate series on
	// rollup-backed reads. The default preserves the historical rollup behavior
	// of merging all matched series into each output bucket.
	//
	// PreserveSeries Preserves entity/tag dimensions as independent aggregate sequences in rollup-based reads.
	// The default value retains the historical rollup behavior: merge matched sequences into the same output bucket.
	PreserveSeries bool `json:"preserve_series,omitempty"`
	// OmitTags is an internal read optimization for callers that only need the
	// entity dimension. It is excluded from serialized query contracts.
	OmitTags bool `json:"-"`
	// BucketLimit and BucketOffset page over the produced aggregate buckets, not
	// the underlying raw points. They are applied consistently across every
	// backend and aggregation type. The embedded Query.Limit/Query.Offset are
	// ignored for aggregation (they describe raw-point paging, which would mean
	// something different depending on whether the aggregation is pushed down to
	// SQL or computed in memory).
	//
	// BucketLimit and BucketOffset page into the resulting aggregate buckets rather than the underlying raw points.
	// They will be consistent across all backends and aggregation types. Embedded Query.Limit/Query.Offset
	// are ignored in aggregations because they describe origin point pagination; depending on whether the aggregation is pushed down to SQL or in
	// In-memory computing, this results in different semantics.
	BucketLimit int `json:"bucket_limit,omitempty"`
	// BucketOffset skips this many aggregate buckets before returning results.
	//
	// BucketOffset skips the specified number of aggregation buckets before returning results.
	BucketOffset int `json:"bucket_offset,omitempty"`
}

// Validate checks whether the value is well formed.
//
// Validate checks whether the time interval, paging and aggregation type of the aggregate query are legal.
func (q AggregateQuery) Validate() error {
	if err := q.Query.Validate(); err != nil {
		return err
	}
	if q.Interval <= 0 {
		return fmt.Errorf("%w: aggregate interval must be positive", ErrInvalidArgument)
	}
	if q.BucketLimit < 0 || q.BucketOffset < 0 {
		return fmt.Errorf("%w: bucket limit and offset cannot be negative", ErrInvalidArgument)
	}
	switch q.Aggregation {
	case AggAvg, AggMin, AggMax, AggSum, AggCount, AggFirst, AggLast, AggRate, AggStdDev:
	default:
		if isInternalPingLossAggregation(q.Aggregation) {
			return nil
		}
		// Any percentile (p50, p95, p99, p99.9, ...) is also valid. The fixed
		// AggP50/AggP95/AggP99 constants fall through to here as well.
		if !isPercentile(q.Aggregation) {
			return fmt.Errorf("%w: unsupported aggregation %q", ErrInvalidArgument, q.Aggregation)
		}
	}
	return nil
}

// AggregatePoint stores one aggregate bucket result.
//
// AggregatePoint represents the results of an aggregate bucket.
type AggregatePoint struct {
	// MetricName is the metric represented by the bucket.
	//
	// MetricName is the name of the metric represented by this bucket.
	MetricName string `json:"metric_name"`
	// EntityID is the entity represented by the bucket when one was requested.
	//
	// EntityID is the entity that the bucket represents when requesting a qualified entity.
	EntityID string `json:"entity_id,omitempty"`
	// Bucket is the bucket start time.
	//
	// Bucket is the bucket starting time.
	Bucket time.Time `json:"bucket"`
	// Value is the computed aggregate value.
	//
	// Value is the calculated aggregate value.
	Value float64 `json:"value"`
	// Count is the number of points represented by the bucket.
	//
	// Count is the number of points represented by this bucket.
	Count int `json:"count"`
	// Tags identify the logical series represented by the bucket.
	//
	// Tags identify the logical sequence to which the aggregation bucket belongs.
	Tags map[string]string `json:"tags,omitempty"`
}

// Stats stores or computes summary statistics for a point series.
//
// Stats represents a statistical summary of a raw point sequence.
type Stats struct {
	// Count is the number of points in the series.
	//
	// Count is the number of points in the sequence.
	Count int `json:"count"`
	// Min is the minimum value.
	//
	// Min is the minimum value.
	Min float64 `json:"min"`
	// Max is the maximum value.
	//
	// Max is the maximum value.
	Max float64 `json:"max"`
	// Avg is the arithmetic mean.
	//
	// Avg is the arithmetic mean.
	Avg float64 `json:"avg"`
	// Sum is the sum of all values.
	//
	// Sum is the sum of all values.
	Sum float64 `json:"sum"`
	// P50 is the 50th percentile.
	//
	// P50 is the 50th percentile.
	P50 float64 `json:"p50"`
	// P95 is the 95th percentile.
	//
	// P95 is the 95th percentile.
	P95 float64 `json:"p95"`
	// P99 is the 99th percentile.
	//
	// P99 is the 99th percentile.
	P99 float64 `json:"p99"`
	// First is the first value in time order.
	//
	// First is the first value in chronological order.
	First float64 `json:"first"`
	// Last is the last value in time order.
	//
	// Last is the last value in chronological order.
	Last float64 `json:"last"`
	// Rate is the reset-aware per-second rate.
	//
	// Rate is the rate per second at which resets can be processed.
	Rate float64 `json:"rate"`
	// Start is the first point timestamp.
	//
	// Start is the timestamp of the first point.
	Start time.Time `json:"start"`
	// End is the last point timestamp.
	//
	// End is the timestamp of the last point.
	End time.Time `json:"end"`
	// StdDev is the population standard deviation.
	//
	// StdDev is the population standard deviation.
	StdDev float64 `json:"std_dev"`
}
