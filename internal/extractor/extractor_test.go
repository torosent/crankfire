package extractor

import (
	"testing"
)

// mockLogger is a test logger that captures warnings
type mockLogger struct {
	warnings []string
}

func (m *mockLogger) Warn(format string, args ...interface{}) {
	m.warnings = append(m.warnings, format)
}

func TestExtract_JSONPath_Simple(t *testing.T) {
	body := []byte(`{"id": 123, "name": "John"}`)
	extractors := []Extractor{
		{
			JSONPath: "id",
			Variable: "user_id",
		},
	}

	result := ExtractAll(body, extractors, nil)

	if result["user_id"] != "123" {
		t.Errorf("expected '123', got '%s'", result["user_id"])
	}
}

func TestExtract_JSONPath_Nested(t *testing.T) {
	body := []byte(`{"user": {"profile": {"name": "Alice"}}}`)
	extractors := []Extractor{
		{
			JSONPath: "user.profile.name",
			Variable: "full_name",
		},
	}

	result := ExtractAll(body, extractors, nil)

	if result["full_name"] != "Alice" {
		t.Errorf("expected 'Alice', got '%s'", result["full_name"])
	}
}

func TestExtract_JSONPath_Array(t *testing.T) {
	body := []byte(`{"items": [{"id": 1}, {"id": 2}]}`)
	extractors := []Extractor{
		{
			JSONPath: "items.0.id",
			Variable: "first_item_id",
		},
	}

	result := ExtractAll(body, extractors, nil)

	if result["first_item_id"] != "1" {
		t.Errorf("expected '1', got '%s'", result["first_item_id"])
	}
}

func TestExtract_JSONPath_DollarPrefix(t *testing.T) {
	body := []byte(`{"id": 456}`)
	extractors := []Extractor{
		{
			JSONPath: "$.id",
			Variable: "user_id",
		},
	}

	result := ExtractAll(body, extractors, nil)

	if result["user_id"] != "456" {
		t.Errorf("expected '456', got '%s'", result["user_id"])
	}
}

func TestExtract_Regex_Simple(t *testing.T) {
	body := []byte(`Response: ID=789`)
	extractors := []Extractor{
		{
			Regex:    `ID=(\d+)`,
			Variable: "response_id",
		},
	}

	result := ExtractAll(body, extractors, nil)

	if result["response_id"] != "789" {
		t.Errorf("expected '789', got '%s'", result["response_id"])
	}
}

func TestExtract_Regex_FullMatch(t *testing.T) {
	body := []byte(`The code is 12345`)
	extractors := []Extractor{
		{
			Regex:    `\d+`,
			Variable: "code",
		},
	}

	result := ExtractAll(body, extractors, nil)

	if result["code"] != "12345" {
		t.Errorf("expected '12345', got '%s'", result["code"])
	}
}

func TestExtract_InvalidRegex(t *testing.T) {
	body := []byte(`some text`)
	logger := &mockLogger{}
	extractors := []Extractor{
		{
			Regex:    `[invalid(regex`,
			Variable: "bad_regex",
		},
	}

	result := ExtractAll(body, extractors, logger)

	// Should log error for invalid regex
	if len(logger.warnings) == 0 {
		t.Errorf("expected warning for invalid regex")
	}

	// Should continue and return empty string
	if result["bad_regex"] != "" {
		t.Errorf("expected empty string for invalid regex, got '%s'", result["bad_regex"])
	}
}

func TestExtract_NoMatch_LogsWarning(t *testing.T) {
	body := []byte(`{"id": 123}`)
	logger := &mockLogger{}
	extractors := []Extractor{
		{
			JSONPath: "missing_field",
			Variable: "result",
		},
	}

	result := ExtractAll(body, extractors, logger)

	// Should log warning for no match
	if len(logger.warnings) == 0 {
		t.Errorf("expected warning for missing field")
	}

	// Should return empty string
	if result["result"] != "" {
		t.Errorf("expected empty string for missing field, got '%s'", result["result"])
	}
}

func TestExtract_Regex_NoMatch_LogsWarning(t *testing.T) {
	body := []byte(`no numbers here`)
	logger := &mockLogger{}
	extractors := []Extractor{
		{
			Regex:    `\d+`,
			Variable: "number",
		},
	}

	result := ExtractAll(body, extractors, logger)

	// Should log warning for no match
	if len(logger.warnings) == 0 {
		t.Errorf("expected warning for regex no match")
	}

	// Should return empty string
	if result["number"] != "" {
		t.Errorf("expected empty string for no match, got '%s'", result["number"])
	}
}

func TestExtractAll_Multiple(t *testing.T) {
	body := []byte(`{"user": {"id": 999, "email": "test@example.com"}, "status": "active"}`)
	extractors := []Extractor{
		{
			JSONPath: "user.id",
			Variable: "user_id",
		},
		{
			JSONPath: "user.email",
			Variable: "email",
		},
		{
			JSONPath: "status",
			Variable: "status",
		},
	}

	result := ExtractAll(body, extractors, nil)

	if result["user_id"] != "999" {
		t.Errorf("user_id: expected '999', got '%s'", result["user_id"])
	}
	if result["email"] != "test@example.com" {
		t.Errorf("email: expected 'test@example.com', got '%s'", result["email"])
	}
	if result["status"] != "active" {
		t.Errorf("status: expected 'active', got '%s'", result["status"])
	}
}

func TestExtractAll_Mixed(t *testing.T) {
	body := []byte(`{"user": {"id": 111}, "message": "User ID: 111"}`)
	extractors := []Extractor{
		{
			JSONPath: "user.id",
			Variable: "json_id",
		},
		{
			Regex:    `User ID: (\d+)`,
			Variable: "regex_id",
		},
	}

	result := ExtractAll(body, extractors, nil)

	if result["json_id"] != "111" {
		t.Errorf("json_id: expected '111', got '%s'", result["json_id"])
	}
	if result["regex_id"] != "111" {
		t.Errorf("regex_id: expected '111', got '%s'", result["regex_id"])
	}
}

func TestExtractAll_EmptyBody(t *testing.T) {
	body := []byte(``)
	extractors := []Extractor{
		{
			JSONPath: "id",
			Variable: "id",
		},
	}

	result := ExtractAll(body, extractors, nil)

	if result["id"] != "" {
		t.Errorf("expected empty string for empty body, got '%s'", result["id"])
	}
}

func TestExtractAll_NilExtractors(t *testing.T) {
	body := []byte(`{"id": 123}`)

	result := ExtractAll(body, nil, nil)

	if len(result) != 0 {
		t.Errorf("expected empty result for nil extractors, got %d items", len(result))
	}
}

func TestExtractAll_NilLogger(t *testing.T) {
	body := []byte(`{"id": 123}`)
	extractors := []Extractor{
		{
			JSONPath: "missing",
			Variable: "result",
		},
	}

	// Should not panic with nil logger
	result := ExtractAll(body, extractors, nil)

	if result["result"] != "" {
		t.Errorf("expected empty string, got '%s'", result["result"])
	}
}

func TestExtract_JSONPath_BareDollar(t *testing.T) {
	body := []byte(`{"id": 123, "name": "test"}`)
	extractors := []Extractor{
		{
			JSONPath: "$",
			Variable: "entire_json",
		},
	}

	result := ExtractAll(body, extractors, nil)

	// Bare "$" should return the entire JSON as a string
	expected := `{"id": 123, "name": "test"}`
	if result["entire_json"] != expected {
		t.Errorf("expected '%s', got '%s'", expected, result["entire_json"])
	}
}
