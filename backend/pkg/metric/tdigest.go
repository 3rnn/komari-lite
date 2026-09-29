package metric

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"sort"
)

// TDigest is a mergeable sketch for estimating arbitrary quantiles of a stream
// of float64 values. It is the piece that makes downsampling lossless *enough*
// for percentiles: count/sum/min/max can be re-aggregated exactly when rolling
// finer buckets into coarser ones, but a percentile cannot be recovered from
// those scalars. Storing a small t-digest per rollup bucket lets a coarse
// bucket's pXX be computed by merging the digests of the finer buckets it
// covers, with bounded error and bounded size.
//
// This is the "merging" variant of Dunning & Ertl's t-digest. Centroids near
// the median are allowed to absorb more weight (lower resolution where the CDF
// is flat) while centroids in the tails stay small (high resolution where
// accuracy matters), giving relative error that is small at the extremes — the
// regime that matters for p95/p99/p99.9 latency work.
//
// TDigest is a mergeable sketch for estimating arbitrary quantiles of float64 data streams. It allows downsampling
// "Lossless enough" in terms of percentiles: count/sum/min/max can accurately re-aggregate when the thin bucket is merged into the thick bucket,
// But these scalars alone cannot recover percentiles. After each rollup bucket saves a small t-digest,
// The pXX of a coarse bucket can then be calculated by merging the thin bucket digests it covers, while maintaining bounded error and bounded size.
//
// This is the "merging" variant of Dunning and Ertl's t-digest. Center of mass near the median allows absorption
// More weight (use lower resolution where CDF is flat), keep the tail center of mass smaller (use where accuracy is important)
// higher resolution), resulting in smaller relative errors at extreme positions, which is exactly what p95/p99/p99.9 delay analysis
// The range of greatest concern.
type TDigest struct {
	// compression controls the size/accuracy tradeoff.
	//
	// compression controls the trade-off between size and precision.
	compression float64
	// centroids stores the digest's weighted clusters.
	//
	// centroids holds weighted clusters of digests.
	centroids []centroid
	// count is the total observed weight.
	//
	// count is the total observed weight.
	count float64
	// min is the minimum observed value.
	//
	// min is the observed minimum value.
	min float64
	// max is the maximum observed value.
	//
	// max is the observed maximum value.
	max float64
	// processed reports whether centroids is sorted+merged. Add() appends
	// unprocessed singletons and flips this false; process() restores it.
	//
	// processed indicates whether centroids have been sorted and merged. Add() will append the unhandled single point and put it
	// Set to false; process() will restore it.
	processed bool
}

// centroid stores one t-digest centroid.
//
// centroid saves a t-digest centroid.
type centroid struct {
	// mean is the centroid's weighted mean.
	//
	// mean is the weighted mean of the centroids.
	mean float64
	// weight is the total weight represented by the centroid.
	//
	// weight is the total weight represented by this centroid.
	weight float64
}

const (
	// defaultTDigestCompression is used when a caller supplies no useful value.
	//
	// defaultTDigestCompression is used when the caller does not provide a valid value.
	defaultTDigestCompression = 100.0
	// tdigestMagic0 is the first magic byte in the binary format.
	//
	// tdigestMagic0 is the first magic byte in binary format.
	tdigestMagic0 = 'T'
	// tdigestMagic1 is the second magic byte in the binary format.
	//
	// tdigestMagic1 is the second magic byte in binary format.
	tdigestMagic1 = 'D'
	// tdigestCompressedMagic1 identifies a losslessly DEFLATE-compressed V1
	// digest. The compressed payload is the complete legacy TD blob, so decoding
	// preserves every floating-point bit and remains backward compatible.
	tdigestCompressedMagic1 = 'Z'
	// tdigestVersion is the current binary encoding version.
	//
	// tdigestVersion is the current binary encoding version.
	tdigestVersion = 1
)

// NewTDigest returns an empty digest. compression trades size for accuracy;
// higher keeps more centroids. Values <= 1 fall back to the default (100),
// which keeps each digest to a few KB while holding tail error to well under
// 1% for typical distributions.
//
// NewTDigest returns an empty digest. compression trade-off between size and precision; higher values preserve
// More centroids. Values less than or equal to 1 will fall back to the default value (100), which usually keeps each digest within
// A few KB while keeping the tail error of a typical distribution well below 1%.
func NewTDigest(compression float64) *TDigest {
	if compression <= 1 {
		compression = defaultTDigestCompression
	}
	return &TDigest{
		compression: compression,
		min:         math.Inf(1),
		max:         math.Inf(-1),
		processed:   true,
	}
}

