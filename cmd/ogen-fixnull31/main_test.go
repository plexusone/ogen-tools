package main

import (
	"reflect"
	"testing"
)

func TestFixNullableTypes_SimpleCase(t *testing.T) {
	input := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"comments": map[string]any{
				"type": []any{"string", "null"},
			},
			"name": map[string]any{
				"type": "string",
			},
		},
	}

	stats := &Stats{}
	result := fixNullableTypes(input, stats)

	if stats.NullableFixed != 1 {
		t.Errorf("Expected 1 fix, got %d", stats.NullableFixed)
	}

	resultMap := result.(map[string]any)
	props := resultMap["properties"].(map[string]any)
	comments := props["comments"].(map[string]any)

	if comments["type"] != "string" {
		t.Errorf("Expected type 'string', got %v", comments["type"])
	}
	if comments["nullable"] != true {
		t.Errorf("Expected nullable: true, got %v", comments["nullable"])
	}

	// Name should be unchanged
	name := props["name"].(map[string]any)
	if name["type"] != "string" {
		t.Errorf("Expected name type 'string', got %v", name["type"])
	}
	if _, hasNullable := name["nullable"]; hasNullable {
		t.Error("Name should not have nullable field")
	}
}

func TestFixNullableTypes_MultipleNonNullTypes(t *testing.T) {
	input := map[string]any{
		"type": []any{"string", "integer", "null"},
	}

	stats := &Stats{}
	result := fixNullableTypes(input, stats)

	if stats.NullableFixed != 1 {
		t.Errorf("Expected 1 fix, got %d", stats.NullableFixed)
	}

	resultMap := result.(map[string]any)
	typeVal := resultMap["type"].([]any)

	if !reflect.DeepEqual(typeVal, []any{"string", "integer"}) {
		t.Errorf("Expected ['string', 'integer'], got %v", typeVal)
	}
	if resultMap["nullable"] != true {
		t.Errorf("Expected nullable: true, got %v", resultMap["nullable"])
	}
}

func TestFixNullableTypes_NoNull(t *testing.T) {
	input := map[string]any{
		"type": []any{"string", "integer"},
	}

	stats := &Stats{}
	result := fixNullableTypes(input, stats)

	if stats.NullableFixed != 0 {
		t.Errorf("Expected 0 fixes, got %d", stats.NullableFixed)
	}

	resultMap := result.(map[string]any)
	if _, hasNullable := resultMap["nullable"]; hasNullable {
		t.Error("Should not add nullable when no null type present")
	}
}

func TestFixNullableTypes_NestedSchemas(t *testing.T) {
	input := map[string]any{
		"components": map[string]any{
			"schemas": map[string]any{
				"User": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"email": map[string]any{
							"type": []any{"string", "null"},
						},
						"address": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"city": map[string]any{
									"type": []any{"string", "null"},
								},
							},
						},
					},
				},
			},
		},
	}

	stats := &Stats{}
	result := fixNullableTypes(input, stats)

	if stats.NullableFixed != 2 {
		t.Errorf("Expected 2 fixes, got %d", stats.NullableFixed)
	}

	// Verify nested structure is correct
	components := result.(map[string]any)["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)
	user := schemas["User"].(map[string]any)
	props := user["properties"].(map[string]any)

	email := props["email"].(map[string]any)
	if email["type"] != "string" || email["nullable"] != true {
		t.Errorf("Email not fixed correctly: %v", email)
	}

	address := props["address"].(map[string]any)
	addressProps := address["properties"].(map[string]any)
	city := addressProps["city"].(map[string]any)
	if city["type"] != "string" || city["nullable"] != true {
		t.Errorf("City not fixed correctly: %v", city)
	}
}

func TestFixNullableTypes_ArrayItems(t *testing.T) {
	input := map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": []any{"string", "null"},
		},
	}

	stats := &Stats{}
	result := fixNullableTypes(input, stats)

	if stats.NullableFixed != 1 {
		t.Errorf("Expected 1 fix, got %d", stats.NullableFixed)
	}

	resultMap := result.(map[string]any)
	items := resultMap["items"].(map[string]any)

	if items["type"] != "string" {
		t.Errorf("Expected items type 'string', got %v", items["type"])
	}
	if items["nullable"] != true {
		t.Errorf("Expected items nullable: true")
	}
}

