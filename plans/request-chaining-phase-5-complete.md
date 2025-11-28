## Phase 5 Complete: Implement Response Extraction in HTTP Requester

Implemented response extraction in the HTTP requester to extract values from responses using JSON path and regex patterns, storing them in the variable store for use in subsequent requests.

**Files created/changed:**
- cmd/crankfire/main.go
- cmd/crankfire/endpoints.go
- cmd/crankfire/main_test.go

**Functions created/changed:**
- `extractors` field added to `endpointTemplate` struct
- `toExtractorList()` function to convert config.Extractor to extractor.Extractor
- `stderrLogger.Warn()` method implementing extractor.Logger interface
- `filterExtractorsForError()` helper to filter extractors by OnError flag
- `storeExtractedValues()` helper to store values in variable store
- Updated `httpRequester.Do()` with extraction logic after response
- Updated `buildEndpointTemplate()` to populate extractors

**Tests created/changed:**
- TestHTTPRequester_ExtractsJSONPath
- TestHTTPRequester_ExtractsRegex
- TestHTTPRequester_ExtractorChaining
- TestHTTPRequester_ExtractorNoMatch_Continues
- TestHTTPRequester_ExtractOnError_WhenEnabled
- TestHTTPRequester_NoExtractOnError_WhenDisabled

**Review Status:** APPROVED

**Git Commit Message:**
```
feat: implement response extraction in HTTP requester

- Add extractors field to endpointTemplate
- Read response body and apply JSON path/regex extractors
- Store extracted values in per-worker variable store
- Filter extractors by OnError flag for error responses
- Log extraction warnings without failing requests
- Add comprehensive extraction tests
```
