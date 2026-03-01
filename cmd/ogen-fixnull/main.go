// Command ogen-fixnull fixes ogen-generated code to handle null values in Opt* types.
//
// This tool addresses a known issue (https://github.com/ogen-go/ogen/issues/1358)
// where ogen generates Opt* types instead of OptNil* types for nullable $ref fields,
// causing JSON decoding to fail when the API returns null.
//
// Usage:
//
//	ogen-fixnull <oas_json_gen.go>
//
// The tool modifies the file in place, adding null checks to Opt* Decode methods
// that are missing them.
//
// This tool is designed to be run as a post-processing step after ogen generation.
// It can be integrated into a generate.sh script:
//
//	ogen --package api --target internal/api --clean openapi.json
//	ogen-fixnull internal/api/oas_json_gen.go
package main

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "ogen-fixnull: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: ogen-fixnull <oas_json_gen.go>")
	}

	filename := args[0]

	content, err := os.ReadFile(filename) //nolint:gosec // G703: CLI tool, filename from trusted args
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	fixed, count := FixOptDecodeNullHandling(content)

	if count == 0 {
		fmt.Printf("No Opt* Decode methods needed fixing in %s\n", filename)
		return nil
	}

	if err := os.WriteFile(filename, fixed, 0600); err != nil { //nolint:gosec // G703: CLI tool, filename from trusted args
		return fmt.Errorf("write file: %w", err)
	}

	fmt.Printf("Fixed %d Opt* Decode methods in %s\n", count, filename)
	return nil
}

// FixOptDecodeNullHandling finds Opt* (non-OptNil*) Decode methods that don't
// handle null values and adds the necessary null check.
//
// The pattern it looks for:
//
//	func (o *OptXxx) Decode(d *jx.Decoder) error {
//		if o == nil {
//			return errors.New("invalid: unable to decode OptXxx to nil")
//		}
//		o.Set = true
//		if err := o.Value.Decode(d); err != nil {
//
// And transforms it to:
//
//	func (o *OptXxx) Decode(d *jx.Decoder) error {
//		if o == nil {
//			return errors.New("invalid: unable to decode OptXxx to nil")
//		}
//		if d.Next() == jx.Null {
//			if err := d.Null(); err != nil {
//				return err
//			}
//			return nil
//		}
//		o.Set = true
//		if err := o.Value.Decode(d); err != nil {
func FixOptDecodeNullHandling(content []byte) ([]byte, int) {
	// Pattern matches Opt* Decode methods that are missing null handling.
	// It captures:
	// 1. The type name (e.g., OptManualVerificationResponseModel)
	// 2. Everything up to "o.Set = true"
	//
	// OptNil* types already have null handling and have a different structure
	// (they include o.Null = true), so they won't match this pattern.
	// The pattern requires o.Set = true to immediately follow the nil check,
	// which is only true for Opt* types that need fixing.
	pattern := regexp.MustCompile(
		`(func \(o \*Opt)([A-Z][^\)]*?)(\) Decode\(d \*jx\.Decoder\) error \{\s*` +
			`if o == nil \{\s*` +
			`return errors\.New\("invalid: unable to decode Opt)([^"]+?)( to nil"\)\s*\}\s*)` +
			`(o\.Set = true)`)

	// The null check to insert
	nullCheck := `if d.Next() == jx.Null {
		if err := d.Null(); err != nil {
			return err
		}
		return nil
	}
	`

	count := 0
	fixed := pattern.ReplaceAllFunc(content, func(match []byte) []byte {
		// Check if this match already has null handling (shouldn't match, but be safe)
		if bytes.Contains(match, []byte("d.Next() == jx.Null")) {
			return match
		}

		count++

		// Find the position to insert the null check (after the nil check, before o.Set = true)
		// The pattern captures groups, so we rebuild with the null check inserted
		submatches := pattern.FindSubmatch(match)
		if len(submatches) < 7 {
			return match
		}

		// Rebuild: func (o *Opt + TypeName + ) Decode... + nil check closing + NULL CHECK + o.Set = true
		var result bytes.Buffer
		result.Write(submatches[1]) // func (o *Opt
		result.Write(submatches[2]) // TypeName (without Opt prefix)
		result.Write(submatches[3]) // ) Decode(d *jx.Decoder) error { if o == nil { return errors.New("invalid: unable to decode Opt
		result.Write(submatches[4]) // TypeName again
		result.Write(submatches[5]) //  to nil") } }
		result.WriteString(nullCheck)
		result.Write(submatches[6]) // o.Set = true

		return result.Bytes()
	})

	return fixed, count
}
