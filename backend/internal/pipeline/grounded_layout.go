package pipeline

import (
	"regexp"
	"strings"
	"table-for-you/backend/internal/domain"
	"unicode"
)

// literalSpan repairs presentation-only differences while returning original bytes.
func literalSpan(text, quote string) string {
	if strings.Contains(text, quote) {
		return quote
	}
	clean := func(s string) (string, []int) {
		var b strings.Builder
		offsets := []int{}
		space := false
		for at, r := range s {
			if strings.ContainsRune("*_`#", r) {
				continue
			}
			if unicode.IsSpace(r) {
				if space {
					continue
				}
				r = ' '
				space = true
			} else {
				space = false
			}
			lower := strings.ToLower(string(r))
			b.WriteString(lower)
			for range []byte(lower) {
				offsets = append(offsets, at)
			}
		}
		offsets = append(offsets, len(s))
		return b.String(), offsets
	}
	body, offsets := clean(text)
	needle, _ := clean(quote)
	needle = strings.TrimSpace(needle)
	at := strings.Index(body, needle)
	if at < 0 || needle == "" {
		return ""
	}
	end := at + len(needle)
	return text[offsets[at]:offsets[end]]
}

func proseSources(g *domain.Grounding, quote string) []int {
	refs := map[int]bool{}
	for _, s := range g.Supports {
		if literalSpan(quote, s.Text) != "" || literalSpan(s.Text, quote) != "" {
			for _, n := range s.Sources {
				refs[n] = true
			}
		}
	}
	out := []int{}
	for _, s := range g.Sources {
		if refs[s.Number] {
			out = append(out, s.Number)
		}
	}
	return out
}

var restaurantHeading = regexp.MustCompile(`(?m)^#{1,4}\s+[0-9]+[.)]\s+([^\r\n]+)`)
var boldDish = regexp.MustCompile(`(?m)^[ \t]*[-*]\s+\*\*([^*\r\n]+)\*\*([^\r\n]*)`)
var layoutPrice = regexp.MustCompile(`(?i)(?:JPY|EUR|GBP|INR|USD|CAD|PLN|BRL|ARS|€|£|₹)\s*([0-9]+(?:[.,][0-9]+)*)`)
var dishPriceDivider = regexp.MustCompile(`\s+[–—-]\s+(?:JPY|EUR|GBP|INR|USD|€|£|₹)`)
var answerLink = regexp.MustCompile(`\[([^\]]+)\]\((https://[^\s)]+)\)`)

// groundedLayout is a no-call fallback for explicitly numbered/bold provider prose.
// It copies literal blocks and amounts only; unfamiliar layouts retain prose instead.
func groundedLayout(g *domain.Grounding) domain.GroundedExtraction {
	raw := domain.GroundedExtraction{}
	heads := restaurantHeading.FindAllStringSubmatchIndex(g.Text, -1)
	for i, h := range heads {
		end := len(g.Text)
		if i+1 < len(heads) {
			end = heads[i+1][0]
		}
		block := g.Text[h[0]:end]
		r := domain.GroundedRestaurant{Name: strings.TrimSpace(g.Text[h[2]:h[3]]), Evidence: domain.GroundedClaim{Quote: block, Sources: proseSources(g, block)}}
		for _, link := range answerLink.FindAllStringSubmatch(block, -1) {
			if r.RestaurantURL == "" {
				r.RestaurantURL = link[2]
			}
			if strings.Contains(strings.ToLower(link[1]), "menu") {
				r.MenuURL = link[2]
			}
		}
		dishes := boldDish.FindAllStringSubmatchIndex(block, -1)
		for n, m := range dishes {
			line := block[m[0]:m[1]]
			name := block[m[2]:m[3]]
			if !layoutPrice.MatchString(line) {
				continue
			}
			if split := dishPriceDivider.FindStringIndex(name); split != nil {
				name = name[:split[0]]
			}
			endDish := len(block)
			if n+1 < len(dishes) {
				endDish = dishes[n+1][0]
			}
			quote := block[m[0]:endDish]
			// Don't consume the separate review section as part of the last dish.
			for _, marker := range []string{"**Review Observations:", "**Diner Feedback:", "---"} {
				if at := strings.Index(quote, marker); at >= 0 {
					quote = quote[:at]
				}
			}
			prices := layoutPrice.FindAllStringSubmatch(line, -1)
			d := domain.GroundedDish{Name: strings.TrimSpace(name), Price: prices[0][1], PriceBasis: "listed", Evidence: domain.GroundedClaim{Quote: quote, Sources: proseSources(g, quote)}}
			if len(prices) > 1 && strings.Contains(line, prices[0][0]+" – "+prices[1][0]) {
				d.Price += "–" + prices[1][1]
				d.PriceBasis = "range"
			}
			if strings.Contains(strings.ToLower(line), "under ") || strings.Contains(strings.ToLower(line), "included in") {
				d.Price = ""
				d.PriceBasis = "unknown"
			}
			if strings.Contains(strings.ToLower(line), "estimate") {
				d.PriceBasis = "estimate"
			}
			for _, code := range []string{"JPY", "EUR", "GBP", "INR", "USD", "CAD", "PLN", "BRL", "ARS"} {
				if strings.Contains(line, code) {
					d.Currency = code
					break
				}
			}
			if strings.Contains(line, "€") {
				d.Currency = "EUR"
			}
			if strings.Contains(line, "£") {
				d.Currency = "GBP"
			}
			if strings.Contains(line, "₹") {
				d.Currency = "INR"
			}
			for _, l := range strings.Split(quote, "\n") {
				if strings.Contains(strings.ToLower(l), "ingredients") {
					if at := strings.Index(l, ":"); at >= 0 {
						d.Description = strings.TrimSpace(strings.TrimLeft(l[at+1:], "* "))
					}
				}
			}
			r.Dishes = append(r.Dishes, d)
		}
		raw.Restaurants = append(raw.Restaurants, r)
	}
	return raw
}
