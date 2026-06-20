"use client";

import { use } from "react";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { RotateCcw, Trash2 } from "lucide-react";

import ApiErrorState from "@/components/api-error-state";
import { useApiClient, useSessionState } from "@/components/app-shell";
import Breadcrumbs from "@/components/ui/breadcrumbs";
import Button from "@/components/ui/button";
import Modal from "@/components/ui/modal";
import Spinner from "@/components/ui/spinner";
import { apiErrorMessage, deleteAgent, getAgent, getRuntime, restartRuntime, type Agent } from "@/lib/api";

export default function AgentSettingsPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const apiClient = useApiClient();
  const router = useRouter();
  const { loading: sessionLoading, user } = useSessionState();
  const [agent, setAgent] = useState<Agent | null>(null);
  const [fetching, setFetching] = useState(true);
  const [errorStatus, setErrorStatus] = useState<number>();
  const [error, setError] = useState("");
  const [restartOpen, setRestartOpen] = useState(false);
  const [pendingRestart, setPendingRestart] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [pendingDelete, setPendingDelete] = useState(false);

  useEffect(() => {
    if (sessionLoading) return;
    if (!user) {
      router.replace("/login");
      return;
    }
    let active = true;
    void (async () => {
      const response = await getAgent(apiClient, id);
      if (!active) return;
      setFetching(false);
      if (!response.ok) {
        setErrorStatus(response.status);
        setError(apiErrorMessage(response.error.code, response.error.message));
        return;
      }
      setAgent(response.data.agent);
    })();
    return () => {
      active = false;
    };
  }, [apiClient, id, router, sessionLoading, user]);

  async function handleRestart() {
    setPendingRestart(true);
    setError("");
    const response = await restartRuntime(apiClient, id);
    setPendingRestart(false);
    setRestartOpen(false);
    if (!response.ok) {
      setErrorStatus(response.status);
      setError(apiErrorMessage(response.error.code, response.error.message));
      return;
    }
    // 刷新 runtime 状态（getRuntime 仅用于触发后端，结果不影响本页展示）
    await getRuntime(apiClient, id);
  }

  async function handleDelete() {
    setPendingDelete(true);
    setError("");
    const response = await deleteAgent(apiClient, id);
    setPendingDelete(false);
    if (!response.ok) {
      setErrorStatus(response.status);
      setError(apiErrorMessage(response.error.code, response.error.message));
      setDeleteOpen(false);
      return;
    }
    router.push("/agents");
  }

  return (
    <>
      <Breadcrumbs
        items={[
          { label: "我的 Agents", href: "/agents" },
          { label: agent?.name ?? "Agent", href: `/agents/${id}` },
          { label: "设置" },
        ]}
      />

      <div className="page-head">
        <div>
          <h1>Agent 设置</h1>
          <p className="lead">运行时与数据生命周期。注意：删除会立即回收运行时与数据目录，且不可恢复。</p>
        </div>
      </div>

      {error ? <ApiErrorState message={error} status={errorStatus} /> : null}

      {fetching ? (
        <div className="row" style={{ color: "var(--muted)", fontSize: 13 }}>
          <Spinner size="sm" />
          <span>加载中…</span>
        </div>
      ) : agent ? (
        <>
          <section className="section-card">
            <div className="section-card-head">
              <h3 style={{ fontSize: 15, fontWeight: 600 }}>基本</h3>
            </div>
            <div className="section-card-body">
              <div className="form-stack" style={{ maxWidth: 480 }}>
                <div className="field">
                  <label className="field-label">显示名称</label>
                  <input className="input" value={agent.name} disabled />
                  <span className="field-help">当前不支持在控制台改名。</span>
                </div>
                <div className="field">
                  <label className="field-label">基于模板</label>
                  <div className="row" style={{ gap: 10, alignItems: "center" }}>
                    <span className="tag tag-mono">v{agent.templateVersion}</span>
                    <span className="meta">已锁定 · 模板更新不会影响此 Agent</span>
                  </div>
                </div>
              </div>
            </div>
          </section>

          <section className="section-card">
            <div className="section-card-head">
              <h3 style={{ fontSize: 15, fontWeight: 600 }}>运行时</h3>
            </div>
            <div className="section-card-body" style={{ padding: 0 }}>
              <div
                style={{
                  display: "grid",
                  gridTemplateColumns: "1fr auto",
                  gap: 16,
                  padding: "18px 22px",
                  alignItems: "center",
                  borderTop: "1px solid var(--border)",
                }}
              >
                <div>
                  <div style={{ fontSize: 14, fontWeight: 600 }}>重启运行时</div>
                  <p className="muted" style={{ fontSize: 13, lineHeight: 1.55, marginTop: 2 }}>
                    停止当前容器并重新启动。数据目录、记忆、微信凭据完全保留，无需重新扫码。预计耗时 10–30 秒。
                  </p>
                </div>
                <Button
                  variant="secondary"
                  leftIcon={<RotateCcw size={14} strokeWidth={1.75} />}
                  onClick={() => setRestartOpen(true)}
                >
                  重启
                </Button>
              </div>
            </div>
          </section>

          <section
            className="section-card"
            style={{ borderColor: "color-mix(in oklch, var(--danger) 30%, var(--border))" }}
          >
            <div className="section-card-head" style={{ background: "var(--danger-soft)" }}>
              <h3 style={{ fontSize: 15, fontWeight: 600, color: "var(--danger)" }}>危险操作</h3>
              <span className="meta" style={{ color: "var(--danger)" }}>不可恢复</span>
            </div>
            <div className="section-card-body" style={{ padding: 0 }}>
              <div
                style={{
                  display: "grid",
                  gridTemplateColumns: "1fr auto",
                  gap: 16,
                  padding: "18px 22px",
                  alignItems: "center",
                  borderTop: "1px solid var(--border)",
                }}
              >
                <div>
                  <div style={{ fontSize: 14, fontWeight: 600 }}>删除 Agent</div>
                  <p className="muted" style={{ fontSize: 13, lineHeight: 1.55, marginTop: 2 }}>
                    立即停止运行时容器并永久回收数据目录，包括人格、记忆、对话历史与微信凭据。
                    <strong style={{ color: "var(--danger)" }}>不可恢复</strong>。
                  </p>
                </div>
                <Button
                  variant="danger"
                  leftIcon={<Trash2 size={14} strokeWidth={1.75} />}
                  onClick={() => setDeleteOpen(true)}
                >
                  删除 Agent
                </Button>
              </div>
            </div>
          </section>
        </>
      ) : null}

      <Modal
        open={restartOpen}
        onClose={() => setRestartOpen(false)}
        title={`重启「${agent?.name ?? ""}」？`}
        footer={
          <>
            <Button variant="ghost" size="sm" onClick={() => setRestartOpen(false)}>
              取消
            </Button>
            <Button
              variant="primary"
              size="sm"
              disabled={pendingRestart}
              loading={pendingRestart}
              onClick={() => void handleRestart()}
            >
              {pendingRestart ? "重启中…" : "确认重启"}
            </Button>
          </>
        }
      >
        <p>当前运行时容器会被停止，新容器随后启动。期间收到的微信消息会被排队，恢复后再处理。</p>
      </Modal>

      <Modal
        open={deleteOpen}
        onClose={() => setDeleteOpen(false)}
        title={`永久删除「${agent?.name ?? ""}」？`}
        footer={
          <>
            <Button variant="ghost" size="sm" onClick={() => setDeleteOpen(false)}>
              取消
            </Button>
            <Button
              variant="danger"
              size="sm"
              disabled={pendingDelete}
              loading={pendingDelete}
              onClick={() => void handleDelete()}
            >
              {pendingDelete ? "删除中…" : "永久删除"}
            </Button>
          </>
        }
      >
        <p>
          这会立即停止运行时、解除微信绑定、回收数据目录。Agent 的人格、记忆、对话历史、微信凭据都将无法恢复。
        </p>
      </Modal>
    </>
  );
}
