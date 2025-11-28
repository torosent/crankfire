package variables

import (
	"testing"
)

func TestMemoryStore_SetGet(t *testing.T) {
	store := NewStore()
	store.Set("username", "john")
	store.Set("token", "abc123")

	value, ok := store.Get("username")
	if !ok {
		t.Fatal("expected to find 'username' key")
	}
	if value != "john" {
		t.Errorf("expected 'john', got %q", value)
	}

	value, ok = store.Get("token")
	if !ok {
		t.Fatal("expected to find 'token' key")
	}
	if value != "abc123" {
		t.Errorf("expected 'abc123', got %q", value)
	}
}

func TestMemoryStore_GetMissing(t *testing.T) {
	store := NewStore()
	store.Set("username", "john")

	value, ok := store.Get("missing_key")
	if ok {
		t.Errorf("expected ok=false for missing key, got ok=true with value %q", value)
	}
	if value != "" {
		t.Errorf("expected empty string for missing key, got %q", value)
	}
}

func TestMemoryStore_GetAll(t *testing.T) {
	store := NewStore()
	store.Set("username", "john")
	store.Set("token", "abc123")
	store.Set("id", "42")

	all := store.GetAll()
	if len(all) != 3 {
		t.Fatalf("expected 3 variables, got %d", len(all))
	}

	expectedValues := map[string]string{
		"username": "john",
		"token":    "abc123",
		"id":       "42",
	}

	for key, expectedValue := range expectedValues {
		if actual, ok := all[key]; !ok || actual != expectedValue {
			t.Errorf("expected all[%q]=%q, got %q (ok=%v)", key, expectedValue, actual, ok)
		}
	}

	// Verify it's a copy by modifying the returned map
	all["username"] = "modified"
	value, _ := store.Get("username")
	if value != "john" {
		t.Errorf("store was affected by modification to returned map, expected 'john', got %q", value)
	}
}

func TestMemoryStore_Merge(t *testing.T) {
	store := NewStore()
	store.Set("username", "john")
	store.Set("token", "xyz789")

	feederRecord := map[string]string{
		"username": "jane",
		"email":    "jane@example.com",
		"token":    "default_token",
	}

	merged := store.Merge(feederRecord)

	expectedMerged := map[string]string{
		"username": "john",
		"token":    "xyz789",
		"email":    "jane@example.com",
	}

	if len(merged) != 3 {
		t.Fatalf("expected 3 keys in merged result, got %d", len(merged))
	}

	for key, expectedValue := range expectedMerged {
		if actual, ok := merged[key]; !ok || actual != expectedValue {
			t.Errorf("expected merged[%q]=%q, got %q (ok=%v)", key, expectedValue, actual, ok)
		}
	}
}

func TestMemoryStore_Clear(t *testing.T) {
	store := NewStore()
	store.Set("username", "john")
	store.Set("token", "abc123")
	store.Set("id", "42")

	all := store.GetAll()
	if len(all) != 3 {
		t.Fatalf("expected 3 variables before clear, got %d", len(all))
	}

	store.Clear()

	all = store.GetAll()
	if len(all) != 0 {
		t.Fatalf("expected 0 variables after clear, got %d", len(all))
	}

	value, ok := store.Get("username")
	if ok {
		t.Errorf("expected ok=false after clear, got ok=true with value %q", value)
	}
}
