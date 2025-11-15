package feeder

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"sync"
)

// CSVFeeder reads records from a CSV file and provides them in round-robin order.
// It is safe for concurrent access.
type CSVFeeder struct {
	records []Record
	index   int
	mu      sync.Mutex
}

// NewCSVFeeder creates a new CSV feeder from the given file path.
// The first row is treated as the header containing field names.
func NewCSVFeeder(path string) (*CSVFeeder, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true

	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read CSV: %w", err)
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("CSV file is empty")
	}

	if len(rows) < 2 {
		return nil, fmt.Errorf("CSV file must have at least one header row and one data row")
	}

	header := rows[0]
	dataRows := rows[1:]

	records := make([]Record, 0, len(dataRows))
	for i, row := range dataRows {
		if len(row) != len(header) {
			return nil, fmt.Errorf("row %d has %d fields, expected %d", i+2, len(row), len(header))
		}

		record := make(Record)
		for j, field := range header {
			record[field] = row[j]
		}
		records = append(records, record)
	}

	return &CSVFeeder{
		records: records,
		index:   0,
	}, nil
}

// Next returns the next record in round-robin order.
// Returns ErrExhausted when all records have been consumed.
func (f *CSVFeeder) Next(ctx context.Context) (Record, error) {
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

// Close releases resources. For CSV feeder, this is a no-op.
func (f *CSVFeeder) Close() error {
	return nil
}

// Len returns the total number of records in the dataset.
func (f *CSVFeeder) Len() int {
	return len(f.records)
}
