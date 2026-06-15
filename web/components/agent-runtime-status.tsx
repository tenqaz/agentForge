"use client";

import { useCallback, useEffect, useState } from "react";

import { useApiClient } from "@/components/app-shell";
import {
  apiErrorMessage,
  getRuntime,
  restartRuntime,
  type AgentRuntime,
} from "@/lib/api";

export default function AgentRuntimeStatus({
  agentId,
  runtime,
  onRuntimeChange,
}: {
  agentId: string;
  runtime: AgentRuntime;
  onRuntimeChange?: (runtime: AgentRuntime) => void;
}) {
  const apiClient = useApiClient();
  const [pendingRestart, setPendingRestart] = useState(false);
  const [error, setError] = useState("");

  const refresh = useCallback(async () => {
    const response = await getRuntime(apiClient, agentId);
    if (response.ok) {
      onRuntimeChange?.(response.data.runtime);
      setError("");
      return;
    }
    setError(apiErrorMessage(response.error.code, response.error.message));
  }, [agentId, apiClient, onRuntimeChange]);

  useEffect(() => {
    if (runtime.status === "running" || runtime.status === "error") {
      return;
    }

    const timer = window.setInterval(() => {
      void refresh();
    }, 2000);
    return () => window.clearInterval(timer);
  }, [refresh, runtime.status]);

  async function handleRestart() {
    setPendingRestart(true);
    setError("");
    const response = await restartRuntime(apiClient, agentId);
    if (!response.ok) {
      setError(apiErrorMessage(response.error.code, response.error.message));
      setPendingRestart(false);
      return;
    }
    setPendingRestart(false);
    await refresh();
  }

  return (
    <div className="panel rounded-[1.75rem] p-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="eyebrow">Runtime</p>
          <h2 className="mt-3 text-2xl font-semibold text-stone-950">{runtime.status}</h2>
          <p className="mt-2 text-sm text-stone-600">
            Runtime ID: {runtime.runtimeId || "pending"}
          </p>
        </div>
        <button
          className="rounded-full border border-stone-900/15 px-4 py-2 text-xs font-semibold uppercase tracking-[0.16em] text-stone-700 hover:border-stone-900 hover:bg-stone-900 hover:text-stone-50 disabled:cursor-not-allowed disabled:opacity-60"
          disabled={pendingRestart}
          onClick={() => void handleRestart()}
          type="button"
        >
          {pendingRestart ? "Restarting..." : "Restart"}
        </button>
      </div>
      {runtime.lastErrorCode ? (
        <div className="mt-4 rounded-[1.25rem] border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-700">
          {runtime.lastErrorCode}: {runtime.lastErrorMessage || "Runtime error"}
        </div>
      ) : null}
      {error ? (
        <div className="mt-4 rounded-[1.25rem] border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-700">
          {error}
        </div>
      ) : null}
    </div>
  );
}
