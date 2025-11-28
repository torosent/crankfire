## Plan: Request Chaining & Variable Extraction

Add the ability to extract values from HTTP responses using JSON path and regex patterns, store them as variables per-worker, and use them in subsequent requests. This enables realistic user session flows where one request's output feeds into another (e.g., create user → get user by extracted ID).

**Phases (6 phases)**

1. **Phase 1: Create Variable Store Package**
    - **Objective:** Create a thread-safe variable store for holding extracted values per-worker
    - **Files/Functions to Create:**
      - `internal/variables/store.go` - `Store` interface and `MemoryStore` implementation
      - `internal/variables/store_test.go` - Unit tests
    - **Tests to Write:**
      - `TestMemoryStore_SetGet`
      - `TestMemoryStore_GetMissing`
      - `TestMemoryStore_GetAll`
      - `TestMemoryStore_Merge`
      - `TestMemoryStore_Clear`
    - **Steps:**
      1. Write tests for the variable store interface and implementation
      2. Run tests to see them fail
      3. Implement `Store` interface with `Set`, `Get`, `GetAll`, `Merge`, `Clear` methods
      4. Implement `MemoryStore` as the concrete implementation (map-based, no mutex needed for per-worker use)
      5. Run tests to confirm they pass

2. **Phase 2: Create Extractor Package**
    - **Objective:** Create extraction logic for JSON path and regex patterns
    - **Files/Functions to Create:**
      - `internal/extractor/extractor.go` - `Extractor` struct and `ExtractAll` function
      - `internal/extractor/jsonpath.go` - JSON path extraction using gjson
      - `internal/extractor/regex.go` - Regex extraction
      - `internal/extractor/extractor_test.go` - Unit tests
    - **Tests to Write:**
      - `TestExtract_JSONPath_Simple`
      - `TestExtract_JSONPath_Nested`
      - `TestExtract_JSONPath_Array`
      - `TestExtract_Regex_Simple`
      - `TestExtract_Regex_CaptureGroup`
      - `TestExtract_InvalidJSONPath`
      - `TestExtract_InvalidRegex`
      - `TestExtract_NoMatch_LogsWarning`
      - `TestExtract_OnError_Configurable`
    - **Steps:**
      1. Add `github.com/tidwall/gjson` dependency
      2. Write tests for JSON path and regex extraction
      3. Run tests to see them fail
      4. Implement `Extractor` struct with `JSONPath`, `Regex`, `Variable`, and `OnError` fields
      5. Implement `ExtractAll` function that applies extractors to response body, logs warnings on no match
      6. Run tests to confirm they pass

3. **Phase 3: Update Configuration Schema**
    - **Objective:** Add extractor configuration to endpoint definitions
    - **Files/Functions to Modify:**
      - `internal/config/config.go` - Add `Extractor` type and `Extractors` field to `Endpoint`
      - `internal/config/config_test.go` - Add tests for extractor configuration
      - `internal/config/loader.go` - Ensure extractors are parsed from YAML
    - **Tests to Write:**
      - `TestConfig_WithExtractors`
      - `TestConfig_Validate_InvalidExtractor`
      - `TestConfig_Validate_ExtractorMissingVariable`
    - **Steps:**
      1. Write tests for config with extractors
      2. Run tests to see them fail
      3. Add `Extractor` struct to config package with JSONPath, Regex, Variable, OnError fields
      4. Add `Extractors []Extractor` field to `Endpoint` struct
      5. Add validation: must have either JSONPath or Regex (not both), must have Variable name
      6. Run tests to confirm they pass

