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
	Recommendations   []GroundedRestaurant `json:"recommendations,omitempty"`
	FormattingError   string               `json:"formatting_error,omitempty"`
	Text              string               `json:"text"`
	Model             string               `json:"model"`
	Sources           []GroundingSource    `json:"sources"`
	Supports          []GroundingSupport   `json:"supports"`
	Queries           []string             `json:"queries"`
	URLs              []URLRetrieval       `json:"urls"`
	SearchSuggestions string               `json:"search_suggestions,omitempty"`
}

// GroundedClaim binds a displayed claim to literal provider prose and attribution numbers.
type GroundedClaim struct {
	Quote   string `json:"quote"`
	Sources []int  `json:"sources"`
}

// GroundedDish is formatting of provider prose, not independent menu verification.
type GroundedDish struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Price       string        `json:"price"`
	Currency    string        `json:"currency"`
	PriceBasis  string        `json:"price_basis"`
	Evidence    GroundedClaim `json:"evidence"`
}

// GroundedReview retains a short provider observation and its supporting prose.
type GroundedReview struct {
	Sentiment string        `json:"sentiment"`
	Summary   string        `json:"summary"`
	Evidence  GroundedClaim `json:"evidence"`
}

// GroundedRestaurant packages grounded claims for the shared shortlist layout.
type GroundedRestaurant struct {
	Name          string           `json:"name"`
	Address       string           `json:"address"`
	Description   string           `json:"description"`
	RestaurantURL string           `json:"restaurant_url"`
	MenuURL       string           `json:"menu_url"`
	Evidence      GroundedClaim    `json:"evidence"`
	Dishes        []GroundedDish   `json:"dishes"`
	Reviews       []GroundedReview `json:"reviews"`
}

// GroundedExtraction is the schema of the single plain formatting request.
type GroundedExtraction struct {
	Restaurants []GroundedRestaurant `json:"restaurants"`
}
