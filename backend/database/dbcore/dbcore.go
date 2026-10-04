package dbcore

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/pkg/migrations"
	logger "github.com/komari-monitor/komari/utils/log"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// zipDirectoryExcluding packs srcDir into dstZip, exclude is a set of absolute paths that need to be excluded
func zipDirectoryExcluding(srcDir, dstZip string, exclude map[string]struct{}) error {
	// Normalize excluded paths to absolute paths
	normExclude := make(map[string]struct{}, len(exclude))
	for p := range exclude {
		abs, _ := filepath.Abs(p)
		normExclude[abs] = struct{}{}
	}

	out, err := os.Create(dstZip)
	if err != nil {
		return err
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	defer zw.Close()

	absSrc, _ := filepath.Abs(srcDir)
	walkErr := filepath.Walk(absSrc, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Exclude backup.zip itself
		if _, ok := normExclude[path]; ok {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// Calculate relative paths within zip
		rel, err := filepath.Rel(absSrc, path)
		if err != nil {
			return err
		}
		// root directory skip
		if rel == "." {
			return nil
		}
		// Replace with forward slash
		zipName := filepath.ToSlash(rel)

		if info.IsDir() {
			_, err := zw.Create(zipName + "/")
			return err
		}
		// Ordinary document
		fh, err := os.Open(path)
		if err != nil {
			return err
		}
		w, err := zw.Create(zipName)
		if err != nil {
			fh.Close()
			return err
		}
		if _, err := io.Copy(w, fh); err != nil {
			fh.Close()
			return err
		}
		fh.Close()
		return nil
	})
	if walkErr != nil {
		return walkErr
	}
	return zw.Close()
}

// removeAllInDirExcept deletes all files and folders under dir except the absolute path specified by exclude
func removeAllInDirExcept(dir string, exclude map[string]struct{}) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	normExclude := make(map[string]struct{}, len(exclude))
	for p := range exclude {
		abs, _ := filepath.Abs(p)
		normExclude[abs] = struct{}{}
	}
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		full := filepath.Join(absDir, e.Name())
		if _, ok := normExclude[full]; ok {
			continue
		}
		if err := os.RemoveAll(full); err != nil {
			return err
		}
	}
	return nil
}

// unzipToDir Unzip zipPath to dstDir, including path traversal protection
func unzipToDir(zipPath, dstDir string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}
	absDst, _ := filepath.Abs(dstDir)

	for _, f := range zr.File {
		// Construct the target path and perform path traversal protection
		cleanName := filepath.Clean(f.Name)
		targetPath := filepath.Join(absDst, cleanName)
		if !strings.HasPrefix(targetPath, absDst+string(os.PathSeparator)) && targetPath != absDst {
			return fmt.Errorf("illegal file path in zip: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(targetPath)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return err
		}
		out.Close()
		rc.Close()
	}
	return nil
}

func validateRestoredSQLite(path, label string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open restored %s: %w", label, err)
	}
	defer file.Close()
	header := make([]byte, 16)
	if _, err := io.ReadFull(file, header); err != nil {
		return fmt.Errorf("read restored %s header: %w", label, err)
	}
	if string(header) != "SQLite format 3\x00" {
		return fmt.Errorf("restored %s is not a valid SQLite database", label)
	}
	return nil
}

// restoreStagedBackup extracts and validates the entire archive before it
// replaces data/. Directory renames keep the previous data intact if the new
// package cannot be prepared or published.
type stagedRestore struct {
	dataDir         string
	previousDir     string
	stageDir        string
	preRestorePath  string
	pendingMarker   string
	committedMarker string
	finished        bool
	mu              sync.Mutex
}

const (
	restorePendingMarkerName   = ".komari-restore-pending.json"
	restoreCommittedMarkerName = ".komari-restore-committed"
)

type restoreJournal struct {
	DataDir     string `json:"data_dir"`
	PreviousDir string `json:"previous_dir"`
	StageDir    string `json:"stage_dir"`
}

func restoreMarkerPaths(dataDir string) (string, string, string, error) {
	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return "", "", "", fmt.Errorf("resolve restore data directory: %w", err)
	}
	parent := filepath.Dir(absDataDir)
	return absDataDir,
		filepath.Join(parent, restorePendingMarkerName),
		filepath.Join(parent, restoreCommittedMarkerName),
		nil
}

