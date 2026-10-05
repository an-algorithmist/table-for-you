package pipeline

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

type interval struct{ start, end int }

// RelevantText selects literal passages without embeddings.
// Full documents remain available for exact citation validation.
func RelevantText(text, kind, name string) string {
	if kind == "menu" {
		return RankedMenuText(text, name)
	}
	limit := 18000
	if kind == "review" {
		limit = 6500
	}
	if kind == "discovery" {
		limit = 1800
	}
	if len(text) <= limit {
		return text
	}
	lower := strings.ToLower(text)
	windows := []interval{{0, 600}, {len(text) - 900, len(text)}}
	words := []string{strings.ToLower(name), "vegan", "vegetarian", "gluten", "lunch", "dinner", "breakfast", "€", "eur", "pln", "zł", "gbp", "£", "usd", "czk", "huf", "dkk", "sek", "nok", "chf", "cad", "jpy", "ars", "brl", "₹", "¥", "円", "R$", "$", "all menu items", "price", "per person", "complaint", "slow", "rude", "delicious", "reviewed", "cons:", "pros:", "service", "without onion", "sin ajo", "sans ail"}
	if kind == "discovery" {
		words = words[:1]
	}
	for _, word := range words {
		offset := 0
		for count := 0; count < 3; count++ {
			index := strings.Index(lower[offset:], word)
			if index < 0 {
				break
			}
			index += offset
			start := index - 220
			if start < 0 {
				start = 0
			}
			end := index + 650
			if end > len(text) {
				end = len(text)
			}
			windows = append(windows, interval{start, end})
			offset = index + len(word)
			if offset >= len(text) {
				break
			}
		}
	}
	sort.Slice(windows, func(i, j int) bool { return windows[i].start < windows[j].start })
	merged := []interval{}
	for _, w := range windows {
		if len(merged) > 0 && w.start <= merged[len(merged)-1].end {
			if w.end > merged[len(merged)-1].end {
				merged[len(merged)-1].end = w.end
			}
		} else {
			merged = append(merged, w)
		}
	}
	var out strings.Builder
	for _, w := range merged {
		remaining := limit - out.Len()
		if remaining <= 0 {
			break
		}
		part := text[w.start:w.end]
		if len(part) > remaining {
			part = part[:remaining]
		}
		for !utf8.ValidString(part) && len(part) > 0 {
			part = part[:len(part)-1]
		}
		out.WriteString(part)
		out.WriteString("\n[passage boundary]\n")
	}
	return out.String()
}

var passageToken = regexp.MustCompile(`[\p{L}\p{N}]+`)
var numericRow = regexp.MustCompile(`(?m)[\p{L}][^\n]{1,100}[ \t|]+[0-9]+[.,][0-9]{1,2}(?:[ \t|]*$)`)

// RankedMenuText uses BM25 over adjacent line windows, preserving literal menu rows.
func RankedMenuText(text, query string) string {
	const limit = 18000
	if len(text) <= limit {
		return text
	}
	lines := strings.SplitAfter(text, "\n")
	parts := []string{}
	var block strings.Builder
	for _, line := range lines {
		block.WriteString(line)
		if block.Len() >= 1200 {
			parts = append(parts, block.String())
			block.Reset()
		}
	}
	if block.Len() > 0 {
		parts = append(parts, block.String())
	}
	// Adjacent blocks overlap, retaining names and prices across block boundaries.
	windows := make([]string, len(parts))
	for i, p := range parts {
		windows[i] = p
		if i+1 < len(parts) {
			windows[i] += parts[i+1]
		}
	}
	terms := passageToken.FindAllString(strings.ToLower(query+" menu price prices"), -1)
	df := map[string]int{}
	freq := make([]map[string]int, len(windows))
	lens := make([]int, len(windows))
	avg := 0.0
	for i, p := range windows {
		freq[i] = map[string]int{}
		tokens := passageToken.FindAllString(strings.ToLower(p), -1)
		lens[i] = len(tokens)
		avg += float64(len(tokens))
		for _, t := range tokens {
			freq[i][t]++
		}
		for t := range freq[i] {
			df[t]++
		}
	}
	avg /= float64(max(1, len(windows)))
	type scored struct {
		i     int
		value float64
	}
	scores := []scored{}
	for i, p := range windows {
		score := 0.0
		for _, t := range terms {
			f := float64(freq[i][t])
			if f == 0 {
				continue
			}
			idf := math.Log(1 + (float64(len(windows)-df[t])+0.5)/(float64(df[t])+0.5))
			score += idf * f * 2.2 / (f + 1.2*(0.25+0.75*float64(lens[i])/math.Max(avg, 1)))
		}
		score += float64(min(8, len(menuPrice.FindAllString(p, -1))))*2 + float64(min(8, len(numericRow.FindAllString(p, -1))))*2
		scores = append(scores, scored{i, score})
	}
	sort.SliceStable(scores, func(i, j int) bool { return scores[i].value > scores[j].value })
	selected := map[int]bool{}
	size := 0
	for _, s := range scores {
		for _, i := range []int{s.i, s.i + 1} {
			if i >= len(parts) || selected[i] {
				continue
			}
			if size+len(parts[i])+24 > limit {
				continue
			}
			selected[i] = true
			size += len(parts[i]) + 24
		}
	}
	var out strings.Builder
	for i, p := range parts {
		if selected[i] {
			out.WriteString(p)
			out.WriteString("\n[passage boundary]\n")
		}
	}
	return out.String()
}
