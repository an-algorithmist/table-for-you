package pipeline

import (
	"fmt"
	"strings"

	"table-for-you/backend/internal/domain"
)

// collectEvidence keeps menu, price and review retrieval scoped to one branch at a time.
// The branch tags are later used to reject cross-restaurant citation borrowing.
func (j *job) collectEvidence(req domain.Requirements, candidates []domain.Candidate) {
	for i := range candidates {
		c := &candidates[i]
		j.activeCandidate = c.Name
		menuHits, err := j.search(fmt.Sprintf(`"%s" %s %s official menu %s prices`, c.Name, req.City, req.Country, req.Meal)+" "+localMenuTerms(req.Country), 5)
		if err != nil {
			j.limitations = append(j.limitations, "Menu search failed for "+c.Name)
			continue
		}
		menuHits = candidateHits(branchHits(relevantHits(menuHits, c.Name), req), *c, req)
		for _, h := range menuHits {
			j.snippet(h, "menu")
		}
		menuHits = prioritizeMenus(menuHits, *c, req)
		if len(menuHits) > 0 {
			c.MenuURL = menuHits[0].URL
		}
		if c.OfficialURL == "" {
			for _, h := range menuHits {
				if !aggregatorURL(h.URL) {
					c.OfficialURL = h.URL
					break
				}
			}
		}
		urls := []string{}
		if c.MenuURL != "" {
			urls = append(urls, c.MenuURL)
		}
		if c.OfficialURL != "" {
			urls = append(urls, c.OfficialURL)
		}
		for _, h := range menuHits {
			if len(urls) < 5 {
				urls = append(urls, h.URL)
			}
		}
		beforeMenu := len(j.docs)
		if err = j.fetch(urls, "menu"); err != nil {
			j.limitations = append(j.limitations, "Some menu text could not be extracted for "+c.Name)
		}
		if err = j.followMenuLinks(beforeMenu); err != nil {
			j.limitations = append(j.limitations, "A linked menu could not be extracted for "+c.Name)
		}
		// Menu prices take priority over another generic review page.
		if !j.hasMenuPrices(c.Name) && j.use.Searches < 14 {
			_ = j.event("prices.searching", "Looking for item prices and per-person cost at "+c.Name)
			priceHits, priceErr := j.search(fmt.Sprintf("\"%s\" %s official menu prices %s cost per person", c.Name, req.City, req.Currency), 4)
			if priceErr == nil {
				priceHits = prioritizeMenus(candidateHits(branchHits(relevantHits(priceHits, c.Name), req), *c, req), *c, req)
				priceURLs := []string{}
				for _, h := range priceHits {
					j.snippet(h, "menu")
					if len(priceURLs) < 3 {
						priceURLs = append(priceURLs, h.URL)
					}
				}
				_ = j.fetch(priceURLs, "menu")
			}
		}
		reviewHits, err := j.search(fmt.Sprintf(`"%s" %s %s customer reviews positive negative complaints %s %s`, c.Name, req.City, req.Country, req.Diet, strings.Join(req.Excluded, " ")), 3)
		if err != nil {
			j.limitations = append(j.limitations, "Review search failed for "+c.Name)
			continue
		}
		reviewHits = relevantHits(reviewHits, c.Name)
		for _, h := range reviewHits {
			j.snippet(h, "review")
		}
		reviewURLs := []string{}
		for _, h := range reviewHits {
			if len(reviewURLs) < 2 {
				reviewURLs = append(reviewURLs, h.URL)
			}
		}
		if err = j.fetch(reviewURLs, "review"); err != nil {
			j.limitations = append(j.limitations, "Review evidence includes search snippets because some pages could not be extracted.")
		}
	}
	j.activeCandidate = ""
}
