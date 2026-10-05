package evidence

import (
	"regexp"
	"strings"
	"table-for-you/backend/internal/domain"
)

const currencyToken = `(?:PLN|EUR|USD|CAD|JPY|ARS|BRL|GBP|CHF|CZK|INR|R\$|CA\$|US\$|€|£|₹|zł|円|yen\b|¥|\$)`
const amountToken = `[0-9]+(?:[.,][0-9]+)*`
const priceToken = `(?:` + currencyToken + `[ \t\x{00A0}]*` + amountToken + `|` + amountToken + `[ \t\x{00A0}]*` + currencyToken + `)`

var listedPrice = regexp.MustCompile(`(?i)` + priceToken)

// Recover only directly adjacent price/name rows in a full menu. Never infer a
// price from the user's budget or an unrelated nearby amount.
func recoverListedPrice(d *domain.Dish, restaurant string, docs []domain.Document) {
	name := regexp.QuoteMeta(d.Name)
	heading := `(?:\*\*|#{1,6}[ \t]+)?` + name + `(?:\*\*)?`
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?mi)^[ \t]*` + heading + `[ \t]+(?:` + priceToken + `|` + amountToken + `)[ \t]*$`),
		regexp.MustCompile(`(?mi)^[ \t]*(?:\$[ \t]*)?` + priceToken + `[ \t]*\r?\n(?:[ \t]*\r?\n)*[ \t]*` + heading + `[ \t]*$`),
		regexp.MustCompile(`(?mi)^[ \t]*` + heading + `[ \t]*\r?\n(?:[ \t]*\r?\n)*[ \t]*(?:\$[ \t]*)?` + priceToken + `[ \t]*$`),
	}
	for _, doc := range docs {
		// An explicit name followed immediately by a currency-bearing price is
		// usable even in a snippet; portions stay visible in the exact source row.
		if doc.Kind == "menu" {
			row := regexp.MustCompile(`(?mi)^[ \t]*(?:[-*][ \t]+)?(?:\*\*)?` + name + `(?:\*\*)?[ \t]*(` + priceToken + `)[^\r\n]*`)
			match := row.FindStringSubmatch(doc.Text)
			if len(match) > 1 {
				amount := amounts.FindString(match[1])
				currency := currencyInText(match[1])
				if n, ok := number(amount); ok && n.Sign() > 0 && currency != "" {
					d.Price = n.FloatString(2)
					d.PriceText = match[1]
					d.Currency = currency
					d.PriceEvidence = domain.Citation{SourceID: doc.ID, Quote: match[0]}
					return
				}
			}
		}
		if doc.Snippet || doc.Kind != "menu" || !strings.Contains(normalize(doc.Text+" "+doc.Title+" "+doc.URL), normalize(restaurant)) {
			continue
		}
		// Bare numeric menu rows need a currency legend in this same document.
		if currency := currencyInText(doc.Text); currency != "" {
			row := regexp.MustCompile(`(?mi)^[ \t]*\|?[ \t]*(?:\*\*)?` + name + `(?:\*\*)?[ \t]*(?:\|[ \t]*|[ \t]+)(` + amountToken + `)[ \t]*\|?[ \t]*$`)
			match := row.FindStringSubmatch(doc.Text)
			if len(match) > 1 {
				if n, ok := number(match[1]); ok && n.Sign() > 0 {
					d.Price = n.FloatString(2)
					d.PriceText = match[1]
					d.Currency = currency
					d.PriceEvidence = domain.Citation{SourceID: doc.ID, Quote: match[0]}
					return
				}
			}
		}
		for _, pattern := range patterns {
			quote := pattern.FindString(doc.Text)
			if quote == "" {
				continue
			}
			price := listedPrice.FindString(quote)
			if price == "" {
				price = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(quote), d.Name))
			}
			amount := amounts.FindString(price)
			currency := ""
			for _, code := range []string{"PLN", "EUR", "USD", "CAD", "JPY", "ARS", "BRL", "GBP", "CHF", "CZK", "INR"} {
				if strings.Contains(strings.ToUpper(price), code) {
					currency = code
					break
				}
			}
			for symbol, code := range map[string]string{"zł": "PLN", "€": "EUR", "£": "GBP", "₹": "INR"} {
				if strings.Contains(strings.ToLower(price), symbol) {
					currency = code
				}
			}
			if currency == "" {
				currency = currencyInText(price)
			}
			if n, ok := number(amount); ok {
				d.Price = n.FloatString(2)
			} else {
				continue
			}
			d.PriceText = price
			d.Currency = currency
			d.PriceEvidence = domain.Citation{SourceID: doc.ID, Quote: quote}
			return
		}
	}
}
