package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/godeps/claude-agent-sdk-go/types"
)

// ResultMeta contains metadata from the query result, separated from the typed output.
type ResultMeta struct {
	SessionID     string
	DurationMs    int
	DurationAPIMs int
	NumTurns      int
	TotalCostUSD  *float64
	IsError       bool
	Usage         map[string]interface{}
}

// QueryTyped executes a query and deserializes the structured output into T.
// T must be a struct type. The function auto-generates a JSON schema from T
// using reflection and sets it as the OutputFormat.
func QueryTyped[T any](ctx context.Context, prompt string, options *types.ClaudeAgentOptions) (T, *ResultMeta, error) {
	var zero T

	if options == nil {
		options = types.NewClaudeAgentOptions()
	}

	schema, err := generateJSONSchema(reflect.TypeOf(zero))
	if err != nil {
		return zero, nil, fmt.Errorf("failed to generate JSON schema for %T: %w", zero, err)
	}

	options.OutputFormat = map[string]interface{}{
		"type":   "json_schema",
		"schema": schema,
	}

	messages, err := Query(ctx, prompt, options)
	if err != nil {
		return zero, nil, err
	}

	var resultMsg *types.ResultMessage
	for msg := range messages {
		if rm, ok := msg.(*types.ResultMessage); ok {
			resultMsg = rm
		}
	}

	if resultMsg == nil {
		return zero, nil, fmt.Errorf("no result message received")
	}

	meta := &ResultMeta{
		SessionID:     resultMsg.SessionID,
		DurationMs:    resultMsg.DurationMs,
		DurationAPIMs: resultMsg.DurationAPIMs,
		NumTurns:      resultMsg.NumTurns,
		TotalCostUSD:  resultMsg.TotalCostUSD,
		IsError:       resultMsg.IsError,
		Usage:         resultMsg.Usage,
	}

	if resultMsg.IsError {
		errText := ""
		if resultMsg.Result != nil {
			errText = *resultMsg.Result
		}
		return zero, meta, fmt.Errorf("query returned error: %s", errText)
	}

	var result T
	if resultMsg.StructuredOutput != nil {
		data, err := json.Marshal(resultMsg.StructuredOutput)
		if err != nil {
			return zero, meta, fmt.Errorf("failed to marshal structured output: %w", err)
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return zero, meta, fmt.Errorf("failed to unmarshal structured output into %T: %w", zero, err)
		}
	} else if resultMsg.Result != nil {
		if err := json.Unmarshal([]byte(*resultMsg.Result), &result); err != nil {
			return zero, meta, fmt.Errorf("failed to unmarshal result string into %T: %w", zero, err)
		}
	} else {
		return zero, meta, fmt.Errorf("no structured output or result in response")
	}

	return result, meta, nil
}

func generateJSONSchema(t reflect.Type) (map[string]interface{}, error) {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("type must be a struct, got %s", t.Kind())
	}

	properties := make(map[string]interface{})
	var required []string

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		jsonTag := field.Tag.Get("json")
		fieldName := field.Name
		omitEmpty := false
		if jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] == "-" {
				continue
			}
			if parts[0] != "" {
				fieldName = parts[0]
			}
			for _, part := range parts[1:] {
				if part == "omitempty" {
					omitEmpty = true
				}
			}
		}

		prop, err := typeToSchema(field.Type)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", field.Name, err)
		}

		if desc := field.Tag.Get("description"); desc != "" {
			prop["description"] = desc
		}

		properties[fieldName] = prop

		if !omitEmpty && field.Type.Kind() != reflect.Ptr {
			required = append(required, fieldName)
		}
	}

	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema, nil
}

func typeToSchema(t reflect.Type) (map[string]interface{}, error) {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.String:
		return map[string]interface{}{"type": "string"}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return map[string]interface{}{"type": "integer"}, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]interface{}{"type": "integer"}, nil
	case reflect.Float32, reflect.Float64:
		return map[string]interface{}{"type": "number"}, nil
	case reflect.Bool:
		return map[string]interface{}{"type": "boolean"}, nil
	case reflect.Slice:
		items, err := typeToSchema(t.Elem())
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"type": "array", "items": items}, nil
	case reflect.Struct:
		return generateJSONSchema(t)
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return map[string]interface{}{"type": "object"}, nil
		}
		valSchema, err := typeToSchema(t.Elem())
		if err != nil {
			return map[string]interface{}{"type": "object"}, nil
		}
		return map[string]interface{}{
			"type":                 "object",
			"additionalProperties": valSchema,
		}, nil
	case reflect.Interface:
		return map[string]interface{}{}, nil
	default:
		return map[string]interface{}{"type": "string"}, nil
	}
}
