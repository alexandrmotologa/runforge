# runforge

> Deterministic durable execution engine, trajectory-to-workflow compiler, and control plane in Go for autonomous AI agents.

`runforge` is a single-binary control plane and durable runtime designed to reduce token waste and execution instability in autonomous agent workflows (such as Claude Code, Cursor, OpenCode, AutoGen, and CrewAI).

By capturing exploratory ReAct trajectories from initial agent executions, `runforge` builds a causal dependency graph, prunes dead-end probes, and compiles validated sequences into deterministic playbooks (`workflow.yaml` or standalone native scripts). Subsequent recurring runs execute rigid commands directly inside native operating system sandboxes with zero LLM tokens, cutting recurring token consumption by 90% to 95%.

---

## Key Capabilities

1. **Zero-CGO Single-Binary Architecture**:
   - Compiles into a self-contained executable under 25MB with no external C runtime dependencies.
   - Starts in under 10 milliseconds with an idle resident memory footprint under 25MB RAM.
   - Eliminates heavy Node.js 24+ runtimes and mandatory Docker daemon prerequisites.

2. **Trajectory-to-Workflow Compiler**:
   - Ingests exploratory multi-turn agent traces (ReAct JSON format and Claude Code JSONL transcripts).
   - Constructs a causal dependency DAG identifying which actions contributed to final validated outcomes.
   - Prunes redundant duplicate file reads, failed directory probes, and non-contributing exploration branches.
   - Synthesizes deterministic playbooks with automated verification gates (`exit_code == 0`) and dynamic input parameterization (`${{ inputs.branch }}`).

3. **Durable Step Ledger (Pure Go SQLite WAL)**:
   - Logs every step, tool call, process output, environment change, and checkpoint directly into an embedded SQLite database using Write-Ahead Logging (WAL).
   - Recovers interrupted or crashed executions immediately without duplicating completed steps or burning additional tokens.

4. **Native Operating System Sandboxing**:
   - Windows: Employs Windows Job Objects (`windows.CreateJobObject`) to enforce hard memory caps and guarantee process-tree termination on completion.
   - Linux: Utilizes Linux namespaces and process isolation.
   - Cross-platform direct runner with environment sanitization and execution timeouts.

5. **Automated Model Handoff**:
   - Routes initial zero-shot exploration to frontier models (Claude 3.5 Sonnet, GPT-4o).
   - Automatically shifts recurring executions to deterministic sandbox scripts or fast, low-cost models (Gemini Flash, Llama 3.1 8B, local Ollama).

6. **Embedded React 19 Control Plane Studio**:
   - Embedded directly into the Go binary via `//go:embed` and served locally on port `7777`.
   - Visualizes live step execution DAGs, trajectory diff comparisons, real-time Server-Sent Events (SSE) terminal logs, and Human-in-the-Loop approval gates.

---

## System Architecture

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
       |  |  - Linux: Namespaces and Process Isolation            |  |
       |  |  - Direct: Isolated process runner with env filters   |  |
       |  +-------------------------------------------------------+  |
       |                              |                              |
       |                              v                              |
       |  +-------------------------------------------------------+  |
       |  |          Trajectory-to-Workflow Compiler              |  |
       |  |  - Ingestion Adapters (Claude JSONL, ReAct traces)    |  |
       |  |  - Causal Dependency Graph Builder (DAG)              |  |
       |  |  - Dead-End Pruner and Redundancy Stripper            |  |
       |  |  - Deterministic Playbook Synthesizer (workflow.yaml) |  |
       |  |  - Native Script Generator (Bash, Python, PowerShell) |  |
       |  +-------------------------------------------------------+  |
       |                              |                              |
       +------------------------------|------------------------------+
                                       |
                                       v
       +-------------------------------------------------------------+
       |                  Enterprise Control Surfaces                |
       |  1. Embedded React 19 Studio (Local HTTP :7777)             |
       |  2. Compiled Deterministic Playbook (workflow.yaml)         |
       |  3. Prometheus Metrics and Non-Repudiable SQLite Audit Log  |
       +-------------------------------------------------------------+
