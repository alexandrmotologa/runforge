import React, { useState } from 'react';
import { CheckCircle2, Clock, XCircle, AlertCircle, ChevronDown, ChevronRight, Terminal, Scissors } from 'lucide-react';
import { Step } from '../types';

interface RunTimelineProps {
  steps: Step[];
  selectedStepId?: string;
  onSelectStep?: (step: Step) => void;
}

export const RunTimeline: React.FC<RunTimelineProps> = ({ steps, selectedStepId, onSelectStep }) => {
  const [expandedSteps, setExpandedSteps] = useState<Record<string, boolean>>({});

  const toggleExpand = (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    setExpandedSteps(prev => ({ ...prev, [id]: !prev[id] }));
  };

  const getStatusIcon = (status: string, isPruned: boolean) => {
    if (isPruned) {
      return <Scissors className="w-4 h-4 text-amber-400" />;
    }
    switch (status) {
      case 'success':
        return <CheckCircle2 className="w-4 h-4 text-emerald-400" />;
      case 'running':
        return <Clock className="w-4 h-4 text-sky-400 animate-spin" />;
      case 'failed':
        return <XCircle className="w-4 h-4 text-rose-400" />;
      case 'waiting_approval':
        return <AlertCircle className="w-4 h-4 text-amber-400 animate-pulse" />;
      default:
        return <Clock className="w-4 h-4 text-slate-500" />;
    }
  };

  if (!steps || steps.length === 0) {
    return (
      <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-8 text-center text-slate-400">
        <Terminal className="w-8 h-8 mx-auto mb-2 text-slate-600" />
        <p className="text-sm">No steps recorded for this execution.</p>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {steps.map((step, idx) => {
        const isExpanded = !!expandedSteps[step.id];
        const isSelected = selectedStepId === step.id;

        return (
          <div
            key={step.id || idx}
            onClick={() => onSelectStep?.(step)}
            className={`border rounded-lg transition overflow-hidden cursor-pointer ${
              step.is_pruned
                ? 'bg-slate-900/30 border-slate-800/50 opacity-60'
                : isSelected
                ? 'bg-slate-900 border-emerald-500/50 shadow-md shadow-emerald-500/5'
                : 'bg-slate-900/70 border-slate-800 hover:border-slate-700'
            }`}
          >
            <div className="p-3.5 flex items-center justify-between">
              <div className="flex items-center space-x-3 min-w-0">
                <span className="font-mono text-xs text-slate-500 w-6">
                  #{step.step_index}
                </span>

                <div className="flex-shrink-0">
                  {getStatusIcon(step.status, step.is_pruned)}
                </div>

                <div className="min-w-0">
                  <div className="flex items-center space-x-2">
                    <span className="font-medium text-sm text-slate-200 truncate">
                      {step.name}
                    </span>
                    {step.tool_name && (
                      <span className="text-[10px] font-mono bg-slate-800 text-slate-400 px-1.5 py-0.5 rounded border border-slate-700">
                        {step.tool_name}
                      </span>
                    )}
                    {step.is_pruned && (
                      <span className="text-[10px] font-mono bg-amber-500/10 text-amber-400 px-1.5 py-0.5 rounded border border-amber-500/20">
                        pruned
                      </span>
                    )}
                  </div>
                </div>
              </div>

              <div className="flex items-center space-x-3 text-xs font-mono text-slate-400 flex-shrink-0">
                {step.duration_ms > 0 && (
                  <span>{step.duration_ms}ms</span>
                )}
                <button
                  onClick={(e) => toggleExpand(step.id, e)}
                  className="p-1 hover:bg-slate-800 rounded text-slate-400 hover:text-slate-200"
                >
                  {isExpanded ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
                </button>
              </div>
            </div>

            {/* Expandable details */}
            {isExpanded && (
              <div className="border-t border-slate-800/80 bg-slate-950/70 p-3.5 text-xs font-mono space-y-2">
                {step.input_payload && (
                  <div>
                    <span className="text-slate-500 text-[11px] block mb-1">Input / Command:</span>
                    <pre className="bg-slate-900 border border-slate-800 p-2 rounded text-emerald-300 overflow-x-auto whitespace-pre-wrap">
                      {step.input_payload}
                    </pre>
                  </div>
                )}
                {step.output_payload && (
                  <div>
                    <span className="text-slate-500 text-[11px] block mb-1">Output / Result:</span>
                    <pre className="bg-slate-900 border border-slate-800 p-2 rounded text-slate-300 overflow-x-auto max-h-48 whitespace-pre-wrap">
                      {step.output_payload}
                    </pre>
                  </div>
                )}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
};