// Add folds a single observation with weight w (w must be > 0) into the digest.
//
// Add adds an observation and its weight to the summary; the weight must be greater than 0.
func (t *TDigest) Add(x, w float64) {
	if w <= 0 || math.IsNaN(x) || math.IsInf(x, 0) {
		return
	}
	t.centroids = append(t.centroids, centroid{mean: x, weight: w})
	t.count += w
	if x < t.min {
		t.min = x
	}
	if x > t.max {
		t.max = x
	}
	t.processed = false
	// Bound the unprocessed buffer so a long stream cannot grow memory without
	// limit; process() collapses it back to ~compression centroids.
	if len(t.centroids) > int(8*t.compression)+16 {
		t.process()
	}
}

// Merge folds every centroid of other into t. This is the operation rollup
// composition relies on: a coarse bucket merges the digests of the finer
// buckets it spans.
//
// Merge merges each centroid of other into t. rollup synthesis relies on this operation: the rough bucket merges the
// Thin barrel digest.
func (t *TDigest) Merge(other *TDigest) {
	if other == nil || other.count == 0 {
		return
	}
	other.process()
	for _, c := range other.centroids {
		t.centroids = append(t.centroids, c)
	}
	t.count += other.count
	if other.min < t.min {
		t.min = other.min
	}
	if other.max > t.max {
		t.max = other.max
	}
	t.processed = false
	t.process()
}

// Count returns the total weight observed.
//
// Count returns the total weight of the observed samples.
func (t *TDigest) Count() float64 { return t.count }

// process sorts the buffered centroids by mean and merges adjacent ones while
// the merged weight stays under the quantile-dependent size limit
// 4*N*q*(1-q)/compression. That limit is generous near q=0.5 and tightens to
// near zero in the tails, which is exactly the t-digest accuracy profile.
//
// process sorts the buffer centroids by mean and after merging the weights are still below the quantile related size limit
// Merge adjacent centroids when 4*N*q*(1-q)/compression. This restriction is looser around q=0.5,
// It will tighten to close to zero in the tail, which is exactly the accuracy distribution characteristic of t-digest.
func (t *TDigest) process() {
	if t.processed {
		return
	}
	if len(t.centroids) == 0 {
		t.processed = true
		return
	}
	sort.Slice(t.centroids, func(i, j int) bool {
		return t.centroids[i].mean < t.centroids[j].mean
	})
	total := t.count
	merged := t.centroids[:0:0] // fresh backing array; don't alias input mid-merge
	cur := t.centroids[0]
	weightBefore := 0.0
	for i := 1; i < len(t.centroids); i++ {
		next := t.centroids[i]
		proposed := cur.weight + next.weight
		// Quantile at the center of the proposed combined centroid.
		q := (weightBefore + proposed/2) / total
		limit := 4 * total * q * (1 - q) / t.compression
		if proposed <= limit || limit < 1 && proposed <= 1 {
			// Weighted-mean update keeps the centroid's mean exact.
			cur.mean += next.weight * (next.mean - cur.mean) / proposed
			cur.weight = proposed
		} else {
			merged = append(merged, cur)
			weightBefore += cur.weight
			cur = next
		}
	}
	merged = append(merged, cur)
	t.centroids = merged
	t.processed = true
}

// Quantile estimates the value at q in [0,1] using linear interpolation between
// centroid centers, with the extreme tails anchored to the observed min/max.
//
// Quantile estimates the [0,1] quantiles with linear interpolation between centroid centers, with extreme tails anchored to
// Observed minimum and maximum values.
func (t *TDigest) Quantile(q float64) float64 {
	t.process()
	n := len(t.centroids)
	if n == 0 {
		return math.NaN()
	}
	if q <= 0 {
		return t.min
	}
	if q >= 1 {
		return t.max
	}
	if n == 1 {
		return t.centroids[0].mean
	}
	index := q * t.count

	// Head: between the observed min and the first centroid's center.
	c0 := t.centroids[0]
	if index < c0.weight/2 {
		z := index / (c0.weight / 2)
		return t.min + (c0.mean-t.min)*z
	}
	weightSoFar := c0.weight / 2
	for i := 0; i < n-1; i++ {
		c := t.centroids[i]
		next := t.centroids[i+1]
		dw := (c.weight + next.weight) / 2
		if index < weightSoFar+dw {
			z := (index - weightSoFar) / dw
			return c.mean*(1-z) + next.mean*z
		}
		weightSoFar += dw
	}
	// Tail: between the last centroid's center and the observed max.
	cl := t.centroids[n-1]
	z := (index - weightSoFar) / (cl.weight / 2)
	if z > 1 {
		z = 1
	}
	return cl.mean + (t.max-cl.mean)*z
}

