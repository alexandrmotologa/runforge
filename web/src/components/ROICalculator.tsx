import React, { useState } from 'react';
import { Calculator, TrendingUp, Sparkles } from 'lucide-react';

export const ROICalculator: React.FC = () => {
  const [runsPerWeek, setRunsPerWeek] = useState<number>(100);
  const [tokensPerRun, setTokensPerRun] = useState<number>(45000);
  const [modelTier, setModelTier] = useState<string>('claude-3-5-sonnet');

  const pricingMap: Record<string, { inPerM: number; outPerM: number; label: string }> = {
    'claude-3-5-sonnet': { inPerM: 3.0, outPerM: 15.0, label: 'Claude 3.5 Sonnet ($3 / $15 per 1M)' },
    'gpt-4o': { inPerM: 2.5, outPerM: 10.0, label: 'GPT-4o ($2.50 / $10 per 1M)' },
    'o1': { inPerM: 15.0, outPerM: 60.0, label: 'OpenAI o1 ($15 / $60 per 1M)' },
  };

  const currentPricing = pricingMap[modelTier] || pricingMap['claude-3-5-sonnet'];

  // Assume 85% input tokens, 15% output tokens
  const costPerToken = ((0.85 * currentPricing.inPerM) + (0.15 * currentPricing.outPerM)) / 1000000;
  const singleRunCost = tokensPerRun * costPerToken;

  const weeklyStandardCost = runsPerWeek * singleRunCost;
  const monthlyStandardCost = weeklyStandardCost * 4.33;
  const annualStandardCost = monthlyStandardCost * 12;

  // runforge compiles repetitive steps to 0 tokens, leaving only ~5% for edge case semantic steps
  const compressionRatio = 0.94; // 94% average savings
  const monthlySavings = monthlyStandardCost * compressionRatio;
  const annualSavings = annualStandardCost * compressionRatio;
  const tokensSavedMonthly = (runsPerWeek * tokensPerRun * 4.33) * compressionRatio;

  return (
    <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 space-y-5">
      <div className="flex items-center justify-between border-b border-slate-800 pb-3">
        <div className="flex items-center space-x-2.5">
          <div className="w-8 h-8 rounded-lg bg-emerald-500/10 border border-emerald-500/30 flex items-center justify-center text-emerald-400">
            <Calculator className="w-4 h-4" />
          </div>
          <div>
            <h3 className="text-sm font-semibold text-white">Enterprise ROI & Token Savings Calculator</h3>
            <p className="text-xs text-slate-400">Simulate financial cost compression across recurring autonomous workflows</p>
          </div>
        </div>
        <span className="text-xs font-mono bg-emerald-500/10 text-emerald-400 px-2 py-0.5 rounded border border-emerald-500/20">
          94% Token Reduction
        </span>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {/* Input 1: Runs per week */}
        <div>
          <label className="text-[11px] font-mono text-slate-400 uppercase tracking-wider block mb-1">
            Runs Per Week: <span className="text-emerald-400 font-bold">{runsPerWeek}</span>
          </label>
          <input
            type="range"
            min={10}
            max={1000}
            step={10}
            value={runsPerWeek}
            onChange={(e) => setRunsPerWeek(Number(e.target.value))}
            className="w-full accent-emerald-500 bg-slate-800 rounded h-1.5 cursor-pointer"
          />
          <div className="flex justify-between text-[10px] text-slate-500 font-mono mt-1">
            <span>10</span>
            <span>500</span>
            <span>1000+</span>
          </div>
        </div>

        {/* Input 2: Tokens per exploratory run */}
        <div>
          <label className="text-[11px] font-mono text-slate-400 uppercase tracking-wider block mb-1">
            Tokens Per Run: <span className="text-emerald-400 font-bold">{tokensPerRun.toLocaleString()}</span>
          </label>
          <input
            type="range"
            min={5000}
            max={150000}
            step={5000}
            value={tokensPerRun}
            onChange={(e) => setTokensPerRun(Number(e.target.value))}
            className="w-full accent-emerald-500 bg-slate-800 rounded h-1.5 cursor-pointer"
          />
          <div className="flex justify-between text-[10px] text-slate-500 font-mono mt-1">
            <span>5k</span>
            <span>75k</span>
            <span>150k</span>
          </div>
        </div>

        {/* Input 3: Model Tier */}
        <div>
          <label className="text-[11px] font-mono text-slate-400 uppercase tracking-wider block mb-1">
            Frontier Model Tier:
          </label>
          <select
            value={modelTier}
            onChange={(e) => setModelTier(e.target.value)}
            className="w-full bg-slate-950 border border-slate-800 rounded-lg px-2.5 py-1.5 text-xs text-slate-200 focus:outline-none focus:border-emerald-500/50"
          >
            {Object.entries(pricingMap).map(([key, item]) => (
              <option key={key} value={key}>
                {item.label}
              </option>
            ))}
          </select>
        </div>
      </div>

      {/* Projection Metric Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 pt-2">
        <div className="bg-slate-950/60 border border-slate-800 rounded-lg p-3.5">
          <span className="text-[10px] font-mono text-slate-500 uppercase block">Monthly Standard Spend</span>
          <div className="mt-1 flex items-baseline space-x-1">
            <span className="text-xl font-bold font-mono text-rose-400">${monthlyStandardCost.toFixed(2)}</span>
            <span className="text-xs text-slate-500">/ mo</span>
          </div>
          <span className="text-[10px] text-slate-500 mt-1 block">Uncompressed exploratory agent loops</span>
        </div>

        <div className="bg-emerald-950/20 border border-emerald-500/30 rounded-lg p-3.5">
          <span className="text-[10px] font-mono text-emerald-400 uppercase block flex items-center space-x-1">
            <Sparkles className="w-3 h-3" />
            <span>Monthly Net Savings</span>
          </span>
          <div className="mt-1 flex items-baseline space-x-1">
            <span className="text-xl font-bold font-mono text-emerald-400">${monthlySavings.toFixed(2)}</span>
            <span className="text-xs text-emerald-500/80">/ mo</span>
          </div>
          <span className="text-[10px] text-slate-400 mt-1 block">
            ~{(tokensSavedMonthly / 1000000).toFixed(1)}M tokens conserved monthly
          </span>
        </div>

        <div className="bg-slate-950/60 border border-slate-800 rounded-lg p-3.5">
          <span className="text-[10px] font-mono text-slate-500 uppercase block flex items-center space-x-1">
            <TrendingUp className="w-3 h-3 text-emerald-400" />
            <span>Projected Annual ROI</span>
          </span>
          <div className="mt-1 flex items-baseline space-x-1">
            <span className="text-xl font-bold font-mono text-white">${annualSavings.toFixed(2)}</span>
            <span className="text-xs text-slate-500">/ year</span>
          </div>
          <span className="text-[10px] text-slate-400 mt-1 block">Direct cloud API cost reduction</span>
        </div>
      </div>
    </div>
  );
};
