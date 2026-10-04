package evidence

import (
	"net/url"
	"regexp"
	"strings"
	"table-for-you/backend/internal/domain"
)

func containsName(names []string, name string) bool {
	for _, v := range names {
		if strings.EqualFold(v, name) {
			return true
		}
	}
	return false
}
func identityMatches(quote string, doc domain.Document, req domain.Requirements) bool {
	if req.City == "" || req.Country == "" {
		return false
	}
	q := normalize(quote)
	if !strings.Contains(q, normalize(req.City)) {
		return false
	}
	text := normalize(doc.Text + " " + doc.URL)
	if strings.Contains(text, normalize(req.Country)) {
		return true
	}
	aliases := map[string][]string{"united states": {"usa", "united states of america", "new york, ny", "new york ny"}, "canada": {"toronto, on", "toronto on", "ontario"}, "brazil": {"brasil", "são paulo - sp", "sao paulo - sp"}, "japan": {"japan", "東京都"}, "spain": {"españa", "barcelona 080", "080 barcelona"}, "india": {"india", "new delhi"}, "argentina": {"argentina", "buenos aires, ar"}}
	for _, a := range aliases[strings.ToLower(req.Country)] {
		if strings.Contains(text, a) {
			return true
		}
	}
	countryTLD := map[string]string{"canada": ".ca", "india": ".in", "japan": ".jp", "brazil": ".br", "argentina": ".ar", "spain": ".es", "france": ".fr"}[strings.ToLower(req.Country)]
	u, _ := url.Parse(doc.URL)
	return u != nil && countryTLD != "" && strings.HasSuffix(u.Hostname(), countryTLD)
}
func recoverIdentity(r *domain.Restaurant, req domain.Requirements, docs []domain.Document) {
	for _, doc := range docs {
		if doc.Snippet {
			continue
		}
		if Supported(r.Identity, map[string]domain.Document{doc.ID: doc}) && identityMatches(r.Identity.Quote, doc, req) && identityBound(*r, doc, r.Identity.Quote) {
			return
		}
	}
	for _, doc := range docs {
		if doc.Snippet {
			continue
		}
		for _, line := range strings.Split(doc.Text, "\n") {
			if identityMatches(line, doc, req) && identityBound(*r, doc, line) && len(strings.TrimSpace(line)) >= 8 {
				r.Identity = domain.Citation{SourceID: doc.ID, Quote: line}
				return
			}
		}
	}
}
func recoverDiet(name string, d domain.Dish, r domain.Restaurant, docs []domain.Document) (domain.Constraint, bool) {
	for _, doc := range docs {
		if doc.Snippet || doc.Historical || doc.Kind != "menu" {
			continue
		}
		if doc.ID != d.Evidence.SourceID {
			u, _ := url.Parse(doc.URL)
			m, _ := url.Parse(r.OfficialURL)
			if m == nil || u == nil || m.Hostname() == "" || m.Hostname() != u.Hostname() {
				continue
			}
		}
		for _, line := range strings.Split(doc.Text, "\n") {
			c := domain.Constraint{Name: name, Status: "supported", Evidence: domain.Citation{SourceID: doc.ID, Quote: line}, Reason: "Explicit statement in this restaurant's retrieved menu."}
			global := false
			for _, scope := range []string{"all menu items", "all dishes", "entire menu", "100% vegan", "100% vegetarian", "fully vegan", "fully vegetarian", "exclusively vegan", "exclusively vegetarian", "100% gluten-free", "100% gluten free"} {
				global = global || strings.Contains(normalize(line), scope)
			}
			if global && len(strings.TrimSpace(line)) >= 8 && dietMarker(name, line) && boundToDish(name, d, c) {
				return c, true
			}
		}
	}
	return domain.Constraint{}, false
}
func recoverMeal(meal string, docs []domain.Document) domain.Citation {
	markers := map[string]string{"lunch": `(?i)\b(lunch|almuerzo|almoço|déjeuner|pranzo|mittag)\b|ランチ`, "dinner": `(?i)\b(dinner|cena|jantar|dîner|abendessen)\b|ディナー`, "breakfast": `(?i)\b(breakfast|desayuno|petit déjeuner|colazione|frühstück)\b|朝食`}[meal]
	if markers == "" {
		return domain.Citation{}
	}
	re := regexp.MustCompile(markers + `|(?i)\b(all[ -]day|todo el día|toute la journée)\b`)
	for _, doc := range docs {
		if doc.Snippet || doc.Kind != "menu" {
			continue
		}
		for _, line := range strings.Split(doc.Text, "\n") {
			if re.MatchString(normalize(line)) && len(line) >= 8 {
				return domain.Citation{SourceID: doc.ID, Quote: line}
			}
		}
	}
	return domain.Citation{}
}

func addressInDocument(address, text string) bool {
	if address == "" {
		return true
	}
	first := strings.Split(address, ",")[0]
	words := strings.Fields(normalize(first))
	if len(words) < 2 {
		return true
	}
	anchor := strings.Join(words[:2], " ")
	return strings.Contains(normalize(text), anchor)
}

var identitySeparators = regexp.MustCompile(`[^\p{L}\p{N}]+`)

func identityBound(r domain.Restaurant, doc domain.Document, quote string) bool {
	text := normalize(doc.Text)
	q := normalize(quote)
	at := strings.Index(text, q)
	if at < 0 {
		return false
	}
	start, end := at-500, at+len(q)+500
	if start < 0 {
		start = 0
	}
	if end > len(text) {
		end = len(text)
	}
	window := text[start:end]
	name := identitySeparators.ReplaceAllString(normalize(r.Name), "")
	if name == "" {
		return false
	}
	nearby := strings.Contains(identitySeparators.ReplaceAllString(window, ""), name)
	dedicated := strings.Contains(identitySeparators.ReplaceAllString(normalize(doc.Title+" "+doc.URL), ""), name)
	return (nearby || dedicated) && addressInDocument(r.Address, window)
}
