package metric

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Store is the main metric storage handle.
//
// Store is the main entry point for the metric package, encapsulating database connections, SQL dialects, and table names.
type Store struct {
	// cfg is the validated store configuration.
	//
	// cfg is the verified Store configuration.
	cfg Config
	// db is the primary database pool used for writes and fallback reads.
	//
	// db is the main database connection pool used for writing and reading.
	db *sql.DB
	// readDB is the optional dedicated read-only pool.
	//
	// readDB is an optional dedicated read-only connection pool.
	readDB *sql.DB
	// ownedDB reports whether Store should close db.
	//
	// ownedDB indicates whether the Store should close the db.
	ownedDB bool
	// ownedReadDB reports whether Store should close readDB.
	//
	// ownedReadDB indicates whether the Store should close readDB.
	ownedReadDB bool
	// heavyReadGate keeps historical decode work bounded independently of the
	// number of connected agents and HTTP requests.
	heavyReadGate chan struct{}
	// axisCache keeps only immutable decoded shared time axes. Its fixed budget
	// prevents dashboard query acceleration from scaling memory with fleet size.
	axisCacheMu sync.Mutex
	axisCache   *sqliteAxisCache
	// dialect renders backend-specific SQL.
	//
	// dialect rendering backend-specific SQL.
	dialect dialect
	// tables stores the physical table names for this store.
	//
	// tables stores the actual table names of the current Store.
	tables tables
	// maintenanceMu serializes physical storage maintenance while allowing
	// concurrent size reads.
	//
	// maintenanceMu serializes physical storage maintenance while allowing concurrent reads of the storage size.
	maintenanceMu sync.RWMutex
	// retentionMu serializes a retention change with writes and compaction so a
	// disabled metric cannot be repopulated by an in-flight operation.
	retentionMu sync.RWMutex
	// mu protects closed state.
	//
	// mu protects the closed state.
	mu sync.RWMutex
	// closed reports whether Close has been called.
	//
	// closed indicates whether Close has been called.
	closed bool
	// sqliteStorageV3 reports that points/rollups are compatibility views over
	// the normalized SQLite series/value tables.
	sqliteStorageV3 bool
	// sqliteStorageV4 reports that sealed raw points are stored in lossless
	// compressed blocks in addition to the V3-compatible hot table.
	sqliteStorageV4 bool
	// sqlitePingMerged reports that ping.loss is a virtual projection over the
	// single physical ping.latency_ms series introduced by SQLite format V8.
	sqlitePingMerged bool
}

// IsVirtualMetric reports whether a public metric is projected from another
// physical series and must therefore be omitted from storage maintenance jobs.
func (s *Store) IsVirtualMetric(metricName string) bool {
	return s != nil && s.sqlitePingMerged && metricName == sqliteVirtualPingLossMetric
}

// Open initializes a Store from a Config.
//
// Open opens the Store according to configuration, initializes the connection pool, and performs automatic migration if needed.
func Open(ctx context.Context, cfg Config) (*Store, error) {
	if cfg.TablePrefix == "" {
		cfg.TablePrefix = "metric_"
	}
	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = 10 * time.Second
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.Driver == DriverSQLite {
		var err error
		cfg, err = prepareSQLiteConfig(cfg)
		if err != nil {
			return nil, err
		}
	}

	heavyReadConcurrency := cfg.HeavyReadConcurrency
	if heavyReadConcurrency <= 0 {
		heavyReadConcurrency = cfg.SQLite.HeavyReadConcurrency
	}
	if heavyReadConcurrency <= 0 {
		heavyReadConcurrency = cfg.SQLite.ReadPoolSize
	}
	if heavyReadConcurrency <= 0 {
		heavyReadConcurrency = 1
	}
	s := &Store{
		cfg:     cfg,
		dialect: newDialect(cfg.Driver),
		tables: tables{
			definitions:  tableName(cfg.TablePrefix, "definitions"),
			points:       tableName(cfg.TablePrefix, "points"),
			rollups:      tableName(cfg.TablePrefix, "rollups"),
			watermarks:   tableName(cfg.TablePrefix, "compaction_watermarks"),
			series:       tableName(cfg.TablePrefix, "series"),
			labels:       tableName(cfg.TablePrefix, "labels"),
			resolutions:  tableName(cfg.TablePrefix, "resolutions"),
			pointValues:  tableName(cfg.TablePrefix, "point_values"),
			pointBlocks:  tableName(cfg.TablePrefix, "point_blocks"),
			pointAxes:    tableName(cfg.TablePrefix, "point_axes"),
			rollupValues: tableName(cfg.TablePrefix, "rollup_values"),
			rollupBlocks: tableName(cfg.TablePrefix, "rollup_blocks"),
			rollupAxes:   tableName(cfg.TablePrefix, "rollup_axes"),
		},
		heavyReadGate: make(chan struct{}, heavyReadConcurrency),
		axisCache:     newSQLiteAxisCache(defaultSQLiteAxisCacheBytes),
	}

	if cfg.DB != nil {
		s.db = cfg.DB
	} else {
		db, err := sql.Open(cfg.driverName(), cfg.DSN)
		if err != nil {
			return nil, err
		}
		s.db = db
		s.ownedDB = true
	}

	if cfg.MaxOpenConns > 0 {
		s.db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		s.db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		s.db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}

	if err := pingDatabaseWithTimeout(ctx, s.db, cfg.ConnectTimeout); err != nil {
		if s.ownedDB {
			_ = s.db.Close()
		}
		return nil, err
	}

	if cfg.Driver == DriverSQLite {
		if err := s.configureSQLitePool(ctx, s.db, 1, cfg.SQLite.CacheSizeKB); err != nil {
			if s.ownedDB {
				_ = s.db.Close()
			}
			return nil, err
		}
	}

	if cfg.AutoMigrate {
		if err := s.Migrate(ctx); err != nil {
			s.closeDBs()
			return nil, err
		}
	} else if cfg.Driver == DriverSQLite {
		pointType, err := sqliteObjectType(ctx, s.db, s.tables.points)
		if err != nil {
			s.closeDBs()
			return nil, err
		}
		rollupType, err := sqliteObjectType(ctx, s.db, s.tables.rollups)
		if err != nil {
			s.closeDBs()
			return nil, err
		}
		s.sqliteStorageV3 = pointType == "view" && rollupType == "view"
		if s.sqliteStorageV3 {
			blockType, err := sqliteObjectType(ctx, s.db, s.tables.pointBlocks)
			if err != nil {
				s.closeDBs()
				return nil, err
			}
			rollupBlockType, err := sqliteObjectType(ctx, s.db, s.tables.rollupBlocks)
			if err != nil {
				s.closeDBs()
				return nil, err
			}
			if (blockType == "table") != (rollupBlockType == "table") {
				s.closeDBs()
				return nil, fmt.Errorf("metric: incomplete SQLite V4 schema requires automatic migration")
			}
			s.sqliteStorageV4 = blockType == "table" && rollupBlockType == "table"
			if s.sqliteStorageV4 {
				var userVersion int
				if err := s.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&userVersion); err != nil {
					s.closeDBs()
					return nil, fmt.Errorf("metric: inspect SQLite storage version: %w", err)
				}
				if userVersion < sqliteStorageVersionCurrent {
					s.closeDBs()
					return nil, fmt.Errorf("metric: SQLite storage V%d requires automatic migration", userVersion)
				}
				s.sqlitePingMerged = true
			}
		}
	}

	// Open the optional read pool only after migrations finish. This keeps a
	// second SQLite connection from holding stale schema state or read locks
	// while an existing database is upgraded atomically.
	if cfg.Driver == DriverSQLite && cfg.SQLite.ReadPoolSize > 1 && cfg.DB == nil && !isMemoryDSN(cfg.DSN) {
		// The primary connection uses BEGIN IMMEDIATE to serialize writers. Reads
		// need a deferred transaction so a V4 hot/block snapshot does not reserve
		// the write lock while it is being decoded.
		readDSN := setSQLiteDSNParam(cfg.DSN, "_txlock", "deferred")
		readDB, err := sql.Open(cfg.driverName(), readDSN)
		if err != nil {
			if s.ownedDB {
				_ = s.db.Close()
			}
			return nil, err
		}
		readDB.SetMaxOpenConns(cfg.SQLite.ReadPoolSize)
		readDB.SetMaxIdleConns(cfg.SQLite.ReadPoolSize)
		if cfg.ConnMaxLifetime > 0 {
			readDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
		}
		if err := pingDatabaseWithTimeout(ctx, readDB, cfg.ConnectTimeout); err != nil {
			_ = readDB.Close()
			if s.ownedDB {
				_ = s.db.Close()
			}
			return nil, err
		}
		readCacheKB := cfg.SQLite.ReadCacheSizeKB
		if readCacheKB <= 0 {
			readCacheKB = cfg.SQLite.CacheSizeKB
		}
		if err := s.configureSQLitePool(ctx, readDB, cfg.SQLite.ReadPoolSize, readCacheKB); err != nil {
			_ = readDB.Close()
			if s.ownedDB {
				_ = s.db.Close()
			}
			return nil, err
		}
		s.readDB = readDB
		s.ownedReadDB = true
	}

	return s, nil
}

