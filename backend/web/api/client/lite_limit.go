package client

import (
	"fmt"
	"io"
)

func readBoundedReport(r io.Reader) ([]byte, error) {
	b, e := io.ReadAll(io.LimitReader(r, (1<<20)+1))
	if len(b) > 1<<20 {
		return nil, fmt.Errorf("report exceeds 1 MiB")
	}
	return b, e
}
