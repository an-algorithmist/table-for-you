package evidence

import (
	"html"
	"regexp"
	"strings"

	"table-for-you/backend/internal/domain"
)

var markdownLink = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)

func normalize(s string) string {
	s = html.UnescapeString(s)
	s = markdownLink.ReplaceAllString(s, "$1")
	s = strings.NewReplacer("**", "", "__", "", "`", "").Replace(s)
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// Supported reports whether a non-trivial citation occurs in the referenced document.
func Supported(c domain.Citation, docs map[string]domain.Document) bool {
	d, ok := docs[c.SourceID]
	return ok && len(strings.TrimSpace(c.Quote)) >= 8 && strings.Contains(normalize(d.Text), normalize(c.Quote))
}
