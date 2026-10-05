package pipeline

import (
	"golang.org/x/text/unicode/norm"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"table-for-you/backend/internal/domain"
	"table-for-you/backend/internal/providers"
)

var markdownTarget = regexp.MustCompile(`!?\[[^\]]*\]\(([^\s\)]+)`)
var sourceLink = regexp.MustCompile(`https://[^\s<>"\)\]]+`)
var datedMenuFile = regexp.MustCompile(`(?i)(?:menu|carta|cardapio)[^/]{0,30}?((?:19|20)\d{2})`)
var datedPath = regexp.MustCompile(`/((?:19|20)\d{2})/`)
var menuPrice = regexp.MustCompile(`(?i)(?:EUR|PLN|USD|CAD|JPY|ARS|BRL|GBP|CZK|HUF|CHF|DKK|SEK|NOK|INR|€|£|\$|zł|₹|円|¥)\s*[0-9]+(?:[.,][0-9]{1,2})?|[0-9]+(?:[.,][0-9]{1,2})?\s*(?:EUR|PLN|USD|GBP|CZK|HUF|CHF|DKK|SEK|NOK|INR|€|£|zł)`)

var nameSeparators = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// Search ranking alone does not establish that a result concerns this branch.
// Require the restaurant name in title, URL or snippet before spending fetch quota.
func relevantHits(hits []domain.SearchHit, name string) []domain.SearchHit {
	words := strings.Fields(nameSeparators.ReplaceAllString(foldName(name), " "))
	if len(words) == 0 {
		return nil
	}
	out := []domain.SearchHit{}
	for _, h := range hits {
		body := " " + nameSeparators.ReplaceAllString(foldName(h.Title+" "+h.URL+" "+h.Content), " ") + " "
		match := true
		for _, word := range words {
			if word == "the" || word == "restaurant" {
				continue
			}
			if !strings.Contains(body, " "+word+" ") && !strings.Contains(strings.ReplaceAll(body, " ", ""), strings.Join(words, "")) {
				match = false
				break
			}
		}
		if match {
			out = append(out, h)
		}
	}
	return out
}

func prioritizeMenus(hits []domain.SearchHit, c domain.Candidate, req domain.Requirements) []domain.SearchHit {
	out := append([]domain.SearchHit(nil), hits...)
	score := func(h domain.SearchHit) int {
		u, _ := url.Parse(h.URL)
		if u == nil {
			return -100
		}
		s := 0
		body := strings.ToLower(h.Content + " " + h.Title)
		path := strings.ToLower(u.Path)
		if strings.Contains(body, strings.ToLower(c.Name)) {
			s += 4
		}
		if strings.Contains(body, strings.ToLower(req.City)) {
			s += 5
		}
		if strings.Contains(body, strings.ToLower(req.Country)) {
			s += 3
		}
		for _, word := range []string{"menu", "carta", ".pdf"} {
			if strings.Contains(path, word) {
				s += 3
			}
		}
		for _, word := range []string{"tripadvisor", "yelp", "happycow", "reddit", "facebook", "blog", "travel", "guid", "michelin"} {
			if strings.Contains(strings.ToLower(h.URL), word) {
				s -= 4
			}
		}
		if menuPrice.MatchString(h.Content) || numericRow.MatchString(h.Content) {
			s += 8
		}
		if aggregatorURL(h.URL) {
			s -= 15
		}
		if official, e := url.Parse(c.OfficialURL); e == nil && official.Hostname() != "" && official.Hostname() == u.Hostname() {
			s += 10
		}
		return s
	}
	sort.SliceStable(out, func(i, j int) bool { return score(out[i]) > score(out[j]) })
	return out
}
func (j *job) followMenuLinks(start int) error {
	urls := []string{}
	for _, d := range j.docs[start:] {
		if d.Snippet || d.Kind != "menu" {
			continue
		}
		origin, _ := url.Parse(d.URL)
		links := sourceLink.FindAllString(d.Text, -1)
		for _, match := range markdownTarget.FindAllStringSubmatch(d.Text, -1) {
			u, e := url.Parse(match[1])
			if e == nil {
				links = append(links, origin.ResolveReference(u).String())
			}
		}
		for _, link := range links {
			u, e := url.Parse(link)
			if e != nil || !providers.SafeURL(link) || !branchURLAllowed(link, j.requirements) {
				continue
			}
			path := strings.ToLower(u.Path)
			image := strings.HasSuffix(path, ".png") || strings.HasSuffix(path, ".jpg") || strings.HasSuffix(path, ".jpeg")
			if image && (strings.Contains(path, "screenshot") || strings.Contains(path, "menu")) && j.visionReads < 2 {
				_ = j.readVisualMenu(link)
				continue
			}
			if !strings.Contains(path, "menu") && !strings.Contains(path, "carta") {
				continue
			}
			if !branchURLAllowed(link, j.requirements) {
				continue
			}
			if u.Hostname() != origin.Hostname() && !strings.HasSuffix(path, ".pdf") {
				continue
			}
			if len(urls) < 3 {
				urls = append(urls, link)
			}
		}
	}
	if len(urls) > 0 {
		return j.fetch(urls, "menu")
	}
	return nil
}
func historicalURL(u string) bool {
	threshold := time.Now().UTC().AddDate(-1, 0, 0).Format("2006")
	for _, pattern := range []*regexp.Regexp{datedPath, datedMenuFile} {
		m := pattern.FindStringSubmatch(u)
		if len(m) > 1 && m[1] < threshold {
			return true
		}
	}
	return false
}

func foldName(v string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(v)) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
