package pipeline

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"table-for-you/backend/internal/config"
	"table-for-you/backend/internal/domain"
	"table-for-you/backend/internal/storage/postgres"
	"table-for-you/backend/internal/testutil"
)

type controlledGround struct {
	calls int
	fail  bool
}

func (g *controlledGround) Ground(ctx context.Context, instruction, input string, max int) (*domain.Grounding, domain.Usage, error) {
	g.calls++
	use := domain.Usage{ModelCalls: 1, Searches: 2, GroundedModel: "test-grounded", UsageKnown: true}
	if g.fail {
		return nil, use, errors.New("test failure")
	}
	return &domain.Grounding{Text: "Soup EUR 12", Model: "test-grounded", Sources: []domain.GroundingSource{{Number: 1, URL: "https://kitchen.example/menu"}}, Queries: []string{"soup menu", "soup price"}}, use, nil
}
func TestGroundedRouteBypassesTavilyAndNeverRetries(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[failure], func(t *testing.T) {
			s := testutil.Database(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			token, _ := s.NewSession(ctx)
			owner, _ := s.Owner(ctx, token)
			chat, _ := s.CreateConversation(ctx, owner)
			model := &controlledModel{}
			search := &controlledSearch{}
			ground := &controlledGround{fail: failure}
			engine := New(ctx, s, model, search, config.Config{GroundedEnabled: true, RunTimeout: time.Minute, GroundedMaxOutput: 8192})
			engine.Grounded = ground
			run, _, err := s.Accept(ctx, owner, chat.ID, uuid.NewString(), "Find lunch", 0, false, time.Minute, 20, 100, postgres.ModeLimits{Mode: "google_grounded", OwnerQuota: 3, GlobalQuota: 10})
			if err != nil {
				t.Fatal(err)
			}
			engine.Start(owner, run, "Find lunch", false)
			until := time.Now().Add(5 * time.Second)
			for time.Now().Before(until) {
				got, err := s.Run(ctx, owner, run.ID)
				if err != nil {
					t.Fatal(err)
				}
				if got.Status != "running" {
					if ground.calls != 1 || search.searches != 0 || search.fetches != 0 || got.Usage.ModelCalls != map[bool]int{true: 2, false: 3}[failure] || got.Usage.Mode != "google_grounded" {
						t.Fatalf("wrong route or retries: %+v", got)
					}
					if failure {
						if got.Status != "failed" {
							t.Fatal("failure hidden")
						}
					} else if got.Result == nil || got.Result.Grounding == nil || len(got.Result.Confirmed) != 0 {
						t.Fatal("grounded result missing or falsely validated")
					}
					return
				}
				time.Sleep(15 * time.Millisecond)
			}
			t.Fatal("timeout")
		})
	}
}

type failingFormatter struct{ calls int }

func (m *failingFormatter) Name() string { return "failing-formatter" }
func (m *failingFormatter) Generate(context.Context, string, string, any, any) (domain.Usage, error) {
	m.calls++
	return domain.Usage{ModelCalls: 1}, errors.New("malformed structured response")
}
func TestFormatterFailureDoesNotRepeatGroundingOrDiscardAnswer(t *testing.T) {
	s := testutil.Database(t)
	ctx := context.Background()
	token, _ := s.NewSession(ctx)
	owner, _ := s.Owner(ctx, token)
	chat, _ := s.CreateConversation(ctx, owner)
	run, _, err := s.Accept(ctx, owner, chat.ID, uuid.NewString(), "Lunch", 0, false, time.Minute, 20, 100)
	if err != nil {
		t.Fatal(err)
	}
	model := &failingFormatter{}
	ground := &controlledGround{}
	e := New(ctx, s, model, &controlledSearch{}, config.Config{MaxModelCalls: 9})
	e.Grounded = ground
	j := job{ctx: ctx, e: e, run: run}
	g := &domain.Grounding{Text: "Original paid answer", Sources: []domain.GroundingSource{{Number: 1, URL: "https://kitchen.example/menu"}}}
	j.formatGrounded(g)
	if model.calls != 1 || ground.calls != 0 || g.Text != "Original paid answer" || len(g.Sources) != 1 || g.FormattingError == "" {
		t.Fatal("paid result lost or request repeated")
	}
}
