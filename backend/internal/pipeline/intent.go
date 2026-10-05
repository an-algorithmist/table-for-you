package pipeline

import (
	"encoding/json"
	"strings"

	"table-for-you/backend/internal/domain"
	"table-for-you/backend/internal/llm"
)

// interpret resolves short replies against stored context before any web research.
func (j *job) interpret(text string) (domain.Interpretation, *domain.Result, error) {
	var parsed domain.Interpretation
	var previous *domain.Result
	input := map[string]any{"previous_requirements": j.run.Requirements, "new_message": text}
	if conversation, err := j.e.Store.Conversation(j.ctx, j.owner, j.run.ConversationID); err == nil {
		messages := conversation.Messages
		if len(messages) > 6 {
			messages = messages[len(messages)-6:]
		}
		input["recent_messages"] = messages
		previous = priorResearch(conversation, j.run.ID, j.run.Requirements)
		if previous != nil {
			input["previous_research"] = researchContext(previous)
		}
	}
	instruction, err := llm.Prompt("intent")
	if err != nil {
		return parsed, previous, err
	}
	if err = j.generate(instruction, marshal(input), domain.Interpretation{}, &parsed); err != nil {
		return parsed, previous, err
	}
	if strings.Contains(strings.ToLower(text), "chicken") && !strings.Contains(strings.ToLower(text), "no chicken") && !strings.Contains(strings.ToLower(text), "without chicken") {
		parsed.Requirements.FoodPreference = "chicken"
	}
	parsed.Requirements = PreserveExclusions(j.run.Requirements, parsed.Requirements, text)
	parsed.Clarification = clarification(parsed.Requirements, parsed.Clarification)
	if err = j.e.Store.Requirements(j.ctx, j.run, parsed.Requirements); err != nil {
		return parsed, previous, err
	}
	j.run.Requirements = parsed.Requirements
	return parsed, previous, nil
}

// existingAnswer clones stored cards and clears attribution offsets that refer to old prose.
func (j *job) existingAnswer(parsed domain.Interpretation, previous *domain.Result) (*domain.Result, error) {
	var reused domain.Result
	if err := json.Unmarshal([]byte(marshal(previous)), &reused); err != nil {
		return nil, err
	}
	reused.Clarification = ""
	reused.Answer = strings.TrimSpace(parsed.Answer)
	if reused.Grounding != nil {
		copyGrounding := *reused.Grounding
		copyGrounding.Text = reused.Answer
		copyGrounding.Supports = nil
		copyGrounding.Queries = nil
		copyGrounding.URLs = nil
		copyGrounding.SearchSuggestions = ""
		reused.Grounding = &copyGrounding
	}
	if len([]rune(reused.Answer)) > 4000 {
		reused.Answer = string([]rune(reused.Answer)[:4000])
	}
	_ = j.event("answer.existing", "Answered from the stored restaurant evidence; no fresh web research was performed.")
	return &reused, nil
}

// PreserveExclusions retains mandatory ingredient exclusions until explicitly removed.
func PreserveExclusions(previous, next domain.Requirements, text string) domain.Requirements {
	if next.Diet == "non-vegetarian" || next.Diet == "no restriction" {
		next.VenueOnly = false
	}
	low := strings.ToLower(text)
	if next.FoodPreference == "" && !strings.Contains(low, "any meat") && !strings.Contains(low, "no food preference") && !strings.Contains(low, "no longer want "+previous.FoodPreference) {
		next.FoodPreference = previous.FoodPreference
	}
	for _, x := range previous.Excluded {
		removed := false
		for _, phrase := range []string{"allow " + x, "remove " + x + " restriction", x + " is okay", x + " is ok", "no longer exclude " + x} {
			removed = removed || strings.Contains(low, phrase)
		}
		if !removed {
			found := false
			for _, y := range next.Excluded {
				found = found || strings.EqualFold(x, y)
			}
			if !found {
				next.Excluded = append(next.Excluded, x)
			}
		}
	}
	if next.Diet == "" {
		next.Diet = previous.Diet
	}
	return next
}
