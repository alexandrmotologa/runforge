package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandrmotologa/runforge/internal/durable"
	"github.com/alexandrmotologa/runforge/internal/engine"
	"github.com/alexandrmotologa/runforge/internal/sandbox"
)

func TestServerEndpoints(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_server.db")

	ledger, err := durable.NewLedger(dbPath)
	if err != nil {
		t.Fatalf("failed to create ledger: %v", err)
	}
	defer ledger.Close()

	runner := sandbox.NewDefaultRunner()
	client := engine.NewMockClient()
	orch := engine.NewOrchestrator(ledger, runner, client)

	srv := NewServer(7777, ledger, orch)
	handler := srv.Handler()

	// 1. Test /api/stats
	req := httptest.NewRequest("GET", "/api/stats", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 for /api/stats, got %d", rec.Code)
	}

	// 2. Test /api/compile
	sampleTrace := `{
		"id": "trace-test-api",
		"goal": "Verify compilation API works",
		"steps": [
			{"index": 0, "tool": "bash", "command": "git status", "exit_code": 0},
			{"index": 1, "tool": "read_file", "path": "file1.txt", "exit_code": 0},
			{"index": 2, "tool": "read_file", "path": "file1.txt", "exit_code": 0},
			{"index": 3, "tool": "bash", "command": "go test ./...", "exit_code": 0}
		]
	}`

	reqCompile := httptest.NewRequest("POST", "/api/compile", bytes.NewReader([]byte(sampleTrace)))
	reqCompile.Header.Set("Content-Type", "application/json")
	recCompile := httptest.NewRecorder()
	handler.ServeHTTP(recCompile, reqCompile)

	if recCompile.Code != http.StatusOK {
		t.Fatalf("expected status 200 for /api/compile, got %d: %s", recCompile.Code, recCompile.Body.String())
	}

	var compileResp map[string]interface{}
	if err := json.Unmarshal(recCompile.Body.Bytes(), &compileResp); err != nil {
		t.Fatalf("failed to parse compile response: %v", err)
	}
	if compileResp["yaml"] == nil || compileResp["scripts"] == nil {
		t.Errorf("missing yaml or scripts in compile response")
	}

	// 3. Test Prometheus /metrics
	reqMetrics := httptest.NewRequest("GET", "/metrics", nil)
	recMetrics := httptest.NewRecorder()
	handler.ServeHTTP(recMetrics, reqMetrics)

	if recMetrics.Code != http.StatusOK {
		t.Errorf("expected status 200 for /metrics, got %d", recMetrics.Code)
	}
	if !strings.Contains(recMetrics.Body.String(), "runforge_runs_total") {
		t.Errorf("metrics body missing expected metric key")
	}

	// 4. Test Embedded Web Studio /index.html
	reqSPA := httptest.NewRequest("GET", "/", nil)
	recSPA := httptest.NewRecorder()
	handler.ServeHTTP(recSPA, reqSPA)

	if recSPA.Code != http.StatusOK {
		t.Errorf("expected status 200 for embedded SPA, got %d", recSPA.Code)
	}
	if !strings.Contains(recSPA.Body.String(), "runforge") {
		t.Errorf("embedded SPA missing runforge title/body: %s", recSPA.Body.String())
	}

	// 5. Test Approval Endpoint
	app := &durable.Approval{
		ID:        "app-api-test",
		RunID:     "run-dummy",
		StepIndex: 1,
		Command:   "git push",
		Reason:    "push requires approval",
		Status:    durable.ApprovalPending,
	}
	_ = ledger.RecordApproval(context.Background(), app)

	reqApprove := httptest.NewRequest("POST", "/api/approvals/app-api-test", strings.NewReader(`{"action":"approve"}`))
	recApprove := httptest.NewRecorder()
	handler.ServeHTTP(recApprove, reqApprove)

	if recApprove.Code != http.StatusOK {
		t.Errorf("expected 200 for approval, got %d", recApprove.Code)
	}
}
