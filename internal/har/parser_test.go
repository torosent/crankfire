package har

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestParseFile_ValidHAR(t *testing.T) {
	har, err := ParseFile("testdata/valid.har")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if har == nil {
		t.Fatal("expected HAR to be non-nil")
	}

	if har.Log == nil {
		t.Fatal("expected Log to be non-nil")
	}

	if har.Log.Version != "1.2" {
		t.Errorf("expected version 1.2, got %s", har.Log.Version)
	}

	if har.Log.Creator == nil {
		t.Fatal("expected Creator to be non-nil")
	}

	if har.Log.Creator.Name == "" {
		t.Error("expected Creator Name to be non-empty")
	}

	if len(har.Log.Entries) == 0 {
		t.Fatal("expected Entries to be non-empty")
	}

	firstEntry := har.Log.Entries[0]
	if firstEntry.Request == nil {
		t.Fatal("expected Request to be non-nil")
	}

	if firstEntry.Request.Method == "" {
		t.Error("expected Request Method to be non-empty")
	}

	if firstEntry.Request.URL == "" {
		t.Error("expected Request URL to be non-empty")
	}

	if firstEntry.Response == nil {
		t.Fatal("expected Response to be non-nil")
	}

	if firstEntry.Response.Status == 0 {
		t.Error("expected Response Status to be non-zero")
	}
}

func TestParseFile_InvalidJSON(t *testing.T) {
	har, err := ParseFile("testdata/invalid.har")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}

	if har != nil {
		t.Error("expected HAR to be nil on error")
	}

	if err.Error() == "" {
		t.Error("expected error message to be non-empty")
	}
}

func TestParseFile_MissingLog(t *testing.T) {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "no_log.har")

	content := []byte(`{"notALog": {}}`)

	if err := os.WriteFile(tempFile, content, 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	har, err := ParseFile(tempFile)
	if err == nil {
		t.Fatal("expected error for missing Log field")
	}

	if har == nil {
		return
	}

	if har.Log == nil {
		t.Fatal("expected validation to catch missing Log")
	}
}

func TestParseFile_FileNotFound(t *testing.T) {
	har, err := ParseFile("testdata/nonexistent.har")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}

	if har != nil {
		t.Error("expected HAR to be nil on error")
	}
}

func TestParse_ValidHAR(t *testing.T) {
	jsonData := []byte(`{
		"log": {
			"version": "1.2",
			"creator": {
				"name": "Test Creator",
				"version": "1.0"
			},
			"entries": [
				{
					"startedDateTime": "2025-01-01T00:00:00Z",
					"time": 100,
					"request": {
						"method": "GET",
						"url": "https://example.com/api/users",
						"httpVersion": "HTTP/1.1",
						"headers": [],
						"queryString": [],
						"cookies": [],
						"headersSize": -1,
						"bodySize": -1
					},
					"response": {
						"status": 200,
						"statusText": "OK",
						"httpVersion": "HTTP/1.1",
						"headers": [],
						"cookies": [],
						"content": {
							"size": 100,
							"mimeType": "application/json"
						},
						"redirectURL": "",
						"headersSize": -1,
						"bodySize": -1
					},
					"cache": {},
					"timings": {
						"wait": 50,
						"receive": 50
					}
				}
			]
		}
	}`)

	reader := bytes.NewReader(jsonData)
	har, err := Parse(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if har == nil {
		t.Fatal("expected HAR to be non-nil")
	}

	if har.Log == nil {
		t.Fatal("expected Log to be non-nil")
	}

	if har.Log.Version != "1.2" {
		t.Errorf("expected version 1.2, got %s", har.Log.Version)
	}

	if len(har.Log.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(har.Log.Entries))
	}

	entry := har.Log.Entries[0]
	if entry.Request.Method != "GET" {
		t.Errorf("expected method GET, got %s", entry.Request.Method)
	}

	if entry.Response.Status != 200 {
		t.Errorf("expected status 200, got %d", entry.Response.Status)
	}
}

func TestParse_InvalidJSON(t *testing.T) {
	reader := bytes.NewReader([]byte(`{invalid json`))
	har, err := Parse(reader)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}

	if har != nil {
		t.Error("expected HAR to be nil on error")
	}
}

func TestParse_EmptyReader(t *testing.T) {
	reader := bytes.NewReader([]byte(""))
	har, err := Parse(reader)
	if err == nil {
		t.Fatal("expected error for empty reader")
	}

	if har != nil {
		t.Error("expected HAR to be nil on error")
	}
}

func TestParse_ReaderError(t *testing.T) {
	errorReader := &erroringReader{}
	har, err := Parse(errorReader)
	if err == nil {
		t.Fatal("expected error from reader")
	}

	if har != nil {
		t.Error("expected HAR to be nil on error")
	}
}

type erroringReader struct{}

func (er *erroringReader) Read(p []byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}
