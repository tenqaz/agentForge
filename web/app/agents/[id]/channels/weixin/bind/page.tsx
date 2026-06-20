"use client";

import { use } from "react";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { Check, RefreshCw } from "lucide-react";

import ApiErrorState from "@/components/api-error-state";
import { useApiClient, useSessionState } from "@/components/app-shell";
import Breadcrumbs from "@/components/ui/breadcrumbs";
import Button from "@/components/ui/button";
import QRBox, { type QRState } from "@/components/ui/qr-box";
import Spinner from "@/components/ui/spinner";
import {
  apiErrorMessage,
  createWeixinPairingSession,
  ensureWeixinChannel,
  getAgent,
  getWeixinPairingSession,
  type Agent,
  type PairingSession,
} from "@/lib/api";

const STAGES = [
  { key: "qr", title: "展示二维码", sub: "平台从微信网关换取登录二维码并显示，30 秒过期会自动刷新。" },
  { key: "scanning", title: "检测到扫描", sub: "状态轮询从「等待扫码」切换到「等待手机确认」。" },
  { key: "confirmed", title: "微信内确认", sub: "凭据被服务端获取，加密保存到 Agent 的独立数据目录。" },
  { key: "bound", title: "网关启动 · 完成", sub: "消息网关启动，Agent 开始接收你本人的私信。群聊默认禁用。" },
];

function sessionToStage(status: PairingSession["status"]): number {
  switch (status) {
    case "connected":
      return 3;
    case "expired":
    case "failed":
      return 0;
    default:
      return 0;
  }
}

function sessionToQrState(status: PairingSession["status"]): QRState {
  switch (status) {
    case "connected":
      return "confirmed";
    case "expired":
    case "failed":
      return "expired";
    default:
      return "pending";
  }
}

