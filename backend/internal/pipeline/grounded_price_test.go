package pipeline

import (
	"table-for-you/backend/internal/domain"
	"testing"
)

func TestGroundedSeparatePriceParagraph(t *testing.T) {
	for _, tc := range []struct{ row, price, basis string }{
		{"* Pricing: Signature item (~¥2,500–¥3,500 JPY per serving)", "2,500–3,500", "estimate"},
		{"* Listed Price: ~¥20,000 JPY per person.", "20,000", "estimate"},
		{"* Listed Price: ¥10,000+ JPY sharing portion.", "10,000+", "listed"},
	} {
		t.Run(tc.price, func(t *testing.T) {
			quote := "1. **Noharayaki**\n\n* Ingredients: Beef."
			block := "### 3. Kitchen\n" + quote + "\n\n" + tc.row + "\n\n1. **Other dish**\n* Listed Price: ¥999 JPY"
			g := &domain.Grounding{Text: block, Sources: []domain.GroundingSource{{Number: 1, URL: "https://menu.example"}}, Supports: []domain.GroundingSupport{{Text: quote, Sources: []int{1}}, {Text: tc.row, Sources: []int{1}}}}
			r := domain.GroundedRestaurant{Name: "Kitchen", Evidence: domain.GroundedClaim{Quote: block, Sources: []int{1}}, Dishes: []domain.GroundedDish{{Name: "Noharayaki", Evidence: domain.GroundedClaim{Quote: quote, Sources: []int{1}}}}}
			got := checkedGrounded(g, domain.GroundedExtraction{Restaurants: []domain.GroundedRestaurant{r}})
			if len(got) != 1 || len(got[0].Dishes) != 1 {
				t.Fatalf("missing dish: %+v", got)
			}
			d := got[0].Dishes[0]
			if d.Price != tc.price || d.Currency != "JPY" || d.PriceBasis != tc.basis {
				t.Fatalf("wrong price %+v", d)
			}
		})
	}
}

func TestGroundedSeparatePriceDoesNotCrossDishOrUseUnattributedPrice(t *testing.T) {
	for _, tail := range []string{"\n1. **Other dish**\n* Listed Price: EUR 99", "\n* Listed Price: EUR 99"} {
		quote := "1. **Soup**\n* Ingredients: Pumpkin."
		block := quote + tail
		g := &domain.Grounding{Text: block, Sources: []domain.GroundingSource{{Number: 1, URL: "https://menu.example"}}, Supports: []domain.GroundingSupport{{Text: quote, Sources: []int{1}}}}
		d := recoverGroundedPrice(g, domain.GroundedRestaurant{Evidence: domain.GroundedClaim{Quote: block}}, domain.GroundedDish{Name: "Soup", Evidence: domain.GroundedClaim{Quote: quote, Sources: []int{1}}})
		if d.Price != "" {
			t.Fatalf("borrowed price: %+v", d)
		}
	}
}
