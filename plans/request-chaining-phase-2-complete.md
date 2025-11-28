## Phase 2 Complete: Create Extractor Package

Created extraction logic for JSON path and regex patterns to extract values from HTTP response bodies. Uses the gjson library for efficient JSON path extraction.

**Files created/changed:**
- internal/extractor/extractor.go
- internal/extractor/jsonpath.go
- internal/extractor/regex.go
- internal/extractor/extractor_test.go
- go.mod (added gjson dependency)
- go.sum (updated)

**Functions created/changed:**
- `Logger` interface for nil-safe warning output
- `Extractor` struct with JSONPath, Regex, Variable, OnError fields
- `ExtractAll()` function to apply extractors and return key-value pairs
- `findJSONPath()` for gjson-based JSON path extraction
- `findRegex()` for regex pattern matching with capture groups

**Tests created/changed:**
- TestExtract_JSONPath_Simple
- TestExtract_JSONPath_Nested
- TestExtract_JSONPath_Array
- TestExtract_JSONPath_DollarPrefix
- TestExtract_JSONPath_BareDollar
- TestExtract_Regex_Simple
- TestExtract_Regex_FullMatch
- TestExtract_InvalidRegex
- TestExtract_NoMatch_LogsWarning
- TestExtract_Regex_NoMatch_LogsWarning
- TestExtractAll_Multiple
- TestExtractAll_Mixed
- TestExtractAll_EmptyBody
- TestExtractAll_NilExtractors
- TestExtractAll_NilLogger

**Review Status:** APPROVED (after minor revisions)

**Git Commit Message:**
```
feat: add extractor package for response value extraction

- Add Extractor struct with JSONPath, Regex, Variable, OnError fields
- Implement JSON path extraction using gjson library
- Implement regex extraction with capture group support
- Add nil-safe Logger interface for warning output
- Log warnings on extraction failures, continue processing
- Add comprehensive unit tests (15 test cases)
```
