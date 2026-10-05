package pipeline

import (
	"regexp"
	"strings"
	"time"

	"table-for-you/backend/internal/domain"
	"table-for-you/backend/internal/evidence"
	"table-for-you/backend/internal/llm"
	"table-for-you/backend/internal/providers"
)

// Formatting is a single plain attempt. Its failure must never lose a paid answer.
func (j *job) formatGrounded(g *domain.Grounding) {
	start := time.Now()
	defer j.duration("grounded_format", start)
	if j.use.ModelCalls >= j.modelCallLimit() {
		g.FormattingError = "Formatting call budget exhausted"
		return
	}
	prompt, err := llm.Prompt("grounded_format")
	if err != nil {
		g.FormattingError = "Formatting unavailable"
		return
	}
	preamble, err := llm.Prompt("untrusted")
	if err != nil {
		g.FormattingError = "Formatting unavailable"
		return
	}
	var raw domain.GroundedExtraction
	use, err := j.e.Model.Generate(j.ctx, preamble+prompt, marshal(map[string]any{"answer": g.Text, "sources": g.Sources, "supports": g.Supports}), domain.GroundedExtraction{}, &raw)
	j.use.ModelCalls += max(1, use.ModelCalls)
	j.use.InputTokens += use.InputTokens
	j.use.OutputTokens += use.OutputTokens
	if err != nil {
		g.FormattingError = "Structured shortlist unavailable; original answer retained"
		_ = j.event("grounded.format_failed", g.FormattingError)
		return
	}
	g.Recommendations = checkedGrounded(g, raw)
	fallback := checkedGrounded(g, groundedLayout(g))
	for _, f := range fallback {
		found := false
		for i, r := range g.Recommendations {
			if strings.Contains(foldName(r.Name), foldName(f.Name)) || strings.Contains(foldName(f.Name), foldName(r.Name)) {
				found = true
				if len(r.Dishes) == 0 {
					g.Recommendations[i].Dishes = f.Dishes
				}
			}
		}
		if !found && len(g.Recommendations) < 4 {
			g.Recommendations = append(g.Recommendations, f)
		}
	}
	_ = j.event("grounded.formatted", "Formatted grounded claims into shortlist cards; no additional search")
}

func groundedClaimOK(g *domain.Grounding, c domain.GroundedClaim) bool {
	if strings.TrimSpace(c.Quote) == "" || !strings.Contains(g.Text, c.Quote) || len(c.Sources) == 0 {
		return false
	}
	valid := map[int]bool{}
	for _, s := range g.Sources {
		valid[s.Number] = providers.SafeURL(s.URL)
	}
	for _, n := range c.Sources {
		if !valid[n] {
			return false
		}
	}
	// When attribution spans exist, reject sources unrelated to the claim's prose.
	if len(g.Supports) > 0 {
		linked := map[int]bool{}
		for _, s := range g.Supports {
			if literalSpan(c.Quote, s.Text) != "" || literalSpan(s.Text, c.Quote) != "" {
				for _, n := range s.Sources {
					linked[n] = true
				}
			}
		}
		for _, n := range c.Sources {
			if !linked[n] {
				return false
			}
		}
	}
	return true
}

var groundedAmount = regexp.MustCompile(`[0-9]+(?:[.,][0-9]+)*`)

