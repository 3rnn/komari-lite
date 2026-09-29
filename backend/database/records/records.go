package records

import (
	"context"
	"time"

	"github.com/komari-monitor/komari/database/metricstore"
	"github.com/komari-monitor/komari/database/models"
)

// Historical monitoring data has been completely migrated to the metric store (default SQLite ./data/metrics.db, or configured
// MySQL/PostgreSQL). old records/records_long_term/gpu_records/ping_records
// The table is only read as a data source when starting a one-time migration, and all reads and writes during runtime go through the metric store.
//
// Therefore, this file no longer retains any reading, writing, compression (rollup) and traffic incremental repair logic for the old table:
// The metric store manages rollup and retention policies by itself, and compression of old tables is meaningless.

func DeleteAll() error {
	return metricstore.DeleteAllRecords(context.Background())
}

// GetGPURecordsByClientAndTime Gets GPU record data.
func GetGPURecordsByClientAndTime(uuid string, start, end time.Time) ([]models.GPURecord, error) {
	return metricstore.GetGPURecordsByClientAndTime(context.Background(), uuid, start, end)
}

func GetRecordsByClientAndTime(uuid string, start, end time.Time) ([]models.Record, error) {
	return metricstore.GetRecordsByClientAndTime(context.Background(), uuid, start, end)
}

func GetRecordsByClientAndTimeForLoadType(uuid string, start, end time.Time, loadType string) ([]models.Record, error) {
	return metricstore.GetRecordsByClientAndTimeForLoadType(context.Background(), uuid, start, end, loadType)
}

func GetRecordsByClientAndTimeForLoadTypeMaxPoints(uuid string, start, end time.Time, loadType string, maxPoints int) ([]models.Record, error) {
	return metricstore.GetRecordsByClientAndTimeForLoadTypeMaxPoints(context.Background(), uuid, start, end, loadType, maxPoints)
}

// GetRecordsByTime Gets all client records within the time range.
func GetRecordsByTime(start, end time.Time) ([]models.Record, error) {
	return metricstore.GetRecordsByTime(context.Background(), start, end)
}

func GetRecordsByTimeForLoadType(start, end time.Time, loadType string) ([]models.Record, error) {
	return metricstore.GetRecordsByTimeForLoadType(context.Background(), start, end, loadType)
}

func GetRecordsByTimeForLoadTypeMaxPoints(start, end time.Time, loadType string, maxPoints int) ([]models.Record, error) {
	return metricstore.GetRecordsByTimeForLoadTypeMaxPoints(context.Background(), start, end, loadType, maxPoints)
}