func TestFixNullableTypes_PreservesOtherFields(t *testing.T) {
	input := map[string]any{
		"type":        []any{"string", "null"},
		"description": "A nullable string field",
		"maxLength":   100,
		"pattern":     "^[a-z]+$",
	}

	stats := &Stats{}
	result := fixNullableTypes(input, stats)

	resultMap := result.(map[string]any)

	if resultMap["type"] != "string" {
		t.Errorf("Expected type 'string', got %v", resultMap["type"])
	}
	if resultMap["nullable"] != true {
		t.Error("Expected nullable: true")
	}
	if resultMap["description"] != "A nullable string field" {
		t.Error("Description not preserved")
	}
	if resultMap["maxLength"] != 100 {
		t.Error("maxLength not preserved")
	}
	if resultMap["pattern"] != "^[a-z]+$" {
		t.Error("pattern not preserved")
	}
}

func TestDowngradeOpenAPIVersion(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		downgrade bool
	}{
		{"3.1.0", "3.1.0", "3.0.3", true},
		{"3.1.1", "3.1.1", "3.0.3", true},
		{"3.0.3", "3.0.3", "3.0.3", false},
		{"3.0.0", "3.0.0", "3.0.0", false},
		{"2.0", "2.0", "2.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := map[string]any{"openapi": tt.input}
			stats := &Stats{}

			result := downgradeOpenAPIVersion(input, stats)
			resultMap := result.(map[string]any)

			if resultMap["openapi"] != tt.expected {
				t.Errorf("Expected openapi '%s', got '%v'", tt.expected, resultMap["openapi"])
			}
			if stats.VersionDowngraded != tt.downgrade {
				t.Errorf("Expected VersionDowngraded=%v, got %v", tt.downgrade, stats.VersionDowngraded)
			}
		})
	}
}

func TestFilterNullType(t *testing.T) {
	tests := []struct {
		name        string
		input       []any
		wantNonNull []any
		wantHasNull bool
	}{
		{
			name:        "string and null",
			input:       []any{"string", "null"},
			wantNonNull: []any{"string"},
			wantHasNull: true,
		},
		{
			name:        "null first",
			input:       []any{"null", "integer"},
			wantNonNull: []any{"integer"},
			wantHasNull: true,
		},
		{
			name:        "no null",
			input:       []any{"string", "integer"},
			wantNonNull: []any{"string", "integer"},
			wantHasNull: false,
		},
		{
			name:        "only null",
			input:       []any{"null"},
			wantNonNull: nil,
			wantHasNull: true,
		},
		{
			name:        "multiple types with null",
			input:       []any{"string", "integer", "null", "boolean"},
			wantNonNull: []any{"string", "integer", "boolean"},
			wantHasNull: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nonNull, hasNull := filterNullType(tt.input)

			if !reflect.DeepEqual(nonNull, tt.wantNonNull) {
				t.Errorf("nonNull = %v, want %v", nonNull, tt.wantNonNull)
			}
			if hasNull != tt.wantHasNull {
				t.Errorf("hasNull = %v, want %v", hasNull, tt.wantHasNull)
			}
		})
	}
}

func TestFixNullableTypes_Idempotent(t *testing.T) {
	// Already in 3.0 format
	input := map[string]any{
		"type":     "string",
		"nullable": true,
	}

	stats := &Stats{}
	result := fixNullableTypes(input, stats)

	if stats.NullableFixed != 0 {
		t.Errorf("Expected 0 fixes on already-fixed content, got %d", stats.NullableFixed)
	}

	resultMap := result.(map[string]any)
	if resultMap["type"] != "string" {
		t.Error("Type changed unexpectedly")
	}
	if resultMap["nullable"] != true {
		t.Error("Nullable changed unexpectedly")
	}
}
