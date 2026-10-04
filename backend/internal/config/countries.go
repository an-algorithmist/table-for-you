package config

import (
	"encoding/json"
	"fmt"
	"strings"
	countrydata "table-for-you/backend/config"
)

// CountrySettings contains search vocabulary and display-currency defaults.
// Currency is an assumption used only after branch evidence establishes location.
type CountrySettings struct {
	Currency  string `json:"currency"`
	MenuTerms string `json:"menu_terms"`
}

// Countries is immutable once loaded. Unknown countries keep generic search terms;
// an absent currency never authorizes inventing one in a menu citation.
type Countries struct {
	Countries      map[string]CountrySettings `json:"countries"`
	BranchSlugs    map[string][]string        `json:"branch_slugs"`
	CompetingSlugs []string                   `json:"competing_slugs"`
}

// LoadCountries parses the embedded YAML 1.2 resource without another dependency.
func LoadCountries() (Countries, error) {
	var data Countries
	if err := json.Unmarshal(countrydata.Read(), &data); err != nil {
		return data, fmt.Errorf("load country configuration: %w", err)
	}
	return data, nil
}

// Country returns a configured country or a safe generic fallback.
func Country(name string) CountrySettings {
	data, err := LoadCountries()
	if err != nil {
		return CountrySettings{}
	}
	return data.Countries[strings.ToLower(strings.TrimSpace(name))]
}
