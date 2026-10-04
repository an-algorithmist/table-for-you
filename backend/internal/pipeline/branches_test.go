package pipeline

import (
	"testing"

	"table-for-you/backend/internal/domain"
)

func TestRejectOtherChainBranches(t *testing.T) {
	req := domain.Requirements{City: "New York", Country: "United States"}
	for _, raw := range []string{"https://example.com/menu/queen-fort-lauderdale-all-day", "https://example.com/bethesda-location", "https://example.com/washington-dc-location"} {
		if branchURLAllowed(raw, req) {
			t.Fatal(raw)
		}
	}
	if !branchURLAllowed("https://example.com/new-york-city-location", req) {
		t.Fatal("correct branch rejected")
	}
}
func TestDistinctCandidates(t *testing.T) {
	v := distinctCandidates([]domain.Candidate{{Name: "PLANTA Queen NoMad"}, {Name: "PLANTA Queen NoMad"}, {Name: "Another"}})
	if len(v) != 2 {
		t.Fatal(v)
	}
}

func TestCandidateMustUseNamedRestaurant(t *testing.T) {
	hits := []domain.SearchHit{{Content: "Sam’s Falafel Stand in the Financial District, New York"}}
	if namedCandidate(domain.Candidate{Name: "Financial District Falafel Stand"}, hits) {
		t.Fatal("generic inferred name accepted")
	}
	if !namedCandidate(domain.Candidate{Name: "Sam's Falafel Stand"}, hits) {
		t.Fatal("real name rejected")
	}
}
