package domain

import "time"

// SearchHit is unvalidated discovery text, not a verified menu fact.
type SearchHit struct {
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	Content   string    `json:"content"`
	Score     float64   `json:"score"`
	FetchedAt time.Time `json:"fetched_at"`
}

// Document is a timestamped source snapshot retained for literal citation checks.
type Document struct {
	Method     string    `json:"method,omitempty"`
	ID         string    `json:"id"`
	URL        string    `json:"url"`
	Title      string    `json:"title"`
	Text       string    `json:"text"`
	Kind       string    `json:"kind"`
	Snippet    bool      `json:"snippet"`
	Historical bool      `json:"historical"`
	FetchedAt  time.Time `json:"fetched_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	Hash       string    `json:"hash"`
}

// Citation ties an exact source passage to its immutable document ID.
type Citation struct {
	SourceID string `json:"source_id"`
	Quote    string `json:"quote"`
}

// Constraint records whether a source explicitly supports a requested condition.
type Constraint struct {
	Name     string   `json:"name"`
	Status   string   `json:"status"`
	Evidence Citation `json:"evidence"`
	Reason   string   `json:"reason"`
}

// Dish contains translated menu data with separate price and dietary evidence.
type Dish struct {
	Name             string       `json:"name"`
	EnglishName      string       `json:"english_name"`
	Description      string       `json:"description"`
	Price            string       `json:"price"`
	Currency         string       `json:"currency"`
	PriceText        string       `json:"price_text"`
	CurrencyBasis    string       `json:"currency_basis"`
	CurrencyEvidence Citation     `json:"currency_evidence"`
	PriceEvidence    Citation     `json:"price_evidence"`
	MenuType         string       `json:"menu_type"`
	ValidDate        string       `json:"valid_date"`
	Meal             string       `json:"meal"`
	Evidence         Citation     `json:"evidence"`
	Constraints      []Constraint `json:"constraints"`
	Status           string       `json:"status"`
}

// Review is a short paraphrased review observation with a supporting passage.
type Review struct {
	Sentiment string   `json:"sentiment"`
	Topic     string   `json:"topic"`
	Summary   string   `json:"summary"`
	Published string   `json:"published"`
	Evidence  Citation `json:"evidence"`
}

// Candidate identifies a proposed restaurant branch before verification.
type Candidate struct {
	Name        string   `json:"name"`
	Address     string   `json:"address"`
	OfficialURL string   `json:"official_url"`
	MenuURL     string   `json:"menu_url"`
	Reason      string   `json:"reason"`
	Identity    Citation `json:"identity"`
}

// Discovery is the bounded restaurant shortlist returned by the discovery model.
type Discovery struct {
	Candidates []Candidate `json:"candidates"`
}

// Restaurant groups a branch with validated dishes, review evidence and remaining gaps.
type Restaurant struct {
	Candidate
	VenueType        string        `json:"venue_type"`
	VenueEvidence    Citation      `json:"venue_evidence"`
	PriceRange       PriceRange    `json:"price_range"`
	Estimate         PriceEstimate `json:"estimate"`
	IdentityVerified bool          `json:"identity_verified"`
	Dishes           []Dish        `json:"dishes"`
	Reviews          []Review      `json:"reviews"`
	Limitations      []string      `json:"limitations"`
}

// PriceRange is a source-listed range, separate from derived per-person guidance.
type PriceRange struct {
	Low      string   `json:"low"`
	High     string   `json:"high"`
	Currency string   `json:"currency"`
	Evidence Citation `json:"evidence"`
}

// PriceEstimate is derived planning guidance whose basis and supporting citations remain visible.
type PriceEstimate struct {
	Low      string     `json:"low"`
	High     string     `json:"high"`
	Currency string     `json:"currency"`
	Basis    string     `json:"basis"`
	Evidence []Citation `json:"evidence"`
}

// Extraction is schema-constrained menu and review output before deterministic validation.
type Extraction struct {
	Restaurants []Restaurant `json:"restaurants"`
}
