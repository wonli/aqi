package mcp

// Schema describes the JSON object accepted by an MCP tool.
//
// It intentionally models the small JSON Schema subset Aqi needs for tool
// inputs instead of exposing map[string]any as public API.
type Schema struct {
	Type                 string            `json:"type"`
	Description          string            `json:"description,omitempty"`
	Properties           map[string]Schema `json:"properties,omitempty"`
	Required             []string          `json:"required,omitempty"`
	Items                *Schema           `json:"items,omitempty"`
	AdditionalProperties *bool             `json:"additionalProperties,omitempty"`
}

func EmptyObjectSchema() Schema {
	allowAdditional := false
	return Schema{
		Type:                 "object",
		Properties:           map[string]Schema{},
		AdditionalProperties: &allowAdditional,
	}
}

func ObjectSchema(properties map[string]Schema, required ...string) Schema {
	s := Schema{
		Type:       "object",
		Properties: properties,
		Required:   required,
	}

	return s
}

func StringSchema(description string) Schema {
	return scalarSchema("string", description)
}

func NumberSchema(description string) Schema {
	return scalarSchema("number", description)
}

func IntegerSchema(description string) Schema {
	return scalarSchema("integer", description)
}

func BooleanSchema(description string) Schema {
	return scalarSchema("boolean", description)
}

func ArraySchema(items Schema, description string) Schema {
	s := Schema{
		Type:  "array",
		Items: &items,
	}
	if description != "" {
		s.Description = description
	}

	return s
}

func scalarSchema(t, description string) Schema {
	return Schema{Type: t, Description: description}
}

func normalizeSchema(schema Schema) Schema {
	if schema.Type == "" {
		return EmptyObjectSchema()
	}

	return schema
}

func validateInputSchema(schema Schema) error {
	schema = normalizeSchema(schema)
	if schema.Type != "object" {
		return ErrInvalidInputSchema
	}

	return nil
}
