package evidence

import (
	"os"
	"strings"
	"table-for-you/backend/internal/domain"
	"testing"
	"time"
)

func TestRecordedChickenQuoteRecovery(t *testing.T) {
	text, err := os.ReadFile("testdata/tokyo-chicken-menu.txt")
	if err != nil {
		t.Fatal(err)
	}
	doc := domain.Document{ID: "menu", Kind: "menu", Text: string(text)}
	d := domain.Dish{Name: "Chicken Roast", Price: "1500", Evidence: domain.Citation{Quote: "Chicken Roast 1500 A Nirvanam special recipe,Chicken legs(2pc) cooked in spicy gravy"}}
	if !RecoverMenuDish(&d, []domain.Document{doc}) || !Supported(d.Evidence, map[string]domain.Document{"menu": doc}) || d.Price != "" {
		t.Fatalf("unsafe recovery: %+v", d)
	}
	r := domain.Restaurant{Candidate: domain.Candidate{Name: "Nirvanam"}, Dishes: []domain.Dish{d}}
	result := Validate(domain.Requirements{City: "Tokyo", Country: "Japan", Meal: "dinner", Diet: "non-vegetarian", FoodPreference: "chicken"}, domain.Extraction{Restaurants: []domain.Restaurant{r}}, []domain.Document{doc}, time.Now())
	if len(result.Alternatives) != 1 || len(result.Alternatives[0].Dishes) != 1 || !strings.Contains(result.Alternatives[0].Dishes[0].Description, "Chicken legs") {
		t.Fatal("literal chicken dish disappeared")
	}
}
func TestMenuRowCannotBorrowAdjacentDish(t *testing.T) {
	d := domain.Dish{Name: "Chicken Roast"}
	if RecoverMenuDish(&d, []domain.Document{{Kind: "menu", Text: "#### Fish Curry\n\nFish in curry sauce"}}) {
		t.Fatal("borrowed another dish")
	}
}
