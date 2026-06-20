// ogen-fixbinary fixes nullable binary file fields in ogen-generated multipart form encoders.
//
// Problem: When OpenAPI spec has a binary field with nullable: true, ogen generates
// OptNilString instead of OptMultipartFile, and encodes it as a string form field
// instead of a proper multipart file attachment.
//
// This tool fixes:
//   - oas_schemas_gen.go: Changes File OptNilString to File OptMultipartFile
//   - oas_request_encoders_gen.go: Changes encoding from string field to multipart file
//
// Usage:
//
//	ogen-fixbinary oas_schemas_gen.go oas_request_encoders_gen.go
package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <oas_schemas_gen.go> <oas_request_encoders_gen.go>\n", os.Args[0])
		os.Exit(1)
	}

	schemasFile := os.Args[1]
	encodersFile := os.Args[2]

	// Fix schemas
	schemasContent, err := os.ReadFile(schemasFile) //nolint:gosec // CLI tool, filename from args
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", schemasFile, err)
		os.Exit(1)
	}

	fixedSchemas, schemasCount := FixSchemasBinaryFields(schemasContent)
	if schemasCount > 0 {
		if err := os.WriteFile(schemasFile, fixedSchemas, 0600); err != nil { //nolint:gosec // CLI tool
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", schemasFile, err)
			os.Exit(1)
		}
		fmt.Printf("Fixed %d binary field type(s) in %s\n", schemasCount, schemasFile)
	} else {
		fmt.Printf("No binary field types to fix in %s\n", schemasFile)
	}

	// Fix encoders
	encodersContent, err := os.ReadFile(encodersFile) //nolint:gosec // CLI tool, filename from args
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", encodersFile, err)
		os.Exit(1)
	}

	fixedEncoders, encodersCount := FixEncodersBinaryFields(encodersContent)
	if encodersCount > 0 {
		if err := os.WriteFile(encodersFile, fixedEncoders, 0600); err != nil { //nolint:gosec // CLI tool
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", encodersFile, err)
			os.Exit(1)
		}
		fmt.Printf("Fixed %d binary field encoder(s) in %s\n", encodersCount, encodersFile)
	} else {
		fmt.Printf("No binary field encoders to fix in %s\n", encodersFile)
	}
}

// FixSchemasBinaryFields changes File OptNilString to File OptMultipartFile
// in multipart request body structs.
func FixSchemasBinaryFields(content []byte) ([]byte, int) {
	// We need to find structs that:
	// 1. Have name ending in "Multipart" (e.g., BodySomethingMultipart)
	// 2. Have a File field with OptNilString type

	lines := bytes.Split(content, []byte("\n"))
	count := 0
	inMultipartStruct := false
	var result [][]byte

	for _, line := range lines {
		// Check if entering a Multipart struct
		// Pattern: "type Body...Multipart struct {"
		// The struct name must end with "Multipart" followed by " struct {"
		if bytes.Contains(line, []byte("type Body")) &&
			bytes.Contains(line, []byte("Multipart struct {")) {
			// This is a multipart struct - any Body struct with "Multipart struct {" pattern
			// covers both "BodyXMultipart struct {" and "BodyXYZMultipart struct {"
			inMultipartStruct = true
		}

		// Check if exiting struct
		if inMultipartStruct && bytes.Equal(bytes.TrimSpace(line), []byte("}")) {
			inMultipartStruct = false
		}

		// Fix File OptNilString to File OptMultipartFile in Multipart structs
		if inMultipartStruct &&
			bytes.Contains(line, []byte("File")) &&
			bytes.Contains(line, []byte("OptNilString")) &&
			bytes.Contains(line, []byte(`json:"file"`)) {
			line = bytes.Replace(line, []byte("OptNilString"), []byte("OptMultipartFile"), 1)
			count++
		}

		result = append(result, line)
	}

	return bytes.Join(result, []byte("\n")), count
}

