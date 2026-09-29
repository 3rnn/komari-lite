package metric

import (
	"strconv"
	"strings"
)

// Pxx builds the Aggregation for an arbitrary percentile. The argument is a
// percentage in (0,100): Pxx(99.9) -> "p99.9", Pxx(50) -> "p50". The fixed
// AggP50/AggP95/AggP99 constants are just the common cases of this same string
// form, so they keep working unchanged.
//
// This is what turns the package from "p50/p95/p99 only" into "any percentile":
// callers can ask for p75, p90, p99.99, etc., and every path (in-memory,
// SQL pushdown, and rollup-via-t-digest) understands it.
//
// Pxx Constructs an Aggregation based on arbitrary percentiles. The parameters are percentages within (0,100):
// Pxx(99.9) -> "p99.9", Pxx(50) -> "p50". Fixed AggP50/AggP95/AggP99
// Constants are just common cases of the same string form, so the original behavior is maintained.
//
// This changes the package from "only supports p50/p95/p99" to "supports any percentile": the caller can request
// p75, p90, p99.99, etc., and per path (memory, SQL pushdown, t-digest based rollup)
// Everybody understands it.
func Pxx(p float64) Aggregation {
	// Trim trailing zeros so Pxx(95) == AggP95 ("p95"), not "p95.000000".
	s := strconv.FormatFloat(p, 'f', -1, 64)
	return Aggregation("p" + s)
}

// parsePercentile reports whether agg names a percentile and, if so, returns
// the corresponding fraction in [0,1]. "p99.9" -> 0.999. Out-of-range
// percentages (<=0 or >=100) are rejected so validation can reject them.
//
// parsePercentile determines whether agg has a named percentile; if so, returns the corresponding [0,1] decimal.
// For example "p99.9" -> 0.999. Out-of-bounds percentages (<=0 or >=100) are rejected to verify the logic
// Be able to reject them.
func parsePercentile(agg Aggregation) (float64, bool) {
	s := string(agg)
	if len(s) < 2 || (s[0] != 'p' && s[0] != 'P') {
		return 0, false
	}
	pct, err := strconv.ParseFloat(s[1:], 64)
	if err != nil {
		return 0, false
	}
	if pct <= 0 || pct >= 100 {
		return 0, false
	}
	return pct / 100, true
}

// isPercentile reports whether agg is any percentile aggregation.
//
// isPercentile determines whether the aggregation type is any percentile aggregation.
func isPercentile(agg Aggregation) bool {
	_, ok := parsePercentile(agg)
	return ok
}

// percentileFractionString renders the fraction for SQL percentile_cont, e.g.
// "p99.9" -> "0.999". Trailing zeros are trimmed for stable SQL text.
//
// percentileFractionString Required to convert percentile aggregation to SQL percentile_cont
// Decimal string, with trailing zeros removed to keep the SQL text stable.
func percentileFractionString(agg Aggregation) (string, bool) {
	f, ok := parsePercentile(agg)
	if !ok {
		return "", false
	}
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		// f is in (0,1) so this should not happen, but guard anyway.
		s = strconv.FormatFloat(f, 'f', 1, 64)
	}
	return s, true
}
