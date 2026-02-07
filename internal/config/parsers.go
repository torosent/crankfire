// Package config provides configuration loading and parsing for crankfire.
package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// lookupSetting searches for a value in settings using multiple candidate keys.
// It performs case-insensitive matching by also checking lowercase versions.
func lookupSetting(settings map[string]interface{}, candidates ...string) (interface{}, bool) {
	for _, key := range candidates {
		if val, ok := settings[key]; ok {
			return val, true
		}
		lower := strings.ToLower(key)
		if val, ok := settings[lower]; ok {
			return val, true
		}
	}
	return nil, false
}

// asString converts an interface value to a string.
// Handles nil, string, fmt.Stringer, []byte, and falls back to fmt.Sprint.
func asString(value interface{}) (string, error) {
	switch v := value.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case fmt.Stringer:
		return v.String(), nil
	case []byte:
		return string(v), nil
	default:
		return fmt.Sprint(v), nil
	}
}

// asInt converts an interface value to an int.
// Handles all numeric types and string representations.
func asInt(value interface{}) (int, error) {
	switch v := value.(type) {
	case nil:
		return 0, nil
	case int:
		return v, nil
	case int8:
		return int(v), nil
	case int16:
		return int(v), nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case uint:
		return int(v), nil
	case uint8:
		return int(v), nil
	case uint16:
		return int(v), nil
	case uint32:
		return int(v), nil
	case uint64:
		return int(v), nil
	case float32:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		if strings.TrimSpace(v) == "" {
			return 0, nil
		}
		i, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0, err
		}
		return i, nil
	default:
		return 0, fmt.Errorf("unsupported numeric type %T", value)
	}
}

// asFloat64 converts an interface value to a float64.
// Handles all numeric types and string representations.
func asFloat64(value interface{}) (float64, error) {
	switch v := value.(type) {
	case nil:
		return 0, nil
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		if strings.TrimSpace(v) == "" {
			return 0, nil
		}
		return strconv.ParseFloat(strings.TrimSpace(v), 64)
	default:
		return 0, fmt.Errorf("unsupported float type %T", value)
	}
}

// asBool converts an interface value to a bool.
// Handles bool and string representations.
func asBool(value interface{}) (bool, error) {
	switch v := value.(type) {
	case nil:
		return false, nil
	case bool:
		return v, nil
	case string:
		if strings.TrimSpace(v) == "" {
			return false, nil
		}
		b, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return false, err
		}
		return b, nil
	default:
		return false, fmt.Errorf("unsupported boolean type %T", value)
	}
}

// asDuration converts an interface value to a time.Duration.
// Handles time.Duration, string (parsed via time.ParseDuration), and numeric types
// (interpreted as seconds).
func asDuration(value interface{}) (time.Duration, error) {
	switch v := value.(type) {
	case nil:
		return 0, nil
	case time.Duration:
		return v, nil
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return 0, nil
		}
		d, err := time.ParseDuration(v)
		if err != nil {
			return 0, err
		}
		return d, nil
	case int, int8, int16, int32, int64:
		iv, _ := asInt(v)
		return time.Duration(iv) * time.Second, nil
	case uint, uint8, uint16, uint32, uint64:
		iv, _ := asInt(v)
		return time.Duration(iv) * time.Second, nil
	case float32, float64:
		iv, _ := asInt(v)
		return time.Duration(iv) * time.Second, nil
	default:
		return 0, fmt.Errorf("unsupported duration type %T", value)
	}
}

// asStringMap converts an interface value to a map[string]string.
// Handles map[string]string, map[string]interface{}, and map[interface{}]interface{}.
func asStringMap(value interface{}) (map[string]string, error) {
	if value == nil {
		return nil, nil
	}
	switch v := value.(type) {
	case map[string]string:
		result := make(map[string]string, len(v))
		for k, val := range v {
			result[k] = val
		}
		return result, nil
	case map[string]interface{}:
		result := make(map[string]string, len(v))
		for k, val := range v {
			str, err := asString(val)
			if err != nil {
				return nil, err
			}
			result[k] = str
		}
		return result, nil
	case map[interface{}]interface{}:
		result := make(map[string]string, len(v))
		for k, val := range v {
			key, err := asString(k)
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(key) == "" {
				return nil, fmt.Errorf("header key cannot be empty")
			}
			str, err := asString(val)
			if err != nil {
				return nil, err
			}
			result[key] = str
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported headers type %T", value)
	}
}

// asStringSlice converts an interface value to a []string.
// Handles []string, []interface{}, and single string values.
func asStringSlice(value interface{}) ([]string, error) {
	if value == nil {
		return nil, nil
	}
	switch v := value.(type) {
	case []string:
		return v, nil
	case []interface{}:
		result := make([]string, len(v))
		for i, item := range v {
			str, err := asString(item)
			if err != nil {
				return nil, fmt.Errorf("index %d: %w", i, err)
			}
			result[i] = str
		}
		return result, nil
	case string:
		return []string{v}, nil
	default:
		return nil, fmt.Errorf("unsupported string slice type %T", value)
	}
}

// toInterfaceSlice converts various slice types to []interface{}.
func toInterfaceSlice(value interface{}) ([]interface{}, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case []interface{}:
		return v, nil
	case []map[string]interface{}:
		items := make([]interface{}, len(v))
		for i := range v {
			items[i] = v[i]
		}
		return items, nil
	case []map[interface{}]interface{}:
		items := make([]interface{}, len(v))
		for i := range v {
			items[i] = v[i]
		}
		return items, nil
	default:
		return nil, fmt.Errorf("expected list, got %T", value)
	}
}

// toStringKeyMap converts a map with various key types to map[string]interface{}.
// Keys are normalized to lowercase.
func toStringKeyMap(value interface{}) (map[string]interface{}, error) {
	result := map[string]interface{}{}
	switch v := value.(type) {
	case map[string]interface{}:
		for key, val := range v {
			result[strings.ToLower(strings.TrimSpace(key))] = val
		}
	case map[interface{}]interface{}:
		for key, val := range v {
			str, err := asString(key)
			if err != nil {
				return nil, err
			}
			result[strings.ToLower(strings.TrimSpace(str))] = val
		}
	default:
		return nil, fmt.Errorf("expected map, got %T", value)
	}
	return result, nil
}
