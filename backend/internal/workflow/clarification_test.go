package workflow

import (
	"nebulaiq/internal/domain"
	"strings"
	"testing"
)

func TestClarificationOnlyAsksMissingFields(t *testing.T) {
	r := domain.Requirements{Country: "Argentina", Meal: "breakfast"}
	q := clarification(r, "Please specify city, country and meal")
	if !strings.Contains(q, "Which city in Argentina") || strings.Contains(q, "specify city, country") {
		t.Fatal(q)
	}
	r.City = "Buenos Aires"
	if q = clarification(r, ""); q != "" {
		t.Fatalf("optional diet/budget blocked research: %s", q)
	}
	r.Budget = "20"
	r.Currency = "EUR"
	q = clarification(r, "")
	if strings.Contains(q, "currency") || !strings.Contains(q, "per dish") {
		t.Fatal(q)
	}
}
