package metric

import (
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

// Config describes database, pooling, migration, retention, and rollup settings.
//
// Config describes the Store's database backend, connection pooling, migration, retention, and rollup configuration.
type Config struct {
	// Driver selects the database backend.
	//
	// Driver selects the database backend.
	Driver Driver
	// DSN is the database connection string when DB is not supplied.
	//
	// DSN is the database connection string used when DB is not passed in.
	DSN string

	// DB can be supplied by the host app when it owns the connection pool.
	// When DB is set, DSN is ignored and Close leaves the supplied DB open.
	//
	// When the host program manages the connection pool by itself, the DB can be passed in; after setting the DB, the DSN will be ignored.
	// Close will not close this external incoming connection pool either.
	DB *sql.DB

	// TablePrefix prefixes every table managed by the store.
	//
	// TablePrefix is the prefix of all table names managed by Store.
	TablePrefix string
	// AutoMigrate controls whether Open creates or updates the schema.
	//
	// AutoMigrate controls whether Open creates or updates table structures.
	AutoMigrate bool
	// MaxOpenConns sets the primary pool's maximum open connections.
	//
	// MaxOpenConns sets the maximum number of open connections for the main connection pool.
	MaxOpenConns int
	// MaxIdleConns sets the primary pool's maximum idle connections.
	//
	// MaxIdleConns sets the maximum number of idle connections in the main connection pool.
	MaxIdleConns int
	// ConnMaxLifetime sets the maximum lifetime for pooled connections.
	//
	// ConnMaxLifetime sets the maximum life cycle of pooled connections.
	ConnMaxLifetime time.Duration
	// ConnectTimeout bounds the initial database ping in Open.
	//
	// ConnectTimeout limits the time to first ping the database in Open.
	ConnectTimeout time.Duration
	// HeavyReadConcurrency bounds simultaneous historical batch scans for every
	// backend. Zero keeps the compatibility default of one heavy scan at a time.
	HeavyReadConcurrency int
	// SQLite holds SQLite-specific options.
	//
	// SQLite holds SQLite-specific options.
	SQLite SQLiteOptions

	// RollupPolicy configures downsampling tiers and tiered retention. When it
	// defines tiers, Store.Compact materializes them and enforces every
	// retention window. The zero value disables rollups (raw points only).
	//
	// RollupPolicy configures the downsampling level and tiered retention time; after defining the level,
	// Store.Compact generates rollups and enforces retention policies. A value of zero disables rollup.
	RollupPolicy RollupPolicy

	// MigrationProgress receives best-effort progress snapshots while an
	// automatic schema migration is running. Callbacks must return quickly and
	// must not call back into the Store being opened.
	MigrationProgress MigrationProgressFunc
}

// SQLiteOptions contains SQLite-specific performance and read-concurrency settings.
//
// SQLiteOptions holds SQLite-specific performance and concurrent read options.
type SQLiteOptions struct {
	// PerformanceProfile selects the SQLite synchronous durability preset.
	//
	// PerformanceProfile selects the SQLite synchronous persistence preset.
	PerformanceProfile SQLitePerformanceProfile
	// BusyTimeout is applied to SQLite busy_timeout.
	//
	// BusyTimeout applies to SQLite busy_timeout.
	BusyTimeout time.Duration
	// CacheSizeKB sets the SQLite page cache size in KB.
	//
	// CacheSizeKB sets the SQLite page cache size in KB.
	CacheSizeKB int
	// ReadCacheSizeKB sets the page-cache budget for each dedicated read
	// connection. Zero reuses CacheSizeKB for backward compatibility.
	ReadCacheSizeKB int
	// PageSize sets SQLite page_size when positive.
	//
	// Sets SQLite page_size when PageSize is positive.
	PageSize int
	// TempStoreMemory enables memory-backed temporary storage.
	//
	// TempStoreMemory enables memory-based temporary storage.
	TempStoreMemory bool
	// MMapSizeBytes sets SQLite mmap_size in bytes.
	//
	// MMapSizeBytes sets the SQLite mmap_size in bytes.
	MMapSizeBytes int64
	// WALAutoCheckpoint sets the SQLite WAL auto-checkpoint page count.
	//
	// WALAutoCheckpoint sets the number of SQLite WAL automatic checkpoint pages.
	WALAutoCheckpoint int
	// JournalSizeLimitBytes caps the WAL file retained after checkpoints.
	//
	// JournalSizeLimitBytes Limits the size of SQLite WAL files retained after checkpoint.
	JournalSizeLimitBytes int64
	// ReadPoolSize, when > 0, opens a second read-only connection pool with
	// this many connections for SELECT-style calls. SQLite serializes writes,
	// but WAL lets readers run concurrently, so a read pool lifts read
	// throughput while keeping writes on the single primary connection.
	// 0 (default) means all calls share the single primary pool, preserving
	// the previous single-connection behavior.
	//
	// ReadPoolSize greater than 0 opens a separate read-only connection pool for SELECT class calls.
	// SQLite writes are serialized, but WAL allows reads to execute concurrently, so the read pool can run while writes are still running.
	// Improve read throughput while using a single primary connection. 0 (default) means all calls share a single main connection pool,
	// Preserve the previous single connection behavior.
	ReadPoolSize int
	// HeavyReadConcurrency bounds decode-heavy historical Series calls. Zero
	// derives the limit from ReadPoolSize and one disables parallel heavy reads.
	HeavyReadConcurrency int
}

// SQLitePerformanceProfile names SQLite durability and performance presets.
//
// SQLitePerformanceProfile represents SQLite synchronous and other persistence policy presets.
type SQLitePerformanceProfile string

const (
	// SQLiteProfileDefault uses the package default SQLite profile.
	//
	// SQLiteProfileDefault uses the package's default SQLite profile.
	SQLiteProfileDefault SQLitePerformanceProfile = ""
	// SQLiteProfileBalanced uses NORMAL synchronous mode.
	//
	// SQLiteProfileBalanced uses NORMAL synchronous mode.
	SQLiteProfileBalanced SQLitePerformanceProfile = "balanced"
	// SQLiteProfilePerformance favors throughput over durability.
	//
	// SQLiteProfilePerformance prioritizes throughput over durability.
	SQLiteProfilePerformance SQLitePerformanceProfile = "performance"
	// SQLiteProfileDurable favors durability over write throughput.
	//
	// SQLiteProfileDurable prioritizes durability over write throughput.
	SQLiteProfileDurable SQLitePerformanceProfile = "durable"
)

// Option mutates a Config value.
//
// Option is a functional option used to modify Config.
type Option func(*Config)

// DefaultConfig returns the default configuration for a backend.
//
// DefaultConfig Returns the default configuration for the specified backend.
func DefaultConfig(driver Driver, dsn string) Config {
	return Config{
		Driver:          driver,
		DSN:             dsn,
		TablePrefix:     "metric_",
		AutoMigrate:     true,
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
		ConnectTimeout:  10 * time.Second,
		SQLite: SQLiteOptions{
			PerformanceProfile:    SQLiteProfileBalanced,
			BusyTimeout:           5 * time.Second,
			CacheSizeKB:           64 * 1024,
			TempStoreMemory:       true,
			MMapSizeBytes:         256 * 1024 * 1024,
			WALAutoCheckpoint:     256,
			JournalSizeLimitBytes: 1024 * 1024,
		},
	}
}

// Backend builds a generic backend configuration and applies options.
//
// Backend Constructs a generic backend configuration and applies additional options.
func Backend(driver Driver, dsn string, opts ...Option) Config {
	cfg := DefaultConfig(driver, dsn)
	applyOptions(&cfg, opts...)
	return cfg
}

// SQLite builds a SQLite backend configuration.
//
// SQLite constructs SQLite backend configuration.
func SQLite(dsn string, opts ...Option) Config {
	cfg := SQLiteConfig(dsn)
	applyOptions(&cfg, opts...)
	return cfg
}

// SQLiteInDir builds a managed SQLite file configuration rooted at a directory.
//
// SQLiteInDir Constructs the SQLite file database configuration of the directory managed by the package.
func SQLiteInDir(dir string, opts ...Option) Config {
	cfg := SQLiteConfig(sqliteFileDSN(filepath.Join(dir, "metrics.db")))
	cfg.MaxOpenConns = 1
	cfg.MaxIdleConns = 1
	applyOptions(&cfg, opts...)
	return cfg
}

// MySQL builds a MySQL backend configuration.
//
// MySQL constructs the MySQL backend configuration.
func MySQL(dsn string, opts ...Option) Config {
	cfg := MySQLConfig(dsn)
	applyOptions(&cfg, opts...)
	return cfg
}

// PostgreSQL builds a PostgreSQL backend configuration.
//
// PostgreSQL constructs the PostgreSQL backend configuration.
func PostgreSQL(dsn string, opts ...Option) Config {
	cfg := PostgreSQLConfig(dsn)
	applyOptions(&cfg, opts...)
	return cfg
}

// SQLiteConfig returns the default SQLite configuration.
//
// SQLiteConfig Returns the default configuration for the SQLite backend.
func SQLiteConfig(dsn string) Config {
	cfg := DefaultConfig(DriverSQLite, dsn)
	if cfg.DSN == "" {
		cfg.DSN = "file:metric?mode=memory&cache=shared"
	}
	// SQLite serializes writes, so a single connection is the safe default and
	// avoids "database is locked" contention. Callers can still override with
	// WithMaxOpenConns/WithMaxIdleConns. Setting it here (rather than inferring
	// it later from the 25/5 defaults) means an explicit WithMaxOpenConns(25)
	// is honored instead of being mistaken for "unset".
	cfg.MaxOpenConns = 1
	cfg.MaxIdleConns = 1
	return cfg
}

// MySQLConfig returns the default MySQL configuration.
//
// MySQLConfig Returns the default configuration for the MySQL backend.
func MySQLConfig(dsn string) Config {
	return DefaultConfig(DriverMySQL, dsn)
}

// PostgreSQLConfig returns the default PostgreSQL configuration.
//
// PostgreSQLConfig Returns the default configuration for the PostgreSQL backend.
func PostgreSQLConfig(dsn string) Config {
	return DefaultConfig(DriverPostgreSQL, dsn)
}

// WithDB sets a caller-owned database connection pool.
//
// WithDB sets the database connection pool provided by the caller.
func WithDB(db *sql.DB) Option {
	return func(c *Config) {
		c.DB = db
	}
}

// WithTablePrefix sets the table-name prefix used by metric tables.
//
// WithTablePrefix sets the table name prefix of the metric private table.
func WithTablePrefix(prefix string) Option {
	return func(c *Config) {
		c.TablePrefix = prefix
	}
}

// WithAutoMigrate controls whether Open creates the schema automatically.
//
// WithAutoMigrate controls whether the table structure is automatically created when Open.
func WithAutoMigrate(enabled bool) Option {
	return func(c *Config) {
		c.AutoMigrate = enabled
	}
}

// WithMigrationProgress observes automatic schema migration progress.
func WithMigrationProgress(progress MigrationProgressFunc) Option {
	return func(c *Config) {
		c.MigrationProgress = progress
	}
}

// WithMaxOpenConns sets the maximum number of open database connections.
//
// WithMaxOpenConns sets the maximum number of open connections for the underlying database connection pool.
func WithMaxOpenConns(n int) Option {
	return func(c *Config) {
		c.MaxOpenConns = n
	}
}

// WithMaxIdleConns sets the maximum number of idle database connections.
//
// WithMaxIdleConns sets the maximum number of idle connections in the underlying database connection pool.
func WithMaxIdleConns(n int) Option {
	return func(c *Config) {
		c.MaxIdleConns = n
	}
}

// WithConnMaxLifetime sets the maximum lifetime for pooled connections.
//
// WithConnMaxLifetime sets the maximum connection reuse time.
func WithConnMaxLifetime(d time.Duration) Option {
	return func(c *Config) {
		c.ConnMaxLifetime = d
	}
}

// WithConnectTimeout sets the timeout used while opening the store.
//
// WithConnectTimeout sets the timeout for pinging the database when opening the Store.
func WithConnectTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.ConnectTimeout = d
	}
}

