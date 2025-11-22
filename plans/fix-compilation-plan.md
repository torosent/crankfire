## Plan: Fix Compilation and Formatting Issues

I have identified a compilation error in `cmd/crankfire/main.go` (missing arguments in `dashboard.New`) and some unformatted files. The research agent has proactively applied these fixes. I will now verify the codebase is stable.

**Phases**
1. **Phase 1: Verify Fixes**
    - **Objective:** Ensure the codebase compiles and passes all tests.
    - **Files Modified:** `cmd/crankfire/main.go` (logic fix), various files (formatting).
    - **Steps:**
        1. Run `go vet ./...` to confirm no errors.
        2. Run `go test ./...` to ensure all tests pass.

**Open Questions**
1. None.
