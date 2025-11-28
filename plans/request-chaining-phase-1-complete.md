## Phase 1 Complete: Create Variable Store Package

Created a thread-safe variable store for holding extracted values per-worker. The store provides a simple map-based implementation with methods for setting, getting, merging with feeder records, and clearing variables.

**Files created/changed:**
- internal/variables/store.go
- internal/variables/store_test.go

**Functions created/changed:**
- `Store` interface (Set, Get, GetAll, Merge, Clear)
- `MemoryStore` struct implementing Store
- `NewStore()` constructor function

**Tests created/changed:**
- TestMemoryStore_SetGet
- TestMemoryStore_GetMissing
- TestMemoryStore_GetAll
- TestMemoryStore_Merge
- TestMemoryStore_Clear

**Review Status:** APPROVED

**Git Commit Message:**
```
feat: add variable store for request chaining

- Add Store interface with Set, Get, GetAll, Merge, Clear methods
- Implement MemoryStore for per-worker variable storage
- Variables take precedence over feeder records in Merge
- Add comprehensive unit tests for all store operations
```
