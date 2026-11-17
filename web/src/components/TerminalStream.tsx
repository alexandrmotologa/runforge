import React, { useEffect, useRef } from 'react';
import { Terminal, Trash2 } from 'lucide-react';

interface TerminalStreamProps {
  logs: Array<{ timestamp: string; message: string; type?: string }>;
  onClear: () => void;
}

export const TerminalStream: React.FC<TerminalStreamProps> = ({ logs, onClear }) => {
  const terminalEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    terminalEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [logs]);

  return (
    <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden flex flex-col h-[320px]">
      <div className="px-4 py-2.5 bg-slate-950 border-b border-slate-800 flex items-center justify-between">
        <div className="flex items-center space-x-2">
          <Terminal className="w-4 h-4 text-emerald-400" />
          <span className="text-xs font-mono font-medium text-slate-300">
            Durable Execution Stream (SSE)
          </span>
        </div>

        <div className="flex items-center space-x-2">
          <button
            onClick={onClear}
            className="p-1 hover:bg-slate-800 text-slate-400 hover:text-slate-200 rounded text-xs flex items-center space-x-1"
            title="Clear terminal logs"
          >
            <Trash2 className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      <div className="flex-1 p-4 overflow-y-auto font-mono text-xs space-y-1 bg-slate-950/80 text-slate-300">
        {logs.length === 0 ? (
          <div className="text-slate-600 italic">Awaiting execution events from Go control plane...</div>
        ) : (
          logs.map((log, idx) => (
            <div key={idx} className="leading-relaxed flex items-start space-x-2">
              <span className="text-slate-600 select-none text-[10px]">{log.timestamp}</span>
              <span
                className={`${
                  log.type === 'error'
                    ? 'text-rose-400'
                    : log.type === 'success'
                    ? 'text-emerald-400 font-semibold'
                    : log.type === 'approval'
                    ? 'text-amber-400 font-semibold'
                    : 'text-slate-300'
                }`}
              >
                {log.message}
              </span>
            </div>
          ))
        )}
        <div ref={terminalEndRef} />
      </div>
    </div>
  );
};
