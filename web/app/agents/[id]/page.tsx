"use client";

import { use } from "react";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { Settings, Trash2 } from "lucide-react";

import ApiErrorState from "@/components/api-error-state";
import { useApiClient, useSessionState } from "@/components/app-shell";
import AgentRuntimeStatus from "@/components/agent-runtime-status";
import WeixinChannelPanel from "@/components/weixin-channel-panel";
import Breadcrumbs from "@/components/ui/breadcrumbs";
import Button from "@/components/ui/button";
import EventLog, { type LogLine } from "@/components/ui/event-log";
import Modal from "@/components/ui/modal";
import StatusChip from "@/components/ui/status-chip";
import Tabs from "@/components/ui/tabs";
import {
  apiErrorMessage,
  deleteAgent,
  getAgent,
  getRuntime,
  getWeixinChannel,
  listWeixinPairingSessions,
  type Agent,
  type AgentRuntime,
  type Channel,
  type PairingSession,
} from "@/lib/api";

export default function AgentDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const apiClient = useApiClient();
  const router = useRouter();
  const { loading: sessionLoading, user } = useSessionState();
  const [agent, setAgent] = useState<Agent | null>(null);
  const [runtime, setRuntime] = useState<AgentRuntime | null>(null);
  const [channel, setChannel] = useState<Channel | null>(null);
  const [session, setSession] = useState<PairingSession | null>(null);
  const [errorStatus, setErrorStatus] = useState<number>();
  const [error, setError] = useState("");
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [tab, setTab] = useState("status");

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
      const [agentResponse, runtimeResponse, channelResponse, sessionsResponse] =
        await Promise.all([
          getAgent(apiClient, id),
          getRuntime(apiClient, id),
          getWeixinChannel(apiClient, id),
          listWeixinPairingSessions(apiClient, id),
        ]);

      if (!active) {
        return;
      }
      if (!agentResponse.ok) {
        setErrorStatus(agentResponse.status);
        setError(apiErrorMessage(agentResponse.error.code, agentResponse.error.message));
        return;
      }
      if (!runtimeResponse.ok) {
        setErrorStatus(runtimeResponse.status);
        setError(apiErrorMessage(runtimeResponse.error.code, runtimeResponse.error.message));
        return;
      }
      if (!channelResponse.ok) {
        setErrorStatus(channelResponse.status);
        setError(apiErrorMessage(channelResponse.error.code, channelResponse.error.message));
        return;
      }

      setAgent(agentResponse.data.agent);
      setRuntime(runtimeResponse.data.runtime);
      setChannel(channelResponse.data.channel);
      if (sessionsResponse.ok) {
        setSession(sessionsResponse.data.sessions[0] ?? null);
      }
    })();

    return () => {
      active = false;
    };
  }, [apiClient, id, sessionLoading, router, user]);

  async function handleDelete() {
    setDeleting(true);
    setError("");
    setErrorStatus(undefined);
    const response = await deleteAgent(apiClient, id);
    if (!response.ok) {
      setDeleting(false);
      setErrorStatus(response.status);
      setError(apiErrorMessage(response.error.code, response.error.message));
      setConfirmingDelete(false);
      return;
    }
    router.push("/agents");
  }

  // 事件日志：用 agent/runtime 的状态与错误拼出可读日志行
  const logLines: LogLine[] = [];
  if (agent) {
    logLines.push({
      time: agent.createdAt.slice(11, 19),
      level: "info",
      message: `agent_created · id=${id} · template=v${agent.templateVersion}`,
    });
  }
  if (runtime) {
    logLines.push({
      time: runtime.updatedAt.slice(11, 19),
      level: runtime.status === "error" ? "err" : "ok",
      message: `runtime_status=${runtime.status}${
        runtime.lastErrorCode ? ` · ${runtime.lastErrorCode}` : ""
      }`,
    });
  }

  return (
    <>
      <Breadcrumbs
        items={[
          { label: "我的 Agents", href: "/agents" },
          { label: agent?.name ?? "Agent" },
        ]}
      />

      <div className="page-head">
        <div className="row" style={{ gap: 16, alignItems: "flex-start" }}>
          <span className="avatar" style={{ width: 48, height: 48, fontSize: 18, borderRadius: 14 }} aria-hidden="true">
            {(agent?.name ?? "A").slice(0, 1)}
          </span>
          <div>
            <h1>{agent?.name ?? "加载中…"}</h1>
            <div className="row" style={{ gap: 10, marginTop: 8, flexWrap: "wrap" }}>
              {agent ? <StatusChip kind="agent" value={agent.status} /> : null}
              <span className="tag tag-mono">v{agent?.templateVersion ?? "-"}</span>
              <span className="meta">{id} · 创建于 {agent?.createdAt.slice(0, 10) ?? "—"}</span>
            </div>
          </div>
        </div>
        <div className="row" style={{ gap: 8 }}>
          <Button
            variant="secondary"
            size="sm"
            leftIcon={<Settings size={14} strokeWidth={1.75} />}
            onClick={() => router.push(`/agents/${id}/settings`)}
          >
            设置
          </Button>
          <Button
            variant="ghost"
            size="sm"
            leftIcon={<Trash2 size={14} strokeWidth={1.75} />}
            onClick={() => setConfirmingDelete(true)}
            style={{ color: "var(--danger)" }}
          >
            删除 Agent
          </Button>
        </div>
      </div>

      {error ? <ApiErrorState message={error} status={errorStatus} /> : null}

      {runtime && channel ? (
        <>
          <Tabs
            items={[
              { key: "status", label: "状态" },
              { key: "events", label: "事件日志" },
              { key: "wx", label: "微信" },
            ]}
            value={tab}
            onChange={setTab}
          />

          {tab === "status" ? (
            <AgentRuntimeStatus
              agentId={id}
              runtime={runtime}
              onRuntimeChange={setRuntime}
            />
          ) : null}

          {tab === "events" ? (
            <section className="section-card">
              <div className="section-card-head">
                <h3>事件流</h3>
                <span className="meta">来自运行时状态</span>
              </div>
              <div className="section-card-body" style={{ padding: 14 }}>
                <EventLog lines={logLines} />
              </div>
            </section>
          ) : null}

          {tab === "wx" ? (
            <WeixinChannelPanel
              agentId={id}
              initialChannel={channel}
              initialSession={session}
              runtimeStatus={runtime.status}
            />
          ) : null}
        </>
      ) : null}

      <Modal
        open={confirmingDelete}
        onClose={() => setConfirmingDelete(false)}
        title={`删除「${agent?.name ?? ""}」？`}
        footer={
          <>
            <Button variant="ghost" size="sm" onClick={() => setConfirmingDelete(false)}>
              取消
            </Button>
            <Button
              variant="danger"
              size="sm"
              disabled={deleting}
              loading={deleting}
              onClick={() => void handleDelete()}
            >
              {deleting ? "删除中…" : "确认删除"}
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
