package ingestion

import (
	"context"
	"strings"
	"testing"
)

func TestFetchURLRejectsLoopback(t *testing.T) {
	_, _, _, err := FetchURL(context.Background(), "http://127.0.0.1/private")
	if err == nil || !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("expected SSRF rejection, got %v", err)
	}
}
