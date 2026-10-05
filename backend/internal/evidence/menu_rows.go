package evidence

import (
	"regexp"
	"strings"
	"table-for-you/backend/internal/domain"
)

var menuHeading = regexp.MustCompile(`(?m)^[ \t]*(?:[0-9]+[.] )?#{2,6}[ \t]+([^\r\n]+)`)

// RecoverMenuDish uses only a literal dish heading and its adjacent description.
// It does not repair arbitrary translated/paraphrased quotations or infer prices.
func RecoverMenuDish(d *domain.Dish, docs []domain.Document) bool {
	for _, doc := range docs {
		if doc.Kind != "menu" || doc.Snippet {
			continue
		}
		for _, m := range menuHeading.FindAllStringSubmatchIndex(doc.Text, -1) {
			name := strings.TrimSpace(doc.Text[m[2]:m[3]])
			if normalize(name) != normalize(d.Name) && (d.EnglishName == "" || normalize(name) != normalize(d.EnglishName)) {
				continue
			}
			after := doc.Text[m[1]:]
			gap := len(after) - len(strings.TrimLeft(after, " \t\r\n"))
			start := m[1] + gap
			tail := doc.Text[start:]
			end := strings.Index(tail, "\n")
			if end < 0 {
				end = len(tail)
			}
			description := strings.TrimSpace(tail[:end])
			if description == "" || strings.HasPrefix(description, "#") || strings.HasPrefix(description, "![") || strings.HasPrefix(description, "1.") {
				continue
			}
			quote := doc.Text[m[0] : start+end]
			if len(quote) > 1000 {
				continue
			}
			d.Name = name
			d.Description = description
			d.Evidence = domain.Citation{SourceID: doc.ID, Quote: quote}
			// The original model amount can have been invented; re-establish pricing separately.
			d.Price = ""
			d.Currency = ""
			d.PriceText = ""
			d.PriceEvidence = domain.Citation{}
			return true
		}
	}
	return false
}
