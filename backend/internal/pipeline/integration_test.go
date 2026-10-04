package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"table-for-you/backend/internal/config"
	"table-for-you/backend/internal/domain"
	"table-for-you/backend/internal/testutil"
)

const synthetic = "Green Table Barcelona, Spain. Lunch all day. Vegan chickpeas — EUR 12.00. Vegan chickpeas are vegan. Customers said: delicious vegan dishes. Customers said: service was slow."

type controlledModel struct {
	mu    sync.Mutex
	calls int
}

func (m *controlledModel) Name() string { return "controlled-test-model" }
func (m *controlledModel) Generate(ctx context.Context, instruction, input string, shape, out any) (domain.Usage, error) {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()
	switch target := out.(type) {
	case *domain.Interpretation:
		*target = domain.Interpretation{Requirements: domain.Requirements{City: "Barcelona", Country: "Spain", Meal: "lunch", Diet: "vegan", Budget: "25", Currency: "EUR", BudgetBasis: "per_dish"}}
		if strings.Contains(input, "cheaper") {
			target.Requirements.Budget = "10"
		}
	case *domain.Discovery:
		*target = domain.Discovery{Candidates: []domain.Candidate{{Name: "Green Table", OfficialURL: "https://restaurant.example/menu", MenuURL: "https://restaurant.example/menu"}}}
	case *domain.Extraction:
		var data struct {
			Sources []struct {
				ID      string `json:"source_id"`
				Snippet bool   `json:"snippet"`
			} `json:"sources"`
		}
		if err := json.Unmarshal([]byte(input), &data); err != nil {
			return domain.Usage{}, err
		}
		id := ""
		for _, s := range data.Sources {
			if !s.Snippet {
				id = s.ID
				break
			}
		}
		if id == "" {
			return domain.Usage{}, fmt.Errorf("no extracted source")
		}
		cite := func(q string) domain.Citation { return domain.Citation{SourceID: id, Quote: q} }
		*target = domain.Extraction{Restaurants: []domain.Restaurant{{Candidate: domain.Candidate{Name: "Green Table", OfficialURL: "https://restaurant.example/menu", MenuURL: "https://restaurant.example/menu", Identity: cite("Green Table Barcelona, Spain.")}, Dishes: []domain.Dish{{Name: "Vegan chickpeas", Price: "12.00", PriceText: "EUR 12.00", Currency: "EUR", Meal: "lunch", MenuType: "regular", Evidence: cite("Vegan chickpeas — EUR 12.00"), Constraints: []domain.Constraint{{Name: "vegan", Status: "supported", Evidence: cite("Vegan chickpeas are vegan.")}, {Name: "meal availability", Status: "supported", Evidence: cite("Lunch all day.")}}}}, Reviews: []domain.Review{{Sentiment: "positive", Summary: "Synthetic positive review", Evidence: cite("Customers said: delicious vegan dishes.")}, {Sentiment: "negative", Summary: "Synthetic negative review", Evidence: cite("Customers said: service was slow.")}}}}}
	}
	return domain.Usage{ModelCalls: 1, InputTokens: 10, OutputTokens: 10, UsageKnown: true}, nil
}

type controlledSearch struct {
	mu                sync.Mutex
	searches, fetches int
}

