package cli

import "testing"

func TestParseHARFilterToOptions(t *testing.T) {
	opts, err := parseHARFilterToOptions("host:a.example.com, b.example.com;method:GET")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts.IncludeHosts) != 2 || opts.IncludeHosts[1] != "b.example.com" || len(opts.IncludeMethods) != 1 {
		t.Errorf("unexpected options: %+v", opts)
	}

	// A typo must not silently mean "no filter": that replays every recorded host.
	for _, bad := range []string{"hosts:example.com", "example.com"} {
		if _, err := parseHARFilterToOptions(bad); err == nil {
			t.Errorf("filter %q: expected an error", bad)
		}
	}
}
