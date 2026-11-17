package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/alexandrmotologa/runforge/internal/durable"
)

func TestLiveRecordingProxy(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "proxy_test.db")

	ledger, err := durable.NewLedger(dbPath)
	if err != nil {
		t.Fatalf("failed to create ledger: %v", err)
	}
	defer ledger.Close()

	// 1. Mock upstream LLM server
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"id": "chatcmpl-mock-123",
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role": "assistant",
						"tool_calls": []map[string]interface{}{
							{
								"id":   "call_01",
								"type": "function",
								"function": map[string]interface{}{
									"name":      "bash",
									"arguments": "git status",
								},
							},
						},
					},
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     1500,
				"completion_tokens": 120,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer upstreamServer.Close()

	// 2. Initialize Proxy Server pointing to mock upstream
	outYAML := filepath.Join(tempDir, "auto_workflow.yaml")
	proxyServer, err := NewProxyServer(ProxyConfig{
		Port:        8085,
		TargetURL:   upstreamServer.URL,
		AutoCompile: true,
		OutputYAML:  outYAML,
	}, ledger)
	if err != nil {
		t.Fatalf("failed to create proxy server: %v", err)
	}

	handler := proxyServer.Handler()

	// 3. Send request through proxy
	reqBody := `{"model": "gpt-4o", "messages": [{"role": "user", "content": "Check git repository status"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from proxy, got %d: %s", rec.Code, rec.Body.String())
	}

	// Allow background recording goroutine to write to SQLite WAL
	time.Sleep(100 * time.Millisecond)

	// 4. Verify run was recorded in SQLite WAL
	runs, err := ledger.ListRuns(context.Background(), 10, 0)
	if err != nil || len(runs) == 0 {
		t.Fatalf("expected recorded run in ledger, got %d (err: %v)", len(runs), err)
	}

	// 5. Finalize session and verify compiled playbook
	playbook, err := proxyServer.FinalizeSession(context.Background())
	if err != nil {
		t.Fatalf("failed to finalize session: %v", err)
	}

	if playbook == nil || len(playbook.Steps) == 0 {
		t.Fatalf("expected compiled playbook steps from live proxy capture")
	}

	t.Logf("Successfully captured and compiled live session: %s with %d steps", playbook.Name, len(playbook.Steps))
}
