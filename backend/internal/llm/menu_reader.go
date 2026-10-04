package llm

import (
	"context"
	"encoding/json"
	"errors"
	"google.golang.org/genai"
	"table-for-you/backend/internal/domain"
	"table-for-you/backend/internal/providers"
	"table-for-you/backend/internal/tools"
)

// ReadMenu transcribes original menu layout without inferring prices or ingredients.
func (g *Gemini) ReadMenu(ctx context.Context, raw string) (string, domain.Usage, error) {
	use := domain.Usage{}
	data, mime, err := tools.FetchMenu(ctx, raw)
	if err != nil {
		return "", use, err
	}
	shape := struct {
		Text string `json:"text"`
	}{}
	instruction, err := Prompt("menu_transcribe")
	if err != nil {
		return "", use, err
	}

	if err := ReserveCall(ctx); err != nil {
		return "", use, err
	}
	use.ModelCalls = 1
	response, err := g.client.Models.GenerateContent(ctx, g.model, []*genai.Content{{Role: "user", Parts: []*genai.Part{{Text: instruction}, {InlineData: &genai.Blob{MIMEType: mime, Data: data}}}}}, &genai.GenerateContentConfig{ResponseMIMEType: "application/json", ResponseJsonSchema: Schema(shape), Temperature: genai.Ptr(float32(0)), MaxOutputTokens: 12000})
	if err != nil {
		var api genai.APIError
		if errors.As(err, &api) {
			return "", use, &providers.Error{Kind: "model", Status: api.Code}
		}
		return "", use, err
	}
	if response.UsageMetadata != nil {
		use.InputTokens = int64(response.UsageMetadata.PromptTokenCount)
		use.OutputTokens = int64(response.UsageMetadata.CandidatesTokenCount + response.UsageMetadata.ThoughtsTokenCount)
		use.UsageKnown = true
	}
	if err = json.Unmarshal([]byte(response.Text()), &shape); err != nil || len(shape.Text) > 60000 || len(shape.Text) < 10 {
		return "", use, errors.New("invalid menu transcription")
	}
	return shape.Text, use, nil
}
