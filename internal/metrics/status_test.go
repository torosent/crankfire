package metrics

import (
	"reflect"
	"testing"
)

func TestFlattenStatusBuckets(t *testing.T) {
	tests := []struct {
		name    string
		buckets map[string]map[string]int
		want    []StatusBucket
	}{
		{
			name:    "nil buckets",
			buckets: nil,
			want:    nil,
		},
		{
			name:    "empty buckets",
			buckets: map[string]map[string]int{},
			want:    nil,
		},
		{
			name: "single bucket",
			buckets: map[string]map[string]int{
				"http": {"200": 10},
			},
			want: []StatusBucket{
				{Protocol: "http", Code: "200", Count: 10},
			},
		},
		{
			name: "multiple buckets sorted by count desc",
			buckets: map[string]map[string]int{
				"http": {
					"200": 10,
					"500": 5,
				},
				"grpc": {
					"OK": 20,
				},
			},
			want: []StatusBucket{
				{Protocol: "grpc", Code: "OK", Count: 20},
				{Protocol: "http", Code: "200", Count: 10},
				{Protocol: "http", Code: "500", Count: 5},
			},
		},
		{
			name: "tie breaking by protocol then code",
			buckets: map[string]map[string]int{
				"http": {
					"200": 10,
					"404": 10,
				},
				"grpc": {
					"OK": 10,
				},
			},
			want: []StatusBucket{
				{Protocol: "grpc", Code: "OK", Count: 10},
				{Protocol: "http", Code: "200", Count: 10},
				{Protocol: "http", Code: "404", Count: 10},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FlattenStatusBuckets(tt.buckets)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FlattenStatusBuckets() = %v, want %v", got, tt.want)
			}
		})
	}
}
