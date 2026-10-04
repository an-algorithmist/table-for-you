package workflow

import (
	"sort"
	"strings"
	"unicode/utf8"
)

type interval struct{ start, end int }

// Literal passage selection, without embeddings. Full documents remain available for citation validation.
func RelevantText(text, kind, name string) string {
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