export default function WeixinBindPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const apiClient = useApiClient();
  const router = useRouter();
  const { loading: sessionLoading, user } = useSessionState();
  const [agent, setAgent] = useState<Agent | null>(null);
  const [session, setSession] = useState<PairingSession | null>(null);
  const [pending, setPending] = useState(false);
  const [fetching, setFetching] = useState(true);
  const [errorStatus, setErrorStatus] = useState<number>();
  const [error, setError] = useState("");
  const [timeLeft, setTimeLeft] = useState(30);

  const createSession = useCallback(async () => {
    setPending(true);
    setError("");
    const ensureRes = await ensureWeixinChannel(apiClient, id);
    if (!ensureRes.ok) {
      setError(apiErrorMessage(ensureRes.error.code, ensureRes.error.message));
      setPending(false);
      return;
    }
    const pairRes = await createWeixinPairingSession(apiClient, id);
    setPending(false);
    if (!pairRes.ok) {
      setError(apiErrorMessage(pairRes.error.code, pairRes.error.message));
      return;
    }
    setSession(pairRes.data.session);
    setTimeLeft(30);
  }, [apiClient, id]);

  useEffect(() => {
    if (sessionLoading) return;
    if (!user) {
      router.replace("/login");
      return;
    }
    let active = true;
    void (async () => {
      const agentRes = await getAgent(apiClient, id);
      if (!active) return;
      setFetching(false);
      if (!agentRes.ok) {
        setErrorStatus(agentRes.status);
        setError(apiErrorMessage(agentRes.error.code, agentRes.error.message));
        return;
      }
      setAgent(agentRes.data.agent);
      void createSession();
    })();
    return () => {
      active = false;
    };
  }, [apiClient, id, createSession, router, sessionLoading, user]);

  // 轮询 pending session
  useEffect(() => {
    if (!session || session.status !== "pending") return;
    const timer = window.setInterval(async () => {
      const res = await getWeixinPairingSession(apiClient, id, session.id);
      if (res.ok) setSession(res.data.session);
    }, 2000);
    return () => window.clearInterval(timer);
  }, [apiClient, id, session]);

  // 倒计时
  useEffect(() => {
    if (!session || session.status !== "pending") return;
    if (timeLeft <= 0) return;
    const timer = window.setTimeout(() => setTimeLeft((t) => t - 1), 1000);
    return () => window.clearTimeout(timer);
  }, [session, timeLeft]);

  const status = session?.status ?? "pending";
  const stageIdx = sessionToStage(status);
  const qrState = sessionToQrState(status);
  const isBound = status === "connected";
  const isExpired = status === "expired" || status === "failed";

  return (
    <>
      <Breadcrumbs
        items={[
          { label: "我的 Agents", href: "/agents" },
          { label: agent?.name ?? "Agent", href: `/agents/${id}` },
          { label: "微信扫码绑定" },
        ]}
      />

      <div className="page-head">
        <div>
          <h1>微信扫码绑定</h1>
          <p className="lead">用你的微信扫描下方二维码，让 Agent 可以收发你本人的微信私信。凭据加密保存在 Agent 的独立数据目录。</p>
        </div>
        <span className={`pill ${isBound ? "pill-success" : "pill-info"}`}>
          <span className="dot live" />
          {isBound ? "已绑定" : isExpired ? "已失效" : "等待扫码"}
        </span>
      </div>

      {error ? <ApiErrorState message={error} status={errorStatus} /> : null}

      {fetching ? (
        <div className="row" style={{ color: "var(--muted)", fontSize: 13 }}>
          <Spinner size="sm" />
          <span>加载中…</span>
        </div>
      ) : (
        <div className="bind-grid">
          <section className="card card-pad-lg" style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 16 }}>
            <div style={{ textAlign: "center" }}>
              <span className="meta">扫码绑定 Agent</span>
              <h3 style={{ fontSize: 18, fontWeight: 600, marginTop: 4 }}>{agent?.name ?? "Agent"}</h3>
            </div>

            {session?.qrPayloadUrl ? (
              <QRBox payload={session.qrPayloadUrl} state={qrState} onRefresh={() => void createSession()} />
            ) : (
              <div className="qr-box" style={{ display: "grid", placeItems: "center" }}>
                <Spinner size="lg" />
              </div>
            )}

            <div className="meta" style={{ display: "inline-flex", alignItems: "center", gap: 8 }}>
              <span className={`status-dot ${isExpired ? "error" : isBound ? "running" : "pending"}`} />
              {isBound
                ? "消息网关已启动"
                : isExpired
                  ? "已失效，请刷新"
                  : `等待扫码 · ${timeLeft}s`}
            </div>

            <div className="row" style={{ gap: 8 }}>
              <Button
                variant="secondary"
                size="sm"
                leftIcon={<RefreshCw size={12} strokeWidth={1.75} />}
                disabled={pending}
                loading={pending}
                onClick={() => void createSession()}
              >
                刷新二维码
              </Button>
            </div>
          </section>

          <section className="stack" style={{ gap: 18 }}>
            <div className="card card-pad-lg">
              <h3 style={{ fontSize: 15, fontWeight: 600, marginBottom: 14 }}>流程</h3>
              <ul className="qr-stage-list">
                {STAGES.map((s, idx) => (
                  <li
                    key={s.key}
                    className={idx === stageIdx && !isBound ? "is-active" : idx < stageIdx || isBound ? "is-done" : ""}
                  >
                    <span className="ws-num">{idx < stageIdx || isBound ? "✓" : idx + 1}</span>
                    <div>
                      <strong>{s.title}</strong>
                      <span className="sub">{s.sub}</span>
                    </div>
                  </li>
                ))}
              </ul>
            </div>

            <div className="card" style={{ padding: 18, background: "var(--surface-2)" }}>
              <div style={{ fontSize: 13.5, fontWeight: 600, marginBottom: 6 }}>为什么这是安全的？</div>
              <ul style={{ listStyle: "none", padding: 0, margin: 0, display: "flex", flexDirection: "column", gap: 6, fontSize: 12.5, color: "var(--muted)", lineHeight: 1.6 }}>
                <li>· 凭据从不出现在前端响应、日志或错误信息里。</li>
                <li>· 仅扫码本人的微信私信会被回复；陌生人来信被默认忽略。</li>
                <li>· 群聊默认禁用，避免 Agent 被群成员滥用。</li>
              </ul>
            </div>
          </section>
        </div>
      )}

      {isBound ? (
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
              微信已绑定 · 凭据已加密落盘
            </div>
            <p style={{ marginTop: 6, color: "var(--muted)", fontSize: 13.5 }}>
              现在你给「{agent?.name ?? "Agent"}」在微信里发消息，它就会回复。
            </p>
          </div>
          <Button variant="primary" onClick={() => router.push(`/agents/${id}`)}>
            查看 Agent →
          </Button>
        </div>
      ) : null}
    </>
  );
}
