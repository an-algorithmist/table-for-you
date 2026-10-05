package pipeline

import (
	"fmt"
	"strings"

	"table-for-you/backend/internal/domain"
)

type researchGap struct{ Candidate, Query, Kind, Reason string }

func nextResearchGap(req domain.Requirements, result domain.Result, candidates []domain.Candidate, attempted map[string]bool) researchGap {
	restaurants := append(append(append([]domain.Restaurant{}, result.Confirmed...), result.Alternatives...), result.Excluded...)
	for _, c := range candidates {
		if attempted[c.Name] {
			continue
		}
		var r *domain.Restaurant
		for i := range restaurants {
			if restaurants[i].Name == c.Name {
				r = &restaurants[i]
				break
			}
		}
		missingPrice, missingDiet := r == nil || len(r.Dishes) == 0, r == nil || len(r.Dishes) == 0
		if r != nil {
			for _, d := range r.Dishes {
				missingPrice = missingPrice || d.Price == "" || d.Currency == ""

				for _, check := range d.Constraints {
					if (check.Name == req.Diet || containsRequirement(req.Excluded, check.Name)) && check.Status != "supported" {
						missingDiet = true
					}
				}
			}
		}
		if missingPrice || missingDiet {
			focus := "menu prices " + req.Currency
			if missingDiet {
				focus += " " + foodSearchTerms(req) + " ingredients " + strings.Join(req.Excluded, " ")
			}
			return researchGap{c.Name, fmt.Sprintf(`"%s" %s %s %s %s %s %s`, c.Name, c.Address, req.City, req.Country, req.Meal, focus, missingDishNames(r)), "menu", "Checking missing prices, currency or dietary ingredients at " + c.Name}
		}
	}
	for _, r := range restaurants {
		if attempted[r.Name] {
			continue
		}
		negative := false
		for _, v := range r.Reviews {
			negative = negative || v.Sentiment == "negative"
		}
		if !negative {
			return researchGap{r.Name, fmt.Sprintf(`"%s" %s %s negative reviews complaints %s`, r.Name, req.City, req.Country, req.Diet), "review", "Looking for relevant criticism at " + r.Name}
		}
	}
	return researchGap{}
}
func branchHits(hits []domain.SearchHit, req domain.Requirements) []domain.SearchHit {
	out := []domain.SearchHit{}
	for _, h := range hits {
		body := strings.ToLower(h.Title + " " + h.URL)
		country := strings.ToLower(req.Country)
		foreignUS := strings.Contains(body, "parisusa") || strings.Contains(body, " usa") || strings.Contains(body, "united states")
		if foreignUS && country != "usa" && country != "united states" && country != "united states of america" {
			continue
		}
		out = append(out, h)
	}
	return out
}

func containsRequirement(values []string, name string) bool {
	for _, value := range values {
		if strings.EqualFold(value, name) {
			return true
		}
	}
	return false
}

func missingDishNames(r *domain.Restaurant) string {
	if r == nil {
		return ""
	}
	names := []string{}
	for _, d := range r.Dishes {
		if d.Price == "" && len(names) < 2 {
			names = append(names, `"`+d.Name+`"`)
		}
	}
	return strings.Join(names, " ")
}