func pingDatabaseWithTimeout(ctx context.Context, db *sql.DB, timeout time.Duration) error {
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return db.PingContext(pingCtx)
}

// reader returns the connection pool to use for read-only queries: the
// dedicated read pool when one is configured, otherwise the primary pool.
//
// reader returns the connection pool that should be used for read-only queries; if a dedicated read pool is configured, the read pool will be used.
// Otherwise the main connection pool is used.
func (s *Store) reader() *sql.DB {
	if s.readDB != nil {
		return s.readDB
	}
	return s.db
}

// closeDBs closes database pools owned by the Store.
//
// closeDBs closes the database connection pool created and owned by Store itself.
func (s *Store) closeDBs() {
	if s.ownedReadDB && s.readDB != nil {
		_ = s.readDB.Close()
	}
	if s.ownedDB && s.db != nil {
		_ = s.db.Close()
	}
}

// prepareSQLiteConfig fills SQLite defaults and prepares file storage.
//
// prepareSQLiteConfig completes the SQLite default parameters and ensures that the file database directory exists.
func prepareSQLiteConfig(cfg Config) (Config, error) {
	if cfg.SQLite.BusyTimeout == 0 {
		cfg.SQLite.BusyTimeout = 5 * time.Second
	}
	if cfg.SQLite.CacheSizeKB == 0 {
		cfg.SQLite.CacheSizeKB = 64 * 1024
	}
	if cfg.SQLite.MMapSizeBytes == 0 {
		cfg.SQLite.MMapSizeBytes = 256 * 1024 * 1024
	}
	if cfg.SQLite.WALAutoCheckpoint == 0 {
		cfg.SQLite.WALAutoCheckpoint = 256
	}
	if cfg.SQLite.JournalSizeLimitBytes == 0 {
		cfg.SQLite.JournalSizeLimitBytes = 1024 * 1024
	}

	if cfg.DB == nil {
		if err := ensureSQLiteDir(cfg.DSN); err != nil {
			return cfg, err
		}
		cfg.DSN = appendSQLiteDSNParam(cfg.DSN, "_busy_timeout", fmt.Sprintf("%d", durationMillis(cfg.SQLite.BusyTimeout)))
	}
	return cfg, nil
}

// ensureSQLiteDir creates the directory for a file-backed SQLite DSN.
//
// ensureSQLiteDir creates the directory where the file database is located based on the SQLite DSN.
func ensureSQLiteDir(dsn string) error {
	path := sqliteFilePath(dsn)
	if path == "" || path == ":memory:" || strings.Contains(dsn, "mode=memory") {
		return nil
	}
	dir := filepath.Dir(filepath.FromSlash(path))
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}

// sqliteFilePath extracts the filesystem path portion of a SQLite DSN, dropping
// the "file:" scheme prefix and any query string.
//
// sqliteFilePath extracts the file path portion from the SQLite DSN and removes the file: prefix and
// Query string.
func sqliteFilePath(dsn string) string {
	path := strings.TrimPrefix(dsn, "file:")
	if idx := strings.Index(path, "?"); idx >= 0 {
		path = path[:idx]
	}
	return path
}

// isMemoryDSN reports whether the DSN refers to an in-memory SQLite database,
// which cannot be shared across independent connection pools.
//
// isMemoryDSN determines whether the DSN points to an in-memory SQLite database; this kind of database cannot be used independently
// Shared between connection pools.
func isMemoryDSN(dsn string) bool {
	if strings.Contains(dsn, "mode=memory") {
		return true
	}
	return sqliteFilePath(dsn) == ":memory:"
}

// configureSQLite applies SQLite PRAGMA settings.
//
// configureSQLite performs WAL, busy_timeout, cache, etc. PRAGMA on SQLite connections.
func (s *Store) configureSQLite(ctx context.Context, db *sql.DB) error {
	return s.configureSQLitePool(ctx, db, 1, s.cfg.SQLite.CacheSizeKB)
}

// configureSQLitePool eagerly opens every pooled connection while earlier
// connections remain pinned. SQLite PRAGMAs such as cache_size, mmap_size and
// temp_store are connection-local, so running them once through *sql.DB is not
// sufficient for a multi-connection read pool.
func (s *Store) configureSQLitePool(ctx context.Context, db *sql.DB, size, cacheSizeKB int) error {
	if size < 1 {
		size = 1
	}
	connections := make([]*sql.Conn, 0, size)
	defer func() {
		for _, connection := range connections {
			_ = connection.Close()
		}
	}()
	for i := 0; i < size; i++ {
		connection, err := db.Conn(ctx)
		if err != nil {
			return err
		}
		connections = append(connections, connection)
		if err := s.configureSQLiteConnection(ctx, connection, cacheSizeKB); err != nil {
			return err
		}
	}
	return nil
}

type sqlitePragmaExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func (s *Store) configureSQLiteConnection(ctx context.Context, db sqlitePragmaExecer, cacheSizeKB int) error {
	if s.cfg.SQLite.PageSize > 0 {
		if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA page_size = %d", s.cfg.SQLite.PageSize)); err != nil {
			return err
		}
	}

	if cacheSizeKB <= 0 {
		cacheSizeKB = s.cfg.SQLite.CacheSizeKB
	}
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA foreign_keys = ON",
		sqliteSynchronousPragma(s.cfg.SQLite.PerformanceProfile),
		fmt.Sprintf("PRAGMA busy_timeout = %d", durationMillis(s.cfg.SQLite.BusyTimeout)),
		fmt.Sprintf("PRAGMA cache_size = -%d", cacheSizeKB),
		fmt.Sprintf("PRAGMA mmap_size = %d", s.cfg.SQLite.MMapSizeBytes),
		fmt.Sprintf("PRAGMA wal_autocheckpoint = %d", s.cfg.SQLite.WALAutoCheckpoint),
		fmt.Sprintf("PRAGMA journal_size_limit = %d", s.cfg.SQLite.JournalSizeLimitBytes),
	}
	if s.cfg.SQLite.TempStoreMemory {
		pragmas = append(pragmas, "PRAGMA temp_store = MEMORY")
	} else {
		pragmas = append(pragmas, "PRAGMA temp_store = FILE")
	}

	for _, pragma := range pragmas {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			return err
		}
	}
	return nil
}

// sqliteSynchronousPragma returns the synchronous PRAGMA for a profile.
//
// sqliteSynchronousPragma Returns a SQLite synchronous PRAGMA based on performance preferences.
func sqliteSynchronousPragma(profile SQLitePerformanceProfile) string {
	switch profile {
	case SQLiteProfilePerformance:
		return "PRAGMA synchronous = OFF"
	case SQLiteProfileDurable:
		return "PRAGMA synchronous = FULL"
	default:
		return "PRAGMA synchronous = NORMAL"
	}
}

// durationMillis converts a duration to rounded-up milliseconds.
//
// durationMillis Converts duration to milliseconds rounded up.
func durationMillis(d time.Duration) int {
	return int(math.Ceil(float64(d) / float64(time.Millisecond)))
}

// Close closes resources owned by the Store.
//
// Close closes the connection pool owned by the Store; external incoming DBs will not be closed.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	var firstErr error
	if s.ownedReadDB && s.readDB != nil {
		if err := s.readDB.Close(); err != nil {
			firstErr = err
		}
	}
	if s.ownedDB && s.db != nil {
		if err := s.db.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Ping verifies that the database connection is usable.
//
// Ping checks whether the underlying database connection is available.
func (s *Store) Ping(ctx context.Context) error {
	if err := s.ensureOpen(); err != nil {
		return err
	}
	return s.db.PingContext(ctx)
}

// ensureOpen verifies that the Store is not closed.
//
// ensureOpen checks whether the Store is still open.
func (s *Store) ensureOpen() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return ErrClosed
	}
	if s.db == nil {
		return ErrClosed
	}
	return nil
}

// CreateMetric creates a metric definition.
//
// CreateMetric creates a new metric definition; returns ErrAlreadyExists if a metric with the same name already exists.
func (s *Store) CreateMetric(ctx context.Context, def Definition) error {
	if err := s.ensureOpen(); err != nil {
		return err
	}
	def = def.withDefaults()
	if err := def.Validate(); err != nil {
		return err
	}
	// Fail fast on an existing name so CreateMetric has create-only semantics.
	// The plain INSERT below still enforces this at the database via the
	// primary-key/unique constraint, closing the check-then-insert race.
	if _, err := s.GetMetric(ctx, def.Name); err == nil {
		return fmt.Errorf("%w: metric %q", ErrAlreadyExists, def.Name)
	} else if !errors.Is(err, ErrNotFound) {
		return err
	}
	metadata, err := encodeMap(def.Metadata)
	if err != nil {
		return err
	}
	now := time.Now().UTC().UnixNano()
	_, err = s.db.ExecContext(
		ctx,
		insertDefinitionOnlySQL(s.dialect, s.tables),
		def.Name,
		string(def.Type),
		def.Unit,
		def.Description,
		def.RetentionDays,
		metadata,
		now,
		now,
	)
	if err != nil && isUniqueViolation(err) {
		return fmt.Errorf("%w: metric %q", ErrAlreadyExists, def.Name)
	}
	return err
}

