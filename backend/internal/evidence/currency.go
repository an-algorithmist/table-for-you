package evidence

import (
	"nebulaiq/internal/domain"
	"regexp"
	"strings"
)

var currencyCodes = []string{"EUR", "GBP", "INR", "PLN", "USD", "CAD", "JPY", "ARS", "BRL", "CNY", "AUD", "NZD", "MXN", "CHF", "CZK", "HUF", "DKK", "SEK", "NOK", "SGD", "THB", "KRW", "IDR", "MYR", "VND", "PHP", "AED", "TRY", "CLP", "COP", "PEN"}
var currencySymbols = map[string]string{"EUR": "€", "GBP": "£", "INR": "₹", "PLN": "zł", "BRL": "R$", "CAD": "CA$", "USD": "US$", "AUD": "AU$", "NZD": "NZ$", "MXN": "MX$", "JPY": "円", "KRW": "₩", "THB": "฿", "VND": "₫"}

func currencyInText(text string) string {
	found := map[string]bool{}
	for _, code := range currencyCodes {
		if regexp.MustCompile(`(?i)(?:\b` + code + `\s*[0-9]|[0-9](?:[.,][0-9]{1,2})?\s*` + code + `\b)`).MatchString(text) {
			found[code] = true
		}
		if symbol := currencySymbols[code]; symbol != "" && strings.Contains(text, symbol) {
			found[code] = true
		}
	}
	// ¥ is ambiguous between JPY and CNY; a bare $ is also ambiguous.
	if len(found) == 1 {
		for code := range found {
			return code
		}
	}
	return ""
}
func menuCurrency(doc domain.Document) (string, domain.Citation) {
	if doc.Snippet || doc.Kind != "menu" {
		return "", domain.Citation{}
	}
	code := currencyInText(doc.Text)
	if code == "" {
		return "", domain.Citation{}
	}
	for _, line := range strings.Split(doc.Text, "\n") {
		if currencyInText(line) == code {
			quote := line
			if len(strings.TrimSpace(quote)) < 8 {
				quote = doc.Text
			}
			return code, domain.Citation{SourceID: doc.ID, Quote: quote}
		}
	}
	return "", domain.Citation{}
}
func localCurrency(country string) string {
	return map[string]string{"spain": "EUR", "france": "EUR", "germany": "EUR", "italy": "EUR", "portugal": "EUR", "netherlands": "EUR", "ireland": "EUR", "austria": "EUR", "belgium": "EUR", "greece": "EUR", "finland": "EUR", "united kingdom": "GBP", "uk": "GBP", "poland": "PLN", "india": "INR", "japan": "JPY", "united states": "USD", "united states of america": "USD", "usa": "USD", "canada": "CAD", "argentina": "ARS", "brazil": "BRL", "china": "CNY", "australia": "AUD", "new zealand": "NZD", "mexico": "MXN", "singapore": "SGD", "thailand": "THB", "south korea": "KRW", "chile": "CLP", "colombia": "COP", "peru": "PEN"}[strings.ToLower(strings.TrimSpace(country))]
}
func containsMeat(quote string) bool {
	q := normalize(quote)
	for _, term := range []string{"vegan", "plant-based", "plant based", "meat-free", "mock", "vegetarian", "without meat", "sans viande", "carrot salmon", "mushroom bacon", "soy chicken", "tofu chicken", "vegan salmon"} {
		if strings.Contains(q, term) {
			return false
		}
	}
	return regexp.MustCompile(`(?i)\b(bacon|chicken|beef|pork|ham|salmon|tuna|shrimp|prawns|lamb|duck|turkey|sausage|fish|poulet|jambon|saumon|boeuf|porc|pollo|jamón|salmón)\b`).MatchString(q)
}
