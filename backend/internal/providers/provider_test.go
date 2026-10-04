package providers

import (
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
