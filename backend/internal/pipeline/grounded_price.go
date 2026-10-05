package pipeline

import (
	"regexp"
	"strings"
	"table-for-you/backend/internal/domain"
)

var groundedPriceRow = regexp.MustCompile(`(?i)(?:EUR|JPY|GBP|INR|USD|CAD|PLN|BRL|ARS|€|£|₹|¥)\s*([0-9]+(?:[.,][0-9]+)*)(?:\s*[–—-]\s*(?:EUR|JPY|GBP|INR|USD|CAD|PLN|BRL|ARS|€|£|₹|¥)?\s*([0-9]+(?:[.,][0-9]+)*))?(\+)?`)
var groundedDishBoundary = regexp.MustCompile(`(?mi)\n\s*(?:#{1,6}\s|[0-9]+[.)]\s|[-*]\s+\*\*(?:[^*\n]+)\*\*)`)
var groundedUpperBound = regexp.MustCompile(`(?i)under|below|less than|included in`)
var groundedPricingLabel = regexp.MustCompile(`(?i)pricing|listed price|price:`)
var groundedApproximate = regexp.MustCompile(`(?i)estimat|approx`)

// Recover a separately attributed price paragraph inside the same dish block.
// This establishes consistency with provider prose, not webpage verification.
func recoverGroundedPrice(g *domain.Grounding, r domain.GroundedRestaurant, d domain.GroundedDish) domain.GroundedDish {
	if d.Evidence.Quote == "" || !strings.Contains(strings.ToLower(d.Evidence.Quote), strings.ToLower(d.Name)) {
		return d
	}
	quote := d.Evidence.Quote
	at := strings.Index(r.Evidence.Quote, quote)
	if at >= 0 {
		tail := r.Evidence.Quote[at+len(quote):]
		end := len(tail)
		for _, match := range groundedDishBoundary.FindAllStringIndex(tail, -1) {
			heading := strings.ToLower(tail[match[0]:match[1]])
			if strings.Contains(heading, "pricing") || strings.Contains(heading, "listed price") || strings.Contains(heading, "ingredients") {
				continue
			}
			end = match[0]
			break
		}
		quote += tail[:end]
	}
	for _, row := range strings.Split(quote, "\n") {
		m := groundedPriceRow.FindStringSubmatch(row)
		if m == nil || groundedUpperBound.MatchString(row) {
			continue
		}
		// A row outside the dish title must explicitly describe pricing.
		if !strings.Contains(strings.ToLower(row), strings.ToLower(d.Name)) && !groundedPricingLabel.MatchString(row) {
			continue
		}
		priceRefs := proseSources(g, row)
		if len(priceRefs) == 0 {
			continue
		}
		claim := domain.GroundedClaim{Quote: quote, Sources: proseSources(g, quote)}
		if !groundedClaimOK(g, claim) {
			continue
		}
		d.Evidence = claim
		d.Price = m[1] + m[3]
		d.PriceBasis = "listed"
		if m[2] != "" {
			d.Price = m[1] + "–" + m[2]
			d.PriceBasis = "range"
		}
		if strings.Contains(row, "~") || groundedApproximate.MatchString(row) {
			d.PriceBasis = "estimate"
		}
		for _, code := range []string{"EUR", "JPY", "GBP", "INR", "USD", "CAD", "PLN", "BRL", "ARS"} {
			if strings.Contains(strings.ToUpper(row), code) {
				d.Currency = code
				break
			}
		}
		for symbol, code := range map[string]string{"€": "EUR", "£": "GBP", "₹": "INR"} {
			if strings.Contains(row, symbol) {
				d.Currency = code
				break
			}
		}
		return d
	}
	return d
}
