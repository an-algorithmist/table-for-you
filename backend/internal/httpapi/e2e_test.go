package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"table-for-you/backend/internal/config"
	"table-for-you/backend/internal/domain"
	"table-for-you/backend/internal/pipeline"
	"table-for-you/backend/internal/testutil"
)

// These menus are explicitly synthetic. The test exercises transport, persistence,
// orchestration and evidence validation without constructing any network adapter.
type cityFixture struct {
	city, country, currency, budget, price, dish, description string
}

func (f cityFixture) requirements() domain.Requirements {
	return domain.Requirements{City: f.city, Country: f.country, Meal: "lunch", Diet: "vegan", Budget: f.budget, Currency: f.currency, BudgetBasis: "per_dish"}
}
func (f cityFixture) venue() string { return "Green Table " + f.city }
func (f cityFixture) url() string {
	return "https://" + strings.ToLower(f.city) + ".restaurant.example/menu"
}
func (f cityFixture) identity() string  { return f.venue() + ", " + f.country + "." }
func (f cityFixture) priceLine() string { return f.dish + " — " + f.currency + " " + f.price }
func (f cityFixture) text() string {
	return f.identity() + " Lunch all day. " + f.priceLine() + ". " + f.dish + " is vegan. Customers said: delicious vegan dishes. Customers said: service was slow."
}

type cityModel struct{ fixture cityFixture }

func (m *cityModel) Name() string { return "synthetic-http-e2e" }
func (m *cityModel) Generate(ctx context.Context, instruction, input string, shape, out any) (domain.Usage, error) {
	if err := ctx.Err(); err != nil {
		return domain.Usage{}, err
	}
	fixture := m.fixture
	switch target := out.(type) {
	case *domain.Interpretation:
		var turn struct {
			Message  string              `json:"new_message"`
			Previous domain.Requirements `json:"previous_requirements"`
			Research json.RawMessage     `json:"previous_research"`
		}
		if err := json.Unmarshal([]byte(input), &turn); err != nil {
			return domain.Usage{}, err
		}
		*target = domain.Interpretation{Requirements: fixture.requirements()}
		if strings.Contains(turn.Message, "Which dish") {
			if len(turn.Research) == 0 || turn.Previous.City != fixture.city || turn.Previous.Currency != fixture.currency {
				return domain.Usage{}, fmt.Errorf("follow-up did not receive stored city, currency and research")
			}
			target.Requirements = turn.Previous
			target.Action = "answer_existing"
			target.Answer = fixture.dish + " is listed at " + fixture.currency + " " + fixture.price + "."
		}
	case *domain.Discovery:
		*target = domain.Discovery{Candidates: []domain.Candidate{{Name: fixture.venue(), OfficialURL: fixture.url(), MenuURL: fixture.url()}}}
	case *domain.Extraction:
		var sources struct {
			Sources []struct {
				ID      string `json:"source_id"`
				Snippet bool   `json:"snippet"`
			} `json:"sources"`
		}
		if err := json.Unmarshal([]byte(input), &sources); err != nil {
			return domain.Usage{}, err
		}
		sourceID := ""
		for _, source := range sources.Sources {
			if !source.Snippet {
				sourceID = source.ID
				break
			}
		}
		if sourceID == "" {
			return domain.Usage{}, fmt.Errorf("no extracted menu reached extraction")
		}
		cite := func(quote string) domain.Citation { return domain.Citation{SourceID: sourceID, Quote: quote} }
		*target = domain.Extraction{Restaurants: []domain.Restaurant{{
			Candidate: domain.Candidate{Name: fixture.venue(), OfficialURL: fixture.url(), MenuURL: fixture.url(), Identity: cite(fixture.identity())},
			Dishes: []domain.Dish{{Name: fixture.dish, EnglishName: fixture.dish, Description: fixture.description, Price: fixture.price, PriceText: fixture.currency + " " + fixture.price, Currency: fixture.currency, Meal: "lunch", MenuType: "regular", Evidence: cite(fixture.priceLine()), Constraints: []domain.Constraint{
				{Name: "vegan", Status: "supported", Evidence: cite(fixture.dish + " is vegan.")},
				{Name: "meal availability", Status: "supported", Evidence: cite("Lunch all day.")},
			}}},
			Reviews: []domain.Review{
				{Sentiment: "positive", Summary: "Diners liked the vegan food.", Evidence: cite("Customers said: delicious vegan dishes.")},
				{Sentiment: "negative", Summary: "Some diners reported slow service.", Evidence: cite("Customers said: service was slow.")},
			},
		}}}
	default:
		return domain.Usage{}, fmt.Errorf("unexpected model contract: %T", out)
	}
	return domain.Usage{ModelCalls: 1, InputTokens: 10, OutputTokens: 10, UsageKnown: true}, nil
}

type citySearch struct {
	fixture           cityFixture
	searches, fetches atomic.Int64
}

func (s *citySearch) Search(ctx context.Context, query string, limit int) ([]domain.SearchHit, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.searches.Add(1)
	return []domain.SearchHit{{Title: s.fixture.venue(), URL: s.fixture.url(), Content: s.fixture.text()}}, nil
}
func (s *citySearch) Extract(ctx context.Context, urls []string) (map[string]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.fetches.Add(int64(len(urls)))
	documents := make(map[string]string)
	for _, url := range urls {
		documents[url] = s.fixture.text()
	}
	return documents, nil
}

