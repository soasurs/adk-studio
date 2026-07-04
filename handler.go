package studio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/soasurs/adk/runner"
	"github.com/soasurs/adk/session"
)

type Handler struct {
	app       *App
	startedAt time.Time
	ui        http.Handler
	uiFS      fs.FS
}

func NewHandler(app *App) http.Handler {
	if app == nil {
		app = NewApp(AppConfig{})
	}
	uiFS, err := fs.Sub(uiFiles, "frontend/dist")
	if err != nil {
		panic(err)
	}

	h := &Handler{
		app:       app,
		startedAt: time.Now(),
		ui:        http.FileServer(http.FS(uiFS)),
		uiFS:      uiFS,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", h.handleHealth)
	mux.HandleFunc("GET /api/app", h.handleApp)
	mux.HandleFunc("GET /api/agents", h.handleAgents)
	mux.HandleFunc("GET /api/agents/{agent_id}", h.handleAgent)
	mux.HandleFunc("POST /api/runs", h.handleCreateRun)
	mux.HandleFunc("/", h.handleUI)
	return mux
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"started_at": h.startedAt.Format(time.RFC3339),
	})
}

func (h *Handler) handleApp(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"name":                h.app.Name(),
		"agent_count":         len(h.app.agentSummaries()),
		"has_session_service": h.app.hasSessionService(),
	})
}

func (h *Handler) handleAgents(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"agents": h.app.agentSummaries(),
	})
}

func (h *Handler) handleAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("agent_id")
	agent, ok := h.app.agentSummary(id)
	if !ok {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	writeJSON(w, http.StatusOK, agent)
}

func (h *Handler) handleCreateRun(w http.ResponseWriter, r *http.Request) {
	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.AgentID = strings.TrimSpace(req.AgentID)
	if req.AgentID == "" {
		writeError(w, http.StatusBadRequest, "agent_id is required")
		return
	}
	if req.UserID == "" {
		req.UserID = "local-user"
	}
	if req.AppName == "" {
		req.AppName = h.app.Name()
	}
	if req.SessionID == "" {
		req.SessionID = newSessionID()
	}
	if req.Input.Content == "" && len(req.Input.Parts) == 0 {
		writeError(w, http.StatusBadRequest, "input content or parts are required")
		return
	}

	agent, ok := h.app.agent(req.AgentID)
	if !ok {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	sessionService := h.app.sessionService()
	if sessionService == nil {
		writeError(w, http.StatusFailedDependency, "session service is not configured")
		return
	}

	if err := ensureSession(r.Context(), sessionService, req); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	adkRunner, err := runner.New(agent, sessionService)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	runID := newRunID()
	events := make([]RunStreamEvent, 0)
	for event, err := range adkRunner.Run(r.Context(), req.SessionID, req.Input) {
		if err != nil {
			errMsg := err.Error()
			writeJSON(w, http.StatusInternalServerError, RunResponse{
				RunID:     runID,
				SessionID: req.SessionID,
				Error:     errMsg,
				Events: append(events, RunStreamEvent{
					Type:      "error",
					RunID:     runID,
					SessionID: req.SessionID,
					Error:     errMsg,
				}),
			})
			return
		}
		events = append(events, RunStreamEvent{
			Type:      "event",
			RunID:     runID,
			SessionID: req.SessionID,
			Event:     event,
		})
	}

	writeJSON(w, http.StatusOK, RunResponse{
		RunID:     runID,
		SessionID: req.SessionID,
		Events:    events,
	})
}

func ensureSession(ctx context.Context, service session.SessionService, req RunRequest) error {
	existing, err := service.GetSession(ctx, req.SessionID)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}
	_, err = service.CreateSession(ctx, session.CreateSessionRequest{
		SessionID: req.SessionID,
		AppID:     req.AppName,
		UserID:    req.UserID,
	})
	return err
}

func (h *Handler) handleUI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	requestPath := path.Clean("/" + r.URL.Path)
	assetPath := requestPath[1:]
	if assetPath == "" {
		assetPath = "index.html"
	}
	if _, err := fs.Stat(h.uiFS, assetPath); err == nil {
		h.serveAsset(w, r, assetPath)
		return
	} else if !errors.Is(err, fs.ErrNotExist) {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.serveAsset(w, r, "index.html")
}

func (h *Handler) serveAsset(w http.ResponseWriter, r *http.Request, assetPath string) {
	if assetPath == "index.html" {
		data, err := fs.ReadFile(h.uiFS, assetPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeContent(w, r, assetPath, time.Time{}, bytes.NewReader(data))
		return
	}

	r2 := new(http.Request)
	*r2 = *r
	r2.URL = cloneURL(r.URL)
	r2.URL.Path = "/" + assetPath
	h.ui.ServeHTTP(w, r2)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
