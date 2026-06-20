"use client";

import { useCallback, useEffect, useState } from "react";
import { RotateCcw } from "lucide-react";

import { useApiClient } from "@/components/app-shell";
import Button from "@/components/ui/button";
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
    <section className="section-card">
      <div className="section-card-head">
        <h3>运行时</h3>
        <Button
          variant="secondary"
          size="sm"
          leftIcon={<RotateCcw size={14} strokeWidth={1.75} />}
          disabled={pendingRestart}
          loading={pendingRestart}
          onClick={() => void handleRestart()}
        >
          {pendingRestart ? "重启中…" : "重启"}
        </Button>
      </div>
      <div className="section-card-body">
        <div className="row-between">
          <div className="stack-sm">
            <span className="meta">状态</span>
            <StatusChip kind="agent" value={runtime.status} size="md" />
          </div>
          <span className="meta" style={{ wordBreak: "break-all", textAlign: "right" }}>
            {runtime.runtimeId ? `ID ${runtime.runtimeId}` : "等待分配…"}
          </span>
        </div>

        {runtime.lastErrorCode ? (
          <div
            className="card"
            style={{
              marginTop: 16,
              padding: "10px 12px",
              borderColor: "color-mix(in oklch, var(--danger) 25%, var(--border))",
              background: "var(--danger-soft)",
              color: "var(--danger)",
            }}
          >
            <p className="meta" style={{ opacity: 0.8 }}>
              {runtime.lastErrorCode}
            </p>
            <p style={{ marginTop: 4, fontSize: 13 }}>
              {runtime.lastErrorMessage || "运行时出错"}
            </p>
          </div>
        ) : null}

        {error ? (
          <div
            role="alert"
            className="card"
            style={{
              marginTop: 16,
              padding: "10px 12px",
              borderColor: "color-mix(in oklch, var(--danger) 25%, var(--border))",
              background: "var(--danger-soft)",
              color: "var(--danger)",
              fontSize: 13,
            }}
          >
            {error}
          </div>
        ) : null}
      </div>
    </section>
  );
}
