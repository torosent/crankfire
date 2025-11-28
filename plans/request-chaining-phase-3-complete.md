## Phase 3 Complete: Update Configuration Schema

Added extractor configuration to endpoint definitions, enabling users to define JSON path and regex extraction rules in their YAML/JSON config files.

**Files created/changed:**
- internal/config/config.go
- internal/config/loader.go
- internal/config/extractor_test.go

**Functions created/changed:**
- `Extractor` struct with JSONPath, Regex, Variable, OnError fields
- `Extractors []Extractor` field added to `Endpoint` struct
- `validateExtractors()` function for extractor validation
- `isValidVariableName()` helper for identifier validation
- `parseExtractors()` function in loader for YAML/JSON parsing
- `buildExtractor()` helper for building extractor from map

**Tests created/changed:**
- TestConfig_WithExtractors (YAML and JSON parsing)
- TestConfig_Validate_InvalidExtractor_NoSource
- TestConfig_Validate_InvalidExtractor_BothSources
- TestConfig_Validate_InvalidExtractor_NoVariable
- TestIsValidVariableName (9 test cases)

**Review Status:** APPROVED

**Git Commit Message:**
```
feat: add extractor configuration to endpoints

- Add Extractor struct with jsonpath, regex, var, on_error fields
- Add Extractors field to Endpoint for response value extraction
- Validate extractors: must have JSONPath XOR Regex, valid variable name
- Add YAML/JSON parsing support in config loader
- Add comprehensive tests for extractor configuration
```
