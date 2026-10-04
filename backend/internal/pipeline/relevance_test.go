package pipeline

import (
	"testing"

	"table-for-you/backend/internal/domain"
)

func TestRestaurantRetrievalRejectsUnrelatedResults(t *testing.T) {
	hits := []domain.SearchHit{
		{Title: "Women's coats and jackets", URL: "https://store.example/coats", Content: "Green jackets on sale"},
		{Title: "The Green Spot reviews", URL: "https://reviews.example/the-green-spot"},
		{Title: "Flamenco dinner Barcelona", URL: "https://reviews.example/flamenco"},
	}
	got := relevantHits(hits, "The Green Spot")
	if len(got) != 1 || got[0].URL != hits[1].URL {
		t.Fatalf("unrelated sources accepted: %+v", got)
	}
}
