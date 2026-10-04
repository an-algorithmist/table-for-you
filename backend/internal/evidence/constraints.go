package evidence

import (
	"regexp"
	"strings"

	"table-for-you/backend/internal/domain"
)

var veganSymbol = regexp.MustCompile(`\bV(?:/GF)?\b`)
var glutenSymbol = regexp.MustCompile(`\bGF\b`)

func dietMarker(name, quote string) bool {
	q := normalize(quote)
	for _, conditional := range []string{"on request", "upon request", "if requested", "can be prepared", "can be made", "may be prepared"} {
		if strings.Contains(q, conditional) {
			return false
		}
	}
	for _, negative := range []string{"not " + name + "-free", "not " + name + " free", "not without " + name, "no longer " + name + "-free"} {
		if strings.Contains(q, negative) {
			return false
		}
	}
	for _, negative := range []string{"not vegan", "not vegetarian", "non vegan", "non-vegetarian", "no es vegano", "no es vegetariano", "not gluten-free", "not gluten free"} {
		if strings.Contains(q, negative) {
			return false
		}
	}
	var markers []string
	switch name {
	case "vegetarian":
		markers = []string{"vegetarian", "vegetari", "végétari", "vegetarisch", "vegan", "vegano", "vegana", "végétal", "plant-based"}
	case "vegan":
		markers = []string{"vegan", "végane", "végétalien", "100% plant-based", "fully plant-based"}
	case "gluten-free":
		markers = []string{"gluten-free", "gluten free", "sin gluten", "sans gluten", "senza glutine", "glutenfrei"}
	case "non-vegetarian":
		return containsMeat(quote)
	case "no restriction":
		return true
	case "vegetarian venue":
		markers = []string{"100% vegetarian", "100% vegan", "fully vegetarian", "fully vegan", "exclusively vegetarian", "exclusively vegan", "100% vegetariano", "100% vegano"}
	default:
		if strings.Contains(q, name+"-free") || strings.Contains(q, name+" free") {
			return true
		}
		for _, prefix := range []string{"no ", "without ", "free from ", "sin ", "sans ", "senza "} {
			if strings.Contains(q, prefix+name) {
				return true
			}
		}
		if name == "onion" {
			markers = []string{"sin cebolla", "sans oignon", "senza cipolla"}
		}
		if name == "garlic" {
			markers = []string{"sin ajo", "sans ail", "senza aglio", "no onion or garlic", "without onion or garlic", "no onion and garlic", "without onion and garlic"}
		}
	}
	for _, m := range markers {
		if strings.Contains(q, m) {
			return true
		}
	}
	return false
}

func mealSupported(meal string, c domain.Citation, docs map[string]domain.Document) bool {
	if !Supported(c, docs) || docs[c.SourceID].Snippet {
		return false
	}
	q := normalize(c.Quote)
	markers := map[string][]string{"breakfast": {"breakfast", "desayuno", "petit déjeuner", "colazione", "frühstück", "朝食"}, "lunch": {"lunch", "almuerzo", "déjeuner", "pranzo", "mittag", "mediodía", "almoço", "ランチ"}, "dinner": {"dinner", "cena", "dîner", "abendessen", "jantar", "ディナー"}}
	for _, m := range append(markers[meal], "all day", "all-day", "todo el día", "toute la journée") {
		if strings.Contains(q, m) {
			return true
		}
	}
	return false
}

func boundToDish(name string, d domain.Dish, c domain.Constraint) bool {
	q := normalize(c.Evidence.Quote)
	if strings.Contains(q, normalize(d.Name)) {
		return true
	}
	if c.Evidence.SourceID == d.Evidence.SourceID {
		if (name == "vegan" || name == "vegetarian") && strings.Contains(q, "v - vegan") && veganSymbol.MatchString(d.Evidence.Quote) {
			return true
		}
		if name == "gluten-free" && strings.Contains(q, "gf - gluten free") && glutenSymbol.MatchString(d.Evidence.Quote) {
			return true
		}
	}
	for _, global := range []string{"100% vegan", "100% vegetarian", "fully vegan", "fully vegetarian", "exclusively vegan", "exclusively vegetarian", "all dishes", "entire menu", "all menu items", "100% gluten-free", "100% gluten free", "we cook"} {
		if strings.Contains(q, global) {
			return true
		}
	}
	return false
}
