package nodes

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	maxSQLParameters      = 1000
	maxSQLParametersBytes = 1 << 20
)

// parseSQLParameters accepts either a structured array (preferred) or a JSON
// array string from the visual editor. Values are passed separately to the
// database driver instead of being interpolated into the SQL statement.
func parseSQLParameters(raw interface{}) ([]interface{}, error) {
	if raw == nil {
		return nil, nil
	}

	var params []interface{}
	switch typed := raw.(type) {
	case []interface{}:
		params = append([]interface{}(nil), typed...)
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return nil, nil
		}
		if len(text) > maxSQLParametersBytes {
			return nil, fmt.Errorf("SQL parameters exceed %d byte limit", maxSQLParametersBytes)
		}
		if err := json.Unmarshal([]byte(text), &params); err != nil {
			return nil, fmt.Errorf("SQL parameters must be a JSON array: %w", err)
		}
	default:
		return nil, fmt.Errorf("SQL parameters must be an array")
	}

	if len(params) > maxSQLParameters {
		return nil, fmt.Errorf("SQL parameters contain %d values; maximum is %d", len(params), maxSQLParameters)
	}
	encoded, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("SQL parameters could not be encoded: %w", err)
	}
	if len(encoded) > maxSQLParametersBytes {
		return nil, fmt.Errorf("SQL parameters exceed %d byte limit", maxSQLParametersBytes)
	}
	return params, nil
}
