# runforge: Architecture and Engineering Master Plan
> High-Performance Autonomous Agent Control Plane, Durable Execution Engine and Workflow Compiler in Go
> Target: Zero-CGO Single-Binary Go Engine (< 25MB RAM, < 10ms startup) with Embedded React 19 Control Plane and Trajectory Compiler

---

## 1. Problem Statement and Motivation

### 1.1 The Amnesic Agent Loop Crisis
Autonomous coding, research, and operational agents (such as Claude Code, Cursor, OpenCode, AutoGen, and CrewAI) demonstrate strong capabilities in initial zero-shot problem discovery. However, recurring production workloads (such as pull request reviews, vendor invoice verification, daily log triaging, or database migrations) expose significant economic and architectural inefficiencies:

1. **Redundant Discovery Overhead**: When an agent executes a recurring task, it re-discovers the directory tree, re-reads configuration files, re-queries database schemas, and reconstructs execution plans from scratch. Each run consumes between 15,000 and 80,000 frontier tokens ($0.50 to $5.00 per execution) for identical deterministic steps.
2. **Fragility of Monolithic Node.js Runtimes**: Existing agent control planes often rely on complex Node.js runtimes requiring specific engine versions (Node 24+), suffering from single-threaded event loop bottlenecks, inconsistent Windows compatibility, and mandatory Docker daemon dependencies for basic terminal commands.
3. **Catastrophic State Loss on Interruption**: Standard agent harnesses maintain execution state solely in volatile memory. If the host process terminates, crashes, or encounters an operating system timeout at step 18 of a 20-step sequence, the entire session is lost, discarding all accumulated tokens and requiring a full re-run.
4. **Absence of Automated Model Handoff**: Teams default to running expensive frontier models (Claude 3.5 Sonnet, GPT-4o, OpenAI o1) across all executions because small models cannot navigate initial zero-shot ambiguity. Once the execution trajectory is established, low-cost models ($0.075/1M tokens) or deterministic scripts can execute subsequent steps with identical results.

### 1.2 The runforge Solution
`runforge` is an enterprise-grade agent control plane and workflow compiler built in pure Go. It operates locally with zero runtime prerequisites:

- **Zero-CGO Single Binary Architecture**: Compiles to a self-contained executable under 25MB, operating natively across Windows, Linux, and macOS without requiring Node.js, Python, or mandatory Docker daemons.
- **Trajectory-to-Workflow Compiler**: Captures exploratory ReAct trajectories from initial agent runs, prunes dead ends and redundant reads, and compiles validated steps into deterministic playbooks (`workflow.yaml` or native scripts). Subsequent runs bypass the LLM loop for deterministic actions, reducing token consumption by 90% to 95%.
- **Durable Step Ledger (Pure Go SQLite WAL)**: Commits every step, tool call, environment change, and token expenditure to an embedded SQLite database configured with Write-Ahead Logging. Interrupted runs resume instantly from the exact point of interruption without duplicating prior steps.
- **Automated Model Handoff (Frontier to SLM Distillation)**: Routes initial exploratory runs through frontier models to formulate execution blueprints, then transitions recurring runs to low-cost models (Gemini Flash, Llama 3.1 8B, local Ollama) or pure deterministic execution.
- **Native OS Sandboxing**: Enforces CPU, memory, and process lifecycle limits using native operating system primitives (Windows Job Objects, Linux namespaces and cgroups) with an optional container driver fallback.
- **Embedded React 19 Control Plane Studio**: Serves an interactive management console embedded inside the Go binary (`:7777`) providing live execution DAGs, dollar savings tracking, trajectory diffs, and human-in-the-loop approvals.

---

## 2. System Architecture

