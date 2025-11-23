package main

import (
	"reflect"
	"testing"
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
