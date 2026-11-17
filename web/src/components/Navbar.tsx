import React from 'react';
import { Cpu, Zap, Database, Activity, Code2 } from 'lucide-react';

interface NavbarProps {
  activeTab: 'dashboard' | 'compiler';
  setActiveTab: (tab: 'dashboard' | 'compiler') => void;
  connected: boolean;
}

export const Navbar: React.FC<NavbarProps> = ({ activeTab, setActiveTab, connected }) => {
  return (
    <header className="border-b border-slate-800 bg-slate-900/60 backdrop-blur sticky top-0 z-50">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
        <div className="flex items-center space-x-6">
          <div className="flex items-center space-x-3">
            <div className="w-9 h-9 rounded-lg bg-emerald-500/10 border border-emerald-500/30 flex items-center justify-center text-emerald-400 font-bold shadow-lg shadow-emerald-500/10">
              <Zap className="w-5 h-5 text-emerald-400" />
            </div>
            <div>
              <span className="text-lg font-bold tracking-tight text-white">runforge</span>
              <span className="ml-2 text-xs font-mono uppercase bg-slate-800 text-emerald-400 px-2 py-0.5 rounded border border-slate-700">
                v1.0.0
              </span>
            </div>
          </div>

          <nav className="flex space-x-2">
            <button
              onClick={() => setActiveTab('dashboard')}
              className={`flex items-center space-x-2 px-3 py-1.5 rounded-md text-sm font-medium transition ${
                activeTab === 'dashboard'
                  ? 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'
              }`}
            >
              <Activity className="w-4 h-4" />
              <span>Control Plane</span>
            </button>
            <button
              onClick={() => setActiveTab('compiler')}
              className={`flex items-center space-x-2 px-3 py-1.5 rounded-md text-sm font-medium transition ${
                activeTab === 'compiler'
                  ? 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'
              }`}
            >
              <Code2 className="w-4 h-4" />
              <span>Trajectory Compiler</span>
            </button>
          </nav>
        </div>

        <div className="flex items-center space-x-4">
          <div className="hidden md:flex items-center space-x-3 text-xs font-mono text-slate-400">
            <span className="flex items-center space-x-1 bg-slate-800/70 border border-slate-700/60 px-2.5 py-1 rounded-md">
              <Cpu className="w-3.5 h-3.5 text-slate-400" />
              <span>&lt;25MB RAM</span>
            </span>
            <span className="flex items-center space-x-1 bg-slate-800/70 border border-slate-700/60 px-2.5 py-1 rounded-md">
              <Zap className="w-3.5 h-3.5 text-amber-400" />
              <span>&lt;10ms Start</span>
            </span>
            <span className="flex items-center space-x-1 bg-slate-800/70 border border-slate-700/60 px-2.5 py-1 rounded-md">
              <Database className="w-3.5 h-3.5 text-emerald-400" />
              <span>SQLite WAL</span>
            </span>
          </div>

          <div className="flex items-center space-x-2 bg-slate-900 border border-slate-800 px-3 py-1 rounded-full text-xs">
            <span
              className={`w-2 h-2 rounded-full ${
                connected ? 'bg-emerald-400 animate-pulse' : 'bg-rose-500'
              }`}
            />
            <span className="text-slate-300 font-mono">
              {connected ? 'SSE Active' : 'Disconnected'}
            </span>
          </div>
        </div>
      </div>
    </header>
  );
};