4. **Phase 4: Integrate Variable Store into Request Flow**
    - **Objective:** Pass variable store through context and use in placeholder resolution with default value support
    - **Files/Functions to Modify:**
      - `cmd/crankfire/endpoints.go` - Create and attach variable store to context per-worker
      - `cmd/crankfire/placeholders.go` - Update to merge variables with feeder records, support `{{var|default}}` syntax
      - `cmd/crankfire/placeholders_test.go` - Add tests for variable merging and defaults
      - `cmd/crankfire/endpoints_test.go` - Add tests for variable store context
    - **Tests to Write:**
      - `TestEndpointSelector_WithVariableStore`
      - `TestPlaceholders_MergeVariables`
      - `TestPlaceholders_DefaultValue`
      - `TestPlaceholders_VariableOverridesFeeder`
    - **Steps:**
      1. Write tests for variable store integration and default value syntax
      2. Run tests to see them fail
      3. Add context key for variable store in endpoints.go
      4. Create variable store per worker and attach to context
      5. Update `resolvePlaceholder` to check variables, support `{{var|default}}` syntax
      6. Run tests to confirm they pass

5. **Phase 5: Implement Response Extraction in HTTP Requester**
    - **Objective:** Extract values from responses and store in variable store
    - **Files/Functions to Modify:**
      - `cmd/crankfire/main.go` - Update `httpRequester.Do()` to extract values from response
      - `cmd/crankfire/endpoints.go` - Pass extractors from endpoint config to context
      - `cmd/crankfire/main_test.go` - Add extraction tests
    - **Tests to Write:**
      - `TestHTTPRequester_ExtractsJSONPath`
      - `TestHTTPRequester_ExtractsRegex`
      - `TestHTTPRequester_ExtractorChaining`
      - `TestHTTPRequester_ExtractorNoMatch_LogsWarning`
      - `TestHTTPRequester_ExtractOnError_Configurable`
    - **Steps:**
      1. Write tests for response extraction
      2. Run tests to see them fail
      3. Update `endpointTemplate` to include extractors
      4. Update `httpRequester.Do()` to read response body when extractors present
      5. Apply extractors, store values in variable store from context
      6. Respect `OnError` config: extract from error responses only if configured
      7. Log warnings when extraction fails to match
      8. Run tests to confirm they pass

6. **Phase 6: Documentation and Integration Tests**
    - **Objective:** Document the feature and add end-to-end integration tests
    - **Files/Functions to Create/Modify:**
      - `docs/request-chaining.md` - New documentation file
      - `docs/configuration.md` - Add extractor section
      - `scripts/chaining-sample.yml` - Sample configuration
      - `cmd/crankfire/integration_test.go` - Add chaining integration test
    - **Tests to Write:**
      - `TestIntegration_RequestChaining`
      - `TestIntegration_RequestChaining_WithDefaults`
    - **Steps:**
      1. Write integration test for full request chaining flow
      2. Run test to see it fail
      3. Fix any integration issues discovered
      4. Run test to confirm it passes
      5. Write documentation with examples showing JSON path, regex, defaults, and OnError config
      6. Create sample configuration file

**Design Decisions (User Approved)**

1. **Variable Scope:** Per-worker - each worker goroutine has its own variable store, ensuring determinism and no locking overhead
2. **Extractor Failures:** Log warning and continue - missing extractions don't fail the request
3. **Default Values:** Support `{{var|default}}` syntax for fallback values
4. **Extraction on Errors:** Configurable per-extractor via `on_error: true` field

**Example Configuration**

```yaml
endpoints:
  - name: create-user
    method: POST
    path: /users
    body: '{"name": "{{name}}"}'
    extractors:
      - jsonpath: "$.id"
        var: "user_id"
      - jsonpath: "$.token"
        var: "auth_token"
  
  - name: get-user
    path: /users/{{user_id}}
    headers:
      Authorization: "Bearer {{auth_token|anonymous}}"
  
  - name: delete-user
    method: DELETE
    path: /users/{{user_id}}
    extractors:
      - regex: "deleted: (\\d+)"
        var: "deleted_count"
        on_error: true  # Extract even from error responses
```

**Dependencies to Add**

- `github.com/tidwall/gjson` - Fast JSON path extraction library
