package pipeline

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"table-for-you/backend/internal/domain"
	"table-for-you/backend/internal/providers"
	"table-for-you/backend/internal/storage/postgres"
)

func (j *job) search(query string, limit int) ([]domain.SearchHit, error) {
	key := postgres.Hash("tavily-branch-menu-v2|" + strings.Join(strings.Fields(query), " ") + fmt.Sprint("|", limit))
	if !j.refresh {
		hits, ok, e := j.e.Store.Search(j.ctx, j.owner, key)
		if e != nil {
			return nil, e
		}
		if ok {
			j.use.CacheHits++
			_ = j.event("cache.hit", "Reused a fresh search result: "+query)
			return hits, nil
		}
	}
	if j.use.Searches >= 14 {
		return nil, errors.New("search call budget exhausted")
	}
	j.use.Searches++
	_ = j.event("search.started", query)
	hits, e := j.e.Search.Search(j.ctx, query, limit)
	if e != nil {
		return nil, e
	}
	now := time.Now().UTC()
	for i := range hits {
		hits[i].FetchedAt = now
	}
	if e = j.e.Store.PutSearch(j.ctx, j.owner, key, hits); e != nil {
		return nil, e
	}
	return hits, nil
}

func (j *job) snippet(h domain.SearchHit, kind string) {
	if h.Content == "" {
		return
	}
	for _, d := range j.docs {
		if d.URL == h.URL && d.Snippet {
			j.tag(d.ID)
			return
		}
	}
	id := uuid.NewString()
	t := h.FetchedAt
	if t.IsZero() {
		t = time.Now().UTC()
	}
	j.docs = append(j.docs, domain.Document{ID: id, URL: h.URL, Title: h.Title, Text: h.Content, Kind: kind, Snippet: true, Historical: historicalURL(h.URL), FetchedAt: t, ExpiresAt: t.Add(6 * time.Hour), Hash: postgres.Hash(h.Content)})
	j.tag(id)
}

func (j *job) fetch(urls []string, kind string) error {
	urls = filterBranchURLs(urls, j.activeCandidate, j.requirements)
	if j.attemptedURLs == nil {
		j.attemptedURLs = map[string]bool{}
	}
	seen := map[string]bool{}
	miss := []string{}
	for _, u := range urls {
		if seen[u] || !providers.SafeURL(u) {
			continue
		}
		seen[u] = true
		already := false
		for _, d := range j.docs {
			if d.URL == u && !d.Snippet {
				already = true
				j.tag(d.ID)
			}
		}
		if already {
			continue
		}
		key := postgres.Hash("tavily-markdown-advanced-v4-menu|" + kind + "|" + u)
		if !j.refresh {
			d, ok, e := j.e.Store.Document(j.ctx, key)
			if e != nil {
				return e
			}
			if ok {
				d.Historical = d.Historical || historicalURL(d.URL)
				j.docs = append(j.docs, d)
				j.tag(d.ID)
				j.use.CacheHits++
				if kind == "menu" && strings.Contains(strings.ToLower(u), ".pdf") {
					_ = j.readVisualMenu(u)
				}
				_ = j.event("cache.hit", "Reused source fetched "+d.FetchedAt.Format(time.RFC3339)+": "+u)
				continue
			}
		}
		if j.attemptedURLs[u] {
			continue
		}
		if j.use.Fetches+len(miss) < 20 {
			j.attemptedURLs[u] = true
			miss = append(miss, u)
		}
	}
	if len(miss) == 0 {
		return nil
	}
	j.use.Fetches += len(miss)
	_ = j.event("source.fetching", fmt.Sprintf("Reading %d %s sources.", len(miss), kind))
	out, e := j.e.Search.Extract(j.ctx, miss)
	if e != nil {
		return e
	}
	for _, u := range miss {
		text := out[u]
		if text == "" {
			if kind == "menu" && j.readVisualMenu(u) == nil {
				continue
			}
			j.limitations = append(j.limitations, "Could not extract source: "+u)
			continue
		}
		if len(text) > 60000 {
			text = strings.ToValidUTF8(text[:60000], "")
			j.limitations = append(j.limitations, "Source text was bounded to 60 KB: "+u)
		}
		now := time.Now().UTC()
		d := domain.Document{ID: uuid.NewString(), URL: u, Title: u, Text: text, Kind: kind, Historical: historicalURL(u), FetchedAt: now, ExpiresAt: now.Add(postgres.DocTTL(kind)), Hash: postgres.Hash(text)}
		key := postgres.Hash("tavily-markdown-advanced-v4-menu|" + kind + "|" + u)
		if e = j.e.Store.PutDocument(j.ctx, key, d); e != nil {
			return e
		}
		j.docs = append(j.docs, d)
		j.tag(d.ID)
		_ = j.event("source.fetched", u)
		if kind == "menu" && strings.Contains(strings.ToLower(u), ".pdf") {
			_ = j.readVisualMenu(u)
		}
	}
	return nil
}
