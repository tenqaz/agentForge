"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { Bot, ChevronRight, Inbox, Trash2 } from "lucide-react";

import ApiErrorState from "@/components/api-error-state";
import { useApiClient, useSessionState } from "@/components/app-shell";
import Button from "@/components/ui/button";
import EmptyState from "@/components/ui/empty-state";
import Spinner from "@/components/ui/spinner";
import StatusChip from "@/components/ui/status-chip";
import { apiErrorMessage, deleteAgent, listAgents, type Agent } from "@/lib/api";

export default function AgentsPage() {
  const apiClient = useApiClient();
  const router = useRouter();
  const { loading: sessionLoading, user } = useSessionState();
  const [agents, setAgents] = useState<Agent[]>([]);
  const [errorStatus, setErrorStatus] = useState<number>();
  const [error, setError] = useState("");
  const [confirmingId, setConfirmingId] = useState("");
  const [pendingId, setPendingId] = useState("");
  const [fetching, setFetching] = useState(true);

  useEffect(() => {
    if (sessionLoading) {
      return;
    }
    if (!user) {
      router.replace("/login");
      return;
    }

    let active = true;
    void (async () => {
      const response = await listAgents(apiClient);
      if (!active) {
        return;
      }
      setFetching(false);
      if (!response.ok) {
        setErrorStatus(response.status);
        setError(apiErrorMessage(response.error.code, response.error.message));
        return;
      }
      setAgents(response.data.agents);
    })();

    return () => {
      active = false;
    };
  }, [apiClient, sessionLoading, router, user]);

  async function handleDelete(agentId: string) {
    setPendingId(agentId);
    setError("");
    setErrorStatus(undefined);
    const response = await deleteAgent(apiClient, agentId);
    setPendingId("");
    if (!response.ok) {
      setErrorStatus(response.status);
      setError(apiErrorMessage(response.error.code, response.error.message));
      return;
    }
    setConfirmingId("");
    setAgents((current) => current.filter((agent) => agent.id !== agentId));
  }

  return (
    <section className="flex flex-col gap-6">
      <header className="flex flex-col gap-1.5">
        <p className="text-xs font-medium uppercase tracking-wider text-[color:var(--color-fg-subtle)]">
          Agents
        </p>
        <h1 className="text-2xl font-semibold tracking-tight text-[color:var(--color-fg)] sm:text-3xl">
          跟踪运行时进度与配对状态
        </h1>
      </header>

      {error ? <ApiErrorState message={error} status={errorStatus} /> : null}

      {fetching ? (
        <div className="flex items-center gap-2 px-1 text-sm text-[color:var(--color-fg-muted)]">
          <Spinner size="sm" />
          <span>加载中...</span>
        </div>
      ) : agents.length === 0 && !error ? (
        <EmptyState
          icon={<Inbox size={24} strokeWidth={1.5} />}
          title="还没有 Agent"
          description="去 Templates 页面挑选一个模板创建你的第一个 Agent。"
          action={
            <Link href="/templates">
              <Button variant="primary">浏览 Templates</Button>
            </Link>
          }
        />
      ) : (
        <div className="grid gap-3 lg:grid-cols-2">
          {agents.map((agent) => (
            <div
              key={agent.id}
              className="rounded-[var(--radius-xl)] border border-[color:var(--color-border-subtle)] bg-[color:var(--color-bg-elevated)] p-5"
            >
              <Link
                href={`/agents/${agent.id}`}
                className="group block"
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="flex items-start gap-3 min-w-0">
                    <span
                      className="grid size-9 shrink-0 place-items-center rounded-[var(--radius-md)] bg-[color:var(--color-bg-hover)] text-[color:var(--color-fg-muted)] group-hover:text-[color:var(--color-accent)]"
                      aria-hidden="true"
                    >
                      <Bot size={18} strokeWidth={1.75} />
                    </span>
                    <div className="min-w-0">
                      <h2 className="truncate text-base font-semibold text-[color:var(--color-fg)]">
                        {agent.name}
                      </h2>
                      <p className="mt-1 font-mono text-[11px] text-[color:var(--color-fg-subtle)]">
                        Template v{agent.templateVersion}
                      </p>
                    </div>
                  </div>
                  <ChevronRight
                    size={16}
                    strokeWidth={1.75}
                    className="mt-1 shrink-0 text-[color:var(--color-fg-subtle)] transition group-hover:translate-x-0.5 group-hover:text-[color:var(--color-fg-muted)]"
                    aria-hidden="true"
                  />
                </div>
                <div className="mt-4 flex items-center gap-2">
                  <StatusChip kind="agent" value={agent.status} />
                  {agent.lastErrorCode ? (
                    <span className="font-mono text-[11px] text-[color:var(--color-danger)]">
                      {agent.lastErrorCode}
                    </span>
                  ) : null}
                </div>
              </Link>

              <div className="mt-5 flex flex-wrap items-center gap-2 border-t border-[color:var(--color-border-subtle)] pt-4">
                {confirmingId === agent.id ? (
                  <>
                    <Button
                      variant="danger"
                      size="sm"
                      disabled={pendingId === agent.id}
                      loading={pendingId === agent.id}
                      onClick={() => void handleDelete(agent.id)}
                    >
                      {pendingId === agent.id ? "删除中..." : "确认删除"}
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      disabled={pendingId === agent.id}
                      onClick={() => setConfirmingId("")}
                    >
                      取消
                    </Button>
                  </>
                ) : (
                  <Button
                    variant="ghost"
                    size="sm"
                    leftIcon={<Trash2 size={14} strokeWidth={1.75} />}
                    onClick={() => setConfirmingId(agent.id)}
                    className="text-[color:var(--color-danger)] hover:bg-[color:var(--color-danger-soft)] hover:text-[color:var(--color-danger)]"
                  >
                    删除 Agent
                  </Button>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}
