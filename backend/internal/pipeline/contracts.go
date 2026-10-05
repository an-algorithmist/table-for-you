package pipeline

import (
	"context"

	"table-for-you/backend/internal/domain"
)

// Model extracts schema-constrained judgments without exposing provider SDK types.
// Usage describes the attempt even when the returned error prevents an answer.
type Model interface {
	Generate(context.Context, string, string, any, any) (domain.Usage, error)
	Name() string
}

// GroundedModel is an optional, explicitly selected paid research route.
// Implementations must not automatically retry this operation.
type GroundedModel interface {
	Ground(context.Context, string, string, int) (*domain.Grounding, domain.Usage, error)
}

// Search retrieves textual evidence independently of model-specific grounding.
type Search interface {
	Search(context.Context, string, int) ([]domain.SearchHit, error)
	Extract(context.Context, []string) (map[string]string, error)
}

// MenuReader is an optional fallback for PDF or image menu layouts.
type MenuReader interface {
	ReadMenu(context.Context, string) (string, domain.Usage, error)
}

// PurposeSearch separates query intent from its text.
type PurposeSearch interface {
	SearchPurpose(context.Context, string, string, int) ([]domain.SearchHit, error)
}

// DetailedExtractor exposes individual failures without discarding successful documents.
type DetailedExtractor interface {
	ExtractDetailed(context.Context, []string) (map[string]string, map[string]string, error)
}