func writeRestoreMarker(path string, content []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".komari-restore-marker-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(content); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func resolveRestoreJournalPath(parent, name, prefix string) (string, error) {
	if name == "" || filepath.Base(name) != name || !strings.HasPrefix(name, prefix) {
		return "", fmt.Errorf("invalid restore journal path %q", name)
	}
	return filepath.Join(parent, name), nil
}

func readRestoreJournal(path, dataDir string) (restoreJournal, string, string, error) {
	var journal restoreJournal
	content, err := os.ReadFile(path)
	if err != nil {
		return journal, "", "", err
	}
	if err := json.Unmarshal(content, &journal); err != nil {
		return journal, "", "", fmt.Errorf("decode interrupted restore journal: %w", err)
	}
	if journal.DataDir != filepath.Base(dataDir) {
		return journal, "", "", fmt.Errorf("restore journal data directory %q does not match %q", journal.DataDir, filepath.Base(dataDir))
	}
	parent := filepath.Dir(dataDir)
	previousDir, err := resolveRestoreJournalPath(parent, journal.PreviousDir, ".komari-restore-old-")
	if err != nil {
		return journal, "", "", err
	}
	stageDir, err := resolveRestoreJournalPath(parent, journal.StageDir, ".komari-restore-")
	if err != nil || strings.HasPrefix(journal.StageDir, ".komari-restore-old-") {
		if err == nil {
			err = fmt.Errorf("invalid restore stage path %q", journal.StageDir)
		}
		return journal, "", "", err
	}
	return journal, previousDir, stageDir, nil
}

func removeRestoreMarkers(pendingMarker, committedMarker string) {
	for _, marker := range []string{pendingMarker, committedMarker} {
		if err := os.Remove(marker); err != nil && !os.IsNotExist(err) {
			logger.Errorf("dbcore", "[restore] failed to remove transaction marker %s: %v", marker, err)
		}
	}
}

func retireInterruptedRestoreArchive(dataDir string) {
	archivePath := filepath.Join(dataDir, "backup.zip")
	failedPath := filepath.Join(dataDir, "backup.interrupted-"+time.Now().UTC().Format("20060102-150405.000000000")+".zip")
	if err := os.Rename(archivePath, failedPath); err != nil && !os.IsNotExist(err) {
		logger.Errorf("dbcore", "[restore] previous data restored but interrupted package could not be renamed: %v", err)
	}
}

func recoverInterruptedRestore(dataDir string) error {
	absDataDir, pendingMarker, committedMarker, err := restoreMarkerPaths(dataDir)
	if err != nil {
		return err
	}
	if _, err := os.Stat(pendingMarker); err != nil {
		if os.IsNotExist(err) {
			if removeErr := os.Remove(committedMarker); removeErr != nil && !os.IsNotExist(removeErr) {
				return fmt.Errorf("remove completed restore marker: %w", removeErr)
			}
			return nil
		}
		return fmt.Errorf("inspect interrupted restore marker: %w", err)
	}

	_, previousDir, stageDir, err := readRestoreJournal(pendingMarker, absDataDir)
	if err != nil {
		return err
	}
	if _, err := os.Stat(committedMarker); err == nil {
		if err := os.RemoveAll(previousDir); err != nil {
			return fmt.Errorf("finish committed restore cleanup: %w", err)
		}
		if err := os.RemoveAll(stageDir); err != nil {
			return fmt.Errorf("remove committed restore staging directory: %w", err)
		}
		removeRestoreMarkers(pendingMarker, committedMarker)
		logger.Infof("dbcore", "[restore] completed cleanup for a restore committed before interruption")
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect committed restore marker: %w", err)
	}

	if _, err := os.Stat(previousDir); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("inspect previous restore directory: %w", err)
		}
		if _, err := os.Stat(absDataDir); err != nil {
			return fmt.Errorf("restore transaction is incomplete and neither active nor previous data is available: %w", err)
		}
		if err := os.RemoveAll(stageDir); err != nil {
			return fmt.Errorf("remove unpublished restore staging directory: %w", err)
		}
		removeRestoreMarkers(pendingMarker, committedMarker)
		return nil
	}

	failedDir, err := os.MkdirTemp(filepath.Dir(absDataDir), ".komari-restore-interrupted-*")
	if err != nil {
		return fmt.Errorf("reserve interrupted restore path: %w", err)
	}
	if err := os.RemoveAll(failedDir); err != nil {
		return fmt.Errorf("prepare interrupted restore path: %w", err)
	}
	activeMoved := false
	if _, err := os.Stat(absDataDir); err == nil {
		if err := os.Rename(absDataDir, failedDir); err != nil {
			return fmt.Errorf("move interrupted restored data aside: %w", err)
		}
		activeMoved = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect interrupted restored data: %w", err)
	}
	if err := os.Rename(previousDir, absDataDir); err != nil {
		if activeMoved {
			_ = os.Rename(failedDir, absDataDir)
		}
		return fmt.Errorf("recover previous data after interrupted restore: %w", err)
	}
	retireInterruptedRestoreArchive(absDataDir)
	if activeMoved {
		if err := os.RemoveAll(failedDir); err != nil {
			logger.Errorf("dbcore", "[restore] previous data recovered but interrupted data could not be removed: %v", err)
		}
	}
	if err := os.RemoveAll(stageDir); err != nil {
		logger.Errorf("dbcore", "[restore] previous data recovered but staging directory could not be removed: %v", err)
	}
	removeRestoreMarkers(pendingMarker, committedMarker)
	logger.Infof("dbcore", "[restore] interrupted startup detected; previous data recovered")
	return nil
}

