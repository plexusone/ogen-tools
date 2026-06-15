package main

import (
	"strings"
	"testing"
)

func TestFixFormDataEmptyJSON(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantCount      int
		wantContains   []string // Check that output contains these strings
		wantNotContain []string // Check that output does NOT contain these strings
	}{
		{
			name: "fix unconditional encode",
			input: `
	if err := q.EncodeParam(cfg, func(e uri.Encoder) error {
		var enc jx.Encoder
		func(e *jx.Encoder) {
			if request.AdditionalFormats != nil {
				request.AdditionalFormats.Encode(e)
			}
		}(&enc)
		return e.EncodeValue(string(enc.Bytes()))
	}); err != nil {
		return errors.Wrap(err, "encode query")
	}`,
			wantCount: 1,
			wantContains: []string{
				"}(&enc)",
				"if len(enc.Bytes()) > 0 {",
				"return e.EncodeValue(string(enc.Bytes()))",
				"return nil",
			},
		},
		{
			name: "already fixed - idempotent",
			input: `
		}(&enc)
		if len(enc.Bytes()) > 0 {
			return e.EncodeValue(string(enc.Bytes()))
		}
		return nil`,
			wantCount: 0,
			wantContains: []string{
				"if len(enc.Bytes()) > 0 {",
				"return e.EncodeValue(string(enc.Bytes()))",
			},
		},
		{
			name: "multiple encoders",
			input: `
		}(&enc)
		return e.EncodeValue(string(enc.Bytes()))
	// another one
		}(&enc)
		return e.EncodeValue(string(enc.Bytes()))`,
			wantCount: 2,
			wantContains: []string{
				"if len(enc.Bytes()) > 0 {",
			},
		},
		{
			name:      "no match - different pattern",
			input:     `return e.EncodeValue("static string")`,
			wantCount: 0,
			wantNotContain: []string{
				"if len(enc.Bytes()) > 0 {",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixed, count := FixFormDataEmptyJSON([]byte(tt.input))

			if count != tt.wantCount {
				t.Errorf("count = %d, want %d", count, tt.wantCount)
			}

			fixedStr := string(fixed)

			for _, want := range tt.wantContains {
				if !strings.Contains(fixedStr, want) {
					t.Errorf("fixed content should contain %q\ngot:\n%s", want, fixedStr)
				}
			}

			for _, notWant := range tt.wantNotContain {
				if strings.Contains(fixedStr, notWant) {
					t.Errorf("fixed content should NOT contain %q\ngot:\n%s", notWant, fixedStr)
				}
			}

			// Verify idempotency - running again should not change anything
			fixed2, count2 := FixFormDataEmptyJSON(fixed)
			if count2 != 0 {
				t.Errorf("idempotency failed: second run fixed %d more patterns", count2)
			}
			if string(fixed) != string(fixed2) {
				t.Error("idempotency failed: content changed on second run")
			}
		})
	}
}
