package llm

import (
	"context"
	"errors"
	"fmt"
	"google.golang.org/genai"
	"os"
	"strings"
	"table-for-you/backend/internal/config"
	"table-for-you/backend/internal/domain"
	"time"
)

// Diagnose performs an explicit, potentially billable provider check. Never run from tests.
func Diagnose(args []string) {
	_ = config.LoadEnv(".env")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	key := os.Getenv("GEMINI_API_KEY")
	client, e := genai.NewClient(ctx, &genai.ClientConfig{APIKey: key, Backend: genai.BackendGeminiAPI})
	if e != nil {
		fmt.Println("Gemini client initialization failed")
		return
	}
	if len(args) > 1 && args[1] == "models" {
		for model, err := range client.Models.All(ctx) {
			if err != nil {
				fmt.Println("Model listing failed")
				return
			}
			if strings.Contains(model.Name, "flash") {
				fmt.Println(model.Name)
			}
		}
		return
	}
	model := os.Getenv("MODEL_NAME")
	if model == "" {
		model = "gemini-3.1-flash-lite"
	}
	if len(args) > 1 && args[1] == "interpret" {
		_, e = client.Models.GenerateContent(ctx, model, genai.Text("Interpret: vegetarian lunch in Barcelona, Spain, EUR 25 per dish. Return complete requirements, empty clarification and a search query."), &genai.GenerateContentConfig{ResponseMIMEType: "application/json", ResponseJsonSchema: Schema(domain.Interpretation{}), MaxOutputTokens: 3000})
	} else {
		_, e = client.Models.GenerateContent(ctx, model, genai.Text("Return the JSON object {\"ok\":true}."), &genai.GenerateContentConfig{ResponseMIMEType: "application/json", ResponseJsonSchema: map[string]any{"type": "object", "properties": map[string]any{"ok": map[string]any{"type": "boolean"}}, "required": []string{"ok"}}, MaxOutputTokens: 100})
	}
	if e == nil {
		fmt.Println("Gemini basic structured-output access: OK; model", model)
		return
	}
	var api genai.APIError
	if errors.As(e, &api) {
		message := strings.ReplaceAll(api.Message, key, "[REDACTED]")
		if len(message) > 1000 {
			message = message[:1000]
		}
		fmt.Printf("Gemini diagnostic HTTP %d: %s\n", api.Code, message)
	} else {
		fmt.Println("Gemini diagnostic transport/timeout failure")
	}
}