```
       +-------------------------------------------------------------+
       |               AI Agent Harness / Client Run Request         |
       |         (Claude Code, Cursor, Custom Agent, CLI, API)       |
       +-------------------------------------------------------------+
                                      |
                     Inbound Run Trigger (CLI / HTTP / SDK)
                                      v
       +-------------------------------------------------------------+
       |                     runforge Core Engine                    |
       |                    (High-Performance Go CLI)                |
       |                                                             |
       |  +---------------------+        +------------------------+  |
       |  | Run Dispatcher and  |------->| Durable State Ledger   |  |
       |  | Task State Machine  |        | (Pure Go SQLite WAL)   |  |
       |  +---------------------+        +------------------------+  |
       |              |                               |              |
       |              v                               v              |
       |  +-------------------------------------------------------+  |
       |  |               Execution Sandbox Engine                |  |
       |  |  - Windows: Job Objects (Process Tree and RAM Caps)   |  |
       |  |  - Linux: Namespaces + cgroups / bubblewrap           |  |
       |  |  - Direct: Isolated process runner with env filters   |  |
       |  |  - Optional: Docker / Podman Container Driver         |  |
       |  +-------------------------------------------------------+  |
       |                              |                              |
       |                              v                              |
       |  +-------------------------------------------------------+  |
       |  |          Trajectory-to-Workflow Compiler              |  |
       |  |  - Ingestion Adapters (Claude, Cursor, ReAct, Open)   |  |
       |  |  - Causal Dependency Graph Builder (DAG)              |  |
       |  |  - Dead-End Pruner and Redundancy Stripper            |  |
       |  |  - Dynamic Input Parameterizer                        |  |
       |  |  - Deterministic Playbook Synthesizer (workflow.yaml) |  |
       |  |  - Native Script Generator (Bash, Python, Go)         |  |
       |  +-------------------------------------------------------+  |
       |                              |                              |
       |                              v                              |
       |  +-------------------------------------------------------+  |
       |  |          Model Handoff and Cost Compression           |  |
       |  |  - Run 1: Frontier Exploration (Claude / GPT-4o)      |  |
       |  |  - Runs 2..N: Compiled Playbook + Fast SLM            |  |
       |  |  - Live Token Counter and USD ROI Calculator          |  |
       |  +-------------------------------------------------------+  |
       |                              |                              |
       +------------------------------|------------------------------+
                                       |
                                       v
       +-------------------------------------------------------------+
       |                  Enterprise Control Surfaces                |
       |  1. Embedded React 19 Studio (Local HTTP :7777)             |
       |  2. Compiled Deterministic Playbook (workflow.yaml)         |
       |  3. Prometheus Metrics and SQLite Audit Trail               |
       |  4. CLI Interface (run, compile, resume, inspect, serve)    |
       +-------------------------------------------------------------+
```

---

## 3. Mathematical Formulation: Trajectory Compilation and Cost Compression

### 3.1 Trajectory Pruning and Causal Dependency Formulation
Let an exploratory agent trajectory be an ordered sequence of action-observation pairs:

$$\mathcal{T} = \big( (a_1, o_1), (a_2, o_2), \dots, (a_N, o_N) \big)$$

Where each action $a_k \in \mathcal{A}_{\text{tool}} \cup \mathcal{A}_{\text{thought}}$.
Actions are categorized into distinct operational classes:
- Read Actions: inspect file or directory contents, query database, fetch URL.
- Mutate Actions: write file, execute mutating command, update database row.
- Compute Actions: compile code, run test suite, lint repository.
- Semantic Actions: summarize text, evaluate complex policy, classify unstructured content.

The compiler constructs a directed causal graph $G = (V, E)$, where:
- Each vertex $v_k \in V$ represents an action $a_k$.
- A directed edge $(v_i, v_j) \in E$ exists if action $a_j$ depends on the output, file side-effect, or state mutation of action $a_i$.

The target outcome is defined by the terminal successful state:

$$V_{\text{compiled}} = \{ a_k \in \mathcal{T} \mid \text{Reachable}(a_k, \text{Target\_Outcome}) = \text{true} \land \text{IsMutatingOrRequired}(a_k) \}$$

Pruning rules:
1. Dead-end commands (failed commands that did not produce state required by subsequent actions) are eliminated.
2. Exploratory reads overwritten by later reads or unused in downstream mutations are discarded.
3. Duplicate idempotent inspections are folded into a single reference step.

The compiled deterministic execution order is the topological sort of the pruned graph:

$$\mathcal{T}_{\text{compiled}} = \text{TopologicalSort}(G[V_{\text{compiled}}])$$

### 3.2 Cost Compression Ratio Formulation
Let $C_{\text{frontier}}$ denote the cost per token of the exploratory model, $C_{\text{slm}}$ the cost per token of the lightweight model, and $N_{\text{runs}}$ the total execution count:

$$\text{Cost}_{\text{standard}} = N_{\text{runs}} \cdot \left( T_{\text{explore}} \cdot C_{\text{frontier}} \right)$$