// UpsertMetric inserts a metric definition or, if one with the same name already
// exists, updates its mutable fields (type, unit, description, retention,
// metadata). Use this when you intentionally want create-or-replace semantics;
// use CreateMetric when a duplicate name should be an error.
//
// UpsertMetric inserts a metric definition; if a definition with the same name already exists, updates its mutable fields
// (type, unit, description, retention, metadata). When you explicitly need to "Create or Replace"
// Use it when semantics are correct; use CreateMetric when duplicate names should be considered an error.
func (s *Store) UpsertMetric(ctx context.Context, def Definition) error {
	if err := s.ensureOpen(); err != nil {
		return err
	}
	def = def.withDefaults()
	if err := def.Validate(); err != nil {
		return err
	}
	if def.RetentionDays == 0 {
		// Serialize the zero-retention transition with writes and compaction before
		// changing the definition or removing data left by an enabled definition.
		s.retentionMu.Lock()
		defer s.retentionMu.Unlock()
	}
	metadata, err := encodeMap(def.Metadata)
	if err != nil {
		return err
	}
	now := time.Now().UTC().UnixNano()
	_, err = s.db.ExecContext(
		ctx,
		s.dialect.insertDefinitionSQL(s.tables),
		def.Name,
		string(def.Type),
		def.Unit,
		def.Description,
		def.RetentionDays,
		metadata,
		now,
		now,
	)
	if err != nil || def.RetentionDays != 0 {
		return err
	}
	_, err = s.DeleteSeries(ctx, Query{MetricName: def.Name})
	return err
}

// GetMetric loads one metric definition by name.
//
// GetMetric reads the metric definition by name and returns ErrNotFound if it does not exist.
func (s *Store) GetMetric(ctx context.Context, name string) (Definition, error) {
	if err := s.ensureOpen(); err != nil {
		return Definition{}, err
	}
	if strings.TrimSpace(name) == "" {
		return Definition{}, fmt.Errorf("%w: metric name is required", ErrInvalidArgument)
	}
	row := s.reader().QueryRowContext(ctx, fmt.Sprintf(
		`SELECT name, type, unit, description, retention_days, metadata, created_at, updated_at FROM %s WHERE name = %s`,
		s.tables.definitions, s.dialect.placeholder(1),
	), name)
	def, err := scanDefinition(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Definition{}, ErrNotFound
	}
	return def, err
}

// ListMetrics lists all metric definitions.
//
// ListMetrics Lists all metric definitions in ascending order of name.
func (s *Store) ListMetrics(ctx context.Context) ([]Definition, error) {
	if err := s.ensureOpen(); err != nil {
		return nil, err
	}
	rows, err := s.reader().QueryContext(ctx, fmt.Sprintf(
		`SELECT name, type, unit, description, retention_days, metadata, created_at, updated_at FROM %s ORDER BY name ASC`,
		s.tables.definitions,
	))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Definition
	for rows.Next() {
		def, err := scanDefinition(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, def)
	}
	return out, rows.Err()
}

