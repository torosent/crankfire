package har

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// ParseFile reads and parses a HAR file from disk
func ParseFile(path string) (*HAR, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open HAR file: %w", err)
	}
	defer file.Close()

	return Parse(file)
}

// Parse reads and parses a HAR from an io.Reader
func Parse(r io.Reader) (*HAR, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read HAR data: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("empty HAR data")
	}

	var har HAR
	if err := json.Unmarshal(data, &har); err != nil {
		return nil, fmt.Errorf("failed to parse HAR JSON: %w", err)
	}

	if har.Log == nil {
		return nil, fmt.Errorf("invalid HAR: missing Log field")
	}

	return &har, nil
}