func checkedGrounded(g *domain.Grounding, raw domain.GroundedExtraction) []domain.GroundedRestaurant {
	out := []domain.GroundedRestaurant{}
	allowed := func(u string) bool {
		if !providers.SafeURL(u) {
			return false
		}
		if strings.Contains(g.Text, u) {
			return true
		}
		for _, s := range g.Sources {
			if s.URL == u {
				return true
			}
		}
		return false
	}
	for _, r := range raw.Restaurants {
		if len(out) == 4 {
			break
		}
		r.Evidence.Quote = literalSpan(g.Text, r.Evidence.Quote)
		if r.Name == "" || !groundedClaimOK(g, r.Evidence) || !strings.Contains(strings.ToLower(r.Evidence.Quote), strings.ToLower(r.Name)) {
			continue
		}
		if !allowed(r.RestaurantURL) {
			r.RestaurantURL = ""
		}
		if !allowed(r.MenuURL) {
			r.MenuURL = ""
		}
		// Use the literal provider passage for free-form text, preventing added facts.
		r.Description = ""
		if r.Address != "" && !strings.Contains(g.Text, r.Address) {
			r.Address = ""
		}
		dishes := []domain.GroundedDish{}
		for _, d := range r.Dishes {
			if len(dishes) == 3 {
				break
			}
			d.Evidence.Quote = literalSpan(g.Text, d.Evidence.Quote)
			d = recoverGroundedPrice(g, r, d)
			if d.Name == "" || !groundedClaimOK(g, d.Evidence) || !strings.Contains(strings.ToLower(d.Evidence.Quote), strings.ToLower(d.Name)) {
				continue
			}
			if d.Description != "" && !strings.Contains(d.Evidence.Quote, d.Description) {
				d.Description = d.Evidence.Quote
			}
			nums := groundedAmount.FindAllString(d.Price, -1)
			for _, n := range nums {
				if !groundedListedAmount(n, d.Evidence.Quote) {
					d.Price = ""
					break
				}
			}
			if len(nums) == 0 {
				d.Price = ""
			}
			if d.Currency != "" && !strings.Contains(strings.ToUpper(d.Evidence.Quote), d.Currency) {
				symbol := map[string]string{"EUR": "€", "GBP": "£", "INR": "₹", "JPY": "¥"}[d.Currency]
				if symbol == "" || !strings.Contains(d.Evidence.Quote, symbol) {
					d.Currency = ""
				}
			}
			for _, source := range g.Sources {
				delivery := false
				for _, host := range []string{"ubereats.com", "deliveroo.", "doordash.com", "just-eat.", "foodpanda."} {
					if strings.Contains(strings.ToLower(source.Title+" "+source.URL), host) {
						delivery = true
					}
				}
				if delivery {
					for _, ref := range d.Evidence.Sources {
						if ref == source.Number {
							d.Price = ""
							d.PriceBasis = "unknown"
						}
					}
				}
			}
			if strings.Contains(strings.ToLower(d.Evidence.Quote), "under "+strings.ToLower(d.Currency)) {
				d.Price = ""
				d.PriceBasis = "unknown"
			}
			switch d.PriceBasis {
			case "listed", "estimate", "range", "unknown":
			default:
				d.PriceBasis = "unknown"
			}
			dishes = append(dishes, d)
		}
		r.Dishes = dishes
		reviews := []domain.GroundedReview{}
		for _, v := range r.Reviews {
			v.Evidence.Quote = literalSpan(g.Text, v.Evidence.Quote)
			if groundedClaimOK(g, v.Evidence) && (v.Sentiment == "positive" || v.Sentiment == "negative") {
				v.Summary = v.Evidence.Quote
				reviews = append(reviews, v)
			}
		}
		r.Reviews = reviews
		out = append(out, r)
	}
	return out
}

// Price tokens must occur in a currency-bearing amount, not just a portion or count.
func groundedListedAmount(amount, quote string) bool {
	quote = regexp.MustCompile(`(?i)(?:under|below|less than)\s*(?:EUR|JPY|GBP|INR|USD|CAD|PLN|BRL|ARS|€|£|₹|¥)\s*[0-9]+(?:[.,][0-9]+)*`).ReplaceAllString(quote, "")
	matches := regexp.MustCompile(`(?i)(?:(?:EUR|JPY|GBP|INR|USD|CAD|PLN|BRL|ARS|€|£|₹|¥)\s*[0-9]+(?:[.,][0-9]+)*(?:\s*[–—-]\s*(?:EUR|JPY|GBP|INR|USD|CAD|PLN|BRL|ARS|€|£|₹|¥)?\s*[0-9]+(?:[.,][0-9]+)*)?|[0-9]+(?:[.,][0-9]+)*\s*(?:EUR|JPY|GBP|INR|USD|CAD|PLN|BRL|ARS|€|£|₹|円))`).FindAllString(quote, -1)
	for _, m := range matches {
		if evidence.ListedAmountInText(amount, m) {
			return true
		}
	}
	return false
}
