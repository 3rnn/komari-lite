package metric

import (
	"bytes"
	"compress/flate"
	"database/sql"
	"fmt"
	"io"
)

func decodeSQLitePointBlockBudgeted(codec, count int, checksum uint32, payload []byte,
	axisCodec, axisChecksum sql.NullInt64, axisPayload []byte, remaining *int,
) ([]sqliteV4BlockPoint, error) {
	if codec == sqliteV4BlockCodec {
		return decodeSQLiteV4BlockBudgeted(codec, count, checksum, payload, remaining)
	}
	if codec != sqliteV6SharedPointBlockCodec {
		return nil, fmt.Errorf("metric: unsupported SQLite point codec %d", codec)
	}
	timestamps, err := decodeSQLiteV6PointAxisBudgeted(int(axisCodec.Int64), count, uint32(axisChecksum.Int64), axisPayload, remaining)
	if err != nil {
		return nil, err
	}
	return decodeSQLiteV6PointValuesBudgeted(count, checksum, payload, timestamps, remaining)
}

// inflateSQLiteV4PointPayloadBudgeted limits allocation before parsing the block.
// Only public budgeted reads use this path; internal codec behavior is unchanged.
func inflateSQLiteV4PointPayloadBudgeted(payload []byte, remaining *int) ([]byte, error) {
	if len(payload) < 2 {
		return nil, fmt.Errorf("metric: truncated point payload")
	}
	switch payload[0] {
	case sqliteV4PayloadRaw:
		if len(payload)-1 > *remaining {
			return nil, ErrQueryWorkBudgetExceeded
		}
		*remaining -= len(payload) - 1
		return payload[1:], nil
	case sqliteV4PayloadDeflate:
		reader := flate.NewReader(bytes.NewReader(payload[1:]))
		raw, err := io.ReadAll(io.LimitReader(reader, int64(*remaining)+1))
		closeErr := reader.Close()
		if err != nil {
			return nil, fmt.Errorf("metric: decompress point payload: %w", err)
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if len(raw) > *remaining {
			return nil, ErrQueryWorkBudgetExceeded
		}
		*remaining -= len(raw)
		return raw, nil
	default:
		return nil, fmt.Errorf("metric: unsupported point payload compression %d", payload[0])
	}
}
