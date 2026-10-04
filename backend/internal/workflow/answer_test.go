package workflow

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"nebulaiq/internal/config"
	"nebulaiq/internal/domain"
	"nebulaiq/internal/testutil"
	"strings"
	"testing"
	"time"
)

type answerModel struct {
	controlledModel
	sawEvidence bool
}

func (m *answerModel) Generate(ctx context.Context, instruction, input string, shape, out any) (domain.Usage, error) {
	var data struct {
		NewMessage string              `json:"new_message"`
		Previous   domain.Requirements `json:"previous_requirements"`
		Research   json.RawMessage     `json:"previous_research"`
	}
	json.Unmarshal([]byte(input), &data)
	if data.NewMessage == "Which prices need confirmation?" {
		m.sawEvidence = strings.Contains(string(data.Research), "Vegan chickpeas") && strings.Contains(string(data.Research), "12.00")
		*out.(*domain.Interpretation) = domain.Interpretation{Requirements: data.Previous, Action: "answer_existing", Answer: "Vegan chickpeas were listed at EUR 12.00. Retrieval does not independently prove today's price."}
		return domain.Usage{ModelCalls: 1, UsageKnown: true}, nil
	}
	return m.controlledModel.Generate(ctx, instruction, input, shape, out)
}
func TestEvidenceQuestionPreservesCardsWithoutNewWebCalls(t *testing.T) {
	s := testutil.Database(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	token, _ := s.NewSession(ctx)
	owner, _ := s.Owner(ctx, token)
	chat, _ := s.CreateConversation(ctx, owner)
	model := &answerModel{}
	search := &controlledSearch{}
	engine := New(ctx, s, model, search, config.Config{RunTimeout: 15 * time.Second})
	runOnce := func(version int, text string) domain.Run {
		t.Helper()
		r, _, err := s.Accept(ctx, owner, chat.ID, uuid.NewString(), text, version, false, 15*time.Second, 20, 100)
		if err != nil {
			t.Fatal(err)
		}
		engine.Start(owner, r, text, false)
		end := time.Now().Add(5 * time.Second)
		for time.Now().Before(end) {
			got, err := s.Run(ctx, owner, r.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != "running" {
				return got
			}
			time.Sleep(15 * time.Millisecond)
		}
		t.Fatal("run timed out")
		return domain.Run{}
	}
	first := runOnce(0, "Find lunch")
	if first.Status != "completed" || first.Result == nil || len(first.Result.Confirmed) != 1 {
		t.Fatalf("first run: %+v", first)
	}
	second := runOnce(1, "Which prices need confirmation?")
	if second.Status != "completed" || second.Result == nil || second.Result.Answer == "" || second.Result.Clarification != "" || len(second.Result.Confirmed) != 1 || second.Usage.Searches != 0 || second.Usage.Fetches != 0 || second.Usage.ModelCalls != 1 || !model.sawEvidence {
		t.Fatalf("answer follow-up failed: %+v", second)
	}
	if !second.Result.ResearchedAt.Equal(first.Result.ResearchedAt) || len(second.Result.Sources) != len(first.Result.Sources) {
		t.Fatal("evidence snapshot changed")
	}
	reopened, err := s.Conversation(ctx, owner, chat.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Messages[len(reopened.Messages)-1].Text != second.Result.Answer {
		t.Fatal("explanation not persisted as assistant message")
	}
}
func TestPriorResearchCannotLeakAcrossDestination(t *testing.T) {
	req := domain.Requirements{City: "Barcelona", Country: "Spain", Meal: "lunch"}
	r := domain.Result{Requirements: req, ResearchedAt: time.Now()}
	chat := domain.Conversation{Runs: []domain.Run{{ID: "old", Result: &r}}}
	if priorResearch(chat, "new", domain.Requirements{City: "Tokyo", Country: "Japan", Meal: "dinner"}) != nil {
		t.Fatal("different destination evidence reused")
	}
	if priorResearch(chat, "new", req) == nil {
		t.Fatal("matching research not found")
	}
	if clarification(req, "I have not yet retrieved specific prices.") != "" {
		t.Fatal("statement treated as clarification question")
	}
}

func TestExplanationIncludesHistoricalVariantQuote(t *testing.T) {
	r := domain.Result{Sources: []domain.Document{{ID: "menu", URL: "https://example.com/Menu_2023.pdf", Kind: "menu"}}, Alternatives: []domain.Restaurant{{Dishes: []domain.Dish{{Name: "Koftas", Evidence: domain.Citation{SourceID: "menu", Quote: "Koftas€8 (3 pcs) / €11.50 (5 pcs)"}}}}}}
	context := marshal(researchContext(&r))
	if !strings.Contains(context, "Koftas€8 (3 pcs) / €11.50 (5 pcs)") || !strings.Contains(context, `"price_policy":`) {
		t.Fatal("source-listed variant prices or policy omitted from explanation context")
	}
}
