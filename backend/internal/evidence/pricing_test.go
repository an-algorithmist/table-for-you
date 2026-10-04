package evidence

import (
	"table-for-you/backend/internal/domain"
	"testing"
	"time"
)

func TestSpendEstimateUsesEvidenceWithoutInventingItemPrices(t *testing.T) {
	docs := map[string]domain.Document{"m": {ID: "m", Text: "Soup PLN 27.00. Lunch set PLN 44.00."}}
	r := domain.Restaurant{Dishes: []domain.Dish{{Name: "Soup", Price: "27.00", Currency: "PLN", Evidence: domain.Citation{SourceID: "m", Quote: "Soup PLN 27.00"}}, {Name: "Lunch set", Price: "44.00", Currency: "PLN", Evidence: domain.Citation{SourceID: "m", Quote: "Lunch set PLN 44.00"}}, {Name: "Unpriced"}}}
	estimateSpend(&r, docs)
	if r.Estimate.Low != "27.00" || r.Estimate.High != "55.00" || r.Estimate.Currency != "PLN" || r.Dishes[2].Price != "" {
		t.Fatalf("bad estimate %+v", r)
	}
	r.Dishes = nil
	r.PriceRange = domain.PriceRange{Low: "20", High: "30", Currency: "PLN", Evidence: domain.Citation{SourceID: "m", Quote: "Soup PLN 27.00"}}
	estimateSpend(&r, docs)
	if r.Estimate.Low != "" {
		t.Fatal("fabricated range accepted")
	}
}
func TestVenueTagRequiresWholeVenueEvidence(t *testing.T) {
	for _, tc := range []struct {
		text string
		want string
	}{{"Sample Kitchen is a vegan restaurant.", "vegan"}, {"Sample Kitchen offers vegan options.", "unknown"}, {"Sample Kitchen is not a vegan restaurant.", "unknown"}} {
		r := domain.Restaurant{Candidate: domain.Candidate{Name: "Sample Kitchen"}, VenueType: "vegan", VenueEvidence: domain.Citation{SourceID: "m", Quote: tc.text}}
		validateVenue(&r, map[string]domain.Document{"m": {Text: tc.text}})
		if r.VenueType != tc.want {
			t.Fatalf("%s: %s", tc.text, r.VenueType)
		}
	}
}

func TestVenueEvidenceDoesNotProveIngredientExclusions(t *testing.T) {
	doc := domain.Document{ID: "m", Kind: "menu", URL: "https://kitchen.example/menu", Text: "Kitchen is a vegan restaurant. Soup PLN 27.00."}
	raw := domain.Extraction{Restaurants: []domain.Restaurant{{Candidate: domain.Candidate{Name: "Kitchen"}, VenueType: "vegan", VenueEvidence: domain.Citation{SourceID: "m", Quote: "Kitchen is a vegan restaurant."}, Dishes: []domain.Dish{{Name: "Soup", Evidence: domain.Citation{SourceID: "m", Quote: "Soup PLN 27.00."}}}}}}
	result := Validate(domain.Requirements{Diet: "vegetarian", Excluded: []string{"onion"}}, raw, []domain.Document{doc}, time.Now())
	if len(result.Excluded) != 1 {
		t.Fatal("unverified strict exclusion must be excluded")
	}
	foundVeg, foundOnion := false, false
	for _, c := range result.Excluded[0].Dishes[0].Constraints {
		if c.Name == "vegetarian" {
			foundVeg = c.Status == "supported"
		}
		if c.Name == "onion" {
			foundOnion = c.Status == "unknown"
		}
	}
	if !foundVeg || !foundOnion {
		t.Fatal("venue evidence incorrectly applied to ingredient exclusion")
	}
}
