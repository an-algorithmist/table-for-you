package pipeline

import (
	"context"
	"strings"
	"testing"

	"table-for-you/backend/internal/domain"
	"table-for-you/backend/internal/testutil"
)

func TestMenuGapBeforeReview(t *testing.T) {
	req := domain.Requirements{City: "Paris", Country: "France", Meal: "breakfast"}
	candidates := []domain.Candidate{{Name: "A"}, {Name: "B"}}
	result := domain.Result{Alternatives: []domain.Restaurant{{Candidate: candidates[0], Dishes: []domain.Dish{{Price: "15", Currency: "EUR"}}}, {Candidate: candidates[1], Dishes: []domain.Dish{{Name: "Soup"}}}}}
	plan := nextResearchGap(req, result, candidates, map[string]bool{})
	if plan.Candidate != "B" || plan.Kind != "menu" || !strings.Contains(plan.Query, "France") {
		t.Fatalf("%#v", plan)
	}
}
func TestWrongRegionalMenu(t *testing.T) {
	hits := []domain.SearchHit{{URL: "https://angelinaparisusa.com/pages/menu", Title: "Angelina Paris USA"}, {URL: "https://angelina-paris.fr/menu", Title: "Angelina Paris"}}
	got := branchHits(hits, domain.Requirements{Country: "France"})
	if len(got) != 1 || got[0].URL != hits[1].URL {
		t.Fatalf("%#v", got)
	}
}

type failedExtractionSearch struct{ controlledSearch }

func (s *failedExtractionSearch) Extract(ctx context.Context, urls []string) (map[string]string, error) {
	s.fetches += len(urls)
	return map[string]string{}, nil
}
func TestFailedExtractionNotRepeatedWithinRun(t *testing.T) {
	store := testutil.Database(t)
	search := &failedExtractionSearch{}
	j := job{e: &Engine{Store: store, Search: search}, ctx: context.Background(), refresh: true}
	for i := 0; i < 3; i++ {
		if err := j.fetch([]string{"https://restaurant.example/menu"}, "menu"); err != nil {
			t.Fatal(err)
		}
	}
	if search.fetches != 1 || len(j.limitations) != 1 {
		t.Fatalf("fetches=%d limitations=%v", search.fetches, j.limitations)
	}
}
