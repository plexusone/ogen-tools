# ogen tools

[![Go CI][go-ci-svg]][go-ci-url]
[![Go Lint][go-lint-svg]][go-lint-url]
[![Go SAST][go-sast-svg]][go-sast-url]
[![Go Report Card][goreport-svg]][goreport-url]
[![Docs][docs-godoc-svg]][docs-godoc-url]
[![Visualization][viz-svg]][viz-url]
[![License][license-svg]][license-url]

 [go-ci-svg]: https://github.com/plexusone/ogen-tools/actions/workflows/go-ci.yaml/badge.svg?branch=main
 [go-ci-url]: https://github.com/plexusone/ogen-tools/actions/workflows/go-ci.yaml
 [go-lint-svg]: https://github.com/plexusone/ogen-tools/actions/workflows/go-lint.yaml/badge.svg?branch=main
 [go-lint-url]: https://github.com/plexusone/ogen-tools/actions/workflows/go-lint.yaml
 [go-sast-svg]: https://github.com/plexusone/ogen-tools/actions/workflows/go-sast-codeql.yaml/badge.svg?branch=main
 [go-sast-url]: https://github.com/plexusone/ogen-tools/actions/workflows/go-sast-codeql.yaml
 [goreport-svg]: https://goreportcard.com/badge/github.com/plexusone/ogen-tools
 [goreport-url]: https://goreportcard.com/report/github.com/plexusone/ogen-tools
 [docs-godoc-svg]: https://pkg.go.dev/badge/github.com/plexusone/ogen-tools
 [docs-godoc-url]: https://pkg.go.dev/github.com/plexusone/ogen-tools
 [docs-mkdoc-svg]: https://img.shields.io/badge/Go-dev%20guide-blue.svg
 [docs-mkdoc-url]: https://plexusone.dev/ogen-tools
 [viz-svg]: https://img.shields.io/badge/Go-visualizaton-blue.svg
 [viz-url]: https://mango-dune-07a8b7110.1.azurestaticapps.net/?repo=plexusone%2Fogen-tools
 [loc-svg]: https://tokei.rs/b1/github/plexusone/ogen-tools
 [repo-url]: https://github.com/plexusone/ogen-tools
 [license-svg]: https://img.shields.io/badge/license-MIT-blue.svg
 [license-url]: https://github.com/plexusone/ogen-tools/blob/main/LICENSE

