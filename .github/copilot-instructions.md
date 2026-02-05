# Copilot Instructions for CallHttpApi

## Purpose
This repository contains a small Go CLI that reads CSV input, constructs JSON payloads, performs HTTP requests (GET/POST/multipart), and writes results to CSV files.

## Key files
- `main.go`: CLI entrypoint and main logic.
- `config.yml`: configuration (base URL, input/output paths, bearer token).

## Important runtime flags
- `-conf` (default: `config.yml`): config file path
- `-url` (required): API resource path to call (appended to `BaseUrl` from config)
- `-method` (default: `GET`): HTTP method, e.g. `GET`, `POST`
- `-path`: working directory to use instead of config paths (input/output)
- `-boundary`: file name placed into multipart form (used with `POST`)
- `-debug`: enable debug prints
 - `-format` (default: `json`): request/response format, one of `json` or `xml`

Examples:
```powershell
.\call.exe -conf=config.yml -url=/resource -method=GET
.\call.exe -conf=config.yml -url=/upload -method=POST -boundary=myfile.csv
.\call.exe -conf=config.yml -url=/resource -method=POST -path=C:\work\api -debug
```

## Input / Output conventions
- Default input files: `input.csv` and `object.csv` in the configured input path.
- The input directory may also contain XML files. Supported names are `object.xml`, `input.xml` and multiple `input_*.xml` files. When `-format=xml` these XML files are parsed and used the same way CSV inputs are (single object from `object.*`, array from `input.*`, map keys from `input_<key>.*`).
- The CLI also accepts multiple `input_*.csv` files; they will be combined into a JSON object where keys come from the suffix after `input_`.
- Default output filename: `output.csv`. When paginated, the tool writes `output_<page>.csv`.
- Errors and runtime logs are written to `errors.log` in the output path (the app redirects stdout to that file).

## Configuration keys (in `config.yml`)
- `BaseUrl` - base API URL used together with `-url` flag
- `InputPath` - directory containing CSV input files
- `OutputPath` - directory where CSV outputs and `errors.log` are written
- `BearerToken` - optional token set as `Authorization: Bearer <token>` header

## Behavior notes / gotchas
- Content negotiation: JSON requests use `Content-Type: application/json`.
- Multipart uploads: pass `-boundary` with a filename located in the input path; the file is sent as form field `file`.
- Pagination: if the API response `meta.totalPage` is greater than current `meta.page`, the tool automatically fetches subsequent pages and writes `output_<n>.csv` for each page.
- Encoding: the tool converts CSV fields from Windows-1251 to UTF-8 on read and converts response values to Windows-1251 before writing CSV via `golang.org/x/text/encoding/charmap`.
- If debug is enabled, the tool prints request/response bodies to the log.

## Developer workflows
- Build:
```powershell
go build -o call.exe
```
- Run (see examples above).
- Tests: no automated tests included; test manually against a staging API.

## External dependencies
- `golang.org/x/text/encoding` (used for Windows-1251/UTF-8 conversions)

## Tips for Copilot/AI agents
- Look at `main.go` for flag names and flow (prepareBody → doHttpMethod → saveResponse).
- Use `readFileContent` and `prepareBody` logic to understand expected CSV schema and JSON payload shapes.
- Respect `errors.log` as the primary runtime output when reproducing or debugging runs.

If you want, I can also add a short example `config.yml` and a sample `input.csv` to the repo.

## XML handling (current state and how to extend)

- Current state: the project does not include native XML parsing or serialization. All request payloads and responses are handled as JSON in `main.go` (see `getJsonBytes`, `doHttpMethod`, and `DecodeJSON`). CSV input is converted to JSON structures via `readFileContent` and `prepareBody`.

- When XML is required: add an explicit payload/response format switch (example flag `-format=json|xml`). The areas to update in `main.go` are:
	- `prepareBody` — produce XML when `-format=xml` instead of JSON. Convert CSV rows to an XML structure (single root element with child items) and return bytes from a new `getXmlBytes(v any)` helper.
	- `doHttpMethod` — set `Content-Type` to `application/xml` for XML requests; detect response `Content-Type` to choose between JSON and XML parsing.
	- `saveResponse` — parse XML responses into the same intermediate `[]map[string]interface{}` representation (or adapt to an XML-specific struct) before writing CSV.
	- Add parsing helpers: `DecodeXML([]byte) (map[string]interface{}, error)` or use a library that converts XML↔map (e.g., `encoding/xml` for strict structs, or `github.com/clbanning/mxj` / `github.com/clbanning/mxj/v2` for map-based parsing).

- Encoding and edge cases:
	- Keep Windows-1251 ↔ UTF-8 conversions as-is when reading CSV and before writing CSV output.
	- When producing XML, ensure strings are valid for the target encoding; escape XML control characters (`&`, `<`, `>`, `"`, `'`).
	- When parsing XML to maps, numeric strings may need normalization similar to `normalizeNumbers` used for JSON; consider reusing that logic after converting values to strings/numbers.

- Minimal example approach (map-based, simple):
	1. Add flag `-format` defaulting to `json`.
	2. Implement `getXmlBytes(v any)` which marshals either a struct or a map to XML using `encoding/xml` or helper lib.
	3. In `doHttpMethod`, set `Content-Type` appropriately and call `DecodeXML` on responses when `Content-Type` contains `xml` or `-format=xml`.

- Recommendation for Copilot/AI agents:
	- Search for `prepareBody`, `readFileContent`, `doHttpMethod`, `saveResponse`, and `DecodeJSON` in `main.go` to identify integration points.
	- Prefer `encoding/xml` when API schemas are fixed and you can define structs; prefer `mxj`/map-based converters when working with dynamic XML structures.
	- Add unit tests for both JSON and XML flows where possible, and validate encoding conversions (Windows-1251) on round-trip.

If you'd like, I can implement a small `-format` flag and XML helpers and update `main.go` with a minimal working XML flow.