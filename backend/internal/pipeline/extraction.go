package pipeline

import (
	"fmt"
	"strings"
	"time"

	"table-for-you/backend/internal/domain"
	"table-for-you/backend/internal/llm"
	"table-for-you/backend/internal/storage/postgres"
)

const extractionVersion = "menu-review-v15-literal-menu-rows"

func (j *job) extract(req domain.Requirements, candidates []domain.Candidate) (domain.Extraction, error) {
	aggregate := domain.Extraction{Restaurants: []domain.Restaurant{}}
	for _, candidate := range candidates {
		raw, err := j.extractCandidate(req, candidate)
		aggregate.Restaurants = append(aggregate.Restaurants, raw.Restaurants...)
		if err != nil {
			return aggregate, err
		}
	}
	return aggregate, nil
}

func (j *job) extractCandidate(req domain.Requirements, candidate domain.Candidate) (domain.Extraction, error) {
	// Budget changes can reuse extraction; dietary/meal changes must reassess against source text.
	keyReq := req
	keyReq.Budget = ""
	keyReq.Currency = ""
	keyReq.BudgetBasis = ""
	bits := []string{extractionVersion, j.e.Model.Name(), marshal(keyReq), candidate.Name, candidate.Address, candidate.OfficialURL, candidate.MenuURL}
	documents := j.candidateDocs(candidate.Name)
	for _, d := range documents {
		bits = append(bits, d.URL, d.Hash, fmt.Sprint(d.Snippet))
	}
	key := postgres.Hash(strings.Join(bits, "|"))
	if v, ok, e := j.e.Store.Extraction(j.ctx, key); e != nil {
		return v, e
	} else if ok {
		j.use.CacheHits++
		_ = j.event("cache.hit", "Reused compatible menu/review extraction; applying current budget separately.")
		return v, nil
	}
	var raw domain.Extraction
	start := time.Now()
	defer j.duration("model_extraction", start)
	instruction, err := llm.Prompt("menu_extract")
	if err != nil {
		return raw, err
	}
	_ = j.event("menu.extracting", "Extracting and translating menu/review evidence for "+candidate.Name)
	sourceInput := []map[string]any{}
	for _, d := range documents {
		selected := RelevantText(d.Text, d.Kind, candidate.Name+" "+req.Meal+" "+req.Currency+" "+req.FoodPreference+" "+j.passageQuery)
		if d.Kind == "menu" && menuPrice.MatchString(d.Text) && !menuPrice.MatchString(selected) {
			_ = j.event("price.selection_gap", "Price-bearing rows omitted during passage selection: "+d.URL)
		}
		sourceInput = append(sourceInput, map[string]any{"source_id": sourceAlias(d), "url": d.URL, "kind": d.Kind, "snippet": d.Snippet, "method": d.Method, "text": selected, "title": d.Title})
	}
	e := j.generate(instruction, marshal(map[string]any{"requirements": keyReq, "candidates": []domain.Candidate{candidate}, "sources": sourceInput}), domain.Extraction{}, &raw)
	if e != nil {
		return raw, e
	}
	if len(raw.Restaurants) > 1 {
		raw.Restaurants = raw.Restaurants[:1]
	}
	for i := range raw.Restaurants {
		raw.Restaurants[i].Name = candidate.Name
		if candidate.Address != "" {
			raw.Restaurants[i].Address = candidate.Address
		}
		if raw.Restaurants[i].OfficialURL == "" {
			raw.Restaurants[i].OfficialURL = candidate.OfficialURL
		}
		if raw.Restaurants[i].MenuURL == "" {
			raw.Restaurants[i].MenuURL = candidate.MenuURL
		}
		if len(raw.Restaurants[i].Dishes) > 4 {
			raw.Restaurants[i].Dishes = raw.Restaurants[i].Dishes[:4]
		}
		if len(raw.Restaurants[i].Reviews) > 8 {
			raw.Restaurants[i].Reviews = raw.Restaurants[i].Reviews[:8]
		}
	}
	return raw, j.e.Store.PutExtraction(j.ctx, key, raw)
}
