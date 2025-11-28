## Phase 6 Complete: Documentation and Integration Tests

Implemented comprehensive documentation and integration tests for the request chaining feature.

**Files created/modified:**
- `docs/request-chaining.md` - New documentation file
- `scripts/chaining-sample.yml` - Sample configuration with extractors
- `cmd/crankfire/integration_test.go` - Added integration tests
- `cmd/crankfire/samples_test.go` - Added chaining-sample.yml to test list

**Documentation (docs/request-chaining.md):**
- Overview of request chaining feature and use cases
- How it works: Extract → Store → Use → Fallback
- Extractor configuration syntax (JSON path and regex)
- Variable placeholder usage with `{{var}}` and `{{var|default}}` syntax
- OnError flag explanation for extracting from error responses
- 6 complete examples:
  - Basic JSON path extraction
  - Regex extraction with capture groups
  - Chaining multiple endpoints
  - Using default values
  - Extracting from error responses
  - Multi-step order workflow
- Variable scope and persistence
- Combining with feeders
- Best practices (7 points)
- Limitations (6 points)
- Debugging guide with examples

**Sample Configuration (scripts/chaining-sample.yml):**
- Realistic user management workflow with 5 endpoints
- Create user → extract user_id and session_token
- Get profile → use extracted user_id in URL, extract profile_url
- Update settings → use multiple extracted variables in body and headers
- List posts → use extracted user_id with query parameters
- Delete user → extract error details on failure with on_error flag
- Integrated feeder example showing data-driven requests
- Extensive comments explaining each section

**Integration Tests (cmd/crankfire/integration_test.go):**
- `TestIntegration_RequestChaining` - Two-step workflow (create user → get by ID)
  - Verifies extraction from first request
  - Verifies extracted value used in second request
  - Tests JSON path extraction
  
- `TestIntegration_RequestChaining_WithDefaults` - Default value fallback
  - Tests extraction failure handling
  - Tests empty value storage
  - Tests default value substitution in subsequent requests
  
- `TestIntegration_RequestChaining_MultiStep` - Complex multi-step workflow
  - Three consecutive requests with dependent extractions
  - Create order → extract order_id
  - Get order → extract order_total
  - Update order → use both extracted values
  - Tests variable persistence across multiple requests

All tests pass successfully:
```
=== RUN   TestIntegration_RequestChaining
--- PASS: TestIntegration_RequestChaining (0.00s)
=== RUN   TestIntegration_RequestChaining_WithDefaults
--- PASS: TestIntegration_RequestChaining_WithDefaults (0.00s)
=== RUN   TestIntegration_RequestChaining_MultiStep
--- PASS: TestIntegration_RequestChaining_MultiStep (0.00s)
PASS
```

**Test Coverage:**
- All existing unit tests still pass (no regressions)
- Sample configuration test updated to include chaining-sample.yml
- Configuration parsing verified for complex extractors

**Documentation Style:**
- Follows existing docs style (see feeders.md, configuration.md)
- Proper markdown formatting with code blocks and examples
- YAML syntax highlighting for configuration examples
- Clear, concise explanations with practical examples
- Cross-references to related documentation

**Review Status:** READY FOR REVIEW

**Git Commit Message:**
```
docs: add request chaining documentation and integration tests

- Add comprehensive request-chaining.md documentation
  - Overview and use cases
  - JSON path and regex extraction guide
  - Variable placeholder syntax with defaults
  - OnError flag for error response extraction
  - 6 complete examples including multi-step workflows
  - Best practices and limitations
  - Debugging guide

- Add chaining-sample.yml configuration
  - User management workflow (5 endpoints)
  - Demonstrates extraction, variable usage, defaults
  - Shows feeder integration with extractors
  - Well-commented for learning

- Add integration tests to integration_test.go
  - TestIntegration_RequestChaining: basic chaining
  - TestIntegration_RequestChaining_WithDefaults: fallback values
  - TestIntegration_RequestChaining_MultiStep: complex workflow
  - All tests verify extraction and value persistence

- Update samples_test.go to validate chaining-sample.yml

All tests pass with no regressions.
```
