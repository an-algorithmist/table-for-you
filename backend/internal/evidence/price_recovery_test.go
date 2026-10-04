package evidence

import (
	"nebulaiq/internal/domain"
	"testing"
)

func TestPriceBeforeDishHeadingAndPolishCurrency(t *testing.T) {
	doc := domain.Document{ID: "menu", Kind: "menu", Text: "Magari\n\n14.00 zł\n\n**Focaccia della casa**\n\nHomemade bread.\n\n31.00 zł\n\n**Caponata**\nVegetables."}
	for _, tc := range []struct{ name, price string }{{"Focaccia della casa", "14.00"}, {"Caponata", "31.00"}, {"Unpriced dish", ""}} {
		d := domain.Dish{Name: tc.name}
		recoverListedPrice(&d, "Magari", []domain.Document{doc})
		if d.Price != tc.price {
			t.Fatalf("%s: %+v", tc.name, d)
		}
		if d.Price != "" && (!Supported(d.PriceEvidence, map[string]domain.Document{"menu": doc}) || d.Currency != "PLN") {
			t.Fatal("price citation invalid")
		}
	}
	d := domain.Dish{Name: "Caponata"}
	recoverListedPrice(&d, "Other venue", []domain.Document{doc})
	if d.Price != "" {
		t.Fatal("cross-restaurant price attached")
	}
}
func TestPriceAfterDishHeading(t *testing.T) {
	d := domain.Dish{Name: "Rosół Z Dyni"}
	recoverListedPrice(&d, "Lokal Vegan Bistro", []domain.Document{{ID: "m", Kind: "menu", Text: "Lokal Vegan Bistro\nRosół Z Dyni\n\n$ PLN 27.00\nPumpkin broth"}})
	if d.Price != "27.00" || d.Currency != "PLN" {
		t.Fatalf("price not recovered: %+v", d)
	}
}

func TestOlderThirdPartyMenuPriceRecovery(t *testing.T) {
	doc := domain.Document{ID: "old", Kind: "menu", Historical: true, URL: "https://guide.example/2022/menu", Text: "Green Table\nSoup\nEUR 12.00\nVegetable soup."}
	dish := domain.Dish{Name: "Soup"}
	recoverListedPrice(&dish, "Green Table", []domain.Document{doc})
	if dish.Price != "12.00" || dish.Currency != "EUR" || dish.PriceEvidence.SourceID != "old" {
		t.Fatalf("source-listed older third-party price lost: %+v", dish)
	}
	r := domain.Restaurant{Dishes: []domain.Dish{dish}}
	estimateSpend(&r, map[string]domain.Document{"old": doc})
	if r.Estimate.Low != "12.00" || r.Estimate.High != "15.00" {
		t.Fatalf("older listed price excluded from estimate: %+v", r.Estimate)
	}
}