// FixEncodersBinaryFields transforms string-based file encoding to proper multipart file encoding.
func FixEncodersBinaryFields(content []byte) ([]byte, int) {
	count := 0
	result := string(content)

	// Pattern to identify the string-based file encoding block
	// This uses a simpler approach - find the comment and work from there
	fileEncodeMarker := `// Encode "file" form field.`

	for {
		markerPos := strings.Index(result, fileEncodeMarker)
		if markerPos == -1 {
			break
		}

		// Find the start of this block - look for opening brace on line before comment
		// Go backwards to find the start of the line with just "{"
		blockStart := markerPos
		for blockStart > 0 && result[blockStart-1] != '\n' {
			blockStart--
		}
		// Now go back one more line to find the { line
		if blockStart > 0 {
			blockStart-- // Skip the newline
			for blockStart > 0 && result[blockStart-1] != '\n' {
				blockStart--
			}
		}

		// Verify we found a line that's just whitespace + {
		lineEnd := strings.Index(result[blockStart:], "\n")
		if lineEnd == -1 {
			break
		}
		openBraceLine := strings.TrimSpace(result[blockStart : blockStart+lineEnd])
		if openBraceLine != "{" {
			// Not the pattern we expect, skip
			result = result[:markerPos] + "// FIXED " + result[markerPos+3:]
			continue
		}

		// Find the end of this block - count braces
		braceCount := 1
		blockEnd := -1

		for i := blockStart + lineEnd + 1; i < len(result); i++ {
			if result[i] == '{' {
				braceCount++
			} else if result[i] == '}' {
				braceCount--
				if braceCount == 0 {
					// Find the end of this line
					lineEnd := strings.Index(result[i:], "\n")
					if lineEnd != -1 {
						blockEnd = i + lineEnd + 1
					} else {
						blockEnd = i + 1
					}
					break
				}
			}
		}

		if blockEnd == -1 {
			break
		}

		// Check if this is in a function with CreateMultipartBody
		funcStart := strings.LastIndex(result[:blockStart], "\nfunc encode")
		if funcStart == -1 {
			funcStart = strings.LastIndex(result[:blockStart], "func encode")
			if funcStart == -1 {
				// Can't find function, skip this one
				result = result[:markerPos] + "// FIXED " + result[markerPos+3:]
				continue
			}
		}

		// Find end of function
		funcEnd := strings.Index(result[blockEnd:], "\nfunc ")
		if funcEnd == -1 {
			funcEnd = len(result)
		} else {
			funcEnd += blockEnd
		}
		funcBody := result[funcStart:funcEnd]

		if !strings.Contains(funcBody, "ht.CreateMultipartBody") {
			// Not a multipart function, skip
			result = result[:markerPos] + "// FIXED " + result[markerPos+3:]
			continue
		}

		// Check if conv.StringToString is used (confirms this is the wrong encoding)
		blockContent := result[blockStart:blockEnd]
		if !strings.Contains(blockContent, "conv.StringToString") {
			// Already fixed or different pattern
			result = result[:markerPos] + "// FIXED " + result[markerPos+3:]
			continue
		}

		// Remove the string-based file encoding block
		result = result[:blockStart] + result[blockEnd:]

		// Now add the WriteMultipart call inside CreateMultipartBody
		// Re-find the function boundaries after modification
		funcEnd = strings.Index(result[funcStart:], "\nfunc ")
		if funcEnd == -1 {
			funcEnd = len(result)
		} else {
			funcEnd += funcStart
		}
		funcBody = result[funcStart:funcEnd]

		// Find where to insert the WriteMultipart call
		insertPattern := "if err := q.WriteMultipart(w); err != nil {"
		insertPos := strings.Index(funcBody, insertPattern)
		if insertPos == -1 {
			count++
			continue
		}

		// Calculate absolute position
		absInsertPos := funcStart + insertPos

		// Insert the file WriteMultipart code before q.WriteMultipart
		writeFileCode := `if val, ok := request.File.Get(); ok {
			if err := val.WriteMultipart("file", w); err != nil {
				return errors.Wrap(err, "write \"file\"")
			}
		}
		`

		result = result[:absInsertPos] + writeFileCode + result[absInsertPos:]
		count++
	}

	// Clean up any FIXED markers we added
	result = strings.ReplaceAll(result, "// FIXED Encode", "// Encode")

	return []byte(result), count
}
