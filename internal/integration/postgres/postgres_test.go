package postgres

import (
	"testing"
)

func TestQueryDatabases(t *testing.T) {
	client := New()

	// Test with invalid connection string
	_, err := client.QueryDatabases(PG15, "invalid-connection-string")
	if err == nil {
		t.Error("Expected error for invalid connection string, got nil")
	}
}
