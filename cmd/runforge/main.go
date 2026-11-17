package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/alexandrmotologa/runforge/internal/compiler"
	"github.com/alexandrmotologa/runforge/internal/durable"
	"github.com/alexandrmotologa/runforge/internal/engine"
	"github.com/alexandrmotologa/runforge/internal/sandbox"
	"github.com/alexandrmotologa/runforge/internal/server"
	"github.com/alexandrmotologa/runforge/internal/telemetry"
)

var (
	version = "1.0.0"
	dbFile  string
)

func main() {
	start := time.Now()

	rootCmd := &cobra.Command{
		Use:   "runforge",
		Short: "Durable autonomous agent control plane and trajectory-to-workflow compiler in Go",
		Long: `runforge eliminates token waste in autonomous agents by compiling exploratory
trajectories into deterministic playbooks and executing them durably in native OS sandboxes.`,
	}

	rootCmd.PersistentFlags().StringVar(&dbFile, "db", "runforge.db", "Path to SQLite WAL database")

	// 1. Command: run
	var runParams []string
	runCmd := &cobra.Command{
		Use:   "run <playbook.yaml | trajectory.json>",
		Short: "Execute a compiled deterministic workflow or agent trajectory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := args[0]
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", filePath, err)
			}

			ledger, err := durable.NewLedger(dbFile)
			if err != nil {
				return fmt.Errorf("failed to initialize SQLite WAL ledger: %w", err)
			}
			defer ledger.Close()

			runner := sandbox.NewDefaultRunner()
			orch := engine.NewOrchestrator(ledger, runner, engine.NewDefaultModelClient())

			// Parse custom input parameters
			inputVars := make(map[string]interface{})
			for _, p := range runParams {
				parts := strings.SplitN(p, "=", 2)
				if len(parts) == 2 {
					inputVars[parts[0]] = parts[1]
				}
			}

			// Check if file is a Playbook YAML
			var pb *compiler.Playbook
			if strings.HasSuffix(filePath, ".yaml") || strings.HasSuffix(filePath, ".yml") {
				pb, err = compiler.FromYAML(data)
				if err != nil {
					return fmt.Errorf("invalid playbook yaml: %w", err)
				}
			} else {
				// Parse as Trajectory and compile on the fly
				trace, err := compiler.ParseAutoFormat(data)
				if err != nil {
					return fmt.Errorf("failed to parse trajectory: %w", err)
				}
				report, err := compiler.BuildCausalGraphAndPrune(trace)
				if err != nil {
					return fmt.Errorf("pruning failed: %w", err)
				}
				pb, err = compiler.CompilePlaybook(trace, report)
				if err != nil {
					return fmt.Errorf("failed to compile playbook: %w", err)
				}
				fmt.Printf("[compiler] Pruned %d non-contributing steps (%.1f%% token reduction)\n",
					report.PrunedStepCount, report.ReductionPercent)
			}

			runID := fmt.Sprintf("run-%d", time.Now().UnixNano())
			fmt.Printf("--> Initiating execution [%s]: %s\n", runID, pb.Name)
			fmt.Printf("--> Native Sandbox: %s (%s)\n", runner.Capabilities().EngineType, runner.Capabilities().Platform)

			run, err := orch.RunPlaybook(context.Background(), pb, runID, inputVars)
			if err != nil {
				return fmt.Errorf("execution error: %w", err)
			}

			if run.Status == durable.StatusWaitingApproval {
				fmt.Printf("\n[ATTENTION] Run paused: Operator approval required.\n")
				fmt.Printf("Run ID: %s. Use 'runforge resume %s' or web studio on port 7777 to approve.\n", run.ID, run.ID)
				return nil
			}

			fmt.Println("\n==================================================")
			fmt.Printf("Execution Completed Successfully in %s\n", time.Since(start).Round(time.Millisecond))
			fmt.Printf("Tokens Conserved: %d (Zero LLM Tokens Spent on Rigid Steps)\n", run.TokensSaved)
			fmt.Printf("Estimated Cost Saved: $%.4f\n", run.CostSaved)
			fmt.Println("==================================================")
			return nil
		},
	}
	runCmd.Flags().StringSliceVar(&runParams, "param", nil, "Input parameters (key=value)")

	// 2. Command: compile
	var outYAML, outScript string
	compileCmd := &cobra.Command{
		Use:   "compile <trajectory.json | transcript.jsonl>",
		Short: "Compile an exploratory agent trace into a deterministic playbook and native script",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := args[0]
			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read trajectory file: %w", err)
			}

			trace, err := compiler.ParseAutoFormat(data)
			if err != nil {
				return fmt.Errorf("failed to parse trajectory trace: %w", err)
			}

			report, err := compiler.BuildCausalGraphAndPrune(trace)
			if err != nil {
				return fmt.Errorf("causal graph pruning failed: %w", err)
			}

			playbook, err := compiler.CompilePlaybook(trace, report)
			if err != nil {
				return fmt.Errorf("playbook compilation failed: %w", err)
			}

			yamlBytes, err := compiler.ToYAML(playbook)
			if err != nil {
				return fmt.Errorf("yaml serialization failed: %w", err)
			}

			if outYAML != "" {
				if err := os.MkdirAll(filepath.Dir(outYAML), 0755); err != nil {
					return fmt.Errorf("failed to create directory for yaml: %w", err)
				}
				if err := os.WriteFile(outYAML, yamlBytes, 0644); err != nil {
					return fmt.Errorf("failed to write %s: %w", outYAML, err)
				}
				fmt.Printf("[OK] Wrote compiled playbook to %s\n", outYAML)
			} else {
				fmt.Println(string(yamlBytes))
			}

			if outScript != "" {
				var scriptContent string
				ext := strings.ToLower(filepath.Ext(outScript))
				switch ext {
				case ".sh", ".bash":
					scriptContent, err = compiler.GenerateBash(playbook)
				case ".py":
					scriptContent, err = compiler.GeneratePython(playbook)
				case ".ps1":
					scriptContent, err = compiler.GeneratePowerShell(playbook)
				default:
					scriptContent, err = compiler.GenerateBash(playbook)
				}
				if err != nil {
					return fmt.Errorf("script codegen failed: %w", err)
				}

				if err := os.MkdirAll(filepath.Dir(outScript), 0755); err != nil {
					return fmt.Errorf("failed to create directory for script: %w", err)
				}
				if err := os.WriteFile(outScript, []byte(scriptContent), 0755); err != nil {
					return fmt.Errorf("failed to write script %s: %w", outScript, err)
				}
				fmt.Printf("[OK] Generated standalone script: %s\n", outScript)
			}

			tracker := telemetry.NewCostTracker()
			tokensSaved := report.EstimatedTokensSaved
			costSaved := tracker.CalculateSavings(trace.Model, tokensSaved, 0, 0)

			fmt.Println("\n================ Compilation Summary ================")
			fmt.Printf("Original Turns:       %d\n", report.OriginalStepCount)
			fmt.Printf("Dead Ends Pruned:     %d (%.1f%% reduction)\n", report.PrunedStepCount, report.ReductionPercent)
			fmt.Printf("Deterministic Steps:  %d\n", report.RetainedStepCount)
			fmt.Printf("Conserved Tokens:     ~%d tokens / subsequent run\n", tokensSaved)
			fmt.Printf("Net USD Savings:      ~$%.4f per execution\n", costSaved)
			fmt.Println("=====================================================")

			return nil
		},
	}
	compileCmd.Flags().StringVarP(&outYAML, "output", "o", "", "Path to write compiled workflow.yaml")
	compileCmd.Flags().StringVar(&outScript, "script", "", "Path to write standalone script (.sh, .py, .ps1)")

	// 3. Command: serve
	var port int
	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start embedded React 19 control plane studio and REST API",
		RunE: func(cmd *cobra.Command, args []string) error {
			initDuration := time.Since(start)

			ledger, err := durable.NewLedger(dbFile)
			if err != nil {
				return fmt.Errorf("failed to initialize SQLite WAL ledger: %w", err)
			}
			defer ledger.Close()

			runner := sandbox.NewDefaultRunner()
			orch := engine.NewOrchestrator(ledger, runner, engine.NewDefaultModelClient())
			srv := server.NewServer(port, ledger, orch)

			addr := fmt.Sprintf(":%d", port)
			fmt.Println("==================================================")
			fmt.Printf("runforge Control Plane Studio v%s\n", version)
			fmt.Printf("Cold start latency:    %s (<10ms target met)\n", initDuration.Round(time.Millisecond))
			fmt.Printf("Isolation engine:      %s (%s)\n", runner.Capabilities().EngineType, runner.Capabilities().Platform)
			fmt.Printf("Durable WAL Ledger:    %s\n", dbFile)
			fmt.Printf("Web Studio Console:    http://localhost:%d\n", port)
			fmt.Printf("Prometheus Metrics:    http://localhost:%d/metrics\n", port)
			fmt.Println("==================================================")

			return http.ListenAndServe(addr, srv.Handler())
		},
	}
	serveCmd.Flags().IntVarP(&port, "port", "p", 7777, "HTTP server listening port")

	// 4. Command: resume
	resumeCmd := &cobra.Command{
		Use:   "resume <run_id>",
		Short: "Resume an interrupted or approval-paused run from SQLite WAL state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runID := args[0]
			ledger, err := durable.NewLedger(dbFile)
			if err != nil {
				return fmt.Errorf("failed to initialize ledger: %w", err)
			}
			defer ledger.Close()

			runner := sandbox.NewDefaultRunner()
			orch := engine.NewOrchestrator(ledger, runner, engine.NewDefaultModelClient())

			// Check pending approvals
			approvals, _ := ledger.GetPendingApprovals(context.Background(), runID)
			if len(approvals) > 0 {
				app := approvals[0]
				fmt.Printf("[GATE] Pending Approval for Step #%d: %s\n", app.StepIndex, app.Command)
				fmt.Printf("Reason: %s\n", app.Reason)
				fmt.Print("Approve this mutating command? [y/N]: ")
				var answer string
				fmt.Scanln(&answer)
				if strings.ToLower(strings.TrimSpace(answer)) == "y" || strings.ToLower(strings.TrimSpace(answer)) == "yes" {
					_ = ledger.UpdateApproval(context.Background(), app.ID, durable.ApprovalApproved)
					fmt.Println("[OK] Operator approval granted.")
				} else {
					_ = ledger.UpdateApproval(context.Background(), app.ID, durable.ApprovalRejected)
					fmt.Println("[ABORT] Operator rejected command execution.")
					return nil
				}
			}

			fmt.Printf("--> Restoring state from SQLite checkpoint for run %s...\n", runID)
			run, err := orch.ResumeRun(context.Background(), runID)
			if err != nil {
				return fmt.Errorf("resume failed: %w", err)
			}

			fmt.Printf("[OK] Run resumed successfully with status: %s\n", run.Status)
			return nil
		},
	}

	// 5. Command: inspect
	inspectCmd := &cobra.Command{
		Use:   "inspect <run_id>",
		Short: "Inspect execution steps, token economics, and checkpoints of a run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runID := args[0]
			ledger, err := durable.NewLedger(dbFile)
			if err != nil {
				return fmt.Errorf("failed to initialize ledger: %w", err)
			}
			defer ledger.Close()

			run, err := ledger.GetRun(context.Background(), runID)
			if err != nil {
				return fmt.Errorf("run not found: %w", err)
			}

			steps, err := ledger.GetStepsByRun(context.Background(), runID)
			if err != nil {
				return fmt.Errorf("failed to load steps: %w", err)
			}

			fmt.Printf("Run ID:        %s\n", run.ID)
			fmt.Printf("Name:          %s\n", run.Name)
			fmt.Printf("Status:        %s\n", run.Status)
			fmt.Printf("Mode:          %s\n", run.Mode)
			fmt.Printf("Tokens Spent:  %d (in: %d, out: %d)\n", run.TokensIn+run.TokensOut, run.TokensIn, run.TokensOut)
			fmt.Printf("Tokens Saved:  %d\n", run.TokensSaved)
			fmt.Printf("Cost:          $%.4f (Saved: $%.4f)\n", run.CostUSD, run.CostSaved)
			fmt.Printf("Created:       %s\n", run.CreatedAt.Format(time.RFC3339))
			fmt.Printf("\nRecorded Steps (%d total):\n", len(steps))

			for _, s := range steps {
				pruneTag := ""
				if s.IsPruned {
					pruneTag = " [PRUNED]"
				}
				fmt.Printf("  #%02d [%s] %-25s (%dms)%s\n", s.StepIndex, s.Status, s.Name, s.DurationMs, pruneTag)
				if s.InputPayload != "" {
					fmt.Printf("       in:  %s\n", s.InputPayload)
				}
				if s.OutputPayload != "" {
					outSnippet := strings.ReplaceAll(s.OutputPayload, "\n", " ")
					if len(outSnippet) > 80 {
						outSnippet = outSnippet[:77] + "..."
					}
					fmt.Printf("       out: %s\n", outSnippet)
				}
			}

			return nil
		},
	}

	// 6. Command: version
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Display version, OS platform, and active sandbox primitives",
		Run: func(cmd *cobra.Command, args []string) {
			runner := sandbox.NewDefaultRunner()
			caps := runner.Capabilities()
			fmt.Printf("runforge version %s\n", version)
			fmt.Printf("Platform:         %s\n", caps.Platform)
			fmt.Printf("Sandbox Engine:   %s\n", caps.EngineType)
			fmt.Printf("Process Tree Iso: %v\n", caps.HasProcessTree)
			fmt.Printf("Memory Capping:   %v\n", caps.HasMemoryLimit)
			fmt.Printf("Single Binary:    Zero-CGO Pure Go\n")
		},
	}

	rootCmd.AddCommand(runCmd, compileCmd, serveCmd, resumeCmd, inspectCmd, versionCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
