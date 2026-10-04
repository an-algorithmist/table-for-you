package pipeline

import (
	"testing"

	"table-for-you/backend/internal/domain"
)

func TestFollowupPreservesExclusions(t *testing.T) {
	old := domain.Requirements{Diet: "vegetarian", Excluded: []string{"onion", "garlic"}}
	next := PreserveExclusions(old, domain.Requirements{Budget: "12"}, "show cheaper options")
	if len(next.Excluded) != 2 || next.Diet != "vegetarian" {
		t.Fatal("follow-up relaxed restrictions")
	}
	next = PreserveExclusions(old, domain.Requirements{}, "allow garlic")
	if len(next.Excluded) != 1 || next.Excluded[0] != "onion" {
		t.Fatal("explicit removal did not preserve the other restriction")
	}
}
