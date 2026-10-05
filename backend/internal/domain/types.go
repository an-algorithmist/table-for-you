package domain

import "time"

// Requirements records explicit traveller constraints; budget remains decimal text to avoid float rounding.
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

// Interpretation is the model intent result used to choose research, clarification or stored-evidence answers.
type Interpretation struct {
	Action        string       `json:"action"`
	Answer        string       `json:"answer"`
	Requirements  Requirements `json:"requirements"`
	Clarification string       `json:"clarification"`
	Query         string       `json:"query"`
}

// Conversation is an owned chat snapshot with optimistic request versioning.
type Conversation struct {
	ID           string       `json:"id"`
	Title        string       `json:"title"`
	Requirements Requirements `json:"requirements"`
	Version      int          `json:"version"`
	UpdatedAt    time.Time    `json:"updated_at"`
	Messages     []Message    `json:"messages,omitempty"`
	Runs         []Run        `json:"runs,omitempty"`
}

// Message is a persisted English chat turn.
type Message struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

// Run is a durable research execution, including its terminal result and observed usage.
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

// Usage records attempted tool calls and provider-reported token counts; UsageKnown distinguishes missing metadata.
type Usage struct {
	Mode              string           `json:"research_mode,omitempty"`
	GroundedModel     string           `json:"grounded_model,omitempty"`
	ThinkingTokens    int64            `json:"thinking_tokens,omitempty"`
	ToolInputTokens   int64            `json:"tool_input_tokens,omitempty"`
	Searches          int              `json:"searches"`
	ExtractRequests   int              `json:"extract_requests,omitempty"`
	StageMilliseconds map[string]int64 `json:"stage_ms,omitempty"`
	Fetches           int              `json:"fetches"`
	ModelCalls        int              `json:"model_calls"`
	CacheHits         int              `json:"cache_hits"`
	InputTokens       int64            `json:"input_tokens"`
	OutputTokens      int64            `json:"output_tokens"`
	UsageKnown        bool             `json:"usage_known"`
}

// Event is a persisted SSE trace entry; Sequence supports reconnect replay.
type Event struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// Result is the persisted renderer contract for recommendations, answers and clarification.
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
