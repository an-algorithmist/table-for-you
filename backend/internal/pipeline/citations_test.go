package pipeline

import (
	"testing"

	"table-for-you/backend/internal/domain"
)

func TestRepairOnlyVerbatimCitationsWithinRestaurant(t *testing.T) {
	menu := domain.Document{ID: "home", URL: "https://restaurant.example", Kind: "menu", Hash: "one", Text: "Kitchen. Soup PLN 27.00."}
	other := domain.Document{ID: "menu", URL: "https://restaurant.example/menu", Kind: "menu", Hash: "two", Text: "Kitchen menu. No soup price."}
	j := job{docs: []domain.Document{menu, other}, docCandidates: map[string]map[string]bool{"home": {"Kitchen": true}, "menu": {"Kitchen": true}}}
	raw := domain.Extraction{Restaurants: []domain.Restaurant{{Candidate: domain.Candidate{Name: "Kitchen"}, Dishes: []domain.Dish{{Name: "Soup", Price: "27.00", PriceText: "PLN 27.00", Currency: "PLN", Evidence: domain.Citation{SourceID: sourceAlias(other), Quote: "Soup PLN 27.00."}}}}}}
	r := j.validate(domain.Requirements{}, raw)
	if len(r.Alternatives) != 1 || r.Alternatives[0].Dishes[0].Evidence.SourceID != "home" {
		t.Fatalf("verbatim citation not recovered: %+v", r)
	}
	raw.Restaurants[0].Dishes[0].Evidence.Quote = "Soup PLN 99.00."
	r = j.validate(domain.Requirements{}, raw)
	if len(r.Alternatives) != 0 {
		t.Fatal("fabricated quote was repaired")
	}
}
