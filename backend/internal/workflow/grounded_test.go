package workflow

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"nebulaiq/internal/config"
	"nebulaiq/internal/domain"
	"nebulaiq/internal/storage/postgres"
	"nebulaiq/internal/testutil"
	"testing"
	"time"
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
					if ground.calls != 1 || search.searches != 0 || search.fetches != 0 || got.Usage.ModelCalls != 2 || got.Usage.Mode != "google_grounded" {
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
