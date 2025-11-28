## Phase 4 Complete: Integrate Variable Store into Request Flow

Integrated variable store into the request flow with context-based passing and enhanced placeholder resolution supporting default values.

**Files created/changed:**
- cmd/crankfire/endpoints.go
- cmd/crankfire/placeholders.go
- cmd/crankfire/placeholders_test.go

**Functions created/changed:**
- `variableStoreCtxKey` struct type for context key
- `contextWithVariableStore()` - attaches store to context
- `variableStoreFromContext()` - retrieves store from context
- `applyPlaceholderWithVariables()` - resolves placeholders with variable store and defaults
- `applyPlaceholdersToMapWithVariables()` - applies variable resolution to map values
- Updated `endpointSelectionRequester.Do()` to create per-worker stores

**Tests created/changed:**
- TestContextWithVariableStore
- TestVariableStoreFromContext
- TestApplyPlaceholders_WithVariables
- TestApplyPlaceholders_DefaultValue
- TestApplyPlaceholders_EmptyDefault
- TestApplyPlaceholders_VariablePriority
- TestApplyPlaceholdersToMapWithVariables
- TestEndpointSelector_CreatesVariableStore

**Review Status:** APPROVED (after minor revision to remove nil context test)

**Git Commit Message:**
```
feat: integrate variable store into request flow

- Add context key and helpers for variable store
- Create variable store per-worker in endpoint selector
- Support {{var|default}} syntax in placeholder resolution
- Implement resolution priority: Store > Feeder > Default
- Add comprehensive tests for variable integration
```
