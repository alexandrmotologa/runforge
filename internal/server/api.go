package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alexandrmotologa/runforge/internal/compiler"
	"github.com/alexandrmotologa/runforge/internal/durable"
	"github.com/alexandrmotologa/runforge/internal/engine"
	"github.com/alexandrmotologa/runforge/internal/telemetry"
)

// Server coordinates the REST API, SSE streams, and embedded web studio.
type Server struct {
	port         int
	ledger       *durable.Ledger
	orchestrator *engine.Orchestrator
	mux          *http.ServeMux
}

func NewServer(port int, ledger *durable.Ledger, orch *engine.Orchestrator) *Server {
	if port <= 0 {
		port = 7777
	}

	s := &Server{
		port:         port,
		ledger:       ledger,
		orchestrator: orch,
		mux:          http.NewServeMux(),
	}

	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	// API Endpoints
	s.mux.HandleFunc("/api/stats", s.handleStats)
	s.mux.HandleFunc("/api/runs", s.handleRuns)
	s.mux.HandleFunc("/api/runs/", s.handleRunByID)
	s.mux.HandleFunc("/api/compile", s.handleCompile)
	s.mux.HandleFunc("/api/approvals/", s.handleApproval)
	s.mux.HandleFunc("/api/events", s.handleSSE)

	// Prometheus Metrics
	s.mux.HandleFunc("/metrics", telemetry.PrometheusHandler(s.ledger))

	// Embedded Web Assets & SPA Fallback
	s.mux.Handle("/", s.spaHandler())
}

// Handler returns the primary HTTP handler with CORS headers enabled.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		s.mux.ServeHTTP(w, r)
	})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats, err := s.ledger.GetAggregateStats(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleRuns(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		limit := 50
		offset := 0
		if lStr := r.URL.Query().Get("limit"); lStr != "" {
			if l, err := strconv.Atoi(lStr); err == nil {
				limit = l
			}
		}
		if oStr := r.URL.Query().Get("offset"); oStr != "" {
			if o, err := strconv.Atoi(oStr); err == nil {
				offset = o
			}
		}

		runs, err := s.ledger.ListRuns(r.Context(), limit, offset)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if runs == nil {
			runs = make([]*durable.Run, 0)
		}
		writeJSON(w, http.StatusOK, runs)

	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
			return
		}

		// Check if payload is a Playbook or a raw Trajectory
		var req struct {
			Playbook *compiler.Playbook `json:"playbook,omitempty"`
			RawYAML  string             `json:"raw_yaml,omitempty"`
			Inputs   map[string]any     `json:"inputs,omitempty"`
		}

		pb := req.Playbook
		if pb == nil && req.RawYAML != "" {
			parsed, err := compiler.FromYAML([]byte(req.RawYAML))
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid yaml: %v", err)})
				return
			}
			pb = parsed
		}

		if pb == nil {
			// Try parsing entire body as Playbook YAML or JSON
			if parsed, err := compiler.FromYAML(body); err == nil && len(parsed.Steps) > 0 {
				pb = parsed
			} else {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid playbook required in request"})
				return
			}
		}

		runID := fmt.Sprintf("run-%d", time.Now().UnixNano())
		// Execute asynchronously in background goroutine to support long runs
		go func() {
			_, _ = s.orchestrator.RunPlaybook(context.Background(), pb, runID, req.Inputs)
		}()

		writeJSON(w, http.StatusAccepted, map[string]string{
			"message": "execution initiated",
			"run_id":  runID,
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleRunByID(w http.ResponseWriter, r *http.Request) {
	// Path format: /api/runs/{id} or /api/runs/{id}/resume
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/runs/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		http.Error(w, "invalid run id", http.StatusBadRequest)
		return
	}

	runID := pathParts[0]

	if len(pathParts) > 1 && pathParts[1] == "resume" && r.Method == http.MethodPost {
		run, err := s.orchestrator.ResumeRun(r.Context(), runID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, run)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	run, err := s.ledger.GetRun(r.Context(), runID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "run not found"})
		return
	}

	steps, _ := s.ledger.GetStepsByRun(r.Context(), runID)
	if steps == nil {
		steps = make([]*durable.Step, 0)
	}

	checkpoint, _ := s.ledger.GetLatestCheckpoint(r.Context(), runID)
	approvals, _ := s.ledger.GetPendingApprovals(r.Context(), runID)

	response := map[string]interface{}{
		"run":               run,
		"steps":             steps,
		"latest_checkpoint": checkpoint,
		"pending_approvals": approvals,
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleCompile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data, err := io.ReadAll(r.Body)
	if err != nil || len(data) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "empty trajectory body"})
		return
	}

	trace, err := compiler.ParseAutoFormat(data)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("failed to parse trajectory: %v", err)})
		return
	}

	report, err := compiler.BuildCausalGraphAndPrune(trace)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("pruning failed: %v", err)})
		return
	}

	playbook, err := compiler.CompilePlaybook(trace, report)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("playbook compilation failed: %v", err)})
		return
	}

	yamlBytes, _ := compiler.ToYAML(playbook)
	bashScript, _ := compiler.GenerateBash(playbook)
	pythonScript, _ := compiler.GeneratePython(playbook)
	psScript, _ := compiler.GeneratePowerShell(playbook)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"trace":          trace,
		"pruning_report": report,
		"playbook":       playbook,
		"yaml":           string(yamlBytes),
		"scripts": map[string]string{
			"bash":       bashScript,
			"python":     pythonScript,
			"powershell": psScript,
		},
	})
}

func (s *Server) handleApproval(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	approvalID := strings.TrimPrefix(r.URL.Path, "/api/approvals/")
	if approvalID == "" {
		http.Error(w, "missing approval id", http.StatusBadRequest)
		return
	}

	var req struct {
		Action string `json:"action"` // approve or reject
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	status := durable.ApprovalApproved
	if strings.ToLower(req.Action) == "reject" {
		status = durable.ApprovalRejected
	}

	if err := s.ledger.UpdateApproval(r.Context(), approvalID, status); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"approval_id": approvalID,
		"status":      string(status),
	})
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	eventsCh := s.orchestrator.Subscribe()
	defer s.orchestrator.Unsubscribe(eventsCh)

	// Send initial ping
	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"ok\"}\n\n")
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-eventsCh:
			if !ok {
				return
			}
			data, err := json.Marshal(ev)
			if err == nil {
				fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, string(data))
				flusher.Flush()
			}
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
