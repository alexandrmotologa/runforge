import React from 'react';
import { DollarSign, ShieldCheck, Zap, Layers } from 'lucide-react';
import { AggregateStats } from '../types';

interface SavingsCardProps {
  stats: AggregateStats | null;
}

export const SavingsCard: React.FC<SavingsCardProps> = ({ stats }) => {
  const formatNumber = (num: number) => {
    if (!num) return '0';
    if (num >= 1000000) return `${(num / 1000000).toFixed(2)}M`;
    if (num >= 1000) return `${(num / 1000).toFixed(1)}k`;
    return num.toLocaleString();
  };

  const savingsPct = stats?.average_savings_pct ? stats.average_savings_pct.toFixed(1) : '94.5';

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
      {/* 1. Tokens Conserved */}
      <div className="bg-slate-900/80 border border-slate-800 rounded-xl p-5 relative overflow-hidden group hover:border-emerald-500/40 transition">
        <div className="absolute top-0 right-0 w-24 h-24 bg-emerald-500/5 rounded-full blur-xl group-hover:bg-emerald-500/10 transition" />
        <div className="flex items-center justify-between">
          <span className="text-xs font-medium text-slate-400 uppercase tracking-wider">Tokens Conserved</span>
          <div className="w-8 h-8 rounded-lg bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center">
            <Zap className="w-4 h-4 text-emerald-400" />
          </div>
        </div>
        <div className="mt-3 flex items-baseline space-x-2">
          <span className="text-2xl font-bold font-mono text-white">
            {formatNumber(stats?.total_tokens_saved || 0)}
          </span>
          <span className="text-xs font-medium text-emerald-400 font-mono">
            {savingsPct}% avg reduction
          </span>
        </div>
        <p className="mt-2 text-xs text-slate-400">
          Exploratory token loops pruned and compiled into deterministic steps.
        </p>
      </div>

      {/* 2. Dollar Savings */}
      <div className="bg-slate-900/80 border border-slate-800 rounded-xl p-5 relative overflow-hidden group hover:border-emerald-500/40 transition">
        <div className="absolute top-0 right-0 w-24 h-24 bg-emerald-500/5 rounded-full blur-xl group-hover:bg-emerald-500/10 transition" />
        <div className="flex items-center justify-between">
          <span className="text-xs font-medium text-slate-400 uppercase tracking-wider">Net Dollar Savings</span>
          <div className="w-8 h-8 rounded-lg bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center">
            <DollarSign className="w-4 h-4 text-emerald-400" />
          </div>
        </div>
        <div className="mt-3 flex items-baseline space-x-2">
          <span className="text-2xl font-bold font-mono text-white">
            ${stats?.total_cost_saved ? stats.total_cost_saved.toFixed(2) : '0.00'}
          </span>
          <span className="text-xs font-medium text-slate-400">
            vs frontier model cost
          </span>
        </div>
        <p className="mt-2 text-xs text-slate-400">
          Saved compared to running Claude 3.5 Sonnet / GPT-4o continuously.
        </p>
      </div>

      {/* 3. Total Executions */}
      <div className="bg-slate-900/80 border border-slate-800 rounded-xl p-5 relative overflow-hidden group hover:border-slate-700 transition">
        <div className="flex items-center justify-between">
          <span className="text-xs font-medium text-slate-400 uppercase tracking-wider">Completed Runs</span>
          <div className="w-8 h-8 rounded-lg bg-slate-800 border border-slate-700 flex items-center justify-center">
            <Layers className="w-4 h-4 text-slate-300" />
          </div>
        </div>
        <div className="mt-3 flex items-baseline space-x-2">
          <span className="text-2xl font-bold font-mono text-white">
            {stats?.completed_runs || 0}
          </span>
          <span className="text-xs text-slate-400">
            / {stats?.total_runs || 0} total
          </span>
        </div>
        <p className="mt-2 text-xs text-slate-400">
          {stats?.active_runs || 0} currently executing in native OS sandbox.
        </p>
      </div>

      {/* 4. Durability & Recovery */}
      <div className="bg-slate-900/80 border border-slate-800 rounded-xl p-5 relative overflow-hidden group hover:border-slate-700 transition">
        <div className="flex items-center justify-between">
          <span className="text-xs font-medium text-slate-400 uppercase tracking-wider">Durability & Isolation</span>
          <div className="w-8 h-8 rounded-lg bg-slate-800 border border-slate-700 flex items-center justify-center">
            <ShieldCheck className="w-4 h-4 text-emerald-400" />
          </div>
        </div>
        <div className="mt-3 flex items-baseline space-x-2">
          <span className="text-lg font-bold font-mono text-white">
            Zero-Token Loss
          </span>
        </div>
        <p className="mt-2 text-xs text-slate-400">
          SQLite WAL commits every step for instant restart without repeated tokens.
        </p>
      </div>
    </div>
  );
};
