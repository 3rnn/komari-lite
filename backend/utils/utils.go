package utils

import (
	"strings"
)

func DataMasking(str string, private []string) string {
	if str == "" || len(private) == 0 {
		return str
	}
	mask := "********"

	// Similarity threshold, which can be adjusted as needed (0~1, the larger the value, the stricter it is)
	const threshold = 0.8

	runes := []rune(str)
	n := len(runes)
	toMask := make([]bool, n)

	// Preprocess the words in private to remove empty spaces and duplicates
	uniq := make(map[string]struct{})
	var words []string
	for _, w := range private {
		w = strings.TrimSpace(w)
		if w == "" {
			continue
		}
		if _, ok := uniq[w]; ok {
			continue
		}
		uniq[w] = struct{}{}
		words = append(words, w)
	}
	if len(words) == 0 {
		return str
	}

	// Sliding window matching + fuzzy matching word by word (Levenshtein similarity)
	for _, w := range words {
		wRunes := []rune(w)
		wl := len(wRunes)
		if wl == 0 || wl > n {
			continue
		}

		// The sliding window size uses the length of the sensitive word
		for i := 0; i <= n-wl; i++ {
			if allMasked(toMask[i : i+wl]) { // If all are marked, skip
				continue
			}
			sub := string(runes[i : i+wl])
			sim := similarity(sub, w)
			if sim >= threshold {
				for k := 0; k < wl; k++ {
					toMask[i+k] = true
				}
			}
		}
	}

	// Construct output: continuous mask segments are output only once; if the original masked length is >5, the first and last characters are displayed
	var b strings.Builder
	i := 0
	for i < n {
		if toMask[i] {
			start := i
			for i < n && toMask[i] {
				i++
			}
			end := i // Not included
			segLen := end - start
			if segLen > 5 {
				b.WriteRune(runes[start])
				b.WriteString(mask)
				b.WriteRune(runes[end-1])
			} else {
				b.WriteString(mask)
			}
		} else {
			b.WriteRune(runes[i])
			i++
		}
	}
	return b.String()
}

// allMasked determines whether all an interval has been marked
func allMasked(bools []bool) bool {
	for _, v := range bools {
		if !v {
			return false
		}
	}
	return true
}

// similarity returns the similarity (0~1) of two strings, based on Levenshtein distance
func similarity(a, b string) float64 {
	if a == b {
		return 1
	}
	ar := []rune(a)
	br := []rune(b)
	dist := levenshtein(ar, br)
	maxLen := len(ar)
	if len(br) > maxLen {
		maxLen = len(br)
	}
	if maxLen == 0 {
		return 1
	}
	return 1 - float64(dist)/float64(maxLen)
}

// levenshtein calculates the edit distance of two rune slices
func levenshtein(a, b []rune) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	// Reduce space complexity using rolling arrays
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			curr[j] = minInt(del, ins, sub)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

func minInt(vals ...int) int {
	m := vals[0]
	for _, v := range vals[1:] {
		if v < m {
			m = v
		}
	}
	return m
}
