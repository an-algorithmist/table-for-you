package llm

import (
	"strings"
	"testing"
)

func TestEmbeddedWorkflowPrompts(t *testing.T) {
	for _, name := range []string{"intent", "discovery", "menu_extract", "grounded", "untrusted", "menu_transcribe"} {
		t.Run(name, func(t *testing.T) {
			text, err := Prompt(name)
			if err != nil || strings.TrimSpace(text) == "" {
				t.Fatalf("embedded prompt unavailable: %v", err)
			}
		})
	}
	for _, name := range []string{"", "../intent", "prompts/intent", "intent.md", "absent"} {
		if _, err := Prompt(name); err == nil {
			t.Fatalf("expected error for prompt %q", name)
		}
	}
}
