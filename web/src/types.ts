export type RunStatus = 'pending' | 'running' | 'paused' | 'waiting_approval' | 'completed' | 'failed';
export type RunMode = 'exploratory' | 'compiled' | 'hybrid';
export type StepStatus = 'pending' | 'running' | 'success' | 'failed' | 'skipped';

export interface Run {
  id: string;
  workflow_id: string;
  name: string;
  status: RunStatus;
  mode: RunMode;
  model: string;
  prompt: string;
  input_vars: Record<string, any>;
  tokens_in: number;
  tokens_out: number;
  tokens_saved: number;
  cost_usd: number;
  cost_saved: number;
  created_at: string;
  updated_at: string;
}

export interface Step {
  id: string;
  run_id: string;
  step_index: number;
  name: string;
  type: string;
  tool_name: string;
  input_payload: string;
  output_payload: string;
  exit_code: number;
  duration_ms: number;
  status: StepStatus;
  is_pruned: boolean;
  causal_deps: string[];
  created_at: string;
}

export interface Approval {
  id: string;
  run_id: string;
  step_index: number;
  command: string;
  reason: string;
  status: 'pending' | 'approved' | 'rejected';
  created_at: string;
}

export interface AggregateStats {
  total_runs: number;
  active_runs: number;
  completed_runs: number;
  failed_runs: number;
  total_tokens_in: number;
  total_tokens_out: number;
  total_tokens_saved: number;
  total_cost_usd: number;
  total_cost_saved: number;
  average_savings_pct: number;
}

export interface PruningReport {
  original_step_count: number;
  pruned_step_count: number;
  retained_step_count: number;
  reduction_percent: number;
  estimated_tokens_in: number;
  estimated_tokens_saved: number;
  pruned_steps: any[];
  retained_steps: any[];
}

export interface CompilationResult {
  trace: any;
  pruning_report: PruningReport;
  playbook: any;
  yaml: string;
  scripts: {
    bash: string;
    python: string;
    powershell: string;
  };
}
