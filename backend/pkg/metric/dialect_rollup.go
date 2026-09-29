package metric

import (
	"database/sql"
	"fmt"
	"strings"
)

// rollupColumns is the fixed column list for the rollups table, shared by the
// upsert builder and the row scanner so they cannot drift apart. tags_hash is a
// stable fingerprint of the (canonical) tag set; tags carries the tag map itself
// so reads can return it and tag filters can be pushed down. Including the tag
// dimension in the key is what keeps each distinct tag combination (e.g. a GPU
// device_index) as its own rollup series instead of being merged together.
//
// rollupColumns is a fixed column list of the rollups table, shared by the upsert constructor and row scanner.
// Avoid drifting between the two. tags_hash is a stable fingerprint of the (normalized) tag collection; tags carries the tag map
// itself, so it can be returned when reading, and also push down tag filtering. Incorporating the label dimension into the key allows
// Each different label combination (e.g. GPU's device_index) is retained as its own rollup sequence,
// rather than being merged together.
const rollupColumns = "metric_name, entity_id, tags_hash, tags, resolution_nano, bucket_nano, " +
	"count, sum, sum_sq, min_val, max_val, first_val, first_ts, last_val, last_ts, digest, created_at"

// rollupColumnCount is the number of columns in rollupColumns.
//
// rollupColumnCount is the number of columns in rollupColumns.
const rollupColumnCount = 17

// rollupTagsArgIndex is the 1-based position of the `tags` JSON column in
// rollupColumns. PostgreSQL needs a ::jsonb cast on that bind placeholder.
//
// rollupTagsArgIndex is the 1-based position of the tags JSON column in rollupColumns;
// PostgreSQL requires appending the ::jsonb conversion to the corresponding placeholder.
const rollupTagsArgIndex = 4

// blobType returns the column type used to store the t-digest sketch.
//
// blobType Returns the binary column type used to hold the t-digest digest.
func (sqliteDialect) blobType() string { return "BLOB" }

// blobType returns the binary column type used for t-digest blobs.
//
// blobType Returns the binary column type used by MySQL to hold t-digest digests.
func (mysqlDialect) blobType() string { return "LONGBLOB" }

// blobType returns the binary column type used for t-digest blobs.
//
// blobType Returns the binary column type used by PostgreSQL to hold t-digest digests.
func (postgresDialect) blobType() string { return "BYTEA" }

// compactTxOptions returns nil so SQLite uses its default isolation. A single
// write connection already serializes the compaction transaction against
// concurrent writers, so the raw scan and the raw deletion observe a stable
// view without escalating the isolation level.
//
// compactTxOptions Return nil to have SQLite use the default isolation level. A single write connection has
// compaction transactions are serialized with concurrent writes, so raw scans and raw deletes do not require raising the isolation level
// A stable view can be observed.
func (sqliteDialect) compactTxOptions() *sql.TxOptions { return nil }

// compactTxOptions escalates to SERIALIZABLE so a point inserted between the raw
// scan and the raw deletion cannot be deleted without first being rolled up.
// InnoDB's SERIALIZABLE turns the scan into a locking read that blocks such a
// concurrent insert in the scanned range until the compaction commits.
//
// compactTxOptions raised to SERIALIZABLE, makes the point written between raw scan and raw delete
// It will not be deleted before entering rollup. InnoDB's SERIALIZABLE will turn scanning into locked reading.
// Blocks such concurrent insertions within the scan scope before compaction commits.
func (mysqlDialect) compactTxOptions() *sql.TxOptions {
	return &sql.TxOptions{Isolation: sql.LevelSerializable}
}

// compactTxOptions escalates to SERIALIZABLE so the raw scan and the raw
// deletion share one snapshot. A concurrent insert into the scanned range that
// would otherwise be deleted without a rollup instead triggers a serialization
// failure that CompactMetric retries on a fresh snapshot.
//
// compactTxOptions raised to SERIALIZABLE so that raw scanning and raw deletion share the same snapshot.
// Concurrent insertions within the scan range (otherwise they will be deleted without entering rollup) will trigger serialization failure.
// Retry on new snapshot by CompactMetric.
func (postgresDialect) compactTxOptions() *sql.TxOptions {
	return &sql.TxOptions{Isolation: sql.LevelSerializable}
}

