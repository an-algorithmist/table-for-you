package llm

import (
	"context"
	"errors"
	"google.golang.org/genai"
	"net/url"
	"strings"
	"table-for-you/backend/internal/domain"
	"table-for-you/backend/internal/providers"
)

// Ground uses one normal Gemini request, not the managed Antigravity agent.
// Google controls search fan-out inside this request; query counts are observed metadata.
func (g *Gemini) Ground(ctx context.Context, instruction, input string, maxOutput int) (*domain.Grounding, domain.Usage, error) {
	if err := ReserveCall(ctx); err != nil {
		return nil, domain.Usage{}, err
	}
	use := domain.Usage{Mode: "google_grounded", GroundedModel: g.model, ModelCalls: 1}
	if maxOutput <= 0 {
		maxOutput = 8192
	}
	response, err := g.client.Models.GenerateContent(ctx, g.model, genai.Text(input), &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(instruction, genai.RoleUser),
		Tools:             []*genai.Tool{{GoogleSearch: &genai.GoogleSearch{}}, {URLContext: &genai.URLContext{}}},
		ThinkingConfig:    &genai.ThinkingConfig{ThinkingLevel: genai.ThinkingLevelLow},
		MaxOutputTokens:   int32(min(maxOutput, 8192)), Temperature: genai.Ptr(float32(0.2)),
	})
	if err != nil {
		var api genai.APIError
		if errors.As(err, &api) {
			return nil, use, &providers.Error{Kind: "grounded model", Status: api.Code}
		}
		return nil, use, err
	}
	if response == nil {
		return nil, use, errors.New("google-grounded request returned no response")
	}
	if u := response.UsageMetadata; u != nil {
		use.InputTokens = int64(u.PromptTokenCount)
		use.OutputTokens = int64(u.CandidatesTokenCount + u.ThoughtsTokenCount)
		use.ThinkingTokens = int64(u.ThoughtsTokenCount)
		use.ToolInputTokens = int64(u.ToolUsePromptTokenCount)
		use.UsageKnown = true
	}
	output := &domain.Grounding{Text: strings.TrimSpace(response.Text()), Model: g.model, Sources: []domain.GroundingSource{}, Supports: []domain.GroundingSupport{}, Queries: []string{}, URLs: []domain.URLRetrieval{}}
	if output.Text == "" || len(output.Text) > 128*1024 {
		return nil, use, errors.New("google-grounded request returned no usable bounded answer")
	}
	if len(response.Candidates) == 0 || response.Candidates[0] == nil {
		return nil, use, errors.New("google-grounded response lacks candidate metadata")
	}
	c := response.Candidates[0]
	if m := c.GroundingMetadata; m != nil {
		for i, v := range m.GroundingChunks {
			if v != nil && v.Web != nil && publicSourceURL(v.Web.URI) {
				output.Sources = append(output.Sources, domain.GroundingSource{Number: i + 1, URL: v.Web.URI, Title: v.Web.Title})
			}
		}
		valid := map[int]bool{}
		for _, s := range output.Sources {
			valid[s.Number] = true
		}
		for _, v := range m.GroundingSupports {
			if v == nil || v.Segment == nil || (v.Segment.Text == "" && v.Segment.EndIndex <= v.Segment.StartIndex) {
				continue
			}
			segmentText := v.Segment.Text
			if segmentText == "" && c.Content != nil && int(v.Segment.PartIndex) < len(c.Content.Parts) && v.Segment.PartIndex >= 0 {
				part := c.Content.Parts[v.Segment.PartIndex]
				if part != nil && v.Segment.StartIndex >= 0 && int(v.Segment.EndIndex) <= len(part.Text) {
					segmentText = part.Text[v.Segment.StartIndex:v.Segment.EndIndex]
				}
			}
			refs := []int{}
			for _, i := range v.GroundingChunkIndices {
				if valid[int(i)+1] {
					refs = append(refs, int(i)+1)
				}
			}
			if len(refs) > 0 {
				output.Supports = append(output.Supports, domain.GroundingSupport{Text: segmentText, Sources: refs})
			}
		}
		for _, q := range m.WebSearchQueries {
			if strings.TrimSpace(q) != "" {
				output.Queries = append(output.Queries, q)
			}
		}
		if m.SearchEntryPoint != nil && len(m.SearchEntryPoint.RenderedContent) < 128*1024 {
			output.SearchSuggestions = m.SearchEntryPoint.RenderedContent
		}
	}
	if m := c.URLContextMetadata; m != nil {
		for _, v := range m.URLMetadata {
			if v != nil && publicSourceURL(v.RetrievedURL) {
				output.URLs = append(output.URLs, domain.URLRetrieval{URL: v.RetrievedURL, Status: string(v.URLRetrievalStatus)})
			}
		}
	}
	use.Searches = len(output.Queries)
	use.Fetches = len(output.URLs)
	if len(output.Sources) == 0 {
		return nil, use, errors.New("google returned an answer without usable grounding sources; retry explicitly or use standard research")
	}
	return output, use, nil
}
func publicSourceURL(raw string) bool {
	u, e := url.Parse(raw)
	return e == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && providers.SafeURL(raw)
}
