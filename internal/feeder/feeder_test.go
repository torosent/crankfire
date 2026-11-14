package feeder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestCSVFeederLoadAndRoundRobin(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "users.csv")
	csvContent := `user_id,email,name
1,alice@example.com,Alice
2,bob@example.com,Bob
3,charlie@example.com,Charlie`

	if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	feeder, err := NewCSVFeeder(csvPath)
	if err != nil {
		t.Fatalf("NewCSVFeeder() error = %v", err)
	}
	defer feeder.Close()

	if feeder.Len() != 3 {
		t.Errorf("Len() = %d, want 3", feeder.Len())
	}

	ctx := context.Background()

	// First round
	rec1, err := feeder.Next(ctx)
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if rec1["user_id"] != "1" || rec1["email"] != "alice@example.com" || rec1["name"] != "Alice" {
		t.Errorf("First record = %v, want Alice's data", rec1)
	}

	rec2, err := feeder.Next(ctx)
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if rec2["user_id"] != "2" || rec2["email"] != "bob@example.com" {
		t.Errorf("Second record = %v, want Bob's data", rec2)
	}

	rec3, err := feeder.Next(ctx)
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if rec3["user_id"] != "3" || rec3["name"] != "Charlie" {
		t.Errorf("Third record = %v, want Charlie's data", rec3)
	}

	// Fourth call should loop back to the first record
	rec4, err := feeder.Next(ctx)
	if err != nil {
		t.Fatalf("Next() after exhaustion error = %v", err)
	}
	if rec4["user_id"] != "1" || rec4["email"] != "alice@example.com" {
		t.Errorf("Fourth record (looped) = %v, want Alice's data", rec4)
	}
}

func TestJSONFeederLoadAndRoundRobin(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "products.json")
	jsonContent := `[
		{"product_id": "p1", "name": "Widget", "price": "19.99"},
		{"product_id": "p2", "name": "Gadget", "price": "29.99"}
	]`

	if err := os.WriteFile(jsonPath, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	feeder, err := NewJSONFeeder(jsonPath)
	if err != nil {
		t.Fatalf("NewJSONFeeder() error = %v", err)
	}
	defer feeder.Close()

	if feeder.Len() != 2 {
		t.Errorf("Len() = %d, want 2", feeder.Len())
	}

	ctx := context.Background()

	rec1, err := feeder.Next(ctx)
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if rec1["product_id"] != "p1" || rec1["name"] != "Widget" {
		t.Errorf("First record = %v, want Widget data", rec1)
	}

	rec2, err := feeder.Next(ctx)
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if rec2["product_id"] != "p2" || rec2["price"] != "29.99" {
		t.Errorf("Second record = %v, want Gadget data", rec2)
	}

	// Third call should loop back
	rec3, err := feeder.Next(ctx)
	if err != nil {
		t.Fatalf("Next() after exhaustion error = %v", err)
	}
	if rec3["product_id"] != "p1" {
		t.Errorf("Third record (looped) = %v, want Widget data", rec3)
	}
}

func TestFeederConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "concurrent.csv")

	// Create CSV with 100 records
	var rows []string
	rows = append(rows, "id,value")
	for i := 1; i <= 100; i++ {
		rows = append(rows, fmt.Sprintf("%d,value-%d", i, i))
	}
	csvContent := strings.Join(rows, "\n")

	if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	feeder, err := NewCSVFeeder(csvPath)
	if err != nil {
		t.Fatalf("NewCSVFeeder() error = %v", err)
	}
	defer feeder.Close()

	ctx := context.Background()
	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	recordsChan := make(chan Record, numGoroutines)
	errorsChan := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			rec, err := feeder.Next(ctx)
			if err != nil {
				errorsChan <- err
				return
			}
			recordsChan <- rec
		}()
	}

	wg.Wait()
	close(recordsChan)
	close(errorsChan)

	// Should get exactly 50 records
	records := make([]Record, 0)
	for rec := range recordsChan {
		records = append(records, rec)
	}

	if len(records) != numGoroutines {
		t.Errorf("Got %d records, want %d", len(records), numGoroutines)
	}

	// Verify no duplicate IDs (deterministic round-robin)
	seen := make(map[string]bool)
	for _, rec := range records {
		id := rec["id"]
		if seen[id] {
			t.Errorf("Duplicate record ID: %s", id)
		}
		seen[id] = true
	}
}

func TestCSVFeederWithMissingFile(t *testing.T) {
	_, err := NewCSVFeeder("/nonexistent/path/file.csv")
	if err == nil {
		t.Fatal("NewCSVFeeder() with missing file error = nil, want error")
	}
}

func TestJSONFeederWithInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "invalid.json")
	if err := os.WriteFile(jsonPath, []byte(`{invalid json`), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := NewJSONFeeder(jsonPath)
	if err == nil {
		t.Fatal("NewJSONFeeder() with invalid JSON error = nil, want error")
	}
}

func TestCSVFeederWithEmptyFile(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "empty.csv")
	if err := os.WriteFile(csvPath, []byte(""), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := NewCSVFeeder(csvPath)
	if err == nil {
		t.Fatal("NewCSVFeeder() with empty file error = nil, want error")
	}
}

func TestFeederContextCancellation(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "data.csv")
	csvContent := "id,value\n1,test"
	if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	feeder, err := NewCSVFeeder(csvPath)
	if err != nil {
		t.Fatalf("NewCSVFeeder() error = %v", err)
	}
	defer feeder.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = feeder.Next(ctx)
	if err != context.Canceled {
		t.Errorf("Next() with cancelled context error = %v, want context.Canceled", err)
	}
}

func TestJSONFeederKeepsNumbersAsWritten(t *testing.T) {
	jsonPath := filepath.Join(t.TempDir(), "ids.json")
	if err := os.WriteFile(jsonPath, []byte(`[{"user_id": 1234567, "price": 19.5}]`), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	feeder, err := NewJSONFeeder(jsonPath)
	if err != nil {
		t.Fatalf("NewJSONFeeder() error = %v", err)
	}
	record, err := feeder.Next(context.Background())
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if record["user_id"] != "1234567" || record["price"] != "19.5" {
		t.Errorf("numbers changed: %v", record)
	}
}
