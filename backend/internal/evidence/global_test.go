package evidence

import (
	"nebulaiq/internal/domain"
	"strings"
	"testing"
	"time"
)

func TestGlobalGlutenFreeStatementAndLocalCurrency(t *testing.T) {
	doc := domain.Document{ID: "m", URL: "https://example.ca/menu", Kind: "menu", Text: "Kitchen — Toronto, Ontario, Canada.\nAll day menu\nAll menu items and baked goods are gluten-free.\nFish & chips $28.75\n"}
	raw := domain.Extraction{Restaurants: []domain.Restaurant{{Candidate: domain.Candidate{Name: "Kitchen", OfficialURL: "https://example.ca", Identity: domain.Citation{SourceID: "m", Quote: "Kitchen — Toronto, Ontario, Canada."}}, Dishes: []domain.Dish{{Name: "Fish & chips", Price: "28.75", PriceText: "$28.75", Evidence: domain.Citation{SourceID: "m", Quote: "Fish & chips $28.75"}}}}}}
	req := domain.Requirements{City: "Toronto", Country: "Canada", Meal: "dinner", Diet: "gluten-free", Budget: "35", Currency: "CAD", BudgetBasis: "per_dish"}
	r := Validate(req, raw, []domain.Document{doc}, time.Now())
	if len(r.Confirmed) != 1 {
		t.Fatalf("%+v", r)
	}
	d := r.Confirmed[0].Dishes[0]
	if d.Currency != "CAD" || d.CurrencyBasis != "location_assumption" {
		t.Fatalf("%+v", d)
	}
	doc.Text = "Kitchen — Toronto, Ontario, Canada.\nAll day menu\nGluten-free options available.\nFish & chips $28.75\n"
	r = Validate(req, raw, []domain.Document{doc}, time.Now())
	if len(r.Confirmed)+len(r.Alternatives) != 0 {
		t.Fatal("options must not establish whole-menu suitability")
	}
}
func TestLocaleNumberNormalization(t *testing.T) {
	for _, tc := range []struct{ s, w string }{{"12.500", "12500.00"}, {"12,500", "12500.00"}, {"1.250,50", "1250.50"}, {"1,250.50", "1250.50"}, {"15,5", "15.50"}, {"28.75", "28.75"}} {
		n, ok := number(tc.s)
		if !ok || n.FloatString(2) != tc.w {
			t.Errorf("%s: %v", tc.s, n)
		}
	}
}
func TestNewCurrencyMarkers(t *testing.T) {
	for _, tc := range []struct{ s, w string }{{"Soup CAD 12", "CAD"}, {"Soup R$ 17,00", "BRL"}, {"Tuna JPY 1500", "JPY"}, {"Steak ARS 12.500", "ARS"}, {"Dish $15", ""}, {"Dish ¥1500", ""}} {
		if got := currencyInText(tc.s); got != tc.w {
			t.Errorf("%s: %s", tc.s, got)
		}
	}
}
func TestInvalidIdentityCleared(t *testing.T) {
	req, raw, docs := fixture()
	raw.Restaurants[0].Identity = domain.Citation{SourceID: "missing", Quote: "Invented city and address"}
	docs[0].Text = "Vegan chickpeas — EUR 12.00"
	r := Validate(req, raw, docs, time.Now())
	for _, v := range r.Alternatives {
		if v.Identity.SourceID != "" {
			t.Fatal("invalid identity retained")
		}
	}
}

func TestIdentityRejectsOtherRestaurantOnCityGuide(t *testing.T) {
	doc := domain.Document{ID: "guide", Kind: "menu", URL: "https://guide.example/new-york", Text: "Sam's Falafel at Cedar Street, New York, NY USA.\n" + strings.Repeat("unrelated guide content ", 70) + "\nPisillo 97 Nassau St, New York, NY USA."}
	r := domain.Restaurant{Candidate: domain.Candidate{Name: "Sam's Falafel", Address: "Cedar Street, New York", Identity: domain.Citation{SourceID: "guide", Quote: "Pisillo 97 Nassau St, New York, NY USA."}}}
	req := domain.Requirements{City: "New York", Country: "United States"}
	if identityBound(r, doc, r.Identity.Quote) {
		t.Fatal("unrelated restaurant address accepted")
	}
	recoverIdentity(&r, req, []domain.Document{doc})
	if !strings.Contains(r.Identity.Quote, "Sam's Falafel") {
		t.Fatal("correct restaurant identity was not recovered")
	}
}
