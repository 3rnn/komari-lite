package metric

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"
)

// queryRelationalRawBudgeted leaves ordinary internal reads untouched. All
// materialized JSON is measured in a stable snapshot before it is selected.
func (s *Store) queryRelationalRawBudgeted(ctx context.Context, query Query) ([]Point, error) {
	tx, err := s.reader().BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	where, args := s.buildWhere(query)
	order := "ASC"
	if query.Order == OrderDesc {
		order = "DESC"
	}
	byteLength := func(column string) string {
		switch s.cfg.Driver {
		case DriverPostgreSQL:
			return "OCTET_LENGTH(" + column + "::text)"
		case DriverMySQL:
			return "OCTET_LENGTH(CAST(" + column + " AS CHAR))"
		default:
			return "length(CAST(" + column + " AS BLOB))"
		}
	}
	limit := query.WorkBudget
	if limit < math.MaxInt {
		limit++
	}
	if query.Limit > 0 && query.Limit < limit {
		limit = query.Limit
	}
	base := fmt.Sprintf(" FROM %s WHERE %s ORDER BY ts_nano %s, metric_name, entity_id, tags_hash", s.tables.points, where, order)
	limitedArgs := append(append([]any{}, args...), limit)
	suffix := " LIMIT " + s.dialect.placeholder(len(limitedArgs))
	if query.Offset > 0 {
		limitedArgs = append(limitedArgs, query.Offset)
		suffix += " OFFSET " + s.dialect.placeholder(len(limitedArgs))
	}
	sizes, err := tx.QueryContext(ctx, "SELECT COALESCE("+byteLength("tags")+",0), COALESCE("+byteLength("labels")+",0)"+base+suffix, limitedArgs...)
	if err != nil {
		return nil, err
	}
	count, remaining := 0, sqliteV4BudgetedPayloadBytes
	for sizes.Next() {
		if err := ctx.Err(); err != nil {
			_ = sizes.Close()
			return nil, err
		}
		count++
		if count > query.WorkBudget {
			_ = sizes.Close()
			return nil, ErrQueryWorkBudgetExceeded
		}
		var tagsBytes, labelsBytes int
		if err := sizes.Scan(&tagsBytes, &labelsBytes); err != nil {
			_ = sizes.Close()
			return nil, err
		}
		if tagsBytes < 0 || labelsBytes < 0 || tagsBytes > remaining || labelsBytes > remaining-tagsBytes {
			_ = sizes.Close()
			return nil, ErrQueryWorkBudgetExceeded
		}
		remaining -= tagsBytes + labelsBytes
	}
	if err := sizes.Err(); err != nil {
		_ = sizes.Close()
		return nil, err
	}
	if err := sizes.Close(); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT metric_name, entity_id, ts_nano, value, tags, labels"+base+suffix, limitedArgs...)
	if err != nil {
		return nil, err
	}
	out := make([]Point, 0, count)
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		var p Point
		var ts int64
		var rawTags, rawLabels any
		if err := rows.Scan(&p.MetricName, &p.EntityID, &ts, &p.Value, &rawTags, &rawLabels); err != nil {
			_ = rows.Close()
			return nil, err
		}
		p.Timestamp = time.Unix(0, ts).UTC()
		p.Tags, err = decodeMap(rawTags)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		p.Labels, err = decodeMap(rawLabels)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}
