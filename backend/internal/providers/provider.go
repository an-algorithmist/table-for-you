package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"nebulaiq/internal/domain"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"
)

type Model interface {
	Generate(context.Context, string, string, any, any) (domain.Usage, error)
	Name() string
}
type GroundedModel interface {
	Ground(context.Context, string, string, int) (*domain.Grounding, domain.Usage, error)
}
type Search interface {
	Search(context.Context, string, int) ([]domain.SearchHit, error)
	Extract(context.Context, []string) (map[string]string, error)
}
type Error struct {
	Kind   string
	Status int
}

func (e *Error) Error() string { return fmt.Sprintf("%s provider failed (HTTP %d)", e.Kind, e.Status) }
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
func Schema(v any) any { return schemaType(reflect.TypeOf(v)) }
func schemaType(t reflect.Type) any {
	if t.Kind() == reflect.Pointer {
		return schemaType(t.Elem())
	}
	switch t.Kind() {
	case reflect.Struct:
		p := map[string]any{}
		required := []string{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.Anonymous {
				m := schemaType(f.Type).(map[string]any)
				for k, v := range m["properties"].(map[string]any) {
					p[k] = v
					required = append(required, k)
				}
				continue
			}
			name := strings.Split(f.Tag.Get("json"), ",")[0]
			if name == "" || name == "-" {
				continue
			}
			p[name] = schemaType(f.Type)
			required = append(required, name)
		}
		return map[string]any{"type": "object", "properties": p, "required": required}
	case reflect.Slice:
		return map[string]any{"type": "array", "items": schemaType(t.Elem())}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int64:
		return map[string]any{"type": "integer"}
	default:
		return map[string]any{"type": "string"}
	}
}
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
func HTTPClient() *http.Client {
	return &http.Client{Timeout: 25 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return errors.New("provider redirects are not allowed")
	}}
}
func readJSON(resp *http.Response, out any) error {
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
