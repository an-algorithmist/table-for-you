package evidence

import (
	"strings"
	"table-for-you/backend/internal/domain"
	"testing"
	"time"
)

func fixture() (domain.Requirements, domain.Extraction, []domain.Document) {
	text := "Green Table — Barcelona, Spain. Lunch all day. Vegan chickpeas — EUR 12.00. Vegan chickpeas are vegan. Delicious vegan dishes. Service was slow."
	doc := domain.Document{ID: "source", Text: text, Kind: "menu"}
	req := domain.Requirements{City: "Barcelona", Country: "Spain", Meal: "lunch", Diet: "vegan", Budget: "15", Currency: "EUR", BudgetBasis: "per_dish"}
	dish := domain.Dish{Name: "Vegan chickpeas", Price: "12.00", PriceText: "EUR 12.00", Currency: "EUR", Meal: "lunch", MenuType: "regular", Evidence: domain.Citation{SourceID: "source", Quote: "Vegan chickpeas — EUR 12.00"}, Constraints: []domain.Constraint{{Name: "vegan", Status: "supported", Evidence: domain.Citation{SourceID: "source", Quote: "Vegan chickpeas are vegan."}}, {Name: "meal availability", Status: "supported", Evidence: domain.Citation{SourceID: "source", Quote: "Lunch all day."}}}}
	r := domain.Restaurant{Candidate: domain.Candidate{Name: "Green Table", Identity: domain.Citation{SourceID: "source", Quote: "Green Table — Barcelona, Spain."}}, Dishes: []domain.Dish{dish}, Reviews: []domain.Review{{Sentiment: "positive", Evidence: domain.Citation{SourceID: "source", Quote: "Delicious vegan dishes."}}, {Sentiment: "negative", Evidence: domain.Citation{SourceID: "source", Quote: "Service was slow."}}}}
	return req, domain.Extraction{Restaurants: []domain.Restaurant{r}}, []domain.Document{doc}
}
func TestSupportedPriceAndBothReviewPerspectives(t *testing.T) {
	req, raw, docs := fixture()
	v := Validate(req, raw, docs, time.Now())
	if len(v.Confirmed) != 1 || len(v.Confirmed[0].Reviews) != 2 {
		t.Fatalf("expected a supported match with both review perspectives: %+v", v)
	}
}
func TestNoIngredientOmissionInference(t *testing.T) {
	req, raw, docs := fixture()
	req.Excluded = []string{"onion", "garlic"}
	raw.Restaurants[0].Dishes[0].Constraints = append(raw.Restaurants[0].Dishes[0].Constraints, domain.Constraint{Name: "garlic", Status: "supported", Evidence: domain.Citation{SourceID: "source", Quote: "Vegan chickpeas are vegan."}})
	v := Validate(req, raw, docs, time.Now())
	if len(v.Confirmed) != 0 || len(v.Excluded) != 1 {
		t.Fatal("unverified strict exclusions must stay out of shortlist")
	}
}
func TestInventedCitationAndPriceAreNotPublishedAsVerified(t *testing.T) {
	req, raw, docs := fixture()
	raw.Restaurants[0].Dishes[0].Evidence.Quote = "Vegan chickpeas — EUR 1.00"
	v := Validate(req, raw, docs, time.Now())
	if len(v.Confirmed)+len(v.Alternatives) != 0 {
		t.Fatal("fabricated passage should remove dish")
	}
	req, raw, docs = fixture()
	raw.Restaurants[0].Dishes[0].Price = "1.00"
	v = Validate(req, raw, docs, time.Now())
	if len(v.Confirmed) != 0 || v.Alternatives[0].Dishes[0].Price != "" {
		t.Fatal("a mismatched price must become unknown")
	}
}
func TestCurrencyBudgetAndExpiredDailyMenu(t *testing.T) {
	req, raw, docs := fixture()
	req.Currency = "USD"
	if len(Validate(req, raw, docs, time.Now()).Confirmed) != 0 {
		t.Fatal("no implicit currency conversion")
	}
	req, raw, docs = fixture()
	req.Budget = "10"
	if len(Validate(req, raw, docs, time.Now()).Excluded) != 1 {
		t.Fatal("over-budget dish not excluded")
	}
	req, raw, docs = fixture()
	raw.Restaurants[0].Dishes[0].MenuType = "daily"
	raw.Restaurants[0].Dishes[0].ValidDate = "2000-01-01"
	if len(Validate(req, raw, docs, time.Now()).Confirmed) != 0 {
		t.Fatal("old daily menu was classified current")
	}
}
func TestSnippetCannotEstablishConfirmedDietaryMatch(t *testing.T) {
	req, raw, docs := fixture()
	docs[0].Snippet = true
	if len(Validate(req, raw, docs, time.Now()).Confirmed) != 0 {
		t.Fatal("snippet should remain incomplete evidence")
	}
}

func TestMealEvidenceRequiredAndOlderSourcePricesAccepted(t *testing.T) {
	req, raw, docs := fixture()
	raw.Restaurants[0].Dishes[0].Constraints = raw.Restaurants[0].Dishes[0].Constraints[:1]
	docs[0].Text = strings.ReplaceAll(docs[0].Text, "Lunch all day.", "")
	if len(Validate(req, raw, docs, time.Now()).Confirmed) != 0 {
		t.Fatal("model meal field without source proof established lunch availability")
	}
	req, raw, docs = fixture()
	docs[0].Historical = true
	if len(Validate(req, raw, docs, time.Now()).Confirmed) != 1 {
		t.Fatal("older source-listed price should remain usable for planning")
	}
	if dietMarker("vegan", "This dish is not vegan.") {
		t.Fatal("negated dietary label was accepted")
	}
	if samePrice("3", "€8 (3 pcs) / €11.50 (5 pcs)") {
		t.Fatal("portion quantity was accepted as price")
	}
}

func TestDietEvidenceCannotBeBorrowedFromAnotherDish(t *testing.T) {
	req, raw, docs := fixture()
	docs[0].Text += " Vegan soup is vegan."
	raw.Restaurants[0].Dishes[0].Constraints[0].Evidence.Quote = "Vegan soup is vegan."
	if len(Validate(req, raw, docs, time.Now()).Confirmed) != 0 {
		t.Fatal("another dish's dietary label established support")
	}
	d := domain.Dish{Name: "Pizza", Evidence: domain.Citation{SourceID: "menu", Quote: "Pizza V/GF 12"}}
	c := domain.Constraint{Evidence: domain.Citation{SourceID: "menu", Quote: "V - VEGAN GF - GLUTEN FREE"}}
	if !boundToDish("vegan", d, c) {
		t.Fatal("explicit symbol legend not decoded")
	}
	d.Evidence.Quote = "Pizza GF 12"
	if boundToDish("vegan", d, c) {
		t.Fatal("legend upgraded an unlabelled dish")
	}
}

func TestConditionalOrNegatedIngredientClaimsRemainUnknown(t *testing.T) {
	for _, quote := range []string{"Soup can be prepared without garlic on request.", "Soup is not garlic-free."} {
		if dietMarker("garlic", quote) {
			t.Fatal("conditional or negated exclusion claim was accepted")
		}
	}
}
