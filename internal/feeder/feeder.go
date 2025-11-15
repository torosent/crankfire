package feeder

import (
	"context"
	"fmt"
)

// Record represents a single row of data with named fields.
type Record map[string]string

// Feeder provides per-request data from a dataset with deterministic round-robin selection.
// Implementations must be safe for concurrent use.
type Feeder interface {
	// Next returns the next record from the dataset or an error if exhausted.
	// Records are returned in deterministic round-robin order.
	Next(ctx context.Context) (Record, error)

	// Close releases any resources held by the feeder.
	Close() error

	// Len returns the total number of records in the dataset.
	Len() int
}

// ErrExhausted is returned when a feeder has no more records and rewind is disabled.
var ErrExhausted = fmt.Errorf("feeder exhausted: no more records available")