func (r *stagedRestore) Commit() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.finished {
		return nil
	}
	if err := writeRestoreMarker(r.committedMarker, []byte("committed\n")); err != nil {
		return fmt.Errorf("persist committed restore state: %w", err)
	}
	r.finished = true
	if err := os.RemoveAll(r.previousDir); err != nil {
		logger.Errorf("dbcore", "[restore] backup was verified and committed, but the previous staging directory could not be removed: %v", err)
		return nil
	}
	removeRestoreMarkers(r.pendingMarker, r.committedMarker)
	logger.Infof("dbcore", "[restore] backup verified and committed; previous data saved to %s", r.preRestorePath)
	return nil
}

func (r *stagedRestore) Rollback() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.finished {
		return nil
	}
	failedDir, err := os.MkdirTemp(filepath.Dir(r.dataDir), ".komari-restore-failed-*")
	if err != nil {
		return fmt.Errorf("reserve failed restore path: %w", err)
	}
	if err := os.RemoveAll(failedDir); err != nil {
		return fmt.Errorf("prepare failed restore path: %w", err)
	}
	if err := os.Rename(r.dataDir, failedDir); err != nil {
		return fmt.Errorf("move failed restored data aside: %w", err)
	}
	if err := os.Rename(r.previousDir, r.dataDir); err != nil {
		_ = os.Rename(failedDir, r.dataDir)
		return fmt.Errorf("restore previous data directory: %w", err)
	}
	failedArchive := filepath.Join(r.dataDir, "backup.failed-"+time.Now().UTC().Format("20060102-150405")+".zip")
	if err := os.Rename(filepath.Join(r.dataDir, "backup.zip"), failedArchive); err != nil && !os.IsNotExist(err) {
		logger.Errorf("dbcore", "[restore] previous data restored but failed package could not be renamed: %v", err)
	}
	if err := os.RemoveAll(failedDir); err != nil {
		logger.Errorf("dbcore", "[restore] previous data restored but failed staging directory could not be removed: %v", err)
	}
	removeRestoreMarkers(r.pendingMarker, r.committedMarker)
	r.finished = true
	logger.Infof("dbcore", "[restore] startup validation failed; previous data restored")
	return nil
}

