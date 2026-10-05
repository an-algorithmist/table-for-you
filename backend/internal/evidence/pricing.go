package evidence

import (
	"math/big"
	"net/url"
	"regexp"
	"strings"
	"table-for-you/backend/internal/domain"
)

var amounts = regexp.MustCompile(`[0-9]+(?:[.,][0-9]+)*`)

func quotedAmount(value, quote string) bool {
	n, ok := number(value)
	if !ok || n.Sign() <= 0 {
		return false
	}
	for _, match := range amounts.FindAllString(quote, -1) {
		v, ok := number(match)
		if ok && v.Cmp(n) == 0 {
			return true
		}
	}
	return false
}

func validateVenue(r *domain.Restaurant, docs map[string]domain.Document) {
	q := normalize(r.VenueEvidence.Quote)
	// A name such as "Vegan Bistro" alone is not a venue-wide dietary statement.
	q = strings.ReplaceAll(q, normalize(r.Name), "")
	valid := Supported(r.VenueEvidence, docs)
	for _, negative := range []string{"not vegan", "not vegetarian", "not a vegan", "not a vegetarian", "vegan options", "vegetarian options"} {
		if strings.Contains(q, negative) {
			valid = false
		}
	}
	markers := []string{}
	switch r.VenueType {
	case "vegan":
		markers = []string{"restaurante vegano", "restaurante vegana", "100% vegano", "100% vegana", "vegan restaurant", "vegan bistro", "fully vegan", "100% vegan", "entirely vegan", "exclusively vegan", "restauracja wegańska"}
	case "vegetarian":
		markers = []string{"restaurante vegetariano", "restaurante vegetariana", "pure veg restaurant", "vegetarian restaurant", "pure vegetarian", "fully vegetarian", "100% vegetarian", "exclusively vegetarian"}
	}
	found := false
	for _, m := range markers {
		found = found || strings.Contains(q, m)
	}
	if !valid || !found {
		r.VenueType = "unknown"
		r.VenueEvidence = domain.Citation{}
		menuURL, _ := url.Parse(r.MenuURL)
		for _, doc := range docs {
			u, _ := url.Parse(doc.URL)
			if doc.Snippet || doc.Kind != "menu" || menuURL == nil || u == nil || u.Hostname() != menuURL.Hostname() || !strings.Contains(normalize(doc.Text), normalize(r.Name)) {
				continue
			}
			for _, kind := range []string{"vegan", "vegetarian"} {
				pattern := regexp.MustCompile(`(?mi)^[ \t]*[·•][ \t]+` + kind + ` restaurant[ \t]*$`)
				if quote := pattern.FindString(doc.Text); quote != "" {
					r.VenueType = kind
					r.VenueEvidence = domain.Citation{SourceID: doc.ID, Quote: quote}
					return
				}
			}
		}
	}
}

// Estimates never become exact item prices or prove budget compliance.
func estimateSpend(r *domain.Restaurant, docs map[string]domain.Document) {
	r.Estimate = domain.PriceEstimate{}
	p := r.PriceRange
	low, lok := number(p.Low)
	high, hok := number(p.High)
	if Supported(p.Evidence, docs) && lok && hok && low.Sign() > 0 && low.Cmp(high) <= 0 && quotedAmount(p.Low, p.Evidence.Quote) && quotedAmount(p.High, p.Evidence.Quote) && currencySupported(p.Currency, p.Evidence.Quote) && strings.Contains(normalize(p.Evidence.Quote), "per person") {
		r.Estimate = domain.PriceEstimate{Low: p.Low, High: p.High, Currency: p.Currency, Basis: "Reported per-person range from the linked source; items and extras may vary.", Evidence: []domain.Citation{p.Evidence}}
		return
	}
	r.PriceRange = domain.PriceRange{}
	var min, max *big.Rat
	currency := ""
	citations := []domain.Citation{}
	for _, d := range r.Dishes {
		price, ok := number(d.Price)
		doc := docs[d.Evidence.SourceID]
		if d.PriceEvidence.SourceID != "" {
			doc = docs[d.PriceEvidence.SourceID]
		}
		if !ok || price.Sign() <= 0 || d.Currency == "" || doc.Snippet {
			continue
		}
		if currency == "" {
			currency = d.Currency
		}
		if currency != d.Currency {
			continue
		}
		if min == nil || price.Cmp(min) < 0 {
			min = new(big.Rat).Set(price)
		}
		if max == nil || price.Cmp(max) > 0 {
			max = new(big.Rat).Set(price)
		}
		if d.PriceEvidence.SourceID != "" {
			citations = append(citations, d.PriceEvidence)
		} else {
			citations = append(citations, d.Evidence)
		}
	}
	if min != nil {
		// This is an explicitly disclosed planning allowance, not a restaurant quote.
		max.Mul(max, big.NewRat(125, 100))
		basis := "Planning estimate for one listed dish per person. Upper end adds 25%; drinks, sides and fees may cost more. Not a quote for an unpriced dish."
		for _, d := range r.Dishes {
			if d.CurrencyBasis == "location_assumption" {
				basis += " Currency inferred from the verified restaurant location, not explicitly printed in the item quote."
				break
			}
		}
		r.Estimate = domain.PriceEstimate{Low: min.FloatString(2), High: max.FloatString(2), Currency: currency, Basis: basis, Evidence: citations}
	}
}

// ListedAmountInText compares an amount to complete numeric tokens in supporting prose.
func ListedAmountInText(value, text string) bool { return quotedAmount(value, text) }
