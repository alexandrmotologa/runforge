import React, { useState } from 'react';
import { Play, Sparkles, Check, FileCode, Terminal } from 'lucide-react';
import { CompilationResult } from '../types';
import { TrajectoryDiff } from '../components/TrajectoryDiff';

const SAMPLE_TRAJECTORY = {
  id: "trace-code-review-01",
  goal: "Review PR #42 on branch feat/login-security and verify tests pass",
  model: "claude-3-5-sonnet",
  steps: [
    { index: 0, tool: "bash", command: "git status", exit_code: 0, tokens_in: 1200, tokens_out: 150 },
    { index: 1, tool: "read_file", path: "package.json", exit_code: 0, tokens_in: 3200, tokens_out: 180 },
    { index: 2, tool: "read_file", path: "package.json", exit_code: 0, tokens_in: 3200, tokens_out: 180 },
    { index: 3, tool: "bash", command: "cat non_existent_file.json", exit_code: 1, tokens_in: 1400, tokens_out: 90 },
    { index: 4, tool: "bash", command: "git checkout feat/login-security", exit_code: 0, tokens_in: 1800, tokens_out: 120 },
    { index: 5, tool: "read_file", path: "src/auth.ts", exit_code: 0, tokens_in: 4500, tokens_out: 320 },
    { index: 6, tool: "bash", command: "go test ./...", exit_code: 0, tokens_in: 4000, tokens_out: 250 }
  ]
};

interface CompilerViewProps {
  onCompile: (jsonText: string) => Promise<CompilationResult | null>;
  onExecutePlaybook: (playbook: any, yaml: string) => void;
}

export const CompilerView: React.FC<CompilerViewProps> = ({ onCompile, onExecutePlaybook }) => {
  const [inputJSON, setInputJSON] = useState<string>(JSON.stringify(SAMPLE_TRAJECTORY, null, 2));
  const [compilation, setCompilation] = useState<CompilationResult | null>(null);
  const [loading, setLoading] = useState<boolean>(false);
  const [copiedScript, setCopiedScript] = useState<string | null>(null);

  const handleCompile = async () => {
    setLoading(true);
    try {
      const res = await onCompile(inputJSON);
      if (res) {
        setCompilation(res);
      }
    } finally {
      setLoading(false);
    }
  };

  const handleCopy = (text: string, label: string) => {
    navigator.clipboard.writeText(text);
    setCopiedScript(label);
    setTimeout(() => setCopiedScript(null), 2000);
  };

  return (
    <div className="space-y-6">
      {/* Header and Controls */}
      <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 space-y-4">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div>
            <h3 className="text-base font-semibold text-white flex items-center space-x-2">
              <Sparkles className="w-5 h-5 text-emerald-400" />
              <span>Trajectory-to-Workflow Compiler</span>
            </h3>
            <p className="text-xs text-slate-400 mt-1">
              Ingest raw multi-turn agent traces (Claude Code, Cursor, ReAct). The compiler builds a causal DAG, prunes dead ends, extracts dynamic variables, and synthesizes a deterministic playbook.
            </p>
          </div>

          <div className="flex items-center space-x-3">
            <button
              onClick={() => setInputJSON(JSON.stringify(SAMPLE_TRAJECTORY, null, 2))}
              className="px-3 py-1.5 rounded-lg text-xs font-mono text-slate-300 bg-slate-800 hover:bg-slate-700 transition"
            >
              Load Sample Trace
            </button>
            <button
              onClick={handleCompile}
              disabled={loading}
              className="flex items-center space-x-2 px-4 py-2 rounded-lg text-xs font-semibold text-white bg-emerald-600 hover:bg-emerald-500 shadow-lg shadow-emerald-600/20 transition disabled:opacity-50"
            >
              <Sparkles className="w-4 h-4" />
              <span>{loading ? 'Compiling DAG...' : 'Compile Playbook'}</span>
            </button>
          </div>
        </div>

        {/* Input Textarea */}
        <div>
          <label className="text-[11px] font-mono text-slate-400 uppercase tracking-wider block mb-1.5">
            Raw Agent Trajectory (JSON or JSONL):
          </label>
          <textarea
            value={inputJSON}
            onChange={(e) => setInputJSON(e.target.value)}
            rows={8}
            className="w-full bg-slate-950 border border-slate-800 rounded-lg p-3 font-mono text-xs text-slate-200 focus:outline-none focus:border-emerald-500/50"
            placeholder="Paste agent execution trace JSON or JSONL..."
          />
        </div>
      </div>

      {/* Trajectory Diff & Generated Artifacts */}
      {compilation && (
        <div className="space-y-6">
          <div className="flex items-center justify-between bg-slate-900 border border-slate-800 p-4 rounded-xl">
            <div>
              <span className="text-xs font-medium text-slate-300">Deterministic Artifacts Ready</span>
              <p className="text-xs text-slate-500">
                Playbook compiled with {compilation.pruning_report.pruned_step_count} dead ends pruned.
              </p>
            </div>

            <div className="flex items-center space-x-3">
              <button
                onClick={() => handleCopy(compilation.yaml, 'yaml')}
                className="flex items-center space-x-1.5 px-3 py-1.5 rounded-lg text-xs font-mono bg-slate-800 text-slate-200 hover:bg-slate-700 transition"
              >
                {copiedScript === 'yaml' ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <FileCode className="w-3.5 h-3.5" />}
                <span>Copy YAML</span>
              </button>

              <button
                onClick={() => handleCopy(compilation.scripts.bash, 'bash')}
                className="flex items-center space-x-1.5 px-3 py-1.5 rounded-lg text-xs font-mono bg-slate-800 text-slate-200 hover:bg-slate-700 transition"
              >
                {copiedScript === 'bash' ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Terminal className="w-3.5 h-3.5" />}
                <span>Copy Bash</span>
              </button>

              <button
                onClick={() => onExecutePlaybook(compilation.playbook, compilation.yaml)}
                className="flex items-center space-x-2 px-4 py-2 rounded-lg text-xs font-semibold text-slate-950 bg-emerald-400 hover:bg-emerald-300 shadow-lg shadow-emerald-400/20 transition"
              >
                <Play className="w-4 h-4 fill-current" />
                <span>Execute in Sandbox (0 Tokens)</span>
              </button>
            </div>
          </div>

          <TrajectoryDiff compilation={compilation} />
        </div>
      )}
    </div>
  );
};
