package widgets

import (
	"fmt"
	"sort"
	"strings"
)

// StatusBucketsTable renders the top maxRows rows from a protocol→code→count
// map, sorted by descending count. Returns "" if buckets is empty.
func StatusBucketsTable(buckets map[string]map[string]int, maxRows int) string {
	type row struct {
		protocol string
		code     string
		count    int
	}
	var rows []row
	for proto, codes := range buckets {
		for code, count := range codes {
			rows = append(rows, row{protocol: proto, code: code, count: count})
		}
	}
	if len(rows) == 0 {
		return ""
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].count != rows[j].count {
			return rows[i].count > rows[j].count
		}
		if rows[i].protocol != rows[j].protocol {
			return rows[i].protocol < rows[j].protocol
		}
		return rows[i].code < rows[j].code
	})
	if maxRows > 0 && len(rows) > maxRows {
		rows = rows[:maxRows]
	}
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "%s %s %d\n", strings.ToUpper(r.protocol), r.code, r.count)
	}
	return b.String()
}
