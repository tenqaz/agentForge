"use client";

import { use } from "react";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { Check } from "lucide-react";

import ApiErrorState from "@/components/api-error-state";
import { useApiClient, useSessionState } from "@/components/app-shell";
import Breadcrumbs from "@/components/ui/breadcrumbs";
import Button from "@/components/ui/button";
import EventLog, { type LogLine } from "@/components/ui/event-log";
import Spinner from "@/components/ui/spinner";
import Timeline, { type TimelineItem, type TimelineItemState } from "@/components/ui/timeline";
import {
  apiErrorMessage,
  getAgent,
  getRuntime,
  listRuntimeJobs,
  type Agent,
  type AgentRuntime,
  type RuntimeJob,
} from "@/lib/api";

// 5 个供应阶段，按 agent.status 推进当前阶段
const STAGE_DEFS: { title: string; sub: string }[] = [
  { title: "分配独立数据目录", sub: "为该 Agent 创建隔离的数据与配置目录。" },
  { title: "复制模板内容", sub: "SOUL.md · USER.md · skills · 锁定模板版本。" },
  { title: "生成运行时配置", sub: "注入模型凭据 · 配置消息网关 · 准备日志通道。" },
  { title: "启动专属运行时容器", sub: "独立进程、独立端口、与其它 Agent 不互通。" },
  { title: "健康检查通过 · RUNNING", sub: "解锁微信扫码绑定入口。" },
];

function statusToStage(status: Agent["status"]): number {
  switch (status) {
    case "creating":
      return 0;
    case "provisioning":
      return 1;
    case "starting":
      return 3;
    case "running":
      return 5;
    case "error":
      return 1;
    default:
      return 0;
  }
}

function pctForStatus(status: Agent["status"]): number {
  switch (status) {
    case "creating":
      return 15;
    case "provisioning":
      return 45;
    case "starting":
      return 80;
    case "running":
      return 100;
    case "error":
      return 45;
    default:
      return 0;
  }
}

