import React from 'react';
import { Check, X, ShieldAlert } from 'lucide-react';
import { Approval } from '../types';

interface ApprovalModalProps {
  approval: Approval | null;
  onApprove: (approvalId: string) => void;
  onReject: (approvalId: string) => void;
  onClose: () => void;
}

export const ApprovalModal: React.FC<ApprovalModalProps> = ({
  approval,
  onApprove,
  onReject,
  onClose,
}) => {
  if (!approval) return null;

  return (
    <div
      onClick={onClose}
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm animate-in fade-in"
    >
      <div
        onClick={(e) => e.stopPropagation()}
        className="bg-slate-900 border border-amber-500/40 rounded-2xl max-w-lg w-full p-6 shadow-2xl shadow-amber-500/10 space-y-5"
      >
        <div className="flex items-center space-x-3">
          <div className="w-10 h-10 rounded-xl bg-amber-500/10 border border-amber-500/30 flex items-center justify-center text-amber-400">
            <ShieldAlert className="w-5 h-5" />
          </div>
          <div>
            <h3 className="text-base font-semibold text-white">Operator Confirmation Required</h3>
            <p className="text-xs text-slate-400">Human-in-the-Loop policy gate triggered</p>
          </div>
        </div>

        <div className="bg-slate-950 border border-slate-800 rounded-xl p-4 space-y-3">
          <div>
            <span className="text-[11px] font-mono text-slate-500 block uppercase">Guarded Command:</span>
            <pre className="mt-1 bg-slate-900 border border-slate-800 p-2.5 rounded text-amber-300 font-mono text-xs overflow-x-auto whitespace-pre-wrap">
              {approval.command}
            </pre>
          </div>

          <div>
            <span className="text-[11px] font-mono text-slate-500 block uppercase">Reason:</span>
            <p className="mt-0.5 text-xs text-slate-300">
              {approval.reason || 'This mutating action requires explicit operator approval before the native sandbox proceeds.'}
            </p>
          </div>

          <div className="flex items-center space-x-2 text-[11px] font-mono text-slate-500">
            <span>Run: {approval.run_id}</span>
            <span>•</span>
            <span>Step Index: #{approval.step_index}</span>
          </div>
        </div>

        <div className="flex items-center justify-end space-x-3 pt-2">
          <button
            onClick={() => onReject(approval.id)}
            className="flex items-center space-x-2 px-4 py-2 rounded-lg text-xs font-semibold text-rose-300 hover:text-rose-200 bg-rose-500/10 hover:bg-rose-500/20 border border-rose-500/30 transition"
          >
            <X className="w-4 h-4" />
            <span>Reject Execution</span>
          </button>
          <button
            onClick={() => onApprove(approval.id)}
            className="flex items-center space-x-2 px-4 py-2 rounded-lg text-xs font-semibold text-white bg-emerald-600 hover:bg-emerald-500 shadow-lg shadow-emerald-600/20 transition"
          >
            <Check className="w-4 h-4" />
            <span>Approve & Resume Run</span>
          </button>
        </div>
      </div>
    </div>
  );
};