// DeleteMetric deletes a metric definition and all of its raw and rollup data.
//
// DeleteMetric deletes the metric definition and all its raw points and rollup data.
func (s *Store) DeleteMetric(ctx context.Context, name string) error {
	if err := s.ensureOpen(); err != nil {
		return err
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%w: metric name is required", ErrInvalidArgument)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if s.sqliteStorageV4 {
		if _, err := s.deleteSQLiteV4PointsTx(ctx, tx, Query{MetricName: name}, nil); err != nil {
			return err
		}
		if _, err := s.deleteSQLiteV4RollupsTx(ctx, tx, Query{MetricName: name}, nil, nil); err != nil {
			return err
		}
	} else {
		if _, err = tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE metric_name = %s`, s.tables.rollups, s.dialect.placeholder(1)), name); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE metric_name = %s`, s.tables.points, s.dialect.placeholder(1)), name)
	}
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE metric_name = %s`, s.tables.watermarks, s.dialect.placeholder(1)), name); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE name = %s`, s.tables.definitions, s.dialect.placeholder(1)), name); err != nil {
		return err
	}
	if err := s.pruneUnusedSQLiteSeries(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

// UpdateMetricRetention updates one metric's retention policy without deleting
// its existing data. A value of zero disables subsequent persistence. Negative
// values are invalid.
func (s *Store) UpdateMetricRetention(ctx context.Context, name string, retentionDays int) (Definition, error) {
	if err := s.ensureOpen(); err != nil {
		return Definition{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Definition{}, fmt.Errorf("%w: metric name is required", ErrInvalidArgument)
	}
	if retentionDays < 0 {
		return Definition{}, fmt.Errorf("%w: retention days cannot be negative", ErrInvalidArgument)
	}

	s.retentionMu.Lock()
	defer s.retentionMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Definition{}, err
	}
	defer func() { _ = tx.Rollback() }()

	updatedAt := time.Now().UTC().UnixNano()
	result, err := tx.ExecContext(ctx,
		fmt.Sprintf(`UPDATE %s SET retention_days = %s, updated_at = %s WHERE name = %s`,
			s.tables.definitions, s.dialect.placeholder(1), s.dialect.placeholder(2), s.dialect.placeholder(3)),
		retentionDays, updatedAt, name,
	)
	if err != nil {
		return Definition{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Definition{}, err
	}
	if affected == 0 {
		return Definition{}, fmt.Errorf("%w: metric %q", ErrNotFound, name)
	}
	if err := tx.Commit(); err != nil {
		return Definition{}, err
	}
	return s.GetMetric(ctx, name)
}

// SetMetricRetention updates one metric's retention policy. A value of zero
// disables persistence for that metric and removes its raw and rollup data.
// Negative values are invalid.
func (s *Store) SetMetricRetention(ctx context.Context, name string, retentionDays int) (Definition, error) {
	if err := s.ensureOpen(); err != nil {
		return Definition{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Definition{}, fmt.Errorf("%w: metric name is required", ErrInvalidArgument)
	}
	if retentionDays < 0 {
		return Definition{}, fmt.Errorf("%w: retention days cannot be negative", ErrInvalidArgument)
	}

	s.retentionMu.Lock()
	defer s.retentionMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Definition{}, err
	}
	defer func() { _ = tx.Rollback() }()

	updatedAt := time.Now().UTC().UnixNano()
	result, err := tx.ExecContext(ctx,
		fmt.Sprintf(`UPDATE %s SET retention_days = %s, updated_at = %s WHERE name = %s`,
			s.tables.definitions, s.dialect.placeholder(1), s.dialect.placeholder(2), s.dialect.placeholder(3)),
		retentionDays, updatedAt, name,
	)
	if err != nil {
		return Definition{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Definition{}, err
	}
	if affected == 0 {
		return Definition{}, fmt.Errorf("%w: metric %q", ErrNotFound, name)
	}
	if retentionDays == 0 {
		if s.sqliteStorageV4 {
			if _, err := s.deleteSQLiteV4PointsTx(ctx, tx, Query{MetricName: name}, nil); err != nil {
				return Definition{}, err
			}
			if _, err := s.deleteSQLiteV4RollupsTx(ctx, tx, Query{MetricName: name}, nil, nil); err != nil {
				return Definition{}, err
			}
		}
		tables := []string{s.tables.watermarks}
		if !s.sqliteStorageV4 {
			tables = append(tables, s.tables.points, s.tables.rollups)
		}
		for _, table := range tables {
			if _, err := tx.ExecContext(ctx,
				fmt.Sprintf(`DELETE FROM %s WHERE metric_name = %s`, table, s.dialect.placeholder(1)), name,
			); err != nil {
				return Definition{}, err
			}
		}
		if err := s.pruneUnusedSQLiteSeries(ctx, tx); err != nil {
			return Definition{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Definition{}, err
	}
	return s.GetMetric(ctx, name)
}

// DeleteMetricData removes all raw, rollup, and watermark data for one metric.
func (s *Store) DeleteMetricData(ctx context.Context, name string) error {
	if err := s.ensureOpen(); err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("%w: metric name is required", ErrInvalidArgument)
	}

	s.retentionMu.Lock()
	defer s.retentionMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if s.sqliteStorageV4 {
		if _, err := s.deleteSQLiteV4PointsTx(ctx, tx, Query{MetricName: name}, nil); err != nil {
			return err
		}
		if _, err := s.deleteSQLiteV4RollupsTx(ctx, tx, Query{MetricName: name}, nil, nil); err != nil {
			return err
		}
	}
	tables := []string{s.tables.watermarks}
	if !s.sqliteStorageV4 {
		tables = append(tables, s.tables.points, s.tables.rollups)
	}
	for _, table := range tables {
		if _, err := tx.ExecContext(ctx,
			fmt.Sprintf(`DELETE FROM %s WHERE metric_name = %s`, table, s.dialect.placeholder(1)), name,
		); err != nil {
			return err
		}
	}
	if err := s.pruneUnusedSQLiteSeries(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteMetricDataIfDisabled removes a metric's data only while its retention
// policy is still disabled. It prevents a delayed background cleanup from
// deleting data after an administrator has re-enabled the metric.
func (s *Store) DeleteMetricDataIfDisabled(ctx context.Context, name string) (bool, error) {
	if err := s.ensureOpen(); err != nil {
		return false, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return false, fmt.Errorf("%w: metric name is required", ErrInvalidArgument)
	}

	s.retentionMu.Lock()
	defer s.retentionMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	var retentionDays int
	if err := tx.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT retention_days FROM %s WHERE name = %s`, s.tables.definitions, s.dialect.placeholder(1)),
		name,
	).Scan(&retentionDays); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, fmt.Errorf("%w: metric %q", ErrNotFound, name)
		}
		return false, err
	}
	if retentionDays != 0 {
		return false, nil
	}
	if s.sqliteStorageV4 {
		if _, err := s.deleteSQLiteV4PointsTx(ctx, tx, Query{MetricName: name}, nil); err != nil {
			return false, err
		}
		if _, err := s.deleteSQLiteV4RollupsTx(ctx, tx, Query{MetricName: name}, nil, nil); err != nil {
			return false, err
		}
	}
	tables := []string{s.tables.watermarks}
	if !s.sqliteStorageV4 {
		tables = append(tables, s.tables.points, s.tables.rollups)
	}
	for _, table := range tables {
		if _, err := tx.ExecContext(ctx,
			fmt.Sprintf(`DELETE FROM %s WHERE metric_name = %s`, table, s.dialect.placeholder(1)), name,
		); err != nil {
			return false, err
		}
	}
	if err := s.pruneUnusedSQLiteSeries(ctx, tx); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// DeleteEntity deletes all raw and rollup data for one entity across every metric.
//
// DeleteEntity deletes the raw point and rollup data of an entity under all metrics.
func (s *Store) DeleteEntity(ctx context.Context, entityID string) (int64, error) {
	if err := s.ensureOpen(); err != nil {
		return 0, err
	}
	if strings.TrimSpace(entityID) == "" {
		return 0, fmt.Errorf("%w: entity id is required", ErrInvalidArgument)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	var total int64
	if s.sqliteStorageV4 {
		n, err := s.deleteSQLiteV4PointsTx(ctx, tx, Query{EntityID: entityID}, nil)
		if err != nil {
			return total, err
		}
		total += n
		n, err = s.deleteSQLiteV4RollupsTx(ctx, tx, Query{EntityID: entityID}, nil, nil)
		if err != nil {
			return total, err
		}
		total += n
	}
	var tables []string
	if !s.sqliteStorageV4 {
		tables = append(tables, s.tables.points, s.tables.rollups)
	}
	for _, table := range tables {
		where := "entity_id = " + s.dialect.placeholder(1)
		n, err := s.deleteRows(ctx, tx, table, where, entityID)
		if err != nil {
			return total, err
		}
		total += n
	}
	if err := s.pruneUnusedSQLiteSeries(ctx, tx); err != nil {
		return total, err
	}
	if err := tx.Commit(); err != nil {
		return total, err
	}
	return total, nil
}

// DeleteSeries deletes raw and rollup data matching a query-shaped series filter.
// MetricName is required; EntityID and Tags are optional, so callers can delete
// one task tag across all agents or one tagged series for a single agent.
//
// DeleteSeries deletes raw point and rollup data that match the query sequence filter. MetricName is required;
// EntityID and Tags are optional, so the caller can delete a task tag for all agents, or delete
// A labeled sequence of a single agent.
func (s *Store) DeleteSeries(ctx context.Context, filter Query) (int64, error) {
	if err := s.ensureOpen(); err != nil {
		return 0, err
	}
	if strings.TrimSpace(filter.MetricName) == "" {
		return 0, fmt.Errorf("%w: metric name is required", ErrInvalidArgument)
	}
	if s.sqlitePingMerged && filter.MetricName == sqliteVirtualPingLossMetric {
		filter.MetricName = sqliteMergedPingLatencyMetric
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	var total int64
	if s.sqliteStorageV4 {
		n, err := s.deleteSQLiteV4PointsTx(ctx, tx, filter, nil)
		if err != nil {
			return total, err
		}
		total += n
		n, err = s.deleteSQLiteV4RollupsTx(ctx, tx, filter, nil, nil)
		if err != nil {
			return total, err
		}
		total += n
	}
	var tables []string
	if !s.sqliteStorageV4 {
		tables = append(tables, s.tables.points, s.tables.rollups)
	}
	for _, table := range tables {
		args := []any{filter.MetricName}
		parts := []string{"metric_name = " + s.dialect.placeholder(1)}
		if strings.TrimSpace(filter.EntityID) != "" {
			args = append(args, filter.EntityID)
			parts = append(parts, "entity_id = "+s.dialect.placeholder(len(args)))
		}
		for _, k := range sortedKeys(filter.Tags) {
			args = append(args, filter.Tags[k])
			parts = append(parts, s.dialect.jsonExtractEquals("tags", k, s.dialect.placeholder(len(args))))
		}
		n, err := s.deleteRows(ctx, tx, table, strings.Join(parts, " AND "), args...)
		if err != nil {
			return total, err
		}
		total += n
	}
	if strings.TrimSpace(filter.EntityID) == "" && len(filter.Tags) == 0 {
		if _, err := tx.ExecContext(ctx,
			fmt.Sprintf(`DELETE FROM %s WHERE metric_name = %s`, s.tables.watermarks, s.dialect.placeholder(1)),
			filter.MetricName,
		); err != nil {
			return total, err
		}
	}
	if err := s.pruneUnusedSQLiteSeries(ctx, tx); err != nil {
		return total, err
	}
	if err := tx.Commit(); err != nil {
		return total, err
	}
	return total, nil
}

// Write stores one metric point.
//
// Write writes a single sample point.
func (s *Store) Write(ctx context.Context, point Point) error {
	return s.WriteBatch(ctx, []Point{point})
}

// writeBatch writes one chunk of metric points through an executor.
//
// WriteBatch writes sample points in batches and maintains overall transactionality when chunking large batches.
func (s *Store) WriteBatch(ctx context.Context, points []Point) error {
	if err := s.ensureOpen(); err != nil {
		return err
	}
	if len(points) == 0 {
		return nil
	}
	if s.sqlitePingMerged {
		physical := make([]Point, 0, len(points))
		for _, point := range points {
			if point.MetricName != sqliteVirtualPingLossMetric {
				physical = append(physical, point)
			}
		}
		points = physical
		if len(points) == 0 {
			return nil
		}
	}
	s.retentionMu.RLock()
	defer s.retentionMu.RUnlock()
	points, err := s.filterDisabledMetricPoints(ctx, points)
	if err != nil {
		return err
	}
	if len(points) == 0 {
		return nil
	}
	const batchSize = 1000
	// A single chunk is one statement; send it directly. Multiple chunks are
	// wrapped in one transaction so the batch is all-or-nothing rather than
	// leaving earlier chunks committed when a later one fails.
	if len(points) <= batchSize {
		return s.writeBatch(ctx, s.db, points)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for i := 0; i < len(points); i += batchSize {
		end := i + batchSize
		if end > len(points) {
			end = len(points)
		}
		if err := s.writeBatch(ctx, tx, points[i:end]); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// filterDisabledMetricPoints rejects points without a definition and drops
// points whose definition has zero retention. Definitions are the source of
// truth for a metric's retention and lifecycle, so accepting an unknown name
// would create data that compaction and retention cleanup cannot manage.
func (s *Store) filterDisabledMetricPoints(ctx context.Context, points []Point) ([]Point, error) {
	defs, err := s.ListMetrics(ctx)
	if err != nil {
		return nil, err
	}
	definitions := make(map[string]Definition, len(defs))
	for _, def := range defs {
		definitions[def.Name] = def
	}
	filtered := make([]Point, 0, len(points))
	for _, point := range points {
		def, ok := definitions[point.MetricName]
		if !ok {
			return nil, fmt.Errorf("%w: metric %q", ErrNotFound, point.MetricName)
		}
		if def.RetentionDays > 0 {
			filtered = append(filtered, point)
		}
	}
	return filtered, nil
}

// execer is satisfied by both *sql.DB and *sql.Tx, letting writeBatch run either
// standalone or inside the batch transaction.
//
// execer is satisfied by *sql.DB and *sql.Tx at the same time, so that writeBatch can be executed independently.
// Can also be executed in batch transactions.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// querier is satisfied by both *sql.DB and *sql.Tx, letting read helpers run
// either standalone (on the read pool / primary) or inside an existing
// transaction. Running a read on the owning *sql.Tx is required when the store
// holds a single connection (e.g. SQLite with MaxOpenConns=1): issuing the read
// against the pool instead would block forever waiting for the connection the
// transaction already holds.
//
// The querier is satisfied by both *sql.DB and *sql.Tx, so that the read auxiliary function can be executed independently (the reading pool
// or main connection), can also be executed within an existing transaction. When the Store only holds a single connection (e.g.
// SQLite with MaxOpenConns=1), the read within the transaction must go through the *sql.Tx to which it belongs; otherwise, it will go to the connection pool
// Initiating a read will wait forever for the connection occupied by the transaction, causing a deadlock.
type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type queryExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// deleteRows preserves RowsAffected semantics for SQLite V3 compatibility
// views. SQLite excludes INSTEAD OF trigger changes from RowsAffected, so count
// the exact same predicate inside the owning transaction before deleting.
func (s *Store) deleteRows(ctx context.Context, q queryExecer, table, where string, args ...any) (int64, error) {
	var count int64
	countedView := s.cfg.Driver == DriverSQLite && s.sqliteStorageV3 &&
		(table == s.tables.points || table == s.tables.rollups)
	if countedView {
		if err := q.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s`, table, where), args...).Scan(&count); err != nil {
			return 0, err
		}
	}
	result, err := q.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE %s`, table, where), args...)
	if err != nil {
		return 0, err
	}
	if countedView {
		return count, nil
	}
	return result.RowsAffected()
}

func (s *Store) pruneUnusedSQLiteSeries(ctx context.Context, q queryExecer) error {
	if s.cfg.Driver != DriverSQLite || !s.sqliteStorageV3 {
		return nil
	}
	blockPredicate := ""
	if s.sqliteStorageV4 {
		blockPredicate = fmt.Sprintf(
			" AND NOT EXISTS (SELECT 1 FROM %s WHERE %s.series_id = %s.id) AND NOT EXISTS (SELECT 1 FROM %s WHERE %s.series_id = %s.id)",
			s.tables.pointBlocks, s.tables.pointBlocks, s.tables.series,
			s.tables.rollupBlocks, s.tables.rollupBlocks, s.tables.series,
		)
	}
	_, err := q.ExecContext(ctx, fmt.Sprintf(
		`DELETE FROM %s WHERE NOT EXISTS (SELECT 1 FROM %s WHERE %s.series_id = %s.id)
		 AND NOT EXISTS (SELECT 1 FROM %s WHERE %s.series_id = %s.id)%s`,
		s.tables.series, s.tables.pointValues, s.tables.pointValues, s.tables.series,
		s.tables.rollupValues, s.tables.rollupValues, s.tables.series, blockPredicate,
	))
	return err
}

// writeBatch writes one chunk of metric points through an executor.
//
// writeBatch writes a batch of sample points using the given executor.
func (s *Store) writeBatch(ctx context.Context, ex execer, points []Point) error {
	args := make([]any, 0, len(points)*8)
	now := time.Now().UTC().UnixNano()
	for i, point := range points {
		if err := point.Validate(); err != nil {
			return fmt.Errorf("point %d (metric %q, entity %q): %w", i, point.MetricName, point.EntityID, err)
		}
		point = point.normalized()
		// tagsFingerprint returns the canonical tag JSON too, so the tags column
		// reuses it rather than encoding the map a second time.
		tagsHash, tags, err := tagsFingerprint(point.Tags)
		if err != nil {
			return err
		}
		labels, err := encodeMap(point.Labels)
		if err != nil {
			return err
		}
		args = append(args, point.MetricName, point.EntityID, tagsHash, point.Timestamp.UnixNano(), point.Value, tags, labels, now)
	}
	_, err := ex.ExecContext(ctx, s.pointUpsertSQL(len(points)), args...)
	return err
}

func (s *Store) pointUpsertSQL(rowCount int) string {
	if s.cfg.Driver == DriverSQLite && s.sqliteStorageV3 {
		return buildInsertPointsSQL(s.tables.points, sqliteDialect{}, rowCount, "")
	}
	return s.dialect.upsertPointSQL(s.tables, rowCount)
}

func (s *Store) rollupUpsertSQL() string {
	if s.cfg.Driver == DriverSQLite && s.sqliteStorageV3 {
		return buildUpsertRollupSQL(s.tables.rollups, sqliteDialect{}, "")
	}
	return s.dialect.upsertRollupSQL(s.tables)
}

// Query loads raw metric points matching a query.
//
// Query queries the original sample points according to conditions.
func (s *Store) Query(ctx context.Context, query Query) ([]Point, error) {
	if err := s.ensureOpen(); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}
	query = query.normalized()
	virtualLoss := s.sqlitePingMerged && query.MetricName == sqliteVirtualPingLossMetric
	if virtualLoss {
		query.MetricName = sqliteMergedPingLatencyMetric
	}
	if s.sqliteStorageV4 {
		points, err := s.querySQLiteV4Snapshot(ctx, query)
		if err != nil || !virtualLoss {
			return points, err
		}
		return restoreVirtualPingLossPoints(points), nil
	}
	where, args := s.buildWhere(query)
	order := "ASC"
	if query.Order == OrderDesc {
		order = "DESC"
	}

	sqlText := fmt.Sprintf(`SELECT metric_name, entity_id, ts_nano, value, tags, labels FROM %s WHERE %s ORDER BY ts_nano %s`,
		s.tables.points, where, order)
	// Tag filtering is now pushed into buildWhere, so paging always runs in SQL.
	if query.Limit > 0 {
		args = append(args, query.Limit)
		sqlText += " LIMIT " + s.dialect.placeholder(len(args))
	}
	if query.Offset > 0 {
		args = append(args, query.Offset)
		sqlText += " OFFSET " + s.dialect.placeholder(len(args))
	}

	rows, err := s.reader().QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Point
	for rows.Next() {
		var p Point
		var ts int64
		var rawTags, rawLabels any
		if err := rows.Scan(&p.MetricName, &p.EntityID, &ts, &p.Value, &rawTags, &rawLabels); err != nil {
			return nil, err
		}
		p.Timestamp = time.Unix(0, ts).UTC()
		p.Tags, err = decodeMap(rawTags)
		if err != nil {
			return nil, err
		}
		p.Labels, err = decodeMap(rawLabels)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if virtualLoss {
		return restoreVirtualPingLossPoints(out), nil
	}
	return out, nil
}

// EntityIDs returns distinct entity ids that have raw or rollup data matching a query.
//
// EntityIDs Returns the entity IDs that match the query criteria at origin or rollup.
func (s *Store) EntityIDs(ctx context.Context, query Query) ([]string, error) {
	if err := s.ensureOpen(); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}
	query = query.normalized()
	if s.sqlitePingMerged && query.MetricName == sqliteVirtualPingLossMetric {
		query.MetricName = sqliteMergedPingLatencyMetric
	}
	if s.sqliteStorageV4 {
		return s.sqliteV4EntityIDs(ctx, query)
	}

	pointsWhere, args := s.buildWhere(query)
	rollupArgsStart := len(args)
	rollupArgs := []any{query.MetricName, query.Start.UnixNano(), query.End.UnixNano()}
	rollupParts := []string{
		"metric_name = " + s.dialect.placeholder(rollupArgsStart+1),
		"bucket_nano >= " + s.dialect.placeholder(rollupArgsStart+2),
		"bucket_nano <= " + s.dialect.placeholder(rollupArgsStart+3),
	}
	if strings.TrimSpace(query.EntityID) != "" {
		rollupArgs = append(rollupArgs, query.EntityID)
		rollupParts = append(rollupParts, "entity_id = "+s.dialect.placeholder(rollupArgsStart+len(rollupArgs)))
	}
	for _, k := range sortedKeys(query.Tags) {
		rollupArgs = append(rollupArgs, query.Tags[k])
		rollupParts = append(rollupParts, s.dialect.jsonExtractEquals("tags", k, s.dialect.placeholder(rollupArgsStart+len(rollupArgs))))
	}
	args = append(args, rollupArgs...)

	sqlText := fmt.Sprintf(`SELECT DISTINCT entity_id FROM (
SELECT entity_id FROM %s WHERE %s
UNION
SELECT entity_id FROM %s WHERE %s
) AS metric_entities ORDER BY entity_id ASC`,
		s.tables.points, pointsWhere,
		s.tables.rollups, strings.Join(rollupParts, " AND "),
	)
	rows, err := s.reader().QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var entityID string
		if err := rows.Scan(&entityID); err != nil {
			return nil, err
		}
		if entityID != "" {
			out = append(out, entityID)
		}
	}
	return out, rows.Err()
}

// AllEntityIDs returns every entity that still owns raw or rollup data.
func (s *Store) AllEntityIDs(ctx context.Context) ([]string, error) {
	if err := s.ensureOpen(); err != nil {
		return nil, err
	}
	var sqlText string
	if s.sqliteStorageV3 {
		sqlText = fmt.Sprintf(`SELECT DISTINCT entity_id FROM %s ORDER BY entity_id ASC`, s.tables.series)
	} else {
		sqlText = fmt.Sprintf(`SELECT entity_id FROM (
			SELECT entity_id FROM %s
			UNION
			SELECT entity_id FROM %s
		) AS metric_entities ORDER BY entity_id ASC`, s.tables.points, s.tables.rollups)
	}
	rows, err := s.reader().QueryContext(ctx, sqlText)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var entityID string
		if err := rows.Scan(&entityID); err != nil {
			return nil, err
		}
		if entityID != "" {
			result = append(result, entityID)
		}
	}
	return result, rows.Err()
}

// MetricTagValues returns the distinct non-empty values of one tag on a metric.
func (s *Store) MetricTagValues(ctx context.Context, metricName, tagName string) ([]string, error) {
	if err := s.ensureOpen(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(metricName) == "" || strings.TrimSpace(tagName) == "" {
		return nil, fmt.Errorf("%w: metric and tag names are required", ErrInvalidArgument)
	}
	for _, r := range tagName {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return nil, fmt.Errorf("%w: invalid tag name", ErrInvalidArgument)
		}
	}

	var expression string
	switch s.cfg.Driver {
	case DriverSQLite:
		expression = fmt.Sprintf("json_extract(tags, '$.%s')", tagName)
	case DriverMySQL:
		expression = fmt.Sprintf("JSON_UNQUOTE(JSON_EXTRACT(tags, '$.%s'))", tagName)
	case DriverPostgreSQL:
		expression = fmt.Sprintf("tags ->> '%s'", tagName)
	default:
		return nil, fmt.Errorf("%w: unsupported driver %q", ErrInvalidArgument, s.cfg.Driver)
	}

	placeholder := s.dialect.placeholder(1)
	var sqlText string
	var args []any
	if s.sqliteStorageV3 {
		sqlText = fmt.Sprintf(`SELECT DISTINCT %s AS tag_value FROM %s
			WHERE metric_name = %s AND %s IS NOT NULL AND %s <> ''
			ORDER BY tag_value ASC`, expression, s.tables.series, placeholder, expression, expression)
		args = []any{metricName}
	} else {
		secondPlaceholder := s.dialect.placeholder(2)
		sqlText = fmt.Sprintf(`SELECT DISTINCT tag_value FROM (
			SELECT %s AS tag_value FROM %s WHERE metric_name = %s
			UNION
			SELECT %s AS tag_value FROM %s WHERE metric_name = %s
		) AS metric_tags WHERE tag_value IS NOT NULL AND tag_value <> '' ORDER BY tag_value ASC`,
			expression, s.tables.points, placeholder,
			expression, s.tables.rollups, secondPlaceholder)
		args = []any{metricName, metricName}
	}
	rows, err := s.reader().QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

// EntityTagValue identifies one metric series by its entity and one tag value.
type EntityTagValue struct {
	EntityID string
	TagValue string
}

// MetricEntityTagValues returns distinct entity/tag pairs for one metric.
func (s *Store) MetricEntityTagValues(ctx context.Context, metricName, tagName string) ([]EntityTagValue, error) {
	if err := s.ensureOpen(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(metricName) == "" || strings.TrimSpace(tagName) == "" {
		return nil, fmt.Errorf("%w: metric and tag names are required", ErrInvalidArgument)
	}
	for _, r := range tagName {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return nil, fmt.Errorf("%w: invalid tag name", ErrInvalidArgument)
		}
	}

	var expression string
	switch s.cfg.Driver {
	case DriverSQLite:
		expression = fmt.Sprintf("json_extract(tags, '$.%s')", tagName)
	case DriverMySQL:
		expression = fmt.Sprintf("JSON_UNQUOTE(JSON_EXTRACT(tags, '$.%s'))", tagName)
	case DriverPostgreSQL:
		expression = fmt.Sprintf("tags ->> '%s'", tagName)
	default:
		return nil, fmt.Errorf("%w: unsupported driver %q", ErrInvalidArgument, s.cfg.Driver)
	}

	placeholder := s.dialect.placeholder(1)
	var sqlText string
	var args []any
	if s.sqliteStorageV3 {
		sqlText = fmt.Sprintf(`SELECT DISTINCT entity_id, %s AS tag_value FROM %s
			WHERE metric_name = %s AND entity_id <> '' AND %s IS NOT NULL AND %s <> ''
			ORDER BY entity_id ASC, tag_value ASC`, expression, s.tables.series, placeholder, expression, expression)
		args = []any{metricName}
	} else {
		secondPlaceholder := s.dialect.placeholder(2)
		sqlText = fmt.Sprintf(`SELECT DISTINCT entity_id, tag_value FROM (
			SELECT entity_id, %s AS tag_value FROM %s WHERE metric_name = %s
			UNION
			SELECT entity_id, %s AS tag_value FROM %s WHERE metric_name = %s
		) AS metric_entity_tags
		WHERE entity_id <> '' AND tag_value IS NOT NULL AND tag_value <> ''
		ORDER BY entity_id ASC, tag_value ASC`,
			expression, s.tables.points, placeholder,
			expression, s.tables.rollups, secondPlaceholder)
		args = []any{metricName, metricName}
	}
	rows, err := s.reader().QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []EntityTagValue
	for rows.Next() {
		var pair EntityTagValue
		if err := rows.Scan(&pair.EntityID, &pair.TagValue); err != nil {
			return nil, err
		}
		result = append(result, pair)
	}
	return result, rows.Err()
}

// Latest loads the newest points for a metric and entity.
//
// Latest queries the latest sample point of an metric and entity.
func (s *Store) Latest(ctx context.Context, metricName, entityID string, limit int) ([]Point, error) {
	if err := s.ensureOpen(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(metricName) == "" {
		return nil, fmt.Errorf("%w: metric name is required", ErrInvalidArgument)
	}
	if strings.TrimSpace(entityID) == "" {
		return nil, fmt.Errorf("%w: entity id is required", ErrInvalidArgument)
	}
	if limit <= 0 {
		limit = 1
	}
	virtualLoss := s.sqlitePingMerged && metricName == sqliteVirtualPingLossMetric
	if virtualLoss {
		metricName = sqliteMergedPingLatencyMetric
	}
	if s.sqliteStorageV4 {
		points, err := s.querySQLiteV4Snapshot(ctx, Query{
			MetricName: metricName,
			EntityID:   entityID,
			Start:      time.Unix(0, math.MinInt64).UTC(),
			End:        time.Unix(0, math.MaxInt64).UTC(),
			Order:      OrderDesc,
			Limit:      limit,
		})
		if err != nil || !virtualLoss {
			return points, err
		}
		return restoreVirtualPingLossPoints(points), nil
	}
	// Dedicated query rather than a full-range Query: no synthetic time bounds,
	// and the index on (metric_name, entity_id, ts_nano) serves the ORDER BY.
	sqlText := fmt.Sprintf(
		`SELECT metric_name, entity_id, ts_nano, value, tags, labels FROM %s WHERE metric_name = %s AND entity_id = %s ORDER BY ts_nano DESC LIMIT %s`,
		s.tables.points, s.dialect.placeholder(1), s.dialect.placeholder(2), s.dialect.placeholder(3),
	)
	rows, err := s.reader().QueryContext(ctx, sqlText, metricName, entityID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Point
	for rows.Next() {
		var p Point
		var ts int64
		var rawTags, rawLabels any
		if err := rows.Scan(&p.MetricName, &p.EntityID, &ts, &p.Value, &rawTags, &rawLabels); err != nil {
			return nil, err
		}
		p.Timestamp = time.Unix(0, ts).UTC()
		p.Tags, err = decodeMap(rawTags)
		if err != nil {
			return nil, err
		}
		p.Labels, err = decodeMap(rawLabels)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if virtualLoss {
		return restoreVirtualPingLossPoints(out), nil
	}
	return out, nil
}

// LatestBefore returns the newest retained point before an exclusive boundary.
// It checks both indexed raw points and rollup last-values so callers do not
// need to scan and aggregate an entire retention window to restore a counter.
func (s *Store) LatestBefore(ctx context.Context, metricName, entityID string, before time.Time) (Point, bool, error) {
	if err := s.ensureOpen(); err != nil {
		return Point{}, false, err
	}
	if strings.TrimSpace(metricName) == "" {
		return Point{}, false, fmt.Errorf("%w: metric name is required", ErrInvalidArgument)
	}
	if strings.TrimSpace(entityID) == "" {
		return Point{}, false, fmt.Errorf("%w: entity id is required", ErrInvalidArgument)
	}
	if before.IsZero() {
		return Point{}, false, fmt.Errorf("%w: before time is required", ErrInvalidArgument)
	}
	virtualLoss := s.sqlitePingMerged && metricName == sqliteVirtualPingLossMetric
	if virtualLoss {
		metricName = sqliteMergedPingLatencyMetric
	}

	beforeNano := before.UTC().UnixNano()
	var latest Point
	var err error
	found := false
	if s.sqliteStorageV4 && beforeNano > math.MinInt64 {
		points, queryErr := s.querySQLiteV4Snapshot(ctx, Query{
			MetricName: metricName,
			EntityID:   entityID,
			Start:      time.Unix(0, math.MinInt64).UTC(),
			End:        time.Unix(0, beforeNano-1).UTC(),
			Order:      OrderDesc,
			Limit:      1,
		})
		if queryErr != nil {
			return Point{}, false, queryErr
		}
		if len(points) > 0 {
			latest = points[0]
			found = true
		}
	}
	if !s.sqliteStorageV4 {
		rawSQL := fmt.Sprintf(
			`SELECT metric_name, entity_id, ts_nano, value, tags, labels FROM %s
		 WHERE metric_name = %s AND entity_id = %s AND ts_nano < %s
		 ORDER BY ts_nano DESC LIMIT 1`,
			s.tables.points, s.dialect.placeholder(1), s.dialect.placeholder(2), s.dialect.placeholder(3),
		)
		var rawTags, rawLabels any
		var rawNano int64
		err = s.reader().QueryRowContext(ctx, rawSQL, metricName, entityID, beforeNano).Scan(
			&latest.MetricName, &latest.EntityID, &rawNano, &latest.Value, &rawTags, &rawLabels,
		)
		if err == nil {
			latest.Timestamp = time.Unix(0, rawNano).UTC()
			latest.Tags, err = decodeMap(rawTags)
			if err != nil {
				return Point{}, false, err
			}
			latest.Labels, err = decodeMap(rawLabels)
			if err != nil {
				return Point{}, false, err
			}
			found = true
		} else if !errors.Is(err, sql.ErrNoRows) {
			return Point{}, false, err
		}
	}
	if found {
		rawCutoff := s.cfg.RollupPolicy.rawCutoff(before)
		if rawCutoff.IsZero() || !latest.Timestamp.Before(rawCutoff) {
			if virtualLoss {
				latest = virtualPingLossPoint(latest)
			}
			return latest, true, nil
		}
	}

	for _, tier := range s.cfg.RollupPolicy.Tiers {
		if s.sqliteStorageV4 {
			candidate, ok, queryErr := s.latestSQLiteV4RollupBefore(ctx, s.reader(), metricName, entityID, tier.Interval.Nanoseconds(), beforeNano)
			if queryErr != nil {
				return Point{}, false, queryErr
			}
			if ok && (!found || candidate.Timestamp.After(latest.Timestamp)) {
				latest = candidate
				found = true
			}
			continue
		}
		rollupSQL := fmt.Sprintf(
			`SELECT tags, last_val, last_ts FROM %s
			 WHERE metric_name = %s AND entity_id = %s AND resolution_nano = %s AND last_ts < %s
			 ORDER BY last_ts DESC LIMIT 1`,
			s.tables.rollups, s.dialect.placeholder(1), s.dialect.placeholder(2),
			s.dialect.placeholder(3), s.dialect.placeholder(4),
		)
		var tags any
		var value float64
		var timestamp int64
		err = s.reader().QueryRowContext(ctx, rollupSQL, metricName, entityID, tier.Interval.Nanoseconds(), beforeNano).Scan(
			&tags, &value, &timestamp,
		)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return Point{}, false, err
		}
		if found && timestamp <= latest.Timestamp.UnixNano() {
			continue
		}
		decodedTags, err := decodeMap(tags)
		if err != nil {
			return Point{}, false, err
		}
		latest = Point{
			MetricName: metricName,
			EntityID:   entityID,
			Timestamp:  time.Unix(0, timestamp).UTC(),
			Value:      value,
			Tags:       decodedTags,
		}
		found = true
	}
	if found && virtualLoss {
		latest = virtualPingLossPoint(latest)
	}
	return latest, found, nil
}

// Aggregate computes bucketed aggregates from raw points.
//
// Aggregate performs bucket aggregation on the original points, and aggregates that can be pushed down to SQL will be pushed down first.
func (s *Store) Aggregate(ctx context.Context, query AggregateQuery) ([]AggregatePoint, error) {
	if err := s.ensureOpen(); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}
	virtualLoss := s.sqlitePingMerged && query.MetricName == sqliteVirtualPingLossMetric
	if virtualLoss {
		query.MetricName = sqliteMergedPingLatencyMetric
		query.Aggregation = pingLossPhysicalAggregation(query.Aggregation)
	}
	// Push simple reductions (avg/min/max/sum/count) down to SQL via GROUP BY on
	// a time bucket so large ranges don't pull every raw point into memory.
	// Percentiles, first/last and rate need the ordered raw series, so those fall
	// back to the in-memory aggregator.
	if valueExpr, ok := sqlAggValueExpr(s.cfg.Driver, query.Aggregation); ok && !s.sqliteStorageV4 {
		return s.aggregateInSQL(ctx, query, valueExpr)
	}
	// In-memory fallback. Strip the embedded raw-point Limit/Offset so the full
	// series feeds the aggregator; paging is then applied per bucket, matching
	// the SQL pushdown path's BucketLimit/BucketOffset semantics.
	rawQuery := query.Query
	rawQuery.Limit = 0
	rawQuery.Offset = 0
	points, err := s.Query(ctx, rawQuery)
	if err != nil {
		return nil, err
	}
	buckets, err := AggregatePoints(points, query)
	if err != nil {
		return nil, err
	}
	buckets = pageBuckets(buckets, query.BucketLimit, query.BucketOffset)
	if virtualLoss {
		return restoreVirtualPingLossAggregates(buckets), nil
	}
	return buckets, nil
}

// pageBuckets applies bucket-level paging to an ordered slice of aggregate
// points. offset buckets are skipped from the front; at most limit buckets are
// returned (limit <= 0 means no limit). It mirrors the SQL LIMIT/OFFSET applied
// in aggregateInSQL so both paths page identically.
//
// pageBuckets applies bucket-level paging to ordered AggregatePoint slices. It will skip offset from the front
// buckets, and returns at most limit buckets (limit <= 0 means no limit). It mirrors aggregateInSQL
// The SQL LIMIT/OFFSET applied in makes the paging behavior of the two paths consistent.
func pageBuckets(buckets []AggregatePoint, limit, offset int) []AggregatePoint {
	if offset > 0 {
		if offset >= len(buckets) {
			return []AggregatePoint{}
		}
		buckets = buckets[offset:]
	}
	if limit > 0 && limit < len(buckets) {
		buckets = buckets[:limit]
	}
	return buckets
}

// aggregateInSQL computes a bucketed aggregate in the database.
//
// aggregateInSQL uses database GROUP BY to perform pushdown-able aggregate queries.
func (s *Store) aggregateInSQL(ctx context.Context, query AggregateQuery, valueExpr string) ([]AggregatePoint, error) {
	q := query.Query.normalized()
	where, args := s.buildWhere(q)
	interval := query.Interval.Nanoseconds()
	// interval is a trusted int64 (validated > 0); inline it so bucket math and
	// GROUP BY/ORDER BY reference the same expression without extra binds.
	//
	// Note: this bucket expression is a computed (non-sargable) value, so the
	// GROUP BY cannot be served directly by the (metric_name, entity_id,
	// ts_nano) index — the database still range-scans the rows selected by the
	// WHERE clause (which IS index-served) and groups them on the fly. That is
	// fine for typical windows; for very large ranges the cost is the scan, not
	// the grouping. We keep the raw-timestamp index rather than materializing a
	// bucket column so writes stay cheap and the bucket size can vary per query.
	bucketExpr := fmt.Sprintf("(ts_nano - ((ts_nano %% %d) + %d) %% %d)", interval, interval, interval)
	sqlText := fmt.Sprintf(
		`SELECT %s AS bucket, metric_name, entity_id, tags_hash, tags, %s AS agg_value, COUNT(*) AS agg_count FROM %s WHERE %s GROUP BY bucket, metric_name, entity_id, tags_hash, tags ORDER BY bucket ASC, metric_name ASC, entity_id ASC, tags_hash ASC`,
		bucketExpr, valueExpr, s.tables.points, where,
	)
	// Page over aggregate buckets (BucketLimit/BucketOffset), not raw points.
	if query.BucketLimit > 0 {
		args = append(args, query.BucketLimit)
		sqlText += " LIMIT " + s.dialect.placeholder(len(args))
	}
	if query.BucketOffset > 0 {
		args = append(args, query.BucketOffset)
		sqlText += " OFFSET " + s.dialect.placeholder(len(args))
	}

	rows, err := s.reader().QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]AggregatePoint, 0)
	for rows.Next() {
		var bucket int64
		var metricName, entityID, tagsHash string
		var rawTags any
		var value float64
		var count int
		if err := rows.Scan(&bucket, &metricName, &entityID, &tagsHash, &rawTags, &value, &count); err != nil {
			return nil, err
		}
		_ = tagsHash
		tags, err := decodeMap(rawTags)
		if err != nil {
			return nil, err
		}
		out = append(out, AggregatePoint{
			MetricName: metricName,
			EntityID:   entityID,
			Bucket:     time.Unix(0, bucket).UTC(),
			Value:      value,
			Count:      count,
			Tags:       tags,
		})
	}
	return out, rows.Err()
}

// Stats stores or computes summary statistics for a point series.
//
// Stats queries the original points and calculates statistical summaries.
func (s *Store) Stats(ctx context.Context, query Query) (Stats, error) {
	points, err := s.Query(ctx, query)
	if err != nil {
		return Stats{}, err
	}
	var stats Stats
	if s.sqlitePingMerged && query.MetricName == sqliteMergedPingLatencyMetric {
		stats, err = calculateMergedPingLatencyStats(points)
	} else {
		stats, err = CalculateStats(points)
	}
	if errors.Is(err, ErrNoData) {
		// No samples in range. Disambiguate from a non-existent metric so the
		// caller can tell "empty window" apart from "unknown metric".
		if _, gerr := s.GetMetric(ctx, query.MetricName); errors.Is(gerr, ErrNotFound) {
			return Stats{}, ErrNotFound
		} else if gerr != nil {
			return Stats{}, gerr
		}
	}
	return stats, err
}

// DeleteBefore deletes raw points older than a cutoff.
//
// DeleteBefore deletes the original points before the specified time, and the range can be limited by the metric name.
func (s *Store) DeleteBefore(ctx context.Context, metricName string, before time.Time) (int64, error) {
	if err := s.ensureOpen(); err != nil {
		return 0, err
	}
	if before.IsZero() {
		return 0, fmt.Errorf("%w: before time is required", ErrInvalidArgument)
	}
	if s.sqlitePingMerged && metricName == sqliteVirtualPingLossMetric {
		metricName = sqliteMergedPingLatencyMetric
	}
	if s.sqliteStorageV4 {
		beforeNano := before.UTC().UnixNano()
		hasPoints, err := s.sqliteV4HasPointsBefore(ctx, metricName, beforeNano)
		if err != nil {
			return 0, err
		}
		if !hasPoints {
			return 0, nil
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return 0, err
		}
		defer func() { _ = tx.Rollback() }()
		deleted, err := s.deleteSQLiteV4PointsTx(ctx, tx, Query{MetricName: metricName}, &beforeNano)
		if err != nil {
			return deleted, err
		}
		if err := s.pruneUnusedSQLiteSeries(ctx, tx); err != nil {
			return deleted, err
		}
		if err := tx.Commit(); err != nil {
			return deleted, err
		}
		return deleted, nil
	}
	args := []any{before.UTC().UnixNano()}
	where := "ts_nano < " + s.dialect.placeholder(1)
	if strings.TrimSpace(metricName) != "" {
		args = append(args, metricName)
		where += " AND metric_name = " + s.dialect.placeholder(2)
	}
	deleted, err := s.deleteRows(ctx, s.db, s.tables.points, where, args...)
	if err != nil {
		return 0, err
	}
	if err := s.pruneUnusedSQLiteSeries(ctx, s.db); err != nil {
		return deleted, err
	}
	return deleted, nil
}

func (s *Store) sqliteV4HasPointsBefore(ctx context.Context, metricName string, beforeNano int64) (bool, error) {
	metricFilter := ""
	blockArgs := []any{beforeNano}
	hotArgs := []any{beforeNano}
	if strings.TrimSpace(metricName) != "" {
		metricFilter = " AND s.metric_name = ?"
		blockArgs = append(blockArgs, metricName)
		hotArgs = append(hotArgs, metricName)
	}
	args := append(blockArgs, hotArgs...)
	var exists bool
	err := s.db.QueryRowContext(ctx, fmt.Sprintf(
		`SELECT EXISTS(
			SELECT 1 FROM %s b JOIN %s s ON s.id = b.series_id
			WHERE b.start_nano < ?%s LIMIT 1
		) OR EXISTS(
			SELECT 1 FROM %s p JOIN %s s ON s.id = p.series_id
			WHERE p.ts_nano < ?%s LIMIT 1
		)`,
		s.tables.pointBlocks, s.tables.series, metricFilter,
		s.tables.pointValues, s.tables.series, metricFilter,
	), args...).Scan(&exists)
	return exists, err
}

// CleanupExpired deletes expired raw points for every metric.
//
// CleanupExpired cleans expired original points based on the retention days of each metric.
func (s *Store) CleanupExpired(ctx context.Context, now time.Time) (int64, error) {
	defs, err := s.ListMetrics(ctx)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, def := range defs {
		if s.sqlitePingMerged && def.Name == sqliteVirtualPingLossMetric {
			continue
		}
		if def.RetentionDays == 0 {
			deleted, err := s.DeleteSeries(ctx, Query{MetricName: def.Name})
			if err != nil {
				return total, err
			}
			total += deleted
			continue
		}
		deleted, err := s.DeleteBefore(ctx, def.Name, now.AddDate(0, 0, -def.RetentionDays))
		if err != nil {
			return total, err
		}
		total += deleted
	}
	return total, nil
}

// buildWhere renders the WHERE clause and arguments for a raw query.
//
// buildWhere constructs SQL WHERE conditions and parameters based on Query.
func (s *Store) buildWhere(query Query) (string, []any) {
	args := []any{query.MetricName, query.Start.UnixNano(), query.End.UnixNano()}
	parts := []string{
		"metric_name = " + s.dialect.placeholder(1),
		"ts_nano >= " + s.dialect.placeholder(2),
		"ts_nano <= " + s.dialect.placeholder(3),
	}
	if strings.TrimSpace(query.EntityID) != "" {
		args = append(args, query.EntityID)
		parts = append(parts, "entity_id = "+s.dialect.placeholder(len(args)))
	}
	// Push tag filtering down into SQL via the dialect's JSON accessor so that
	// LIMIT/OFFSET can also be applied by the database instead of pulling every
	// matching row into memory. Keys are sorted for deterministic SQL.
	for _, k := range sortedKeys(query.Tags) {
		args = append(args, query.Tags[k])
		parts = append(parts, s.dialect.jsonExtractEquals("tags", k, s.dialect.placeholder(len(args))))
	}
	return strings.Join(parts, " AND "), args
}

// sortedKeys returns sorted map keys.
//
// sortedKeys returns a map's ordered key list, used to generate stable SQL.
func sortedKeys(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// scanDefinition scans a metric definition from one row.
//
// scanDefinition scans a metric definition from a row of query results.
func scanDefinition(scanner interface{ Scan(dest ...any) error }) (Definition, error) {
	var def Definition
	var typ string
	var rawMetadata any
	var created, updated int64
	if err := scanner.Scan(&def.Name, &typ, &def.Unit, &def.Description, &def.RetentionDays, &rawMetadata, &created, &updated); err != nil {
		return Definition{}, err
	}
	metadata, err := decodeMap(rawMetadata)
	if err != nil {
		return Definition{}, err
	}
	def.Type = MetricType(typ)
	def.Metadata = metadata
	def.CreatedAt = time.Unix(0, created).UTC()
	def.UpdatedAt = time.Unix(0, updated).UTC()
	return def, nil
}

// sortedPoints returns points ordered by timestamp.
//
// sortedPoints returns points sorted by time; if the input is sorted, it is reused directly.
func sortedPoints(points []Point) []Point {
	// Callers frequently pass series that are already time-ordered (the SQL
	// queries ORDER BY ts_nano). Detecting that lets us return the input as-is
	// and skip the copy + sort allocation on the common path.
	if isTimeSorted(points) {
		return points
	}
	out := make([]Point, len(points))
	copy(out, points)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Timestamp.Before(out[j].Timestamp)
	})
	return out
}

// isTimeSorted reports whether points are already time sorted.
//
// isTimeSorted determines whether the point sequence has been sorted in ascending order of time.
func isTimeSorted(points []Point) bool {
	for i := 1; i < len(points); i++ {
		if points[i].Timestamp.Before(points[i-1].Timestamp) {
			return false
		}
	}
	return true
}

// isUniqueViolation reports whether err is a unique/primary-key constraint
// violation. It matches on driver error text so the package stays free of
// driver-specific error type imports; this is a best-effort backstop behind the
// explicit existence check in CreateMetric.
//
// isUniqueViolation checks whether err indicates a unique or primary-key constraint violation by
// matching driver error text, avoiding driver-specific error imports. It is a
// best-effort fallback after the explicit existence check.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "unique constraint"): // sqlite, postgres
		return true
	case strings.Contains(msg, "duplicate entry"): // mysql
		return true
	case strings.Contains(msg, "duplicate key"): // postgres
		return true
	case strings.Contains(msg, "constraint failed"): // sqlite variants
		return true
	default:
		return false
	}
}