func restoreStagedBackup(dataDir string) (*stagedRestore, error) {
	if err := recoverInterruptedRestore(dataDir); err != nil {
		return nil, fmt.Errorf("recover interrupted restore: %w", err)
	}
	backupZipPath := filepath.Join(dataDir, "backup.zip")
	if _, err := os.Stat(backupZipPath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("inspect staged backup: %w", err)
	}

	stageDir, err := os.MkdirTemp(filepath.Dir(dataDir), ".komari-restore-*")
	if err != nil {
		return nil, fmt.Errorf("create restore staging directory: %w", err)
	}
	stagePublished := false
	defer func() {
		if !stagePublished {
			_ = os.RemoveAll(stageDir)
		}
	}()
	if err := unzipToDir(backupZipPath, stageDir); err != nil {
		return nil, fmt.Errorf("extract backup into staging directory: %w", err)
	}
	if err := validateRestoredSQLite(filepath.Join(stageDir, "komari.db"), "komari.db"); err != nil {
		return nil, err
	}
	metricsPath := filepath.Join(stageDir, "metrics.db")
	if _, err := os.Stat(metricsPath); err == nil {
		if err := validateRestoredSQLite(metricsPath, "metrics.db"); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("inspect restored metrics.db: %w", err)
	}
	_ = os.Remove(filepath.Join(stageDir, "komari-backup-markup"))
	if err := os.MkdirAll(filepath.Join(stageDir, "theme"), 0o755); err != nil {
		return nil, fmt.Errorf("prepare restored theme directory: %w", err)
	}

	backupDir := filepath.Join(filepath.Dir(dataDir), "backup")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return nil, fmt.Errorf("create pre-restore backup directory: %w", err)
	}
	if err := os.Chmod(backupDir, 0o700); err != nil {
		return nil, fmt.Errorf("harden pre-restore backup directory: %w", err)
	}
	timestamp := time.Now().UTC().Format("20060102-150405")
	preRestorePath := filepath.Join(backupDir, fmt.Sprintf("pre-restore-%s.zip", timestamp))
	if err := zipDirectoryExcluding(dataDir, preRestorePath, map[string]struct{}{backupZipPath: {}}); err != nil {
		return nil, fmt.Errorf("preserve current data before restore: %w", err)
	}

	oldDir, err := os.MkdirTemp(filepath.Dir(dataDir), ".komari-restore-old-*")
	if err != nil {
		return nil, fmt.Errorf("reserve previous data path: %w", err)
	}
	if err := os.RemoveAll(oldDir); err != nil {
		return nil, fmt.Errorf("prepare previous data path: %w", err)
	}
	absDataDir, pendingMarker, committedMarker, err := restoreMarkerPaths(dataDir)
	if err != nil {
		return nil, err
	}
	journal := restoreJournal{
		DataDir:     filepath.Base(absDataDir),
		PreviousDir: filepath.Base(oldDir),
		StageDir:    filepath.Base(stageDir),
	}
	journalData, err := json.Marshal(journal)
	if err != nil {
		return nil, fmt.Errorf("encode restore transaction journal: %w", err)
	}
	if err := writeRestoreMarker(pendingMarker, journalData); err != nil {
		return nil, fmt.Errorf("persist restore transaction journal: %w", err)
	}
	if err := os.Rename(dataDir, oldDir); err != nil {
		removeRestoreMarkers(pendingMarker, committedMarker)
		return nil, fmt.Errorf("move current data aside: %w", err)
	}
	if err := os.Rename(stageDir, dataDir); err != nil {
		rollbackErr := os.Rename(oldDir, dataDir)
		if rollbackErr != nil {
			return nil, fmt.Errorf("publish restored data: %v; restore previous data: %w", err, rollbackErr)
		}
		removeRestoreMarkers(pendingMarker, committedMarker)
		return nil, fmt.Errorf("publish restored data: %w", err)
	}
	stagePublished = true
	logger.Infof("dbcore", "[restore] backup published for startup validation; previous data remains available")
	return &stagedRestore{
		dataDir:         dataDir,
		previousDir:     oldDir,
		stageDir:        stageDir,
		preRestorePath:  preRestorePath,
		pendingMarker:   pendingMarker,
		committedMarker: committedMarker,
	}, nil
}

var (
	instance       *gorm.DB
	once           sync.Once
	initErr        error
	pendingRestore *stagedRestore
)

// SystemVersionKey is the configuration key (stored in the configs table) that records the "last boot version identifier".
// Replaces the old ./data/.komari-version file: the version identification is backed up/restored along with the configuration repository,
// Also avoids additional bare file dependencies.
const SystemVersionKey = "system_version"

// versionID is the version ID of the current build, injected by SetVersionID before Initialize.
var versionID string

// dbFileExistedAtStartup records whether komari.db already exists before this process starts and the database is opened.
// Used to distinguish between "fresh installation" and "upgrade from an older version (no version tag)". Open the database in doInitialize
// collected before.
var dbFileExistedAtStartup bool

