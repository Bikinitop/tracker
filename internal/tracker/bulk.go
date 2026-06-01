package tracker

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// BulkRequest represents a bulk tracking payload
type BulkRequest struct {
	Requests  []string `json:"requests"`
	TokenAuth string   `json:"token_auth,omitempty"`
}

// ParseBulkRequest parses a JSON bulk tracking request
func ParseBulkRequest(body []byte) (*BulkRequest, error) {
	var bulk BulkRequest
	if err := json.Unmarshal(body, &bulk); err != nil {
		return nil, fmt.Errorf("invalid bulk request JSON: %w", err)
	}

	if len(bulk.Requests) == 0 {
		return nil, fmt.Errorf("bulk request must contain at least one request")
	}

	return &bulk, nil
}

// ExtractParamsFromQueryString parses a single request string (e.g., "?idsite=1&rec=1")
func ExtractParamsFromQueryString(queryString string) (map[string]string, error) {
	// Remove leading ? if present
	queryString = strings.TrimPrefix(queryString, "?")

	values, err := url.ParseQuery(queryString)
	if err != nil {
		return nil, fmt.Errorf("invalid query string: %w", err)
	}

	params := make(map[string]string)
	for key, vals := range values {
		if len(vals) > 0 {
			params[key] = vals[0]
		}
	}

	return params, nil
}
