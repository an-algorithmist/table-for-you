package evidence

import (
	"time"

	"table-for-you/backend/internal/domain"
)

// validateRestaurant binds venue identity and review claims before checking dishes.
// Model-supplied limitations are rebuilt from the actual retrieved evidence.
func validateRestaurant(req domain.Requirements, r domain.Restaurant, docs []domain.Document, byID map[string]domain.Document, now time.Time) (domain.Restaurant, []domain.Dish, []domain.Dish, []domain.Dish) {
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
	confirmed, unknown, rejected := validateDishes(req, r, docs, byID, now)
	return r, confirmed, unknown, rejected
}