// SetVersionID sets the version ID of the current build (usually CurrentVersion+"-"+VersionHash),
// Used for version upgrade detection and automatic backup. Should be called before Initialize(); empty to skip upgrade backup.
func SetVersionID(id string) {
	versionID = id
}

// resolveDatabaseFile Returns the path to the currently used SQLite database file.
func resolveDatabaseFile() string {
	dbFile := flags.DatabaseFile
	if dbFile == "" {
		dbFile = "./data/komari.db"
	}
	return dbFile
}

// backupOnVersionUpgrade When detecting a version upgrade, package the current ./data into
// ./backup/upgrade-{time}.zip, to facilitate rollback when upgrade (including metrics migration) is abnormal.
//
// The version identifier is stored in the configuration database (configs table, key system_version), so this function must be in
// Called after config.SetDb and before one-time metrics migration (InitStores).
//
// Trigger rules:
//   - versionID is empty: skip (no version is injected, such as some test scenarios).
//   - There is no version in the configuration and no database file before startup: fresh installation, only writing version, no backup.
//   - There is no version in the configuration but there is a database file before startup: upgrade from the old stable version without version mark and back up.
//   - The version in the configuration is different from the current version: version upgrade, backup.
//   - The version in the configuration is consistent with the current version: no backup is required.
//
// Backup failure does not prevent startup, but prints clear errors; the version is written/updated after a successful backup (or no backup required).
func backupOnVersionUpgrade() {
	if versionID == "" {
		return
	}

	prevVersion, readErr := config.GetAs[string](SystemVersionKey)
	prevVersion = strings.TrimSpace(prevVersion)
	versionRecorded := readErr == nil && prevVersion != ""

	// The version has not changed and no backup is required.
	if versionRecorded && prevVersion == versionID {
		return
	}

	// New installation: There is no version in the configuration and there is no database file before startup. The version is written directly without backup.
	if !versionRecorded && !dbFileExistedAtStartup {
		writeVersionMarker()
		return
	}

	// Requires backup (upgrade or first version-tagged boot from an old stable version).
	// First do a WAL checkpoint to ensure that the main komari.db file contains the latest data.
	// Avoid backing up a database that is missing writes that remain in -wal.
	if instance != nil {
		instance.Exec("PRAGMA wal_checkpoint(TRUNCATE);")
	}

	if err := os.MkdirAll("./backup", 0755); err != nil {
		logger.Errorf("dbcore", "[upgrade-backup] failed to create backup dir: %v", err)
		return
	}
	tsName := time.Now().UTC().Format("20060102-150405")
	bakPath := filepath.Join("./backup", fmt.Sprintf("upgrade-%s.zip", tsName))
	backupZipPath := filepath.Join(".", "data", "backup.zip")
	if zipErr := zipDirectoryExcluding("./data", bakPath, map[string]struct{}{backupZipPath: {}}); zipErr != nil {
		logger.Errorf("dbcore", "[upgrade-backup] failed to backup ./data before upgrade (from %q to %q): %v", prevVersion, versionID, zipErr)
		return
	}
	logger.Infof("dbcore", "[upgrade-backup] ./data backed up to %s before upgrade (from %q to %q)", bakPath, prevVersion, versionID)

	writeVersionMarker()
}

// writeVersionMarker writes the current versionID to the configuration database.
func writeVersionMarker() {
	if err := config.Set(SystemVersionKey, versionID); err != nil {
		logger.Errorf("dbcore", "[upgrade-backup] failed to persist version marker: %v", err)
	}
}

func buildSQLiteDSN(databaseFile string) string {
	if databaseFile == "" {
		databaseFile = "./data/komari.db"
	}

	params := "_busy_timeout=5000&_txlock=immediate&_journal_mode=WAL&_synchronous=NORMAL"
	separator := "?"
	if strings.Contains(databaseFile, "?") {
		separator = "&"
	}

	if strings.HasPrefix(databaseFile, "file:") {
		return databaseFile + separator + params
	}

	if databaseFile == ":memory:" {
		return "file::memory:?cache=shared&" + params
	}

	return "file:" + filepath.ToSlash(databaseFile) + separator + params
}

