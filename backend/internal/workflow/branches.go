package workflow

import (
	"nebulaiq/internal/domain"
	"net/url"
	"strings"
)

func aggregatorURL(raw string) bool {
	u, _ := url.Parse(raw)
	if u == nil {
		return true
	}
	for _, word := range []string{"tripadvisor", "yelp", "zomato", "ubereats", "grubhub", "doordash", "happycow", "reddit", "facebook", "instagram", "blog", "wheree", "restaurants-us", "restaurantguru"} {
		if strings.Contains(u.Hostname(), word) {
			return true
		}
	}
	return false
}
func distinctCandidates(values []domain.Candidate) []domain.Candidate {
	out := []domain.Candidate{}
	seen := map[string]bool{}
	for _, c := range values {
		key := strings.ToLower(nameSeparators.ReplaceAllString(c.Name, ""))
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, c)
	}
	return out
}
func branchURLAllowed(raw string, req domain.Requirements) bool {
	u, e := url.Parse(raw)
	if e != nil {
		return false
	}
	path := strings.ToLower(u.Path)
	citySlugs := map[string][]string{"new york": {"new-york", "nyc", "nomad"}, "toronto": {"toronto", "dundas"}, "paris": {"paris", "rivoli"}, "barcelona": {"barcelona"}, "delhi": {"delhi"}, "tokyo": {"tokyo"}, "buenos aires": {"buenos-aires"}, "são paulo": {"sao-paulo"}}
	// Reject explicit competing branch locations. This list is configurable code,
	// not an assertion that a matching host means every menu belongs to the city.
	competing := []string{"fort-lauderdale", "bethesda", "washington-dc", "miami", "los-angeles", "chicago", "boston", "atlanta", "vancouver", "montreal", "london", "dubai", "singapore", "new-york", "toronto", "paris", "barcelona", "delhi", "tokyo", "buenos-aires", "sao-paulo"}
	allowed := citySlugs[strings.ToLower(req.City)]
	for _, slug := range competing {
		if !strings.Contains(path, slug) {
			continue
		}
		own := false
		for _, v := range allowed {
			if slug == v {
				own = true
			}
		}
		if !own && len(allowed) > 0 {
			return false
		}
	}
	return true
}
func candidateHits(hits []domain.SearchHit, c domain.Candidate, req domain.Requirements) []domain.SearchHit {
	out := []domain.SearchHit{}
	for _, h := range hits {
		if branchURLAllowed(h.URL, req) {
			out = append(out, h)
		}
	}
	return out
}
func filterBranchURLs(values []string, name string, req domain.Requirements) []string {
	out := []string{}
	for _, v := range values {
		if branchURLAllowed(v, req) {
			out = append(out, v)
		}
	}
	return out
}

func localMenuTerms(country string) string {
	return map[string]string{"Spain": "carta precios", "France": "carte prix", "Japan": "メニュー 料金", "Argentina": "carta precios", "Brazil": "cardápio preços", "India": "menu prices"}[country]
}

func namedCandidate(c domain.Candidate, hits []domain.SearchHit) bool {
	key := strings.ToLower(nameSeparators.ReplaceAllString(c.Name, ""))
	if key == "" {
		return false
	}
	for _, h := range hits {
		if strings.Contains(strings.ToLower(nameSeparators.ReplaceAllString(h.Title+" "+h.Content, "")), key) {
			return true
		}
	}
	return false
}

func deliveryURL(raw string) bool {
	u, e := url.Parse(raw)
	if e != nil {
		return false
	}
	h := strings.ToLower(u.Hostname())
	for _, host := range []string{"postmates.com", "ubereats.com", "doordash.com", "grubhub.com", "deliveroo.com", "just-eat.com", "seamless.com", "swiggy.com", "zomato.com"} {
		if h == host || strings.HasSuffix(h, "."+host) {
			return true
		}
	}
	return false
}
