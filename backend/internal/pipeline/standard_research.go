package pipeline

import (
	"fmt"
	"time"

	"table-for-you/backend/internal/domain"
	"table-for-you/backend/internal/evidence"
)

// standardResearch runs the fixed discovery, retrieval, extraction and gap-repair stages.
func (j *job) standardResearch(req domain.Requirements, text, query string) (*domain.Result, error) {
	candidates, err := j.discover(req, text, query)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		resultPtr := evidence.Validate(req, domain.Extraction{}, j.docs, time.Now())
		result := &resultPtr
		result.Limitations = append(result.Limitations, "No candidate could be shortlisted from accessible search results.")
		return result, nil
	}
	_ = j.event("discovery.completed", fmt.Sprintf("Shortlisted %d candidate branches; checking menus and review evidence.", len(candidates)))
	j.collectEvidence(req, candidates)
	j.activeCandidate = ""
	raw, err := j.extract(req, candidates)
	value := j.validate(req, raw)
	result := &value
	if err != nil {
		return result, err
	}

	attempted := map[string]bool{}
	for round := 0; round < 2 && j.use.Searches < 14 && j.use.ModelCalls < j.modelCallLimit() && j.ctx.Err() == nil; round++ {
		plan := nextResearchGap(req, *result, candidates, attempted)
		if plan.Query == "" {
			break
		}
		attempted[plan.Candidate] = true
		j.activeCandidate = plan.Candidate
		_ = j.event("research.followup", plan.Reason)
		extra, er := j.search(plan.Query, 4)
		if er != nil {
			j.limitations = append(j.limitations, "Targeted follow-up failed for "+plan.Candidate)
			continue
		}
		extra = branchHits(relevantHits(extra, plan.Candidate), req)
		for _, c := range candidates {
			if c.Name == plan.Candidate {
				extra = candidateHits(extra, c, req)
			}
		}
		before := len(j.docs)
		urls := []string{}
		for _, h := range extra {
			j.snippet(h, plan.Kind)
			if len(urls) < 3 {
				urls = append(urls, h.URL)
			}
		}
		_ = j.fetch(urls, plan.Kind)
		if len(j.docs) <= before {
			continue
		}
		for _, c := range candidates {
			if c.Name != plan.Candidate {
				continue
			}
			updated, extractErr := j.extractCandidate(req, c)
			if extractErr != nil {
				j.limitations = append(j.limitations, "Follow-up extraction did not finish for "+c.Name)
				break
			}
			kept := []domain.Restaurant{}
			for _, r := range raw.Restaurants {
				if r.Name != c.Name {
					kept = append(kept, r)
				}
			}
			raw.Restaurants = append(kept, updated.Restaurants...)
			value = j.validate(req, raw)
			result = &value
		}
	}

	j.activeCandidate = ""
	result.Limitations = append(result.Limitations, j.limitations...)
	_ = j.event("validation.completed", fmt.Sprintf("Validated exact source passages: %d confirmed restaurants; %d requiring confirmation.", len(result.Confirmed), len(result.Alternatives)))
	return result, nil
}
