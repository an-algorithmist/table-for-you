package evidence

import (
	"fmt"
	"html"
	"math/big"
	"nebulaiq/internal/domain"
	"regexp"
	"strings"
	"time"
	"unicode"
)

var markdownLink = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)

func normalize(s string) string {
	s = html.UnescapeString(s)
	s = markdownLink.ReplaceAllString(s, "$1")
	s = strings.NewReplacer("**", "", "__", "", "`", "").Replace(s)
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}
func Supported(c domain.Citation, docs map[string]domain.Document) bool {
	d, ok := docs[c.SourceID]
	return ok && len(strings.TrimSpace(c.Quote)) >= 8 && strings.Contains(normalize(d.Text), normalize(c.Quote))
}
func Mandatory(req domain.Requirements) []string {
	out := []string{}
	if req.Diet != "" && req.Diet != "no restriction" {
		out = append(out, req.Diet)
	}
	for _, x := range req.Excluded {
		if strings.TrimSpace(x) != "" {
			out = append(out, strings.ToLower(x))
		}
	}
	if req.VenueOnly {
		out = append(out, "vegetarian venue")
	}
	return out
}
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
func currencySupported(currency, quote string) bool {
	return currency != "" && currencyInText(quote) == currency
}

var veganSymbol = regexp.MustCompile(`\bV(?:/GF)?\b`)
var glutenSymbol = regexp.MustCompile(`\bGF\b`)

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
func number(v string) (*big.Rat, bool) {
	v = strings.TrimSpace(v)
	if strings.Contains(v, ",") {
		if strings.Contains(v, ".") {
			if strings.LastIndex(v, ",") > strings.LastIndex(v, ".") {
				v = strings.ReplaceAll(v, ".", "")
				v = strings.ReplaceAll(v, ",", ".")
			} else {
				v = strings.ReplaceAll(v, ",", "")
			}
		} else {
			parts := strings.Split(v, ",")
			if len(parts[len(parts)-1]) == 3 {
				v = strings.Join(parts, "")
			} else {
				v = strings.ReplaceAll(v, ",", ".")
			}
		}
	} else if strings.Count(v, ".") > 1 || strings.Contains(v, ".") && len(v)-strings.LastIndex(v, ".")-1 == 3 {
		v = strings.ReplaceAll(v, ".", "")
	}
	r, ok := new(big.Rat).SetString(v)
	return r, ok && r.Sign() >= 0
}
func samePrice(price, text string) bool {
	var nums []string
	var b strings.Builder
	flush := func() {
		if b.Len() > 0 {
			nums = append(nums, b.String())
			b.Reset()
		}
	}
	for _, r := range text {
		if unicode.IsDigit(r) || r == '.' || r == ',' {
			b.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	p, ok := number(price)
	if !ok {
		return false
	}
	if len(nums) == 0 {
		return false
	}
	v, ok := number(nums[0])
	return ok && v.Cmp(p) == 0
}

// Validate never upgrades missing evidence into support. Output is reconstructed from cited records.
func Validate(req domain.Requirements, raw domain.Extraction, docs []domain.Document, now time.Time) domain.Result {
	byID := map[string]domain.Document{}
	for _, d := range docs {
		byID[d.ID] = d
	}
	out := domain.Result{ValidationVersion: "evidence-v4-source-prices", Requirements: req, Confirmed: []domain.Restaurant{}, Alternatives: []domain.Restaurant{}, Excluded: []domain.Restaurant{}, Sources: docs, ResearchedAt: now.UTC(), Limitations: []string{"Review evidence is a selected sample, not a representative survey. Ingredient labels are source claims; cross-contamination is not verified."}}
	for _, r := range raw.Restaurants {
		// Free-form model notes can contradict validated constraints. Rebuild them.
		r.Limitations = nil
		validateVenue(&r, byID)
		recoverIdentity(&r, req, docs)
		r.IdentityVerified = Supported(r.Identity, byID) && identityMatches(r.Identity.Quote, byID[r.Identity.SourceID], req) && identityBound(r, byID[r.Identity.SourceID], r.Identity.Quote)
		if !r.IdentityVerified {
			r.Identity = domain.Citation{}
		}
		if !r.IdentityVerified {
			r.Limitations = append(r.Limitations, "Restaurant identity/city/country is not fully verified by the cited passage.")
		}
		reviews := []domain.Review{}
		seen := map[string]bool{}
		positive, negative := false, false
		for _, v := range r.Reviews {
			key := v.Evidence.SourceID + normalize(v.Evidence.Quote)
			if !Supported(v.Evidence, byID) || seen[key] || (v.Sentiment != "positive" && v.Sentiment != "negative") {
				continue
			}
			seen[key] = true
			reviews = append(reviews, v)
			positive = positive || v.Sentiment == "positive"
			negative = negative || v.Sentiment == "negative"
		}
		r.Reviews = reviews
		if !positive {
			r.Limitations = append(r.Limitations, "No supported positive review passage was available.")
		}
		if !negative {
			r.Limitations = append(r.Limitations, "No supported negative review passage was available.")
		}
		confirmed, unknown, rejected := []domain.Dish{}, []domain.Dish{}, []domain.Dish{}
		for _, d := range r.Dishes {
			if !Supported(d.Evidence, byID) || d.Name == "" || !strings.Contains(normalize(d.Evidence.Quote), normalize(d.Name)) {
				continue
			}
			state := "supported"
			mealEvidence := domain.Citation{}
			for _, constraint := range d.Constraints {
				if constraint.Name == "meal availability" {
					mealEvidence = constraint.Evidence
				}
			}
			assess := []domain.Constraint{}
			for _, name := range Mandatory(req) {
				v := domain.Constraint{Name: name, Status: "unknown", Reason: "No explicit, supported source label or exclusion statement."}
				for _, c := range d.Constraints {
					if normalize(c.Name) != normalize(name) {
						continue
					}
					if c.Status == "contradicted" && Supported(c.Evidence, byID) {
						v = c
						break
					}
					if c.Status == "supported" && Supported(c.Evidence, byID) && dietMarker(name, c.Evidence.Quote) && boundToDish(name, d, c) && (!byID[c.Evidence.SourceID].Snippet || name == "non-vegetarian") {
						v = c
					}
					break
				}
				if name == "non-vegetarian" && containsMeat(d.Evidence.Quote) && !byID[d.Evidence.SourceID].Snippet {
					v = domain.Constraint{Name: name, Status: "supported", Evidence: d.Evidence, Reason: "The menu explicitly includes meat or fish."}
				}
				venueMatches := (name == "vegetarian" || name == "vegetarian venue") && (r.VenueType == "vegan" || r.VenueType == "vegetarian") || name == "vegan" && r.VenueType == "vegan"
				if v.Status == "unknown" && venueMatches && Supported(r.VenueEvidence, byID) && !byID[r.VenueEvidence.SourceID].Snippet {
					v = domain.Constraint{Name: name, Status: "supported", Evidence: r.VenueEvidence, Reason: "The retrieved source describes the whole venue as " + r.VenueType + "."}
				}
				if v.Status == "contradicted" {
					state = "contradicted"
				} else if v.Status != "supported" && state != "contradicted" {
					state = "unknown"
				}
				if v.Status == "unknown" {
					if c, ok := recoverDiet(name, d, r, docs); ok {
						v = c
					}
				}

				assess = append(assess, v)
			}
			state = "supported"
			for _, c := range assess {
				if c.Status == "contradicted" {
					state = "contradicted"
					break
				}
				if c.Status != "supported" {
					state = "unknown"
				}
			}
			d.Constraints = assess
			if !mealSupported(req.Meal, mealEvidence, byID) {
				mealEvidence = recoverMeal(req.Meal, docs)
			}
			if !r.IdentityVerified && state != "contradicted" {
				state = "unknown"
			}
			if byID[d.Evidence.SourceID].Snippet && state != "contradicted" {
				state = "unknown"
				d.Constraints = append(d.Constraints, domain.Constraint{Name: "menu completeness", Status: "unknown", Reason: "Only a search snippet is available, not an extracted menu."})
			}
			if req.Meal != "" && !mealSupported(req.Meal, mealEvidence, byID) && state != "contradicted" {
				state = "unknown"
				d.Constraints = append(d.Constraints, domain.Constraint{Name: "meal availability", Status: "unknown", Reason: "The cited menu does not establish availability for the requested meal."})
			}
			if d.MenuType == "daily" {
				date, e := time.Parse("2006-01-02", d.ValidDate)
				if e != nil || date.Format("2006-01-02") != now.UTC().Format("2006-01-02") {
					if state != "contradicted" {
						state = "unknown"
					}
					d.Constraints = append(d.Constraints, domain.Constraint{Name: "daily menu date", Status: "unknown", Reason: "Daily menu validity does not establish current availability."})
				}
			}
			d.CurrencyEvidence = domain.Citation{}
			d.CurrencyBasis = "source"
			priceCitation := d.Evidence
			if Supported(d.PriceEvidence, byID) && strings.Contains(normalize(d.PriceEvidence.Quote), normalize(d.Name)) {
				priceCitation = d.PriceEvidence
			}
			if d.Price != "" && (!samePrice(d.Price, d.PriceText) || !strings.Contains(normalize(priceCitation.Quote), normalize(d.PriceText))) {
				d.Price = ""
				d.PriceText = ""
				d.Currency = ""
			}
			if !currencySupported(d.Currency, priceCitation.Quote) {
				d.Currency = ""
			}
			d.PriceEvidence = priceCitation
			if d.Price != "" && d.Currency == "" {
				d.Currency, d.CurrencyEvidence = menuCurrency(byID[priceCitation.SourceID])
			}
			if d.Price == "" {
				d.PriceEvidence = domain.Citation{}
			}
			if d.Price == "" || d.Currency == "" {
				recoverListedPrice(&d, r.Name, docs)
			}
			if d.Price != "" && d.Currency == "" && r.IdentityVerified && !byID[d.PriceEvidence.SourceID].Snippet {
				d.Currency = localCurrency(req.Country)
				if d.Currency != "" {
					d.CurrencyBasis = "location_assumption"
					d.CurrencyEvidence = r.Identity
				}
			}
			if req.Budget != "" {
				p, ok := number(d.Price)
				budget, bok := number(req.Budget)
				c := domain.Constraint{Name: "budget", Status: "unknown", Reason: "A listed price in the requested currency and budget basis is required."}
				if ok && bok && d.Currency == req.Currency && req.BudgetBasis == "per_dish" {
					if p.Cmp(budget) <= 0 {
						c.Status = "supported"
						c.Reason = "Listed dish price is within the per-dish budget."
					} else {
						c.Status = "contradicted"
						c.Reason = "Listed dish price exceeds the budget."
					}
				}
				d.Constraints = append(d.Constraints, c)
				if c.Status == "contradicted" {
					state = "contradicted"
				} else if c.Status != "supported" && state != "contradicted" {
					state = "unknown"
				}
			}
			d.Status = state
			if len(req.Excluded) > 0 || req.Diet == "gluten-free" || req.Diet == "vegan" {
				eligible := true
				for _, c := range d.Constraints {
					if (c.Name == req.Diet || containsName(req.Excluded, c.Name)) && c.Status != "supported" {
						eligible = false
					}
				}
				if !eligible {
					rejected = append(rejected, d)
					continue
				}
			}
			if req.Diet == "non-vegetarian" {
				matched := false
				for _, c := range d.Constraints {
					if c.Name == "non-vegetarian" && c.Status == "supported" {
						matched = true
					}
				}
				if !matched {
					rejected = append(rejected, d)
					continue
				}
			}
			switch state {
			case "supported":
				confirmed = append(confirmed, d)
			case "contradicted":
				rejected = append(rejected, d)
			default:
				unknown = append(unknown, d)
			}
		}
		if len(confirmed) > 0 {
			v := r
			v.Dishes = confirmed
			estimateSpend(&v, byID)
			out.Confirmed = append(out.Confirmed, v)
		}
		if len(unknown) > 0 {
			v := r
			v.Dishes = unknown
			estimateSpend(&v, byID)
			out.Alternatives = append(out.Alternatives, v)
		}
		if len(rejected) > 0 {
			v := r
			v.Dishes = rejected
			estimateSpend(&v, byID)
			out.Excluded = append(out.Excluded, v)
		}
	}
	if len(out.Confirmed) == 0 {
		out.Limitations = append(out.Limitations, "No restaurant/dish was confirmed against every mandatory requirement. Alternatives require restaurant confirmation.")
	}
	if len(out.Confirmed) > 3 {
		out.Confirmed = out.Confirmed[:3]
	}
	if len(out.Alternatives) > 3 {
		out.Alternatives = out.Alternatives[:3]
	}
	return out
}
func Summary(req domain.Requirements) string {
	return fmt.Sprintf("%s in %s, %s; %s; exclusions: %s", req.Meal, req.City, req.Country, req.Diet, strings.Join(req.Excluded, ", "))
}
