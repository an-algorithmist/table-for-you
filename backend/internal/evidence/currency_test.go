package evidence

import (
	"nebulaiq/internal/domain"
	"testing"
)

func TestMenuCurrencyContext(t *testing.T) {
	doc := domain.Document{ID: "menu", Kind: "menu", Text: "Savory stack 15,5\nExtra eggs +4€\nNOK available"}
	code, c := menuCurrency(doc)
	if code != "EUR" || c.SourceID != "menu" || c.Quote != "Extra eggs +4€" {
		t.Fatalf("%s %#v", code, c)
	}
	doc.Text += "\nUSD 20"
	if code, _ := menuCurrency(doc); code != "" {
		t.Fatal("mixed currencies must remain unconfirmed")
	}
	doc.Text = "Paris France\nSavory stack 15,5"
	if code, _ := menuCurrency(doc); code != "" {
		t.Fatal("location cannot establish currency")
	}
}
func TestMeatEvidence(t *testing.T) {
	for _, tc := range []struct {
		q    string
		want bool
	}{{"Pancakes with eggs and bacon", true}, {"Chicken bun with aioli", true}, {"Vegan bacon pancakes", false}, {"Eggs on toast", false}, {"Mont Blanc chestnut pastry", false}} {
		if containsMeat(tc.q) != tc.want {
			t.Errorf("%s", tc.q)
		}
	}
}