// WithSQLiteProfile sets the SQLite durability and performance profile.
//
// WithSQLiteProfile sets the SQLite persistence performance profile.
func WithSQLiteProfile(profile SQLitePerformanceProfile) Option {
	return func(c *Config) {
		c.SQLite.PerformanceProfile = profile
	}
}

// WithSQLiteBusyTimeout sets SQLite busy_timeout.
//
// WithSQLiteBusyTimeout sets SQLite busy_timeout.
func WithSQLiteBusyTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.SQLite.BusyTimeout = d
	}
}

// WithSQLiteCacheSizeKB sets the SQLite page cache size in KB.
//
// WithSQLiteCacheSizeKB sets the SQLite page cache size in KB.
func WithSQLiteCacheSizeKB(kb int) Option {
	return func(c *Config) {
		c.SQLite.CacheSizeKB = kb
	}
}

// WithSQLiteReadCacheSizeKB sets the per-connection cache for the read pool.
func WithSQLiteReadCacheSizeKB(kb int) Option {
	return func(c *Config) {
		c.SQLite.ReadCacheSizeKB = kb
	}
}

// WithSQLiteMMapSize sets SQLite mmap_size.
//
// WithSQLiteMMapSize sets SQLite mmap_size.
func WithSQLiteMMapSize(bytes int64) Option {
	return func(c *Config) {
		c.SQLite.MMapSizeBytes = bytes
	}
}