export default function ProvisionPage({
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
  const [jobs, setJobs] = useState<RuntimeJob[]>([]);
  const [errorStatus, setErrorStatus] = useState<number>();
  const [error, setError] = useState("");
  const [fetching, setFetching] = useState(true);
  const [elapsed, setElapsed] = useState(0);

  const refresh = useCallback(async () => {
    const [agentRes, runtimeRes, jobsRes] = await Promise.all([
      getAgent(apiClient, id),
      getRuntime(apiClient, id),
      listRuntimeJobs(apiClient, id),
    ]);
    if (agentRes.ok) setAgent(agentRes.data.agent);
    if (runtimeRes.ok) setRuntime(runtimeRes.data.runtime);
    if (jobsRes.ok) setJobs(jobsRes.data.jobs);
    return agentRes.ok ? agentRes.data.agent : null;
  }, [apiClient, id]);

  useEffect(() => {
    if (sessionLoading) return;
    if (!user) {
      router.replace("/login");
      return;
    }

    let active = true;
    void (async () => {
      const a = await refresh();
      if (!active) return;
      setFetching(false);
      if (!a) {
        const agentRes = await getAgent(apiClient, id);
        if (!agentRes.ok && active) {
          setErrorStatus(agentRes.status);
          setError(apiErrorMessage(agentRes.error.code, agentRes.error.message));
        }
      }
    })();

    return () => {
      active = false;
    };
  }, [apiClient, id, refresh, router, sessionLoading, user]);

  // 轮询：非终态时每 2s 刷新
  useEffect(() => {
    if (!agent) return;
    if (agent.status === "running" || agent.status === "error" || agent.status === "stopped") {
      return;
    }
    const timer = window.setInterval(() => {
      void refresh();
    }, 2000);
    return () => window.clearInterval(timer);
  }, [agent, refresh]);

  // 耗时计时
  useEffect(() => {
    if (!agent || agent.status === "running") return;
    const timer = window.setInterval(() => {
      setElapsed((e) => e + 1);
    }, 1000);
    return () => window.clearInterval(timer);
  }, [agent]);

  const status = agent?.status ?? "creating";
  const currentStage = statusToStage(status);
  const isError = status === "error";
  const isRunning = status === "running";
  const pct = pctForStatus(status);

  const timelineItems: TimelineItem[] = STAGE_DEFS.map((def, idx) => {
    let state: TimelineItemState = "pending";
    if (idx < currentStage) state = "done";
    else if (idx === currentStage && !isRunning) state = isError && idx === currentStage ? "error" : "active";
    if (isRunning) state = "done";
    return { title: def.title, sub: def.sub, state, time: state === "done" ? "已完成" : state === "active" ? "进行中" : "— —" };
  });

  const logLines: LogLine[] = jobs.map((job) => ({
    time: job.updatedAt.slice(11, 19),
    level: job.status === "failed" ? "err" : job.status === "succeeded" ? "ok" : "info",
    message: `${job.type} · ${job.status}${job.lastErrorCode ? ` · ${job.lastErrorCode}` : ""}`,
  }));
  if (agent) {
    logLines.unshift({
      time: agent.createdAt.slice(11, 19),
      level: "info",
      message: `agent_created · template=v${agent.templateVersion}`,
    });
  }
  if (isRunning) {
    logLines.push({ time: runtime?.updatedAt.slice(11, 19) ?? "", level: "ok", message: "✓ status -> RUNNING · ready for wechat bind" });
  }
  if (isError) {
    logLines.push({ time: runtime?.updatedAt.slice(11, 19) ?? "", level: "err", message: `✗ provisioning failed · ${agent?.lastErrorCode ?? ""} ${agent?.lastErrorMessage ?? ""}` });
  }

  const minutes = String(Math.floor(elapsed / 60)).padStart(2, "0");
  const seconds = String(elapsed % 60).padStart(2, "0");

  return (
    <>
      <Breadcrumbs
        items={[
          { label: "我的 Agents", href: "/agents" },
          { label: "新建", href: "/agents/new" },
          { label: "异步供应" },
        ]}
      />

      <div className="page-head">
        <div>
          <h1>正在准备「{agent?.name ?? "Agent"}」</h1>
          <p className="lead">分配数据目录 → 注入人格 → 启动运行时。整个过程通常在数十秒内完成，可以离开页面，回来继续看。</p>
        </div>
        <span className={`pill ${isRunning ? "pill-success" : isError ? "pill-danger" : "pill-info"}`}>
          <span className="dot live" />
          {isRunning ? "RUNNING" : isError ? "ERROR" : status.toUpperCase()}
        </span>
      </div>

      {error ? <ApiErrorState message={error} status={errorStatus} /> : null}

      {fetching ? (
        <div className="row" style={{ color: "var(--muted)", fontSize: 13 }}>
          <Spinner size="sm" />
          <span>加载中…</span>
        </div>
      ) : (
        <>
          <div className="card" style={{ display: "grid", gridTemplateColumns: "1fr auto", gap: 16, alignItems: "center" }}>
            <div>
              <span className="meta">总进度</span>
              <div className="row" style={{ gap: 12, alignItems: "baseline", marginTop: 4 }}>
                <span className="num" style={{ fontSize: 22, fontWeight: 600 }}>{pct}%</span>
                <span className="meta">{isRunning ? "全部完成" : isError ? "供应失败" : `阶段 ${Math.min(currentStage + 1, 5)} / 5 · 进行中`}</span>
              </div>
              <div className="progress-bar">
                <span style={{ width: `${pct}%` }} />
              </div>
            </div>
            <div style={{ textAlign: "right" }}>
              <span className="meta">耗时</span>
              <div className="num" style={{ fontSize: 18, marginTop: 4 }}>
                {minutes}:{seconds}
              </div>
            </div>
          </div>

          <div className="grid-2-1" style={{ gap: 24 }}>
            <section className="section-card">
              <div className="section-card-head">
                <h3 style={{ fontSize: 15, fontWeight: 600 }}>供应阶段</h3>
                <span className="meta">5 个阶段</span>
              </div>
              <div className="section-card-body">
                <Timeline items={timelineItems} />
              </div>
            </section>

            <section className="stack" style={{ gap: 16 }}>
              <div className="card">
                <span className="meta">Agent</span>
                <div className="row" style={{ gap: 10, marginTop: 10, alignItems: "center" }}>
                  <span className="avatar" style={{ width: 32, height: 32, fontSize: 13 }}>
                    {(agent?.name ?? "A").slice(0, 1)}
                  </span>
                  <div>
                    <div style={{ fontWeight: 500 }}>{agent?.name ?? "—"}</div>
                    <span className="meta">{id}</span>
                  </div>
                </div>
                <hr style={{ all: "unset", display: "block", height: 1, background: "var(--border)", margin: "14px 0" }} />
                <span className="meta">基于模板</span>
                <div style={{ marginTop: 6 }}>
                  <span className="tag tag-mono">v{agent?.templateVersion ?? "-"}</span>
                </div>
              </div>

              {isError ? (
                <div className="card" style={{ padding: 18 }}>
                  <span className="meta">出现问题？</span>
                  <p style={{ marginTop: 8, fontSize: 13, color: "var(--muted)", lineHeight: 1.6 }}>
                    {agent?.lastErrorMessage || "供应失败。平台会保留中间产物，可稍后重试或前往设置页重启运行时。"}
                  </p>
                  <Button
                    variant="secondary"
                    fullWidth
                    style={{ marginTop: 12 }}
                    onClick={() => router.push(`/agents/${id}/settings`)}
                  >
                    前往设置
                  </Button>
                </div>
              ) : null}
            </section>
          </div>

          <section className="section-card">
            <div className="section-card-head">
              <h3 style={{ fontSize: 15, fontWeight: 600 }}>事件流</h3>
              <span className="meta">来自运行时任务</span>
            </div>
            <div className="section-card-body" style={{ padding: 14 }}>
              <EventLog lines={logLines} maxHeight={320} />
            </div>
          </section>

          {isRunning ? (
            <div
              className="card"
              style={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                gap: 16,
                flexWrap: "wrap",
                borderColor: "var(--success)",
                background: "var(--success-soft)",
              }}
            >
              <div>
                <div className="row" style={{ gap: 8, fontWeight: 500 }}>
                  <span style={{ color: "var(--success)" }} aria-hidden="true">
                    <Check size={16} strokeWidth={2} />
                  </span>
                  Agent 已进入 RUNNING
                </div>
                <p style={{ marginTop: 6, color: "var(--muted)", fontSize: 13.5 }}>
                  下一步：扫一次微信码，让它活在你的微信里。
                </p>
              </div>
              <div className="row" style={{ gap: 8 }}>
                <Button variant="secondary" onClick={() => router.push("/agents")}>
                  稍后再说
                </Button>
                <Button variant="primary" onClick={() => router.push(`/agents/${id}/channels/weixin/bind`)}>
                  现在扫码绑定 →
                </Button>
              </div>
            </div>
          ) : null}
        </>
      )}
    </>
  );
}
