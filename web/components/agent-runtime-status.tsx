"use client";

import { useCallback, useEffect, useState } from "react";
import { RotateCcw } from "lucide-react";

import { useApiClient } from "@/components/app-shell";
import Button from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import StatusChip from "@/components/ui/status-chip";
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
    <Card>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-xs font-medium uppercase tracking-wider text-[color:var(--color-fg-subtle)]">
            Runtime
          </p>
          <div className="mt-2 flex items-center gap-2">
            <StatusChip kind="agent" value={runtime.status} size="md" />
          </div>
          <p className="mt-3 break-all font-mono text-[11px] text-[color:var(--color-fg-subtle)]">
            {runtime.runtimeId ? `ID ${runtime.runtimeId}` : "等待分配..."}
          </p>
        </div>
        <Button
          variant="secondary"
          size="sm"
          leftIcon={<RotateCcw size={14} strokeWidth={1.75} />}
          disabled={pendingRestart}
          loading={pendingRestart}
          onClick={() => void handleRestart()}
        >
          {pendingRestart ? "重启中..." : "重启"}
        </Button>
      </div>

      {runtime.lastErrorCode ? (
        <div className="mt-4 rounded-[var(--radius-md)] border border-[color:var(--color-danger)]/25 bg-[color:var(--color-danger-soft)] px-3 py-2.5 text-sm text-[color:var(--color-danger)]">
          <p className="font-mono text-[11px] opacity-80">{runtime.lastErrorCode}</p>
          <p className="mt-1">{runtime.lastErrorMessage || "运行时出错"}</p>
        </div>
      ) : null}

      {error ? (
        <div
          role="alert"
          className="mt-4 rounded-[var(--radius-md)] border border-[color:var(--color-danger)]/25 bg-[color:var(--color-danger-soft)] px-3 py-2.5 text-sm text-[color:var(--color-danger)]"
        >
          {error}
        </div>
      ) : null}
    </Card>
  );
}
