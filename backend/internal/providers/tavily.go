package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"nebulaiq/internal/domain"
	"net/http"
	"strings"
)

type Tavily struct {
	key    string
	client *http.Client
	base   string
}

func NewTavily(key string) (*Tavily, error) {
	if key == "" {
		return nil, errors.New("TAVILY_API_KEY is not configured")
	}
	return &Tavily{key, HTTPClient(), "https://api.tavily.com"}, nil
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
	return readJSON(resp, out)
}
func (t *Tavily) Search(ctx context.Context, query string, limit int) ([]domain.SearchHit, error) {
	var out struct {
		Results []domain.SearchHit `json:"results"`
	}
	body := map[string]any{"query": query, "max_results": limit, "search_depth": "basic", "include_answer": false, "include_raw_content": false, "topic": "general"}
	if strings.Contains(strings.ToLower(query), "official menu") {
		body["search_depth"] = "advanced"
		body["exclude_domains"] = []string{"tripadvisor.com", "tripadvisor.co.uk", "yelp.com", "facebook.com", "instagram.com", "reddit.com", "happycow.net", "zomato.com", "ubereats.com", "grubhub.com", "restaurantguru.com", "restaurants-us.com", "postmates.com", "doordash.com", "seamless.com", "deliveroo.com", "just-eat.com", "swiggy.com"}
	}
	e := t.post(ctx, "/search", body, &out)
	if e != nil {
		return nil, e
	}
	hits := []domain.SearchHit{}
	for _, h := range out.Results {
		if SafeURL(h.URL) {
			if len(h.Content) > 10000 {
				h.Content = h.Content[:10000]
			}
			hits = append(hits, h)
		}
	}
	return hits, nil
}
func (t *Tavily) Extract(ctx context.Context, urls []string) (map[string]string, error) {
	for _, u := range urls {
		if !SafeURL(u) {
			return nil, errors.New("unsafe source URL")
		}
	}
	var out struct {
		Results []struct {
			URL     string `json:"url"`
			Content string `json:"raw_content"`
		} `json:"results"`
	}
	e := t.post(ctx, "/extract", map[string]any{"urls": urls, "extract_depth": "advanced", "format": "markdown", "timeout": 20}, &out)
	if e != nil {
		return nil, e
	}
	docs := map[string]string{}
	for _, r := range out.Results {
		if len(r.Content) > 1024*1024 {
			r.Content = r.Content[:1024*1024]
		}
		if r.Content != "" {
			docs[r.URL] = r.Content
		}
	}
	return docs, nil
}
