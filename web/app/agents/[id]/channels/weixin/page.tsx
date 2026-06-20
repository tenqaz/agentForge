"use client";

import { use } from "react";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { Lock } from "lucide-react";

import ApiErrorState from "@/components/api-error-state";
import { useApiClient, useSessionState } from "@/components/app-shell";
import Breadcrumbs from "@/components/ui/breadcrumbs";
import Button from "@/components/ui/button";
import Modal from "@/components/ui/modal";
import Spinner from "@/components/ui/spinner";
import StatusChip from "@/components/ui/status-chip";
import {
  apiErrorMessage,
  getAgent,
  getWeixinChannel,
  type Agent,
  type Channel,
} from "@/lib/api";

export default function WeixinManagePage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const apiClient = useApiClient();
  const router = useRouter();
  const { loading: sessionLoading, user } = useSessionState();
  const [agent, setAgent] = useState<Agent | null>(null);
  const [channel, setChannel] = useState<Channel | null>(null);
  const [fetching, setFetching] = useState(true);
  const [errorStatus, setErrorStatus] = useState<number>();
  const [error, setError] = useState("");
  const [unbindOpen, setUnbindOpen] = useState(false);
  const [pendingUnbind, setPendingUnbind] = useState(false);

  useEffect(() => {
    if (sessionLoading) return;
    if (!user) {
      router.replace("/login");
      return;
    }
    let active = true;
    void (async () => {
      const [agentRes, channelRes] = await Promise.all([
        getAgent(apiClient, id),
        getWeixinChannel(apiClient, id),
      ]);
      if (!active) return;
      setFetching(false);
      if (!agentRes.ok) {
        setErrorStatus(agentRes.status);
        setError(apiErrorMessage(agentRes.error.code, agentRes.error.message));
        return;
      }
      setAgent(agentRes.data.agent);
      if (channelRes.ok) setChannel(channelRes.data.channel);
    })();
    return () => {
      active = false;
    };
  }, [apiClient, id, router, sessionLoading, user]);

  async function handleUnbind() {
    setPendingUnbind(true);
    setError("");
    const response = await apiClient.delete(`/api/agents/${id}/channels/weixin`);
    setPendingUnbind(false);
    if (!response.ok) {
      setErrorStatus(response.status);
      setError(apiErrorMessage(response.error.code, response.error.message));
      setUnbindOpen(false);
      return;
    }
    setUnbindOpen(false);
    // 刷新渠道状态
    const channelRes = await getWeixinChannel(apiClient, id);
    if (channelRes.ok) setChannel(channelRes.data.channel);
  }

  const connected = channel?.status === "connected";
  const notConfigured = !channel || channel.status === "not_configured";

  return (
    <>
      <Breadcrumbs
        items={[
          { label: "我的 Agents", href: "/agents" },
          { label: agent?.name ?? "Agent", href: `/agents/${id}` },
          { label: "微信连接" },
        ]}
      />

      <div className="page-head">
        <div>
          <h1>微信连接管理</h1>
          <p className="lead">查看连接状态、断开或重新绑定。凭据加密保存在 Agent 的独立数据目录。</p>
        </div>
      </div>

      {error ? <ApiErrorState message={error} status={errorStatus} /> : null}

      {fetching ? (
        <div className="row" style={{ color: "var(--muted)", fontSize: 13 }}>
          <Spinner size="sm" />
          <span>加载中…</span>
        </div>
      ) : (
        <>
          <section className="section-card">
            <div className="section-card-head">
              <h3 style={{ fontSize: 15, fontWeight: 600 }}>微信状态</h3>
              <span className="meta">仅扫码本人 · 群聊默认禁用</span>
            </div>
            <div className="section-card-body" style={{ padding: 0 }}>
              <div className="conn-row">
                <span className="avatar" style={{ width: 36, height: 36, fontSize: 14 }} aria-hidden="true">
                  {(agent?.name ?? "A").slice(0, 1)}
                </span>
                <div>
                  <div className="who-name">{agent?.name ?? "Agent"}</div>
                  <span className="meta">{id}</span>
                </div>
                <div className="col-tablet">
                  {channel ? <StatusChip kind="channel" value={channel.status} /> : <StatusChip kind="channel" value="not_configured" />}
                  <div className="meta" style={{ marginTop: 4 }}>
                    {channel?.externalAccountId ? `已绑定 ${channel.externalAccountId}` : "尚未绑定"}
                  </div>
                </div>
                <div className="row" style={{ gap: 6 }}>
                  {notConfigured || !connected ? (
                    <Button variant="primary" size="sm" onClick={() => router.push(`/agents/${id}/channels/weixin/bind`)}>
                      扫码绑定
                    </Button>
                  ) : null}
                  {connected ? (
                    <Button variant="danger" size="sm" onClick={() => setUnbindOpen(true)}>
                      解绑
                    </Button>
                  ) : null}
                </div>
              </div>
            </div>
          </section>

          <div className="card" style={{ display: "grid", gridTemplateColumns: "32px 1fr", gap: 16, alignItems: "flex-start", padding: "18px 22px" }}>
            <span style={{ color: "var(--accent)", marginTop: 2 }} aria-hidden="true">
              <Lock size={18} strokeWidth={1.75} />
            </span>
            <div>
              <div style={{ fontSize: 14, fontWeight: 600, marginBottom: 6 }}>凭据安全</div>
              <p className="muted" style={{ fontSize: 13, lineHeight: 1.6 }}>
                微信登录凭据被加密保存在每个 Agent 的独立数据目录中。它们从不出现在前端响应、日志或错误信息里，跨 Agent 不互通。运行时容器升级或重建都不会让你重新扫码。
              </p>
            </div>
          </div>
        </>
      )}

      <Modal
        open={unbindOpen}
        onClose={() => setUnbindOpen(false)}
        title={`解除「${agent?.name ?? ""}」的微信绑定？`}
        footer={
          <>
            <Button variant="ghost" size="sm" onClick={() => setUnbindOpen(false)}>
              取消
            </Button>
            <Button
              variant="danger"
              size="sm"
              disabled={pendingUnbind}
              loading={pendingUnbind}
              onClick={() => void handleUnbind()}
            >
              {pendingUnbind ? "解绑中…" : "确认解绑"}
            </Button>
          </>
        }
      >
        <p>解绑后这个 Agent 将停止接收微信消息，凭据从数据目录中清除。Agent 本身仍存在，可以扫一个新的码重新绑定。</p>
      </Modal>
    </>
  );
}