// WithSQLitePageSize sets SQLite page_size.
//
// WithSQLitePageSize sets the SQLite page_size.
func WithSQLitePageSize(bytes int) Option {
	return func(c *Config) {
		c.SQLite.PageSize = bytes
	}
}

// WithSQLiteTempStoreMemory controls whether SQLite uses memory for temporary storage.
//
// WithSQLiteTempStoreMemory sets whether SQLite uses memory for temporary storage.
func WithSQLiteTempStoreMemory(enabled bool) Option {
	return func(c *Config) {
		c.SQLite.TempStoreMemory = enabled
	}
}

// WithSQLiteWALAutoCheckpoint sets the SQLite WAL auto-checkpoint page count.
//
// WithSQLiteWALAutoCheckpoint sets the number of SQLite WAL automatic checkpoint pages.
func WithSQLiteWALAutoCheckpoint(pages int) Option {
	return func(c *Config) {
		c.SQLite.WALAutoCheckpoint = pages
	}
}

// WithSQLiteJournalSizeLimit sets the WAL size retained after checkpoints.
func WithSQLiteJournalSizeLimit(bytes int64) Option {
	return func(c *Config) {
		c.SQLite.JournalSizeLimitBytes = bytes
	}
}

// WithSQLiteReadPool enables a dedicated read-only connection pool of n
// connections for SQLite. Writes stay on the single primary connection
// (SQLite serializes them); reads fan out across the pool, which WAL mode
// allows to run concurrently. Pass n <= 1 to disable (the default).
//
// WithSQLiteReadPool Enables an independent read-only connection pool of n connections for SQLite. Write still goes
// Single master connection (SQLite serializes writes); reads are spread across this connection pool, WAL mode allows them
// Concurrent execution. Passing n <= 1 disables this feature (default behavior).
func WithSQLiteReadPool(n int) Option {
	return func(c *Config) {
		c.SQLite.ReadPoolSize = n
	}
}