```

---

## Installation and Quick Start

### Build from Source

Requirements: Go 1.22+ and Node.js 20+ (for building the web assets).

```bash
# Clone the repository
git clone https://github.com/alexandrmotologa/runforge.git
cd runforge

# Build the embedded React 19 studio and Go executable
make build

# Or compile the Go executable directly if web assets are bundled
go build -o runforge.exe ./cmd/runforge
```

### CLI Usage

#### 1. Compile an Agent Trajectory
Ingest a recorded trace and synthesize a deterministic playbook and native script:

```bash
runforge compile examples/sample_trajectory.json \
  -o workflow.yaml \
  --script run_task.sh
```

Sample output:
```
[OK] Wrote compiled playbook to workflow.yaml
[OK] Generated standalone script: run_task.sh

================ Compilation Summary ================
Original Turns:       12
Dead Ends Pruned:     9 (75.0% reduction)
Deterministic Steps:  3
Conserved Tokens:     ~10190 tokens / subsequent run
Net USD Savings:      ~$0.0306 per execution
=====================================================
```

#### 2. Execute a Compiled Playbook
Run the workflow inside the native OS sandbox:

```bash
runforge run workflow.yaml --param branch_name=feat/token-compression
```

#### 3. Start the Control Plane Studio
Launch the local HTTP server and embedded React 19 interface on port 7777:

```bash
runforge serve --port 7777
```
Open `http://localhost:7777` to inspect live DAG progression, savings charts, and approvals.

#### 4. Resume an Interrupted Run
Resume execution from the exact SQLite WAL checkpoint without repeated steps:

```bash
runforge resume <run_id>
```

---

## Playbook Specification (`workflow.yaml`)

Compiled workflows use a clear, declarative schema:

```yaml
version: 1
name: "Audit Pull Request on branch feat/token-compression"
description: "Compiled deterministic workflow synthesized from agent trace"

inputs:
  branch_name:
    type: string
    default: "feat/token-compression"
    description: "Target Git branch name"

steps:
  - id: step_01
    name: "Run git"
    type: command
    run: "git status --porcelain"
    assertions:
      - "exit_code == 0"

  - id: step_02
    name: "Run git"
    type: command
    run: "git checkout ${{ inputs.branch_name }}"
    assertions:
      - "exit_code == 0"
    depends_on:
      - step_01

  - id: step_03
    name: "Run go test"
    type: command
    run: "go test ./..."
    assertions:
      - "exit_code == 0"
    depends_on:
      - step_02

metadata:
  compiled_at: "2026-11-17T20:17:27Z"
  original_tokens: 65350
  compiled_tokens: 0
  estimated_savings_pct: 75.0
  source_agent: "Claude Code"
```

---

## Token Cost Compression Formulation

Let $C_{\text{frontier}}$ denote the cost per token of the exploratory model, $C_{\text{slm}}$ the cost per token of the lightweight model, and $N_{\text{runs}}$ the total execution count:

$$\text{Cost}_{\text{standard}} = N_{\text{runs}} \cdot \left( T_{\text{explore}} \cdot C_{\text{frontier}} \right)$$

$$\text{Cost}_{\text{runforge}} = \left( T_{\text{explore}} \cdot C_{\text{frontier}} \right) + (N_{\text{runs}} - 1) \cdot \left( T_{\text{semantic}} \cdot C_{\text{slm}} + T_{\text{code}} \cdot 0 \right)$$

Because compiled deterministic commands execute natively within the sandbox without token consumption ($T_{\text{code}} \cdot 0$), recurring token expenditure approaches zero for rigid steps, yielding:

$$\text{Net Savings Ratio } R_{\text{savings}} \ge 0.90 \quad (\text{for } N_{\text{runs}} \ge 10)$$

---

## License

MIT License. See [LICENSE](LICENSE) for details.
