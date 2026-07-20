package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/reinh2/docmind/internal/metrics"
	"github.com/reinh2/docmind/internal/platform/ratelimit"
)

// newTestServer builds a Server whose store, ingestion, and chat dependencies are nil.
// The exercised routes (auth rejection, request validation, middleware, static assets)
// all return before touching those collaborators, so a database is not required.
func newTestServer(t *testing.T, limit int) http.Handler {
	t.Helper()
	limiter := ratelimit.New(limit, time.Minute)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(nil, nil, nil, "secret-token", "*", metrics.New(), limiter, log).Handler()
}

func TestAdminRoutesRejectMissingToken(t *testing.T) {
	handler := newTestServer(t, 100)
	for _, path := range []string{"/v1/admin/documents", "/v1/admin/dashboard", "/v1/admin/conversations"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s without token: got %d, want 401", path, rec.Code)
		}
	}
}

func TestAdminRoutesRejectWrongToken(t *testing.T) {
	handler := newTestServer(t, 100)
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/documents", nil)
	req.Header.Set("Authorization", "Bearer not-the-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token: got %d, want 401", rec.Code)
	}
}

func TestChatRejectsEmptyQuestion(t *testing.T) {
	handler := newTestServer(t, 100)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/messages", strings.NewReader(`{"question":"   "}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty question: got %d, want 400", rec.Code)
	}
}

func TestChatRejectsOversizedQuestion(t *testing.T) {
	handler := newTestServer(t, 100)
	body := `{"question":"` + strings.Repeat("a", maxQuestion+1) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/messages", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("oversized question: got %d, want 400", rec.Code)
	}
}

func TestChatRejectsInvalidJSON(t *testing.T) {
	handler := newTestServer(t, 100)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/messages", strings.NewReader("{not json"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid JSON: got %d, want 400", rec.Code)
	}
}

func TestIPRateLimitReturns429(t *testing.T) {
	handler := newTestServer(t, 1)
	// The first request consumes the single IP token; the second must be rejected before
	// reaching any handler. An invalid body keeps the first request off the database path.
	first := httptest.NewRequest(http.MethodPost, "/v1/chat/messages", strings.NewReader("{bad"))
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, first)

	second := httptest.NewRequest(http.MethodPost, "/v1/chat/messages", strings.NewReader("{bad"))
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, second)
	if secondRec.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: got %d, want 429", secondRec.Code)
	}
}

func TestHealthEndpointsBypassRateLimit(t *testing.T) {
	handler := newTestServer(t, 1)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("healthz call %d: got %d, want 200", i, rec.Code)
		}
	}
}

func TestCORSPreflightIsAnswered(t *testing.T) {
	handler := newTestServer(t, 100)
	req := httptest.NewRequest(http.MethodOptions, "/v1/chat/messages", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight: got %d, want 204", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("missing CORS allow-origin header: %v", rec.Header())
	}
}

func TestWidgetIsServedAsJavaScript(t *testing.T) {
	handler := newTestServer(t, 100)
	req := httptest.NewRequest(http.MethodGet, "/docmind-widget.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("widget: got %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "javascript") {
		t.Fatalf("widget content-type = %q", rec.Header().Get("Content-Type"))
	}
	if !strings.Contains(rec.Body.String(), "DocMindWidget") {
		t.Fatal("widget body does not contain the DocMindWidget global")
	}
}
