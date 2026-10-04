package pipeline

import (
	"fmt"
	"strings"

	"table-for-you/backend/internal/domain"
	"table-for-you/backend/internal/providers"
)

// discover reuses a compatible shortlist before paying for new discovery.
func (j *job) discover(req domain.Requirements, text, query string) ([]domain.Candidate, error) {
	candidates, err := j.previousShortlist(req)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		discoveryQuery := fmt.Sprintf("%s %s %s %s restaurants official menu address", req.City, req.Country, req.Meal, req.Diet)
		if strings.TrimSpace(query) != "" {
			discoveryQuery = query
		}
		hits, err := j.search(discoveryQuery, 5)
		if err != nil {
			return nil, err
		}
		for _, h := range hits {
			j.snippet(h, "discovery")
		}
		var discovery domain.Discovery
		if err = j.generatePrompt("discovery", marshal(map[string]any{"requirements": req, "new_message": text, "sources": j.aliases()}), domain.Discovery{}, &discovery); err != nil {
			return nil, err
		}
		if len(discovery.Candidates) > 3 {
			discovery.Candidates = discovery.Candidates[:3]
		}
		candidates = []domain.Candidate{}
		for _, c := range discovery.Candidates {
			if c.Name == "" || !namedCandidate(c, hits) {
				continue
			}
			if !allowedURL(c.OfficialURL, hits) {
				c.OfficialURL = ""
			}
			if !allowedURL(c.MenuURL, hits) {
				c.MenuURL = ""
			}
			candidates = append(candidates, c)
		}
	}
	candidates = distinctCandidates(candidates)
	return candidates, nil
}

func allowedURL(u string, hits []domain.SearchHit) bool {
	if !providers.SafeURL(u) {
		return false
	}
	for _, h := range hits {
		if u == h.URL {
			return true
		}
	}
	return false
}
