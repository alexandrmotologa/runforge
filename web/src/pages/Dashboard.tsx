import React, { useState } from 'react';
import { RotateCcw, Clock, Search } from 'lucide-react';
import { Run, Step, AggregateStats, Approval } from '../types';
import { SavingsCard } from '../components/SavingsCard';
import { RunTimeline } from '../components/RunTimeline';
import { TerminalStream } from '../components/TerminalStream';
import { ApprovalModal } from '../components/ApprovalModal';

interface DashboardProps {
  stats: AggregateStats | null;
  runs: Run[];
  activeRun: Run | null;
  steps: Step[];
  logs: Array<{ timestamp: string; message: string; type?: string }>;
  onSelectRun: (runId: string) => void;
  onResumeRun: (runId: string) => void;
  onApprove: (approvalId: string) => void;
  onReject: (approvalId: string) => void;
  onClearLogs: () => void;
  pendingApproval: Approval | null;
}

export const Dashboard: React.FC<DashboardProps> = ({
  stats,
  runs,
  activeRun,
  steps,
  logs,
  onSelectRun,
  onResumeRun,
  onApprove,
  onReject,
  onClearLogs,
  pendingApproval,
}) => {
  const [searchTerm, setSearchTerm] = useState('');
  const [statusFilter, setStatusFilter] = useState<string>('all');

  const filteredRuns = runs.filter(r => {
    const matchesSearch = r.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
                          r.id.toLowerCase().includes(searchTerm.toLowerCase());
    const matchesStatus = statusFilter === 'all' || r.status === statusFilter;
    return matchesSearch && matchesStatus;
  });

  return (
    <div className="space-y-6">
      {/* 1. Hero Metrics */}
      <SavingsCard stats={stats} />

      {/* 2. Main Workbench */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left Column: Execution History List */}
        <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden flex flex-col h-[700px]">
          <div className="p-4 border-b border-slate-800 bg-slate-950/60 space-y-3">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-semibold text-white">Execution History</h3>
              <span className="text-xs font-mono text-slate-400 bg-slate-800 px-2 py-0.5 rounded">
                {runs.length} runs recorded
              </span>
            </div>

            <div className="flex space-x-2">
              <div className="relative flex-1">
                <Search className="w-3.5 h-3.5 text-slate-500 absolute left-2.5 top-2.5" />
                <input
                  type="text"
                  placeholder="Filter runs..."
                  value={searchTerm}
                  onChange={(e) => setSearchTerm(e.target.value)}
                  className="w-full bg-slate-900 border border-slate-800 rounded-lg pl-8 pr-3 py-1.5 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-emerald-500/50"
                />
              </div>

              <select
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value)}
                className="bg-slate-900 border border-slate-800 rounded-lg px-2.5 py-1.5 text-xs text-slate-300 focus:outline-none focus:border-emerald-500/50"
              >
                <option value="all">All</option>
                <option value="running">Running</option>
                <option value="waiting_approval">Approval</option>
                <option value="completed">Completed</option>
                <option value="failed">Failed</option>
              </select>
            </div>
          </div>

          <div className="flex-1 overflow-y-auto p-3 space-y-2">
            {filteredRuns.length === 0 ? (
              <div className="text-center py-12 text-slate-500 text-xs">
                No matching runs found in SQLite WAL.
              </div>
            ) : (
              filteredRuns.map((r) => {
                const isSelected = activeRun?.id === r.id;
                return (
                  <div
                    key={r.id}
                    onClick={() => onSelectRun(r.id)}
                    className={`p-3 rounded-lg border text-xs cursor-pointer transition ${
                      isSelected
                        ? 'bg-slate-800/90 border-emerald-500/50 shadow-md shadow-emerald-500/5'
                        : 'bg-slate-950/40 border-slate-800/80 hover:border-slate-700 hover:bg-slate-800/40'
                    }`}
                  >
                    <div className="flex items-center justify-between mb-1.5">
                      <span className="font-mono text-[11px] text-slate-400 truncate max-w-[150px]">
                        {r.id}
                      </span>
                      <span
                        className={`text-[10px] font-mono px-2 py-0.5 rounded-full uppercase tracking-wider font-semibold ${
                          r.status === 'completed'
                            ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                            : r.status === 'running'
                            ? 'bg-sky-500/10 text-sky-400 border border-sky-500/20 animate-pulse'
                            : r.status === 'waiting_approval'
                            ? 'bg-amber-500/10 text-amber-400 border border-amber-500/20 animate-pulse'
                            : 'bg-rose-500/10 text-rose-400 border border-rose-500/20'
                        }`}
                      >
                        {r.status}
                      </span>
                    </div>

                    <div className="font-medium text-slate-200 truncate mb-1">
                      {r.name}
                    </div>

                    <div className="flex items-center justify-between text-[11px] font-mono text-slate-500 mt-2">
                      <span>{r.mode}</span>
                      {r.tokens_saved > 0 && (
                        <span className="text-emerald-400">
                          +{r.tokens_saved.toLocaleString()} tokens saved
                        </span>
                      )}
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </div>

        {/* Right Column: Run Details & Live Timeline */}
        <div className="lg:col-span-2 space-y-6">
          {activeRun ? (
            <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden">
              <div className="p-4 border-b border-slate-800 bg-slate-950/60 flex items-center justify-between">
                <div>
                  <div className="flex items-center space-x-2">
                    <h3 className="text-base font-semibold text-white">{activeRun.name}</h3>
                    <span className="text-xs font-mono bg-slate-800 text-slate-300 px-2 py-0.5 rounded border border-slate-700">
                      {activeRun.mode}
                    </span>
                  </div>
                  <p className="text-xs text-slate-400 mt-0.5 font-mono">{activeRun.id}</p>
                </div>

                <div className="flex items-center space-x-3">
                  {activeRun.status === 'waiting_approval' && (
                    <button
                      onClick={() => onResumeRun(activeRun.id)}
                      className="flex items-center space-x-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold bg-amber-500 text-slate-950 hover:bg-amber-400 transition"
                    >
                      <RotateCcw className="w-3.5 h-3.5" />
                      <span>Review Gate</span>
                    </button>
                  )}
                  {activeRun.status === 'failed' && (
                    <button
                      onClick={() => onResumeRun(activeRun.id)}
                      className="flex items-center space-x-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold bg-emerald-600 text-white hover:bg-emerald-500 transition"
                    >
                      <RotateCcw className="w-3.5 h-3.5" />
                      <span>Resume from SQLite Checkpoint</span>
                    </button>
                  )}
                </div>
              </div>

              {/* Run Metrics Bar */}
              <div className="p-4 grid grid-cols-2 sm:grid-cols-4 gap-3 bg-slate-950/30 border-b border-slate-800 text-xs font-mono">
                <div>
                  <span className="text-slate-500 block text-[10px]">TOKENS USED</span>
                  <span className="text-slate-200 font-bold">
                    {(activeRun.tokens_in + activeRun.tokens_out).toLocaleString()}
                  </span>
                </div>
                <div>
                  <span className="text-slate-500 block text-[10px]">TOKENS SAVED</span>
                  <span className="text-emerald-400 font-bold">
                    {activeRun.tokens_saved.toLocaleString()}
                  </span>
                </div>
                <div>
                  <span className="text-slate-500 block text-[10px]">USD COST</span>
                  <span className="text-slate-200 font-bold">
                    ${activeRun.cost_usd.toFixed(4)}
                  </span>
                </div>
                <div>
                  <span className="text-slate-500 block text-[10px]">NET SAVED</span>
                  <span className="text-emerald-400 font-bold">
                    ${activeRun.cost_saved.toFixed(2)}
                  </span>
                </div>
              </div>

              {/* Execution Steps DAG */}
              <div className="p-5 max-h-[380px] overflow-y-auto">
                <h4 className="text-xs font-semibold uppercase tracking-wider text-slate-400 mb-3">
                  Step Progression DAG ({steps.length} steps)
                </h4>
                <RunTimeline steps={steps} />
              </div>
            </div>
          ) : (
            <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-12 text-center text-slate-400">
              <Clock className="w-10 h-10 mx-auto mb-3 text-slate-600" />
              <h4 className="text-sm font-semibold text-slate-300">Select an execution run</h4>
              <p className="text-xs mt-1 text-slate-500">
                Choose a run from the history ledger to inspect durable steps, checkpoints, and resource metrics.
              </p>
            </div>
          )}

          {/* Live Terminal SSE Stream */}
          <TerminalStream logs={logs} onClear={onClearLogs} />
        </div>
      </div>

      {/* Human In The Loop Modal */}
      <ApprovalModal
        approval={pendingApproval}
        onApprove={onApprove}
        onReject={onReject}
        onClose={() => {}}
      />
    </div>
  );
};
