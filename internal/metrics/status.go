package metrics

import "sort"

// StatusBucket represents the aggregated failure count for a protocol/code pair.
type StatusBucket struct {
	Protocol string
	Code     string
	Count    int
}

// FlattenStatusBuckets converts a nested protocol->status map into a sorted slice of StatusBucket rows.
// Rows are sorted by descending count, then by protocol/code for stability.
func FlattenStatusBuckets(buckets map[string]map[string]int) []StatusBucket {
	if len(buckets) == 0 {
		return nil
	}
	rows := make([]StatusBucket, 0)
	for protocol, codes := range buckets {
		for code, count := range codes {
			rows = append(rows, StatusBucket{Protocol: protocol, Code: code, Count: count})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Count == rows[j].Count {
			if rows[i].Protocol == rows[j].Protocol {
				return rows[i].Code < rows[j].Code
			}
			return rows[i].Protocol < rows[j].Protocol
		}
		return rows[i].Count > rows[j].Count
	})
	return rows
}
