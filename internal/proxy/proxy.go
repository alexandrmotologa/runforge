package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/alexandrmotologa/runforge/internal/compiler"
	"github.com/alexandrmotologa/runforge/internal/durable"
	"github.com/alexandrmotologa/runforge/internal/telemetry"
)

// ProxyConfig defines settings for the live trajectory recording proxy.
type ProxyConfig struct {
	Port        int    `json:"port"`
	TargetURL   string `json:"target_url"`
	AutoCompile bool   `json:"auto_compile"`
	OutputYAML  string `json:"output_yaml"`
	SessionTTL  time.Duration
}

// ProxyServer intercepts agent LLM calls and logs turns into SQLite WAL in real-time.
type ProxyServer struct {
	cfg         ProxyConfig
	ledger      *durable.Ledger
	costTracker *telemetry.CostTracker
	target      *url.URL
	httpClient  *http.Client

	mu          sync.Mutex
	activeRun   *durable.Run
	stepIndex   int
	lastActivity time.Time
}

func NewProxyServer(cfg ProxyConfig, ledger *durable.Ledger) (*ProxyServer, error) {
	if cfg.Port <= 0 {
		cfg.Port = 8080
	}
	if cfg.TargetURL == "" {
		cfg.TargetURL = "https://api.openai.com"
	}
	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = 15 * time.Minute
	}

	targetURL, err := url.Parse(cfg.TargetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid target url: %w", err)
	}

	return &ProxyServer{
		cfg:         cfg,
		ledger:      ledger,
		costTracker: telemetry.NewCostTracker(),
		target:      targetURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}, nil
}

// Handler returns the HTTP request handler for the recording proxy.
func (p *ProxyServer) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only intercept completions endpoints
		if strings.HasSuffix(r.URL.Path, "/chat/completions") || strings.HasSuffix(r.URL.Path, "/messages") {
			p.handleIntercept(w, r)
			return
		}

		// Transparently pass-through any other endpoints (models, health)
		p.forwardDirect(w, r)
	})
}

func (p *ProxyServer) handleIntercept(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// 1. Read and buffer request body
	reqBody, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	r.Body.Close()

	// 2. Parse incoming prompt or tool use intention
	var reqMap map[string]interface{}
	_ = json.Unmarshal(reqBody, &reqMap)

	modelName, _ := reqMap["model"].(string)
	if modelName == "" {
		modelName = "claude-3-5-sonnet"
	}

	// 3. Ensure active run in SQLite WAL
	p.ensureActiveRun(modelName)

	// 4. Forward to upstream target
	targetPath := p.target.ResolveReference(r.URL).String()
	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetPath, bytes.NewReader(reqBody))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Copy headers
	for k, vv := range r.Header {
		for _, v := range vv {
			proxyReq.Header.Add(k, v)
		}
	}
	proxyReq.Host = p.target.Host

	resp, err := p.httpClient.Do(proxyReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("upstream error: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// 5. Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "failed to read upstream response", http.StatusInternalServerError)
		return
	}

	duration := time.Since(start)

	// 6. Record step into SQLite WAL asynchronously
	go p.recordTurn(reqMap, respBody, modelName, duration.Milliseconds())

	// 7. Write response back to agent client
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)
}

