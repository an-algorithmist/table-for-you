package evidence

import (
	"fmt"
	"strings"

	"table-for-you/backend/internal/domain"
)

// Mandatory lists the dietary requirements every recommended dish must satisfy.
func Mandatory(req domain.Requirements) []string {
	out := []string{}
	if req.Diet != "" && req.Diet != "no restriction" {
		out = append(out, req.Diet)
	}
	for _, x := range req.Excluded {
		if strings.TrimSpace(x) != "" {
			out = append(out, strings.ToLower(x))
		}
	}
	if req.VenueOnly {
		out = append(out, "vegetarian venue")
	}
	return out
}

// Summary describes the current location, meal and dietary context for research traces.
func Summary(req domain.Requirements) string {
	return fmt.Sprintf("%s in %s, %s; %s; exclusions: %s", req.Meal, req.City, req.Country, req.Diet, strings.Join(req.Excluded, ", "))
}
