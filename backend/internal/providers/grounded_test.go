package providers

import (
	"context"
	"encoding/json"
	"google.golang.org/genai"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGroundingToolsMetadataAndNoRetry(t *testing.T) {
	requests := 0
	fail := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if fail {
			w.WriteHeader(503)
			w.Write([]byte(`{"error":{"code":503,"message":"busy"}}`))
			return
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		tools, ok := body["tools"].([]any)
		if !ok || len(tools) != 2 {
			t.Error("search and URL context tools missing")
		}
		config := body["generationConfig"].(map[string]any)
		if config["maxOutputTokens"] != float64(8192) {
			t.Error("output cap not applied")
		}
		if config["responseMimeType"] != nil {
			t.Error("grounded narrative forced into extraction schema")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"candidates":[{"content":{"role":"model","parts":[{"text":"Soup costs EUR 12."}]},"groundingMetadata":{"webSearchQueries":["Kitchen soup price"],"groundingChunks":[{"web":{"uri":"https://kitchen.example/menu","title":"Kitchen menu"}},{"web":{"uri":"javascript:alert(1)","title":"unsafe"}}],"groundingSupports":[{"segment":{"text":"Soup costs EUR 12."},"groundingChunkIndices":[0,1]}],"searchEntryPoint":{"renderedContent":"<div>Google suggestions</div>"}},"urlContextMetadata":{"urlMetadata":[{"retrievedUrl":"https://kitchen.example/menu","urlRetrievalStatus":"URL_RETRIEVAL_STATUS_SUCCESS"}]}}],"usageMetadata":{"promptTokenCount":200,"candidatesTokenCount":50,"thoughtsTokenCount":30,"toolUsePromptTokenCount":100}}`))
	}))
	defer server.Close()
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{APIKey: "test", HTTPClient: server.Client(), HTTPOptions: genai.HTTPOptions{BaseURL: server.URL, APIVersion: "v1beta", RetryOptions: &genai.HTTPRetryOptions{Attempts: genai.Ptr(int32(1))}}})
	if err != nil {
		t.Fatal(err)
	}
	g := &Gemini{client: client, model: "gemini-3.8-flash"}
	answer, use, err := g.Ground(context.Background(), "research", "Tokyo", 20000)
	if err != nil {
		t.Fatal(err)
	}
	if len(answer.Sources) != 1 || len(answer.Supports) != 1 || len(answer.Supports[0].Sources) != 1 || use.Searches != 1 || use.Fetches != 1 || use.OutputTokens != 80 || use.ThinkingTokens != 30 || use.ToolInputTokens != 100 {
		t.Fatalf("metadata or usage lost: %+v %+v", answer, use)
	}
	fail = true
	before := requests
	_, _, err = g.Ground(context.Background(), "research", "Tokyo", 8192)
	if err == nil || requests != before+1 {
		t.Fatal("failure retried or hidden")
	}
}