// upsertRollupSQL builds a single-row upsert for a rollup cell, keyed by
// (metric_name, entity_id, tags_hash, resolution_nano, bucket_nano). Compact
// recomputes a bucket wholesale from its inputs each run, so the conflict action
// replaces every summary column rather than trying to merge in SQL.
//
// upsertRollupSQL constructs the upsert SQL of a single rollup unit, and its key is
// (metric_name, entity_id, tags_hash, resolution_nano, bucket_nano).Compact
// Each time a bucket is recalculated from the input as a whole, so conflicting actions replace each summary column,
// Instead of trying to merge in SQL.
func (d sqliteDialect) upsertRollupSQL(t tables) string {
	return buildUpsertRollupSQL(t.rollups, d, sqliteRollupConflict)
}

// upsertRollupSQL builds backend-specific SQL for upserting a rollup cell.
//
// upsertRollupSQL constructs the MySQL rollup unit upsert SQL.
func (d mysqlDialect) upsertRollupSQL(t tables) string {
	return buildUpsertRollupSQL(t.rollups, d, mysqlRollupConflict)
}

// upsertRollupSQL builds backend-specific SQL for upserting a rollup cell.
//
// upsertRollupSQL Constructs the PostgreSQL rollup unit upsert SQL.
func (d postgresDialect) upsertRollupSQL(t tables) string {
	return buildUpsertRollupSQL(t.rollups, d, postgresRollupConflict)
}

// sqliteRollupConflict is the SQLite conflict clause for replacing rollup cells.
//
// sqliteRollupConflict is the conflict handling clause that SQLite uses to replace the rollup unit.
const sqliteRollupConflict = `ON CONFLICT(metric_name, entity_id, tags_hash, resolution_nano, bucket_nano) DO UPDATE SET
	tags = excluded.tags,
	count = excluded.count, sum = excluded.sum, sum_sq = excluded.sum_sq,
	min_val = excluded.min_val, max_val = excluded.max_val,
	first_val = excluded.first_val, first_ts = excluded.first_ts,
	last_val = excluded.last_val, last_ts = excluded.last_ts,
	digest = excluded.digest, created_at = excluded.created_at`

// postgresRollupConflict is the PostgreSQL conflict clause for replacing rollup cells.
//
// postgresRollupConflict is the conflict handling clause that PostgreSQL uses to replace the rollup unit.
const postgresRollupConflict = `ON CONFLICT(metric_name, entity_id, tags_hash, resolution_nano, bucket_nano) DO UPDATE SET
	tags = EXCLUDED.tags,
	count = EXCLUDED.count, sum = EXCLUDED.sum, sum_sq = EXCLUDED.sum_sq,
	min_val = EXCLUDED.min_val, max_val = EXCLUDED.max_val,
	first_val = EXCLUDED.first_val, first_ts = EXCLUDED.first_ts,
	last_val = EXCLUDED.last_val, last_ts = EXCLUDED.last_ts,
	digest = EXCLUDED.digest, created_at = EXCLUDED.created_at`

// mysqlRollupConflict is the MySQL conflict clause for replacing rollup cells.
//
// mysqlRollupConflict is the conflict handling clause used by MySQL to replace the rollup unit.
const mysqlRollupConflict = `ON DUPLICATE KEY UPDATE
	tags = VALUES(tags),
	count = VALUES(count), sum = VALUES(sum), sum_sq = VALUES(sum_sq),
	min_val = VALUES(min_val), max_val = VALUES(max_val),
	first_val = VALUES(first_val), first_ts = VALUES(first_ts),
	last_val = VALUES(last_val), last_ts = VALUES(last_ts),
	digest = VALUES(digest), created_at = VALUES(created_at)`

// buildUpsertRollupSQL builds a single-row rollup upsert statement.
//
// buildUpsertRollupSQL Constructs a single-row rollup upsert SQL.
func buildUpsertRollupSQL(table string, d dialect, conflict string) string {
	ph := make([]string, rollupColumnCount)
	for i := 0; i < rollupColumnCount; i++ {
		if i+1 == rollupTagsArgIndex {
			// The tags column is JSON; PostgreSQL requires an explicit ::jsonb cast.
			ph[i] = d.jsonPlaceholder(i + 1)
		} else {
			ph[i] = d.placeholder(i + 1)
		}
	}
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) %s",
		table, rollupColumns, strings.Join(ph, ", "), conflict)
}
