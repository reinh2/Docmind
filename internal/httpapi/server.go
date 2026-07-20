package httpapi

import (
	"crypto/subtle"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/reinh2/docmind/internal/chat"
	"github.com/reinh2/docmind/internal/domain"
	"github.com/reinh2/docmind/internal/ingestion"
	"github.com/reinh2/docmind/internal/metrics"
	"github.com/reinh2/docmind/internal/platform/id"
	"github.com/reinh2/docmind/internal/platform/ratelimit"
	"github.com/reinh2/docmind/internal/store/postgres"
)

const maxUpload = 5 << 20
const maxQuestion = 4000

//go:embed assets/docmind-widget.js
var widgetScript []byte

type Server struct {
	store       *postgres.Store
	ingestion   *ingestion.Service
	chat        *chat.Service
	adminToken  string
	log         *slog.Logger
	metrics     *metrics.Registry
	limiter     *ratelimit.Limiter
	corsOrigins map[string]bool
}

func New(store *postgres.Store, ingestion *ingestion.Service, chat *chat.Service, token, origins string, registry *metrics.Registry, limiter *ratelimit.Limiter, log *slog.Logger) *Server {
	allowed := map[string]bool{}
	for _, origin := range strings.Split(origins, ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			allowed[origin] = true
		}
	}
	return &Server{store: store, ingestion: ingestion, chat: chat, adminToken: token, log: log, metrics: registry, limiter: limiter, corsOrigins: allowed}
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("GET /metrics", s.metricsHandler)
	mux.HandleFunc("GET /docmind-widget.js", s.widget)
	mux.HandleFunc("POST /v1/admin/documents", s.upload)
	mux.HandleFunc("GET /v1/admin/documents", s.documents)
	mux.HandleFunc("DELETE /v1/admin/documents/{id}", s.deleteDocument)
	mux.HandleFunc("POST /v1/admin/documents/{id}/reindex", s.reindex)
	mux.HandleFunc("POST /v1/admin/documents/url", s.uploadURL)
	mux.HandleFunc("GET /v1/admin/conversations", s.conversations)
	mux.HandleFunc("GET /v1/admin/conversations/{id}", s.conversation)
	mux.HandleFunc("GET /v1/admin/dashboard", s.dashboard)
	mux.HandleFunc("GET /v1/admin/jobs/{id}", s.job)
	mux.HandleFunc("POST /v1/chat/messages", s.message)
	mux.HandleFunc("POST /v1/chat/messages/{id}/feedback", s.feedback)
	mux.HandleFunc("POST /v1/chat/conversations/{id}/escalate", s.escalate)
	return s.recover(s.cors(s.rateLimit(s.logRequest(mux))))
}
func (s *Server) widget(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(widgetScript)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database is not ready")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
func (s *Server) metricsHandler(w http.ResponseWriter, r *http.Request) {
	s.metrics.Set("docmind_ingestion_queue_depth", float64(s.ingestion.QueueDepth()))
	s.metrics.Handler(w, r)
}
func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "admin token required")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid upload")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()
	body, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "cannot read file")
		return
	}
	sourceType := r.FormValue("source_type")
	if sourceType == "" {
		sourceType = strings.TrimPrefix(filepath.Ext(header.Filename), ".")
	}
	title := r.FormValue("title")
	if title == "" {
		title = header.Filename
	}
	doc, job, duplicate, err := s.ingestion.Submit(r.Context(), ingestion.Request{Title: title, SourceType: sourceType, Content: string(body)}, id.New)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "queue is full") {
			status = http.StatusServiceUnavailable
		}
		writeError(w, status, err.Error())
		return
	}
	if duplicate {
		writeJSON(w, http.StatusOK, map[string]any{"document": doc, "duplicate": true})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"document": doc, "job": job, "duplicate": false})
}
func (s *Server) documents(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "admin token required")
		return
	}
	docs, err := s.store.Documents(r.Context())
	if err != nil {
		writeError(w, 500, "cannot list documents")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"documents": docs})
}
func (s *Server) deleteDocument(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "admin token required")
		return
	}
	if err := s.store.DeleteDocument(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) reindex(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "admin token required")
		return
	}
	job, err := s.ingestion.Reindex(r.Context(), r.PathValue("id"), id.New)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job": job})
}
func (s *Server) uploadURL(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "admin token required")
		return
	}
	var req struct {
		URL   string `json:"url"`
		Title string `json:"title"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}
	content, sourceType, title, err := ingestion.FetchURL(r.Context(), strings.TrimSpace(req.URL))
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if strings.TrimSpace(req.Title) != "" {
		title = strings.TrimSpace(req.Title)
	}
	doc, job, duplicate, err := s.ingestion.Submit(r.Context(), ingestion.Request{Title: title, SourceType: sourceType, Content: content}, id.New)
	if err != nil {
		writeError(w, 503, err.Error())
		return
	}
	status := http.StatusAccepted
	if duplicate {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{"document": doc, "job": job, "duplicate": duplicate})
}
func (s *Server) job(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeError(w, http.StatusUnauthorized, "admin token required")
		return
	}
	job, err := s.store.Job(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	writeJSON(w, http.StatusOK, job)
}
func (s *Server) conversations(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeError(w, 401, "admin token required")
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if _, err := fmt.Sscanf(raw, "%d", &limit); err != nil {
			writeError(w, 400, "invalid limit")
			return
		}
	}
	items, err := s.store.Conversations(r.Context(), r.URL.Query().Get("channel"), limit)
	if err != nil {
		writeError(w, 500, "cannot list conversations")
		return
	}
	writeJSON(w, 200, map[string]any{"conversations": items})
}
func (s *Server) conversation(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeError(w, 401, "admin token required")
		return
	}
	item, messages, err := s.store.Conversation(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, 404, "conversation not found")
		return
	}
	writeJSON(w, 200, map[string]any{"conversation": item, "messages": messages})
}
func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeError(w, 401, "admin token required")
		return
	}
	data, err := s.store.Dashboard(r.Context())
	if err != nil {
		writeError(w, 500, "cannot build dashboard")
		return
	}
	writeJSON(w, 200, data)
}

type questionRequest struct {
	SessionID string `json:"session_id"`
	Channel   string `json:"channel"`
	Question  string `json:"question"`
}

func (s *Server) message(w http.ResponseWriter, r *http.Request) {
	var req questionRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxQuestion+1024)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	req.Question = strings.TrimSpace(req.Question)
	if req.Question == "" || len(req.Question) > maxQuestion {
		writeError(w, http.StatusBadRequest, "question must be 1–4000 characters")
		return
	}
	if req.SessionID == "" {
		req.SessionID = id.New()
	}
	if !s.limiter.Allow("session:" + req.SessionID) {
		writeError(w, http.StatusTooManyRequests, "session rate limit exceeded")
		return
	}
	if req.Channel == "" {
		req.Channel = "widget"
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unavailable")
		return
	}
	// Once the first event is flushed the status line is already 200, so a later
	// generation error can only end the stream with an "error" event, not change the code.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	stream := &chat.Stream{
		OnSources: func(sources []domain.Source) {
			writeEvent(w, "sources", sources)
			flusher.Flush()
		},
		OnToken: func(text string) {
			if text == "" {
				return
			}
			writeEvent(w, "token", map[string]string{"text": text})
			flusher.Flush()
		},
	}
	result, err := s.chat.Ask(r.Context(), req.SessionID, req.Channel, req.Question, stream)
	if err != nil {
		s.log.Error("chat failed", "error", err)
		writeEvent(w, "error", map[string]string{"message": "I couldn't process that right now. Please try again or contact a human."})
		flusher.Flush()
		return
	}
	writeEvent(w, "complete", map[string]any{"conversation_id": result.ConversationID, "message_id": result.MessageID, "not_found": result.NotFound})
	flusher.Flush()
}
func (s *Server) feedback(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Rating  int    `json:"rating"`
		Comment string `json:"comment"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&req); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}
	if len(req.Comment) > 1000 {
		writeError(w, 400, "comment is too long")
		return
	}
	if err := s.chat.Feedback(r.Context(), r.PathValue("id"), req.Rating, strings.TrimSpace(req.Comment)); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) escalate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&req); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "customer requested a human"
	}
	if len(reason) > 500 {
		writeError(w, 400, "reason is too long")
		return
	}
	if err := s.chat.Escalate(r.Context(), r.PathValue("id"), reason); err != nil {
		writeError(w, 503, "could not request an operator")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "operator_notified"})
}
func (s *Server) authorized(r *http.Request) bool {
	auth := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return auth != "" && subtle.ConstantTimeCompare([]byte(auth), []byte(s.adminToken)) == 1
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
func writeEvent(w http.ResponseWriter, event string, payload any) {
	b, _ := json.Marshal(payload)
	_, _ = w.Write([]byte("event: " + event + "\ndata: " + string(b) + "\n\n"))
}
func (s *Server) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.log.Info("http request", "method", r.Method, "path", r.URL.Path, "latency_ms", time.Since(start).Milliseconds())
	})
}
func (s *Server) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" || r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		if !s.limiter.Allow("ip:" + host) {
			writeError(w, http.StatusTooManyRequests, "IP rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Server) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				s.log.Error("panic", "error", v)
				writeError(w, 500, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if s.corsOrigins["*"] {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if s.corsOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
