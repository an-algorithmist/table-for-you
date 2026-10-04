package domain

// GroundingSource is a numbered provider attribution link, not an independently extracted document.
type GroundingSource struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
	Title  string `json:"title"`
}

// GroundingSupport maps provider text spans to source numbers.
type GroundingSupport struct {
	Text    string `json:"text"`
	Sources []int  `json:"sources"`
}

// URLRetrieval reports URL-context status returned by Gemini.
type URLRetrieval struct {
	URL    string `json:"url"`
	Status string `json:"status"`
}

// Grounding retains provider prose and attribution independently of standard evidence validation.
type Grounding struct {
	Text              string             `json:"text"`
	Model             string             `json:"model"`
	Sources           []GroundingSource  `json:"sources"`
	Supports          []GroundingSupport `json:"supports"`
	Queries           []string           `json:"queries"`
	URLs              []URLRetrieval     `json:"urls"`
	SearchSuggestions string             `json:"search_suggestions,omitempty"`
}
