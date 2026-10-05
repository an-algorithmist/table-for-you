package evidence

import (
	"strings"
	"time"

	"table-for-you/backend/internal/domain"
)

// validateDishes partitions menu-backed dishes by all requested constraints.
// Strict exclusions and non-vegetarian requests reject dishes without explicit support.
func validateDishes(req domain.Requirements, r domain.Restaurant, docs []domain.Document, byID map[string]domain.Document, now time.Time) ([]domain.Dish, []domain.Dish, []domain.Dish) {
	confirmed, unknown, rejected := []domain.Dish{}, []domain.Dish{}, []domain.Dish{}
	for _, d := range r.Dishes {
		if !Supported(d.Evidence, byID) || d.Name == "" || !strings.Contains(normalize(d.Evidence.Quote), normalize(d.Name)) {
			continue
		}
		if req.Meal == "dinner" && breakfastOnly(d.Evidence.Quote) {
			d.Status = "contradicted"
			d.Constraints = []domain.Constraint{{Name: "meal availability", Status: "contradicted", Evidence: d.Evidence, Reason: "This dish passage specifies a breakfast-only or morning-only offer."}}
			rejected = append(rejected, d)
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
	return confirmed, unknown, rejected
}

func breakfastOnly(text string) bool {
	q := strings.ToLower(text)
	return strings.Contains(q, "breakfast only") || strings.Contains(q, "morning only") || strings.Contains(q, "until 11am") || strings.Contains(q, "until 11 am") || strings.Contains(q, "before 11am")
}
