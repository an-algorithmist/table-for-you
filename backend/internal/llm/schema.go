package llm

import (
	"reflect"
	"strings"
)

// Schema derives required JSON fields from domain contracts for structured model output.
func Schema(v any) any { return schemaType(reflect.TypeOf(v)) }
func schemaType(t reflect.Type) any {
	if t.Kind() == reflect.Pointer {
		return schemaType(t.Elem())
	}
	switch t.Kind() {
	case reflect.Struct:
		p := map[string]any{}
		required := []string{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.Anonymous {
				m := schemaType(f.Type).(map[string]any)
				for k, v := range m["properties"].(map[string]any) {
					p[k] = v
					required = append(required, k)
				}
				continue
			}
			name := strings.Split(f.Tag.Get("json"), ",")[0]
			if name == "" || name == "-" {
				continue
			}
			p[name] = schemaType(f.Type)
			required = append(required, name)
		}
		return map[string]any{"type": "object", "properties": p, "required": required}
	case reflect.Slice:
		return map[string]any{"type": "array", "items": schemaType(t.Elem())}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int64:
		return map[string]any{"type": "integer"}
	default:
		return map[string]any{"type": "string"}
	}
}
