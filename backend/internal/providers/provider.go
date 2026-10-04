package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Error preserves provider status without leaking response bodies or credentials.
type Error struct {
	Kind   string
	Status int
}

func (e *Error) Error() string { return fmt.Sprintf("%s provider failed (HTTP %d)", e.Kind, e.Status) }

// PublicError maps internal failures to actionable, credential-free UI messages.
func PublicError(err error) string {
	var e *Error
	if errors.As(err, &e) {
		switch e.Status {
		case 401, 403:
			return "Provider credentials or project access were rejected. Check server configuration."
		case 429:
			if e.Kind == "grounded model" {
				return "Google-grounded quota is unavailable. Check the API project’s billing and limits, or switch off Google-grounded research to use standard research. No paid fallback was used."
			}
			return "Provider rate limit or quota reached. Retry later; no paid fallback was used."
		case 500, 502, 503, 504:
			return "The model or search service is temporarily unavailable. Retry later; any collected evidence is preserved."
		default:
			return "A provider request failed. Partial evidence may be available; retry explicitly."
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "Research timed out. Retry explicitly; cached evidence may be reused."
	}
	if errors.Is(err, context.Canceled) {
		return "Research was cancelled."
	}
	return "Research could not finish. Check server configuration and retry explicitly."
}

// SafeURL rejects non-public URL forms before provider retrieval.
func SafeURL(raw string) bool {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return false
	}
	h := strings.ToLower(u.Hostname())
	if h == "localhost" || strings.HasSuffix(h, ".localhost") || strings.HasSuffix(h, ".local") || strings.HasSuffix(h, ".internal") {
		return false
	}
	return !strings.Contains(h, ":") && !isIP(h)
}
func isIP(h string) bool {
	for _, c := range h {
		if (c < '0' || c > '9') && c != '.' {
			return false
		}
	}
	return true
}

// HTTPClient disables provider redirects and bounds transport duration.
func HTTPClient() *http.Client {
	return &http.Client{Timeout: 25 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return errors.New("provider redirects are not allowed")
	}}
}

// ReadJSON decodes a bounded provider response and preserves HTTP status failures.
func ReadJSON(resp *http.Response, out any) error {
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &Error{Kind: "search", Status: resp.StatusCode}
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024+1))
	if e != nil {
		return e
	}
	if len(b) > 4*1024*1024 {
		return errors.New("provider response exceeds size limit")
	}
	return json.Unmarshal(b, out)
}