// WithSQLiteHeavyReadConcurrency limits simultaneous historical decodes.
func WithSQLiteHeavyReadConcurrency(n int) Option {
	return func(c *Config) {
		c.SQLite.HeavyReadConcurrency = n
		c.HeavyReadConcurrency = n
	}
}

// WithHeavyReadConcurrency limits simultaneous historical scans on every
// backend. It does not split one scan into per-node goroutines.
func WithHeavyReadConcurrency(n int) Option {
	return func(c *Config) {
		c.HeavyReadConcurrency = n
	}
}

// WithRollupPolicy sets the downsampling/retention ladder. Compact uses it to
// build rollup tiers (each progressively coarser and longer-lived) and to age
// out raw points and expired tiers.
//
// WithRollupPolicy sets the downsampling and retention time ladder; Compact will build the rolling
// Coarser, longer-lasting rollups, and cleans up expired origins and expired levels.
func WithRollupPolicy(p RollupPolicy) Option {
	return func(c *Config) {
		c.RollupPolicy = p
	}
}

// applyOptions applies configuration options in order.
//
// applyOptions applies a set of options to a Config in sequence.
func applyOptions(cfg *Config, opts ...Option) {
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}
}

// Validate checks whether the value is well formed.
//
// Validate checks whether the Config's backend, table name prefix, retention time, and rollup policy are legal.
func (c Config) Validate() error {
	switch c.Driver {
	case DriverSQLite, DriverMySQL, DriverPostgreSQL:
	default:
		return fmt.Errorf("%w: unsupported driver %q", ErrInvalidArgument, c.Driver)
	}
	if c.DB == nil && strings.TrimSpace(c.DSN) == "" {
		return fmt.Errorf("%w: dsn is required when db is not supplied", ErrInvalidArgument)
	}
	if c.TablePrefix == "" {
		return fmt.Errorf("%w: table prefix cannot be empty", ErrInvalidArgument)
	}
	for _, r := range c.TablePrefix {
		if !(r == '_' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z') {
			return fmt.Errorf("%w: table prefix must contain only letters, digits, and underscores", ErrInvalidArgument)
		}
	}
	if err := c.RollupPolicy.Validate(); err != nil {
		return err
	}
	return nil
}