// Encode serializes the (processed) digest to a compact little-endian blob:
// magic[2] version[1] compression[8] min[8] max[8] count[8] nCentroids[4]
// then nCentroids * (mean[8] weight[8]).
//
// Encode serializes the processed digest into a compact little-endian binary blob:
// magic[2] version[1] compression[8] min[8] max[8] count[8] nCentroids[4],
// Followed by nCentroids * (mean[8] weight[8]).
func (t *TDigest) Encode() []byte {
	return compressTDigestBlob(t.encodeRaw())
}

func (t *TDigest) encodeRaw() []byte {
	t.process()
	n := len(t.centroids)
	buf := make([]byte, 0, 3+8*4+4+n*16)
	buf = append(buf, tdigestMagic0, tdigestMagic1, tdigestVersion)
	var tmp [8]byte
	putF := func(f float64) {
		binary.LittleEndian.PutUint64(tmp[:], math.Float64bits(f))
		buf = append(buf, tmp[:]...)
	}
	putF(t.compression)
	putF(t.min)
	putF(t.max)
	putF(t.count)
	var u32 [4]byte
	binary.LittleEndian.PutUint32(u32[:], uint32(n))
	buf = append(buf, u32[:]...)
	for _, c := range t.centroids {
		putF(c.mean)
		putF(c.weight)
	}
	return buf
}

func compressTDigestBlob(raw []byte) []byte {
	if len(raw) < 96 || (len(raw) >= 3 && raw[0] == tdigestMagic0 && raw[1] == tdigestCompressedMagic1) {
		return raw
	}
	var payload bytes.Buffer
	writer, err := flate.NewWriter(&payload, flate.BestSpeed)
	if err != nil {
		return raw
	}
	if _, err := writer.Write(raw); err != nil {
		_ = writer.Close()
		return raw
	}
	if err := writer.Close(); err != nil {
		return raw
	}
	if payload.Len()+3 >= len(raw) {
		return raw
	}
	out := make([]byte, 0, payload.Len()+3)
	out = append(out, tdigestMagic0, tdigestCompressedMagic1, tdigestVersion)
	return append(out, payload.Bytes()...)
}

// DecodeTDigest reconstructs a digest produced by Encode. A nil/empty blob
// yields an empty digest so callers can treat "no sketch stored" uniformly.
//
// DecodeTDigest restores the digest generated by Encode; nil or empty blob returns an empty digest.
// It is convenient for the caller to uniformly handle the situation of "no sketch is saved".
func DecodeTDigest(b []byte) (*TDigest, error) {
	if len(b) == 0 {
		return NewTDigest(defaultTDigestCompression), nil
	}
	if len(b) >= 3 && b[0] == tdigestMagic0 && b[1] == tdigestCompressedMagic1 {
		if b[2] != tdigestVersion {
			return nil, errors.New("metric: unsupported compressed t-digest version")
		}
		reader := flate.NewReader(bytes.NewReader(b[3:]))
		decompressed, err := io.ReadAll(io.LimitReader(reader, (64<<20)+1))
		closeErr := reader.Close()
		if err != nil {
			return nil, errors.New("metric: invalid compressed t-digest blob")
		}
		if closeErr != nil {
			return nil, errors.New("metric: invalid compressed t-digest blob")
		}
		if len(decompressed) > 64<<20 {
			return nil, errors.New("metric: compressed t-digest blob is too large")
		}
		b = decompressed
	}
	if len(b) < 3+8*4+4 || b[0] != tdigestMagic0 || b[1] != tdigestMagic1 {
		return nil, errors.New("metric: invalid t-digest blob")
	}
	if b[2] != tdigestVersion {
		return nil, errors.New("metric: unsupported t-digest version")
	}
	off := 3
	getF := func() float64 {
		v := math.Float64frombits(binary.LittleEndian.Uint64(b[off : off+8]))
		off += 8
		return v
	}
	t := &TDigest{processed: true}
	t.compression = getF()
	t.min = getF()
	t.max = getF()
	t.count = getF()
	n := int(binary.LittleEndian.Uint32(b[off : off+4]))
	off += 4
	if n < 0 || off+n*16 > len(b) {
		return nil, errors.New("metric: truncated t-digest blob")
	}
	t.centroids = make([]centroid, n)
	for i := 0; i < n; i++ {
		t.centroids[i].mean = getF()
		t.centroids[i].weight = getF()
	}
	if t.compression <= 1 {
		t.compression = defaultTDigestCompression
	}
	return t, nil
}
