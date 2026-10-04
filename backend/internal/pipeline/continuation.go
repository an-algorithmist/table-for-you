package pipeline

import (
	"strings"
	"time"

	"table-for-you/backend/internal/domain"
)

func sameResearchScope(a, b domain.Requirements) bool {
	a.Budget = ""
	a.Currency = ""
	a.BudgetBasis = ""
	b.Budget = ""
	b.Currency = ""
	b.BudgetBasis = ""
	return marshal(a) == marshal(b)
}
func (j *job) previousShortlist(req domain.Requirements) ([]domain.Candidate, error) {
	if !sameResearchScope(j.run.Requirements, req) {
		return nil, nil
	}
	chat, err := j.e.Store.Conversation(j.ctx, j.owner, j.run.ConversationID)
	if err != nil {
		return nil, err
	}
	for _, run := range chat.Runs {
		if run.ID == j.run.ID || run.Result == nil || !sameResearchScope(run.Result.Requirements, req) {
			continue
		}
		r := run.Result
		restaurants := append(append(append([]domain.Restaurant{}, r.Confirmed...), r.Alternatives...), r.Excluded...)
		if len(restaurants) == 0 {
			continue
		}
		candidates := []domain.Candidate{}
		seen := map[string]bool{}
		byCitation := map[string][]string{}
		for _, restaurant := range restaurants {
			if seen[strings.ToLower(restaurant.Name)] {
				continue
			}
			seen[strings.ToLower(restaurant.Name)] = true
			candidate := restaurant.Candidate
			candidate.Identity = domain.Citation{}
			candidates = append(candidates, candidate)
			tag := func(id string) {
				if id != "" {
					byCitation[id] = append(byCitation[id], restaurant.Name)
				}
			}
			tag(restaurant.Identity.SourceID)
			tag(restaurant.VenueEvidence.SourceID)
			for _, dish := range restaurant.Dishes {
				tag(dish.Evidence.SourceID)
				tag(dish.PriceEvidence.SourceID)
				tag(dish.CurrencyEvidence.SourceID)
				for _, constraint := range dish.Constraints {
					tag(constraint.Evidence.SourceID)
				}
			}
			for _, review := range restaurant.Reviews {
				tag(review.Evidence.SourceID)
			}
			if len(candidates) == 3 {
				break
			}
		}
		if !j.refresh {
			for _, source := range r.Sources {
				if !source.ExpiresAt.After(time.Now()) {
					continue
				}
				j.docs = append(j.docs, source)
				j.tag(source.ID)
				for _, name := range byCitation[source.ID] {
					j.activeCandidate = name
					j.tag(source.ID)
				}
				j.activeCandidate = ""
			}
		}
		j.use.CacheHits++
		_ = j.event("shortlist.reused", "Reused the previous restaurant branches; applying updated requirements and checking evidence freshness.")
		return candidates, nil
	}
	return nil, nil
}
