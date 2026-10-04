package llm

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed prompts/*.md
var promptFiles embed.FS

// Prompt loads a static instruction from the embedded prompt resources.
// User and source text are supplied separately as model input, never interpreted as templates.
func Prompt(name string) (string, error) {
	return RenderPrompt(name, nil)
}

// RenderPrompt renders named placeholders and rejects missing template data.
// Templates are parsed on demand to avoid mutable caches and initialization side effects.
func RenderPrompt(name string, data any) (string, error) {
	if strings.ContainsAny(name, "/\\.") || name == "" {
		return "", fmt.Errorf("invalid prompt name %q", name)
	}
	content, err := promptFiles.ReadFile("prompts/" + name + ".md")
	if err != nil {
		return "", fmt.Errorf("read prompt %q: %w", name, err)
	}
	tmpl, err := template.New(name).Option("missingkey=error").Parse(string(content))
	if err != nil {
		return "", fmt.Errorf("parse prompt %q: %w", name, err)
	}
	var result bytes.Buffer
	if err := tmpl.Execute(&result, data); err != nil {
		return "", fmt.Errorf("render prompt %q: %w", name, err)
	}
	return strings.TrimSuffix(result.String(), "\n"), nil
}
