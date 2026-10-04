package domain

import "time"

type Requirements struct {
	City        string   `json:"city"`
	Country     string   `json:"country"`
	Meal        string   `json:"meal"`
	Diet        string   `json:"diet"`
	Excluded    []string `json:"excluded"`
	Budget      string   `json:"budget"`
	Currency    string   `json:"currency"`
	BudgetBasis string   `json:"budget_basis"`
	Cuisine     string   `json:"cuisine"`
	Area        string   `json:"area"`
	VenueOnly   bool     `json:"venue_only"`
}
type Interpretation struct {
	Action        string       `json:"action"`
	Answer        string       `json:"answer"`
	Requirements  Requirements `json:"requirements"`
	Clarification string       `json:"clarification"`
	Query         string       `json:"query"`
}
type Conversation struct {
	ID           string       `json:"id"`
	Title        string       `json:"title"`
	Requirements Requirements `json:"requirements"`
	Version      int          `json:"version"`
	UpdatedAt    time.Time    `json:"updated_at"`
	Messages     []Message    `json:"messages,omitempty"`
	Runs         []Run        `json:"runs,omitempty"`
}
type Message struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}
type Run struct {
	ID             string       `json:"id"`
	ConversationID string       `json:"conversation_id"`
	Status         string       `json:"status"`
	Requirements   Requirements `json:"requirements"`
	Result         *Result      `json:"result,omitempty"`
	Error          string       `json:"error,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
	Usage          Usage        `json:"usage"`
}
type Usage struct {
	Mode            string `json:"research_mode,omitempty"`
	GroundedModel   string `json:"grounded_model,omitempty"`
	ThinkingTokens  int64  `json:"thinking_tokens,omitempty"`
	ToolInputTokens int64  `json:"tool_input_tokens,omitempty"`
	Searches        int    `json:"searches"`
	Fetches         int    `json:"fetches"`
	ModelCalls      int    `json:"model_calls"`
	CacheHits       int    `json:"cache_hits"`
	InputTokens     int64  `json:"input_tokens"`
	OutputTokens    int64  `json:"output_tokens"`
	UsageKnown      bool   `json:"usage_known"`
}
type Event struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}
type SearchHit struct {
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	Content   string    `json:"content"`
	Score     float64   `json:"score"`
	FetchedAt time.Time `json:"fetched_at"`
}
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
type Citation struct {
	SourceID string `json:"source_id"`
	Quote    string `json:"quote"`
}
type Constraint struct {
	Name     string   `json:"name"`
	Status   string   `json:"status"`
	Evidence Citation `json:"evidence"`
	Reason   string   `json:"reason"`
}
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
type Review struct {
	Sentiment string   `json:"sentiment"`
	Topic     string   `json:"topic"`
	Summary   string   `json:"summary"`
	Published string   `json:"published"`
	Evidence  Citation `json:"evidence"`
}
type Candidate struct {
	Name        string   `json:"name"`
	Address     string   `json:"address"`
	OfficialURL string   `json:"official_url"`
	MenuURL     string   `json:"menu_url"`
	Reason      string   `json:"reason"`
	Identity    Citation `json:"identity"`
}
type Discovery struct {
	Candidates []Candidate `json:"candidates"`
}
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
type PriceRange struct {
	Low      string   `json:"low"`
	High     string   `json:"high"`
	Currency string   `json:"currency"`
	Evidence Citation `json:"evidence"`
}
type PriceEstimate struct {
	Low      string     `json:"low"`
	High     string     `json:"high"`
	Currency string     `json:"currency"`
	Basis    string     `json:"basis"`
	Evidence []Citation `json:"evidence"`
}
type Extraction struct {
	Restaurants []Restaurant `json:"restaurants"`
}
type GroundingSource struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
	Title  string `json:"title"`
}
type GroundingSupport struct {
	Text    string `json:"text"`
	Sources []int  `json:"sources"`
}
type URLRetrieval struct {
	URL    string `json:"url"`
	Status string `json:"status"`
}
type Grounding struct {
	Text              string             `json:"text"`
	Model             string             `json:"model"`
	Sources           []GroundingSource  `json:"sources"`
	Supports          []GroundingSupport `json:"supports"`
	Queries           []string           `json:"queries"`
	URLs              []URLRetrieval     `json:"urls"`
	SearchSuggestions string             `json:"search_suggestions,omitempty"`
}
type Result struct {
	Grounding         *Grounding   `json:"grounding,omitempty"`
	Answer            string       `json:"answer,omitempty"`
	ValidationVersion string       `json:"validation_version,omitempty"`
	Clarification     string       `json:"clarification,omitempty"`
	Requirements      Requirements `json:"requirements"`
	Confirmed         []Restaurant `json:"confirmed"`
	Alternatives      []Restaurant `json:"alternatives"`
	Excluded          []Restaurant `json:"excluded"`
	Sources           []Document   `json:"sources"`
	Limitations       []string     `json:"limitations"`
	ResearchedAt      time.Time    `json:"researched_at"`
	Usage             Usage        `json:"usage"`
}