// driverName returns the database/sql driver name for the configured backend.
//
// driverName returns the driver name registered by database/sql.
func (c Config) driverName() string {
	switch c.Driver {
	case DriverSQLite:
		return "sqlite3"
	case DriverMySQL:
		return "mysql"
	case DriverPostgreSQL:
		return "pgx"
	default:
		return string(c.Driver)
	}
}

// sqliteFileDSN converts a filesystem path into a SQLite file DSN.
//
// sqliteFileDSN Converts a file path to SQLite file: DSN.
func sqliteFileDSN(path string) string {
	// WAL already lets independent connection caches read concurrently with the
	// single writer. SQLite shared-cache mode adds table-level locks for which
	// busy_timeout is ineffective, causing avoidable SQLITE_LOCKED errors while
	// V4 blocks are sealed.
	return "file:" + filepath.ToSlash(path) + "?mode=rwc&_txlock=immediate"
}

// appendSQLiteDSNParam appends a query parameter to a SQLite DSN.
//
// appendSQLiteDSNParam Appends query parameters to a SQLite DSN.
func appendSQLiteDSNParam(dsn, key, value string) string {
	if strings.Contains(dsn, "?") {
		return dsn + "&" + url.QueryEscape(key) + "=" + url.QueryEscape(value)
	}
	return dsn + "?" + url.QueryEscape(key) + "=" + url.QueryEscape(value)
}

func setSQLiteDSNParam(dsn, key, value string) string {
	base, rawQuery, found := strings.Cut(dsn, "?")
	if !found {
		return base + "?" + url.QueryEscape(key) + "=" + url.QueryEscape(value)
	}
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		return appendSQLiteDSNParam(dsn, key, value)
	}
	query.Set(key, value)
	return base + "?" + query.Encode()
}