$$\text{Cost}_{\text{runforge}} = \left( T_{\text{explore}} \cdot C_{\text{frontier}} \right) + (N_{\text{runs}} - 1) \cdot \left( T_{\text{semantic}} \cdot C_{\text{slm}} + T_{\text{code}} \cdot 0 \right)$$

Because compiled deterministic commands execute directly within the native sandbox without token consumption ($T_{\text{code}} \cdot 0$), token expenditure on subsequent runs approaches zero for fully deterministic tasks.

The Net Savings Ratio is defined as:

$$R_{\text{savings}} = 1 - \frac{\text{Cost}_{\text{runforge}}}{\text{Cost}_{\text{standard}}} \ge 0.90 \quad (\text{for } N_{\text{runs}} \ge 5)$$

---

## 4. Repository Structure

```
runforge/
├── cmd/
│   └── runforge/
│       └── main.go                 # CLI entry point (run, compile, serve, resume, inspect, version)
├── internal/
│   ├── config/
│   │   ├── config.go               # Configuration parser for models, storage, and limits
│   │   └── schema.go               # Workflow schema definitions and spend limits
│   ├── engine/
│   │   ├── orchestrator.go         # Core agent execution runner and lifecycle state machine
│   │   ├── state.go                # Step progression, pause, cancel, and checkpoint handlers
│   │   ├── handoff.go              # Model routing: Frontier exploration vs SLM replay
│   │   └── client.go               # Model provider abstraction (OpenAI, Gemini, Ollama, Mock)
│   ├── durable/
│   │   ├── ledger.go               # Transactional step logger with pure Go SQLite WAL
│   │   ├── store.go                # Checkpoint persistence and variable store
│   │   ├── models.go               # Database record types (runs, steps, checkpoints, approvals)
│   │   └── recovery.go             # State restoration on process interruption
│   ├── compiler/
│   │   ├── trajectory.go           # Trace parser and causal dependency graph builder
│   │   ├── pruner.go               # Dead-end elimination and redundant tool call stripper
│   │   ├── parameterizer.go        # Dynamic input extraction and token placeholder replacement
│   │   ├── codegen.go              # Generates standalone Bash, Python, or Go scripts
│   │   └── playbook.go             # Compiles validated runs into declarative YAML playbooks
│   ├── sandbox/
│   │   ├── runner.go               # Unified sandbox abstraction interface
│   │   ├── windows_jobs.go         # Native Windows Job Objects for RAM and process isolation
│   │   ├── linux_ns.go             # Native Linux namespaces and cgroups runner
│   │   ├── direct_runner.go        # Fast process runner with strict timeout and env isolation
│   │   └── docker_driver.go        # Optional fallback for containerized environments
│   ├── telemetry/
│   │   ├── cost_tracker.go         # Token counter, USD expenditures, and ROI calculations
│   │   └── metrics_export.go       # Prometheus /metrics endpoint exporter
│   └── server/
│       ├── api.go                  # Management REST API (/api/runs, /api/compile, /api/stats)
│       ├── embed.go                # Embedded React 19 studio static assets via //go:embed
│       └── sse.go                  # Real-time event and terminal stream for web console
├── web/
│   ├── src/
│   │   ├── App.tsx                 # React 19 root application
│   │   ├── components/
│   │   │   ├── RunTimeline.tsx     # Step-by-step execution DAG visualizer
│   │   │   ├── SavingsCard.tsx     # Token reduction and dollar savings hero metrics
│   │   │   ├── TrajectoryDiff.tsx  # Side-by-side view: Exploratory Run vs Compiled Playbook
│   │   │   ├── TerminalStream.tsx  # Real-time execution log console
│   │   │   └── ApprovalModal.tsx   # Human-in-the-loop approval dialog
│   │   └── pages/
│   │       ├── Dashboard.tsx       # Primary Control Plane dashboard
│   │       └── PlaybookEditor.tsx  # Playbook and compilation inspector
│   ├── package.json
│   ├── tsconfig.json
│   └── vite.config.ts
├── examples/
│   ├── runforge.yaml               # Enterprise platform configuration sample
│   ├── sample_trajectory.json      # Raw recorded agent trace for offline testing
│   ├── sample_claude_trace.jsonl   # Claude Code format trajectory trace
│   └── templates/
│       ├── code_review_pipeline.yaml # Sample compiled code review agent
│       └── invoice_triage.yaml     # Sample accounts payable data extraction agent
├── Makefile
├── go.mod
├── go.sum
├── LICENSE
└── README.md
```

