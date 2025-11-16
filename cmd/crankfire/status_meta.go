package main

import (
	"fmt"
	"strings"

	"github.com/torosent/crankfire/internal/metrics"
)

func ensureProtocolMeta(meta *metrics.RequestMetadata, protocol string) *metrics.RequestMetadata {
	if meta == nil {
		meta = &metrics.RequestMetadata{}
	}
	if protocol != "" && meta.Protocol == "" {
		meta.Protocol = protocol
	}
	return meta
}

func annotateStatus(meta *metrics.RequestMetadata, protocol, status string) *metrics.RequestMetadata {
	meta = ensureProtocolMeta(meta, protocol)
	meta.StatusCode = sanitizeStatusCode(status)
	return meta
}

func sanitizeStatusCode(status string) string {
	trimmed := strings.TrimSpace(status)
	if trimmed == "" {
		return "UNKNOWN"
	}
	replacer := strings.NewReplacer(" ", "_", "/", "_", ".", "_", "-", "_")
	normalized := replacer.Replace(trimmed)
	normalized = strings.ToUpper(normalized)
	normalized = strings.Trim(normalized, "_")
	if normalized == "" {
		return "UNKNOWN"
	}
	return normalized
}

func fallbackStatusCode(err error) string {
	if err == nil {
		return ""
	}
	typeName := fmt.Sprintf("%T", err)
	typeName = strings.TrimPrefix(typeName, "*")
	if idx := strings.LastIndex(typeName, "/"); idx != -1 {
		typeName = typeName[idx+1:]
	}
	if idx := strings.LastIndex(typeName, "."); idx != -1 {
		typeName = typeName[idx+1:]
	}
	return sanitizeStatusCode(typeName)
}
