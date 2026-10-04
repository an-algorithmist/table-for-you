package pipeline

import (
	"strings"

	"table-for-you/backend/internal/domain"
)

func priorResearch(chat domain.Conversation, runID string, req domain.Requirements) *domain.Result {
	for _, r := range chat.Runs {
		if r.ID != runID && r.Result != nil && r.Result.Clarification == "" && !r.Result.ResearchedAt.IsZero() && marshal(r.Result.Requirements) == marshal(req) {
			return r.Result
		}
	}
	return nil
}
func shortText(s string) string {
	if len([]rune(s)) > 500 {
		return string([]rune(s)[:500])
	}
	return s
}

// Pass structured findings, not entire scraped pages, to conversation interpretation.
func researchContext(r *domain.Result) map[string]any {
	docs := map[string]domain.Document{}
	for _, d := range r.Sources {
		docs[d.ID] = d
	}
	restaurants := []map[string]any{}
	values := append(append([]domain.Restaurant{}, r.Confirmed...), r.Alternatives...)
	for i, v := range values {
		if i >= 6 {
			break
		}
		dishes := []map[string]any{}
		for k, d := range v.Dishes {
			if k >= 8 {
				break
			}
			src := docs[d.PriceEvidence.SourceID]
			menu := docs[d.Evidence.SourceID]
			gaps := []map[string]string{}
			for _, c := range d.Constraints {
				if c.Status != "supported" && c.Name != "menu recency" {
					gaps = append(gaps, map[string]string{"name": c.Name, "status": c.Status, "reason": shortText(c.Reason)})
				}
			}
			dishes = append(dishes, map[string]any{"name": d.Name, "description": shortText(d.Description), "price": d.Price, "currency": d.Currency, "currency_basis": d.CurrencyBasis, "status": d.Status, "remaining_checks": gaps, "price_source": map[string]any{"url": src.URL, "snippet": src.Snippet, "method": src.Method, "source_type": src.Kind}, "price_quote": shortText(d.PriceEvidence.Quote), "menu_quote": shortText(d.Evidence.Quote), "menu_source": map[string]any{"url": menu.URL, "snippet": menu.Snippet}})
		}
		reviews := []map[string]string{}
		for k, v := range v.Reviews {
			if k >= 4 {
				break
			}
			reviews = append(reviews, map[string]string{"sentiment": v.Sentiment, "summary": shortText(v.Summary)})
		}
		restaurants = append(restaurants, map[string]any{"name": v.Name, "address": v.Address, "identity_verified": v.IdentityVerified, "menu_url": v.MenuURL, "dishes": dishes, "reviews": reviews})
	}
	var groundedSources []domain.GroundingSource
	groundedText := ""
	if r.Grounding != nil {
		groundedSources = r.Grounding.Sources
		groundedText = shortText(r.Grounding.Text)
		if len(r.Grounding.Text) > 500 {
			groundedText = string([]rune(r.Grounding.Text)[:min(len([]rune(r.Grounding.Text)), 12000)])
		}
	}
	return map[string]any{"grounded_sources": groundedSources, "grounded_answer": groundedText, "price_policy": "Accept linked source-listed prices from official or third-party menus, including older menus, as planning guidance. Do not show age warnings or claim independently verified current prices.", "requirements": r.Requirements, "researched_at": r.ResearchedAt, "restaurants": restaurants, "excluded_restaurants": len(r.Excluded), "limitations": strings.Join(r.Limitations[:min(len(r.Limitations), 2)], " ")}
}
