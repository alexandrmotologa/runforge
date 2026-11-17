import React, { useState, useEffect, useCallback } from 'react';
import { Navbar } from './components/Navbar';
import { Dashboard } from './pages/Dashboard';
import { CompilerView } from './pages/CompilerView';
import { Run, Step, AggregateStats, Approval, CompilationResult } from './types';

export const App: React.FC = () => {
  const [activeTab, setActiveTab] = useState<'dashboard' | 'compiler'>('dashboard');
  const [connected, setConnected] = useState<boolean>(false);
  const [stats, setStats] = useState<AggregateStats | null>(null);
  const [runs, setRuns] = useState<Run[]>([]);
  const [selectedRunId, setSelectedRunId] = useState<string>('');
  const [activeRun, setActiveRun] = useState<Run | null>(null);
  const [steps, setSteps] = useState<Step[]>([]);
  const [pendingApproval, setPendingApproval] = useState<Approval | null>(null);
  const [logs, setLogs] = useState<Array<{ timestamp: string; message: string; type?: string }>>([]);

  const addLog = useCallback((message: string, type: string = 'info') => {
    const timestamp = new Date().toLocaleTimeString();
    setLogs(prev => [...prev.slice(-300), { timestamp, message, type }]);
  }, []);

  const fetchStats = useCallback(async () => {
    try {
      const res = await fetch('/api/stats');
      if (res.ok) {
        const data = await res.json();
        setStats(data);
      }
    } catch {
      // Ignore network errors in polling
    }
  }, []);

  const fetchRuns = useCallback(async () => {
    try {
      const res = await fetch('/api/runs');
      if (res.ok) {
        const data = await res.json();
        setRuns(data);
        if (data.length > 0 && !selectedRunId) {
          setSelectedRunId(data[0].id);
        }
      }
    } catch {
      // Ignore network errors in polling
    }
  }, [selectedRunId]);

  const fetchRunDetails = useCallback(async (id: string) => {
    if (!id) return;
    try {
      const res = await fetch(`/api/runs/${id}`);
      if (res.ok) {
        const data = await res.json();
        setActiveRun(data.run);
        setSteps(data.steps || []);
        if (data.pending_approvals && data.pending_approvals.length > 0) {
          setPendingApproval(data.pending_approvals[0]);
        } else {
          setPendingApproval(null);
        }
      }
    } catch {
      // Ignore
    }
  }, []);

  // Initial load
  useEffect(() => {
    fetchStats();
    fetchRuns();
    const interval = setInterval(() => {
      fetchStats();
      fetchRuns();
    }, 5000);
    return () => clearInterval(interval);
  }, [fetchStats, fetchRuns]);

  // When selected run changes
  useEffect(() => {
    if (selectedRunId) {
      fetchRunDetails(selectedRunId);
    }
  }, [selectedRunId, fetchRunDetails]);

  // Connect to SSE stream
  useEffect(() => {
    const eventSource = new EventSource('/api/events');

    eventSource.onopen = () => {
      setConnected(true);
      addLog('Connected to runforge SSE stream', 'success');
    };

    eventSource.onerror = () => {
      setConnected(false);
    };

    eventSource.addEventListener('run_started', (e: MessageEvent) => {
      const data = JSON.parse(e.data);
      addLog(`Run started: ${data.run_id}`, 'info');
      fetchRuns();
      fetchStats();
      if (selectedRunId === data.run_id) {
        fetchRunDetails(data.run_id);
      }
    });

    eventSource.addEventListener('step_started', (e: MessageEvent) => {
      const data = JSON.parse(e.data);
      addLog(`Step #${data.step_index} started: ${data.data?.name}`, 'info');
      if (selectedRunId === data.run_id) {
        fetchRunDetails(data.run_id);
      }
    });

    eventSource.addEventListener('step_completed', (e: MessageEvent) => {
      const data = JSON.parse(e.data);
      addLog(`Step #${data.step_index} completed (${data.data?.duration_ms}ms)`, 'success');
      if (selectedRunId === data.run_id) {
        fetchRunDetails(data.run_id);
      }
    });

    eventSource.addEventListener('approval_required', (e: MessageEvent) => {
      const data = JSON.parse(e.data);
      addLog(`Approval required for step #${data.step_index}`, 'approval');
      if (data.data) {
        setPendingApproval(data.data);
      }
      fetchRuns();
    });

    eventSource.addEventListener('run_completed', (e: MessageEvent) => {
      const data = JSON.parse(e.data);
      addLog(`Run completed: ${data.run_id} (+${data.data?.tokens_saved || 0} tokens saved)`, 'success');
      fetchRuns();
      fetchStats();
      if (selectedRunId === data.run_id) {
        fetchRunDetails(data.run_id);
      }
    });

    eventSource.addEventListener('run_failed', (e: MessageEvent) => {
      const data = JSON.parse(e.data);
      addLog(`Run failed: ${data.run_id}`, 'error');
      fetchRuns();
      fetchStats();
      if (selectedRunId === data.run_id) {
        fetchRunDetails(data.run_id);
      }
    });

    return () => {
      eventSource.close();
    };
  }, [addLog, fetchRuns, fetchStats, fetchRunDetails, selectedRunId]);

  const handleResumeRun = async (runId: string) => {
    try {
      const res = await fetch(`/api/runs/${runId}/resume`, { method: 'POST' });
      if (res.ok) {
        addLog(`Resumed run ${runId}`, 'info');
        fetchRunDetails(runId);
        fetchRuns();
      } else {
        const err = await res.json();
        addLog(`Resume failed: ${err.error}`, 'error');
      }
    } catch (err: any) {
      addLog(`Resume error: ${err.message}`, 'error');
    }
  };

  const handleApprove = async (approvalId: string) => {
    try {
      const res = await fetch(`/api/approvals/${approvalId}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action: 'approve' }),
      });
      if (res.ok) {
        addLog(`Approved action ${approvalId}`, 'success');
        setPendingApproval(null);
        if (selectedRunId) {
          handleResumeRun(selectedRunId);
        }
      }
    } catch (err: any) {
      addLog(`Approval error: ${err.message}`, 'error');
    }
  };

  const handleReject = async (approvalId: string) => {
    try {
      const res = await fetch(`/api/approvals/${approvalId}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action: 'reject' }),
      });
      if (res.ok) {
        addLog(`Rejected action ${approvalId}`, 'error');
        setPendingApproval(null);
        if (selectedRunId) {
          fetchRunDetails(selectedRunId);
        }
      }
    } catch (err: any) {
      addLog(`Reject error: ${err.message}`, 'error');
    }
  };

  const handleCompile = async (jsonText: string): Promise<CompilationResult | null> => {
    try {
      const res = await fetch('/api/compile', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: jsonText,
      });
      if (res.ok) {
        const result: CompilationResult = await res.json();
        addLog(`Compiled trajectory ${result.playbook.name} (Pruned: ${result.pruning_report.pruned_step_count} steps)`, 'success');
        return result;
      } else {
        const err = await res.json();
        addLog(`Compilation failed: ${err.error}`, 'error');
      }
    } catch (err: any) {
      addLog(`Compilation request failed: ${err.message}`, 'error');
    }
    return null;
  };

  const handleExecutePlaybook = async (playbook: any, rawYaml: string) => {
    try {
      const res = await fetch('/api/runs', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ playbook, raw_yaml: rawYaml }),
      });
      if (res.ok) {
        const data = await res.json();
        addLog(`Initiated execution run: ${data.run_id}`, 'info');
        setSelectedRunId(data.run_id);
        setActiveTab('dashboard');
        fetchRuns();
      }
    } catch (err: any) {
      addLog(`Execution trigger failed: ${err.message}`, 'error');
    }
  };

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col">
      <Navbar
        activeTab={activeTab}
        setActiveTab={setActiveTab}
        connected={connected}
      />

      <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-6">
        {activeTab === 'dashboard' ? (
          <Dashboard
            stats={stats}
            runs={runs}
            activeRun={activeRun}
            steps={steps}
            logs={logs}
            onSelectRun={(id) => setSelectedRunId(id)}
            onResumeRun={handleResumeRun}
            onApprove={handleApprove}
            onReject={handleReject}
            onClearLogs={() => setLogs([])}
            pendingApproval={pendingApproval}
          />
        ) : (
          <CompilerView
            onCompile={handleCompile}
            onExecutePlaybook={handleExecutePlaybook}
          />
        )}
      </main>
    </div>
  );
};

export default App;