---

## 5. Detailed Implementation Plan and Work Packages

### Work Package 1: Foundation and Durable SQLite WAL Ledger
- Initialize Go module with pure Go dependencies (`modernc.org/sqlite`, `gopkg.in/yaml.v3`, `github.com/spf13/cobra`).
- Design SQLite database schema with WAL mode enabled (`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;`).
- Implement tables: `runs`, `steps`, `checkpoints`, `approvals`, `metrics`.
- Implement transactional operations: `CreateRun`, `RecordStep`, `CreateCheckpoint`, `RecordApproval`, `UpdateRunStatus`.
- Implement recovery logic to detect unfinished runs and resume from the latest valid checkpoint.

### Work Package 2: Native OS Sandboxing (Zero-Docker Primitives)
- Build unified `sandbox.Runner` interface with resource constraints (MaxMemoryBytes, MaxCPURate, Timeout, WorkingDir, Env).
- Implement Windows Job Objects integration on Windows via `windows.CreateJobObject`, configuring memory limits and process termination on handle close.
- Implement Linux namespaces runner with process group constraints for Linux hosts.
- Implement cross-platform direct runner with environment sanitization and execution isolation.
- Add unit tests verifying process termination upon timeout and environment isolation.

### Work Package 3: Trajectory Parser, Causal DAG and Pruner
- Build multi-format trajectory ingestion supporting ReAct JSON traces and transcript formats.
- Construct causal dependency DAG tracking file mutations, command outputs, and downstream references.
- Implement graph-based pruning algorithm removing unreferenced reads, failed exploratory branches, and dead loops.
- Implement parameterizer identifying static paths and dynamic arguments for reusable playbooks.

### Work Package 4: Deterministic Workflow Compiler and Script Synthesizer
- Implement `playbook.go` serializing pruned execution graphs into declarative YAML workflows (`workflow.yaml`).
- Implement `codegen.go` generating standalone native scripts (Bash, Python, Go) that execute without LLM calls.
- Synthesize deterministic assertions (exit code verification, regex output matching, file presence checks).

### Work Package 5: Execution Engine, State Machine and Model Handoff
- Implement `orchestrator.go` managing workflow state transitions (PENDING, RUNNING, PAUSED, WAITING_APPROVAL, COMPLETED, FAILED).
- Implement `handoff.go` supporting dual/hybrid execution: deterministic sandbox execution for rigid steps and SLM routing for semantic steps.
- Implement client abstraction supporting OpenAI-compatible APIs, Google Gemini API, and local Ollama instances.
- Implement live token counting and USD savings calculation based on exact model pricing tiers.

### Work Package 6: Embedded React 19 Control Plane Studio
- Initialize Vite + React 19 + TypeScript + Tailwind CSS application in `web/`.
- Build Control Plane Dashboard displaying active runs, token reduction percentages, and dollar savings.
- Build interactive Run Timeline DAG component displaying step states and pruning indicators.
- Build Trajectory Diff inspector comparing raw exploratory traces with compiled playbooks.
- Build real-time event streaming via Server-Sent Events (SSE).
- Build Human-in-the-Loop approval modal for guarded actions.
- Bundle React production build into `web/dist` and embed directly into Go binary using `//go:embed`.

### Work Package 7: CLI Interface and Subcommands
- Build primary CLI in `cmd/runforge/main.go` using Cobra:
  - `runforge run`: Executes a workflow or initiates an exploratory run.
  - `runforge compile`: Compiles raw trajectory into a deterministic playbook and script.
  - `runforge serve`: Starts embedded HTTP server and React 19 studio on port 7777.
  - `runforge resume`: Resumes an interrupted run from the SQLite ledger.
  - `runforge inspect`: Inspects run execution steps, causal graph, and token economics.
  - `runforge version`: Displays version, commit, and platform sandbox support.

### Work Package 8: Verification, Test Suite and Benchmarking
- Author unit tests for SQLite WAL transactions, recovery, pruning algorithms, and code generators.
- Execute offline compilation test on `examples/sample_trajectory.json` proving 90%+ token reduction.
- Verify binary compilation with zero CGO dependencies.
- Measure cold-start latency and resident memory footprint.
- Author humanized README and architectural documentation.
