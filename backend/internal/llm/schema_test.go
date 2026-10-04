package llm

import (
	"encoding/json"
	"strings"
	"table-for-you/backend/internal/domain"
	"testing"
)

func TestSchemaFlattensRestaurantAndRequiresEvidence(t *testing.T) {
	b, _ := json.Marshal(Schema(domain.Extraction{}))
	for _, field := range []string{"official_url", "identity", "constraints", "source_id"} {
		if !strings.Contains(string(b), field) {
			t.Fatalf("missing schema field %s", field)
		}
	}
}
