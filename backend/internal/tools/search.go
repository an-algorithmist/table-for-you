package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"table-for-you/backend/internal/domain"
	"table-for-you/backend/internal/providers"
)

// Tavily retrieves public search snippets and extracted source text.
type Tavily struct {
	key    string
	client *http.Client
	base   string
}

// NewTavily configures authenticated search without exposing the key to callers.
func NewTavily(key string) (*Tavily, error) {
	if key == "" {
		return nil, errors.New("TAVILY_API_KEY is not configured")
	}
	return &Tavily{key, providers.HTTPClient(), "https://api.tavily.com"}, nil
}
func (t *Tavily) post(ctx context.Context, path string, body, out any) error {
	b, e := json.Marshal(body)
	if e != nil {
		return e
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, t.base+path, bytes.NewReader(b))
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.key)
	resp, e := t.client.Do(req)
	if e != nil {
		return e
	}
	return providers.ReadJSON(resp, out)
}

// Search filters unsafe URLs and focuses official-menu queries on menu sources.
func (t *Tavily) Search(ctx context.Context, query string, limit int) ([]domain.SearchHit, error) {
	return t.SearchPurpose(ctx, query, "discovery", limit)
}

// SearchPurpose controls source exclusions independently of query wording.
func (t *Tavily) SearchPurpose(ctx context.Context, query, purpose string, limit int) ([]domain.SearchHit, error) {
	var out struct {
		Results []domain.SearchHit `json:"results"`
	}
	body := map[string]any{"query": query, "max_results": limit, "search_depth": "basic", "include_answer": false, "include_raw_content": false, "topic": "general"}
	if purpose == "menu" || purpose == "price" {
		body["search_depth"] = "advanced"
		body["exclude_domains"] = []string{"tripadvisor.com", "tripadvisor.co.uk", "yelp.com", "facebook.com", "instagram.com", "reddit.com", "ubereats.com", "grubhub.com", "postmates.com", "doordash.com", "seamless.com", "deliveroo.com", "just-eat.com", "swiggy.com"}
	}
	if purpose == "price" {
		body["exclude_domains"] = []string{"ubereats.com", "grubhub.com", "doordash.com", "deliveroo.com", "just-eat.com", "swiggy.com"}
	}
	e := t.post(ctx, "/search", body, &out)
	if e != nil {
		return nil, e
	}
	hits := []domain.SearchHit{}
	for _, h := range out.Results {
		if providers.SafeURL(h.URL) {
			if len(h.Content) > 10000 {
				h.Content = h.Content[:10000]
			}
			hits = append(hits, h)
		}
	}
	return hits, nil
}

// Extract preserves the compatibility contract for text-only consumers.
func (t *Tavily) Extract(ctx context.Context, urls []string) (map[string]string, error) {
	docs, _, err := t.ExtractDetailed(ctx, urls)
	return docs, err
}

// ExtractDetailed retains successful pages and per-URL provider failure reasons.
func (t *Tavily) ExtractDetailed(ctx context.Context, urls []string) (map[string]string, map[string]string, error) {
	for _, u := range urls {
		if !providers.SafeURL(u) {
			return nil, nil, errors.New("unsafe source URL")
		}
	}
	var out struct {
		Results []struct {
			URL     string `json:"url"`
			Content string `json:"raw_content"`
		} `json:"results"`
		Failed []struct {
			URL   string `json:"url"`
			Error string `json:"error"`
		} `json:"failed_results"`
	}
	err := t.post(ctx, "/extract", map[string]any{"urls": urls, "extract_depth": "advanced", "format": "markdown", "timeout": 20}, &out)
	if err != nil {
		return nil, nil, err
	}
	docs := map[string]string{}
	failures := map[string]string{}
	for _, r := range out.Results {
		if len(r.Content) > 1024*1024 {
			r.Content = r.Content[:1024*1024]
		}
		if r.Content != "" {
			docs[r.URL] = r.Content
		}
	}
	for _, r := range out.Failed {
		failures[r.URL] = r.Error
	}
	return docs, failures, nil
}
