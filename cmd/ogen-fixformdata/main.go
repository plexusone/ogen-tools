// Command ogen-fixformdata fixes ogen-generated code to skip encoding empty JSON arrays/objects in form-data.
//
// This tool addresses an issue where ogen generates form-data encoders that always call
// e.EncodeValue(string(enc.Bytes())) even when the JSON encoder produced no output.
// This results in empty strings being sent for nil slice/map fields, which many APIs reject.
//
// Usage:
//
//	ogen-fixformdata <oas_request_encoders_gen.go>
//
// The tool modifies the file in place, adding a length check before encoding JSON content
// in form-data requests.
//
// This tool is designed to be run as a post-processing step after ogen generation.
// It can be integrated into a generate.sh script:
//
//	ogen --package api --target internal/api --clean openapi.json
//	ogen-fixformdata internal/api/oas_request_encoders_gen.go
package main

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "ogen-fixformdata: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: ogen-fixformdata <oas_request_encoders_gen.go>")
	}

	filename := args[0]

	content, err := os.ReadFile(filename) // #nosec G304 -- CLI tool, filename from trusted args
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	fixed, count := FixFormDataEmptyJSON(content)

	if count == 0 {
		fmt.Printf("No form-data JSON encoders needed fixing in %s\n", filename)
		return nil
	}

	if err := os.WriteFile(filename, fixed, 0600); err != nil { // #nosec G306 -- CLI tool, filename from trusted args
		return fmt.Errorf("write file: %w", err)
	}

	fmt.Printf("Fixed %d form-data JSON encoders in %s\n", count, filename)
	return nil
}

// FixFormDataEmptyJSON finds patterns where ogen unconditionally encodes JSON content
// in form-data fields, even when the JSON encoder produced no output (nil slice/map).
//
// The pattern it looks for:
//
//	if err := q.EncodeParam(cfg, func(e uri.Encoder) error {
//		var enc jx.Encoder
//		func(e *jx.Encoder) {
//			if request.SomeField != nil {
//				request.SomeField.Encode(e)
//			}
//		}(&enc)
//		return e.EncodeValue(string(enc.Bytes()))
//	}); err != nil {
//
// And transforms it to:
//
//	if err := q.EncodeParam(cfg, func(e uri.Encoder) error {
//		var enc jx.Encoder
//		func(e *jx.Encoder) {
//			if request.SomeField != nil {
//				request.SomeField.Encode(e)
//			}
//		}(&enc)
//		if len(enc.Bytes()) > 0 {
//			return e.EncodeValue(string(enc.Bytes()))
//		}
//		return nil
//	}); err != nil {
func FixFormDataEmptyJSON(content []byte) ([]byte, int) {
	// Pattern matches form-data JSON encoders that unconditionally call e.EncodeValue.
	//
	// The pattern looks for:
	// - }(&enc)
	// - return e.EncodeValue(string(enc.Bytes()))
	//
	// And replaces the return statement with a conditional check.
	pattern := regexp.MustCompile(
		`(\}\(&enc\))(\s*)(return e\.EncodeValue\(string\(enc\.Bytes\(\)\)\))`)

	count := 0
	fixed := pattern.ReplaceAllFunc(content, func(match []byte) []byte {
		// Check if this already has the length check (idempotency)
		if bytes.Contains(match, []byte("len(enc.Bytes()) > 0")) {
			return match
		}

		count++

		// Extract the whitespace to preserve indentation
		submatches := pattern.FindSubmatch(match)
		if len(submatches) < 4 {
			return match
		}

		// Build replacement: }(&enc) + whitespace + if len check + return + closing
		var result bytes.Buffer
		result.Write(submatches[1])                          // }(&enc)
		result.Write(submatches[2])                          // whitespace
		result.WriteString("if len(enc.Bytes()) > 0 {\n\t\t\t") // open if
		result.Write(submatches[3])                          // return e.EncodeValue(...)
		result.WriteString("\n\t\t}\n\t\treturn nil")        // close if and return nil

		return result.Bytes()
	})

	return fixed, count
}
