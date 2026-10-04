package pipeline

import (
	"encoding/json"
	"time"

	"table-for-you/backend/internal/domain"
	"table-for-you/backend/internal/evidence"
)

func (j *job) validate(req domain.Requirements, raw domain.Extraction) domain.Result {
	result := evidence.Validate(req, domain.Extraction{}, j.docs, time.Now())
	result.Limitations = result.Limitations[:1]
	// Clone before replacing cached source aliases with immutable concrete citation IDs.
	var copy domain.Extraction
	_ = json.Unmarshal([]byte(marshal(raw)), &copy)
	mapping := map[string]string{}
	for _, d := range j.sortedDocs() {
		mapping[sourceAlias(d)] = d.ID
	}
	fix := func(c *domain.Citation) { c.SourceID = mapping[c.SourceID] }
	for i := range copy.Restaurants {
		r := &copy.Restaurants[i]
		// A model can quote the homepage accurately but label it with the /menu
		// source ID. Repair only exact quotations in this branch's retrieved docs.
		branchDocs := j.candidateDocs(r.Name)
		branchByID := map[string]domain.Document{}
		for _, d := range branchDocs {
			branchByID[d.ID] = d
		}
		cite := func(c *domain.Citation) {
			fix(c)
			if evidence.Supported(*c, branchByID) {
				return
			}
			for _, d := range branchDocs {
				candidate := domain.Citation{SourceID: d.ID, Quote: c.Quote}
				if evidence.Supported(candidate, branchByID) {
					*c = candidate
					return
				}
			}
			*c = domain.Citation{}
		}
		cite(&r.Identity)
		cite(&r.VenueEvidence)
		cite(&r.PriceRange.Evidence)
		branchURLs := map[string]bool{}
		for _, d := range branchDocs {
			branchURLs[d.URL] = true
		}
		if !branchURLs[r.OfficialURL] {
			r.OfficialURL = ""
		}
		if !branchURLs[r.MenuURL] {
			r.MenuURL = ""
		}
		for k := range r.Dishes {
			d := &r.Dishes[k]
			cite(&d.Evidence)
			cite(&d.PriceEvidence)
			for x := range d.Constraints {
				cite(&d.Constraints[x].Evidence)
			}
		}
		for k := range r.Reviews {
			cite(&r.Reviews[k].Evidence)
		}
		v := evidence.Validate(req, domain.Extraction{Restaurants: []domain.Restaurant{*r}}, branchDocs, time.Now())
		result.Confirmed = append(result.Confirmed, v.Confirmed...)
		result.Alternatives = append(result.Alternatives, v.Alternatives...)
		result.Excluded = append(result.Excluded, v.Excluded...)
	}
	if len(result.Confirmed) == 0 {
		result.Limitations = append(result.Limitations, "No fully confirmed match; inspect the remaining source and availability gaps.")
	}
	return result
}