func TestRestaurantChatEndToEndAcrossCountries(t *testing.T) {
	fixtures := []cityFixture{
		{city: "Barcelona", country: "Spain", currency: "EUR", budget: "25", price: "12.00", dish: "Vegan chickpeas", description: "Chickpeas with vegetables."},
		{city: "Tokyo", country: "Japan", currency: "JPY", budget: "3000", price: "1200", dish: "Tofu bowl", description: "Tofu and vegetables over rice."},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.city, func(t *testing.T) {
			store := testutil.Database(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			search := &citySearch{fixture: fixture}
			cfg := config.Config{Local: true, RunTimeout: 15 * time.Second, OwnerQuota: 20, GlobalQuota: 100}
			engine := pipeline.New(ctx, store, &cityModel{fixture: fixture}, search, cfg)
			server := httptest.NewServer(New(store, engine, cfg))
			defer server.Close()
			client := server.Client()
			client.Timeout = 20 * time.Second
			var session *http.Cookie
			request := func(method, path string, payload any, wantStatus int, out any) {
				t.Helper()
				var encoded []byte
				var err error
				if payload != nil {
					encoded, err = json.Marshal(payload)
					if err != nil {
						t.Fatal(err)
					}
				}
				req, err := http.NewRequestWithContext(ctx, method, server.URL+path, bytes.NewReader(encoded))
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Nebula-Request", "1")
				if session != nil {
					req.AddCookie(session)
				}
				res, err := client.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				defer res.Body.Close()
				if res.StatusCode != wantStatus {
					details, readErr := io.ReadAll(res.Body)
					if readErr != nil {
						t.Fatal(readErr)
					}
					t.Fatalf("%s %s: status %d: %s", method, path, res.StatusCode, details)
				}
				if method == http.MethodPost && path == "/api/access" {
					if len(res.Cookies()) == 0 {
						t.Fatal("browser session missing")
					}
					session = res.Cookies()[0]
				}
				if out != nil {
					if err := json.NewDecoder(res.Body).Decode(out); err != nil {
						t.Fatal(err)
					}
				}
			}
			request(http.MethodPost, "/api/access", map[string]string{"code": ""}, http.StatusOK, nil)
			var chat domain.Conversation
			request(http.MethodPost, "/api/conversations", map[string]string{}, http.StatusCreated, &chat)
			turn := func(text string, version int) domain.Run {
				t.Helper()
				var run domain.Run
				request(http.MethodPost, "/api/conversations/"+chat.ID+"/messages", map[string]any{
					"text": text, "research_mode": "standard", "request_id": uuid.NewString(), "version": version,
				}, http.StatusAccepted, &run)
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/runs/"+run.ID+"/events", nil)
				if err != nil {
					t.Fatal(err)
				}
				req.AddCookie(session)
				res, err := client.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				stream, err := io.ReadAll(res.Body)
				res.Body.Close()
				if err != nil {
					t.Fatal(err)
				}
				if res.StatusCode != http.StatusOK || !strings.Contains(string(stream), "event: progress") || !strings.Contains(string(stream), "event: done") {
					t.Fatalf("SSE did not expose saved progress and completion: %d %s", res.StatusCode, stream)
				}
				request(http.MethodGet, "/api/runs/"+run.ID, nil, http.StatusOK, &run)
				if run.Status != "completed" || run.Result == nil {
					t.Fatalf("research did not complete: %+v", run)
				}
				return run
			}
			first := turn("Find vegan lunch in "+fixture.city+", "+fixture.country+" under "+fixture.currency+" "+fixture.budget+" per dish.", 0)
			if len(first.Result.Confirmed) != 1 {
				t.Fatalf("expected one evidence-backed restaurant: %+v", first.Result)
			}
			restaurant := first.Result.Confirmed[0]
			if len(restaurant.Dishes) != 1 || len(restaurant.Reviews) != 2 {
				t.Fatalf("menu/reviews not retained: %+v", restaurant)
			}
			dish := restaurant.Dishes[0]
			if dish.Currency != fixture.currency || dish.Price != fixture.price || dish.Description != fixture.description || dish.PriceEvidence.SourceID == "" || restaurant.MenuURL != fixture.url() {
				t.Fatalf("price, currency, description or menu attribution changed: %+v", dish)
			}
			beforeSearch, beforeFetch := search.searches.Load(), search.fetches.Load()
			second := turn("Which dish did you recommend and what is its price?", 1)
			if second.Requirements.City != fixture.city || second.Requirements.Currency != fixture.currency || second.Result.Answer != fixture.dish+" is listed at "+fixture.currency+" "+fixture.price+"." || len(second.Result.Confirmed) != 1 {
				t.Fatalf("follow-up lost stored context/cards: %+v", second)
			}
			if search.searches.Load() != beforeSearch || search.fetches.Load() != beforeFetch || second.Usage.Searches != 0 || second.Usage.Fetches != 0 {
				t.Fatal("evidence question unexpectedly repeated web research")
			}
			var restored domain.Conversation
			request(http.MethodGet, "/api/conversations/"+chat.ID, nil, http.StatusOK, &restored)
			if restored.Version != 2 || len(restored.Messages) != 4 || len(restored.Runs) != 2 || restored.Messages[3].Text != second.Result.Answer {
				t.Fatalf("history did not persist both turns: %+v", restored)
			}
		})
	}
}
