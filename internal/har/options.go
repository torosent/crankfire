package har

// ConvertOptions provides configuration for converting HAR entries to Endpoints.
type ConvertOptions struct {
	// IncludeHosts specifies which hosts to include (empty = all hosts)
	IncludeHosts []string
	// ExcludeHosts specifies which hosts to exclude
	ExcludeHosts []string
	// IncludeMethods specifies which HTTP methods to include (empty = all methods)
	IncludeMethods []string
	// ExcludeStatic determines whether to exclude static assets (.js, .css, images, fonts)
	ExcludeStatic bool
	// IncludeHeaders determines whether to include request headers in endpoints
	IncludeHeaders bool
}

// DefaultOptions returns ConvertOptions with sensible defaults.
func DefaultOptions() ConvertOptions {
	return ConvertOptions{
		ExcludeStatic:  true,
		IncludeHeaders: true,
	}
}
