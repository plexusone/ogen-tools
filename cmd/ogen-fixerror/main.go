// Command ogen-fixerror fixes ogen-generated code to preserve error response bodies.
//
// This tool addresses an issue where ogen's UnexpectedStatusCodeError contains the
// http.Response but the body gets closed by defer before callers can read it.
//
// Usage:
//
//	ogen-fixerror <oas_response_decoders_gen.go>
//
// The tool modifies the file in place, changing:
//
//	return res, validate.UnexpectedStatusCodeWithResponse(resp)
//
// To read and buffer the body before returning:
//
//	body, _ := io.ReadAll(resp.Body)
//	resp.Body = io.NopCloser(bytes.NewReader(body))
//	return res, validate.UnexpectedStatusCodeWithResponse(resp)
//
// This ensures the error body is preserved and can be read by error handlers.
package main

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "ogen-fixerror: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: ogen-fixerror <oas_response_decoders_gen.go>")
	}

	filename := args[0]

	content, err := os.ReadFile(filename) //nolint:gosec // G703: CLI tool, filename from trusted args
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	fixed, count := FixUnexpectedStatusCodeBody(content)

	if count == 0 {
		fmt.Printf("No UnexpectedStatusCode returns needed fixing in %s\n", filename)
		return nil
	}

	if err := os.WriteFile(filename, fixed, 0600); err != nil { //nolint:gosec // G703: CLI tool, filename from trusted args
		return fmt.Errorf("write file: %w", err)
	}

	fmt.Printf("Fixed %d UnexpectedStatusCode returns in %s\n", count, filename)
	return nil
}

// FixUnexpectedStatusCodeBody finds returns of validate.UnexpectedStatusCodeWithResponse
// and adds code to buffer the response body before returning.
func FixUnexpectedStatusCodeBody(content []byte) ([]byte, int) {
	// Check if we need to add imports
	needsImports := !bytes.Contains(content, []byte(`"bytes"`)) ||
		!bytes.Contains(content, []byte(`"io"`))

	// Pattern matches the return statement
	pattern := regexp.MustCompile(
		`(\t*)return res, validate\.UnexpectedStatusCodeWithResponse\(resp\)`)

	count := 0
	fixed := pattern.ReplaceAllFunc(content, func(match []byte) []byte {
		// Check if already fixed (has body buffering before it)
		count++

		// Get the indentation
		submatches := pattern.FindSubmatch(match)
		indent := string(submatches[1])

		// Create the replacement with body buffering
		replacement := fmt.Sprintf(`%s// Buffer the response body so it survives resp.Body.Close()
%sbody, _ := io.ReadAll(resp.Body)
%sresp.Body = io.NopCloser(bytes.NewReader(body))
%sreturn res, validate.UnexpectedStatusCodeWithResponse(resp)`,
			indent, indent, indent, indent)

		return []byte(replacement)
	})

	// Add imports if needed
	if count > 0 && needsImports {
		fixed = addImports(fixed)
	}

	return fixed, count
}

// addImports ensures "bytes" and "io" are in the import block
func addImports(content []byte) []byte {
	// Find the import block
	importPattern := regexp.MustCompile(`(import \(\n)([\s\S]*?)(\n\))`)

	return importPattern.ReplaceAllFunc(content, func(match []byte) []byte {
		submatches := importPattern.FindSubmatch(match)
		if len(submatches) < 4 {
			return match
		}

		imports := string(submatches[2])
		var additions []string

		if !strings.Contains(imports, `"bytes"`) {
			additions = append(additions, `	"bytes"`)
		}
		if !strings.Contains(imports, `"io"`) {
			additions = append(additions, `	"io"`)
		}

		if len(additions) == 0 {
			return match
		}

		// Add new imports after the opening
		var result bytes.Buffer
		result.Write(submatches[1]) // import (\n
		result.WriteString(strings.Join(additions, "\n"))
		result.WriteString("\n")
		result.Write(submatches[2]) // existing imports
		result.Write(submatches[3]) // \n)

		return result.Bytes()
	})
}
