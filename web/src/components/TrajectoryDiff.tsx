import React from 'react';
import { ArrowRight, CheckCircle2, Scissors, Zap } from 'lucide-react';
import { CompilationResult } from '../types';

interface TrajectoryDiffProps {
  compilation: CompilationResult | null;
}

export const TrajectoryDiff: React.FC<TrajectoryDiffProps> = ({ compilation }) => {
  if (!compilation) {
    return (
      <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-8 text-center text-slate-400">
        <Zap className="w-8 h-8 mx-auto mb-2 text-slate-600" />
        <p className="text-sm">Compile or select a trajectory to inspect exploratory vs compiled diff.</p>
      </div>
    );
  }

  const { trace, pruning_report, playbook, yaml } = compilation;
  const originalSteps = trace?.steps || [];
  const compiledSteps = playbook?.steps || [];

  return (
    <div className="space-y-4">
      {/* Reduction Banner */}
      <div className="bg-emerald-500/10 border border-emerald-500/30 rounded-xl p-4 flex items-center justify-between">
        <div className="flex items-center space-x-3">
          <div className="w-10 h-10 rounded-lg bg-emerald-500/20 flex items-center justify-center text-emerald-400">
            <Zap className="w-5 h-5" />
          </div>
          <div>
            <h4 className="text-sm font-semibold text-white">Trajectory Compiler Optimization</h4>
            <p className="text-xs text-slate-300">
              Eliminated {pruning_report.pruned_step_count} dead exploratory steps. Token spend reduced by{' '}
              <span className="font-bold text-emerald-400 font-mono">
                {pruning_report.reduction_percent.toFixed(1)}%
              </span>.
            </p>
          </div>
        </div>

        <div className="flex items-center space-x-4 text-xs font-mono">
          <div className="text-right">
            <span className="text-slate-400 block text-[10px]">BEFORE</span>
            <span className="text-rose-400 font-bold">{originalSteps.length} steps</span>
          </div>
          <ArrowRight className="w-4 h-4 text-slate-500" />
          <div>
            <span className="text-slate-400 block text-[10px]">AFTER</span>
            <span className="text-emerald-400 font-bold">{compiledSteps.length} steps</span>
          </div>
        </div>
      </div>

      {/* Side-by-side comparison */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {/* Left Column: Raw Exploratory Trace */}
        <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden flex flex-col">
          <div className="p-3.5 border-b border-slate-800 bg-slate-950/60 flex items-center justify-between">
            <span className="text-xs font-medium text-slate-300 uppercase tracking-wider flex items-center space-x-2">
              <span className="w-2 h-2 rounded-full bg-rose-400" />
              <span>Exploratory Agent Trajectory</span>
            </span>
            <span className="text-xs font-mono text-slate-400">
              {originalSteps.length} turns
            </span>
          </div>

          <div className="p-4 space-y-2.5 overflow-y-auto max-h-[500px]">
            {originalSteps.map((step: any, idx: number) => {
              const isPruned = step.is_pruned;
              return (
                <div
                  key={idx}
                  className={`p-3 rounded-lg border text-xs font-mono transition ${
                    isPruned
                      ? 'bg-rose-950/20 border-rose-900/40 text-slate-400 opacity-60'
                      : 'bg-slate-950/50 border-slate-800 text-slate-200'
                  }`}
                >
                  <div className="flex items-center justify-between mb-1">
                    <span className="text-slate-500">#{step.index ?? idx}</span>
                    {isPruned ? (
                      <span className="flex items-center space-x-1 text-[10px] text-rose-400 bg-rose-500/10 px-1.5 py-0.5 rounded border border-rose-500/20">
                        <Scissors className="w-3 h-3" />
                        <span>pruned: {step.prune_reason || 'redundant'}</span>
                      </span>
                    ) : (
                      <span className="text-[10px] text-emerald-400 bg-emerald-500/10 px-1.5 py-0.5 rounded">
                        retained
                      </span>
                    )}
                  </div>
                  <div className="truncate font-semibold text-slate-300">
                    {step.tool || 'command'}: {step.command || step.path || 'step'}
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        {/* Right Column: Compiled Deterministic Playbook */}
        <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden flex flex-col">
          <div className="p-3.5 border-b border-slate-800 bg-slate-950/60 flex items-center justify-between">
            <span className="text-xs font-medium text-emerald-400 uppercase tracking-wider flex items-center space-x-2">
              <span className="w-2 h-2 rounded-full bg-emerald-400" />
              <span>Compiled Playbook (workflow.yaml)</span>
            </span>
            <span className="text-xs font-mono text-emerald-400">
              0 tokens / run
            </span>
          </div>

          <div className="p-4 space-y-2.5 overflow-y-auto max-h-[500px]">
            {compiledSteps.map((step: any, idx: number) => (
              <div
                key={idx}
                className="p-3 rounded-lg border border-emerald-500/30 bg-emerald-950/10 text-xs font-mono"
              >
                <div className="flex items-center justify-between mb-1">
                  <span className="text-emerald-400 font-bold">{step.id}</span>
                  <span className="flex items-center space-x-1 text-[10px] text-emerald-400">
                    <CheckCircle2 className="w-3 h-3" />
                    <span>deterministic</span>
                  </span>
                </div>
                <div className="text-slate-200 font-medium truncate mb-1">
                  {step.name}
                </div>
                {step.run && (
                  <pre className="bg-slate-950/80 p-2 rounded text-emerald-300 text-[11px] overflow-x-auto whitespace-pre-wrap">
                    {step.run}
                  </pre>
                )}
                {step.assertions && step.assertions.length > 0 && (
                  <div className="mt-1.5 text-[10px] text-slate-400">
                    assert: {step.assertions.join(', ')}
                  </div>
                )}
              </div>
            ))}

            <div className="mt-4 pt-3 border-t border-slate-800">
              <span className="text-xs text-slate-400 block mb-1">YAML Source:</span>
              <pre className="bg-slate-950 p-3 rounded-lg text-slate-300 text-[11px] overflow-x-auto font-mono max-h-48 whitespace-pre-wrap">
                {yaml}
              </pre>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