// Initialize explicitly initializes the database connection and table structure and is only executed once.
// Unlike GetDBInstance, Initialize returns an error rather than exiting the process directly.
// It facilitates unified handling of errors in the startup life cycle and isolation in test/CLI commands.
func Initialize() error {
	once.Do(func() {
		initErr = doInitialize()
	})
	return initErr
}

// GetDBInstance returns the global database instance.
// In order to be compatible with the existing large number of call points, the semantics of "exit on error" are retained here;
// Startup processes that require error handling should call Initialize() first.
func GetDBInstance() *gorm.DB {
	if err := Initialize(); err != nil {
		logger.Fatalf("dbcore", "Failed to initialize database: %v", err)
	}
	return instance
}

// Close closes the underlying database connection for the shutdown process to call.
func Close() error {
	var closeErr error
	if instance != nil {
		sqlDB, err := instance.DB()
		if err != nil {
			closeErr = err
		} else {
			closeErr = sqlDB.Close()
		}
	}
	rollbackErr := RollbackPendingRestore()
	return errors.Join(closeErr, rollbackErr)
}

func CommitPendingRestore() error {
	if pendingRestore == nil {
		return nil
	}
	if err := pendingRestore.Commit(); err != nil {
		return err
	}
	pendingRestore = nil
	return nil
}

func RollbackPendingRestore() error {
	if pendingRestore == nil {
		return nil
	}
	err := pendingRestore.Rollback()
	if err == nil {
		pendingRestore = nil
	}
	return err
}

func doInitialize() error {
	var err error

	// Publish the staged package, but keep the previous directory until every
	// startup migration, including metric storage, has completed successfully.
	pendingRestore, err = restoreStagedBackup(filepath.Join(".", "data"))
	if err != nil {
		return fmt.Errorf("restore staged backup: %w", err)
	}
	initialized := false
	defer func() {
		if initialized || pendingRestore == nil {
			return
		}
		if instance != nil {
			if sqlDB, closeErr := instance.DB(); closeErr == nil {
				_ = sqlDB.Close()
			}
		}
		_ = RollbackPendingRestore()
	}()

	// Record whether komari.db already exists "before opening the database" to distinguish between new installations and old version upgrades.
	// Must be collected after (possible) recovery logic and before gorm.Open: recovery will decompress the old database,
	// gorm.Open will create an empty database.
	if _, statErr := os.Stat(resolveDatabaseFile()); statErr == nil {
		dbFileExistedAtStartup = true
	}

	logConfig := &gorm.Config{
		Logger:  logger.NewGormLogger(),
		NowFunc: func() time.Time { return time.Now().UTC() },
	}

	// Choose different connection methods according to database type
	switch flags.ApplyDatabaseTypeNormalization() {
	case flags.DatabaseTypeSQLite:
		// SQLite connection
		// Pass in parameters such as _busy_timeout / _txlock through DSN to ensure that every connection in the connection pool
		// All take effect:
		//   - _busy_timeout=5000: When encountering a write lock, wait up to 5s before returning to avoid instantaneous
		//     "database is locked" fails directly (only subsequent PRAGMA Exec only works on
		//     The single connection that executes this statement at that time will not take effect on other connections in the pool).
		//   - _txlock=immediate: Obtain the write lock at the beginning of the transaction to avoid "SELECT first and write later"
		//     Lock escalation produces a deadlock-like immediate SQLITE_BUSY under concurrent writes.
		//   - _journal_mode=WAL / _synchronous=NORMAL: consistent with PRAGMA below,
		//     Preset for all connections at the DSN level.
		dsn := buildSQLiteDSN(flags.DatabaseFile)
		instance, err = gorm.Open(sqlite.Open(dsn), logConfig)
		if err != nil {
			return fmt.Errorf("failed to connect to SQLite3 database: %w", err)
		}
		if sqlDB, dbErr := instance.DB(); dbErr == nil {
			// SQLite only allows one writer at a time; limiting the number of connections can avoid write competition at the connection pool level.
			// The load history will execute transactions including reading and writing every minute. If the connection pool allows multiple connections, it is easy to
			// Short writes such as ping results hit the lock and cause the entire batch of load records to be rolled back.
			sqlDB.SetMaxOpenConns(1)
			sqlDB.SetMaxIdleConns(1)
			sqlDB.SetConnMaxLifetime(0)
		} else {
			logger.Errorf("dbcore", "Failed to access underlying sql.DB for SQLite tuning: %v", dbErr)
		}
		instance.Exec("PRAGMA wal = ON;")
		if err := instance.Exec("PRAGMA journal_mode = WAL;").Error; err != nil {
			logger.Errorf("dbcore", "Failed to enable WAL mode for SQLite: %v", err)
		}
		instance.Exec("PRAGMA synchronous = NORMAL;")
		instance.Exec("PRAGMA busy_timeout = 5000;")
		instance.Exec("PRAGMA wal_autocheckpoint = 256;")
		instance.Exec("PRAGMA journal_size_limit = 1048576;")
		instance.Exec(fmt.Sprintf("PRAGMA cache_size = -%d;", mainDatabaseCacheSizeKB()))
		instance.Exec("PRAGMA temp_store = MEMORY;")
		instance.Exec("PRAGMA wal_checkpoint(TRUNCATE);")
	default:
		return fmt.Errorf("unsupported database type: %s (supported: %s)", flags.DatabaseType, flags.SupportedDatabaseTypes())
	}
	// Refuse databases produced by newer binaries before legacy compatibility
	// migrations have any opportunity to modify them.
	if err := migrations.CheckSchemaVersion(instance); err != nil {
		return fmt.Errorf("incompatible main database: %w", err)
	}
	if err := migrations.Run(migrations.Context{DB: instance}); err != nil {
		return fmt.Errorf("failed to run startup migrations: %w", err)
	}
	config.BindDb(instance)

	// Retain the existing upgrade backup sequence for legacy installs. The
	// standalone upgrader also snapshots the database before starting this binary.
	backupOnVersionUpgrade()

	// Main-schema creation and changes are applied only as numbered steps.
	// Legacy records are left intact for the separate metric-store migration.
	if err := migrations.RunVersioned(instance); err != nil {
		return fmt.Errorf("failed to migrate main database schema: %w", err)
	}
	if err := cleanupOrphanedClientData(instance); err != nil {
		return fmt.Errorf("failed to clean orphaned client data: %w", err)
	}

	initialized = true
	return nil
}

