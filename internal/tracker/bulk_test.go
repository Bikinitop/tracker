package tracker

import (
	"testing"
)

func TestParseBulkRequest(t *testing.T) {
	jsonBody := `{
		"requests": [
			"?idsite=1&rec=1&url=https://example.com",
			"?idsite=1&rec=1&url=https://example.net"
		],
		"token_auth": "abc123"
	}`

	bulk, err := ParseBulkRequest([]byte(jsonBody))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(bulk.Requests) != 2 {
		t.Errorf("expected 2 requests, got %d", len(bulk.Requests))
	}
	if bulk.TokenAuth != "abc123" {
		t.Errorf("expected token_auth abc123, got %s", bulk.TokenAuth)
	}
}

func TestParseBulkRequest_Empty(t *testing.T) {
	jsonBody := `{"requests": []}`
	_, err := ParseBulkRequest([]byte(jsonBody))
	if err == nil {
		t.Fatal("expected error for empty requests")
	}
}

func TestParseBulkRequest_InvalidJSON(t *testing.T) {
	jsonBody := `not json`
	_, err := ParseBulkRequest([]byte(jsonBody))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestExtractParamsFromQueryString(t *testing.T) {
	params, err := ExtractParamsFromQueryString("?idsite=1&rec=1&action_name=Test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if params["idsite"] != "1" {
		t.Errorf("expected idsite 1, got %s", params["idsite"])
	}
	if params["action_name"] != "Test" {
		t.Errorf("expected action_name Test, got %s", params["action_name"])
	}
}

func TestExtractParamsFromQueryString_NoLeadingQuestion(t *testing.T) {
	params, err := ExtractParamsFromQueryString("idsite=1&rec=1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if params["idsite"] != "1" {
		t.Errorf("expected idsite 1, got %s", params["idsite"])
	}
}
