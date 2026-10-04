package evidence

import (
	"time"

	"table-for-you/backend/internal/domain"
)

// Validate reconstructs recommendations from cited evidence and partitions their eligibility.
// Missing evidence never becomes a supported requirement.
func Validate(req domain.Requirements, raw domain.Extraction, docs []domain.Document, now time.Time) domain.Result {
	byID := map[string]domain.Document{}
	for _, d := range docs {
		byID[d.ID] = d
	}
	out := domain.Result{ValidationVersion: "evidence-v4-source-prices", Requirements: req, Confirmed: []domain.Restaurant{}, Alternatives: []domain.Restaurant{}, Excluded: []domain.Restaurant{}, Sources: docs, ResearchedAt: now.UTC(), Limitations: []string{"Review evidence is a selected sample, not a representative survey. Ingredient labels are source claims; cross-contamination is not verified."}}
	for _, r := range raw.Restaurants {
		r, confirmed, unknown, rejected := validateRestaurant(req, r, docs, byID, now)
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
