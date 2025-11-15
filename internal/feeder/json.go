package feeder

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// JSONFeeder reads records from a JSON file containing an array of objects.
// It provides records in round-robin order and is safe for concurrent access.
type JSONFeeder struct {
	records []Record
	index   int
	mu      sync.Mutex
}

// NewJSONFeeder creates a new JSON feeder from the given file path.
// The file must contain a JSON array of objects.
func NewJSONFeeder(path string) (*JSONFeeder, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open JSON file: %w", err)
	}
	defer file.Close()

	var rawRecords []map[string]interface{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&rawRecords); err != nil {
		return nil, fmt.Errorf("decode JSON: %w", err)
	}

	if len(rawRecords) == 0 {
		return nil, fmt.Errorf("JSON file contains empty array")
	}

	records := make([]Record, 0, len(rawRecords))
	for i, rawRecord := range rawRecords {
		record := make(Record)
		for key, value := range rawRecord {
			// Convert all values to strings
			record[key] = fmt.Sprintf("%v", value)
		}
		if len(record) == 0 {
			return nil, fmt.Errorf("record %d is empty", i)
		}
		records = append(records, record)
	}

	return &JSONFeeder{
		records: records,
		index:   0,
	}, nil
}

// Next returns the next record in round-robin order.
// Returns ErrExhausted when all records have been consumed.
func (f *JSONFeeder) Next(ctx context.Context) (Record, error) {
	// Check context cancellation first
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.index >= len(f.records) {
		return nil, ErrExhausted
	}

	record := f.records[f.index]
	f.index++
	return record, nil
}

// Close releases resources. For JSON feeder, this is a no-op.
func (f *JSONFeeder) Close() error {
	return nil
}

// Len returns the total number of records in the dataset.
func (f *JSONFeeder) Len() int {
	return len(f.records)
}
