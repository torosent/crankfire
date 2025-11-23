package feeder

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sync"
)

// CSVFeeder reads records from a CSV file and provides them in round-robin order.
// It is safe for concurrent access.
type CSVFeeder struct {
	file        *os.File
	reader      *csv.Reader
	header      []string
	mu          sync.Mutex
	recordCount int
}

// NewCSVFeeder creates a new CSV feeder from the given file path.
// The first row is treated as the header containing field names.
func NewCSVFeeder(path string) (*CSVFeeder, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open CSV file: %w", err)
	}

	// Read header
	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("read CSV header: %w", err)
	}

	// Count records
	count := 0
	for {
		_, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			file.Close()
			return nil, fmt.Errorf("scan CSV: %w", err)
		}
		count++
	}

	if count == 0 {
		file.Close()
		return nil, fmt.Errorf("CSV file must have at least one data row")
	}

	// Reset to start of data
	if _, err := file.Seek(0, 0); err != nil {
		file.Close()
		return nil, fmt.Errorf("seek CSV: %w", err)
	}

	// Re-initialize reader and skip header
	reader = csv.NewReader(file)
	reader.TrimLeadingSpace = true
	if _, err := reader.Read(); err != nil {
		file.Close()
		return nil, fmt.Errorf("read CSV header after reset: %w", err)
	}

	return &CSVFeeder{
		file:        file,
		reader:      reader,
		header:      header,
		recordCount: count,
	}, nil
}

// Next returns the next record in round-robin order.
// It loops back to the beginning when the file is exhausted.
func (f *CSVFeeder) Next(ctx context.Context) (Record, error) {
	// Check context cancellation first
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	row, err := f.reader.Read()
	if err == io.EOF {
		// Loop back to start
		if _, seekErr := f.file.Seek(0, 0); seekErr != nil {
			return nil, fmt.Errorf("seek CSV: %w", seekErr)
		}
		f.reader = csv.NewReader(f.file)
		f.reader.TrimLeadingSpace = true
		// Skip header
		if _, headerErr := f.reader.Read(); headerErr != nil {
			return nil, fmt.Errorf("read CSV header: %w", headerErr)
		}
		// Read first row again
		row, err = f.reader.Read()
	}

	if err != nil {
		return nil, fmt.Errorf("read CSV row: %w", err)
	}

	if len(row) != len(f.header) {
		return nil, fmt.Errorf("row has %d fields, expected %d", len(row), len(f.header))
	}

	record := make(Record)
	for j, field := range f.header {
		record[field] = row[j]
	}
	return record, nil
}

// Close releases resources.
func (f *CSVFeeder) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.file != nil {
		return f.file.Close()
	}
	return nil
}

// Len returns the total number of records in the dataset.
func (f *CSVFeeder) Len() int {
	return f.recordCount
}
