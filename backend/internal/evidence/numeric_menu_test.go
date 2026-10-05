package evidence

import (
	"encoding/json"
	"os"
	"strings"
	"table-for-you/backend/internal/domain"
	"testing"
)

func TestBareTablePriceRequiresSameMenuCurrency(t *testing.T) {
	for _, currency := range []bool{true, false} {
		text := "Kitchen menu\n| Pumpkin ravioli | 18.50 |\n"
		if currency {
			text = "All prices in EUR\n" + text
		}
		d := domain.Dish{Name: "Pumpkin ravioli"}
		recoverListedPrice(&d, "Kitchen", []domain.Document{{ID: "menu", Title: "Kitchen menu", Text: text, Kind: "menu"}})
		if currency && (d.Price != "18.50" || d.Currency != "EUR") {
			t.Fatalf("supported numeric row lost: %+v", d)
		}
		if !currency && d.Price != "" {
			t.Fatal("bare ambiguous row accepted")
		}
	}
}

func TestExplicitSnippetPricePreservesPortion(t *testing.T) {
	d := domain.Dish{Name: "Koftas"}
	recoverListedPrice(&d, "Kitchen", []domain.Document{{ID: "snippet", Kind: "menu", Snippet: true, Text: "Koftas€8 (3 pcs) / €11.50 (5 pcs)"}})
	if d.Price != "8.00" || d.Currency != "EUR" || !strings.Contains(d.PriceEvidence.Quote, "3 pcs") {
		t.Fatalf("named portion price lost: %+v", d)
	}
}

func TestRecordedBarcelonaPriceRecovery(t *testing.T) {
	b, err := os.ReadFile("testdata/barcelona-pricing.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Dish   domain.Dish     `json:"dish"`
		Source domain.Document `json:"source"`
	}
	if err = json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	recoverListedPrice(&f.Dish, "Aguaribay", []domain.Document{f.Source})
	if f.Dish.Price != "8.00" || f.Dish.Currency != "EUR" || !strings.Contains(f.Source.Text, f.Dish.PriceEvidence.Quote) {
		t.Fatalf("recorded price not recovered: %+v", f.Dish)
	}
}

func TestMorningOfferIsNotDinner(t *testing.T) {
	if !breakfastOnly("Mackerel set JPY 627 until 11AM") || breakfastOnly("Dinner mackerel JPY 900") {
		t.Fatal("meal contradiction detection failed")
	}
}
