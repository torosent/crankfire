package main

import (
	"context"
	"reflect"
	"testing"

	"github.com/torosent/crankfire/internal/variables"
)

func TestApplyPlaceholders(t *testing.T) {
	tests := []struct {
		name     string
		template string
		record   map[string]string
		want     string
	}{
		{
			name:     "no placeholders",
			template: "hello world",
			record:   map[string]string{"key": "value"},
			want:     "hello world",
		},
		{
			name:     "single placeholder",
			template: "hello {{name}}",
			record:   map[string]string{"name": "world"},
			want:     "hello world",
		},
		{
			name:     "multiple placeholders",
			template: "{{greeting}} {{name}}",
			record:   map[string]string{"greeting": "hello", "name": "world"},
			want:     "hello world",
		},
		{
			name:     "missing placeholder in record",
			template: "hello {{name}}",
			record:   map[string]string{"other": "value"},
			want:     "hello {{name}}",
		},
		{
			name:     "empty record",
			template: "hello {{name}}",
			record:   nil,
			want:     "hello {{name}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyPlaceholders(tt.template, tt.record)
			if got != tt.want {
				t.Errorf("applyPlaceholders() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestApplyPlaceholdersToMap(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
		record map[string]string
		want   map[string]string
	}{
		{
			name:   "nil values",
			values: nil,
			record: map[string]string{"key": "value"},
			want:   nil,
		},
		{
			name:   "empty values",
			values: map[string]string{},
			record: map[string]string{"key": "value"},
			want:   nil,
		},
		{
			name: "replace values",
			values: map[string]string{
				"key1": "value1-{{id}}",
				"key2": "static",
			},
			record: map[string]string{"id": "123"},
			want: map[string]string{
				"key1": "value1-123",
				"key2": "static",
			},
		},
		{
			name: "no record",
			values: map[string]string{
				"key1": "value1-{{id}}",
			},
			record: nil,
			want: map[string]string{
				"key1": "value1-{{id}}",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyPlaceholdersToMap(tt.values, tt.record)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("applyPlaceholdersToMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContextWithVariableStore(t *testing.T) {
	store := variables.NewStore()
	store.Set("key", "value")

	ctx := context.Background()
	ctxWithStore := variables.NewContext(ctx, store)

	retrieved := variables.FromContext(ctxWithStore)
	if retrieved == nil {
		t.Fatalf("variableStoreFromContext() returned nil")
	}

	val, ok := retrieved.Get("key")
	if !ok || val != "value" {
		t.Errorf("retrieved store has wrong value: got %q, want %q", val, "value")
	}
}

func TestVariableStoreFromContext(t *testing.T) {
	// Test with context without store
	ctx := context.TODO()
	got := variables.FromContext(ctx)
	if got != nil {
		t.Errorf("variableStoreFromContext(no store) = %v, want nil", got)
	}

	// Test with context with store
	store := variables.NewStore()
	ctx = variables.NewContext(ctx, store)
	got = variables.FromContext(ctx)
	if got != store {
		t.Errorf("variableStoreFromContext(with store) returned wrong store")
	}
}

func TestApplyPlaceholders_WithVariables(t *testing.T) {
	tests := []struct {
		name     string
		template string
		record   map[string]string
		store    variables.Store
		want     string
	}{
		{
			name:     "variable overrides record",
			template: "{{user_id}}",
			record:   map[string]string{"user_id": "from-record"},
			store:    newStoreWithValues(map[string]string{"user_id": "from-store"}),
			want:     "from-store",
		},
		{
			name:     "fallback to record when variable missing",
			template: "{{user_id}}",
			record:   map[string]string{"user_id": "from-record"},
			store:    variables.NewStore(),
			want:     "from-record",
		},
		{
			name:     "variable in empty record",
			template: "{{user_id}}",
			record:   map[string]string{},
			store:    newStoreWithValues(map[string]string{"user_id": "from-store"}),
			want:     "from-store",
		},
		{
			name:     "multiple placeholders mixed sources",
			template: "user={{user_id}}, name={{name}}",
			record:   map[string]string{"user_id": "123", "name": "alice"},
			store:    newStoreWithValues(map[string]string{"user_id": "999"}),
			want:     "user=999, name=alice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyPlaceholders(tt.template, tt.record, tt.store)
			if got != tt.want {
				t.Errorf("applyPlaceholdersWithVariables() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestApplyPlaceholders_DefaultValue(t *testing.T) {
	tests := []struct {
		name     string
		template string
		record   map[string]string
		store    variables.Store
		want     string
	}{
		{
			name:     "default value when missing",
			template: "{{user_id|anonymous}}",
			record:   map[string]string{},
			store:    variables.NewStore(),
			want:     "anonymous",
		},
		{
			name:     "empty default when missing",
			template: "{{token|}}",
			record:   map[string]string{},
			store:    variables.NewStore(),
			want:     "",
		},
		{
			name:     "default not used when variable present",
			template: "{{user_id|anonymous}}",
			record:   map[string]string{},
			store:    newStoreWithValues(map[string]string{"user_id": "123"}),
			want:     "123",
		},
		{
			name:     "default not used when in record",
			template: "{{user_id|anonymous}}",
			record:   map[string]string{"user_id": "456"},
			store:    variables.NewStore(),
			want:     "456",
		},
		{
			name:     "default with variable override",
			template: "{{user_id|default_user}}",
			record:   map[string]string{"user_id": "from-record"},
			store:    newStoreWithValues(map[string]string{"user_id": "from-store"}),
			want:     "from-store",
		},
		{
			name:     "multiple placeholders with defaults",
			template: "user={{uid|unknown}}, token={{tok|none}}",
			record:   map[string]string{"uid": "123"},
			store:    variables.NewStore(),
			want:     "user=123, token=none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyPlaceholders(tt.template, tt.record, tt.store)
			if got != tt.want {
				t.Errorf("applyPlaceholdersWithVariables() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestApplyPlaceholders_EmptyDefault(t *testing.T) {
	template := "prefix{{value|}}suffix"
	record := map[string]string{}
	store := variables.NewStore()

	got := applyPlaceholders(template, record, store)
	expected := "prefixsuffix"

	if got != expected {
		t.Errorf("applyPlaceholdersWithVariables() = %q, want %q", got, expected)
	}
}

func TestApplyPlaceholders_VariablePriority(t *testing.T) {
	tests := []struct {
		name     string
		template string
		record   map[string]string
		store    variables.Store
		want     string
	}{
		{
			name:     "priority 1: variable store",
			template: "{{value}}",
			record:   map[string]string{"value": "from-record"},
			store:    newStoreWithValues(map[string]string{"value": "from-store"}),
			want:     "from-store",
		},
		{
			name:     "priority 2: feeder record",
			template: "{{value}}",
			record:   map[string]string{"value": "from-record"},
			store:    variables.NewStore(),
			want:     "from-record",
		},
		{
			name:     "priority 3: default",
			template: "{{value|default-val}}",
			record:   map[string]string{},
			store:    variables.NewStore(),
			want:     "default-val",
		},
		{
			name:     "no match: keep placeholder",
			template: "{{value|}}",
			record:   map[string]string{},
			store:    variables.NewStore(),
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyPlaceholders(tt.template, tt.record, tt.store)
			if got != tt.want {
				t.Errorf("applyPlaceholdersWithVariables() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestApplyPlaceholdersToMapWithVariables(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
		record map[string]string
		store  variables.Store
		want   map[string]string
	}{
		{
			name: "apply variables and defaults",
			values: map[string]string{
				"url":     "http://api.example.com/users/{{id}}",
				"header":  "Bearer {{token|none}}",
				"default": "{{missing|default-val}}",
			},
			record: map[string]string{"id": "123"},
			store:  newStoreWithValues(map[string]string{}),
			want: map[string]string{
				"url":     "http://api.example.com/users/123",
				"header":  "Bearer none",
				"default": "default-val",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyPlaceholdersToMap(tt.values, tt.record, tt.store)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("applyPlaceholdersToMapWithVariables() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper function for tests
func newStoreWithValues(values map[string]string) variables.Store {
	store := variables.NewStore()
	for k, v := range values {
		store.Set(k, v)
	}
	return store
}
