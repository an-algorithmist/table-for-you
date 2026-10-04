package providers

import (
	"context"
	"encoding/json"
	"errors"
	"google.golang.org/genai"
	"log/slog"
	"nebulaiq/internal/domain"
	"strings"
	"time"
)

type Gemini struct {
	client *genai.Client
	model  string
}

func NewGemini(ctx context.Context, key, model string) (*Gemini, error) {
	return newGemini(ctx, key, model, 25*time.Second)
}
func NewGroundedGemini(ctx context.Context, key, model string) (*Gemini, error) {
	return newGemini(ctx, key, model, 150*time.Second)
}
func newGemini(ctx context.Context, key, model string, timeout time.Duration) (*Gemini, error) {
	if key == "" {
		return nil, errors.New("GEMINI_API_KEY is not configured")
	}
	httpClient := HTTPClient()
	httpClient.Timeout = timeout
	c, e := genai.NewClient(ctx, &genai.ClientConfig{APIKey: key, Backend: genai.BackendGeminiAPI, HTTPClient: httpClient, HTTPOptions: genai.HTTPOptions{RetryOptions: &genai.HTTPRetryOptions{Attempts: genai.Ptr(int32(1))}}})
	if e != nil {
		return nil, e
	}
	return &Gemini{c, model}, nil
}
func (g *Gemini) Name() string { return "gemini/" + g.model }
func (g *Gemini) Generate(ctx context.Context, instruction, input string, shape, out any) (domain.Usage, error) {
	use := domain.Usage{ModelCalls: 1}
	r, e := g.client.Models.GenerateContent(ctx, g.model, genai.Text(input), &genai.GenerateContentConfig{SystemInstruction: genai.NewContentFromText(instruction, genai.RoleUser), ResponseMIMEType: "application/json", ResponseJsonSchema: Schema(shape), Temperature: genai.Ptr(float32(0)), MaxOutputTokens: 12000})
	if e != nil {
		var api genai.APIError
		if errors.As(e, &api) {
			slog.Warn("Gemini request rejected", "http_status", api.Code, "model", g.model)
			return use, &Error{Kind: "model", Status: api.Code}
		}
		return use, e
	}
	if r.UsageMetadata != nil {
		use.InputTokens = int64(r.UsageMetadata.PromptTokenCount)
		use.OutputTokens = int64(r.UsageMetadata.CandidatesTokenCount + r.UsageMetadata.ThoughtsTokenCount)
		use.UsageKnown = true
	}
	text := strings.TrimSpace(r.Text())
	if text == "" {
		return use, errors.New("model returned no usable output")
	}
	if len(text) > 256*1024 {
		return use, errors.New("model output exceeds size limit")
	}
	if e = json.Unmarshal([]byte(text), out); e != nil {
		return use, errors.New("model output was not valid structured JSON")
	}
	return use, nil
}