A collection of tools to enable [ogen](https://github.com/ogen-go/ogen) to accommodate some specific spec features.

This repo provides post-processing tools to work around known issues until they're fixed upstream. These tools are designed to be able to be able to run on all code without side effects.

## Tools

| Tool | Description | Issue |
|------|-------------|-------|
| [ogen-fixnull](cmd/ogen-fixnull/) | Fix null handling in `Opt*` types | [#1358](https://github.com/ogen-go/ogen/issues/1358) |
| [ogen-fixerror](cmd/ogen-fixerror/) | Preserve error response bodies | - |
| [ogen-fixformdata](cmd/ogen-fixformdata/) | Fix form-data encoding of empty JSON arrays/objects | - |
| [ogen-fixbinary](cmd/ogen-fixbinary/) | Fix nullable binary file fields in multipart forms | - |
| [ogen-fixnull31](cmd/ogen-fixnull31/) | Preprocess OpenAPI 3.1 nullable type arrays for ogen | [#880](https://github.com/ogen-go/ogen/issues/880) |

## Packages

| Package | Description |
|---------|-------------|
| [ogenerror](ogenerror/) | Extract status code and body from ogen errors |

## Quick Start

### ogen-fixnull

Fixes JSON decoding errors when APIs return `null` for nullable `$ref` fields.

**Install:**
```bash
go install github.com/plexusone/ogen-tools/cmd/ogen-fixnull@latest
```

**Use:**
```bash
ogen --package api --target internal/api --clean openapi.json
ogen-fixnull internal/api/oas_json_gen.go
```

**Or without installing:**
```bash
go run github.com/plexusone/ogen-tools/cmd/ogen-fixnull@latest internal/api/oas_json_gen.go
```

See [cmd/ogen-fixnull/README.md](cmd/ogen-fixnull/README.md) for detailed documentation.

### ogen-fixerror

Preserves error response bodies so they can be read after the response is closed.

**Problem:** ogen's `UnexpectedStatusCodeError` contains the `*http.Response`, but the body gets closed by `defer resp.Body.Close()` before callers can read it.

**Use:**
```bash
ogen --package api --target internal/api --clean openapi.json
ogen-fixerror internal/api/oas_response_decoders_gen.go
```

**Or without installing:**
```bash
go run github.com/plexusone/ogen-tools/cmd/ogen-fixerror@latest internal/api/oas_response_decoders_gen.go
```

### ogen-fixformdata

Fixes form-data encoding that sends empty strings for nil array/object fields.

**Problem:** ogen's form-data encoders unconditionally encode JSON fields even when they're nil, sending empty strings that cause API 400 errors like "Invalid additional format options".

**Use:**
```bash
ogen --package api --target internal/api --clean openapi.json
ogen-fixformdata internal/api/oas_request_encoders_gen.go
```

**Or without installing:**
```bash
go run github.com/plexusone/ogen-tools/cmd/ogen-fixformdata@latest internal/api/oas_request_encoders_gen.go
```

### ogen-fixbinary

Fixes nullable binary file fields in multipart form encoders.

**Problem:** When an OpenAPI spec has a binary field with `nullable: true`, ogen generates `OptNilString` instead of `OptMultipartFile`, causing file uploads to be encoded as string fields.

**Use:**
```bash
ogen --package api --target internal/api --clean openapi.json
ogen-fixbinary internal/api/oas_schemas_gen.go internal/api/oas_request_encoders_gen.go
```

**Or without installing:**
```bash
go run github.com/plexusone/ogen-tools/cmd/ogen-fixbinary@latest internal/api/oas_schemas_gen.go internal/api/oas_request_encoders_gen.go
```

### ogen-fixnull31

Preprocesses OpenAPI 3.1 specs for ogen compatibility.

**Problem:** ogen cannot parse OpenAPI 3.1 specs that use the nullable type array syntax (`type: [string, "null"]`). This tool converts them to OpenAPI 3.0 format (`type: string` + `nullable: true`).

**Use:**
```bash
# Preprocess spec before running ogen
ogen-fixnull31 openapi.yaml -o openapi-fixed.yaml
ogen --package api --target internal/api --clean openapi-fixed.yaml
```

**Or without installing:**
```bash
go run github.com/plexusone/ogen-tools/cmd/ogen-fixnull31@latest openapi.yaml -o openapi-fixed.yaml
```

### ogenerror

Extract error details from ogen client errors:

```go
import "github.com/plexusone/ogen-tools/ogenerror"

resp, err := client.SomeMethod(ctx, req)
if err != nil {
    if status := ogenerror.Parse(err); status != nil {
        fmt.Printf("Status: %d, Body: %s\n", status.StatusCode, status.Body)
    }
}
```

See [ogenerror/README.md](ogenerror/README.md) for detailed documentation.

## Typical generate.sh

```bash
#!/bin/bash
set -e

# Prerequisites:
#   go install github.com/ogen-go/ogen/cmd/ogen@latest

# Preprocess OpenAPI 3.1 spec (if needed)
go run github.com/plexusone/ogen-tools/cmd/ogen-fixnull31@latest openapi.yaml -o openapi-ogen.yaml

# Generate API code
ogen --package api --target internal/api --clean openapi-ogen.yaml

# Post-process: Fix ogen bugs
go run github.com/plexusone/ogen-tools/cmd/ogen-fixnull@latest internal/api/oas_json_gen.go
go run github.com/plexusone/ogen-tools/cmd/ogen-fixerror@latest internal/api/oas_response_decoders_gen.go
go run github.com/plexusone/ogen-tools/cmd/ogen-fixformdata@latest internal/api/oas_request_encoders_gen.go
go run github.com/plexusone/ogen-tools/cmd/ogen-fixbinary@latest internal/api/oas_schemas_gen.go internal/api/oas_request_encoders_gen.go

# Verify
go build ./...
```

## Contributing

Found another ogen issue that needs a workaround? PRs welcome.

## License

MIT
