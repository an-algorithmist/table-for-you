package evidence

import (
	"nebulaiq/internal/domain"
	"regexp"
	"strings"
)

const currencyToken = `(?:PLN|EUR|USD|CAD|JPY|ARS|BRL|GBP|CHF|CZK|INR|R\$|CA\$|US\$|€|£|₹|zł|円|¥|\$)`
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
		if doc.Snippet || doc.Kind != "menu" || !strings.Contains(normalize(doc.Text+" "+doc.Title+" "+doc.URL), normalize(restaurant)) {
			continue
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
