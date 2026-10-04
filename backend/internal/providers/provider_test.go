package providers

import (
	"context"
	"encoding/json"
	"nebulaiq/internal/domain"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSourceURLBoundary(t *testing.T) {
	for _, u := range []string{"http://example.com", "https://127.0.0.1", "https://[::1]", "https://user:pass@example.com", "https://localhost", "https://db.internal"} {
		if SafeURL(u) {
			t.Fatalf("unsafe source accepted: %s", u)
		}
	}
	if !SafeURL("https://restaurant.example/menu") {
		t.Fatal("public HTTPS URL rejected")
	}
}
func TestTavilyAuthenticationAndIncompleteExtraction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("missing provider authorization")
		}
		if r.URL.Path == "/extract" {
			json.NewEncoder(w).Encode(map[string]any{"results": []any{}, "failed_results": []any{map[string]string{"url": "https://restaurant.example/menu", "error": "blocked"}}})
			return
		}
		if r.URL.Path != "/search" {
			t.Error("wrong path")
		}
		json.NewEncoder(w).Encode(map[string]any{"results": []domain.SearchHit{{URL: "https://restaurant.example", Content: "Menu"}, {URL: "http://localhost", Content: "unsafe"}}})
	}))
	defer server.Close()
	p := &Tavily{"test-key", server.Client(), server.URL}
	hits, e := p.Search(context.Background(), "vegan lunch", 3)
	if e != nil || len(hits) != 1 {
		t.Fatal("search filtering failed")
	}
	docs, e := p.Extract(context.Background(), []string{"https://restaurant.example/menu"})
	if e != nil || len(docs) != 0 {
		t.Fatal("failed extraction must not become a fabricated source")
	}
}
func TestSchemaFlattensRestaurantAndRequiresEvidence(t *testing.T) {
	b, _ := json.Marshal(Schema(domain.Extraction{}))
	for _, field := range []string{"official_url", "identity", "constraints", "source_id"} {
		if !strings.Contains(string(b), field) {
			t.Fatalf("missing schema field %s", field)
		}
	}
}

func TestOfficialMenuSearchUsesFocusedRetrieval(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		query := body["query"].(string)
		if strings.Contains(query, "official menu") {
			if body["search_depth"] != "advanced" || len(body["exclude_domains"].([]any)) == 0 {
				t.Error("official menu query must focus on source menus")
			}
		} else if body["search_depth"] != "basic" || body["exclude_domains"] != nil {
			t.Error("review search must retain review domains")
		}
		json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
	}))
	defer server.Close()
	p := &Tavily{"test-key", server.Client(), server.URL}
	for _, q := range []string{"Kitchen Barcelona official menu lunch prices", "Kitchen Barcelona negative reviews"} {
		if _, err := p.Search(context.Background(), q, 5); err != nil {
			t.Fatal(err)
		}
	}
}