func (s *controlledSearch) Search(ctx context.Context, q string, limit int) ([]domain.SearchHit, error) {
	s.mu.Lock()
	s.searches++
	s.mu.Unlock()
	return []domain.SearchHit{{Title: "Green Table", URL: "https://restaurant.example/menu", Content: synthetic}}, nil
}
func (s *controlledSearch) Extract(ctx context.Context, urls []string) (map[string]string, error) {
	s.mu.Lock()
	s.fetches += len(urls)
	s.mu.Unlock()
	out := map[string]string{}
	for _, u := range urls {
		out[u] = synthetic
	}
	return out, nil
}
func TestFullWorkflowThenPersistentCacheReuse(t *testing.T) {
	s := testutil.Database(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	token, _ := s.NewSession(ctx)
	owner, _ := s.Owner(ctx, token)
	chat, _ := s.CreateConversation(ctx, owner)
	model := &controlledModel{}
	search := &controlledSearch{}
	cfg := config.Config{RunTimeout: 15 * time.Second}
	engine := New(ctx, s, model, search, cfg)
	runOnce := func(version int) domain.Run {
		t.Helper()
		run, _, e := s.Accept(ctx, owner, chat.ID, uuid.NewString(), "Synthetic request", version, false, 15*time.Second, 20, 100)
		if e != nil {
			t.Fatal(e)
		}
		engine.Start(owner, run, "Synthetic request", false)
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			r, e := s.Run(ctx, owner, run.ID)
			if e != nil {
				t.Fatal(e)
			}
			if r.Status != "running" {
				return r
			}
			time.Sleep(15 * time.Millisecond)
		}
		t.Fatal("workflow did not finish")
		return domain.Run{}
	}
	first := runOnce(0)
	if first.Status != "completed" || first.Result == nil || len(first.Result.Confirmed) != 1 || len(first.Result.Confirmed[0].Reviews) != 2 {
		t.Fatalf("full controlled-provider workflow failed: %+v", first)
	}
	second := runOnce(1)
	if second.Status != "completed" || second.Usage.Searches != 0 || second.Usage.Fetches != 0 || second.Usage.CacheHits < 4 || second.Usage.ModelCalls >= first.Usage.ModelCalls {
		t.Fatalf("persistent cache did not reduce tool calls: first=%+v second=%+v", first.Usage, second.Usage)
	}
	events, e := s.Events(ctx, second.ID, 0)
	if e != nil || len(events) == 0 {
		t.Fatal("progress events not persisted")
	}
	for i, event := range events {
		if event.Sequence != int64(i+1) {
			t.Fatal("event sequence not contiguous")
		}
	}
	third, _, e := s.Accept(ctx, owner, chat.ID, uuid.NewString(), "Refresh synthetic sources", 2, true, 15*time.Second, 20, 100)
	if e != nil {
		t.Fatal(e)
	}
	engine.Start(owner, third, "Refresh synthetic sources", true)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r, err := s.Run(ctx, owner, third.ID)
		if err != nil {
			t.Fatal(err)
		}
		if r.Status != "running" {
			if r.Status != "completed" || r.Usage.Searches == 0 || r.Usage.Fetches == 0 || r.Usage.ModelCalls != 1 {
				t.Fatalf("refresh failed to fetch while reusing unchanged extraction: %+v", r.Usage)
			}
			fourth, _, err := s.Accept(ctx, owner, chat.ID, uuid.NewString(), "cheaper", 3, false, 15*time.Second, 20, 100)
			if err != nil {
				t.Fatal(err)
			}
			engine.Start(owner, fourth, "cheaper", false)
			end := time.Now().Add(5 * time.Second)
			for time.Now().Before(end) {
				v, err := s.Run(ctx, owner, fourth.ID)
				if err != nil {
					t.Fatal(err)
				}
				if v.Status != "running" {
					if v.Status != "completed" || v.Requirements.Budget != "10" || len(v.Result.Excluded) != 1 || v.Usage.Searches != 0 || v.Usage.Fetches != 0 || v.Usage.ModelCalls != 1 {
						t.Fatalf("budget continuation failed: %+v", v)
					}
					return
				}
				time.Sleep(15 * time.Millisecond)
			}
			t.Fatal("budget continuation did not finish")
		}
		time.Sleep(15 * time.Millisecond)
	}
	t.Fatal("refresh did not finish")
}

type clarificationModel struct{}

func (*clarificationModel) Name() string { return "clarification-test" }
func (*clarificationModel) Generate(ctx context.Context, instruction, input string, shape, out any) (domain.Usage, error) {
	target, ok := out.(*domain.Interpretation)
	if !ok {
		return domain.Usage{}, fmt.Errorf("unexpected research model call before city supplied")
	}
	*target = domain.Interpretation{Requirements: domain.Requirements{Country: "Argentina", Meal: "breakfast"}, Clarification: "What city?"}
	return domain.Usage{ModelCalls: 1}, nil
}
func TestClarificationPersistsWithoutResearchOrFalseSummary(t *testing.T) {
	s := testutil.Database(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	token, _ := s.NewSession(ctx)
	owner, _ := s.Owner(ctx, token)
	chat, _ := s.CreateConversation(ctx, owner)
	search := &controlledSearch{}
	engine := New(ctx, s, &clarificationModel{}, search, config.Config{RunTimeout: 15 * time.Second})
	run, _, err := s.Accept(ctx, owner, chat.ID, uuid.NewString(), "Breakfast in Argentina", 0, false, 15*time.Second, 20, 100)
	if err != nil {
		t.Fatal(err)
	}
	engine.Start(owner, run, "Breakfast in Argentina", false)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r, err := s.Run(ctx, owner, run.ID)
		if err != nil {
			t.Fatal(err)
		}
		if r.Status != "running" {
			if r.Status != "completed" || r.Result == nil || !strings.Contains(r.Result.Clarification, "Which city in Argentina") || !r.Result.ResearchedAt.IsZero() {
				t.Fatalf("invalid clarification result: %+v", r)
			}
			if search.searches != 0 || search.fetches != 0 {
				t.Fatal("clarification called search")
			}
			restored, err := s.Conversation(ctx, owner, chat.ID)
			if err != nil {
				t.Fatal(err)
			}
			answer := restored.Messages[len(restored.Messages)-1].Text
			if answer != r.Result.Clarification || strings.Contains(answer, "Research finished") {
				t.Fatal(answer)
			}
			return
		}
		time.Sleep(15 * time.Millisecond)
	}
	t.Fatal("clarification did not finish")
}