func cleanupOrphanedPingLossNotifications(db *gorm.DB) error {
	if err := db.Where(`
		NOT EXISTS (
			SELECT 1 FROM clients
			WHERE clients.uuid = ping_loss_notifications.client
		)
		OR NOT EXISTS (
			SELECT 1 FROM ping_tasks
			WHERE ping_tasks.id = ping_loss_notifications.task_id
		)`,
	).Delete(&models.PingLossNotification{}).Error; err != nil {
		return err
	}

	var pingTasks []models.PingTask
	if err := db.Select("id", "clients").Find(&pingTasks).Error; err != nil {
		return fmt.Errorf("list ping tasks for notification reconciliation: %w", err)
	}
	assignedClients := make(map[uint]map[string]struct{}, len(pingTasks))
	for _, task := range pingTasks {
		clients := make(map[string]struct{}, len(task.Clients))
		for _, client := range task.Clients {
			clients[client] = struct{}{}
		}
		assignedClients[task.Id] = clients
	}

	var notifications []models.PingLossNotification
	if err := db.Select("id", "client", "task_id").Find(&notifications).Error; err != nil {
		return fmt.Errorf("list ping loss notifications for assignment reconciliation: %w", err)
	}
	staleIDs := make([]uint, 0)
	for _, notification := range notifications {
		if _, assigned := assignedClients[notification.TaskId][notification.Client]; !assigned {
			staleIDs = append(staleIDs, notification.Id)
		}
	}
	if len(staleIDs) == 0 {
		return nil
	}
	return db.Where("id IN ?", staleIDs).Delete(&models.PingLossNotification{}).Error
}

