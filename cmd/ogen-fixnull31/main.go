// Command ogen-fixnull31 preprocesses OpenAPI 3.1 specs for ogen compatibility.
//
// ogen cannot parse OpenAPI 3.1 specs that use the nullable type array syntax:
//
//	type:
//	  - string
//	  - "null"
//
// This tool converts such specs to OpenAPI 3.0 format:
//
//	type: string
//	nullable: true
//
// Usage:
//
//	ogen-fixnull31 openapi.yaml                    # writes to stdout
//	ogen-fixnull31 openapi.yaml -o openapi-fixed.yaml
//
// This addresses ogen issue: https://github.com/ogen-go/ogen/issues/880
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "ogen-fixnull31: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("ogen-fixnull31", flag.ExitOnError)
	output := fs.String("o", "", "output file (default: stdout)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		return fmt.Errorf("usage: ogen-fixnull31 <openapi.yaml|openapi.json> [-o output]")
	}

	inputFile := fs.Arg(0)

	content, err := os.ReadFile(inputFile) // #nosec G304 -- CLI tool, filename from args
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(inputFile))
	isJSON := ext == ".json"

	var doc any
	if isJSON {
		if err := json.Unmarshal(content, &doc); err != nil {
			return fmt.Errorf("parse JSON: %w", err)
		}
	} else {
		if err := yaml.Unmarshal(content, &doc); err != nil {
			return fmt.Errorf("parse YAML: %w", err)
		}
	}

	stats := &Stats{}
	doc = fixNullableTypes(doc, stats)
	doc = downgradeOpenAPIVersion(doc, stats)

	var out []byte
	if isJSON {
		out, err = json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal JSON: %w", err)
		}
		out = append(out, '\n')
	} else {
		out, err = yaml.Marshal(doc)
		if err != nil {
			return fmt.Errorf("marshal YAML: %w", err)
		}
	}

	var w io.Writer = os.Stdout
	if *output != "" {
		f, err := os.Create(*output) // #nosec G304 -- CLI tool, filename from args
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}
		defer f.Close()
		w = f
	}

	if _, err := w.Write(out); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	// Print stats to stderr
	fmt.Fprintf(os.Stderr, "Fixed %d nullable type array(s)", stats.NullableFixed)
	if stats.VersionDowngraded {
		fmt.Fprintf(os.Stderr, ", downgraded openapi version to 3.0.3")
	}
	fmt.Fprintln(os.Stderr)

	return nil
}

// Stats tracks transformation statistics.
type Stats struct {
	NullableFixed     int
	VersionDowngraded bool
}

// fixNullableTypes recursively transforms type arrays with "null" to nullable: true.
func fixNullableTypes(v any, stats *Stats) any {
	switch val := v.(type) {
	case map[string]any:
		return fixMap(val, stats)
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = fixNullableTypes(item, stats)
		}
		return result
	default:
		return v
	}
}

func fixMap(m map[string]any, stats *Stats) map[string]any {
	result := make(map[string]any, len(m))

	// First, check if this map has a type array that needs fixing
	if typeVal, hasType := m["type"]; hasType {
		if typeArr, isArray := typeVal.([]any); isArray {
			nonNullTypes, hasNull := filterNullType(typeArr)
			if hasNull {
				stats.NullableFixed++
				if len(nonNullTypes) == 1 {
					// Simple case: [T, "null"] -> type: T, nullable: true
					result["type"] = nonNullTypes[0]
					result["nullable"] = true
				} else if len(nonNullTypes) > 1 {
					// Multiple non-null types: keep as array but add nullable
					// This is a best-effort approach; ogen may still have issues
					result["type"] = nonNullTypes
					result["nullable"] = true
				}
				// Copy remaining keys
				for k, v := range m {
					if k != "type" {
						result[k] = fixNullableTypes(v, stats)
					}
				}
				return result
			}
		}
	}

	// No type array fix needed, recurse into all values
	for k, v := range m {
		result[k] = fixNullableTypes(v, stats)
	}
	return result
}

// filterNullType separates non-null types from "null" in a type array.
func filterNullType(types []any) (nonNull []any, hasNull bool) {
	for _, t := range types {
		if s, ok := t.(string); ok && s == "null" {
			hasNull = true
		} else {
			nonNull = append(nonNull, t)
		}
	}
	return nonNull, hasNull
}

// downgradeOpenAPIVersion changes openapi: 3.1.x to openapi: 3.0.3.
func downgradeOpenAPIVersion(v any, stats *Stats) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}

	if version, hasVersion := m["openapi"]; hasVersion {
		if vStr, isString := version.(string); isString {
			if strings.HasPrefix(vStr, "3.1") {
				m["openapi"] = "3.0.3"
				stats.VersionDowngraded = true
			}
		}
	}

	return m
}
