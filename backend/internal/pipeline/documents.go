package pipeline

import (
	"fmt"
	"sort"
	"strings"

	"table-for-you/backend/internal/domain"
	"table-for-you/backend/internal/storage/postgres"
)

func (j *job) sortedDocs() []domain.Document {
	d := append([]domain.Document(nil), j.docs...)
	sort.Slice(d, func(i, k int) bool {
		a, b := d[i], d[k]
		return a.URL+fmt.Sprint(a.Snippet)+a.Hash < b.URL+fmt.Sprint(b.Snippet)+b.Hash
	})
	return d
}

func (j *job) aliases() []map[string]any {
	out := []map[string]any{}
	for _, d := range j.sortedDocs() {
		out = append(out, map[string]any{"source_id": sourceAlias(d), "url": d.URL, "kind": d.Kind, "snippet": d.Snippet, "text": d.Text, "title": d.Title})
	}
	return out
}

func sourceAlias(d domain.Document) string {
	return "S" + postgres.Hash(d.URL + "|" + d.Hash + fmt.Sprint(d.Snippet))[:12]
}

func (j *job) tag(id string) {
	if j.docCandidates[id] == nil {
		j.docCandidates[id] = map[string]bool{}
	}
	j.docCandidates[id][j.activeCandidate] = true
}

func (j *job) candidateDocs(name string) []domain.Document {
	out := []domain.Document{}
	for _, d := range j.sortedDocs() {
		// Keep snippets as separate evidence: extraction can omit a price range present in the snippet.
		if j.docCandidates[d.ID][name] || (d.Kind == "discovery" && strings.Contains(strings.ToLower(d.Text), strings.ToLower(name))) {
			if !branchURLAllowed(d.URL, j.requirements) || d.Kind == "menu" && deliveryURL(d.URL) {
				continue
			}
			out = append(out, d)
		}
	}
	return out
}