func cleanupOrphanedClientData(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		for label, model := range map[string]any{
			"load notification states":        &models.LoadNotificationState{},
			"offline notifications":           &models.OfflineNotification{},
			"traffic report notifications":    &models.TrafficReportNotification{},
			"traffic daily ledger":            &models.TrafficDailyLedger{},
			"traffic calibration adjustments": &models.TrafficCalibrationAdjustment{},
		} {
			if !tx.Migrator().HasTable(model) {
				continue
			}
			if err := tx.Where(`NOT EXISTS (
				SELECT 1 FROM clients WHERE clients.uuid = client
			)`).Delete(model).Error; err != nil {
				return fmt.Errorf("delete orphaned %s: %w", label, err)
			}
		}
		if err := cleanupOrphanedPingLossNotifications(tx); err != nil {
			return err
		}
		var clients []models.Client
		if err := tx.Select("uuid").Find(&clients).Error; err != nil {
			return fmt.Errorf("list clients for orphan cleanup: %w", err)
		}
		validClients := make(map[string]struct{}, len(clients))
		for _, client := range clients {
			validClients[client.UUID] = struct{}{}
		}

		var pingTasks []models.PingTask
		if err := tx.Select("id", "clients").Find(&pingTasks).Error; err != nil {
			return fmt.Errorf("list ping tasks for orphan cleanup: %w", err)
		}
		for _, task := range pingTasks {
			remaining, changed := keepKnownClients(task.Clients, validClients)
			if changed {
				if err := tx.Model(&models.PingTask{}).Where("id = ?", task.Id).Update("clients", remaining).Error; err != nil {
					return fmt.Errorf("clean ping task %d clients: %w", task.Id, err)
				}
			}
		}

		var loadNotifications []models.LoadNotification
		if err := tx.Select("id", "clients").Find(&loadNotifications).Error; err != nil {
			return fmt.Errorf("list load notifications for orphan cleanup: %w", err)
		}
		if err := cleanupOrphanedLoadNotificationStates(tx, loadNotifications); err != nil {
			return err
		}
		for _, notification := range loadNotifications {
			remaining, changed := keepKnownClients(notification.Clients, validClients)
			if len(remaining) == 0 {
				if err := tx.Delete(&models.LoadNotification{}, notification.Id).Error; err != nil {
					return fmt.Errorf("delete empty load notification %d: %w", notification.Id, err)
				}
				continue
			}
			if !changed {
				continue
			}
			if err := tx.Model(&models.LoadNotification{}).Where("id = ?", notification.Id).Update("clients", remaining).Error; err != nil {
				return fmt.Errorf("clean load notification %d clients: %w", notification.Id, err)
			}
		}

		for _, table := range []string{"records", "records_long_term", "gpu_records", "ping_records"} {
			if !tx.Migrator().HasTable(table) {
				continue
			}
			predicate := fmt.Sprintf(`NOT EXISTS (
				SELECT 1 FROM clients WHERE clients.uuid = %s.client
			)`, table)
			if table == "ping_records" {
				predicate += ` OR NOT EXISTS (
					SELECT 1 FROM ping_tasks WHERE ping_tasks.id = ping_records.task_id
				)`
			}
			query := "DELETE FROM " + table + " WHERE " + predicate
			if err := tx.Exec(query).Error; err != nil {
				return fmt.Errorf("delete orphaned rows from legacy table %s: %w", table, err)
			}
		}
		return nil
	})
}

func cleanupOrphanedLoadNotificationStates(db *gorm.DB, notifications []models.LoadNotification) error {
	if !db.Migrator().HasTable(&models.LoadNotificationState{}) {
		return nil
	}
	assigned := make(map[uint]map[string]struct{}, len(notifications))
	for _, notification := range notifications {
		clients := make(map[string]struct{}, len(notification.Clients))
		for _, client := range notification.Clients {
			clients[client] = struct{}{}
		}
		assigned[notification.Id] = clients
	}
	var states []models.LoadNotificationState
	if err := db.Select("notification_id", "client").Find(&states).Error; err != nil {
		return fmt.Errorf("list load notification states for reconciliation: %w", err)
	}
	for _, state := range states {
		if _, ok := assigned[state.NotificationID][state.Client]; ok {
			continue
		}
		if err := db.Where("notification_id = ? AND client = ?", state.NotificationID, state.Client).
			Delete(&models.LoadNotificationState{}).Error; err != nil {
			return fmt.Errorf("delete orphaned load notification state: %w", err)
		}
	}
	return nil
}

func keepKnownClients(clients models.StringArray, valid map[string]struct{}) (models.StringArray, bool) {
	remaining := make(models.StringArray, 0, len(clients))
	changed := false
	for _, client := range clients {
		if _, ok := valid[client]; !ok {
			changed = true
			continue
		}
		remaining = append(remaining, client)
	}
	return remaining, changed
}
