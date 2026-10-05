package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"table-for-you/backend/internal/domain"
	"testing"
)

func TestBM25RetainsLatePriceRows(t *testing.T) {
	text := strings.Repeat("Welcome to our restaurant. Contact and directions.\n", 900) + "\nDinner menu EUR\nPumpkin ravioli | 18.50\nHomemade ravioli with pumpkin and sage.\n"
	selected := RankedMenuText(text, "Pumpkin ravioli dinner EUR")
	if !strings.Contains(selected, "Pumpkin ravioli | 18.50") || !strings.Contains(selected, "pumpkin and sage") {
		t.Fatal("late price row or adjacent description lost")
	}
	if len(selected) > 18000 {
		t.Fatal("passage budget exceeded")
	}
}
func TestGroundedFormattingRejectsInventedPriceAndSources(t *testing.T) {
	g := &domain.Grounding{Text: "Kitchen: Soup EUR 12 with pumpkin", Sources: []domain.GroundingSource{{Number: 1, URL: "https://kitchen.example/menu"}}}
	quote := domain.GroundedClaim{Quote: g.Text, Sources: []int{1}}
	raw := domain.GroundedExtraction{Restaurants: []domain.GroundedRestaurant{{Name: "Kitchen", Evidence: quote, Dishes: []domain.GroundedDish{{Name: "Soup", Description: "invented truffle", Price: "99", Currency: "EUR", Evidence: quote}, {Name: "Soup", Price: "12", Currency: "EUR", Evidence: domain.GroundedClaim{Quote: g.Text, Sources: []int{99}}}}}}}
	got := checkedGrounded(g, raw)
	if len(got) != 1 || len(got[0].Dishes) != 1 || got[0].Dishes[0].Price != "" || got[0].Dishes[0].Description != "Kitchen: Soup EUR 12 with pumpkin" {
		t.Fatalf("unsafe formatting accepted: %+v", got)
	}
	raw.Restaurants[0].Evidence.Quote = "Kitchen invented"
	if len(checkedGrounded(g, raw)) != 0 {
		t.Fatal("nonliteral claim accepted")
	}
}
func TestNameNormalizationAndOlderMenuRanking(t *testing.T) {
	hits := []domain.SearchHit{{Title: "Cafe Example", URL: "https://cafeexample.example/menu", Content: "Cafe Example Barcelona menu EUR 12"}}
	if len(relevantHits(hits, "Café Example")) != 1 {
		t.Fatal("accent alias rejected")
	}
	ranked := prioritizeMenus([]domain.SearchHit{{URL: "https://cafe.example/2023/menu", Content: "Cafe Example Barcelona EUR 12"}, {URL: "https://blog.example/guide", Content: "Cafe Example Barcelona"}}, domain.Candidate{Name: "Cafe Example", OfficialURL: "https://cafe.example"}, domain.Requirements{City: "Barcelona", Country: "Spain"})
	if !strings.Contains(ranked[0].URL, "2023/menu") {
		t.Fatal("supported older official menu deprioritized")
	}
}

func TestRecordedGroundedLayout(t *testing.T) {
	for _, city := range []string{"barcelona", "tokyo"} {
		t.Run(city, func(t *testing.T) {
			b, err := os.ReadFile(filepath.Join("testdata", city+"-grounded.json"))
			if err != nil {
				t.Fatal(err)
			}
			var g domain.Grounding
			if err = json.Unmarshal(b, &g); err != nil {
				t.Fatal(err)
			}
			g.Recommendations = checkedGrounded(&g, groundedLayout(&g))
			dishes, prices := 0, 0
			for _, r := range g.Recommendations {
				for _, d := range r.Dishes {
					dishes++
					if d.Price != "" && d.Currency != "" {
						prices++
					}
				}
			}
			t.Logf("%s: restaurants=%d dishes=%d prices=%d", city, len(g.Recommendations), dishes, prices)
			if len(g.Recommendations) < 2 || prices < 3 {
				t.Fatalf("recorded layout lost attributed prices")
			}
			if output := os.Getenv("REPLAY_OUTPUT_DIR"); output != "" {
				b, _ = json.MarshalIndent(g, "", "  ")
				if err = os.WriteFile(filepath.Join(output, city+"-replay.json"), b, 0600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestGroundedPriceRejectsUnrelatedAmounts(t *testing.T) {
	g := &domain.Grounding{Text: "Kitchen Soup EUR 12 for 199 diners", Sources: []domain.GroundingSource{{Number: 1, URL: "https://kitchen.example/menu"}}}
	c := domain.GroundedClaim{Quote: g.Text, Sources: []int{1}}
	got := checkedGrounded(g, domain.GroundedExtraction{Restaurants: []domain.GroundedRestaurant{{Name: "Kitchen", Evidence: c, Dishes: []domain.GroundedDish{{Name: "Soup", Price: "99", Currency: "EUR", Evidence: c}}}}})
	if got[0].Dishes[0].Price != "" || groundedListedAmount("199", g.Text) || groundedListedAmount("16", "Curry included in set / under €16") {
		t.Fatal("price matched substring of unrelated number")
	}
}
func TestGroundedDeliveryAmountIsNotDineInPrice(t *testing.T) {
	g := &domain.Grounding{Text: "Kitchen Soup EUR 12", Sources: []domain.GroundingSource{{Number: 1, URL: "https://ubereats.com/menu", Title: "ubereats.com"}}}
	c := domain.GroundedClaim{Quote: g.Text, Sources: []int{1}}
	got := checkedGrounded(g, domain.GroundedExtraction{Restaurants: []domain.GroundedRestaurant{{Name: "Kitchen", Evidence: c, Dishes: []domain.GroundedDish{{Name: "Soup", Price: "12", Currency: "EUR", Evidence: c}}}}})
	if got[0].Dishes[0].Price != "" {
		t.Fatal("delivery amount displayed as dine-in price")
	}
}
func TestGroundedBudgetExhaustionRetainsProse(t *testing.T) {
	j := job{e: &Engine{}, use: domain.Usage{ModelCalls: 9}}
	g := &domain.Grounding{Text: "Original paid answer"}
	j.formatGrounded(g)
	if g.Text != "Original paid answer" || g.FormattingError == "" || j.use.ModelCalls != 9 {
		t.Fatal("formatting budget did not retain original answer")
	}
}

func TestPriceSnippetIsNotLostBehindEarlierSnippet(t *testing.T) {
	j := job{docCandidates: map[string]map[string]bool{}}
	j.snippet(domain.SearchHit{URL: "https://kitchen.example/menu", Content: "Kitchen menu soup"}, "menu")
	j.snippet(domain.SearchHit{URL: "https://kitchen.example/menu", Content: "Kitchen Soup EUR 12"}, "menu")
	j.snippet(domain.SearchHit{URL: "https://kitchen.example/menu", Content: "Kitchen Soup EUR 12"}, "menu")
	if len(j.docs) != 2 || !strings.Contains(j.docs[1].Text, "EUR 12") {
		t.Fatal("new price snippet discarded or exact duplicate retained")
	}
}
