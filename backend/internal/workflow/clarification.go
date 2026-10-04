package workflow

import (
	"fmt"
	"nebulaiq/internal/domain"
	"strings"
)

// Ask for missing input before spending search/extraction quota. Optional
// preferences never prevent research; evidence gaps cannot be answered by users.
func clarification(r domain.Requirements, modelQuestion string) string {
	missing := []string{}
	if strings.TrimSpace(r.City) == "" {
		missing = append(missing, "city")
	}
	if strings.TrimSpace(r.Country) == "" {
		missing = append(missing, "country")
	}
	if strings.TrimSpace(r.Meal) == "" {
		missing = append(missing, "meal (breakfast, lunch or dinner)")
	}
	if len(missing) > 0 {
		if len(missing) == 1 && missing[0] == "city" && r.Country != "" {
			return fmt.Sprintf("Which city in %s will you be visiting? I have %s and your stated preferences noted.", r.Country, mealDescription(r.Meal))
		}
		return "To find relevant restaurants, I still need your " + strings.Join(missing, " and ") + ". Which should I use?"
	}
	if r.Budget != "" {
		budgetMissing := []string{}
		if r.Currency == "" {
			budgetMissing = append(budgetMissing, "currency")
		}
		if r.BudgetBasis == "" {
			budgetMissing = append(budgetMissing, "whether the amount is per dish, per person or per meal")
		}
		if len(budgetMissing) > 0 {
			return "For your budget of " + r.Budget + ", what " + strings.Join(budgetMissing, " and ") + " should I use?"
		}
	}
	q := strings.TrimSpace(modelQuestion)
	if !strings.Contains(q, "?") {
		return ""
	}
	return q
}

func mealDescription(meal string) string {
	if meal == "" {
		return "your destination"
	}
	return meal
}