func (p *ProxyServer) forwardDirect(w http.ResponseWriter, r *http.Request) {
	targetPath := p.target.ResolveReference(r.URL).String()
	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetPath, r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for k, vv := range r.Header {
		for _, v := range vv {
			proxyReq.Header.Add(k, v)
		}
	}
	proxyReq.Host = p.target.Host

	resp, err := p.httpClient.Do(proxyReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (p *ProxyServer) ensureActiveRun(model string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now().UTC()
	if p.activeRun == nil || now.Sub(p.lastActivity) > p.cfg.SessionTTL {
		runID := fmt.Sprintf("proxy-run-%d", now.UnixNano())
		run := &durable.Run{
			ID:         runID,
			WorkflowID: "live-captured-agent",
			Name:       fmt.Sprintf("Live Recorded Run (%s)", now.Format("15:04:05")),
			Status:     durable.StatusRunning,
			Mode:       durable.ModeExploratory,
			Model:      model,
			CreatedAt:  now,
		}
		_ = p.ledger.CreateRun(context.Background(), run)
		p.activeRun = run
		p.stepIndex = 0
	}
	p.lastActivity = now
}

func (p *ProxyServer) recordTurn(reqMap map[string]interface{}, respBody []byte, model string, durationMs int64) {
	p.mu.Lock()
	runID := p.activeRun.ID
	idx := p.stepIndex
	p.stepIndex++
	p.mu.Unlock()

	var respMap struct {
		Choices []struct {
			Message struct {
				Role      string `json:"role"`
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
		} `json:"usage"`
	}

	_ = json.Unmarshal(respBody, &respMap)

	toolName := "llm_turn"
	commandPayload := ""

	if len(respMap.Choices) > 0 {
		msg := respMap.Choices[0].Message
		if len(msg.ToolCalls) > 0 {
			tc := msg.ToolCalls[0]
			toolName = tc.Function.Name
			commandPayload = tc.Function.Arguments
		} else {
			commandPayload = msg.Content
		}
	}

	step := &durable.Step{
		ID:            fmt.Sprintf("%s-step-%02d", runID, idx),
		RunID:         runID,
		StepIndex:     idx,
		Name:          fmt.Sprintf("Action: %s", toolName),
		Type:          durable.StepTypeTool,
		ToolName:      toolName,
		InputPayload:  commandPayload,
		OutputPayload: string(respBody),
		ExitCode:      0,
		DurationMs:    durationMs,
		Status:        durable.StepSuccess,
		CreatedAt:     time.Now().UTC(),
	}

	_ = p.ledger.RecordStep(context.Background(), step)

	// Update run token totals
	p.mu.Lock()
	p.activeRun.TokensIn += respMap.Usage.PromptTokens
	p.activeRun.TokensOut += respMap.Usage.CompletionTokens
	p.activeRun.CostUSD += p.costTracker.CalculateCost(model, respMap.Usage.PromptTokens, respMap.Usage.CompletionTokens)
	_ = p.ledger.UpdateRunStatus(context.Background(), runID, durable.StatusRunning,
		p.activeRun.TokensIn, p.activeRun.TokensOut, 0, p.activeRun.CostUSD, 0)
	p.mu.Unlock()
}

// FinalizeSession compiles the currently recorded trajectory into a deterministic playbook.
func (p *ProxyServer) FinalizeSession(ctx context.Context) (*compiler.Playbook, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.activeRun == nil {
		return nil, fmt.Errorf("no active recording session")
	}

	runID := p.activeRun.ID
	steps, err := p.ledger.GetStepsByRun(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch steps for run %s: %w", runID, err)
	}

	// Synthesize TrajectoryTrace
	trace := &compiler.TrajectoryTrace{
		ID:             runID,
		Goal:           p.activeRun.Name,
		Model:          p.activeRun.Model,
		TotalTokensIn:  p.activeRun.TokensIn,
		TotalTokensOut: p.activeRun.TokensOut,
		Steps:          make([]*compiler.TrajectoryStep, len(steps)),
	}

	for i, s := range steps {
		trace.Steps[i] = &compiler.TrajectoryStep{
			Index:      s.StepIndex,
			Tool:       s.ToolName,
			Command:    s.InputPayload,
			Output:     s.OutputPayload,
			ExitCode:   s.ExitCode,
			DurationMs: s.DurationMs,
		}
	}

	report, err := compiler.BuildCausalGraphAndPrune(trace)
	if err != nil {
		return nil, fmt.Errorf("pruning failed: %w", err)
	}

	playbook, err := compiler.CompilePlaybook(trace, report)
	if err != nil {
		return nil, fmt.Errorf("playbook compilation failed: %w", err)
	}

	if p.cfg.OutputYAML != "" {
		yamlBytes, err := compiler.ToYAML(playbook)
		if err == nil {
			_ = os.WriteFile(p.cfg.OutputYAML, yamlBytes, 0644)
		}
	}

	_ = p.ledger.UpdateRunStatus(ctx, runID, durable.StatusCompleted,
		p.activeRun.TokensIn, p.activeRun.TokensOut, playbook.Metadata.OriginalTokens, p.activeRun.CostUSD, 0)

	p.activeRun = nil
	return playbook, nil
}
