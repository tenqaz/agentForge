"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { Bot, Inbox, Plus, Trash2 } from "lucide-react";

import ApiErrorState from "@/components/api-error-state";
import { useApiClient, useSessionState } from "@/components/app-shell";
import Button from "@/components/ui/button";
import EmptyState from "@/components/ui/empty-state";
import Modal from "@/components/ui/modal";
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

  const confirmingAgent = agents.find((a) => a.id === confirmingId) ?? null;

  return (
    <>
      <div className="page-head">
        <div>
          <span className="meta">Agents</span>
          <h1>我的 Agents</h1>
          <p className="lead">跟踪运行时进度与配对状态。</p>
        </div>
        <Link href="/agents/new">
          <Button variant="primary" leftIcon={<Plus size={16} strokeWidth={1.75} />}>
            新建 Agent
          </Button>
        </Link>
      </div>

      {error ? <ApiErrorState message={error} status={errorStatus} /> : null}

      {fetching ? (
        <div className="row" style={{ color: "var(--muted)", fontSize: 13 }}>
          <Spinner size="sm" />
          <span>加载中…</span>
        </div>
      ) : agents.length === 0 && !error ? (
        <EmptyState
          icon={<Inbox size={24} strokeWidth={1.5} />}
          title="还没有 Agent"
          description="去挑选一个模板创建你的第一个 Agent。"
          action={
            <Link href="/templates">
              <Button variant="primary">浏览模板</Button>
            </Link>
          }
        />
      ) : (
        <section className="section-card">
          <div className="section-card-body no-pad">
            <table className="table">
              <thead>
                <tr>
                  <th>名称</th>
                  <th>状态</th>
                  <th>模板版本</th>
                  <th>创建于</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {agents.map((agent) => (
                  <tr key={agent.id}>
                    <td>
                      <Link
                        href={`/agents/${agent.id}`}
                        className="row"
                        style={{ gap: 10, alignItems: "center" }}
                      >
                        <span className="avatar" aria-hidden="true">
                          <Bot size={16} strokeWidth={1.75} />
                        </span>
                        <span style={{ fontWeight: 500 }}>{agent.name}</span>
                      </Link>
                    </td>
                    <td>
                      <StatusChip kind="agent" value={agent.status} />
                      {agent.lastErrorCode ? (
                        <span className="meta" style={{ color: "var(--danger)", marginLeft: 8 }}>
                          {agent.lastErrorCode}
                        </span>
                      ) : null}
                    </td>
                    <td className="num-col">v{agent.templateVersion}</td>
                    <td className="meta num-col">{agent.createdAt.slice(0, 10)}</td>
                    <td className="row-actions">
                      <Button
                        variant="ghost"
                        size="sm"
                        leftIcon={<Trash2 size={14} strokeWidth={1.75} />}
                        onClick={() => setConfirmingId(agent.id)}
                        style={{ color: "var(--danger)" }}
                      >
                        删除
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      )}

      <Modal
        open={confirmingId !== ""}
        onClose={() => setConfirmingId("")}
        title={`删除「${confirmingAgent?.name ?? ""}」？`}
        footer={
          <>
            <Button variant="ghost" size="sm" onClick={() => setConfirmingId("")}>
              取消
            </Button>
            <Button
              variant="danger"
              size="sm"
              disabled={pendingId === confirmingId}
              loading={pendingId === confirmingId}
              onClick={() => confirmingId && void handleDelete(confirmingId)}
            >
              {pendingId === confirmingId ? "删除中…" : "确认删除"}
            </Button>
          </>
        }
      >
        <p>
          这会立即停止运行时、解除微信绑定、回收数据目录。Agent 的人格、记忆、对话历史都将无法恢复。
        </p>
      </Modal>
    </>
  );
}
